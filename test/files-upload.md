# File Upload Endpoint Tests

These tests cover the file upload endpoint that uses the token from the URL (no auth header required).

Every upload is associated with a bucket. The bucket is chosen at signed URL generation time and is carried through the upload token.

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

3. Create a client (see `clients.md`), create a bucket (see `buckets.md`), then generate a signed URL (see `files-signed-url.md`) to get a token.

---

## 1. Upload File Successfully

Upload a file using the signed URL token.

### Request
```bash
# Replace <TOKEN> with the actual token from the signed URL response
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>" \
  -F "file=@/path/to/your/file.pdf"
```

### Example
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=102c69213109d22661c87b4d17a09a839f06eadb227431178971e604ebdbdfd7" \
  -F "file=@./test-document.pdf"
```

### Expected Response (200 OK)
```json
{
  "message": "File uploaded successfully",
  "file_id": "550e8400-e29b-41d4-a716-446655440000",
  "file_name": "document.pdf",
  "file_size": 1048576,
  "bucket_id": 1,
  "saved_path": "./uploads/550e8400-e29b-41d4-a716-446655440000"
}
```

**Note:** The token is deleted after successful upload (one-time use).

---

## 2. Upload Without Token

### Request
```bash
curl -s -X POST http://localhost:8080/files/upload \
  -F "file=@./test-document.pdf"
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "Missing upload token"
}
```

---

## 3. Upload With Invalid/Expired Token

### Request
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=invalid-token-123" \
  -F "file=@./test-document.pdf"
```

### Expected Response (401 Unauthorized)
```json
{
  "Code": 401,
  "Message": "Invalid or expired upload token"
}
```

---

## 4. Upload Without File

### Request
```bash
# Replace <TOKEN> with a valid token
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>"
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "Missing file in upload"
}
```

---

## 5. Upload File Exceeding Size Limit

If the uploaded file is larger than the `file_size` specified when generating the signed URL:

### Request
```bash
# Replace <TOKEN> with a valid token (generated with file_size: 1024)
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>" \
  -F "file=@./large-file.pdf"
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "File size exceeds allowed limit"
}
```

---

## 6. Token Can Only Be Used Once

After a successful upload, the same token cannot be reused.

### First Request (Success)
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>" \
  -F "file=@./test-document.pdf"
```

### Second Request (Same Token - Fails)
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>" \
  -F "file=@./test-document.pdf"
```

### Expected Response (401 Unauthorized)
```json
{
  "Code": 401,
  "Message": "Invalid or expired upload token"
}
```

---

## Full Workflow Test

Here's the complete workflow from client creation to file upload:

### Step 1: Create a Client (Bearer auth)
```bash
curl -s -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-upload-client"}' | tee /tmp/client.json

export CLIENT_ID=$(cat /tmp/client.json | grep -o '"client_id":"[^"]*"' | cut -d'"' -f4)
export CLIENT_SECRET=$(cat /tmp/client.json | grep -o '"client_secret":"[^"]*"' | cut -d'"' -f4)
export CREDENTIALS=$(echo -n "$CLIENT_ID:$CLIENT_SECRET" | base64)
```

### Step 2: Create a Bucket (Basic auth)
```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-uploads"}' | tee /tmp/bucket.json

export BUCKET_ID=$(cat /tmp/bucket.json | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
```

### Step 3: Generate Signed URL (Basic auth)
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d "{
    \"bucket_id\": $BUCKET_ID,
    \"file_name\": \"document.pdf\",
    \"file_size\": 10485760,
    \"mimetype\": \"application/pdf\",
    \"owner_entity_type\": \"user\",
    \"owner_entity_id\": \"user-123\"
  }" | tee /tmp/upload_url.json

export UPLOAD_TOKEN=$(cat /tmp/upload_url.json | grep -o '"signed_url":"[^"]*"' | grep -o 'token=[^"]*' | cut -d= -f2)
```

### Step 4: Upload File (No auth - token in URL)
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=$UPLOAD_TOKEN" \
  -F "file=@./sample.pdf"
```