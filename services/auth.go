package services

import (
	"cron-job-manager/config"
	"cron-job-manager/models"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ⚠️ IMPORTANT: Change this in production!
	JWTSecret       = []byte("your-super-secret-jwt-key-change-this-in-production")
	TokenExpiration = 24 * time.Hour // 24 hours
)

// HashPassword hashes a plain text password
func HashPassword(password string) (string, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %v", err)
	}
	return string(hashedBytes), nil
}

// CheckPassword compares a hashed password with a plain text password
func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// CreateUser creates a new user
func CreateUser(req models.RegisterRequest) (*models.User, error) {
	if req.Username == "" || req.Password == "" || req.Email == "" {
		return nil, errors.New("username, email, and password are required")
	}

	hashedPassword, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	role := req.Role
	if role == "" {
		role = "user"
	}

	if role != "admin" && role != "user" && role != "viewer" {
		return nil, errors.New("invalid role")
	}

	var userID int
	err = config.DB.QueryRow(`
		INSERT INTO users (username, email, password, role, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, true, $5, $6)
		RETURNING id
	`, req.Username, req.Email, hashedPassword, role, time.Now(), time.Now()).Scan(&userID)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			if strings.Contains(err.Error(), "username") {
				return nil, errors.New("username already exists")
			}
			if strings.Contains(err.Error(), "email") {
				return nil, errors.New("email already exists")
			}
		}
		return nil, fmt.Errorf("failed to create user: %v", err)
	}

	return GetUserByID(userID)
}

// AuthenticateUser authenticates a user
func AuthenticateUser(username, password string) (*models.User, error) {
	var user models.User
	var hashedPassword string
	var lastLogin sql.NullTime

	err := config.DB.QueryRow(`
		SELECT id, username, email, password, role, is_active, created_at, updated_at, last_login
		FROM users
		WHERE username = $1 OR email = $1
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &hashedPassword,
		&user.Role, &user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("invalid username or password")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	if !user.IsActive {
		return nil, errors.New("user account is disabled")
	}

	if err := CheckPassword(hashedPassword, password); err != nil {
		return nil, errors.New("invalid username or password")
	}

	config.DB.Exec("UPDATE users SET last_login = $1 WHERE id = $2", time.Now(), user.ID)

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// GenerateToken generates a JWT token
func GenerateToken(user *models.User) (string, time.Time, error) {
	expirationTime := time.Now().Add(TokenExpiration)

	claims := &models.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "cronjob-manager",
			Subject:   user.Username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(JWTSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate token: %v", err)
	}

	return tokenString, expirationTime, nil
}

// ValidateToken validates a JWT token
func ValidateToken(tokenString string) (*models.Claims, error) {
	claims := &models.Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return JWTSecret, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// GetUserByID retrieves a user by ID
func GetUserByID(id int) (*models.User, error) {
	var user models.User
	var lastLogin sql.NullTime

	err := config.DB.QueryRow(`
		SELECT id, username, email, role, is_active, created_at, updated_at, last_login
		FROM users
		WHERE id = $1
	`, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Role,
		&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
	)

	if err == sql.ErrNoRows {
		return nil, errors.New("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return &user, nil
}

// GetAllUsers retrieves all users
func GetAllUsers() ([]models.User, error) {
	rows, err := config.DB.Query(`
		SELECT id, username, email, role, is_active, created_at, updated_at, last_login
		FROM users
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %v", err)
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		var lastLogin sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Username, &user.Email, &user.Role,
			&user.IsActive, &user.CreatedAt, &user.UpdatedAt, &lastLogin,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %v", err)
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// UpdateUser updates user information
func UpdateUser(id int, updates map[string]interface{}) error {
	query := "UPDATE users SET updated_at = $1"
	args := []interface{}{time.Now()}
	paramCount := 2

	if username, ok := updates["username"].(string); ok && username != "" {
		query += fmt.Sprintf(", username = $%d", paramCount)
		args = append(args, username)
		paramCount++
	}

	if email, ok := updates["email"].(string); ok && email != "" {
		query += fmt.Sprintf(", email = $%d", paramCount)
		args = append(args, email)
		paramCount++
	}

	if password, ok := updates["password"].(string); ok && password != "" {
		hashedPassword, err := HashPassword(password)
		if err != nil {
			return err
		}
		query += fmt.Sprintf(", password = $%d", paramCount)
		args = append(args, hashedPassword)
		paramCount++
	}

	if role, ok := updates["role"].(string); ok && role != "" {
		query += fmt.Sprintf(", role = $%d", paramCount)
		args = append(args, role)
		paramCount++
	}

	if isActive, ok := updates["is_active"].(bool); ok {
		query += fmt.Sprintf(", is_active = $%d", paramCount)
		args = append(args, isActive)
		paramCount++
	}

	query += fmt.Sprintf(" WHERE id = $%d", paramCount)
	args = append(args, id)

	_, err := config.DB.Exec(query, args...)
	return err
}

// DeleteUser soft deletes a user
func DeleteUser(id int) error {
	_, err := config.DB.Exec("UPDATE users SET is_active = false WHERE id = $1", id)
	return err
}
