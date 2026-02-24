-- Migration: files_add_bucket_id
-- Created: 2026-02-23

-- Add bucket_id column to files table
ALTER TABLE files ADD COLUMN bucket_id INTEGER NOT NULL DEFAULT 0 REFERENCES buckets(id);

-- Create index on bucket_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_files_bucket_id ON files(bucket_id);

