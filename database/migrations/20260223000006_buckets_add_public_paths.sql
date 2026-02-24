-- Migration: buckets_add_public_paths
-- Created: 2026-02-23

-- Add public_paths column to buckets table.
-- This stores a JSON array of path patterns that should be publicly accessible
-- without requiring a signed URL. Patterns can include wildcards.
-- Example: ["images/*", "public/*", "*.jpg"]
ALTER TABLE buckets ADD COLUMN public_paths TEXT NOT NULL DEFAULT '[]';

-- Create index for faster bucket lookups by name
CREATE INDEX IF NOT EXISTS idx_buckets_name ON buckets(name);
