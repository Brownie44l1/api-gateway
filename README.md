# api-gateway

A personal, reusable API gateway written in Go. Drop it in front of any project and get authentication, authorization, and rate limiting out of the box.

## What it does

- **JWT authentication** — verifies tokens on every protected route, rejects invalid or expired ones
- **Role-based access control** — gates routes by role (`user`, `admin`, `service`)
- **Token bucket rate limiting** — per-user on authenticated routes, per-IP on public routes, backed by Redis
- Fails open on Redis downtime so your API stays available

## Stack

| Concern | Tool |
|---|---|
| Router | [Chi](https://github.com/go-chi/chi) |
| JWT | [golang-jwt/jwt](https://github.com/golang-jwt/jwt) |
| Rate limiting | [Brownie44l1/rate-limiter](https://github.com/Brownie44l1/rate-limiter) |
| State store | Redis |

## Project structure

```
api-gateway/
├── cmd/
│   └── main.go               # entry point
├── internal/
│   ├── config/
│   │   └── config.go         # env config
│   ├── middleware/
│   │   ├── auth.go           # JWT + RBAC middleware
│   │   └── context.go        # context helpers
│   └── server/
│       └── server.go         # router + middleware wiring
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | Server port |
| `JWT_SECRET` | `change-me-in-production` | Secret used to sign and verify JWTs |
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | `` | Redis password (leave empty if none) |

## Getting started

```bash
# install dependencies
go mod tidy

# run (Redis must be running)
go run cmd/main.go
```

## Routes

| Method | Path | Auth | Role |
|---|---|---|---|
| GET | `/health` | No | — |
| POST | `/auth/login` | No | — |
| GET | `/me` | Yes | `user` |
| GET | `/admin/dashboard` | Yes | `admin` |
| GET | `/internal/stats` | Yes | `service` |

## Rate limits

| Route type | Limit | Refill | Key |
|---|---|---|---|
| Public | 20 requests | 20/min | IP address |
| Authenticated | 100 requests | 60/min | User ID |

Responses include `X-RateLimit-Limit`, `X-RateLimit-Remaining`, and `X-RateLimit-Reset` headers on every request. A `Retry-After` header is added on `429` responses.

## Token format

```json
{
  "user_id": "42",
  "roles": ["user"],
  "exp": 1234567890,
  "iat": 1234567890
}
```

Access tokens expire in 15 minutes. Signed with HS256.

## Phases completed

- [x] Phase 1 — JWT authentication
- [x] Phase 2 — Role-based authorization
- [x] Phase 3 — Token bucket rate limiting
- [ ] Phase 4 — Request validation
- [ ] Phase 5 — Routing & proxy
- [ ] Phase 6 — Caching
- [ ] Phase 7 — Observability & logging
- [ ] Phase 8 — Security hardening
- [ ] Phase 9 — Config & DX polish