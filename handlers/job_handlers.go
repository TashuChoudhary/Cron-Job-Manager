package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"cron-job-manager/config"
	"cron-job-manager/models"
	"cron-job-manager/services"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

// GetJobsHandler retrieves all jobs
func GetJobsHandler(w http.ResponseWriter, r *http.Request) {
	jobs, err := services.GetAllJobs()
	if err != nil {
		http.Error(w, "Error fetching jobs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

// CreateJobHandler creates a new job
func CreateJobHandler(w http.ResponseWriter, r *http.Request) {
	var job models.Job
	err := json.NewDecoder(r.Body).Decode(&job)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	if job.Name == "" || job.CronExpression == "" || job.Command == "" {
		http.Error(w, "Missing required fields: name, cron_expression, command", http.StatusBadRequest)
		return
	}

	// Validate cron expression format
	cronFields := strings.Fields(job.CronExpression)
	if len(cronFields) != 6 {
		http.Error(w, "Cron expression must have 6 fields", http.StatusBadRequest)
		return
	}

	// Test cron expression
	testCron := cron.New(cron.WithSeconds())
	_, err = testCron.AddFunc(job.CronExpression, func() {})
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid cron expression: %v", err), http.StatusBadRequest)
		return
	}

	err = services.CreateJob(&job)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Job name already exists", http.StatusConflict)
		} else {
			http.Error(w, "Error creating job", http.StatusInternalServerError)
		}
		return
	}

	// Schedule the job if it's active
	if job.IsActive {
		services.ScheduleJob(job)
	}

	services.BroadcastSystemStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}

// UpdateJobHandler updates an existing job
func UpdateJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	var job models.Job
	err = json.NewDecoder(r.Body).Decode(&job)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	err = services.UpdateJob(jobID, &job)
	if err != nil {
		http.Error(w, "Error updating job", http.StatusInternalServerError)
		return
	}

	// Handle scheduling changes
	services.UnscheduleJob(jobID)
	if job.IsActive {
		job.ID = jobID
		services.ScheduleJob(job)
	}

	// Fetch updated job
	var envVarsStr string
	err = config.DB.QueryRow(`
		SELECT id, name, description, cron_expression, command, is_active, category, max_retries, 
			   retry_delay_seconds, timeout_seconds, environment_vars, tags, priority, 
			   concurrent_executions, created_at, updated_at 
		FROM jobs WHERE id = $1
	`, jobID).Scan(&job.ID, &job.Name, &job.Description, &job.CronExpression, &job.Command,
		&job.IsActive, &job.Category, &job.MaxRetries, &job.RetryDelaySeconds, &job.TimeoutSeconds,
		&envVarsStr, pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions,
		&job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		http.Error(w, "Error fetching updated job", http.StatusInternalServerError)
		return
	}

	json.Unmarshal([]byte(envVarsStr), &job.EnvironmentVars)
	job.Dependencies = services.GetJobDependencies(job.ID)

	services.BroadcastSystemStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// DeleteJobHandler deletes a job
func DeleteJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	// Remove from scheduler
	services.UnscheduleJob(jobID)

	_, err = services.DeleteJob(jobID)
	if err != nil {
		if err.Error() == "job not found" {
			http.Error(w, "Job not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error deleting job", http.StatusInternalServerError)
		}
		return
	}

	services.BroadcastSystemStats()
	w.WriteHeader(http.StatusNoContent)
}

// TriggerJobHandler manually triggers a job
func TriggerJobHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	// Get job details
	var job models.Job
	var envVarsStr string
	err = config.DB.QueryRow(`
		SELECT id, name, description, cron_expression, command, is_active, category, max_retries,
			   retry_delay_seconds, timeout_seconds, environment_vars, tags, priority, concurrent_executions
		FROM jobs WHERE id = $1
	`, jobID).Scan(&job.ID, &job.Name, &job.Description, &job.CronExpression, &job.Command, &job.IsActive,
		&job.Category, &job.MaxRetries, &job.RetryDelaySeconds, &job.TimeoutSeconds, &envVarsStr,
		pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions)

	if err == sql.ErrNoRows {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Error fetching job", http.StatusInternalServerError)
		return
	}

	json.Unmarshal([]byte(envVarsStr), &job.EnvironmentVars)

	// Execute job manually
	go services.ExecuteJob(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "triggered",
		"message": fmt.Sprintf("Job '%s' has been triggered manually", job.Name),
	})
}

// GetJobExecutionsHandler gets execution history for a job
func GetJobExecutionsHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	executions, err := services.GetJobExecutions(jobID)
	if err != nil {
		http.Error(w, "Error fetching executions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(executions)
}

// AddJobDependencyHandler adds a dependency to a job
func AddJobDependencyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	var req struct {
		DependsOnJobID int    `json:"depends_on_job_id"`
		DependencyType string `json:"dependency_type"`
	}

	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid JSON data", http.StatusBadRequest)
		return
	}

	if req.DependencyType == "" {
		req.DependencyType = "success"
	}

	// Validate dependency type
	validTypes := []string{"success", "completion", "failure"}
	valid := false
	for _, t := range validTypes {
		if req.DependencyType == t {
			valid = true
			break
		}
	}
	if !valid {
		http.Error(w, "Invalid dependency type. Must be: success, completion, or failure", http.StatusBadRequest)
		return
	}

	// Check for circular dependencies
	if jobID == req.DependsOnJobID {
		http.Error(w, "Job cannot depend on itself", http.StatusBadRequest)
		return
	}

	// Insert dependency
	_, err = config.DB.Exec(`
		INSERT INTO job_dependencies (job_id, depends_on_job_id, dependency_type)
		VALUES ($1, $2, $3)
	`, jobID, req.DependsOnJobID, req.DependencyType)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Dependency already exists", http.StatusConflict)
		} else {
			http.Error(w, "Error creating dependency", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "created",
		"message": "Job dependency added successfully",
	})
}

// RemoveJobDependencyHandler removes a job dependency
func RemoveJobDependencyHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dependencyID, err := strconv.Atoi(vars["dependency_id"])
	if err != nil {
		http.Error(w, "Invalid dependency ID", http.StatusBadRequest)
		return
	}

	result, err := config.DB.Exec("DELETE FROM job_dependencies WHERE id = $1", dependencyID)
	if err != nil {
		http.Error(w, "Error removing dependency", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		http.Error(w, "Dependency not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
