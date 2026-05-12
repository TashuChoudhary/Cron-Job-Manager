# 🕐 CronJob Manager

A **production-ready Cron Job Management System** built with Go, PostgreSQL, WebSockets, and a JavaScript/HTML/CSS frontend. Features role-based authentication, job dependencies, retry logic, email notifications, real-time updates, and full observability via Prometheus metrics on a Grafana Cloud dashboard — all containerized with Docker and deployed on Render.

🔗 **Live Demo:** [https://cron-job-manager.onrender.com](https://cron-job-manager.onrender.com)
📦 **Repo:** [github.com/TashuChoudhary/Cron-Job-Manager](https://github.com/TashuChoudhary/Cron-Job-Manager)

---

## 📌 What It Does

CronJob Manager is a full-featured job scheduling platform. You manage scheduled jobs through a browser dashboard that gets live updates over WebSockets. Under the hood, a Go scheduling engine handles execution, retries, and dependency chains — persisting everything to PostgreSQL. The system emits Prometheus metrics for observability and sends email notifications on job events.

**Core capabilities:**
- Create, update, delete, and manually trigger scheduled cron jobs
- Real-time job status updates pushed to the browser via WebSockets
- Job dependency chains — jobs can depend on the success of other jobs
- Automatic retry logic on failure
- Job templates for quick creation of common job patterns
- Full execution history with per-job logs and stats
- Email notifications (SMTP) on job completion or failure
- Role-based access control: `admin` and `user` roles with separate permissions
- Prometheus metrics at `/metrics` protected by HTTP Basic Auth
- Live Grafana Cloud dashboard showing health, success rates, and latency

---

## 🛠️ Tech Stack

| Layer | Technology |
|---|---|
| **Backend** | Go (Gorilla Mux router) |
| **Frontend** | JavaScript, HTML, CSS |
| **Database** | PostgreSQL 15 |
| **Real-time** | WebSockets |
| **Metrics** | Prometheus |
| **Dashboards** | Grafana Cloud |
| **Notifications** | SMTP Email |
| **Containerization** | Docker, Docker Compose |
| **Build System** | Makefile |
| **Deployment** | Render |

---

## 🏗️ Architecture

```
┌────────────────────────────────────────────────────────────────┐
│                     Browser (JS/HTML/CSS)                      │
│              REST API calls  +  WebSocket live updates         │
└───────────────────────────┬────────────────────────────────────┘
                            │
                            ▼
┌────────────────────────────────────────────────────────────────┐
│                        Go Backend                              │
│                                                                │
│   ┌────────────┐    ┌────────────┐    ┌────────────┐           │
│   │  handlers/ │───▶│  services/ │───▶│   models/  │           │
│   │ (HTTP + WS)│    │ (scheduler │    │ (DB schema)│           │
│   └────────────┘    │  retries,  │    └─────┬──────┘           │
│                     │  notifs,   │          │                  │
│   ┌────────────┐    │  deps)     │          ▼                  │
│   │middleware/ │    └─────┬──────┘    ┌──────────────┐         │
│   │(Auth/RBAC) │          │           │  PostgreSQL  │         │
│   └────────────┘          │           └──────────────┘         │
│                           │                                    │
│   ┌────────────┐    ┌─────▼───── ─┐                            │
│   │  metrics/  │    │notifications│                            │
│   │(Prometheus)│    │  (SMTP)     │                            │
│   └─────┬──────┘    └──────────── ┘                            │
└─────────┼──────────────────────────────────────────────────────┘
          │ scrape /metrics (Basic Auth)
          ▼
┌──────────────────────────┐
│      Grafana Cloud       │
│  Dashboards + Alerting   │
└──────────────────────────┘
```

---

## 📁 Project Structure

```
Cron-Job-Manager/
├── main.go                  # App entry point, router setup, startup checks
├── handlers/                # HTTP handlers + WebSocket endpoint
├── services/                # Scheduler, retry logic, notifications, WebSocket broadcast
├── models/                  # Job, User, Execution Log DB schemas
├── middleware/              # JWT auth, role middleware (admin/user), conditional auth
├── metrics/                 # Prometheus metrics + BasicAuth wrapper
├── notifications/           # Email notification logic
├── config/                  # DB connection (PostgreSQL)
├── utils/                   # WebSocket hub, helper functions
├── frontend/
│   └── dashboard.html       # Single-page JS/HTML/CSS dashboard
├── docker/
│   └── postgres/
│       └── init.sql         # DB schema auto-applied on container start
├── Dockerfile               # Multi-stage Go build
├── docker-compose.yml       # App + PostgreSQL with health checks
├── Makefile                 # Full dev/ops command suite
├── go.mod / go.sum
└── package.json             # Frontend tooling
```

---

## 📊 Metrics & Observability

The `/metrics` endpoint is protected by **HTTP Basic Auth** via a custom Go wrapper and exposes Prometheus-format data scraped by Grafana Cloud.

### Metrics Tracked

| Metric | Type | Description |
|---|---|---|
| `cronjob_executions_total` | Counter | Total job executions |
| `cronjob_failures_total` | Counter | Total failed executions |
| `cronjob_success_rate` | Gauge | Live % of successful jobs |
| `cronjob_duration_seconds` | Histogram | Execution time distribution |
| `http_request_duration_seconds` | Histogram | HTTP response time per route |

### Grafana Dashboard Panels
- Execution rate over time
- Failure count (total + recent)
- Success rate % gauge
- p50 / p90 / p99 response latency
- Job duration histogram buckets

---

## 🔐 Authentication & Authorization

The app supports toggling auth on/off via the `AUTH_REQUIRED` environment variable — useful for dev/demo mode.

| Role | Permissions |
|---|---|
| `admin` | Full access — create, update, delete jobs, manage users |
| `user` | Create and trigger jobs, view logs and stats |
| Public | Login, register, health check only |

Default admin credentials (change immediately in production):
```
Username: admin
Password: admin123
```

---

## 📡 API Reference

| Method | Endpoint | Auth | Description |
|---|---|---|---|
| POST | `/api/v1/auth/login` | Public | Login |
| POST | `/api/v1/auth/register` | Public | Register |
| GET | `/api/v1/auth/me` | User | Get current user |
| GET | `/api/v1/jobs` | User | List all jobs |
| POST | `/api/v1/jobs` | User | Create a job |
| PUT | `/api/v1/jobs/:id` | User | Update a job |
| POST | `/api/v1/jobs/:id/trigger` | User | Manually trigger a job |
| DELETE | `/api/v1/jobs/:id` | Admin | Delete a job |
| GET | `/api/v1/jobs/:id/executions` | User | Get execution history |
| GET | `/api/v1/jobs/:id/stats` | User | Get job stats |
| GET | `/api/v1/logs` | User | All execution logs |
| GET | `/api/v1/logs/recent` | User | Recent logs |
| GET | `/api/v1/users` | Admin | List all users |
| GET | `/api/v1/health` | Public | Health check |
| GET | `/metrics` | Basic Auth | Prometheus metrics |
| WS | `/ws` | — | WebSocket live updates |

---

## 🚀 Getting Started

### Prerequisites
- Go 1.21+
- Docker & Docker Compose

### Quickstart with Docker (Recommended)

```bash
git clone https://github.com/TashuChoudhary/Cron-Job-Manager.git
cd Cron-Job-Manager

# Initial setup (creates .env files)
make setup

# Start app + PostgreSQL
make docker-build
make docker-up
```

App → `http://localhost:5000`
Metrics → `http://localhost:5000/metrics` _(Basic Auth required)_

### Run Locally Without Docker

```bash
go mod download
go run main.go
```

### Common Makefile Commands

| Command | Description |
|---|---|
| `make dev` | Run with hot reload (requires `air`) |
| `make run` | Run without hot reload |
| `make docker-up` | Start all containers |
| `make docker-down` | Stop containers (data preserved) |
| `make docker-logs` | Follow container logs |
| `make docker-clean` | Stop + delete all volumes ⚠️ |
| `make backup` | Backup PostgreSQL to `backups/` |
| `make restore FILE=...` | Restore from backup file |
| `make test` | Run tests with coverage report |
| `make status` | Show container health status |
| `make stats` | Show CPU/memory usage |

---

## ⚙️ Environment Variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `5000` | Server port |
| `AUTH_REQUIRED` | `false` | Enable JWT auth |
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_USER` | `postgres` | DB username |
| `DB_PASSWORD` | `changeme` | DB password |
| `DB_NAME` | `cronjob_manager` | Database name |
| `NOTIFICATION_SERVICE_URL` | `http://localhost:3001` | Notification service |
| `SMTP_HOST` | — | SMTP server for email alerts |
| `SMTP_USER` | — | SMTP username |
| `SMTP_PASS` | — | SMTP password |
| `FROM_EMAIL` | — | Sender email address |

---

## 💡 What I Learned

- Building a clean layered architecture in Go: handlers → services → models
- Implementing JWT-based auth with role middleware (`admin` / `user`) in Go
- WebSocket-based real-time broadcasting of system stats to connected clients
- Prometheus instrumentation with counters, gauges, and histograms
- Securing observability endpoints with custom Go Basic Auth middleware
- PostgreSQL schema management with Docker init scripts
- Writing a production Makefile with colored output, dev/prod targets, DB backup/restore, health checks, and deployment commands
- Multi-container orchestration with Docker Compose health checks and service dependencies

---

## 🔭 Roadmap

- [ ] Grafana alerting rule: fire on spike in `cronjob_failures_total`
- [ ] Slack webhook notifications on job failure
- [ ] GitHub Actions CI pipeline (test + lint on push)
- [ ] Job execution timeline visualization in the dashboard

---

## 📄 License

MIT

---

_Built by [Tashu Choudhary](https://github.com/TashuChoudhary)_