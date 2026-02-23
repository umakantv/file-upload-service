package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// PublicFileHandler handles public file access operations
type PublicFileHandler struct {
	db *sqlx.DB
}

// NewPublicFileHandler creates a new public file handler
func NewPublicFileHandler(db *sqlx.DB) *PublicFileHandler {
	return &PublicFileHandler{
		db: db,
	}
}

// logRequest logs the request with the specified format
func (h *PublicFileHandler) logRequest(ctx context.Context, level string, message string, fields ...zap.Field) {
	routeName := httpserver.GetRouteName(ctx)
	method := httpserver.GetRouteMethod(ctx)
	path := httpserver.GetRoutePath(ctx)

	logMsg := time.Now().Format("2006-01-02 15:04:05") + " - " + routeName + " - " + method + " - " + path

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

// ServePublicFile handles GET /files/{bucket_name}/{file_path...} - serve public files
// No authentication required, but CORS policy is enforced if configured
func (h *PublicFileHandler) ServePublicFile(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	bucketName := vars["bucket_name"]
	filePath := vars["file_path"]

	h.logRequest(ctx, "info", "Serving public file",
		zap.String("bucket_name", bucketName),
		zap.String("file_path", filePath),
	)

	// Look up the bucket by name
	var bucket models.Bucket
	var corsPolicyStr string
	var publicPathsStr string
	var archivedInt int
	err := h.db.QueryRow(
		"SELECT id, name, client_id, cors_policy, public_paths, archived, created_at, updated_at FROM buckets WHERE name = ?",
		bucketName,
	).Scan(&bucket.ID, &bucket.Name, &bucket.ClientID, &corsPolicyStr, &publicPathsStr, &archivedInt, &bucket.CreatedAt, &bucket.UpdatedAt)

	if err != nil {
		h.logRequest(ctx, "error", "Bucket not found", zap.String("bucket_name", bucketName), zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}

	bucket.CORSPolicy = json.RawMessage(corsPolicyStr)
	bucket.PublicPaths = json.RawMessage(publicPathsStr)
	bucket.Archived = archivedInt != 0

	// Check if bucket is archived
	if bucket.Archived {
		h.logRequest(ctx, "error", "Bucket is archived", zap.String("bucket_name", bucketName))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("Bucket not found"))
		return
	}

	// Parse public paths
	var publicPaths []string
	if err := json.Unmarshal(bucket.PublicPaths, &publicPaths); err != nil {
		h.logRequest(ctx, "error", "Failed to parse public_paths", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to check public access"))
		return
	}

	// Check if the requested file path matches any public path pattern
	// filePath from mux includes the full path, we need to check if it's public
	if !matchesPublicPath(filePath, publicPaths) {
		h.logRequest(ctx, "info", "File is not publicly accessible",
			zap.String("bucket_name", bucketName),
			zap.String("file_path", filePath),
		)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errs.NewAuthorizationError("File is not publicly accessible"))
		return
	}

	// Fetch the client name for constructing the file path
	var clientName string
	err = h.db.QueryRow("SELECT name FROM clients WHERE client_id = ?", bucket.ClientID).Scan(&clientName)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to fetch client name", zap.String("client_id", bucket.ClientID), zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to locate file"))
		return
	}

	// Construct the full file path: ./uploads/<client_name>/<bucket_name>/<file_path>
	fullPath := filepath.Join("./uploads", clientName, bucketName, filePath)

	// Check if file exists
	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			h.logRequest(ctx, "info", "File not found on disk",
				zap.String("bucket_name", bucketName),
				zap.String("file_path", filePath),
				zap.String("full_path", fullPath),
			)
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(errs.NewNotFoundError("File not found"))
			return
		}
		h.logRequest(ctx, "error", "Failed to stat file", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to access file"))
		return
	}

	// Check if it's a directory (shouldn't serve directories)
	if fileInfo.IsDir() {
		h.logRequest(ctx, "error", "Requested path is a directory", zap.String("full_path", fullPath))
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(errs.NewNotFoundError("File not found"))
		return
	}

	// Open the file
	file, err := os.Open(fullPath)
	if err != nil {
		h.logRequest(ctx, "error", "Failed to open file", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errs.NewInternalServerError("Failed to read file"))
		return
	}
	defer file.Close()

	// Determine content type based on file extension
	contentType := getContentTypeFromExtension(filepath.Ext(filePath))

	// Apply CORS headers if configured
	applyCORSHeaders(w, r, bucket.CORSPolicy)

	h.logRequest(ctx, "info", "Serving public file",
		zap.String("bucket_name", bucketName),
		zap.String("file_path", filePath),
		zap.String("content_type", contentType),
	)

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.WriteHeader(http.StatusOK)

	// Stream file content
	if _, err := io.Copy(w, file); err != nil {
		h.logRequest(ctx, "error", "Failed to stream file", zap.Error(err))
	}
}

// getContentTypeFromExtension returns the content type based on file extension
func getContentTypeFromExtension(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".webp":
		return "image/webp"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".mp4":
		return "video/mp4"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}

// applyCORSHeaders applies CORS headers based on the bucket's CORS policy
func applyCORSHeaders(w http.ResponseWriter, r *http.Request, corsPolicy json.RawMessage) {
	// Parse CORS policy
	var rules []models.CORSRule
	if err := json.Unmarshal(corsPolicy, &rules); err != nil {
		return
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}

	// Find a matching rule
	for _, rule := range rules {
		if isOriginAllowed(origin, rule.AllowedOrigins) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")

			if len(rule.AllowedMethods) > 0 {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(rule.AllowedMethods, ", "))
			}

			if len(rule.AllowedHeaders) > 0 {
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(rule.AllowedHeaders, ", "))
			}

			if len(rule.ExposeHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(rule.ExposeHeaders, ", "))
			}

			break
		}
	}
}

// isOriginAllowed checks if the origin matches any of the allowed origins
// Supports wildcards: * matches any sequence of characters
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Check for wildcard match
		if strings.Contains(allowed, "*") {
			if matchWildcard(origin, allowed) {
				return true
			}
		}
	}
	return false
}

// matchWildcard matches an origin against a pattern with wildcards
func matchWildcard(origin, pattern string) bool {
	// Simple wildcard matching - * matches any sequence of characters
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return origin == pattern
	}

	// Check prefix
	if !strings.HasPrefix(origin, parts[0]) {
		return false
	}

	// Check suffix
	if !strings.HasSuffix(origin, parts[len(parts)-1]) {
		return false
	}

	// Check middle parts in order
	remaining := origin[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(remaining, parts[i])
		if idx == -1 {
			return false
		}
		remaining = remaining[idx+len(parts[i]):]
	}

	return true
}