# File Download API Tests

These tests cover the file download signed URL generation and the download endpoint.

## Prerequisites

1. Redis server running:
```bash
redis-server
```

2. Service running:
```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go
```

3. A file must already have been uploaded. Use `clients.md`, `files-signed-url.md`, and `files-upload.md` to create a client and upload a file first. Note the `file_id` returned.

---

## Authentication

Download URL generation uses **Basic auth** with `client_id:client_secret`.

```bash
export CREDENTIALS=$(echo -n "your-client-id:your-client-secret" | base64)
```

---

## 1. Generate Download Signed URL

Request a signed URL to download a specific file. Valid for 15 minutes.

### Request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "d8055fb1-5699-43b4-9ce1-d11fe60894d4"}'
```

### Expected Response (201 Created)
```json
{
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "signed_url": "http://localhost:8080/files/download?token=abc123...",
  "expires_at": "2026-02-23T10:15:00Z"
}
```

**Note:** The token embedded in `signed_url` is valid for 15 minutes and can only be used once.

---

## 2. Download File Using Signed URL

Use the token from the signed URL to download the file. No auth header required.

### Request
```bash
# Replace <TOKEN> with the token from the signed URL
curl -s -X GET "http://localhost:8080/files/download?token=<TOKEN>" \
  --output downloaded-file.pdf
```

### Example
```bash
curl -s -X GET "http://localhost:8080/files/download?token=102c69213109d22661c87b4d17a09a839f06eadb227431178971e604ebdbdfd7" \
  --output downloaded-file.pdf
```

The response is the raw file binary streamed with headers:
```
Content-Type: application/pdf
Content-Disposition: attachment; filename="document.pdf"
```

**Note:** The token is deleted after the first successful download (one-time use).

---

## Full Workflow

```bash
# Step 1: Create a client (Bearer auth)
curl -s -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-service"}' | tee /tmp/client.json

# Extract credentials
export CLIENT_ID=$(cat /tmp/client.json | grep -o '"client_id":"[^"]*"' | cut -d'"' -f4)
export CLIENT_SECRET=$(cat /tmp/client.json | grep -o '"client_secret":"[^"]*"' | cut -d'"' -f4)
export CREDENTIALS=$(echo -n "$CLIENT_ID:$CLIENT_SECRET" | base64)

# Step 2: Generate upload signed URL (Basic auth)
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "sample.pdf",
    "file_size": 10485760,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }' | tee /tmp/upload_url.json

export FILE_ID=$(cat /tmp/upload_url.json | grep -o '"file_id":"[^"]*"' | cut -d'"' -f4)
export UPLOAD_TOKEN=$(cat /tmp/upload_url.json | grep -o '"signed_url":"[^"]*"' | grep -o 'token=[^"]*' | cut -d= -f2)

# Step 3: Upload the file (no auth - token in URL)
curl -s -X POST "http://localhost:8080/files/upload?token=$UPLOAD_TOKEN" \
  -F "file=@./sample.pdf"

# Step 4: Generate download signed URL (Basic auth)
curl -s -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d "{\"file_id\": \"$FILE_ID\"}" | tee /tmp/download_url.json

export DOWNLOAD_TOKEN=$(cat /tmp/download_url.json | grep -o '"signed_url":"[^"]*"' | grep -o 'token=[^"]*' | cut -d= -f2)

# Step 5: Download the file (no auth - token in URL)
curl -s -X GET "http://localhost:8080/files/download?token=$DOWNLOAD_TOKEN" \
  --output ./downloaded-sample.pdf

echo "Download complete. Verifying..."
ls -lh ./downloaded-sample.pdf
```

---

## 3. Error Cases

### Missing file_id in request
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Expected Response (400 Bad Request):**
```json
{"Code": 422, "Message": "file_id is required"}
```

---

### File not found (wrong file_id)
```bash
export CREDENTIALS=$(echo -n "client_xxx:secret_xxx" | base64)

curl -s -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "non-existent-id"}'
```

**Expected Response (404 Not Found):**
```json
{"Code": 404, "Message": "File not found"}
```

---

### Access denied (file belongs to a different client)
```bash
# Use credentials of a different client than the one that uploaded the file
export OTHER_CREDENTIALS=$(echo -n "other-client-id:other-secret" | base64)

curl -s -X POST http://localhost:8080/files/download-url \
  -H "Authorization: Basic $OTHER_CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "550e8400-e29b-41d4-a716-446655440000"}'
```

**Expected Response (403 Forbidden):**
```json
{"Code": 403, "Message": "Access denied"}
```

---

### Invalid or expired download token
```bash
curl -s -X GET "http://localhost:8080/files/download?token=invalidtoken123"
```

**Expected Response (401 Unauthorized):**
```json
{"Code": 401, "Message": "Invalid or expired download token"}
```

---

### Missing token in URL
```bash
curl -s -X GET "http://localhost:8080/files/download"
```

**Expected Response (400 Bad Request):**
```json
{"Code": 422, "Message": "Missing download token"}
```

---

### Token can only be used once
```bash
# First download succeeds
curl -s -X GET "http://localhost:8080/files/download?token=<TOKEN>" --output file1.pdf

# Second download with same token fails
curl -s -X GET "http://localhost:8080/files/download?token=<TOKEN>"
```

**Expected second response (401 Unauthorized):**
```json
{"Code": 401, "Message": "Invalid or expired download token"}
```

---

### Unauthorized - no auth header
```bash
curl -s -X POST http://localhost:8080/files/download-url \
  -H "Content-Type: application/json" \
  -d '{"file_id": "550e8400-e29b-41d4-a716-446655440000"}'
```

**Expected Response (401 Unauthorized):**
```
Unauthorized
```
