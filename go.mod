module cron-job-manager

go 1.24.0

toolchain go1.24.9

require github.com/joho/godotenv v1.5.1

require (
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/lib/pq v1.10.9
	github.com/robfig/cron/v3 v3.0.1
	golang.org/x/crypto v0.43.0
)
