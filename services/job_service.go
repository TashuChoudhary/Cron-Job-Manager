package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cron-job-manager/config"
	"cron-job-manager/models"
	"cron-job-manager/utils"

	"github.com/lib/pq"
)

// GetAllJobs retrieves all jobs with their dependencies
func GetAllJobs() ([]models.Job, error) {
	query := `
		SELECT j.id, j.name, j.description, j.cron_expression, j.command, j.is_active,
			   j.category, j.max_retries, j.retry_delay_seconds, j.timeout_seconds,
			   j.environment_vars, j.tags, j.priority, j.concurrent_executions,
			   j.created_at, j.updated_at
		FROM jobs j ORDER BY j.priority ASC, j.created_at DESC
	`

	rows, err := config.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := []models.Job{}
	for rows.Next() {
		var job models.Job
		var envVarsJSON string

		err := rows.Scan(&job.ID, &job.Name, &job.Description, &job.CronExpression,
			&job.Command, &job.IsActive, &job.Category, &job.MaxRetries,
			&job.RetryDelaySeconds, &job.TimeoutSeconds, &envVarsJSON,
			pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions,
			&job.CreatedAt, &job.UpdatedAt)

		if err != nil {
			log.Printf("Error scanning job: %v", err)
			continue
		}

		// Parse environment variables
		if err := json.Unmarshal([]byte(envVarsJSON), &job.EnvironmentVars); err != nil {
			job.EnvironmentVars = make(map[string]string)
		}

		// Load dependencies
		job.Dependencies = GetJobDependencies(job.ID)
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// GetJobDependencies gets dependencies for a specific job
func GetJobDependencies(jobID int) []models.JobDependency {
	rows, err := config.DB.Query(`
		SELECT jd.id, jd.job_id, jd.depends_on_job_id, jd.dependency_type, jd.created_at, j.name
		FROM job_dependencies jd
		JOIN jobs j ON j.id = jd.depends_on_job_id
		WHERE jd.job_id = $1
	`, jobID)

	if err != nil {
		return []models.JobDependency{}
	}
	defer rows.Close()

	var dependencies []models.JobDependency
	for rows.Next() {
		var dep models.JobDependency
		err := rows.Scan(&dep.ID, &dep.JobID, &dep.DependsOnJobID,
			&dep.DependencyType, &dep.CreatedAt, &dep.DependsOnJobName)
		if err != nil {
			continue
		}
		dependencies = append(dependencies, dep)
	}

	return dependencies
}

// CreateJob creates a new job
func CreateJob(job *models.Job) error {
	// Set defaults
	if job.Category == "" {
		job.Category = "general"
	}
	if job.TimeoutSeconds == 0 {
		job.TimeoutSeconds = 300
	}
	if job.Priority == 0 {
		job.Priority = 5
	}
	if job.ConcurrentExecutions == 0 {
		job.ConcurrentExecutions = 1
	}
	if job.EnvironmentVars == nil {
		job.EnvironmentVars = make(map[string]string)
	}

	// Convert environment variables to JSON
	envVarsJSON, _ := json.Marshal(job.EnvironmentVars)

	// Insert job into database
	err := config.DB.QueryRow(`
		INSERT INTO jobs (name, description, cron_expression, command, is_active, category, 
			max_retries, retry_delay_seconds, timeout_seconds, environment_vars, tags, 
			priority, concurrent_executions) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) 
		RETURNING id, created_at, updated_at
	`, job.Name, job.Description, job.CronExpression, job.Command, job.IsActive,
		job.Category, job.MaxRetries, job.RetryDelaySeconds, job.TimeoutSeconds,
		envVarsJSON, pq.Array(job.Tags), job.Priority, job.ConcurrentExecutions).Scan(
		&job.ID, &job.CreatedAt, &job.UpdatedAt)

	if err != nil {
		return err
	}

	// Broadcast job created event
	utils.BroadcastToClients(models.WSMessage{
		Type: "job_created",
		Payload: models.JobStatusUpdate{
			JobID:     job.ID,
			JobName:   job.Name,
			Status:    "created",
			Message:   "New job created with advanced features",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})

	return nil
}

// UpdateJob updates an existing job
func UpdateJob(jobID int, job *models.Job) error {
	if job.EnvironmentVars == nil {
		job.EnvironmentVars = make(map[string]string)
	}

	envVarsJSON, _ := json.Marshal(job.EnvironmentVars)

	_, err := config.DB.Exec(`
		UPDATE jobs 
		SET name = $1, description = $2, cron_expression = $3, command = $4, is_active = $5, 
			category = $6, max_retries = $7, retry_delay_seconds = $8, timeout_seconds = $9,
			environment_vars = $10, tags = $11, priority = $12, concurrent_executions = $13,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $14
	`, job.Name, job.Description, job.CronExpression, job.Command, job.IsActive,
		job.Category, job.MaxRetries, job.RetryDelaySeconds, job.TimeoutSeconds,
		envVarsJSON, pq.Array(job.Tags), job.Priority, job.ConcurrentExecutions, jobID)

	if err != nil {
		return err
	}

	// Broadcast job updated event
	utils.BroadcastToClients(models.WSMessage{
		Type: "job_updated",
		Payload: models.JobStatusUpdate{
			JobID:     jobID,
			JobName:   job.Name,
			Status:    "updated",
			Message:   "Job configuration updated",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})

	return nil
}

// DeleteJob deletes a job
func DeleteJob(jobID int) (string, error) {
	var jobName string
	config.DB.QueryRow("SELECT name FROM jobs WHERE id = $1", jobID).Scan(&jobName)

	result, err := config.DB.Exec("DELETE FROM jobs WHERE id = $1", jobID)
	if err != nil {
		return jobName, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		return jobName, fmt.Errorf("job not found")
	}

	// Broadcast job deleted event
	utils.BroadcastToClients(models.WSMessage{
		Type: "job_deleted",
		Payload: models.JobStatusUpdate{
			JobID:     jobID,
			JobName:   jobName,
			Status:    "deleted",
			Message:   "Job has been deleted",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})

	return jobName, nil
}

// GetJobExecutions gets execution history for a job
func GetJobExecutions(jobID int) ([]models.JobExecution, error) {
	rows, err := config.DB.Query(`
		SELECT id, job_id, status, started_at, completed_at, output, error_message, duration_ms,
			   retry_count, is_retry, parent_execution_id
		FROM job_executions WHERE job_id = $1 ORDER BY started_at DESC LIMIT 50
	`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var executions []models.JobExecution
	for rows.Next() {
		var exec models.JobExecution
		err := rows.Scan(&exec.ID, &exec.JobID, &exec.Status, &exec.StartedAt,
			&exec.CompletedAt, &exec.Output, &exec.ErrorMessage, &exec.DurationMs,
			&exec.RetryCount, &exec.IsRetry, &exec.ParentExecutionID)
		if err != nil {
			log.Printf("Error scanning execution: %v", err)
			continue
		}
		executions = append(executions, exec)
	}

	return executions, nil
}
