package models

import "time"

type ExecutionLog struct {
	ID         int        `json:"id"`
	JobID      int        `json:"job_id"`
	JobName    string     `json:"job_name"`
	Status     string     `json:"status"` // "success", "failed", "running"
	StartTime  time.Time  `json:"start_time"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	Duration   float64    `json:"duration"` // in seconds
	Output     string     `json:"output"`
	ErrorMsg   string     `json:"error_msg,omitempty"`
	ExitCode   int        `json:"exit_code"`
	RetryCount int        `json:"retry_count"`
	CreatedAt  time.Time  `json:"created_at"`
}

type ExecutionStats struct {
	TotalExecutions int        `json:"total_executions"`
	SuccessCount    int        `json:"success_count"`
	FailureCount    int        `json:"failure_count"`
	SuccessRate     float64    `json:"success_rate"`
	AvgDuration     float64    `json:"avg_duration"`
	LastExecution   *time.Time `json:"last_execution,omitempty"`
}

type LogFilter struct {
	JobID     *int       `json:"job_id,omitempty"`
	Status    string     `json:"status,omitempty"`
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}
