-- Function versions table: created FIRST to avoid FK reference issues
CREATE TABLE IF NOT EXISTS function_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_id UUID,
    version_number INTEGER NOT NULL,
    image_uri TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'QUEUED',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Users table: authentication and API keys
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    api_key TEXT UNIQUE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Functions table: function definitions and resource limits
CREATE TABLE IF NOT EXISTS functions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    entrypoint VARCHAR(255) NOT NULL,
    memory_limit INTEGER NOT NULL DEFAULT 128,
    cpu_limit INTEGER NOT NULL DEFAULT 100,
    timeout INTEGER NOT NULL DEFAULT 30,
    max_concurrency INTEGER NOT NULL DEFAULT 1,
    active_version_id UUID REFERENCES function_versions(id),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Now add the FK from function_versions back to functions
ALTER TABLE function_versions
    ADD CONSTRAINT fk_function_versions_function_id
    FOREIGN KEY (function_id) REFERENCES functions(id) ON DELETE CASCADE;

-- Workers table: registered execution nodes
CREATE TABLE IF NOT EXISTS workers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    worker_id VARCHAR(100) UNIQUE NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'HEALTHY',
    cpu_capacity INTEGER NOT NULL,
    memory_capacity INTEGER NOT NULL,
    last_heartbeat TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Invocations: execution tracking
CREATE TABLE IF NOT EXISTS invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_id UUID REFERENCES functions(id),
    version_id UUID REFERENCES function_versions(id),
    worker_id UUID,
    status VARCHAR(20) NOT NULL,
    duration_ms INTEGER,
    cold_start BOOLEAN DEFAULT false,
    retry_count INTEGER DEFAULT 0,
    error_message TEXT,
    idempotency_key TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Logs: structured invocation logs
CREATE TABLE IF NOT EXISTS logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invocation_id UUID REFERENCES invocations(id) ON DELETE CASCADE,
    level VARCHAR(10) NOT NULL,
    message TEXT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Dead Letter Queue: failed invocations after max retries
CREATE TABLE IF NOT EXISTS dlq_invocations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    function_id UUID REFERENCES functions(id),
    version_id UUID REFERENCES function_versions(id),
    attempt_count INTEGER,
    last_error TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for query performance
CREATE INDEX IF NOT EXISTS idx_functions_user_id ON functions(user_id);
CREATE INDEX IF NOT EXISTS idx_function_versions_function_id ON function_versions(function_id);
CREATE INDEX IF NOT EXISTS idx_invocations_function_id ON invocations(function_id);
CREATE INDEX IF NOT EXISTS idx_invocations_worker_id ON invocations(worker_id);
CREATE INDEX IF NOT EXISTS idx_invocations_status ON invocations(status);
CREATE INDEX IF NOT EXISTS idx_invocations_idempotency ON invocations(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_logs_invocation_id ON logs(invocation_id);
CREATE INDEX IF NOT EXISTS idx_workers_status ON workers(status);
CREATE INDEX IF NOT EXISTS idx_workers_last_heartbeat ON workers(last_heartbeat);
