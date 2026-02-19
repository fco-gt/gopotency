# SQL Storage Schema

To use the SQL storage backend, you need to create the following tables in your database.

## PostgreSQL

```sql
-- Main records table
CREATE TABLE idempotency_records (
    key VARCHAR(255) PRIMARY KEY,
    data BYTEA NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Index for automatic cleanup (optional)
CREATE INDEX idx_idempotency_expires ON idempotency_records(expires_at);

-- Locks table
CREATE TABLE idempotency_records_locks (
    key VARCHAR(255) PRIMARY KEY,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);
```

## MySQL

```sql
-- Main records table
CREATE TABLE idempotency_records (
    `key` VARCHAR(255) PRIMARY KEY,
    `data` BLOB NOT NULL,
    `expires_at` DATETIME NOT NULL,
    INDEX (expires_at)
);

-- Locks table
CREATE TABLE idempotency_records_locks (
    `key` VARCHAR(255) PRIMARY KEY,
    `expires_at` DATETIME NOT NULL
);
```
