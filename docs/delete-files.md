# Delete Files Endpoint Tests

These tests cover deleting files by file IDs **or** by bucket path. The endpoint removes files from disk and marks them deleted in the database.

**Two modes (mutually exclusive):**
- `file_ids` — delete specific files by ID
- `bucket_id` + `path` — delete all files under a path in a bucket (recursive)

## Prerequisites

1. Start Redis locally.
2. Start the file upload service.
3. Create a client, bucket, and upload files with nested keys. Capture file IDs from upload responses or list results.

---

## 1. Delete Files by IDs Successfully

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>" \
  -H "Content-Type: application/json" \
  -d '{"file_ids": ["<FILE_ID_1>", "<FILE_ID_2>"]}'
```

### Example
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_ids": ["550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440001"]}'
```

### Expected Response (200 OK)
```json
{
  "deleted": [
    "550e8400-e29b-41d4-a716-446655440000",
    "550e8400-e29b-41d4-a716-446655440001"
  ],
  "missing": [],
  "failed": []
}
```

---

## 2. Delete With Missing IDs

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>" \
  -H "Content-Type: application/json" \
  -d '{"file_ids": ["<MISSING_ID>"]}'
```

### Expected Response (200 OK)
```json
{
  "deleted": [],
  "missing": [
    "<MISSING_ID>"
  ],
  "failed": []
}
```

---

## 3. Delete Without Any Parameters

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "Either file_ids or (bucket_id and path) is required"
}
```

---

## 4. Delete Files by Bucket Path

Delete all files recursively under a given path in a bucket.

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>" \
  -H "Content-Type: application/json" \
  -d '{"bucket_id": <BUCKET_ID>, "path": "<PATH>"}'
```

### Example
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"bucket_id": 1, "path": "reports/2024"}'
```

### Expected Response (200 OK)
```json
{
  "deleted": [
    "550e8400-e29b-41d4-a716-446655440002",
    "550e8400-e29b-41d4-a716-446655440003"
  ],
  "missing": [],
  "failed": []
}
```

---

## 5. Delete by Path — Path Not Found

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"bucket_id": 1, "path": "nonexistent/path"}'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "No files found at the given path"
}
```

---

## 6. Delete by Path — Missing bucket_id

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"path": "reports/2024"}'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "bucket_id is required when path is provided"
}
```

---

## 7. Both file_ids and path Provided

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"file_ids": ["some-id"], "bucket_id": 1, "path": "reports"}'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "file_ids and path cannot be used together"
}
```

---

## 8. Delete by Path — Bucket Not Owned by Client

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic <BASE64_OTHER_CLIENT>" \
  -H "Content-Type: application/json" \
  -d '{"bucket_id": 1, "path": "reports"}'
```

### Expected Response (403 Forbidden)
```json
{
  "Code": 403,
  "Message": "Access denied: bucket does not belong to your account"
}
```

---

## 9. Delete by Path — Archived Bucket

### Request
```bash
curl -s -X DELETE "http://localhost:8080/files" \
  -H "Authorization: Basic $CREDENTIALS" \
  -H "Content-Type: application/json" \
  -d '{"bucket_id": <ARCHIVED_BUCKET_ID>, "path": "reports"}'
```

### Expected Response (409 Conflict)
```json
{
  "Code": 422,
  "Message": "Cannot delete files in an archived bucket"
}
```
