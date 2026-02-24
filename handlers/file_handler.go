package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"file-upload-service/models"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/jmoiron/sqlx"
	"github.com/umakantv/go-utils/cache"
	"github.com/umakantv/go-utils/errs"
	"github.com/umakantv/go-utils/httpserver"
	logger "github.com/umakantv/go-utils/logger"
	"go.uber.org/zap"
)

// FileHandler handles file-related operations
type FileHandler struct {
	db    *sqlx.DB
	cache cache.Cache
}

// NewFileHandler creates a new file handler
func NewFileHandler(db *sqlx.DB, cache cache.Cache) *FileHandler {
	return &FileHandler{
		db:    db,
		cache: cache,
	}
}

// logRequest logs the request with the specified format
func (h *FileHandler) logRequest(ctx context.Context, level string, message string, fields ...zap.Field) {
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

// generateUploadToken generates a random token for signed URL
func generateUploadToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateSignedURL handles POST /files/signed-url - generate a signed URL for file upload
func (h *FileHandler) GenerateSignedURL(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req models.CreateSignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	// Validate input
	if req.BucketID <= 0 {
		h.logRequest(ctx, "error", "Missing or invalid required field: bucket_id")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("bucket_id is required and must be a positive integer"))
		return
	}
	if req.Key == "" {
		h.logRequest(ctx, "error", "Missing required field: key")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("key is required"))
		return
	}
	if req.FileName == "" {
		h.logRequest(ctx, "error", "Missing required field: file_name")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("file_name is required"))
		return
	}
	if req.FileSize <= 0 {
		h.logRequest(ctx, "error", "Invalid file_size", zap.Int64("file_size", req.FileSize))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("file_size must be greater than 0"))
		return
	}
	if req.Mimetype == "" {
		h.logRequest(ctx, "error", "Missing required field: mimetype")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("mimetype is required"))
		return
	}
	if req.OwnerEntityType == "" {
		h.logRequest(ctx, "error", "Missing required field: owner_entity_type")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("owner_entity_type is required"))
		return
	}
	if req.OwnerEntityID == "" {
		h.logRequest(ctx, "error", "Missing required field: owner_entity_id")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("owner_entity_id is required"))
		return
	}

	// Get client ID from auth context (from Basic auth)
	auth := httpserver.GetRequestAuth(ctx)
	if auth == nil {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}
	clientID := auth.Client

	// Verify the bucket exists, belongs to the authenticated client, and is not archived
	// Also fetch the bucket name for folder structure
	var bucketClientID string
	var bucketName string
	var bucketArchived int
	err := h.db.QueryRow(
		"SELECT client_id, name, archived FROM buckets WHERE id = ?",
		req.BucketID,
	).Scan(&bucketClientID, &bucketName, &bucketArchived)
	if err != nil {
		h.logRequest(ctx, "error", "Bucket not found", zap.Int("bucket_id", req.BucketID), zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}
	if bucketClientID != clientID {
		h.logRequest(ctx, "error", "Bucket does not belong to client",
			zap.Int("bucket_id", req.BucketID),
			zap.String("client_id", clientID),
		)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errs.NewAuthorizationError("Access denied: bucket does not belong to your account"))
		return
	}
	if bucketArchived != 0 {
		h.logRequest(ctx, "error", "Bucket is archived", zap.Int("bucket_id", req.BucketID))
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(errs.NewValidationError("Cannot upload to an archived bucket"))
		return
	}

	// Fetch the client name for folder structure
	var clientName string
	err = h.db.QueryRow("SELECT name FROM clients WHERE client_id = ?", clientID).Scan(&clientName)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to fetch client name", zap.String("client_id", clientID), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to fetch client information"))
		return
	}

	h.logRequest(ctx, "info", "Generating signed URL",
		zap.String("file_name", req.FileName),
		zap.String("client_id", clientID),
		zap.Int("bucket_id", req.BucketID),
		zap.String("key", req.Key),
	)

	// Generate file ID
	fileID := uuid.New().String()
	now := time.Now()

	// Build the resolved file path: <client_name>/<bucket_name>/<key>
	// The key may contain slashes for deeper nesting (e.g. "invoices/2024/receipt.pdf")
	filePath := filepath.Join(clientName, bucketName, req.Key)

	// Insert file record into database (including the key)
	_, err = h.db.Exec(
		"INSERT INTO files (id, file_name, file_size, mimetype, client_id, bucket_id, key, owner_entity_type, owner_entity_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		fileID, req.FileName, req.FileSize, req.Mimetype, clientID, req.BucketID, req.Key, req.OwnerEntityType, req.OwnerEntityID, now, now,
	)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to create file record", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to create file record"))
		return
	}

	// Generate upload token
	uploadToken := generateUploadToken()

	// Store upload token data in Redis with 15 minute TTL.
	// FilePath carries the full resolved path so the upload handler needs no extra DB lookups.
	tokenData := models.UploadTokenData{
		FileID:          fileID,
		FileName:        req.FileName,
		FileSize:        req.FileSize,
		Mimetype:        req.Mimetype,
		ClientID:        clientID,
		BucketID:        req.BucketID,
		FilePath:        filePath,
		OwnerEntityType: req.OwnerEntityType,
		OwnerEntityID:   req.OwnerEntityID,
	}

	ttl := 15 * time.Minute

	err = h.cache.Set("upload:"+uploadToken, tokenData, ttl)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to store upload token in cache", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to generate signed URL"))
		return
	}

	// Generate signed URL
	signedURL := fmt.Sprintf("http://localhost:8080/files/upload?token=%s", uploadToken)
	expiresAt := now.Add(ttl)

	h.logRequest(ctx, "info", "Signed URL generated successfully",
		zap.String("file_id", fileID),
		zap.String("client_id", clientID),
		zap.Int("bucket_id", req.BucketID),
	)

	// Return signed URL response
	response := models.SignedURLResponse{
		FileID:    fileID,
		SignedURL: signedURL,
		ExpiresAt: expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// UploadFile handles POST /files/upload - upload file using token from URL (no auth header required)
func (h *FileHandler) UploadFile(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	// Get token from URL query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		h.logRequest(ctx, "error", "Missing upload token")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Missing upload token"))
		return
	}

	h.logRequest(ctx, "info", "Processing file upload", zap.String("token", token[:8]+"..."))

	// Retrieve token data from Redis
	cachedData, err := h.cache.Get("upload:" + token)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid or expired upload token", zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Invalid or expired upload token"))
		return
	}

	// Parse token data.
	// The Redis cache layer does json.Marshal on Set and json.Unmarshal on Get,
	// so cachedData comes back as map[string]interface{} for a JSON object.
	// Re-marshal to JSON then unmarshal into the typed struct.
	var tokenData models.UploadTokenData

	intermediate, err := json.Marshal(cachedData)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to re-marshal token data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to parse token data"))
		return
	}
	if err := json.Unmarshal(intermediate, &tokenData); err != nil {
		h.logRequest(ctx, "error", "Failed to parse token data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to parse token data"))
		return
	}

	// Parse multipart form
	err = r.ParseMultipartForm(100 << 20) // 100 MB max memory
	if err != nil {
		h.logRequest(ctx, "error", "Failed to parse multipart form", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Failed to parse upload form"))
		return
	}

	// Get file from form
	file, header, err := r.FormFile("file")
	if err != nil {
		h.logRequest(ctx, "error", "Failed to get file from form", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Missing file in upload"))
		return
	}
	defer file.Close()

	// Validate file size
	if header.Size > tokenData.FileSize {
		h.logRequest(ctx, "error", "File size exceeds limit",
			zap.Int64("uploaded_size", header.Size),
			zap.Int64("max_size", tokenData.FileSize),
		)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("File size exceeds allowed limit"))
		return
	}

	// Resolve the full on-disk path from the token.
	// tokenData.FilePath is <client_name>/<bucket_name>/<key> where key may contain slashes.
	// The actual file is stored at that exact path under ./uploads/.
	absFilePath := filepath.Join("./uploads", tokenData.FilePath)

	// Ensure all parent directories exist (key may introduce extra nesting)
	if err := os.MkdirAll(filepath.Dir(absFilePath), 0755); err != nil {
		h.logRequest(ctx, "error", "Failed to create nested upload directory", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to prepare upload storage"))
		return
	}

	filePath := absFilePath

	// Create destination file
	destFile, err := os.Create(filePath)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to create destination file", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to save file"))
		return
	}
	defer destFile.Close()

	// Copy file content
	written, err := io.Copy(destFile, file)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to write file", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to save file"))
		return
	}

	// Delete the token from Redis (one-time use)
	h.cache.Delete("upload:" + token)

	h.logRequest(ctx, "info", "File uploaded successfully",
		zap.String("file_id", tokenData.FileID),
		zap.String("client_id", tokenData.ClientID),
		zap.Int("bucket_id", tokenData.BucketID),
		zap.Int64("bytes_written", written),
	)

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":    "File uploaded successfully",
		"file_id":    tokenData.FileID,
		"file_name":  tokenData.FileName,
		"file_size":  written,
		"bucket_id":  tokenData.BucketID,
		"saved_path": filePath,
	})
}

