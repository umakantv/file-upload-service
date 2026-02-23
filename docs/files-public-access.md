# Public File Access API Tests

These tests cover the public file access endpoint that allows serving files without authentication.

Public file access is configured per bucket via `public_paths` — an array of path patterns that specify which files can be accessed publicly. Patterns support wildcards (`*`) for flexible matching.

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

3. A bucket with `public_paths` configured and files uploaded. See the full workflow below.

---

## How Public Access Works

1. **Configure public paths on a bucket**: Set `public_paths` to an array of patterns like `["images/*", "*.jpg", "public/*"]`
2. **Upload files** using the signed URL flow (see `files-signed-url.md` and `files-upload.md`)
3. **Access files directly** via `GET /files/{bucket_name}/{file_path}` — no authentication required
4. **CORS is enforced**: If the bucket has CORS policy configured, the appropriate headers will be returned

---

## Pattern Matching Rules

- `*` matches any sequence of characters **except `/`**
- Patterns are matched against the full file key (path within the bucket)
- Examples:
  - `"images/*"` matches `images/photo.jpg` but not `images/subfolder/photo.jpg`
  - `"images/**/*"` or `"images/*/*"` would match nested paths (if supported)
  - `"*.jpg"` matches any `.jpg` file in the root of the bucket
  - `"public/*"` matches all files in the `public/` folder (not recursive)
  - `"*"` matches any file in the bucket (use with caution)

---

## 1. Create Bucket with Public Paths

### Request
```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-public-bucket",
    "cors_policy": [
      {
        "AllowedOrigins": ["https://example.com", "https://app.example.com"],
        "AllowedMethods": ["GET"],
        "AllowedHeaders": ["*"],
        "ExposeHeaders": ["Content-Type", "Content-Length"]
      }
    ],
    "public_paths": ["images/*", "*.jpg", "*.png"]
  }'
```

### Expected Response (201 Created)
```json
{
  "id": 1,
  "name": "my-public-bucket",
  "client_id": "client_...",
  "cors_policy": [...],
  "public_paths": ["images/*", "*.jpg", "*.png"],
  "archived": false,
  "created_at": "2026-02-23T10:00:00Z",
  "updated_at": "2026-02-23T10:00:00Z"
}
```

---

## 2. Update Bucket to Add Public Paths

### Request
```bash
curl -s -X PUT http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "public_paths": ["catalog/*", "banners/*", "*.svg"]
  }'
```

### Expected Response (200 OK)
```json
{
  "id": 1,
  "name": "my-public-bucket",
  "client_id": "client_...",
  "cors_policy": [...],
  "public_paths": ["catalog/*", "banners/*", "*.svg"],
  "archived": false,
  "created_at": "2026-02-23T10:00:00Z",
  "updated_at": "2026-02-23T10:05:00Z"
}
```

---

## 3. Upload a File to a Public Path

### Step 1: Generate Signed URL
```bash
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "bucket_id": 1,
    "key": "images/product-photo.jpg",
    "file_name": "product-photo.jpg",
    "file_size": 1048576,
    "mimetype": "image/jpeg",
    "owner_entity_type": "product",
    "owner_entity_id": "prod-123"
  }'
```

Save the `signed_url` from the response.

### Step 2: Upload the File
```bash
curl -s -X POST "http://localhost:8080/files/upload?token=<TOKEN>" \
  -F "file=@./product-photo.jpg"
```

---

## 4. Access Public File (No Authentication)

Once uploaded, the file can be accessed directly without any authentication:

### Request
```bash
curl -s -X GET "http://localhost:8080/files/my-public-bucket/images/product-photo.jpg" \
  --output downloaded-photo.jpg
```

### Response Headers
```
HTTP/1.1 200 OK
Content-Type: image/jpeg
Content-Length: 1048576
Cache-Control: public, max-age=3600
Access-Control-Allow-Origin: https://example.com
Vary: Origin
```

The file is streamed directly. No JSON response body — just the raw file content.

---

## 5. Access Public File from Browser (CORS)

When accessing from a web page, the browser sends an `Origin` header:

### Request
```bash
curl -s -X GET "http://localhost:8080/files/my-public-bucket/images/product-photo.jpg" \
  -H "Origin: https://example.com" \
  -I
```

### Expected Response Headers
```
HTTP/1.1 200 OK
Content-Type: image/jpeg
Access-Control-Allow-Origin: https://example.com
Vary: Origin
Access-Control-Allow-Methods: GET
Access-Control-Allow-Headers: *
Access-Control-Expose-Headers: Content-Type, Content-Length
```

---

## 6. Error Cases

### File Not in Public Paths (403 Forbidden)

If the file path doesn't match any public path pattern:

```bash
# Assuming public_paths is ["images/*"] and we try to access "private/data.txt"
curl -s -X GET "http://localhost:8080/files/my-public-bucket/private/data.txt"
```

