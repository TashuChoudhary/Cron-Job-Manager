package services

import (
	"cron-job-manager/config"
	"cron-job-manager/models"
	"database/sql"
	"fmt"
	"log"
	"time"
)

// CreateExecutionLog creates a new execution log entry
func CreateExecutionLog(jobID int, jobName string) (int, error) {
	var logID int
	err := config.DB.QueryRow(`
		INSERT INTO execution_logs (job_id, job_name, status, start_time, created_at)
		VALUES ($1, $2, 'running', $3, $4)
		RETURNING id
	`, jobID, jobName, time.Now(), time.Now()).Scan(&logID)

	if err != nil {
		return 0, fmt.Errorf("failed to create execution log: %v", err)
	}

	return logID, nil
}

// UpdateExecutionLog updates an execution log when job completes
func UpdateExecutionLog(logID int, status string, output string, errorMsg string, exitCode int, duration float64) error {
	_, err := config.DB.Exec(`
		UPDATE execution_logs 
		SET status = $1, end_time = $2, output = $3, error_msg = $4, exit_code = $5, duration = $6
		WHERE id = $7
	`, status, time.Now(), output, errorMsg, exitCode, duration, logID)

	if err != nil {
		return fmt.Errorf("failed to update execution log: %v", err)
	}

	return nil
}

// GetExecutionLogs retrieves execution logs with filters
func GetExecutionLogs(filter models.LogFilter) ([]models.ExecutionLog, error) {
	query := `
		SELECT id, job_id, job_name, status, start_time, end_time, 
		       duration, output, error_msg, exit_code, retry_count, created_at
		FROM execution_logs
		WHERE 1=1
	`
	args := []interface{}{}
	paramCount := 1

	if filter.JobID != nil {
		query += fmt.Sprintf(" AND job_id = $%d", paramCount)
		args = append(args, *filter.JobID)
		paramCount++
	}

	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", paramCount)
		args = append(args, filter.Status)
		paramCount++
	}

	if filter.StartDate != nil {
		query += fmt.Sprintf(" AND start_time >= $%d", paramCount)
		args = append(args, *filter.StartDate)
		paramCount++
	}

	if filter.EndDate != nil {
		query += fmt.Sprintf(" AND start_time <= $%d", paramCount)
		args = append(args, *filter.EndDate)
		paramCount++
	}

	query += " ORDER BY start_time DESC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", paramCount)
		args = append(args, filter.Limit)
		paramCount++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", paramCount)
		args = append(args, filter.Offset)
	}

	rows, err := config.DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query execution logs: %v", err)
	}
	defer rows.Close()

	var logs []models.ExecutionLog
	for rows.Next() {
		var log models.ExecutionLog
		var endTime sql.NullTime

		err := rows.Scan(
			&log.ID, &log.JobID, &log.JobName, &log.Status,
			&log.StartTime, &endTime, &log.Duration, &log.Output,
			&log.ErrorMsg, &log.ExitCode, &log.RetryCount, &log.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan execution log: %v", err)
		}

		if endTime.Valid {
			log.EndTime = &endTime.Time
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetExecutionLogByID retrieves a single execution log
func GetExecutionLogByID(id int) (*models.ExecutionLog, error) {
	var log models.ExecutionLog
	var endTime sql.NullTime

	err := config.DB.QueryRow(`
		SELECT id, job_id, job_name, status, start_time, end_time, 
		       duration, output, error_msg, exit_code, retry_count, created_at
		FROM execution_logs
		WHERE id = $1
	`, id).Scan(
		&log.ID, &log.JobID, &log.JobName, &log.Status,
		&log.StartTime, &endTime, &log.Duration, &log.Output,
		&log.ErrorMsg, &log.ExitCode, &log.RetryCount, &log.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("execution log not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get execution log: %v", err)
	}

	if endTime.Valid {
		log.EndTime = &endTime.Time
	}

	return &log, nil
}

// GetExecutionStats retrieves execution statistics for a job
func GetExecutionStats(jobID int) (*models.ExecutionStats, error) {
	var stats models.ExecutionStats
	var lastExec sql.NullTime

	err := config.DB.QueryRow(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failure_count,
			AVG(CASE WHEN status = 'success' THEN duration ELSE NULL END) as avg_duration,
			MAX(start_time) as last_execution
		FROM execution_logs
		WHERE job_id = $1
	`, jobID).Scan(
		&stats.TotalExecutions,
		&stats.SuccessCount,
		&stats.FailureCount,
		&stats.AvgDuration,
		&lastExec,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get execution stats: %v", err)
	}

	if stats.TotalExecutions > 0 {
		stats.SuccessRate = (float64(stats.SuccessCount) / float64(stats.TotalExecutions)) * 100
	}

	if lastExec.Valid {
		stats.LastExecution = &lastExec.Time
	}

	return &stats, nil
}

// DeleteOldExecutionLogs deletes logs older than specified days
func DeleteOldExecutionLogs(daysToKeep int) error {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep)

	result, err := config.DB.Exec(`
		DELETE FROM execution_logs 
		WHERE created_at < $1
	`, cutoffDate)

	if err != nil {
		return fmt.Errorf("failed to delete old logs: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	log.Printf("🗑️ Deleted %d old execution logs (older than %d days)", rowsAffected, daysToKeep)

	return nil
}

// GetRecentExecutionLogs gets the most recent N logs across all jobs
func GetRecentExecutionLogs(limit int) ([]models.ExecutionLog, error) {
	return GetExecutionLogs(models.LogFilter{
		Limit: limit,
	})
}
