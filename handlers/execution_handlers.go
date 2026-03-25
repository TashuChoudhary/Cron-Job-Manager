package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cron-job-manager/models"
	"cron-job-manager/services"

	"github.com/gorilla/mux"
)

// GetExecutionLogsHandler handles GET /api/v1/logs
func GetExecutionLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse query parameters
	filter := models.LogFilter{
		Limit: 100, // Default limit
	}

	if jobIDStr := r.URL.Query().Get("job_id"); jobIDStr != "" {
		jobID, err := strconv.Atoi(jobIDStr)
		if err == nil {
			filter.JobID = &jobID
		}
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = status
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	if startDateStr := r.URL.Query().Get("start_date"); startDateStr != "" {
		if startDate, err := time.Parse(time.RFC3339, startDateStr); err == nil {
			filter.StartDate = &startDate
		}
	}

	if endDateStr := r.URL.Query().Get("end_date"); endDateStr != "" {
		if endDate, err := time.Parse(time.RFC3339, endDateStr); err == nil {
			filter.EndDate = &endDate
		}
	}

	logs, err := services.GetExecutionLogs(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logs)
}

// GetExecutionLogByIDHandler handles GET /api/v1/logs/{id}
func GetExecutionLogByIDHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid log ID", http.StatusBadRequest)
		return
	}

	log, err := services.GetExecutionLogByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(log)
}

// GetExecutionStatsHandler handles GET /api/v1/jobs/{id}/stats
func GetExecutionStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	jobID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid job ID", http.StatusBadRequest)
		return
	}

	stats, err := services.GetExecutionStats(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(stats)
}

// GetRecentLogsHandler handles GET /api/v1/logs/recent
func GetRecentLogsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	limit := 50 // Default
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	logs, err := services.GetRecentExecutionLogs(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(logs)
}
