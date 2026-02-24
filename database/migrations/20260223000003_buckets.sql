-- Migration: buckets
-- Created: 2026-02-23

-- Create buckets table
CREATE TABLE IF NOT EXISTS buckets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    client_id TEXT NOT NULL,
    cors_policy TEXT NOT NULL DEFAULT '[]',
    archived INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, client_id)
);

-- Create index on client_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_buckets_client_id ON buckets(client_id);

-- Create index on name for faster lookups
CREATE INDEX IF NOT EXISTS idx_buckets_name ON buckets(name);