// generateDownloadToken generates a random token for a download signed URL
func generateDownloadToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateDownloadSignedURL handles POST /files/download-url - generate a signed URL for file download
func (h *FileHandler) GenerateDownloadSignedURL(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req models.GenerateDownloadSignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	if req.FileID == "" {
		h.logRequest(ctx, "error", "Missing required field: file_id")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("file_id is required"))
		return
	}

	// Get client ID from Basic auth context
	auth := httpserver.GetRequestAuth(ctx)
	if auth == nil {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}
	clientID := auth.Client

	h.logRequest(ctx, "info", "Generating download signed URL",
		zap.String("file_id", req.FileID),
		zap.String("client_id", clientID),
	)

	// Look up the file record — verify it exists and belongs to this client.
	// Also fetch client name, bucket name, and key to reconstruct the storage path.
	var file models.File
	var clientName string
	var bucketName string
	var deletedAt sql.NullTime
	err := h.db.QueryRow(
		`SELECT f.id, f.file_name, f.mimetype, f.client_id, f.bucket_id, f.key, f.deleted_at, c.name, b.name
		 FROM files f
		 JOIN clients c ON f.client_id = c.client_id
		 JOIN buckets b ON f.bucket_id = b.id
		 WHERE f.id = ?`,
		req.FileID,
	).Scan(&file.ID, &file.FileName, &file.Mimetype, &file.ClientID, &file.BucketID, &file.Key, &deletedAt, &clientName, &bucketName)
	if err != nil {
		h.logRequest(ctx, "info", "File not found", zap.String("file_id", req.FileID), zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("File not found"))
		return
	}

	if deletedAt.Valid {
		h.logRequest(ctx, "info", "File has been deleted", zap.String("file_id", req.FileID))
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(errs.NewValidationError("File has been deleted"))
		return
	}

	// Verify the requesting client owns the file
	if file.ClientID != clientID {
		h.logRequest(ctx, "error", "Client does not own this file",
			zap.String("file_id", req.FileID),
			zap.String("requesting_client", clientID),
			zap.String("owner_client", file.ClientID),
		)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errs.NewAuthorizationError("Access denied"))
		return
	}

	// Reconstruct the storage path: <client_name>/<bucket_name>/<key>
	resolvedFilePath := filepath.Join(clientName, bucketName, file.Key)

	// Verify the file exists on disk
	absFilePath := filepath.Join("./uploads", resolvedFilePath)
	if _, err := os.Stat(absFilePath); os.IsNotExist(err) {
		h.logRequest(ctx, "error", "File missing on disk",
			zap.String("file_id", file.ID),
			zap.String("path", absFilePath),
		)
		w.WriteHeader(http.StatusGone)
		json.NewEncoder(w).Encode(errs.NewValidationError("File has been deleted"))
		return
	}

	// Generate download token
	downloadToken := generateDownloadToken()
	ttl := 15 * time.Minute

	// Store token data in Redis.
	// FilePath carries the full resolved path so the download handler needs no extra DB lookups.
	tokenData := models.DownloadTokenData{
		FileID:   file.ID,
		FileName: file.FileName,
		Mimetype: file.Mimetype,
		ClientID: clientID,
		BucketID: file.BucketID,
		FilePath: resolvedFilePath,
	}

	if err := h.cache.Set("download:"+downloadToken, tokenData, ttl); err != nil {
		h.logRequest(ctx, "error", "Failed to store download token in cache", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to generate download URL"))
		return
	}

	now := time.Now()
	signedURL := fmt.Sprintf("http://localhost:8080/files/download?token=%s", downloadToken)
	expiresAt := now.Add(ttl)

	h.logRequest(ctx, "info", "Download signed URL generated successfully",
		zap.String("file_id", file.ID),
		zap.String("client_id", clientID),
		zap.Int("bucket_id", file.BucketID),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(models.SignedURLResponse{
		FileID:    file.ID,
		SignedURL: signedURL,
		ExpiresAt: expiresAt,
	})
}

// DownloadFile handles GET /files/download - download file using token from URL (no auth header required)
func (h *FileHandler) DownloadFile(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.logRequest(ctx, "error", "Missing download token")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Missing download token"))
		return
	}

	h.logRequest(ctx, "info", "Processing file download", zap.String("token", token[:8]+"..."))

	// Retrieve token data from Redis
	cachedData, err := h.cache.Get("download:" + token)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid or expired download token", zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Invalid or expired download token"))
		return
	}

	// Re-marshal through generic map → typed struct (Redis cache returns map[string]interface{})
	var tokenData models.DownloadTokenData
	intermediate, err := json.Marshal(cachedData)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to re-marshal token data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to parse token data"))
		return
	}
	if err := json.Unmarshal(intermediate, &tokenData); err != nil {
		h.logRequest(ctx, "error", "Failed to parse token data", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to parse token data"))
		return
	}

	// Open the file from disk using the resolved path stored in the token
	filePath := filepath.Join("./uploads", tokenData.FilePath)
	f, err := os.Open(filePath)
	if err != nil {
		h.logRequest(ctx, "error", "File not found on disk",
			zap.String("file_id", tokenData.FileID),
			zap.Error(err),
		)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("File not found"))
		return
	}
	defer f.Close()

	// Delete the token from Redis (one-time use)
	h.cache.Delete("download:" + token)

	h.logRequest(ctx, "info", "Serving file download",
		zap.String("file_id", tokenData.FileID),
		zap.String("file_name", tokenData.FileName),
		zap.String("client_id", tokenData.ClientID),
		zap.Int("bucket_id", tokenData.BucketID),
	)

	// Set response headers for file download
	w.Header().Set("Content-Type", tokenData.Mimetype)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, tokenData.FileName))
	w.WriteHeader(http.StatusOK)

	// Stream file content to response
	if _, err := io.Copy(w, f); err != nil {
		h.logRequest(ctx, "error", "Failed to stream file", zap.Error(err))
	}
}

