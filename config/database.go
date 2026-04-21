package config

import (
	"database/sql"
	_ "embed"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var DB *sql.DB

var initSQL string

// InitDatabase initializes the database connection
func InitDatabase() {
	godotenv.Load()

	var dsn string

	// Render provides DATABASE_URL — always takes priority
	if url := os.Getenv("DATABASE_URL"); url != "" {
		dsn = url
		log.Println("📦 Using DATABASE_URL (Render/prod mode)")
	} else {

		host := getEnvRequired("DB_HOST")
		port := getEnvRequired("DB_PORT")
		user := getEnvRequired("DB_USER")
		password := getEnvRequired("DB_PASSWORD")
		dbname := getEnvRequired("DB_NAME")
		sslmode := getEnv("DB_SSLMODE", "disable")

		dsn = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
			host, port, user, password, dbname, sslmode)
		log.Println("🔧 Using individual DB vars (local dev mode)")
	}

	var err error
	DB, err = sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatal("Error connecting to database:", err)
	}

	log.Println("✅ Successfully connected to PostgreSQL database!")

	initSchema()

}

// CloseDatabase closes the database connection
func CloseDatabase() {
	if DB != nil {
		DB.Close()
		log.Println("Database connection closed")
	}
}

// getEnv gets environment variable with default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvRequired(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("FATAL: Required environment variable %s is not set!", key)
	}
	return value
}

func initSchema() {
	// Read the existing init.sql file embedded in the Docker image
	sqlBytes, err := os.ReadFile("./docker/postgres/init.sql")
	if err != nil {
		log.Printf("⚠️  Could not read init.sql: %v (skipping schema init)", err)
		return
	}

	if _, err := DB.Exec(string(sqlBytes)); err != nil {
		log.Fatalf("❌ Schema initialization failed: %v", err)
	}
	log.Println("✅ Schema initialized from init.sql successfully")
}
