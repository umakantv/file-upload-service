# File Signed URL API Tests

These tests cover the file upload signed URL generation endpoint.

## Prerequisites

1. Start Redis server locally:
```bash
redis-server
```

2. Start the file upload service:
```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go
```

3. Create a client to get client_id and client_secret (see `clients.md`).

---

## Authentication for File Operations

File operations use **Basic auth** with `client_id:client_secret` encoded in base64.

```bash
# Encode client_id:client_secret in base64
export CREDENTIALS=$(echo -n "your-client-id:your-client-secret" | base64)
```

Then use: `Authorization: Basic $CREDENTIALS`

---

## 1. Generate Signed URL

Generate a signed URL for uploading a file. The URL is valid for 15 minutes.

### Request
```bash
# First, encode your client credentials
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
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

### Expected Response (201 Created)
```json
{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "signed_url": "http://localhost:8080/files/upload?token=abc123...",
  "expires_at": "2026-02-23T10:15:00Z"
}
```

**Note:** Save the `signed_url` — it can be used to upload the file within 15 minutes without additional authentication.

---

## 2. Validation Error - Missing file_name

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "file_name is required"
}
```

---

## 3. Validation Error - Invalid file_size

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 0,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "file_size must be greater than 0"
}
```

---

## 4. Validation Error - Missing mimetype

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "mimetype is required"
}
```

---

## 5. Validation Error - Missing owner_entity_type

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "owner_entity_type is required"
}
```

---

## 6. Validation Error - Missing owner_entity_id

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user"
  }'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "owner_entity_id is required"
}
```

---

## 7. Unauthorized Request - No Auth

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (401 Unauthorized)
```
Unauthorized
```

---

## 8. Unauthorized Request - Invalid Credentials

### Request
```bash
# Use invalid credentials
export CREDENTIALS=$(echo -n "invalid-client:invalid-secret" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
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

### Expected Response (401 Unauthorized)
```
Unauthorized
```

---

## 9. Wrong Auth Type - Bearer Token Not Allowed

Bearer tokens are only for client management, not file operations.

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (401 Unauthorized)
```
Unauthorized
```