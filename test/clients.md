# Client API Tests

These tests cover the client management endpoints. Clients are IAM-like entities used for authentication with the file upload service.

## Prerequisites

Start the server:
```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go
```

---

## 1. Create Client

Create a new client with auto-generated credentials.

### Request
```bash
curl -s -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{"name": "my-service-client"}'
```

### Expected Response (201 Created)
```json
{
  "id": 1,
  "name": "my-service-client",
  "client_id": "client_...",
  "client_secret": "secret_...",
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

**Note:** The `client_secret` is only returned once during creation. Save it securely.

---

## 2. List All Clients

Get a list of all clients (without secrets).

### Request
```bash
curl -s -X GET http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token"
```

### Expected Response (200 OK)
```json
[
  {
    "id": 1,
    "name": "my-service-client",
    "client_id": "client_...",
    "created_at": "2026-02-23T...",
    "updated_at": "2026-02-23T..."
  }
]
```

---

## 3. Get Client by ID

Get a specific client by its database ID (without secret).

### Request
```bash
curl -s -X GET http://localhost:8080/clients/1 \
  -H "Authorization: Bearer secret-token"
```

### Expected Response (200 OK)
```json
{
  "id": 1,
  "name": "my-service-client",
  "client_id": "client_...",
  "created_at": "2026-02-23T...",
  "updated_at": "2026-02-23T..."
}
```

---

## 4. Get Non-Existent Client

Test error handling for a client that doesn't exist.

### Request
```bash
curl -s -X GET http://localhost:8080/clients/99999 \
  -H "Authorization: Bearer secret-token"
```

### Expected Response (404 Not Found)
```json
{
  "Code": 404,
  "Message": "Client not found"
}
```

---

## 5. Create Client Without Name (Validation Error)

Test validation by omitting the required `name` field.

### Request
```bash
curl -s -X POST http://localhost:8080/clients \
  -H "Authorization: Bearer secret-token" \
  -H "Content-Type: application/json" \
  -d '{}'
```

### Expected Response (400 Bad Request)
```json
{
  "Code": 422,
  "Message": "Name is required"
}
```

---

## 6. Unauthorized Request

Test that endpoints require authentication.

### Request
```bash
curl -s -X GET http://localhost:8080/clients
```

### Expected Response (401 Unauthorized)
```
Unauthorized
```