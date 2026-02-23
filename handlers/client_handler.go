package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"file-upload-service/models"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/umakantv/go-utils/errs"
	"github.com/umakantv/go-utils/httpserver"
	logger "github.com/umakantv/go-utils/logger"
	"go.uber.org/zap"
)

// ClientHandler handles client-related operations
type ClientHandler struct {
	db *sqlx.DB
}

// NewClientHandler creates a new client handler
func NewClientHandler(db *sqlx.DB) *ClientHandler {
	return &ClientHandler{
		db: db,
	}
}

// logRequest logs the request with the specified format
func (h *ClientHandler) logRequest(ctx context.Context, level string, message string, fields ...zap.Field) {
	routeName := httpserver.GetRouteName(ctx)
	method := httpserver.GetRouteMethod(ctx)
	path := httpserver.GetRoutePath(ctx)
	auth := httpserver.GetRequestAuth(ctx)

	// Build log message
	logMsg := time.Now().Format("2006-01-02 15:04:05") + " - " + routeName + " - " + method + " - " + path
	if auth != nil {
		logMsg += " - client:" + auth.Client
	}

	// Add custom fields
	allFields := append([]zap.Field{
		zap.String("route", routeName),
		zap.String("method", method),
		zap.String("path", path),
	}, fields...)

	switch level {
	case "info":
		logger.Info(logMsg, allFields...)
	case "error":
		logger.Error(logMsg, allFields...)
	case "debug":
		logger.Debug(logMsg, allFields...)
	}
}

// generateClientCredentials generates a client_id and client_secret
func generateClientCredentials(name string) (string, string) {
	// Simple generation - in production use crypto/rand
	timestamp := time.Now().Unix()
	clientID := "client_" + strconv.FormatInt(timestamp, 36)
	clientSecret := "secret_" + strconv.FormatInt(timestamp*7, 36) + "_" + name
	return clientID, clientSecret
}

// toClientResponse converts Client to ClientResponse (hides secret)
func toClientResponse(client models.Client) models.ClientResponse {
	return models.ClientResponse{
		ID:        client.ID,
		Name:      client.Name,
		ClientID:  client.ClientID,
		CreatedAt: client.CreatedAt,
		UpdatedAt: client.UpdatedAt,
	}
}

// CreateClient handles POST /clients - create a new client
func (h *ClientHandler) CreateClient(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req models.CreateClientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	// Validate input
	if req.Name == "" {
		h.logRequest(ctx, "error", "Missing required field: name")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Name is required"))
		return
	}

	h.logRequest(ctx, "info", "Creating client", zap.String("name", req.Name))

	// Generate credentials
	clientID, clientSecret := generateClientCredentials(req.Name)
	now := time.Now()

	// Insert client
	result, err := h.db.Exec(
		"INSERT INTO clients (name, client_id, client_secret, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		req.Name, clientID, clientSecret, now, now,
	)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to create client", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to create client"))
		return
	}

	id, _ := result.LastInsertId()

	h.logRequest(ctx, "info", "Client created successfully", zap.Int64("client_db_id", id), zap.String("client_id", clientID))

	// Return created client with credentials (only time secret is shown)
	client := models.Client{
		ID:           int(id),
		Name:         req.Name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(client)
}

// GetClients handles GET /clients - list all clients
func (h *ClientHandler) GetClients(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	h.logRequest(ctx, "info", "Listing clients")

	// Query database
	rows, err := h.db.Query("SELECT id, name, client_id, created_at, updated_at FROM clients ORDER BY created_at DESC")
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query clients", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Database error"))
		return
	}
	defer rows.Close()

	var clients []models.ClientResponse
	for rows.Next() {
		var client models.Client
		err := rows.Scan(&client.ID, &client.Name, &client.ClientID, &client.CreatedAt, &client.UpdatedAt)
		if err != nil {
			h.logRequest(ctx, "error", "Failed to scan client", zap.Error(err))
			continue
		}
		clients = append(clients, toClientResponse(client))
	}

	h.logRequest(ctx, "info", "Clients retrieved successfully", zap.Int("count", len(clients)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clients)
}

// GetClient handles GET /clients/{id} - get client by ID
func (h *ClientHandler) GetClient(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid client ID", zap.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid client ID"))
		return
	}

	h.logRequest(ctx, "info", "Getting client", zap.Int("client_id", id))

	// Query database (without returning secret)
	var client models.Client
	err = h.db.QueryRow("SELECT id, name, client_id, created_at, updated_at FROM clients WHERE id = ?", id).
		Scan(&client.ID, &client.Name, &client.ClientID, &client.CreatedAt, &client.UpdatedAt)

	if err == sql.ErrNoRows {
		h.logRequest(ctx, "info", "Client not found", zap.Int("client_id", id))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Client not found"))
		return
	}
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query client", zap.Error(err), zap.Int("client_id", id))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Database error"))
		return
	}

	h.logRequest(ctx, "info", "Client retrieved successfully", zap.Int("client_id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toClientResponse(client))
}