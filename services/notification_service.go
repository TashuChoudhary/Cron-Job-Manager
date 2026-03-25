package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

type NotificationService struct {
	BaseURL    string
	HTTPClient *http.Client
}

type JobNotificationData struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Schedule  string    `json:"schedule"`
	Duration  string    `json:"duration"`
	ExitCode  int       `json:"exitCode"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Error     string    `json:"error,omitempty"`
	Output    string    `json:"output,omitempty"`
	Command   string    `json:"command,omitempty"`
}

// NotificationRequest represents the request payload to notification service
type NotificationRequest struct {
	JobData   JobNotificationData `json:"jobData"`
	EventType string              `json:"eventType"`
}

type NotificationResponse struct {
	Success        bool                 `json:"success"`
	Message        string               `json:"message"`
	JobID          string               `json:"jobId"`
	JobName        string               `json:"jobName"`
	EventType      string               `json:"eventType"`
	RulesProcessed int                  `json:"rulesProcessed"`
	Results        []NotificationResult `json:"results"`
}

// NotificationResult represents individual notification result
type NotificationResult struct {
	RuleID        int64           `json:"ruleId"`
	RuleName      string          `json:"ruleName"`
	Triggered     bool            `json:"triggered"`
	Reason        string          `json:"reason,omitempty"`
	Error         string          `json:"error,omitempty"`
	Notifications []ChannelResult `json:"notifications,omitempty"`
}

// ChannelResult represents individual channel notification result
type ChannelResult struct {
	Channel   string `json:"channel"`
	Recipient string `json:"recipient"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// NewNotificationService creates a new notification service instance
func NewNotificationService() *NotificationService {
	baseURL := os.Getenv("NOTIFICATION_SERVICE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:3001" // Default notification service URL
	}

	return &NotificationService{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NotifyJobStatus sends job status notification to the notification service
func (ns *NotificationService) NotifyJobStatus(jobData JobNotificationData, eventType string) error {
	// Prepare the notification request
	request := NotificationRequest{
		JobData:   jobData,
		EventType: eventType,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal notification request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/api/notifications/process-job-event", ns.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "CronJobManager-Go/1.0")

	// Send request
	resp, err := ns.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send notification request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notification service returned status %d", resp.StatusCode)
	}

	// Parse response
	var notificationResp NotificationResponse
	if err := json.NewDecoder(resp.Body).Decode(&notificationResp); err != nil {
		log.Printf("Warning: Failed to parse notification response: %v", err)
		// Don't return error here as the notification was likely sent
		return nil
	}

	// Log the results
	if notificationResp.Success {
		log.Printf("✅ Notifications processed for job '%s': %d rules processed",
			notificationResp.JobName, notificationResp.RulesProcessed)

		// Log individual results
		for _, result := range notificationResp.Results {
			if result.Triggered {
				log.Printf("  📧 Rule '%s' triggered: %d notifications sent",
					result.RuleName, len(result.Notifications))
			} else {
				log.Printf("  ⏭️ Rule '%s' skipped: %s", result.RuleName, result.Reason)
			}
		}
	} else {
		log.Printf("⚠️ Notification processing failed for job '%s': %s",
			jobData.Name, notificationResp.Message)
	}

	return nil
}

// NotifyJobSuccess is a convenience method for successful job completions
func (ns *NotificationService) NotifyJobSuccess(jobID, jobName, schedule, duration string, exitCode int, startTime, endTime time.Time, output string) error {
	jobData := JobNotificationData{
		ID:        jobID,
		Name:      jobName,
		Status:    "success",
		Schedule:  schedule,
		Duration:  duration,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Output:    output,
	}

	return ns.NotifyJobStatus(jobData, "success")
}

// NotifyJobFailure is a convenience method for failed job completions
func (ns *NotificationService) NotifyJobFailure(jobID, jobName, schedule, duration string, exitCode int, startTime, endTime time.Time, errorMsg, command string) error {
	jobData := JobNotificationData{
		ID:        jobID,
		Name:      jobName,
		Status:    "failed",
		Schedule:  schedule,
		Duration:  duration,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Error:     errorMsg,
		Command:   command,
	}

	return ns.NotifyJobStatus(jobData, "failure")
}

// NotifyJobWarning is a convenience method for job warnings (long duration, etc.)
func (ns *NotificationService) NotifyJobWarning(jobID, jobName, schedule, duration string, exitCode int, startTime, endTime time.Time, warningMsg string) error {
	jobData := JobNotificationData{
		ID:        jobID,
		Name:      jobName,
		Status:    "warning",
		Schedule:  schedule,
		Duration:  duration,
		ExitCode:  exitCode,
		StartTime: startTime,
		EndTime:   endTime,
		Error:     warningMsg,
	}

	return ns.NotifyJobStatus(jobData, "warning")
}

// NotifyJobTimeout is a convenience method for job timeouts
func (ns *NotificationService) NotifyJobTimeout(jobID, jobName, schedule, duration string, startTime, endTime time.Time, command string) error {
	jobData := JobNotificationData{
		ID:        jobID,
		Name:      jobName,
		Status:    "failed",
		Schedule:  schedule,
		Duration:  duration,
		ExitCode:  124, // Standard timeout exit code
		StartTime: startTime,
		EndTime:   endTime,
		Error:     "Job execution timed out",
		Command:   command,
	}

	return ns.NotifyJobStatus(jobData, "timeout")
}

// TestNotificationService tests the connection to notification service
func (ns *NotificationService) TestNotificationService() error {
	url := fmt.Sprintf("%s/api/notifications/health", ns.BaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := ns.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to reach notification service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("notification service health check failed with status %d", resp.StatusCode)
	}

	log.Printf("✅ Notification service is healthy")
	return nil
}

// FormatDuration converts duration to string format expected by notification service
func FormatDuration(duration time.Duration) string {
	totalSeconds := int(duration.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// Helper function to generate job ID if not provided
func GenerateJobID(jobName string) string {
	return fmt.Sprintf("job_%s_%d", jobName, time.Now().Unix())
}

// Async notification - fire and forget (recommended for non-critical notifications)
func (ns *NotificationService) NotifyJobStatusAsync(jobData JobNotificationData, eventType string) {
	go func() {
		if err := ns.NotifyJobStatus(jobData, eventType); err != nil {
			log.Printf("🔔 Async notification failed for job '%s': %v", jobData.Name, err)
		}
	}()
}

// Batch notification for multiple jobs (useful for daily reports, etc.)
func (ns *NotificationService) NotifyBatchJobs(jobs []JobNotificationData, eventType string) error {
	for _, job := range jobs {
		if err := ns.NotifyJobStatus(job, eventType); err != nil {
			log.Printf("⚠️ Batch notification failed for job '%s': %v", job.Name, err)
			// Continue with other jobs even if one fails
		}

		// Small delay between batch notifications to avoid overwhelming the service
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// Global notification service instance (singleton pattern)
var globalNotificationService *NotificationService

// GetNotificationService returns the global notification service instance
func GetNotificationService() *NotificationService {
	if globalNotificationService == nil {
		globalNotificationService = NewNotificationService()
	}
	return globalNotificationService
}

// Initialize notification service and test connection on startup
func InitNotificationService() error {
	ns := GetNotificationService()

	// Test the connection
	if err := ns.TestNotificationService(); err != nil {
		log.Printf("⚠️ Warning: Notification service connection failed: %v", err)
		log.Printf("🔔 Notifications will be disabled until service is available")
		return err
	}

	log.Printf("🔔 Notification service initialized successfully")
	return nil
}
