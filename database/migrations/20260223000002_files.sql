-- Migration: files
-- Created: 2026-02-23

-- Create files table for storing file metadata
CREATE TABLE IF NOT EXISTS files (
    id TEXT PRIMARY KEY,
    file_name TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    mimetype TEXT NOT NULL,
    client_id TEXT NOT NULL,
    owner_entity_type TEXT NOT NULL,
    owner_entity_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME
);

-- Create index on client_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_files_client_id ON files(client_id);

-- Create index on owner_entity for faster lookups
CREATE INDEX IF NOT EXISTS idx_files_owner_entity ON files(owner_entity_type, owner_entity_id);

-- Create index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files(deleted_at);