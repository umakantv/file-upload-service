package models

import (
	"database/sql"
	"time"
)

// File represents a file record in the system
type File struct {
	ID              string       `json:"id" db:"id"`
	FileName        string       `json:"file_name" db:"file_name"`
	FileSize        int64        `json:"file_size" db:"file_size"`
	Mimetype        string       `json:"mimetype" db:"mimetype"`
	ClientID        string       `json:"client_id" db:"client_id"`
	BucketID        int          `json:"bucket_id" db:"bucket_id"`
	Key             string       `json:"key" db:"key"`
	OwnerEntityType string       `json:"owner_entity_type" db:"owner_entity_type"`
	OwnerEntityID   string       `json:"owner_entity_id" db:"owner_entity_id"`
	CreatedAt       time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at" db:"updated_at"`
	DeletedAt       sql.NullTime `json:"deleted_at,omitempty" db:"deleted_at"`
}

// CreateSignedURLRequest represents the request to generate a signed URL for upload
type CreateSignedURLRequest struct {
	BucketID        int    `json:"bucket_id"`
	Key             string `json:"key"`
	FileName        string `json:"file_name"`
	FileSize        int64  `json:"file_size"`
	Mimetype        string `json:"mimetype"`
	OwnerEntityType string `json:"owner_entity_type"`
	OwnerEntityID   string `json:"owner_entity_id"`
}

// SignedURLResponse represents the response with signed URL
type SignedURLResponse struct {
	FileID    string    `json:"file_id"`
	SignedURL string    `json:"signed_url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UploadTokenData represents the data stored in Redis for upload validation
type UploadTokenData struct {
	FileID          string `json:"file_id"`
	FileName        string `json:"file_name"`
	FileSize        int64  `json:"file_size"`
	Mimetype        string `json:"mimetype"`
	ClientID        string `json:"client_id"`
	BucketID        int    `json:"bucket_id"`
	// FilePath is the resolved storage path relative to ./uploads/
	// Format: <client_name>/<bucket_name>/<key>  (key may itself contain slashes)
	FilePath        string `json:"file_path"`
	OwnerEntityType string `json:"owner_entity_type"`
	OwnerEntityID   string `json:"owner_entity_id"`
}

// GenerateDownloadSignedURLRequest represents the request to generate a download signed URL
type GenerateDownloadSignedURLRequest struct {
	FileID string `json:"file_id"`
}

// DownloadTokenData represents the data stored in Redis for download validation
type DownloadTokenData struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	Mimetype string `json:"mimetype"`
	ClientID string `json:"client_id"`
	BucketID int    `json:"bucket_id"`
	// FilePath is the resolved storage path relative to ./uploads/
	// Format: <client_name>/<bucket_name>/<key>  (key may itself contain slashes)
	FilePath string `json:"file_path"`
}