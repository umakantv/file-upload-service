package models

import (
	"encoding/json"
	"time"
)

// CORSRule represents a single CORS rule for a bucket
type CORSRule struct {
	AllowedHeaders []string `json:"AllowedHeaders"`
	AllowedMethods []string `json:"AllowedMethods"`
	AllowedOrigins []string `json:"AllowedOrigins"`
	ExposeHeaders  []string `json:"ExposeHeaders"`
}

// CORSPolicy is a list of CORS rules
type CORSPolicy []CORSRule

// Bucket represents a storage bucket
type Bucket struct {
	ID         int             `json:"id" db:"id"`
	Name       string          `json:"name" db:"name"`
	ClientID   string          `json:"client_id" db:"client_id"`
	CORSPolicy json.RawMessage `json:"cors_policy" db:"cors_policy"`
	Archived   bool            `json:"archived" db:"archived"`
	CreatedAt  time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at" db:"updated_at"`
}

// CreateBucketRequest represents the request to create a bucket
type CreateBucketRequest struct {
	Name       string          `json:"name"`
	CORSPolicy json.RawMessage `json:"cors_policy"`
}

// UpdateBucketRequest represents the request to update a bucket
type UpdateBucketRequest struct {
	CORSPolicy json.RawMessage `json:"cors_policy"`
}
