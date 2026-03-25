package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type WorkflowStep struct {
	ID          int                    `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Config      map[string]interface{} `json:"config"`
}

type Workflow struct {
	ID          int            `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      string         `json:"status"`
	Schedule    string         `json:"schedule"`
	Steps       []WorkflowStep `json:"steps"`
	LastRun     *time.Time     `json:"last_run,omitempty"`
	NextRun     *time.Time     `json:"next_run,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

var workflows = make(map[int]*Workflow)
var workflowCounter = 1

func SetupWorkflowRoutes() {
	// Serve the workflow builder page
	http.HandleFunc("/workflow-builder", workflowBuilderHandler)

	// API endpoints
	http.HandleFunc("/api/workflows", workflowsHandler)
	http.HandleFunc("/api/workflows/", workflowHandler)
	http.HandleFunc("/api/workflows/validate/", validateWorkflowHandler)
	http.HandleFunc("/api/workflows/execute/", executeWorkflowHandler)
}

// Serve the workflow builder HTML page
func workflowBuilderHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "templates/workflow-builder.html")
}

// Handle workflows collection
func workflowsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getWorkflows(w, r)
	case "POST":
		createWorkflow(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Handle individual workflow
func workflowHandler(w http.ResponseWriter, r *http.Request) {
	// Extract workflow ID from URL
	idStr := r.URL.Path[len("/api/workflows/"):]
	if idStr == "" {
		http.Error(w, "Workflow ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		getWorkflow(w, r, id)
	case "PUT":
		updateWorkflow(w, r, id)
	case "DELETE":
		deleteWorkflow(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// Get all workflows
func getWorkflows(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	workflowList := make([]*Workflow, 0, len(workflows))
	for _, workflow := range workflows {
		workflowList = append(workflowList, workflow)
	}

	json.NewEncoder(w).Encode(workflowList)
}

// Get single workflow
func getWorkflow(w http.ResponseWriter, r *http.Request, id int) {
	w.Header().Set("Content-Type", "application/json")

	workflow, exists := workflows[id]
	if !exists {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(workflow)
}

// Create new workflow
func createWorkflow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var workflow Workflow
	if err := json.NewDecoder(r.Body).Decode(&workflow); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	workflow.ID = workflowCounter
	workflowCounter++
	workflow.CreatedAt = time.Now()
	workflow.UpdatedAt = time.Now()

	if workflow.Status == "" {
		workflow.Status = "active"
	}
	if workflow.Schedule == "" {
		workflow.Schedule = "0 0 * * *"
	}

	// Store workflow
	workflows[workflow.ID] = &workflow

	// Return created workflow
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(workflow)
}

// Update existing workflow
func updateWorkflow(w http.ResponseWriter, r *http.Request, id int) {
	w.Header().Set("Content-Type", "application/json")

	workflow, exists := workflows[id]
	if !exists {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	var updates Workflow
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Update workflow fields
	workflow.Name = updates.Name
	workflow.Description = updates.Description
	workflow.Status = updates.Status
	workflow.Schedule = updates.Schedule
	workflow.Steps = updates.Steps
	workflow.UpdatedAt = time.Now()

	json.NewEncoder(w).Encode(workflow)
}

// Delete workflow
func deleteWorkflow(w http.ResponseWriter, r *http.Request, id int) {
	_, exists := workflows[id]
	if !exists {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	delete(workflows, id)
	w.WriteHeader(http.StatusNoContent)
}

// Validate workflow
func validateWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract workflow ID from URL
	idStr := r.URL.Path[len("/api/workflows/validate/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	workflow, exists := workflows[id]
	if !exists {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	// Validate workflow
	errors := validateWorkflow(workflow)

	response := map[string]interface{}{
		"valid":  len(errors) == 0,
		"errors": errors,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Validate workflow logic
func validateWorkflow(workflow *Workflow) []string {
	var errors []string

	if workflow.Name == "" {
		errors = append(errors, "Workflow name is required")
	}

	if workflow.Schedule == "" {
		errors = append(errors, "Workflow schedule is required")
	}

	if len(workflow.Steps) == 0 {
		errors = append(errors, "Workflow must have at least one step")
	}

	// Validate each step
	for i, step := range workflow.Steps {
		stepErrors := validateStep(step, i+1)
		errors = append(errors, stepErrors...)
	}

	return errors
}

// Validate individual step
func validateStep(step WorkflowStep, stepNum int) []string {
	var errors []string

	if step.Name == "" {
		errors = append(errors, fmt.Sprintf("Step %d: Name is required", stepNum))
	}

	if step.Type == "" {
		errors = append(errors, fmt.Sprintf("Step %d: Type is required", stepNum))
	}

	// Type-specific validation
	switch step.Type {
	case "database":
		if host, ok := step.Config["host"].(string); !ok || host == "" {
			errors = append(errors, fmt.Sprintf("Step %d: Database host is required", stepNum))
		}
	case "email":
		if to, ok := step.Config["to"].(string); !ok || to == "" {
			errors = append(errors, fmt.Sprintf("Step %d: Email recipient is required", stepNum))
		}
	case "script":
		if script, ok := step.Config["script"].(string); !ok || script == "" {
			errors = append(errors, fmt.Sprintf("Step %d: Script path is required", stepNum))
		}
	case "api":
		if url, ok := step.Config["url"].(string); !ok || url == "" {
			errors = append(errors, fmt.Sprintf("Step %d: API URL is required", stepNum))
		}
	}

	return errors
}

// Execute workflow
func executeWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract workflow ID from URL
	idStr := r.URL.Path[len("/api/workflows/execute/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid workflow ID", http.StatusBadRequest)
		return
	}

	workflow, exists := workflows[id]
	if !exists {
		http.Error(w, "Workflow not found", http.StatusNotFound)
		return
	}

	// Execute workflow (implement your execution logic here)
	result := executeWorkflow(workflow)

	// Update last run time
	now := time.Now()
	workflow.LastRun = &now

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// Workflow execution logic (stub - implement based on your needs)
func executeWorkflow(workflow *Workflow) map[string]interface{} {
	// This is a simplified execution - implement your actual logic here
	results := make([]map[string]interface{}, 0, len(workflow.Steps))

	for i, step := range workflow.Steps {
		stepResult := map[string]interface{}{
			"step_number":  i + 1,
			"step_id":      step.ID,
			"step_name":    step.Name,
			"status":       "success",
			"started_at":   time.Now().Format(time.RFC3339),
			"completed_at": time.Now().Add(time.Second * 2).Format(time.RFC3339),
			"output":       executeStep(step),
		}

		results = append(results, stepResult)
	}

	return map[string]interface{}{
		"workflow_id":  workflow.ID,
		"status":       "completed",
		"started_at":   time.Now().Format(time.RFC3339),
		"completed_at": time.Now().Add(time.Minute).Format(time.RFC3339),
		"steps":        results,
	}
}

// Execute individual step (implement based on step type)
func executeStep(step WorkflowStep) string {
	switch step.Type {
	case "database":
		return executeDatabase(step)
	case "email":
		return executeEmail(step)
	case "script":
		return executeScript(step)
	case "backup":
		return executeBackup(step)
	case "api":
		return executeAPI(step)
	case "condition":
		return executeCondition(step)
	default:
		return "Unknown step type"
	}
}

func executeDatabase(step WorkflowStep) string {
	host, _ := step.Config["host"].(string)
	port, _ := step.Config["port"].(float64)
	database, _ := step.Config["database"].(string)

	return fmt.Sprintf("Connected to %s:%v/%s and executed query", host, int(port), database)
}

func executeEmail(step WorkflowStep) string {
	to, _ := step.Config["to"].(string)
	subject, _ := step.Config["subject"].(string)

	return fmt.Sprintf("Email sent to %s with subject: %s", to, subject)
}

func executeScript(step WorkflowStep) string {
	script, _ := step.Config["script"].(string)
	parameters, _ := step.Config["parameters"].(string)

	return fmt.Sprintf("Executed script %s with parameters: %s", script, parameters)
}

func executeBackup(step WorkflowStep) string {
	source, _ := step.Config["source"].(string)
	destination, _ := step.Config["destination"].(string)

	return fmt.Sprintf("Backup created from %s to %s", source, destination)
}

func executeAPI(step WorkflowStep) string {
	url, _ := step.Config["url"].(string)
	method, _ := step.Config["method"].(string)

	return fmt.Sprintf("API call made: %s %s", method, url)
}

func executeCondition(step WorkflowStep) string {
	condition, _ := step.Config["condition"].(string)

	return fmt.Sprintf("Condition evaluated: %s", condition)
}

func InitSampleWorkflows() {
	now := time.Now()
	nextRun := now.Add(24 * time.Hour)

	workflows[1] = &Workflow{
		ID:          1,
		Name:        "Database Backup",
		Description: "Daily backup of production database",
		Status:      "active",
		Schedule:    "0 2 * * *",
		Steps: []WorkflowStep{
			{
				ID:          1,
				Type:        "database",
				Name:        "Connect to DB",
				Description: "Establish database connection",
				Config: map[string]interface{}{
					"host":     "prod-db",
					"port":     5432,
					"database": "main",
				},
			},
			{
				ID:          2,
				Type:        "backup",
				Name:        "Create Backup",
				Description: "Generate database backup",
				Config: map[string]interface{}{
					"format":      "sql",
					"compress":    true,
					"includeData": true,
				},
			},
			{
				ID:          3,
				Type:        "api",
				Name:        "Upload to S3",
				Description: "Store backup in cloud storage",
				Config: map[string]interface{}{
					"bucket": "backups",
					"region": "us-east-1",
				},
			},
			{
				ID:          4,
				Type:        "email",
				Name:        "Send Notification",
				Description: "Notify admin of backup completion",
				Config: map[string]interface{}{
					"to":      "admin@company.com",
					"subject": "Backup Complete",
				},
			},
		},
		LastRun:   &now,
		NextRun:   &nextRun,
		CreatedAt: now,
		UpdatedAt: now,
	}

	workflows[2] = &Workflow{
		ID:          2,
		Name:        "Report Generation",
		Description: "Weekly sales report generation",
		Status:      "active",
		Schedule:    "0 9 * * 1",
		Steps: []WorkflowStep{
			{
				ID:          1,
				Type:        "database",
				Name:        "Query Sales Data",
				Description: "Extract sales data from database",
				Config: map[string]interface{}{
					"query":      "SELECT * FROM sales WHERE date >= ?",
					"parameters": "last_week",
				},
			},
			{
				ID:          2,
				Type:        "script",
				Name:        "Generate Charts",
				Description: "Create visual charts from data",
				Config: map[string]interface{}{
					"script":       "generate_charts.py",
					"outputFormat": "png",
				},
			},
			{
				ID:          3,
				Type:        "email",
				Name:        "Email Report",
				Description: "Send report to stakeholders",
				Config: map[string]interface{}{
					"to":          "sales@company.com",
					"attachments": true,
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	workflowCounter = 3
}
