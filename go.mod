module github.com/Brownie44l1/api-gateway

go 1.24.0

toolchain go1.24.12

replace github.com/Brownie44l1/http => ../http

require (
	github.com/Brownie44l1/http v0.0.0-20260121033342-f4a5e86521ab
	github.com/golang-jwt/jwt/v5 v5.3.0
	github.com/redis/go-redis/v9 v9.17.2
	golang.org/x/crypto v0.47.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
)