// ListFiles handles GET /buckets/{id}/files - list files at a path (non-recursive)
func (h *FileHandler) ListFiles(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	bucketID, err := strconv.Atoi(idStr)
	if err != nil {
		h.logRequest(ctx, "error", "Invalid bucket ID", zap.String("id", idStr))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid bucket ID"))
		return
	}

	path := strings.Trim(r.URL.Query().Get("path"), "/")

	clientID := ""
	if auth := httpserver.GetRequestAuth(ctx); auth != nil {
		clientID = auth.Client
	}

	if clientID == "" {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	h.logRequest(ctx, "info", "Listing files in bucket", zap.Int("bucket_id", bucketID), zap.String("path", path))

	var bucketClientID string
	var bucketArchived int
	if err := h.db.QueryRow("SELECT client_id, archived FROM buckets WHERE id = ?", bucketID).Scan(&bucketClientID, &bucketArchived); err != nil {
		h.logRequest(ctx, "error", "Bucket not found", zap.Int("bucket_id", bucketID), zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}

	if bucketClientID != clientID {
		h.logRequest(ctx, "error", "Bucket does not belong to client",
			zap.Int("bucket_id", bucketID),
			zap.String("client_id", clientID),
		)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errs.NewAuthorizationError("Access denied: bucket does not belong to your account"))
		return
	}

	if bucketArchived != 0 {
		h.logRequest(ctx, "error", "Bucket is archived", zap.Int("bucket_id", bucketID))
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(errs.NewValidationError("Cannot list files in an archived bucket"))
		return
	}

	query := `SELECT id, file_name, file_size, mimetype, key, created_at
		FROM files
		WHERE bucket_id = ? AND deleted_at IS NULL`
	args := []interface{}{bucketID}

	if path == "" {
		query += " AND key <> ''"
	} else {
		query += " AND key LIKE ?"
		args = append(args, path+"/%")
	}

	query += " ORDER BY key ASC"

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query files", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to list files"))
		return
	}
	defer rows.Close()

	foldersSet := map[string]struct{}{}
	files := make([]models.FileListItem, 0)
	prefix := path
	if prefix != "" {
		prefix += "/"
	}

	for rows.Next() {
		var file models.FileListItem
		var key string
		if err := rows.Scan(&file.ID, &file.FileName, &file.FileSize, &file.Mimetype, &key, &file.CreatedAt); err != nil {
			h.logRequest(ctx, "error", "Failed to scan file row", zap.Error(err))
			continue
		}

		if !strings.HasPrefix(key, prefix) {
			continue
		}

		remainder := strings.TrimPrefix(key, prefix)
		if remainder == "" {
			continue
		}

		segments := strings.Split(remainder, "/")
		if len(segments) == 1 {
			file.Key = key
			files = append(files, file)
		} else {
			foldersSet[segments[0]] = struct{}{}
		}
	}

	folders := make([]string, 0, len(foldersSet))
	for folder := range foldersSet {
		folders = append(folders, folder)
	}
	sort.Strings(folders)

	response := models.ListFilesResponse{
		BucketID: bucketID,
		Path:     path,
		Files:    files,
		Folders:  folders,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DeleteFiles handles DELETE /files - delete files by IDs or by bucket path
func (h *FileHandler) DeleteFiles(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	var req models.DeleteFilesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logRequest(ctx, "error", "Invalid request body", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Invalid JSON"))
		return
	}

	hasFileIDs := len(req.FileIDs) > 0
	hasPath := req.Path != nil
	hasBucketID := req.BucketID != nil

	// Validate: exactly one mode
	if hasFileIDs && (hasPath || hasBucketID) {
		h.logRequest(ctx, "error", "Cannot specify both file_ids and bucket_id/path")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("file_ids and path cannot be used together"))
		return
	}

	if hasPath && !hasBucketID {
		h.logRequest(ctx, "error", "bucket_id is required when path is provided")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("bucket_id is required when path is provided"))
		return
	}

	if !hasFileIDs && !hasPath {
		h.logRequest(ctx, "error", "Missing file_ids or path")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("Either file_ids or (bucket_id and path) is required"))
		return
	}

	clientID := ""
	if auth := httpserver.GetRequestAuth(ctx); auth != nil {
		clientID = auth.Client
	}

	if clientID == "" {
		h.logRequest(ctx, "error", "Client ID not found in auth context")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errs.NewAuthenticationError("Authentication required"))
		return
	}

	if hasFileIDs {
		h.deleteFilesByIDs(ctx, w, clientID, req.FileIDs)
	} else {
		h.deleteFilesByPath(ctx, w, clientID, *req.BucketID, *req.Path)
	}
}

