package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	cachepackage "file-upload-service/cache"
	"file-upload-service/database"
	"file-upload-service/handlers"
	"os"

	"github.com/jmoiron/sqlx"
	"github.com/umakantv/go-utils/httpserver"
	"github.com/umakantv/go-utils/logger"
	"go.uber.org/zap"
)

// AuthChecker handles authentication for the service
type AuthChecker struct {
	db *sqlx.DB
}

// NewAuthChecker creates a new auth checker with database access
func NewAuthChecker(db *sqlx.DB) *AuthChecker {
	return &AuthChecker{db: db}
}

// CheckAuth implements authentication for the service
func (a *AuthChecker) CheckAuth(r *http.Request) (bool, httpserver.RequestAuth) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return false, httpserver.RequestAuth{}
	}

	// Bearer token auth (for client management)
	if len(auth) > 7 && strings.HasPrefix(auth, "Bearer ") {
		token := auth[7:]
		if token == "secret-token" { // Simple check for demo
			return true, httpserver.RequestAuth{
				Type:   "bearer",
				Client: "admin",
				Claims: map[string]interface{}{"role": "admin"},
			}
		}
	}

	// Basic auth (for file operations - validate client_id and client_secret)
	if len(auth) > 6 && strings.HasPrefix(auth, "Basic ") {
		encoded := auth[6:]
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return false, httpserver.RequestAuth{}
		}

		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) != 2 {
			return false, httpserver.RequestAuth{}
		}

		clientID := parts[0]
		clientSecret := parts[1]

		// Validate against database
		var dbClientID string
		err = a.db.QueryRow("SELECT client_id FROM clients WHERE client_id = ? AND client_secret = ?", clientID, clientSecret).Scan(&dbClientID)
		if err == nil && dbClientID == clientID {
			return true, httpserver.RequestAuth{
				Type:   "basic",
				Client: clientID,
				Claims: map[string]interface{}{"client_id": clientID},
			}
		}
	}

	return false, httpserver.RequestAuth{}
}

func StartServer() {
	// Initialize logger
	logger.Init(logger.LoggerConfig{
		CallerKey:  "file",
		TimeKey:    "timestamp",
		CallerSkip: 1,
	})

	logger.Info("Starting File Upload Service...")

	// Initialize database
	dbConn := database.InitializeDatabase()
	defer dbConn.Close()

	// Initialize cache
	cache := cachepackage.InitializeCache()
	defer cache.Close()

	// Initialize auth checker
	authChecker := NewAuthChecker(dbConn)

	// Initialize handlers
	clientHandler := handlers.NewClientHandler(dbConn)
	fileHandler := handlers.NewFileHandler(dbConn, cache)
	bucketHandler := handlers.NewBucketHandler(dbConn)
	publicFileHandler := handlers.NewPublicFileHandler(dbConn)

	// Create HTTP server with authentication
	server := httpserver.New("8080", authChecker.CheckAuth)

	// Register routes
	server.Register(httpserver.Route{
		Name:     "HealthCheck",
		Method:   "GET",
		Path:     "/health",
		AuthType: "none",
	}, httpserver.HandlerFunc(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "healthy", "service": "file-upload-service"}`))
	}))

	// Client management routes (Bearer auth)
	server.Register(httpserver.Route{
		Name:     "CreateClient",
		Method:   "POST",
		Path:     "/clients",
		AuthType: "bearer",
	}, httpserver.HandlerFunc(clientHandler.CreateClient))

	server.Register(httpserver.Route{
		Name:     "ListClients",
		Method:   "GET",
		Path:     "/clients",
		AuthType: "bearer",
	}, httpserver.HandlerFunc(clientHandler.GetClients))

	server.Register(httpserver.Route{
		Name:     "GetClient",
		Method:   "GET",
		Path:     "/clients/{id}",
		AuthType: "bearer",
	}, httpserver.HandlerFunc(clientHandler.GetClient))

	// Bucket management routes (Basic auth - client credentials)
	server.Register(httpserver.Route{
		Name:     "CreateBucket",
		Method:   "POST",
		Path:     "/buckets",
		AuthType: "basic",
	}, httpserver.HandlerFunc(bucketHandler.CreateBucket))

	server.Register(httpserver.Route{
		Name:     "ListBuckets",
		Method:   "GET",
		Path:     "/buckets",
		AuthType: "basic",
	}, httpserver.HandlerFunc(bucketHandler.GetBuckets))

	server.Register(httpserver.Route{
		Name:     "GetBucket",
		Method:   "GET",
		Path:     "/buckets/{id}",
		AuthType: "basic",
	}, httpserver.HandlerFunc(bucketHandler.GetBucket))

	server.Register(httpserver.Route{
		Name:     "UpdateBucket",
		Method:   "PUT",
		Path:     "/buckets/{id}",
		AuthType: "basic",
	}, httpserver.HandlerFunc(bucketHandler.UpdateBucket))

	server.Register(httpserver.Route{
		Name:     "ArchiveBucket",
		Method:   "POST",
		Path:     "/buckets/{id}/archive",
		AuthType: "basic",
	}, httpserver.HandlerFunc(bucketHandler.ArchiveBucket))

	// File upload routes (Basic auth for signed URL generation)
	server.Register(httpserver.Route{
		Name:     "GenerateSignedURL",
		Method:   "POST",
		Path:     "/files/signed-url",
		AuthType: "basic",
	}, httpserver.HandlerFunc(fileHandler.GenerateSignedURL))

	// File upload endpoint (no auth - token in URL)
	server.Register(httpserver.Route{
		Name:     "UploadFile",
		Method:   "POST",
		Path:     "/files/upload",
		AuthType: "none",
	}, httpserver.HandlerFunc(fileHandler.UploadFile))

	// File download routes (Basic auth for signed URL generation)
	server.Register(httpserver.Route{
		Name:     "GenerateDownloadSignedURL",
		Method:   "POST",
		Path:     "/files/download-url",
		AuthType: "basic",
	}, httpserver.HandlerFunc(fileHandler.GenerateDownloadSignedURL))

	// File download endpoint (no auth - token in URL)
	server.Register(httpserver.Route{
		Name:     "DownloadFile",
		Method:   "GET",
		Path:     "/files/download",
		AuthType: "none",
	}, httpserver.HandlerFunc(fileHandler.DownloadFile))

	// Public file access endpoint (no auth, CORS enforced if configured)
	server.Register(httpserver.Route{
		Name:     "ServePublicFile",
		Method:   "GET",
		Path:     "/files/{bucket_name}/{file_path:.*}",
		AuthType: "none",
	}, httpserver.HandlerFunc(publicFileHandler.ServePublicFile))

	logger.Info("File Upload Service started on port 8080")
	logger.Info("Health check: GET /health")
	logger.Info("Client API: POST/GET /clients, GET /clients/{id} (Bearer auth)")
	logger.Info("Bucket API: POST/GET /buckets, GET/PUT /buckets/{id}, POST /buckets/{id}/archive (Basic auth)")
	logger.Info("File API: POST /files/signed-url (Basic auth), POST /files/upload (token in URL)")
	logger.Info("File API: POST /files/download-url (Basic auth), GET /files/download (token in URL)")
	logger.Info("Public File API: GET /files/{bucket_name}/{file_path} (no auth, CORS enforced)")

	// Start server
	if err := server.Start(); err != nil {
		logger.Error("Server failed to start", zap.Error(err))
		os.Exit(1)
	}
}
