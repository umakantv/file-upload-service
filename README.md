# File Upload Service

An in-house alternative to AWS S3 for file storage and management. The service generates signed URLs for file uploads, allowing clients to upload files directly using the signed URL. Files are stored on disk.

## Features

- **Signed URL Generation**: Generate time-limited signed URLs for secure file uploads
- **Disk Storage**: Files stored on the local filesystem
- **Database**: SQLite for tracking file metadata
- **Cache**: Redis for storing upload tokens
- **HTTP Server**: Standardized routing with multiple authentication methods
- **Logger**: Structured logging with custom format
- **Errors**: Standardized error responses

## How It Works

1. An admin creates a client using Bearer token auth (returns client_id and client_secret)
2. The client uses Basic auth (client_id:client_secret) to request a signed URL with file metadata
3. The service records the file metadata in the database and stores an upload token in Redis with 15-minute TTL
4. The service returns a signed URL containing the token
5. The client uses the signed URL to upload the file directly (no auth header needed — the token is embedded in the URL)
6. The service validates the token against Redis, stores the file on disk, and deletes the token (one-time use)

## Prerequisites

- Go 1.19+
- Redis server running locally on port 6379

## Database

### Creating migrations

```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go --command create-migration --name <migration_name> --dir database/migrations
```

## API Endpoints

### Public Endpoints
- `GET /health` - Health check (no auth required)
- `POST /files/upload?token=<token>` - Upload file using signed URL token (no auth header)
- `GET /files/download?token=<token>` - Download file using signed URL token (no auth header)

### Protected Endpoints

#### Client Management (Bearer Auth)
Admin-only endpoints using `Authorization: Bearer secret-token`.

- `POST /clients` - Create a new client (returns credentials once)
- `GET /clients` - List all clients (without secrets)
- `GET /clients/{id}` - Get a specific client by ID (without secret)

#### File Operations (Basic Auth)
Client endpoints using `Authorization: Basic <base64(client_id:client_secret)>`.

- `POST /files/signed-url` - Generate a signed URL for file upload (valid for 15 minutes)
- `POST /files/download-url` - Generate a signed URL for file download (valid for 15 minutes)

## Authentication

The service supports three authentication methods:

### 1. Bearer Token (Admin Only)
For client management operations. Uses a static secret token.

```
Authorization: Bearer secret-token
```

### 2. Basic Auth (Clients)
For file operations. Uses client_id and client_secret from the created client.

```bash
# Encode credentials
export CREDENTIALS=$(echo -n "client_id:client_secret" | base64)

# Use in request
Authorization: Basic $CREDENTIALS
```

### 3. URL Token (Upload / Download)
For file uploads and downloads. The token is embedded in the signed URL, no auth header required.

```
POST /files/upload?token=<upload-token>
GET  /files/download?token=<download-token>
```

## Request/Response Examples

### Create Client (Bearer Auth)
```bash
curl -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-service-client"}'
```

**Response (201 Created):**
```json
{
  "id": 1,
  "name": "my-service-client",
  "client_id": "client_...",
  "client_secret": "secret_...",
  "created_at": "2026-02-23T10:00:00Z",
  "updated_at": "2026-02-23T10:00:00Z"
}
```

**Note:** The `client_secret` is only returned once during creation. Save it securely.

### List Clients (Bearer Auth)
```bash
curl -X GET http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token"
```

**Response (200 OK):**
```json
[
  {
    "id": 1,
    "name": "my-service-client",
    "client_id": "client_...",
    "created_at": "2026-02-23T10:00:00Z",
    "updated_at": "2026-02-23T10:00:00Z"
  }
]
```

### Get Client by ID (Bearer Auth)
```bash
curl -X GET http://localhost:8080/clients/1 \
  -H "Authorization: Bearer secret-token"
```

**Response (200 OK):**
```json
{
  "id": 1,
  "name": "my-service-client",
  "client_id": "client_...",
  "created_at": "2026-02-23T10:00:00Z",
  "updated_at": "2026-02-23T10:00:00Z"
}
```

### Generate Signed URL (Basic Auth)
```bash
# Encode client_id:client_secret in base64
export CREDENTIALS=$(echo -n "your-client-id:your-client-secret" | base64)

curl -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

**Response (201 Created):**
```json
{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "signed_url": "http://localhost:8080/files/upload?token=abc123...",
  "expires_at": "2026-02-23T10:15:00Z"
}
```

**Note:** Save the `signed_url` — it can be used to upload the file within 15 minutes without additional authentication.

### Upload File (Token in URL - No Auth Header)
```bash
# Use the signed_url from the previous response
curl -X POST "http://localhost:8080/files/upload?token=YOUR_TOKEN" \
  -F "file=@./document.pdf"
```

**Response (200 OK):**
```json
{
  "message": "File uploaded successfully",
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "file_name": "document.pdf",
  "file_size": 1048576,
  "saved_path": "./uploads/550e8400-e29b-41d4-a716-446655440000"
}
```

### Generate Download Signed URL (Basic Auth)
```bash
export CREDENTIALS=$(echo -n "your-client-id:your-client-secret" | base64)

curl -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "550e8400-e29b-41d4-a716-446655440000"}'
```

**Response (201 Created):**
```json
{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "signed_url": "http://localhost:8080/files/download?token=xyz789...",
  "expires_at": "2026-02-23T10:15:00Z"
}
```

**Note:** Only the client that uploaded the file can generate a download URL for it. The token is valid for 15 minutes and can only be used once.

### Download File (Token in URL - No Auth Header)
```bash
# Use the signed_url from the previous response
curl -X GET "http://localhost:8080/files/download?token=YOUR_TOKEN" \
  --output document.pdf
```

The response is the raw file binary streamed with headers:
```
Content-Type: application/pdf
Content-Disposition: attachment; filename="document.pdf"
```

## Running the Service

1. **Start Redis:**
   ```bash
   redis-server
   ```

2. **Start the service:**
   ```bash
   export PATH=$PATH:/usr/local/go/bin
   go run main.go
   ```

**Check health:**
```bash
curl http://localhost:8080/health
```

## Database

The service uses SQLite with a local file `./file_upload_service.db`.

### Schema

**clients table:**
- `id` - Primary key
- `name` - Client name
- `client_id` - Unique client identifier (auto-generated)
- `client_secret` - Client secret for authentication (auto-generated)
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp

**files table:**
- `id` - UUID primary key
- `file_name` - Original file name
- `file_size` - File size in bytes
- `mimetype` - MIME type of the file
- `client_id` - ID of the client who created the file
- `owner_entity_type` - Type of entity that owns the file (e.g., "user", "organization")
- `owner_entity_id` - ID of the owning entity
- `created_at` - Creation timestamp
- `updated_at` - Last update timestamp
- `deleted_at` - Soft delete timestamp (nullable)

## Architecture

```
main.go              - Service entry point
server/              - HTTP server setup and route registration
handlers/            - Request handlers
models/              - Data models
database/            - Database initialization and migrations
cache/               - Cache initialization (Redis)
test/                - Test documentation with curl commands
```

## Error Responses

Standardized error responses using the errs package:

```json
{
  "Code": 404,
  "Message": "Client not found"
}
```