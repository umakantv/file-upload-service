# List Files Endpoint Tests

These tests cover listing files in a bucket at a given path. The response returns files directly in that path and folder names for the next level only (non-recursive).

## Prerequisites

1. Start Redis locally.
2. Start the file upload service.
3. Create a client, create a bucket, and upload a few files with keys that include nested paths.

---

## 1. List Root Path

List files and folders at the bucket root.

### Request
```bash
curl -s -X GET "http://localhost:8080/buckets/<BUCKET_ID>/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>"
```

### Example
```bash
curl -s -X GET "http://localhost:8080/buckets/1/files" \
  -H "Authorization: Basic $CREDENTIALS"
```

### Expected Response (200 OK)
```json
{
  "bucket_id": 1,
  "path": "",
  "files": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "key": "invoice.pdf",
      "file_name": "invoice.pdf",
      "file_size": 1048576,
      "mimetype": "application/pdf",
      "created_at": "2026-02-24T00:00:00Z"
    }
  ],
  "folders": [
    "reports",
    "uploads"
  ]
}
```

---

## 2. List Nested Path

List files inside a folder (non-recursive).

### Request
```bash
curl -s -X GET "http://localhost:8080/buckets/<BUCKET_ID>/files?path=reports/2024" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>"
```

### Example
```bash
curl -s -X GET "http://localhost:8080/buckets/1/files?path=reports/2024" \
  -H "Authorization: Basic $CREDENTIALS"
```

### Expected Response (200 OK)
```json
{
  "bucket_id": 1,
  "path": "reports/2024",
  "files": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440001",
      "key": "reports/2024/summary.pdf",
      "file_name": "summary.pdf",
      "file_size": 2048,
      "mimetype": "application/pdf",
      "created_at": "2026-02-24T00:01:00Z"
    }
  ],
  "folders": [
    "images"
  ]
}
```

---

## 3. List Path Without Access

Attempt to list a bucket owned by another client.

### Request
```bash
curl -s -X GET "http://localhost:8080/buckets/<BUCKET_ID>/files" \
  -H "Authorization: Basic <BASE64_OTHER_CLIENT>"
```

### Expected Response (403 Forbidden)
```json
{
  "Code": 403,
  "Message": "Access denied: bucket does not belong to your account"
}
```

---

## 4. List Archived Bucket

### Request
```bash
curl -s -X GET "http://localhost:8080/buckets/<BUCKET_ID>/files" \
  -H "Authorization: Basic <BASE64_CLIENT_ID_SECRET>"
```

### Expected Response (409 Conflict)
```json
{
  "Code": 422,
  "Message": "Cannot list files in an archived bucket"
}
```