**Expected Response (403 Forbidden):**
```json
{
  "Code": 403,
  "Message": "File is not publicly accessible"
}
```

---

### Bucket Not Found (404 Not Found)

```bash
curl -s -X GET "http://localhost:8080/files/non-existent-bucket/images/photo.jpg"
```

**Expected Response (404 Not Found):**
```json
{
  "Code": 404,
  "Message": "Bucket not found"
}
```

---

### File Not Found (404 Not Found)

```bash
# Bucket exists, but file doesn't
curl -s -X GET "http://localhost:8080/files/my-public-bucket/images/non-existent.jpg"
```

**Expected Response (404 Not Found):**
```json
{
  "Code": 404,
  "Message": "File not found"
}
```

---

### Archived Bucket (404 Not Found)

Archived buckets don't serve public files:

```bash
curl -s -X GET "http://localhost:8080/files/archived-bucket/images/photo.jpg"
```

**Expected Response (404 Not Found):**
```json
{
  "Code": 404,
  "Message": "Bucket not found"
}
```

---

### CORS Not Allowed (No CORS Headers)

If the origin doesn't match the CORS policy:

```bash
curl -s -X GET "http://localhost:8080/files/my-public-bucket/images/photo.jpg" \
  -H "Origin: https://unauthorized-site.com" \
  -I
```

**Expected Response Headers:**
```
HTTP/1.1 200 OK
Content-Type: image/jpeg
Cache-Control: public, max-age=3600
# No Access-Control-Allow-Origin header
```

The browser will block the request due to CORS policy.

---

## Full Workflow Test

```bash
# Step 1: Create a client (Bearer auth)
curl -s -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-public-client"}' | tee /tmp/client.json

export CLIENT_ID=$(cat /tmp/client.json | grep -o '"client_id":"[^"]*"' | cut -d'"' -f4)
export CLIENT_SECRET=$(cat /tmp/client.json | grep -o '"client_secret":"[^"]*"' | cut -d'"' -f4)
export CREDENTIALS=$(echo -n "$CLIENT_ID:$CLIENT_SECRET" | base64)

# Step 2: Create a bucket with public paths (Basic auth)
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "catalog-images",
    "cors_policy": [
      {
        "AllowedOrigins": ["*"],
        "AllowedMethods": ["GET"],
        "AllowedHeaders": ["*"],
        "ExposeHeaders": ["Content-Type"]
      }
    ],
    "public_paths": ["products/*", "banners/*"]
  }' | tee /tmp/bucket.json

export BUCKET_ID=$(cat /tmp/bucket.json | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)

# Step 3: Generate upload signed URL (Basic auth)
curl -s -X POST http://localhost:8080/files/signed-url \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d "{
    \"bucket_id\": $BUCKET_ID,
    \"key\": \"products/t-shirt-red.jpg\",
    \"file_name\": \"t-shirt-red.jpg\",
    \"file_size\": 1048576,
    \"mimetype\": \"image/jpeg\",
    \"owner_entity_type\": \"product\",
    \"owner_entity_id\": \"prod-456\"
  }" | tee /tmp/upload_url.json

export UPLOAD_TOKEN=$(cat /tmp/upload_url.json | grep -o '"signed_url":"[^"]*"' | grep -o 'token=[^"]*' | cut -d= -f2)

# Step 4: Upload the file (no auth - token in URL)
curl -s -X POST "http://localhost:8080/files/upload?token=$UPLOAD_TOKEN" \
  -F "file=@./sample-product.jpg"

# Step 5: Access the file publicly (no auth required)
curl -s -X GET "http://localhost:8080/files/catalog-images/products/t-shirt-red.jpg" \
  --output ./downloaded-product.jpg

echo "Download complete. Verifying..."
ls -lh ./downloaded-product.jpg

# Step 6: Test CORS headers
curl -s -X GET "http://localhost:8080/files/catalog-images/products/t-shirt-red.jpg" \
  -H "Origin: https://myshop.com" \
  -I
```

---

## Use Cases

### E-commerce Product Images
```json
{
  "public_paths": ["products/*", "thumbnails/*"]
}
```
Access: `http://localhost:8080/files/shop-bucket/products/shoes-nike-123.jpg`

### User Avatars
```json
{
  "public_paths": ["avatars/*"]
}
```
Access: `http://localhost:8080/files/user-bucket/avatars/user-456.png`

### Marketing Banners
```json
{
  "public_paths": ["banners/*", "promotions/*"]
}
```
Access: `http://localhost:8080/files/marketing-bucket/banners/summer-sale-2024.jpg`

### All Images Public
```json
{
  "public_paths": ["*.jpg", "*.png", "*.gif", "*.svg", "*.webp"]
}
```
Access: `http://localhost:8080/files/media-bucket/any-image.png`

### Everything Public (Use with Caution)
```json
{
  "public_paths": ["*"]
}
```
Access: `http://localhost:8080/files/public-bucket/any/path/to/file.pdf`