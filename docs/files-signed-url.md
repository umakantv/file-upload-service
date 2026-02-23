# File Signed URL API Tests

These tests cover the file upload signed URL generation endpoint.

Every file must be associated with a **bucket**. The bucket must belong to the authenticated client and must not be archived.

Files are stored using a client-provided **key** (similar to AWS S3 object keys). The key determines the path within the client/bucket folder and may contain slashes for deeper nesting (e.g. `invoices/2024/january/receipt.pdf`). The key is stored in the database and the full resolved path `<client_name>/<bucket_name>/<key>` is carried in the signed URL token.

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

## 1. Generate Signed URL (flat key)

Generate a signed URL for uploading a file with a simple flat key. The URL is valid for 15 minutes.

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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
The file will be stored at `./uploads/<client_name>/<bucket_name>/document.pdf`.

---

## 2. Generate Signed URL (nested key)

Use a slash-delimited key to store the file in a deeper nested path — just like AWS S3 object keys.

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "invoices/2024/january/receipt.pdf",
    "file_name": "receipt.pdf",
    "file_size": 204800,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

### Expected Response (201 Created)
```json
{
  "file_id": "661f9511-f30c-52e5-b827-557766551111",
  "signed_url": "http://localhost:8080/files/upload?token=def456...",
  "expires_at": "2026-02-23T10:15:00Z"
}
```

The file will be stored at `./uploads/<client_name>/<bucket_name>/invoices/2024/january/receipt.pdf`.

---

## 3. Validation Error - Missing bucket_id

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "document.pdf",
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

## 4. Validation Error - Missing key

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

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "key is required"
}
```

---

## 5. Validation Error - Bucket Not Found

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 99999,
    "key": "document.pdf",
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

## 6. Validation Error - Bucket Belongs to Another Client (403 Forbidden)

```bash
# Use a bucket_id that belongs to a different client
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 2,
    "key": "document.pdf",
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

## 7. Validation Error - Bucket is Archived (409 Conflict)

```bash
# Use the id of a bucket that has been archived
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 8. Validation Error - Missing file_name

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 9. Validation Error - Invalid file_size

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 10. Validation Error - Missing mimetype

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 11. Validation Error - Missing owner_entity_type

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 12. Validation Error - Missing owner_entity_id

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 13. Unauthorized Request - No Auth

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 14. Unauthorized Request - Invalid Credentials

```bash
export BAD_CREDENTIALS=$(echo -n "invalid-client:invalid-secret" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $BAD_CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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

## 15. Wrong Auth Type - Bearer Token Not Allowed

Bearer tokens are only for client management, not file operations.

```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "document.pdf",
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