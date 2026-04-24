# API Gateway

A reusable API gateway written in Go with authentication, authorization, rate limiting, caching, and reverse proxy capabilities.

## Features

- **JWT Authentication** — verifies tokens on every protected route
- **Role-based Access Control** — gates routes by role (`user`, `admin`, `service`)
- **Token Bucket Rate Limiting** — per-user on authenticated routes, per-IP on public routes, backed by Redis
- **Response Caching** — Redis-backed caching for GET requests with configurable TTL
- **Reverse Proxy** — forwards requests to upstream services with circuit breakers
- **Request Validation** — enforces JSON content-type, max body size, valid JSON bodies
- **Security Headers** — sets common defensive headers (CSP, X-Frame-Options, etc.)
- **Observability** — Prometheus metrics, structured logging, request ID correlation
- **Circuit Breaker** — per-upstream failure tracking with automatic recovery

## Stack

| Concern | Tool |
|---------|------|
| Router | [Chi](https://github.com/go-chi/chi) |
| JWT | [golang-jwt/jwt](https://github.com/golang-jwt/jwt) |
| Rate limiting | Custom token bucket with Redis |
| Cache | Redis |
| Metrics | [Prometheus](https://github.com/prometheus/client_golang) |
| Proxy | Go stdlib `httputil.ReverseProxy` |

## Project Structure

```
api-gateway/
├── cmd/
│   └── main.go               # entry point
├── internal/
│   ├── cache/
│   │   └── cache.go         # Redis caching layer
│   ├── config/
│   │   └── config.go         # env config
│   ├── middleware/
│   │   ├── auth.go           # JWT + RBAC middleware
│   │   ├── context.go        # context helpers
│   │   ├── logger.go         # structured logging
│   │   ├── metrics.go        # Prometheus metrics
│   │   ├── requestid.go      # request ID generation
│   │   └── validator.go      # body & header validation
│   ├── metrics/
│   │   └── metrics.go       # Prometheus metrics definitions
│   ├── proxy/
│   │   ├── proxy.go          # reverse proxy with circuit breaker
│   │   ├── config.go         # route parsing
│   │   └── circuitbreaker.go # circuit breaker implementation
│   └── server/
│       └── server.go         # router + middleware wiring
```

## Getting Started

```bash
# install dependencies
go mod tidy

# run (Redis must be running)
go run cmd/main.go
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `JWT_SECRET` | `change-me-in-production` | Secret used to sign/verify JWTs |
| `JWT_EXPIRY` | `15m` | Token expiry duration |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password |
| `RATE_LIMIT` | `100` | Authenticated requests per window |
| `RATE_REFILL` | `60` | Requests added per minute |
| `IP_RATE_LIMIT` | `20` | Public requests per window |
| `IP_RATE_REFILL` | `20` | Requests added per minute |
| `MAX_BODY_BYTES` | `1048576` | Max request body size (1MB) |
| `CB_MAX_FAILURES` | `5` | Circuit breaker failure threshold |
| `CB_RESET_TIMEOUT` | `30s` | Circuit breaker reset duration |
| `CACHE_ENABLED` | `false` | Enable response caching |
| `CACHE_TTL` | `5m` | Cache TTL duration |
| `ROUTES` | `/api/users:http://localhost:3001:5,/api/posts:http://localhost:3002:5` | Comma-separated `prefix:upstream:timeout` |

## Routes

| Method | Path | Auth | Role |
|--------|------|------|------|
| GET | `/health` | No | — |
| POST | `/auth/login` | No | — |
| GET | `/me` | Yes | `user` |
| GET | `/admin/dashboard` | Yes | `admin` |
| GET | `/internal/stats` | Yes | `service` |
| GET | `/metrics` | No | — | Prometheus metrics endpoint |

All other routes are proxied to upstream services defined in `ROUTES`.

## Token Format

```json
{
  "user_id": "42",
  "roles": ["user"],
  "exp": 1234567890,
  "iat": 1234567890
}
```

Signed with HS256.

## Prometheus Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|------------|
| `gateway_requests_total` | Counter | method, path, status | Total requests |
| `gateway_request_duration_seconds` | Histogram | method, path | Request latency |
| `gateway_rate_limit_hits_total` | Counter | type | Rate limit rejections |
| `gateway_upstream_requests_total` | Counter | upstream, status | Upstream requests |
| `gateway_upstream_duration_seconds` | Histogram | upstream | Upstream latency |
| `gateway_circuit_breaker_state` | Gauge | upstream | Circuit breaker state |
| `gateway_cache_hits_total` | Counter | path | Cache hits |
| `gateway_cache_misses_total` | Counter | path | Cache misses |

## Implemented

- [x] JWT authentication
- [x] Role-based authorization
- [x] Token bucket rate limiting (Redis-backed)
- [x] Request validation (JSON, body size)
- [x] Reverse proxy with routing
- [x] Circuit breaker per upstream
- [x] Response caching (GET only, Redis)
- [x] Prometheus metrics
- [x] Security headers
- [x] Structured logging (slog)
- [x] Request ID correlation