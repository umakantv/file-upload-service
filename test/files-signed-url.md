# File Signed URL API Tests

These tests cover the file upload signed URL generation endpoint.

Every file must be associated with a **bucket**. The bucket must belong to the authenticated client and must not be archived.

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

3. Create a client (see `clients.md`) and a bucket (see `buckets.md`), then export credentials:
```bash
export CLIENT_ID="client_..."
export CLIENT_SECRET="secret_..."
export CREDENTIALS=$(echo -n "$CLIENT_ID:$CLIENT_SECRET" | base64)
export BUCKET_ID=1   # use the id returned when creating the bucket
```

---

## Authentication for File Operations

File operations use **Basic auth** with `client_id:client_secret` encoded in base64.

```bash
export CREDENTIALS=$(echo -n "your-client-id:your-client-secret" | base64)
```

Then use: `Authorization: Basic $CREDENTIALS`

---

## 1. Generate Signed URL

Generate a signed URL for uploading a file. The URL is valid for 15 minutes.

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 2. Validation Error - Missing bucket_id

### Request
```bash
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

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "bucket_id is required and must be a positive integer"
}
```

---

## 3. Validation Error - Bucket Not Found

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 99999,
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (404 Not Found)
```json
{
  "Code": 404,
  "Message": "Bucket not found"
}
```

---

## 4. Validation Error - Bucket Belongs to Another Client (403 Forbidden)

### Request
```bash
# Use a bucket_id that belongs to a different client
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 2,
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (403 Forbidden)
```json
{
  "Code": 403,
  "Message": "Access denied: bucket does not belong to your account"
}
```

---

## 5. Validation Error - Bucket is Archived (409 Conflict)

### Request
```bash
# Use the id of a bucket that has been archived
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "file_name": "document.pdf",
    "file_size": 1048576,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (409 Conflict)
```json
{
  "Code": 422,
  "Message": "Cannot upload to an archived bucket"
}
```

---

## 6. Validation Error - Missing file_name

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 7. Validation Error - Invalid file_size

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 8. Validation Error - Missing mimetype

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 9. Validation Error - Missing owner_entity_type

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 10. Validation Error - Missing owner_entity_id

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 11. Unauthorized Request - No Auth

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 12. Unauthorized Request - Invalid Credentials

### Request
```bash
export BAD_CREDENTIALS=$(echo -n "invalid-client:invalid-secret" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $BAD_CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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

## 13. Wrong Auth Type - Bearer Token Not Allowed

Bearer tokens are only for client management, not file operations.

### Request
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
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