package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"

	"cron-job-manager/models"
	"cron-job-manager/services"
)

// AuthMiddleware validates JWT token
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Extract token (format: "Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := services.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Get user from database
		user, err := services.GetUserByID(claims.UserID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Check if user is active
		if !user.IsActive {
			http.Error(w, "User account is disabled", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminMiddleware checks if user has admin role
func AdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value("user").(*models.User)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if user.Role != "admin" {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RoleMiddleware checks if user has one of the allowed roles
func RoleMiddleware(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := r.Context().Value("user").(*models.User)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			allowed := false
			for _, role := range allowedRoles {
				if user.Role == role {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ConditionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ✅ CHECK: Is authentication required?
		authRequired := os.Getenv("AUTH_REQUIRED")

		// ✅ IF AUTH NOT REQUIRED: Skip validation and proceed
		if authRequired != "true" {
			// Create a dummy user for context (so handlers don't break)
			dummyUser := &models.User{
				ID:       0,
				Username: "anonymous",
				Role:     "user",
				IsActive: true,
			}
			ctx := context.WithValue(r.Context(), "user", dummyUser)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// ✅ IF AUTH REQUIRED: Perform full JWT validation (same as AuthMiddleware)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing authorization header", http.StatusUnauthorized)
			return
		}

		// Extract token (format: "Bearer <token>")
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		claims, err := services.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Get user from database
		user, err := services.GetUserByID(claims.UserID)
		if err != nil {
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Check if user is active
		if !user.IsActive {
			http.Error(w, "User account is disabled", http.StatusUnauthorized)
			return
		}

		// Add user to request context
		ctx := context.WithValue(r.Context(), "user", user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ConditionalAdminMiddleware checks for admin role ONLY if AUTH_REQUIRED=true
func ConditionalAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ✅ CHECK: Is authentication required?
		authRequired := os.Getenv("AUTH_REQUIRED")

		// ✅ IF AUTH NOT REQUIRED: Skip admin check
		if authRequired != "true" {
			next.ServeHTTP(w, r)
			return
		}

		// ✅ IF AUTH REQUIRED: Check admin role
		user, ok := r.Context().Value("user").(*models.User)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if user.Role != "admin" {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ConditionalRoleMiddleware checks roles ONLY if AUTH_REQUIRED=true
func ConditionalRoleMiddleware(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// ✅ CHECK: Is authentication required?
			authRequired := os.Getenv("AUTH_REQUIRED")

			// ✅ IF AUTH NOT REQUIRED: Skip role check
			if authRequired != "true" {
				next.ServeHTTP(w, r)
				return
			}

			// ✅ IF AUTH REQUIRED: Check user roles
			user, ok := r.Context().Value("user").(*models.User)
			if !ok {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			allowed := false
			for _, role := range allowedRoles {
				if user.Role == role {
					allowed = true
					break
				}
			}

			if !allowed {
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
