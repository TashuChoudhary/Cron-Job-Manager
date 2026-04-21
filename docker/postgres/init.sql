-- 1. Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(100) NOT NULL UNIQUE,
    email VARCHAR(255) NOT NULL UNIQUE,
    password VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user' CHECK(role IN ('admin', 'user', 'viewer')),
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_login TIMESTAMP
);

-- 2. Create jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    cron_expression VARCHAR(255) NOT NULL,
    command TEXT NOT NULL,
    is_active BOOLEAN DEFAULT true,
    category VARCHAR(100) DEFAULT 'general',
    max_retries INTEGER DEFAULT 3,
    retry_delay_seconds INTEGER DEFAULT 60,
    timeout_seconds INTEGER DEFAULT 300,
    environment_vars JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    priority INTEGER DEFAULT 0,
    concurrent_executions INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 3. Create job_dependencies table
CREATE TABLE IF NOT EXISTS job_dependencies (
    id SERIAL PRIMARY KEY,
    job_id INTEGER NOT NULL,
    depends_on_job_id INTEGER NOT NULL,
    dependency_type VARCHAR(50) DEFAULT 'success',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    UNIQUE(job_id, depends_on_job_id)
);

-- 4. Create job_templates table
CREATE TABLE IF NOT EXISTS job_templates (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT,
    category VARCHAR(100),
    command_template TEXT NOT NULL,
    default_cron_expression VARCHAR(255) NOT NULL,
    default_environment_vars JSONB DEFAULT '{}',
    default_max_retries INTEGER DEFAULT 3,
    default_timeout_seconds INTEGER DEFAULT 300,
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 5. Create job_executions table
CREATE TABLE IF NOT EXISTS job_executions (
    id SERIAL PRIMARY KEY,
    job_id INTEGER NOT NULL,
    status VARCHAR(50) NOT NULL CHECK(status IN ('pending', 'running', 'success', 'failed', 'timeout')),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    output TEXT,
    error_message TEXT,
    duration_ms INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    is_retry BOOLEAN DEFAULT false,
    parent_execution_id INTEGER,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_execution_id) REFERENCES job_executions(id) ON DELETE SET NULL
);

-- 6. Create execution_logs table
CREATE TABLE IF NOT EXISTS execution_logs (
    id SERIAL PRIMARY KEY,
    job_id INTEGER NOT NULL,
    job_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK(status IN ('running', 'success', 'failed')),
    start_time TIMESTAMP NOT NULL,
    end_time TIMESTAMP,
    duration DOUBLE PRECISION DEFAULT 0,
    output TEXT,
    error_msg TEXT,
    exit_code INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (job_id) REFERENCES jobs(id) ON DELETE CASCADE
);

-- 7. Create indexes
CREATE INDEX IF NOT EXISTS idx_jobs_is_active ON jobs(is_active);
CREATE INDEX IF NOT EXISTS idx_jobs_category ON jobs(category);
CREATE INDEX IF NOT EXISTS idx_job_executions_job_id ON job_executions(job_id);
CREATE INDEX IF NOT EXISTS idx_job_executions_status ON job_executions(status);
CREATE INDEX IF NOT EXISTS idx_job_executions_started_at ON job_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_execution_logs_job_id ON execution_logs(job_id);
CREATE INDEX IF NOT EXISTS idx_execution_logs_status ON execution_logs(status);
CREATE INDEX IF NOT EXISTS idx_execution_logs_start_time ON execution_logs(start_time DESC);
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- 8. Insert default admin user (using pgcrypto for bcrypt)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO users (username, email, password, role, is_active)
VALUES (
    'admin',
    'admin@cronjob.local',
    crypt('admin123', gen_salt('bf')),  -- bcrypt hash
    'admin',
    true
)
ON CONFLICT (username) DO NOTHING;

-- 9. Insert sample jobs 
INSERT INTO jobs (name, description, cron_expression, command, category) VALUES
    ('Database Backup', 'Daily backup of production database', '0 2 * * *', '/usr/local/bin/backup-db.sh', 'backup'),
    ('Cache Cleanup', 'Clear expired cache entries', '*/30 * * * *', 'redis-cli FLUSHDB', 'maintenance'),
    ('API Health Check', 'Monitor API endpoints', '*/5 * * * *', 'curl -f http://api.example.com/health', 'monitoring')
ON CONFLICT DO NOTHING;