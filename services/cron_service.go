package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"cron-job-manager/config"
	"cron-job-manager/metrics"
	"cron-job-manager/models"
	"cron-job-manager/utils"

	"github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

var (
	CronManager    *cron.Cron
	JobEntries     map[int]cron.EntryID
	RunningJobs    map[int]int
	RunningJobsMux sync.RWMutex

	notificationService *NotificationService
)

func InitCronManager() {
	CronManager = cron.New(cron.WithSeconds())
	JobEntries = make(map[int]cron.EntryID)
	RunningJobs = make(map[int]int)

	// Initialize notification service
	notificationService = GetNotificationService()

	LoadAndScheduleJobs()
	CronManager.Start()
	log.Println("✅ Cron manager started!")
}

// StopCronManager stops the cron manager
func StopCronManager() {
	if CronManager != nil {
		CronManager.Stop()
		log.Println("Cron manager stopped")
	}
}

// LoadAndScheduleJobs loads all active jobs and schedules them
func LoadAndScheduleJobs() {
	rows, err := config.DB.Query(`
		SELECT id, name, cron_expression, command, category, max_retries, retry_delay_seconds,
				timeout_seconds, environment_vars, tags, priority, concurrent_executions
		FROM jobs WHERE is_active = true
	`)
	if err != nil {
		log.Printf("❌ Error loading jobs for scheduling: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var job models.Job
		var envVarsJSON string

		err := rows.Scan(&job.ID, &job.Name, &job.CronExpression, &job.Command,
			&job.Category, &job.MaxRetries, &job.RetryDelaySeconds, &job.TimeoutSeconds,
			&envVarsJSON, pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions)

		if err != nil {
			log.Printf("❌ Error scanning job: %v", err)
			continue
		}

		// Parse environment variables
		if err := json.Unmarshal([]byte(envVarsJSON), &job.EnvironmentVars); err != nil {
			job.EnvironmentVars = make(map[string]string)
		}

		ScheduleJob(job)
		count++
	}

	log.Printf("✅ Scheduled %d active jobs", count)
}

// ScheduleJob schedules a single job
func ScheduleJob(job models.Job) {
	entryID, err := CronManager.AddFunc(job.CronExpression, func() {
		ExecuteJob(job)
	})

	if err != nil {
		log.Printf("❌ Error scheduling job '%s': %v", job.Name, err)
		LogJobEvent(job.ID, 0, "error", fmt.Sprintf("Failed to schedule job: %v", err))
		return
	}

	JobEntries[job.ID] = entryID
	log.Printf("✅ Scheduled job '%s' with cron expression '%s'", job.Name, job.CronExpression)

	// Broadcast job scheduled event
	utils.BroadcastToClients(models.WSMessage{
		Type: "job_scheduled",
		Payload: models.JobStatusUpdate{
			JobID:     job.ID,
			JobName:   job.Name,
			Status:    "scheduled",
			Message:   "Job has been scheduled",
			Timestamp: time.Now().Format(time.RFC3339),
		},
	})
}

// UnscheduleJob removes a job from the scheduler
func UnscheduleJob(jobID int) {
	if entryID, exists := JobEntries[jobID]; exists {
		CronManager.Remove(entryID)
		delete(JobEntries, jobID)
		log.Printf("✅ Unscheduled job ID %d", jobID)
	}
}

// ExecuteJob executes a job with dependency and concurrency checks
func ExecuteJob(job models.Job) {
	// Check dependencies
	canRun, err := CheckJobDependencies(job.ID)
	if err != nil {
		log.Printf("❌ Error checking dependencies for job '%s': %v", job.Name, err)
		return
	}
	if !canRun {
		log.Printf("⏳ Job '%s' skipped: dependencies not satisfied", job.Name)
		utils.BroadcastToClients(models.WSMessage{
			Type: "job_dependency_wait",
			Payload: models.JobStatusUpdate{
				JobID:     job.ID,
				JobName:   job.Name,
				Status:    "waiting",
				Message:   "Waiting for dependencies",
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	// Check concurrent execution limit
	if !CanJobRun(job.ID, job.ConcurrentExecutions) {
		log.Printf("⏳ Job '%s' skipped: max concurrent executions reached (%d)", job.Name, job.ConcurrentExecutions)
		utils.BroadcastToClients(models.WSMessage{
			Type: "job_concurrent_limit",
			Payload: models.JobStatusUpdate{
				JobID:     job.ID,
				JobName:   job.Name,
				Status:    "throttled",
				Message:   fmt.Sprintf("Max concurrent executions reached (%d)", job.ConcurrentExecutions),
				Timestamp: time.Now().Format(time.RFC3339),
			},
		})
		return
	}

	ExecuteJobWithRetry(job, 0, nil)
}

// ExecuteJobWithRetry executes a job with retry logic and notifications
func ExecuteJobWithRetry(job models.Job, retryCount int, parentExecutionID *int) {
	log.Printf("🚀 Executing job '%s' (attempt %d/%d): %s", job.Name, retryCount+1, job.MaxRetries+1, job.Command)

	metrics.RunningJobs.Inc()

	TrackRunningJob(job.ID, true)
	defer TrackRunningJob(job.ID, false)

	startTime := time.Now()

	// Broadcast job started event
	utils.BroadcastToClients(models.WSMessage{
		Type: "job_started",
		Payload: models.JobStatusUpdate{
			JobID:      job.ID,
			JobName:    job.Name,
			Status:     "running",
			Message:    fmt.Sprintf("Job started (attempt %d)", retryCount+1),
			Timestamp:  startTime.Format(time.RFC3339),
			RetryCount: retryCount,
		},
	})

	// Create execution record
	var executionID int
	err := config.DB.QueryRow(`
		INSERT INTO job_executions (job_id, status, started_at, retry_count, is_retry, parent_execution_id)
		VALUES ($1, 'running', $2, $3, $4, $5)
		RETURNING id
	`, job.ID, startTime, retryCount, retryCount > 0, parentExecutionID).Scan(&executionID)

	if err != nil {
		log.Printf("❌ Error creating execution record: %v", err)
		return
	}

	// Substitute environment variables in command
	finalCommand := SubstituteEnvVars(job.Command, job.EnvironmentVars)

	// Log job start
	LogJobEvent(job.ID, executionID, "info", fmt.Sprintf("Job started (attempt %d): %s", retryCount+1, finalCommand))

	// Execute the command with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(job.TimeoutSeconds)*time.Second)
	defer cancel()

	output, execErr := RunCommandWithContext(ctx, finalCommand)
	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Determine status and exit code
	status := "success"
	errorMessage := ""
	exitCode := 0
	isTimeout := false

	if execErr != nil {
		status = "failed"
		errorMessage = execErr.Error()

		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			isTimeout = true
			exitCode = 124 // Standard timeout exit code
		} else if exitError, ok := execErr.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = 1
		}

		log.Printf("❌ Job '%s' failed (attempt %d): %v", job.Name, retryCount+1, execErr)
		LogJobEvent(job.ID, executionID, "error", fmt.Sprintf("Job failed (attempt %d): %v", retryCount+1, execErr))
	} else {
		log.Printf("✅ Job '%s' completed successfully (attempt %d)", job.Name, retryCount+1)
		LogJobEvent(job.ID, executionID, "info", fmt.Sprintf("Job completed successfully (attempt %d)", retryCount+1))
	}

	// Update execution record
	_, err = config.DB.Exec(`
		UPDATE job_executions
		SET status = $1, completed_at = $2, output = $3, error_message = $4, duration_ms = $5
		WHERE id = $6
	`, status, endTime, output, errorMessage, duration.Milliseconds(), executionID)

	if err != nil {
		log.Printf("❌ Error updating execution record: %v", err)
	}

	// Send notifications for final result (success or final failure)
	isFinalAttempt := retryCount >= job.MaxRetries || status == "success"

	if isFinalAttempt {
		go sendJobNotification(job, status, startTime, endTime, duration, exitCode, output, errorMessage, finalCommand, isTimeout)
	}

	// Handle retries for failed jobs
	if status == "failed" && retryCount < job.MaxRetries {
		log.Printf("🔄 Scheduling retry %d for job '%s' in %d seconds", retryCount+1, job.Name, job.RetryDelaySeconds)

		// Schedule retry
		go func() {
			time.Sleep(time.Duration(job.RetryDelaySeconds) * time.Second)
			ExecuteJobWithRetry(job, retryCount+1, &executionID)
		}()

		// Broadcast retry scheduled event
		utils.BroadcastToClients(models.WSMessage{
			Type: "job_retry_scheduled",
			Payload: models.JobStatusUpdate{
				JobID:       job.ID,
				JobName:     job.Name,
				Status:      "retry_scheduled",
				Message:     fmt.Sprintf("Retry %d scheduled in %d seconds", retryCount+1, job.RetryDelaySeconds),
				Timestamp:   endTime.Format(time.RFC3339),
				ExecutionID: executionID,
				RetryCount:  retryCount + 1,
			},
		})
		return
	}

	// Broadcast job completed event (final result)
	finalStatus := status
	if status == "failed" && retryCount >= job.MaxRetries {
		finalStatus = "failed_final"
	}

	utils.BroadcastToClients(models.WSMessage{
		Type: "job_completed",
		Payload: models.JobStatusUpdate{
			JobID:       job.ID,
			JobName:     job.Name,
			Status:      finalStatus,
			Message:     fmt.Sprintf("Job %s in %dms (attempt %d)", status, duration.Milliseconds(), retryCount+1),
			Timestamp:   endTime.Format(time.RFC3339),
			ExecutionID: executionID,
			RetryCount:  retryCount,
		},
	})

	// Update system stats
	BroadcastSystemStats()

	// Trigger dependent jobs if this job succeeded
	if status == "success" {
		TriggerDependentJobs(job.ID)
	}
	metrics.RunningJobs.Dec()
	metrics.JobExecutionsTotal.WithLabelValues(job.Name, "success").Inc() // or "failed"
	metrics.JobExecutionDuration.WithLabelValues(job.Name).Observe(duration.Seconds())
}

// sendJobNotification sends notifications for job completion
func sendJobNotification(job models.Job, status string, startTime, endTime time.Time, duration time.Duration, exitCode int, output, errorMessage, command string, isTimeout bool) {
	if notificationService == nil {
		return
	}

	durationStr := FormatDuration(duration) // Use the notification service's FormatDuration
	jobID := strconv.Itoa(job.ID)

	// Create job notification data
	jobData := JobNotificationData{
		ID:        jobID,
		Name:      job.Name,
		Status:    status,
		Schedule:  job.CronExpression,
		Duration:  durationStr,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Command:   command,
	}

	// Add output or error details
	if status == "success" {
		jobData.Output = output
	} else {
		jobData.Error = errorMessage
		if len(output) > 0 && jobData.Error != output {
			jobData.Error += fmt.Sprintf("\nOutput: %s", output)
		}
	}

	// Send appropriate notification based on status
	var err error

	if status == "success" {
		// Check for long-running job warning
		if duration > 30*time.Minute {
			warningMsg := fmt.Sprintf("Job took longer than expected: %s", durationStr)
			if warnErr := notificationService.NotifyJobWarning(
				jobID, job.Name, job.CronExpression, durationStr,
				exitCode, startTime, endTime, warningMsg,
			); warnErr != nil {
				log.Printf("⚠️ Failed to send warning notification: %v", warnErr)
			}
		}

		// Send success notification
		err = notificationService.NotifyJobSuccess(
			jobID, job.Name, job.CronExpression, durationStr,
			exitCode, startTime, endTime, output,
		)
		if err != nil {
			log.Printf("⚠️ Failed to send success notification: %v", err)
		}

		// Send generic status notification
		if statusErr := notificationService.NotifyJobStatus(jobData, "success"); statusErr != nil {
			log.Printf("⚠️ Failed to send status notification: %v", statusErr)
		}

	} else {
		// Determine if it's a timeout or regular failure
		if isTimeout {
			err = notificationService.NotifyJobTimeout(
				jobID, job.Name, job.CronExpression, durationStr,
				startTime, endTime, command,
			)
			if err != nil {
				log.Printf("⚠️ Failed to send timeout notification: %v", err)
			}
		} else {
			err = notificationService.NotifyJobFailure(
				jobID, job.Name, job.CronExpression, durationStr,
				exitCode, startTime, endTime, errorMessage, command,
			)
			if err != nil {
				log.Printf("⚠️ Failed to send failure notification: %v", err)
			}
		}

		// Send generic status notification
		eventType := "failure"
		if isTimeout {
			eventType = "timeout"
		}
		if statusErr := notificationService.NotifyJobStatus(jobData, eventType); statusErr != nil {
			log.Printf("⚠️ Failed to send status notification: %v", statusErr)
		}
	}
}

// FormatDurationForDisplay formats a duration into a human-readable string for logs/display
func FormatDurationForDisplay(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// CheckJobDependencies checks if job dependencies are satisfied
func CheckJobDependencies(jobID int) (bool, error) {
	rows, err := config.DB.Query(`
		SELECT jd.depends_on_job_id, jd.dependency_type, j.name
		FROM job_dependencies jd
		JOIN jobs j ON j.id = jd.depends_on_job_id
		WHERE jd.job_id = $1
	`, jobID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var dependsOnJobID int
		var dependencyType, dependsOnJobName string
		if err := rows.Scan(&dependsOnJobID, &dependencyType, &dependsOnJobName); err != nil {
			continue
		}

		// Check the latest execution of the dependency
		var lastStatus sql.NullString
		var lastCompleted sql.NullTime

		err = config.DB.QueryRow(`
			SELECT status, completed_at
			FROM job_executions
			WHERE job_id = $1 AND completed_at IS NOT NULL
			ORDER BY completed_at DESC LIMIT 1
		`, dependsOnJobID).Scan(&lastStatus, &lastCompleted)

		// If no execution found, dependency not satisfied
		if err == sql.ErrNoRows {
			log.Printf("🔒 Job %d waiting: dependency job %s has never been executed", jobID, dependsOnJobName)
			return false, nil
		}

		// Check if dependency requirement is met
		switch dependencyType {
		case "success":
			if !lastStatus.Valid || lastStatus.String != "success" {
				log.Printf("🔒 Job %d waiting: dependency job %s last status was %s, need success", jobID, dependsOnJobName, lastStatus.String)
				return false, nil
			}
		case "completion":
			if !lastCompleted.Valid {
				log.Printf("🔒 Job %d waiting: dependency job %s has not completed", jobID, dependsOnJobName)
				return false, nil
			}
		case "failure":
			if !lastStatus.Valid || lastStatus.String != "failed" {
				log.Printf("🔒 Job %d waiting: dependency job %s last status was %s, need failure", jobID, dependsOnJobName, lastStatus.String)
				return false, nil
			}
		}
	}

	return true, nil
}

// CanJobRun checks if job can run based on concurrent execution limit
func CanJobRun(jobID, maxConcurrent int) bool {
	RunningJobsMux.RLock()
	currentRunning := RunningJobs[jobID]
	RunningJobsMux.RUnlock()

	return currentRunning < maxConcurrent
}

// TrackRunningJob tracks running job count
func TrackRunningJob(jobID int, increment bool) {
	RunningJobsMux.Lock()
	defer RunningJobsMux.Unlock()

	if increment {
		RunningJobs[jobID]++
	} else {
		RunningJobs[jobID]--
		if RunningJobs[jobID] <= 0 {
			delete(RunningJobs, jobID)
		}
	}
}

// TriggerDependentJobs triggers jobs that depend on the completed job
func TriggerDependentJobs(completedJobID int) {
	rows, err := config.DB.Query(`
		SELECT j.id, j.name, j.description, j.cron_expression, j.command, j.is_active,
				j.category, j.max_retries, j.retry_delay_seconds, j.timeout_seconds,
				j.environment_vars, j.tags, j.priority, j.concurrent_executions,
				j.created_at, j.updated_at
		FROM jobs j
		JOIN job_dependencies jd ON jd.job_id = j.id
		WHERE jd.depends_on_job_id = $1 AND jd.dependency_type = 'success' AND j.is_active = true
	`, completedJobID)

	if err != nil {
		log.Printf("❌ Error fetching dependent jobs: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var job models.Job
		var envVarsJSON string

		err := rows.Scan(&job.ID, &job.Name, &job.Description, &job.CronExpression,
			&job.Command, &job.IsActive, &job.Category, &job.MaxRetries,
			&job.RetryDelaySeconds, &job.TimeoutSeconds, &envVarsJSON,
			pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions,
			&job.CreatedAt, &job.UpdatedAt)

		if err != nil {
			log.Printf("❌ Error scanning dependent job: %v", err)
			continue
		}

		// Parse environment variables
		if err := json.Unmarshal([]byte(envVarsJSON), &job.EnvironmentVars); err != nil {
			job.EnvironmentVars = make(map[string]string)
		}

		log.Printf("🎯 Triggering dependent job '%s' after completion of job ID %d", job.Name, completedJobID)
		go ExecuteJob(job)
	}
}

// BroadcastSystemStats broadcasts system statistics
func BroadcastSystemStats() {
	var totalJobs, activeJobs, runningJobsCount, pendingRetries int

	config.DB.QueryRow("SELECT COUNT(*) FROM jobs").Scan(&totalJobs)
	config.DB.QueryRow("SELECT COUNT(*) FROM jobs WHERE is_active = true").Scan(&activeJobs)
	config.DB.QueryRow("SELECT COUNT(*) FROM job_executions WHERE status = 'running'").Scan(&runningJobsCount)
	config.DB.QueryRow(`
		SELECT COUNT(*) FROM job_executions
		WHERE status = 'failed' AND retry_count < (
			SELECT max_retries FROM jobs WHERE jobs.id = job_executions.job_id
		)
	`).Scan(&pendingRetries)

	// Count categories
	categories := make(map[string]int)
	rows, err := config.DB.Query("SELECT category, COUNT(*) FROM jobs GROUP BY category")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var category string
			var count int
			if rows.Scan(&category, &count) == nil {
				categories[category] = count
			}
		}
	}

	stats := models.SystemStats{
		TotalJobs:      totalJobs,
		ActiveJobs:     activeJobs,
		RunningJobs:    runningJobsCount,
		CronEntries:    len(JobEntries),
		PendingRetries: pendingRetries,
		Categories:     categories,
		LastUpdated:    time.Now().Format(time.RFC3339),
	}

	utils.BroadcastToClients(models.WSMessage{
		Type:    "system_stats",
		Payload: stats,
	})
}

// SubstituteEnvVars substitutes environment variables in command
func SubstituteEnvVars(command string, envVars map[string]string) string {
	result := command

	// Add current date/time variables
	envVars["DATE"] = time.Now().Format("2006-01-02")
	envVars["DATETIME"] = time.Now().Format("2006-01-02_15-04-05")
	envVars["TIMESTAMP"] = strconv.FormatInt(time.Now().Unix(), 10)

	// Substitute ${VAR} and $VAR patterns
	for key, value := range envVars {
		result = strings.ReplaceAll(result, "${"+key+"}", value)
		result = strings.ReplaceAll(result, "$"+key, value)
	}

	return result
}

// RunCommandWithContext executes a command with timeout context
func RunCommandWithContext(ctx context.Context, command string) (string, error) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	output, err := cmd.CombinedOutput()

	if ctx.Err() == context.DeadlineExceeded {
		return string(output), fmt.Errorf("command timed out")
	}

	return string(output), err
}

// LogJobEvent logs a job event
func LogJobEvent(jobID, executionID int, level, message string) {
	var query string
	var args []interface{}

	if executionID > 0 {
		query = `INSERT INTO job_logs (job_id, execution_id, log_level, message) VALUES ($1, $2, $3, $4)`
		args = []interface{}{jobID, executionID, level, message}
	} else {
		query = `INSERT INTO job_logs (job_id, log_level, message) VALUES ($1, $2, $3)`
		args = []interface{}{jobID, level, message}
	}

	_, err := config.DB.Exec(query, args...)
	if err != nil {
		log.Printf("❌ Error logging job event: %v", err)
	}
}

// Manual job execution function for API calls
func ExecuteJobManually(jobID int) error {
	// Fetch job details from database
	var job models.Job
	var envVarsJSON string

	err := config.DB.QueryRow(`
		SELECT id, name, cron_expression, command, category, max_retries, retry_delay_seconds,
				timeout_seconds, environment_vars, tags, priority, concurrent_executions
		FROM jobs WHERE id = $1 AND is_active = true
	`, jobID).Scan(&job.ID, &job.Name, &job.CronExpression, &job.Command,
		&job.Category, &job.MaxRetries, &job.RetryDelaySeconds, &job.TimeoutSeconds,
		&envVarsJSON, pq.Array(&job.Tags), &job.Priority, &job.ConcurrentExecutions)

	if err != nil {
		return fmt.Errorf("failed to fetch job: %v", err)
	}

	// Parse environment variables
	if err := json.Unmarshal([]byte(envVarsJSON), &job.EnvironmentVars); err != nil {
		job.EnvironmentVars = make(map[string]string)
	}

	// Execute the job asynchronously
	go ExecuteJob(job)

	return nil
}

// Health check for notification service
func CheckNotificationServiceHealth() error {
	if notificationService == nil {
		return fmt.Errorf("notification service not initialized")
	}
	return notificationService.TestNotificationService()
}
