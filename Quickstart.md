# API Gateway - Quick Start Guide

## Installation Steps

### 1. Run the setup script

From the current directory (`/home/claude`):

```bash
chmod +x setup.sh
./setup.sh
```

This will create the proper directory structure and move all files to their correct locations.

### 2. Navigate to the api-gateway directory

```bash
cd api-gateway
```

### 3. Initialize the Go module

Replace `yourusername` with your actual GitHub username:

```bash
go mod init github.com/yourusername/api-gateway
```

### 4. Update the imports in main.go

Edit `cmd/gateway/main.go` and replace `yourusername` in the import paths with your actual GitHub username.

### 5. Add to Go workspace

From the distributed-systems root:

```bash
go work use ./api-gateway
```

### 6. Tidy dependencies

```bash
go mod tidy
```

### 7. Run the gateway

```bash
make run
```

Or directly:

```bash
go run cmd/gateway/main.go
```

## Testing the Gateway

### 1. Start a test backend service

In a new terminal:

```bash
# Simple Python HTTP server
python3 -m http.server 8081
```

Or create a simple Go service:

```go
// test-service.go
package main

import (
    "fmt"
    "log"
    "net/http"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello from backend service! Path: %s\n", r.URL.Path)
    })
    log.Println("Test service running on :8081")
    log.Fatal(http.ListenAndServe(":8081", nil))
}
```

```bash
go run test-service.go
```

### 2. Update config to point to your backend

Edit `internal/config/config.go` and add your service:

```go
Services: []ServiceConfig{
    {
        Name:        "test-service",
        PathPrefix:  "/api/test",
        UpstreamURL: "http://localhost:8081",
        Timeout:     30 * time.Second,
    },
},
```

### 3. Make requests through the gateway

```bash
# Health check (no auth required)
curl http://localhost:8080/health

# Request to backend (requires API key)
curl -H "X-API-Key: gw_test_key_1" http://localhost:8080/api/test

# Without API key (should fail with 401)
curl http://localhost:8080/api/test

# Test rate limiting (make 15+ rapid requests)
for i in {1..15}; do
    curl -H "X-API-Key: gw_test_key_1" http://localhost:8080/api/test
    echo ""
done
```

## Project Structure After Setup

```
api-gateway/
├── cmd/
│   └── gateway/
│       └── main.go                 # Entry point
├── internal/
│   ├── auth/
│   │   ├── apikey.go              # API key management
│   │   └── middleware.go           # Auth middleware
│   ├── ratelimit/
│   │   ├── limiter.go             # Token bucket rate limiter
│   │   └── middleware.go           # Rate limit middleware
│   ├── proxy/
│   │   └── proxy.go               # HTTP reverse proxy
│   ├── registry/
│   │   └── registry.go            # Service registry
│   └── config/
│       └── config.go              # Configuration
├── go.mod
├── Makefile
└── README.md
```

## Common Issues

### Import errors

Make sure you've:
1. Replaced `yourusername` in import paths with your GitHub username
2. Run `go mod tidy`
3. Added the module to the workspace with `go work use ./api-gateway`

### Connection refused

Make sure your backend service is running on the port specified in the config.

### Rate limiting too aggressive

Adjust the rate limit in `internal/config/config.go`:

```go
RateLimit: RateLimitConfig{
    RequestsPerMinute: 120,  // Increase this
    BurstSize:         20,   // And this
},
```

## Next Steps

1. **Add your own services** - Edit `internal/config/config.go`
2. **Create real API keys** - Use the API key store programmatically
3. **Add logging** - Implement request/response logging
4. **Add metrics** - Track request counts, latencies, errors
5. **Add circuit breaker** - Protect against failing services
6. **Add health checks** - Periodically check backend health

## Development Tips

```bash
# Format code
make fmt

# Run linter
make lint

# Run tests
make test

# Build binary
make build

# Clean artifacts
make clean
```

## API Key Management

Create new API keys programmatically:

```go
import "github.com/yourusername/api-gateway/internal/auth"

store := auth.NewAPIKeyStore()

// Create permanent key
key, err := store.CreateAPIKey("user_123", nil)

// Create expiring key (30 days)
expiresAt := time.Now().Add(30 * 24 * time.Hour)
key, err := store.CreateAPIKey("user_456", &expiresAt)

// Revoke key
err = store.RevokeAPIKey(key.Key)
```

## Production Considerations

Before deploying to production:

1. Store API keys in a database (currently in-memory)
2. Use Redis for distributed rate limiting
3. Add TLS/HTTPS support
4. Implement proper logging and monitoring
5. Add circuit breakers for resilience
6. Set up health checks for backend services
7. Configure proper timeouts
8. Add request ID tracking
9. Implement distributed tracing
10. Set up metrics and alerting

Enjoy building with your API Gateway! 🚀