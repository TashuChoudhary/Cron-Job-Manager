package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"cron-job-manager/config"
	"cron-job-manager/handlers"
	"cron-job-manager/middleware"
	"cron-job-manager/services"
	"cron-job-manager/utils"

	"github.com/gorilla/mux"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func loadEnvironmentVariables() {
	//  Add AUTH_REQUIRED configuration
	authRequired := getEnv("AUTH_REQUIRED", "false")
	os.Setenv("AUTH_REQUIRED", authRequired)
	log.Printf("🔐 AUTH_REQUIRED set to: %s", authRequired)

	notificationServiceURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if notificationServiceURL == "" {
		log.Printf("🔔 NOTIFICATION_SERVICE_URL not set, using default: http://localhost:3001")
		os.Setenv("NOTIFICATION_SERVICE_URL", "http://localhost:3001")
	}

	requiredNotificationVars := []string{
		"SMTP_HOST",
		"SMTP_USER",
		"SMTP_PASS",
		"FROM_EMAIL",
	}

	missingVars := []string{}
	for _, envVar := range requiredNotificationVars {
		if os.Getenv(envVar) == "" {
			missingVars = append(missingVars, envVar)
		}
	}

	if len(missingVars) > 0 {
		log.Printf("⚠️  Warning: Missing notification environment variables: %v", missingVars)
		log.Printf("📧 Email notifications will not work until these are configured")
	}
}

func performStartupHealthChecks() {
	log.Println("🏥 Performing startup health checks...")

	if err := services.CheckNotificationServiceHealth(); err != nil {
		log.Printf("⚠️  Notification service health check failed: %v", err)
	} else {
		log.Printf("✅ Notification service is healthy")
	}
}

//func gracefulShutdown() {
//	log.Println("🛑 Shutting down gracefully...")
//	log.Println("🔔 Allowing notification service to finish pending notifications...")
//	log.Println("✅ Shutdown complete")
//}

