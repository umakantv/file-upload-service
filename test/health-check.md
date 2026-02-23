# Health Check

Test that the service is running and healthy.

## Start the server

```bash
export PATH=$PATH:/usr/local/go/bin
go run main.go
```

## Test

```bash
curl -s http://localhost:8080/health
```

### Expected Response

**Status:** `200 OK`

```json
{"status": "healthy", "service": "file-upload-service"}
```
