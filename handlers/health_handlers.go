package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"cron-job-manager/config"
	"cron-job-manager/services"
	"cron-job-manager/utils"
)

// HealthHandler provides health check endpoint
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	err := config.DB.Ping()
	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "unhealthy",
			"error":  "database connection failed",
		})
		return
	}

	var totalJobs, activeJobs, runningJobsCount int
	config.DB.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&totalJobs)
	config.DB.QueryRow("SELECT COUNT(*) FROM jobs WHERE is_active = true").Scan(&activeJobs)
	config.DB.QueryRow("SELECT COUNT(*) FROM job_executions WHERE status = 'running'").Scan(&runningJobsCount)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "healthy",
		"timestamp":    time.Now().Format(time.RFC3339),
		"total_jobs":   totalJobs,
		"active_jobs":  activeJobs,
		"running_jobs": runningJobsCount,
		"cron_entries": len(services.JobEntries),
		"ws_clients":   utils.GetClientCount(),
		"features": []string{
			"job_dependencies",
			"retry_logic",
			"environment_variables",
			"job_templates",
			"concurrent_execution_limits",
			"job_categories",
			"priority_scheduling",
		},
	})
}
