# File Upload Endpoint Tests

These tests cover the file upload endpoint that uses the token from the URL (no auth header required).

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

3. Create a client and get credentials (see `clients.md`), then generate a signed URL (see `files-signed-url.md`) to get a token.

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
  -d '{"name": "my-upload-client"}'
```

Save the `client_id` and `client_secret` from the response.

### Step 2: Generate Signed URL (Basic auth)
```bash
# Encode client_id:client_secret in base64
export CREDENTIALS=$(echo -n "client_id:client_secret" | base64)

curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "file_name": "document.pdf",
    "file_size": 10485760,
    "mimetype": "application/pdf",
    "owner_entity_type": "user",
    "owner_entity_id": "user-123"
  }'
```

Save the `signed_url` from the response.

### Step 3: Upload File (No auth - token in URL)
```bash
# Extract token from signed_url (the part after ?token=)
curl -s -X POST "http://localhost:8080/files/upload?token=EXTRACTED_TOKEN" \
  -F "file=@./document.pdf"
```