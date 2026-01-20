# API Gateway with Rate Limiting & Authentication

## 🎯 Project Overview

A production-ready API Gateway built from scratch in Go featuring:
- **API Key Authentication** with expiration support
- **Token Bucket Rate Limiting** per client
- **Reverse Proxy** with intelligent routing
- **Service Registry** for dynamic service discovery
- **Graceful Shutdown** and proper error handling
- **Health Checks** and monitoring

## 📁 What You've Got

```
api-gateway/
├── cmd/gateway/main.go              # Main entry point
├── internal/
│   ├── auth/
│   │   ├── apikey.go               # API key management & storage
│   │   └── middleware.go           # Authentication middleware
│   ├── ratelimit/
│   │   ├── limiter.go             # Token bucket rate limiter
│   │   └── middleware.go           # Rate limiting middleware
│   ├── proxy/
│   │   └── proxy.go               # HTTP reverse proxy
│   ├── registry/
│   │   └── registry.go            # Service registry & routing
│   └── config/
│       └── config.go              # Configuration management
├── test-backend.go                 # Simple test backend service
├── test.sh                        # Automated test suite
├── Makefile                       # Build automation
├── README.md                      # Comprehensive documentation
└── QUICKSTART.md                  # Quick start guide
```

## 🚀 Quick Start

### 1. Set Up the Module

```bash
cd api-gateway
go mod init github.com/YOUR_USERNAME/api-gateway
```

**Important:** Update the import paths in `cmd/gateway/main.go` to match your module name.

### 2. Add to Workspace

From your `distributed-systems` root:

```bash
go work use ./api-gateway
go mod tidy
```

### 3. Run the Gateway

```bash
# Option 1: Using Make
make run

# Option 2: Direct
go run cmd/gateway/main.go
```

The gateway will start on `http://localhost:8080`

### 4. Test It Out

```bash
# Health check (no auth needed)
curl http://localhost:8080/health

# Make authenticated request
curl -H "X-API-Key: gw_test_key_1" http://localhost:8080/api/users
```

## 🔑 Key Features Explained

### 1. API Key Authentication

**How it works:**
- Client sends API key in `X-API-Key` header
- Middleware validates key exists and hasn't expired
- Valid keys add user context to request
- Invalid/missing keys return 401

**Sample keys provided:**
- `gw_test_key_1` (user_1)
- `gw_test_key_2` (user_2)

**Create new keys:**
```go
store := auth.NewAPIKeyStore()
key, err := store.CreateAPIKey("user_123", nil) // never expires
```

### 2. Rate Limiting (Token Bucket)

**Algorithm:**
- Each API key gets a "bucket" of tokens (burst capacity)
- Tokens refill at a constant rate (requests per minute / 60)
- Each request consumes 1 token
- No tokens = 429 Too Many Requests

**Default limits:**
- 60 requests per minute
- Burst size of 10 tokens

**Rate limit headers:**
```
X-RateLimit-Limit: 10
X-RateLimit-Remaining: 7
```

### 3. Service Registry

**Path-based routing:**
- Routes requests to backend services based on path prefix
- Supports multiple backend services
- Longest prefix match wins

**Example config:**
```go
Services: []ServiceConfig{
    {
        Name:        "user-service",
        PathPrefix:  "/api/users",
        UpstreamURL: "http://localhost:8081",
        Timeout:     30 * time.Second,
    },
}
```

Request to `/api/users/123` → `http://localhost:8081/123`

### 4. Reverse Proxy

**Features:**
- Preserves request method, headers, body
- Adds `X-Forwarded-*` headers
- Strips hop-by-hop headers
- Handles timeouts and errors gracefully
- Returns upstream response to client

## 🧪 Testing

### Run the Test Suite

```bash
chmod +x test.sh
./test.sh
```

Tests cover:
- ✅ Health checks
- ✅ Missing API keys
- ✅ Invalid API keys  
- ✅ Valid authentication
- ✅ Rate limiting
- ✅ Separate rate limits per key
- ✅ Rate limit headers
- ✅ Non-existent routes

### Manual Testing with Backend

**Terminal 1 - Start test backend:**
```bash
go run test-backend.go
# Runs on :8081
```

**Terminal 2 - Start gateway:**
```bash
make run
# Runs on :8080
```

**Terminal 3 - Make requests:**
```bash
# Update config to route /api/test to localhost:8081
# Then make requests:
curl -H "X-API-Key: gw_test_key_1" http://localhost:8080/api/test
```

