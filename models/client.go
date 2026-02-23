package models

import "time"

// Client represents an IAM-like client for authentication
type Client struct {
	ID           int       `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	ClientID     string    `json:"client_id" db:"client_id"`
	ClientSecret string    `json:"client_secret,omitempty" db:"client_secret"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// CreateClientRequest represents the request to create a client
type CreateClientRequest struct {
	Name string `json:"name"`
}

// ClientResponse represents the client response (without secret)
type ClientResponse struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	ClientID  string    `json:"client_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}