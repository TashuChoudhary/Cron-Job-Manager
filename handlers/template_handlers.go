package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"cron-job-manager/config"
	"cron-job-manager/models"
	"cron-job-manager/services"

	"github.com/gorilla/mux"
	"github.com/lib/pq"
)

// GetJobTemplatesHandler retrieves all job templates
func GetJobTemplatesHandler(w http.ResponseWriter, r *http.Request) {
	rows, err := config.DB.Query(`
		SELECT id, name, description, category, command_template, default_cron_expression,
			   default_environment_vars, default_max_retries, default_timeout_seconds, tags,
			   created_at, updated_at
		FROM job_templates ORDER BY category, name
	`)
	if err != nil {
		http.Error(w, "Error fetching templates", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var templates []models.JobTemplate
	for rows.Next() {
		var template models.JobTemplate
		var envVarsJSON string

		err := rows.Scan(&template.ID, &template.Name, &template.Description, &template.Category,
			&template.CommandTemplate, &template.DefaultCronExpression, &envVarsJSON,
			&template.DefaultMaxRetries, &template.DefaultTimeoutSeconds, pq.Array(&template.Tags),
			&template.CreatedAt, &template.UpdatedAt)

		if err != nil {
			continue
		}

		json.Unmarshal([]byte(envVarsJSON), &template.DefaultEnvironmentVars)
		templates = append(templates, template)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(templates)
}

// CreateJobFromTemplateHandler creates a job from a template
func CreateJobFromTemplateHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	// Get template
	var template models.JobTemplate
	var envVarsJSON string
	err = config.DB.QueryRow(`
		SELECT name, description, category, command_template, default_cron_expression,
			   default_environment_vars, default_max_retries, default_timeout_seconds, tags
		FROM job_templates WHERE id = $1
	`, templateID).Scan(&template.Name, &template.Description, &template.Category,
		&template.CommandTemplate, &template.DefaultCronExpression, &envVarsJSON,
		&template.DefaultMaxRetries, &template.DefaultTimeoutSeconds, pq.Array(&template.Tags))

	if err == sql.ErrNoRows {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "Error fetching template", http.StatusInternalServerError)
		return
	}

	json.Unmarshal([]byte(envVarsJSON), &template.DefaultEnvironmentVars)

	// Parse request body for customizations
	var customization struct {
		Name            string            `json:"name"`
		Description     string            `json:"description,omitempty"`
		CronExpression  string            `json:"cron_expression,omitempty"`
		EnvironmentVars map[string]string `json:"environment_vars,omitempty"`
		IsActive        bool              `json:"is_active"`
	}

	json.NewDecoder(r.Body).Decode(&customization)

	if customization.Name == "" {
		http.Error(w, "Job name is required", http.StatusBadRequest)
		return
	}

	// Create job from template
	job := models.Job{
		Name:                 customization.Name,
		Description:          template.Description,
		CronExpression:       template.DefaultCronExpression,
		Command:              template.CommandTemplate,
		IsActive:             customization.IsActive,
		Category:             template.Category,
		MaxRetries:           template.DefaultMaxRetries,
		RetryDelaySeconds:    60,
		TimeoutSeconds:       template.DefaultTimeoutSeconds,
		EnvironmentVars:      template.DefaultEnvironmentVars,
		Tags:                 template.Tags,
		Priority:             5,
		ConcurrentExecutions: 1,
	}

	// Apply customizations
	if customization.Description != "" {
		job.Description = customization.Description
	}
	if customization.CronExpression != "" {
		job.CronExpression = customization.CronExpression
	}
	if customization.EnvironmentVars != nil {
		for k, v := range customization.EnvironmentVars {
			job.EnvironmentVars[k] = v
		}
	}

	// Create the job using the service
	err = services.CreateJob(&job)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, "Job name already exists", http.StatusConflict)
		} else {
			http.Error(w, "Error creating job", http.StatusInternalServerError)
		}
		return
	}

	if job.IsActive {
		services.ScheduleJob(job)
	}

	// Broadcast template job creation
	services.BroadcastSystemStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(job)
}