You'll see the request flow through both services!

## 📊 Request Flow

```
1. Client → Gateway
   ↓
2. Auth Middleware
   - Validates X-API-Key header
   - Adds user context
   ↓
3. Rate Limit Middleware  
   - Checks token bucket for API key
   - Consumes token or rejects (429)
   ↓
4. Service Registry
   - Matches path to upstream service
   - Returns service config
   ↓
5. Proxy
   - Builds upstream request
   - Adds X-Forwarded-* headers
   - Forwards to backend
   ↓
6. Backend Service
   - Processes request
   - Returns response
   ↓
7. Proxy → Client
   - Returns backend response
```

## 🔧 Configuration

Edit `internal/config/config.go`:

```go
config := &Config{
    Gateway: GatewayConfig{
        Host: "localhost",
        Port: 8080,
    },
    Services: []ServiceConfig{
        {
            Name:        "your-service",
            PathPrefix:  "/api/your-service",
            UpstreamURL: "http://localhost:8081",
            Timeout:     30 * time.Second,
        },
    },
    RateLimit: RateLimitConfig{
        RequestsPerMinute: 60,   // Adjust as needed
        BurstSize:         10,   // Adjust as needed
    },
}
```

## 🛠️ Development Commands

```bash
make help          # Show all available commands
make build         # Build binary
make run           # Run from source
make test          # Run tests
make fmt           # Format code
make clean         # Clean build artifacts
```

## 📈 What's Next?

### Immediate Enhancements
1. **Persistent Storage:** Move API keys from memory to database
2. **Logging:** Add structured logging (logrus, zap)
3. **Metrics:** Track requests, latency, errors (Prometheus)
4. **Health Checks:** Ping backend services periodically

### Advanced Features
1. **Circuit Breaker:** Protect against cascading failures
2. **Load Balancing:** Multiple instances per service
3. **JWT Support:** In addition to API keys
4. **WebSocket Proxying:** Support real-time connections
5. **Request/Response Transforms:** Modify headers, bodies
6. **Caching:** Cache upstream responses
7. **Distributed Rate Limiting:** Use Redis for multi-instance deployments

## 🎓 Learning Highlights

This project demonstrates:
- **Middleware Pattern:** Composable request processing
- **Concurrency:** Thread-safe rate limiting with sync.RWMutex
- **Interface Design:** Loose coupling between components
- **HTTP Proxying:** Request forwarding and header management
- **Token Bucket Algorithm:** Fair rate limiting
- **Graceful Shutdown:** Proper cleanup on termination
- **Go Project Structure:** Clean architecture with internal packages

## 📝 Key Implementation Details

### Thread Safety
- API key store uses `sync.RWMutex` for concurrent reads/writes
- Rate limiter uses separate mutexes per bucket
- Service registry is thread-safe

### Performance
- Lazy bucket creation (only when needed)
- HTTP client connection reuse
- Efficient header copying

### Error Handling
- Proper context timeout handling
- Gateway timeout vs bad gateway errors
- Informative error messages

## 🚀 Production Readiness Checklist

Before deploying to production:

- [ ] Replace in-memory storage with database (PostgreSQL, MySQL)
- [ ] Add Redis for distributed rate limiting
- [ ] Implement structured logging
- [ ] Add metrics and monitoring (Prometheus/Grafana)
- [ ] Enable HTTPS/TLS
- [ ] Add circuit breakers
- [ ] Implement health checking for backends
- [ ] Set up distributed tracing (Jaeger, Zipkin)
- [ ] Add request ID tracking
- [ ] Configure proper timeouts
- [ ] Set up alerts
- [ ] Add admin API for management
- [ ] Implement configuration hot-reload
- [ ] Add comprehensive tests
- [ ] Set up CI/CD pipeline

## 📚 Documentation

- **README.md** - Full feature documentation
- **QUICKSTART.md** - Installation and setup guide
- **This file** - Project overview and summary

## 🎉 You've Built

A fully functional API Gateway with:
- ✅ Secure authentication
- ✅ Fair rate limiting
- ✅ Intelligent routing
- ✅ Request proxying
- ✅ Health monitoring
- ✅ Production-ready structure

Great work! This is a solid foundation for building distributed systems.

---

**Next Project Ideas:**
- Service Mesh
- Message Queue
- Distributed Cache
- Load Balancer
- Service Discovery

Keep building! 🚀