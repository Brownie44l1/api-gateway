package main

import (
    "fmt"
    "net/http"

    "github.com/Brownie44l1/api-gateway/internal/config"
    "github.com/Brownie44l1/api-gateway/internal/server"
)

func main() {
    cfg := config.Load()
    srv := server.New(cfg)

    fmt.Println("Gateway running on port", cfg.Port)
    http.ListenAndServe(":"+cfg.Port, srv)
}