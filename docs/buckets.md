# Bucket API Tests

These tests cover the bucket management endpoints. Buckets are scoped to a client and are authenticated using Basic auth (client_id:client_secret).

## Prerequisites

1. Start the server:
```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go
```

2. Create a client first (see `clients.md`) and export the credentials:
```bash
# Replace with values returned by POST /clients
export CLIENT_ID="client_..."
export CLIENT_SECRET="secret_..."
export BASIC_AUTH=$(echo -n "$CLIENT_ID:$CLIENT_SECRET" | base64)
```

---

## 1. Create a Bucket (minimal — no CORS policy)

Create a bucket with just a name; `cors_policy` defaults to an empty array.

### Request
```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-bucket"}'
```

### Expected Response (201 Created)
```json
{
  "id": 1,
  "name": "my-bucket",
  "client_id": "client_...",
  "cors_policy": [],
  "archived": false,
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

---

## 2. Create a Bucket with CORS Policy

Create a bucket with a full CORS policy configuration.

### Request
```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-cors-bucket",
    "cors_policy": [
      {
        "AllowedHeaders": ["Authorization", "Content-Type"],
        "AllowedMethods": ["GET", "HEAD", "PUT"],
        "AllowedOrigins": ["http://www.example.com", "https://app.example.com"],
        "ExposeHeaders": ["Access-Control-Allow-Origin", "ETag"]
      }
    ]
  }'
```

### Expected Response (201 Created)
```json
{
  "id": 2,
  "name": "my-cors-bucket",
  "client_id": "client_...",
  "cors_policy": [
    {
      "AllowedHeaders": ["Authorization", "Content-Type"],
      "AllowedMethods": ["GET", "HEAD", "PUT"],
      "AllowedOrigins": ["http://www.example.com", "https://app.example.com"],
      "ExposeHeaders": ["Access-Control-Allow-Origin", "ETag"]
    }
  ],
  "archived": false,
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

---

## 3. List All Buckets

Retrieve all buckets belonging to the authenticated client.

### Request
```bash
curl -s -X GET http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH"
```

### Expected Response (200 OK)
```json
[
  {
    "id": 2,
    "name": "my-cors-bucket",
    "client_id": "client_...",
    "cors_policy": [...],
    "archived": false,
    "created_at": "2026-02-23T...",
    "updated_at": "2026-02-23T..."
  },
  {
    "id": 1,
    "name": "my-bucket",
    "client_id": "client_...",
    "cors_policy": [],
    "archived": false,
    "created_at": "2026-02-23T...",
    "updated_at": "2026-02-23T..."
  }
]
```

---

## 4. Get a Bucket by ID

Retrieve a single bucket by its database ID.

### Request
```bash
curl -s -X GET http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $BASIC_AUTH"
```

### Expected Response (200 OK)
```json
{
  "id": 1,
  "name": "my-bucket",
  "client_id": "client_...",
  "cors_policy": [],
  "archived": false,
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

---

## 5. Update a Bucket's CORS Policy

Replace the CORS policy for an existing (non-archived) bucket.

### Request
```bash
curl -s -X PUT http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{
    "cors_policy": [
      {
        "AllowedHeaders": ["*"],
        "AllowedMethods": ["GET", "HEAD"],
        "AllowedOrigins": ["https://trusted.example.com"],
        "ExposeHeaders": []
      }
    ]
  }'
```

### Expected Response (200 OK)
```json
{
  "id": 1,
  "name": "my-bucket",
  "client_id": "client_...",
  "cors_policy": [
    {
      "AllowedHeaders": ["*"],
      "AllowedMethods": ["GET", "HEAD"],
      "AllowedOrigins": ["https://trusted.example.com"],
      "ExposeHeaders": []
    }
  ],
  "archived": false,
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

### Clear the CORS policy (set to empty array)
```bash
curl -s -X PUT http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"cors_policy": []}'
```

---

## 6. Archive a Bucket

Mark a bucket as archived. Archived buckets cannot be updated.

### Request
```bash
curl -s -X POST http://localhost:8080/buckets/1/archive \
  -H "Authorization: Basic $BASIC_AUTH"
```

### Expected Response (200 OK)
```json
{
  "id": 1,
  "name": "my-bucket",
  "client_id": "client_...",
  "cors_policy": [...],
  "archived": true,
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

---

## 7. Error Cases

### 7a. Duplicate Bucket Name (409 Conflict)

Attempting to create a bucket whose name already exists for the same client.

```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-bucket"}'
```

**Expected Response (409 Conflict)**
```json
{
  "Code": 422,
  "Message": "A bucket with this name already exists for your account"
}
```

### 7b. Invalid Bucket Name (400 Bad Request)

Names must be alphanumeric with dashes; they cannot start or end with a dash.

```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "-invalid-name-"}'
```

**Expected Response (400 Bad Request)**
```json
{
  "Code": 422,
  "Message": "name must be alphanumeric with dashes (cannot start or end with a dash)"
}
```

### 7c. Invalid CORS Policy (400 Bad Request)

```bash
curl -s -X POST http://localhost:8080/buckets \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"name": "bad-cors-bucket", "cors_policy": "not-an-array"}'
```

**Expected Response (400 Bad Request)**
```json
{
  "Code": 422,
  "Message": "cors_policy must be a valid JSON array of CORS rules"
}
```

### 7d. Update an Archived Bucket (409 Conflict)

```bash
# Archive bucket 1 first (see section 6), then try to update it:
curl -s -X PUT http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $BASIC_AUTH" \
  -H "Content-Type: application/json" \
  -d '{"cors_policy": []}'
```

**Expected Response (409 Conflict)**
```json
{
  "Code": 422,
  "Message": "Cannot update an archived bucket"
}
```

### 7e. Archive an Already-Archived Bucket (409 Conflict)

```bash
curl -s -X POST http://localhost:8080/buckets/1/archive \
  -H "Authorization: Basic $BASIC_AUTH"
```

**Expected Response (409 Conflict)**
```json
{
  "Code": 422,
  "Message": "Bucket is already archived"
}
```

### 7f. Bucket Not Found (404)

```bash
curl -s -X GET http://localhost:8080/buckets/99999 \
  -H "Authorization: Basic $BASIC_AUTH"
```

**Expected Response (404 Not Found)**
```json
{
  "Code": 404,
  "Message": "Bucket not found"
}
```

### 7g. Unauthorized Request (401)

```bash
curl -s -X GET http://localhost:8080/buckets
```

**Expected Response (401 Unauthorized)**
```
Unauthorized
```

### 7h. Cross-client isolation

A client cannot see or modify another client's buckets. If client B tries to access a bucket owned by client A using client A's bucket ID, they will receive a 404 (not found) rather than a 403 — the bucket simply doesn't appear to exist for them.

```bash
# Using a different client's credentials:
export OTHER_BASIC_AUTH=$(echo -n "other_client_id:other_client_secret" | base64)

curl -s -X GET http://localhost:8080/buckets/1 \
  -H "Authorization: Basic $OTHER_BASIC_AUTH"
```

**Expected Response (404 Not Found)**
```json
{
  "Code": 404,
  "Message": "Bucket not found"
}
```
