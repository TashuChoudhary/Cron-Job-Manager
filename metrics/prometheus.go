package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	JobsTotal = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "cronjob_jobs_total",
		Help: "Total number of jobs by status",
	}, []string{"status"})

	JobExecutionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cronjob_executions_total",
		Help: "Total job executions",
	}, []string{"job_name", "status"})

	JobExecutionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "cronjob_execution_duration_seconds",
		Help:    "Job execution duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"job_name"})

	ActiveJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cronjob_active_jobs",
		Help: "Number of currently active (enabled) jobs",
	})

	RunningJobs = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "cronjob_running_jobs",
		Help: "Number of jobs currently executing",
	})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cronjob_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})
)

func Handler() http.Handler {
	return promhttp.Handler()
}

func BasicAuthHandler(username, password string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != username || p != password {
			w.Header().Set("WWW-Authenticate", `Basic realm="metrics"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	})
}