func main() {
	loadEnvironmentVariables()

	handlers.InitSampleWorkflows()
	handlers.SetupWorkflowRoutes()

	config.InitDatabase()
	defer config.CloseDatabase()

	utils.InitWebSocket()
	services.InitCronManager()
	defer services.StopCronManager()

	log.Println("🔔 Initializing notification services...")

	if err := services.InitNotificationService(); err != nil {
		log.Printf("⚠️  Warning: Notification service initialization failed: %v", err)
		log.Printf("📄 Job execution will continue, but notifications will be disabled")
	} else {
		log.Printf("✅ Notification service initialized successfully")
	}

	router := mux.NewRouter()
	router.Use(enableCORS)

	// WebSocket endpoint
	router.HandleFunc("/ws", handlers.WebSocketHandler)

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Public endpoints (no auth required)
	api.HandleFunc("/auth/login", handlers.LoginHandler).Methods("POST")
	api.HandleFunc("/auth/register", handlers.RegisterHandler).Methods("POST")
	api.HandleFunc("/auth/logout", handlers.LogoutHandler).Methods("POST")
	api.HandleFunc("/health", handlers.HealthHandler).Methods("GET")

	protected := api.PathPrefix("").Subrouter()
	protected.Use(middleware.ConditionalAuthMiddleware)

	// User routes
	protected.HandleFunc("/auth/me", handlers.GetCurrentUserHandler).Methods("GET")

	// Job management (all authenticated users can view)
	protected.HandleFunc("/jobs", handlers.GetJobsHandler).Methods("GET")
	protected.HandleFunc("/jobs/{id:[0-9]+}", handlers.GetJobExecutionsHandler).Methods("GET")
	protected.HandleFunc("/jobs/{id:[0-9]+}/executions", handlers.GetJobExecutionsHandler).Methods("GET")
	protected.HandleFunc("/jobs/{id:[0-9]+}/stats", handlers.GetExecutionStatsHandler).Methods("GET")

	adminUser := protected.PathPrefix("").Subrouter()
	adminUser.Use(middleware.ConditionalRoleMiddleware("admin", "user"))
	adminUser.HandleFunc("/jobs", handlers.CreateJobHandler).Methods("POST")
	adminUser.HandleFunc("/jobs/{id:[0-9]+}", handlers.UpdateJobHandler).Methods("PUT")
	adminUser.HandleFunc("/jobs/{id:[0-9]+}/trigger", handlers.TriggerJobHandler).Methods("POST")

	// Replace AdminMiddleware with ConditionalAdminMiddleware
	admin := protected.PathPrefix("").Subrouter()
	admin.Use(middleware.ConditionalAdminMiddleware)
	admin.HandleFunc("/jobs/{id:[0-9]+}", handlers.DeleteJobHandler).Methods("DELETE")

	// User management (admin only)
	admin.HandleFunc("/users", handlers.GetAllUsersHandler).Methods("GET")
	admin.HandleFunc("/users/{id:[0-9]+}", handlers.UpdateUserHandler).Methods("PUT")
	admin.HandleFunc("/users/{id:[0-9]+}", handlers.DeleteUserHandler).Methods("DELETE")

	// Execution logs routes (authenticated users)
	protected.HandleFunc("/logs", handlers.GetExecutionLogsHandler).Methods("GET")
	protected.HandleFunc("/logs/recent", handlers.GetRecentLogsHandler).Methods("GET")
	protected.HandleFunc("/logs/{id:[0-9]+}", handlers.GetExecutionLogByIDHandler).Methods("GET")

	// Job dependencies (authenticated)
	protected.HandleFunc("/jobs/{id:[0-9]+}/dependencies", handlers.AddJobDependencyHandler).Methods("POST")
	protected.HandleFunc("/dependencies/{dependency_id:[0-9]+}", handlers.RemoveJobDependencyHandler).Methods("DELETE")

	// Job templates (authenticated)
	protected.HandleFunc("/templates", handlers.GetJobTemplatesHandler).Methods("GET")
	protected.HandleFunc("/templates/{id:[0-9]+}/create-job", handlers.CreateJobFromTemplateHandler).Methods("POST")

	performStartupHealthChecks()

	// Start periodic system stats broadcast
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			services.BroadcastSystemStats()
		}
	}()

	fs := http.FileServer(http.Dir("./frontend"))
	router.PathPrefix("/").Handler(fs)

	port := getEnv("PORT", "5000")

	authRequired := os.Getenv("AUTH_REQUIRED")
	authStatus := "DISABLED ❌"
	if authRequired == "true" {
		authStatus = "ENABLED ✅"
	}

	log.Printf("🚀 Advanced Cronjob Manager Server starting on port %s", port)
	log.Printf("🔐 Authentication: %s", authStatus)
	log.Printf("📊 Health check: http://localhost:%s/api/v1/health", port)
	log.Printf("🔧 API endpoints: http://localhost:%s/api/v1/jobs", port)
	log.Printf("🌐 WebSocket endpoint: ws://localhost:%s/ws", port)
	log.Printf("⏰ Cron scheduler running with advanced features")
	log.Printf("🎯 Features: Auth, Dependencies, Retries, Templates, Execution Logs")
	log.Printf("📁 Clean architecture: handlers, services, models, config, utils, middleware")
	log.Printf("")

	if authRequired == "true" {
		log.Printf("🔑 Default admin credentials:")
		log.Printf("   Username: admin")
		log.Printf("   Password: admin123")
		log.Printf("⚠️  IMPORTANT: Change the default password immediately!")
	} else {
		log.Printf("⚠️  WARNING: Authentication is DISABLED")
		log.Printf("💡 All API endpoints are publicly accessible")
		log.Printf("💡 To enable auth: Set AUTH_REQUIRED=true")
		log.Printf("💡 Example: AUTH_REQUIRED=true go run main.go")
	}

	log.Fatal(http.ListenAndServe(":"+port, router))
}
