package db

const SchemaSQL = `
CREATE TABLE IF NOT EXISTS customers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    customer_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    hourly_rate REAL DEFAULT 0.0,
    budget_hours REAL DEFAULT 0.0,
    budget_cost REAL DEFAULT 0.0,
    color TEXT DEFAULT '#3B82F6',
    is_active INTEGER DEFAULT 1,
    created_at DATETIME NOT NULL,
    FOREIGN KEY(customer_id) REFERENCES customers(id) ON DELETE CASCADE,
    UNIQUE(customer_id, name)
);

CREATE TABLE IF NOT EXISTS time_entries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER,
    task_name TEXT NOT NULL,
    booking_text TEXT DEFAULT '',
    started_at DATETIME NOT NULL,
    ended_at DATETIME,
    duration_sec INTEGER DEFAULT 0,
    is_billable INTEGER DEFAULT 1,
    is_quickshift INTEGER DEFAULT 0,
    parent_id INTEGER,
    paused_at DATETIME,
    paused_ns INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    FOREIGN KEY(project_id) REFERENCES projects(id) ON DELETE SET NULL,
    FOREIGN KEY(parent_id) REFERENCES time_entries(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_entries_started_at ON time_entries(started_at);
CREATE INDEX IF NOT EXISTS idx_entries_project_id ON time_entries(project_id);
CREATE INDEX IF NOT EXISTS idx_entries_parent_id ON time_entries(parent_id);
CREATE INDEX IF NOT EXISTS idx_projects_customer_id ON projects(customer_id);

CREATE TABLE IF NOT EXISTS settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
`