// deleteFilesByIDs deletes files by their IDs
func (h *FileHandler) deleteFilesByIDs(ctx context.Context, w http.ResponseWriter, clientID string, fileIDs []string) {
	h.logRequest(ctx, "info", "Deleting files by IDs", zap.Int("count", len(fileIDs)))

	placeholders := strings.Repeat("?,", len(fileIDs))
	placeholders = strings.TrimSuffix(placeholders, ",")
	args := make([]interface{}, 0, len(fileIDs)+1)
	args = append(args, clientID)
	for _, id := range fileIDs {
		args = append(args, id)
	}

	query := fmt.Sprintf(`SELECT f.id, f.key, c.name, b.name
		FROM files f
		JOIN clients c ON f.client_id = c.client_id
		JOIN buckets b ON f.bucket_id = b.id
		WHERE f.client_id = ? AND f.deleted_at IS NULL AND f.id IN (%s)`, placeholders)

	rows, err := h.db.Query(query, args...)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query files", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to delete files"))
		return
	}
	defer rows.Close()

	records := make(map[string]string)
	for rows.Next() {
		var fileID, key, clientName, bucketName string
		if err := rows.Scan(&fileID, &key, &clientName, &bucketName); err != nil {
			h.logRequest(ctx, "error", "Failed to scan file row", zap.Error(err))
			continue
		}
		records[fileID] = filepath.Join("./uploads", clientName, bucketName, key)
	}

	deleted, missing, failed := h.removeFiles(ctx, fileIDs, records)

	response := models.DeleteFilesResponse{
		Deleted: deleted,
		Missing: missing,
		Failed:  failed,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// deleteFilesByPath deletes all files in a bucket under the given path
func (h *FileHandler) deleteFilesByPath(ctx context.Context, w http.ResponseWriter, clientID string, bucketID int, path string) {
	path = strings.Trim(path, "/")

	h.logRequest(ctx, "info", "Deleting files by path", zap.Int("bucket_id", bucketID), zap.String("path", path))

	// Verify bucket exists and belongs to client
	var bucketClientID string
	var bucketArchived int
	if err := h.db.QueryRow("SELECT client_id, archived FROM buckets WHERE id = ?", bucketID).Scan(&bucketClientID, &bucketArchived); err != nil {
		h.logRequest(ctx, "error", "Bucket not found", zap.Int("bucket_id", bucketID))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}

	if bucketClientID != clientID {
		h.logRequest(ctx, "error", "Bucket does not belong to client",
			zap.Int("bucket_id", bucketID),
			zap.String("client_id", clientID),
		)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errs.NewAuthorizationError("Access denied: bucket does not belong to your account"))
		return
	}

	if bucketArchived != 0 {
		h.logRequest(ctx, "error", "Bucket is archived", zap.Int("bucket_id", bucketID))
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(errs.NewValidationError("Cannot delete files in an archived bucket"))
		return
	}

	// Query all files under the given path (recursive)
	query := `SELECT f.id, f.key, c.name, b.name
		FROM files f
		JOIN clients c ON f.client_id = c.client_id
		JOIN buckets b ON f.bucket_id = b.id
		WHERE f.bucket_id = ? AND f.client_id = ? AND f.deleted_at IS NULL AND f.key LIKE ?`

	prefix := path + "/%"
	rows, err := h.db.Query(query, bucketID, clientID, prefix)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to query files by path", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to delete files"))
		return
	}
	defer rows.Close()

	fileIDs := make([]string, 0)
	records := make(map[string]string)
	for rows.Next() {
		var fileID, key, clientName, bucketName string
		if err := rows.Scan(&fileID, &key, &clientName, &bucketName); err != nil {
			h.logRequest(ctx, "error", "Failed to scan file row", zap.Error(err))
			continue
		}
		fileIDs = append(fileIDs, fileID)
		records[fileID] = filepath.Join("./uploads", clientName, bucketName, key)
	}

	if len(fileIDs) == 0 {
		h.logRequest(ctx, "error", "No files found at path", zap.String("path", path))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errs.NewValidationError("No files found at the given path"))
		return
	}

	deleted, missing, failed := h.removeFiles(ctx, fileIDs, records)

	response := models.DeleteFilesResponse{
		Deleted: deleted,
		Missing: missing,
		Failed:  failed,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// removeFiles deletes files from disk and marks them deleted in the database.
// Returns lists of deleted, missing, and failed file IDs.
func (h *FileHandler) removeFiles(ctx context.Context, fileIDs []string, records map[string]string) (deleted, missing, failed []string) {
	deleted = make([]string, 0)
	missing = make([]string, 0)
	failed = make([]string, 0)

	for _, id := range fileIDs {
		diskPath, ok := records[id]
		if !ok {
			missing = append(missing, id)
			continue
		}

		if err := os.Remove(diskPath); err != nil {
			if os.IsNotExist(err) {
				missing = append(missing, id)
				continue
			}
			h.logRequest(ctx, "error", "Failed to delete file from disk", zap.String("file_id", id), zap.Error(err))
			failed = append(failed, id)
			continue
		}

		_, err := h.db.Exec("UPDATE files SET deleted_at = ?, updated_at = ? WHERE id = ?", time.Now(), time.Now(), id)
		if err != nil {
			h.logRequest(ctx, "error", "Failed to mark file deleted", zap.String("file_id", id), zap.Error(err))
			failed = append(failed, id)
			continue
		}

		deleted = append(deleted, id)
	}

	return deleted, missing, failed
}
