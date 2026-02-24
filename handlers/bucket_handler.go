package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"file-upload-service/models"

	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/umakantv/go-utils/errs"
	"github.com/umakantv/go-utils/httpserver"
	logger "github.com/umakantv/go-utils/logger"
	"go.uber.org/zap"
)

// bucketNameRegex allows alphanumeric characters and dashes
var bucketNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]$|^[a-zA-Z0-9]$`)

// BucketHandler handles bucket-related operations
type BucketHandler struct {
	db *sqlx.DB
}

// NewBucketHandler creates a new bucket handler
func NewBucketHandler(db *sqlx.DB) *BucketHandler {
	return &BucketHandler{
		db: db,
	}
}

// logRequest logs the request with the specified format
func (h *BucketHandler) logRequest(ctx context.Context, level string, message string, fields ...zap.Field) {
	routeName := httpserver.GetRouteName(ctx)
	method := httpserver.GetRouteMethod(ctx)
	path := httpserver.GetRoutePath(ctx)
	auth := httpserver.GetRequestAuth(ctx)

	logMsg := time.Now().Format("2006-01-02 15:04:05") + " - " + routeName + " - " + method + " - " + path
	if auth != nil {
		logMsg += " - client:" + auth.Client
	}

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

// getClientID extracts the authenticated client ID from context (Basic auth)
func (h *BucketHandler) getClientID(ctx context.Context) (string, bool) {
	auth := httpserver.GetRequestAuth(ctx)
	if auth == nil || auth.Client == "" {
		return "", false
	}
	return auth.Client, true
}

// validateCORSPolicy validates that the cors_policy field is a valid JSON array
// Returns the raw JSON to store (defaults to "[]" if nil/empty)
func validateCORSPolicy(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("[]"), nil
	}
	// Ensure it is a valid JSON array of CORS rules
	var rules []models.CORSRule
	if err := json.Unmarshal(raw, &rules); err != nil {
		return nil, err
	}
	// Re-marshal to ensure clean storage
	clean, err := json.Marshal(rules)
	if err != nil {
		return nil, err
	}
	return clean, nil
}

// validatePublicPaths validates that the public_paths field is a valid JSON array of strings
// Returns the raw JSON to store (defaults to "[]" if nil/empty)
func validatePublicPaths(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("[]"), nil
	}
	// Ensure it is a valid JSON array of strings
	var paths []string
	if err := json.Unmarshal(raw, &paths); err != nil {
		return nil, err
	}
	// Re-marshal to ensure clean storage
	clean, err := json.Marshal(paths)
	if err != nil {
		return nil, err
	}
	return clean, nil
}

// matchesPublicPath checks if a given file key matches any of the public path patterns
// Supports wildcards: * matches any sequence of characters except /
// Example patterns: "images/*", "*.jpg", "public/*"
func matchesPublicPath(key string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchPattern(key, pattern) {
			return true
		}
	}
	return false
}

// matchPattern matches a key against a pattern with * wildcards
func matchPattern(key, pattern string) bool {
	// Simple wildcard matching: * matches any sequence of characters
	if pattern == "*" || pattern == "*/*" || pattern == "**" {
		return true
	}
	
	// Convert pattern to regex
	// Escape special regex characters except *
	regexPattern := "^"
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch c {
		case '*':
			regexPattern += "[^/]*"
		case '.', '+', '?', '(', ')', '[', ']', '{', '}', '^', '$', '|', '\\':
			regexPattern += "\\" + string(c)
		default:
			regexPattern += string(c)
		}
	}
	regexPattern += "$"
	
	matched, _ := regexp.MatchString(regexPattern, key)
	return matched
}

// CreateBucket handles POST /buckets - create a new bucket for the authenticated client
func (h *BucketHandler) CreateBucket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.getClientID(ctx)
	if !ok {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	var req models.CreateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	// Validate name
	if req.Name == "" {
		h.logRequest(ctx, "error", "Missing required field: name")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("name is required"))
		return
	}
	if !bucketNameRegex.MatchString(req.Name) {
		h.logRequest(ctx, "error", "Invalid bucket name", zap.String("name", req.Name))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("name must be alphanumeric with dashes (cannot start or end with a dash)"))
		return
	}

	// Validate and normalise CORS policy
	corsPolicy, err := validateCORSPolicy(req.CORSPolicy)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid cors_policy", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("cors_policy must be a valid JSON array of CORS rules"))
		return
	}

	// Validate and normalise public paths
	publicPaths, err := validatePublicPaths(req.PublicPaths)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid public_paths", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("public_paths must be a valid JSON array of strings"))
		return
	}

	h.logRequest(ctx, "info", "Creating bucket", zap.String("name", req.Name), zap.String("client_id", clientID))

	now := time.Now()
	result, err := h.db.Exec(
		"INSERT INTO buckets (name, client_id, cors_policy, public_paths, archived, created_at, updated_at) VALUES (?, ?, ?, ?, 0, ?, ?)",
		req.Name, clientID, string(corsPolicy), string(publicPaths), now, now,
	)
	if err != nil {
		// SQLite UNIQUE constraint violation
		if isUniqueConstraintError(err) {
			h.logRequest(ctx, "error", "Bucket name already exists for client", zap.String("name", req.Name))
			w.WriteHeader(http.StatusConflict)
			json.NewEncoder(w).Encode(errs.NewValidationError("A bucket with this name already exists for your account"))
			return
		}
		h.logRequest(ctx, "error", "Failed to create bucket", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to create bucket"))
		return
	}

	id, _ := result.LastInsertId()

	h.logRequest(ctx, "info", "Bucket created successfully", zap.Int64("bucket_id", id), zap.String("name", req.Name))

	bucket := models.Bucket{
		ID:          int(id),
		Name:        req.Name,
		ClientID:    clientID,
		CORSPolicy:  corsPolicy,
		PublicPaths: publicPaths,
		Archived:    false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bucket)
}

// GetBuckets handles GET /buckets - list all buckets for the authenticated client
func (h *BucketHandler) GetBuckets(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.getClientID(ctx)
	if !ok {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	h.logRequest(ctx, "info", "Listing buckets", zap.String("client_id", clientID))

	rows, err := h.db.Query(
		"SELECT id, name, client_id, cors_policy, public_paths, archived, created_at, updated_at FROM buckets WHERE client_id = ? ORDER BY created_at DESC",
		clientID,
	)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query buckets", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Database error"))
		return
	}
	defer rows.Close()

	var buckets []models.Bucket
	for rows.Next() {
		var b models.Bucket
		var corsPolicyStr string
		var publicPathsStr string
		var archivedInt int
		if err := rows.Scan(&b.ID, &b.Name, &b.ClientID, &corsPolicyStr, &publicPathsStr, &archivedInt, &b.CreatedAt, &b.UpdatedAt); err != nil {
			h.logRequest(ctx, "error", "Failed to scan bucket row", zap.Error(err))
			continue
		}
		b.CORSPolicy = json.RawMessage(corsPolicyStr)
		b.PublicPaths = json.RawMessage(publicPathsStr)
		b.Archived = archivedInt != 0
		buckets = append(buckets, b)
	}

	h.logRequest(ctx, "info", "Buckets retrieved successfully", zap.Int("count", len(buckets)))

	w.Header().Set("Content-Type", "application/json")
	if buckets == nil {
		buckets = []models.Bucket{}
	}
	json.NewEncoder(w).Encode(buckets)
}

// GetBucket handles GET /buckets/{id} - get a bucket by ID for the authenticated client
func (h *BucketHandler) GetBucket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.getClientID(ctx)
	if !ok {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid bucket ID", zap.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid bucket ID"))
		return
	}

	h.logRequest(ctx, "info", "Getting bucket", zap.Int("bucket_id", id), zap.String("client_id", clientID))

	var b models.Bucket
	var corsPolicyStr string
	var publicPathsStr string
	var archivedInt int
	err = h.db.QueryRow(
		"SELECT id, name, client_id, cors_policy, public_paths, archived, created_at, updated_at FROM buckets WHERE id = ? AND client_id = ?",
		id, clientID,
	).Scan(&b.ID, &b.Name, &b.ClientID, &corsPolicyStr, &publicPathsStr, &archivedInt, &b.CreatedAt, &b.UpdatedAt)

	if err == sql.ErrNoRows {
		h.logRequest(ctx, "info", "Bucket not found", zap.Int("bucket_id", id))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query bucket", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Database error"))
		return
	}

	b.CORSPolicy = json.RawMessage(corsPolicyStr)
	b.PublicPaths = json.RawMessage(publicPathsStr)
	b.Archived = archivedInt != 0

	h.logRequest(ctx, "info", "Bucket retrieved successfully", zap.Int("bucket_id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// UpdateBucket handles PUT /buckets/{id} - update a bucket's CORS policy
func (h *BucketHandler) UpdateBucket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.getClientID(ctx)
	if !ok {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid bucket ID", zap.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid bucket ID"))
		return
	}

	var req models.UpdateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	corsPolicy, err := validateCORSPolicy(req.CORSPolicy)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid cors_policy", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("cors_policy must be a valid JSON array of CORS rules"))
		return
	}

	publicPaths, err := validatePublicPaths(req.PublicPaths)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid public_paths", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("public_paths must be a valid JSON array of strings"))
		return
	}

	h.logRequest(ctx, "info", "Updating bucket", zap.Int("bucket_id", id), zap.String("client_id", clientID))

	now := time.Now()
	result, err := h.db.Exec(
		"UPDATE buckets SET cors_policy = ?, public_paths = ?, updated_at = ? WHERE id = ? AND client_id = ? AND archived = 0",
		string(corsPolicy), string(publicPaths), now, id, clientID,
	)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to update bucket", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to update bucket"))
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		// Check whether it exists at all (might be archived or belong to another client)
		var count int
		h.db.QueryRow("SELECT COUNT(*) FROM buckets WHERE id = ? AND client_id = ?", id, clientID).Scan(&count)
		if count == 0 {
			h.logRequest(ctx, "info", "Bucket not found", zap.Int("bucket_id", id))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
			return
		}
		// It exists but is archived
		h.logRequest(ctx, "error", "Cannot update an archived bucket", zap.Int("bucket_id", id))
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(errs.NewValidationError("Cannot update an archived bucket"))
		return
	}

	// Fetch the updated bucket to return
	var b models.Bucket
	var corsPolicyStr string
	var publicPathsStr string
	var archivedInt int
	h.db.QueryRow(
		"SELECT id, name, client_id, cors_policy, public_paths, archived, created_at, updated_at FROM buckets WHERE id = ?",
		id,
	).Scan(&b.ID, &b.Name, &b.ClientID, &corsPolicyStr, &publicPathsStr, &archivedInt, &b.CreatedAt, &b.UpdatedAt)
	b.CORSPolicy = json.RawMessage(corsPolicyStr)
	b.PublicPaths = json.RawMessage(publicPathsStr)
	b.Archived = archivedInt != 0

	h.logRequest(ctx, "info", "Bucket updated successfully", zap.Int("bucket_id", id))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// ArchiveBucket handles POST /buckets/{id}/archive - archive a bucket
func (h *BucketHandler) ArchiveBucket(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.getClientID(ctx)
	if !ok {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid bucket ID", zap.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid bucket ID"))
		return
	}

	h.logRequest(ctx, "info", "Archiving bucket", zap.Int("bucket_id", id), zap.String("client_id", clientID))

	now := time.Now()
	result, err := h.db.Exec(
		"UPDATE buckets SET archived = 1, updated_at = ? WHERE id = ? AND client_id = ? AND archived = 0",
		now, id, clientID,
	)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to archive bucket", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to archive bucket"))
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		var count int
		h.db.QueryRow("SELECT COUNT(*) FROM buckets WHERE id = ? AND client_id = ?", id, clientID).Scan(&count)
		if count == 0 {
			h.logRequest(ctx, "info", "Bucket not found", zap.Int("bucket_id", id))
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
			return
		}
		// Already archived
		h.logRequest(ctx, "info", "Bucket is already archived", zap.Int("bucket_id", id))
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(errs.NewValidationError("Bucket is already archived"))
		return
	}

	h.logRequest(ctx, "info", "Bucket archived successfully", zap.Int("bucket_id", id))

	// Fetch and return the archived bucket
	var b models.Bucket
	var corsPolicyStr string
	var publicPathsStr string
	var archivedInt int
	h.db.QueryRow(
		"SELECT id, name, client_id, cors_policy, public_paths, archived, created_at, updated_at FROM buckets WHERE id = ?",
		id,
	).Scan(&b.ID, &b.Name, &b.ClientID, &corsPolicyStr, &publicPathsStr, &archivedInt, &b.CreatedAt, &b.UpdatedAt)
	b.CORSPolicy = json.RawMessage(corsPolicyStr)
	b.PublicPaths = json.RawMessage(publicPathsStr)
	b.Archived = archivedInt != 0

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}

// isUniqueConstraintError checks if the error is a SQLite UNIQUE constraint violation
func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
