package models

import "time"

type Job struct {
	ID                   int               `json:"id"`
	Name                 string            `json:"name"`
	Description          string            `json:"description"`
	CronExpression       string            `json:"cron_expression"`
	Command              string            `json:"command"`
	IsActive             bool              `json:"is_active"`
	Category             string            `json:"category"`
	MaxRetries           int               `json:"max_retries"`
	RetryDelaySeconds    int               `json:"retry_delay_seconds"`
	TimeoutSeconds       int               `json:"timeout_seconds"`
	EnvironmentVars      map[string]string `json:"environment_vars"`
	Tags                 []string          `json:"tags"`
	Priority             int               `json:"priority"`
	ConcurrentExecutions int               `json:"concurrent_executions"`
	CreatedAt            time.Time         `json:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at"`
	Dependencies         []JobDependency   `json:"dependencies,omitempty"`
}

type JobDependency struct {
	ID               int       `json:"id"`
	JobID            int       `json:"job_id"`
	DependsOnJobID   int       `json:"depends_on_job_id"`
	DependsOnJobName string    `json:"depends_on_job_name,omitempty"`
	DependencyType   string    `json:"dependency_type"`
	CreatedAt        time.Time `json:"created_at"`
}

// JobTemplate represents a reusable job template
type JobTemplate struct {
	ID                     int               `json:"id"`
	Name                   string            `json:"name"`
	Description            string            `json:"description"`
	Category               string            `json:"category"`
	CommandTemplate        string            `json:"command_template"`
	DefaultCronExpression  string            `json:"default_cron_expression"`
	DefaultEnvironmentVars map[string]string `json:"default_environment_vars"`
	DefaultMaxRetries      int               `json:"default_max_retries"`
	DefaultTimeoutSeconds  int               `json:"default_timeout_seconds"`
	Tags                   []string          `json:"tags"`
	CreatedAt              time.Time         `json:"created_at"`
	UpdatedAt              time.Time         `json:"updated_at"`
}

// JobExecution represents a job execution record with retry info
type JobExecution struct {
	ID                int        `json:"id"`
	JobID             int        `json:"job_id"`
	Status            string     `json:"status"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	Output            string     `json:"output,omitempty"`
	ErrorMessage      string     `json:"error_message,omitempty"`
	DurationMs        int        `json:"duration_ms,omitempty"`
	RetryCount        int        `json:"retry_count"`
	IsRetry           bool       `json:"is_retry"`
	ParentExecutionID *int       `json:"parent_execution_id,omitempty"`
}

// WebSocket message types
type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

// JobStatusUpdate represents a job status change for WebSocket broadcasts
type JobStatusUpdate struct {
	JobID       int    `json:"job_id"`
	JobName     string `json:"job_name"`
	Status      string `json:"status"`
	Message     string `json:"message"`
	Timestamp   string `json:"timestamp"`
	ExecutionID int    `json:"execution_id,omitempty"`
	RetryCount  int    `json:"retry_count,omitempty"`
}

// SystemStats represents system-wide statistics
type SystemStats struct {
	TotalJobs      int            `json:"total_jobs"`
	ActiveJobs     int            `json:"active_jobs"`
	RunningJobs    int            `json:"running_jobs"`
	CronEntries    int            `json:"cron_entries"`
	PendingRetries int            `json:"pending_retries"`
	Categories     map[string]int `json:"categories"`
	LastUpdated    string         `json:"last_updated"`
}
