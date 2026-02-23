-- Migration: files_add_key
-- Created: 2026-02-23

-- Add key column to files table.
-- The key is a client-provided path string (e.g. "invoices/2024/january/receipt.pdf")
-- that determines the nested storage path within the client/bucket folder.
-- Defaults to empty string; existing rows keep an empty key (path falls back to file ID).
ALTER TABLE files ADD COLUMN key TEXT NOT NULL DEFAULT '';

-- Create index on key for faster lookups within a bucket
CREATE INDEX IF NOT EXISTS idx_files_key ON files(bucket_id, key);
