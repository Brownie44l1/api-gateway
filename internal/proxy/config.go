package proxy

// Route maps an incoming path prefix to an upstream service URL.
type Route struct {
	Prefix string
	Upstream string
	Timeout int
}

// Config holds all route definitions for the gateway.
type Config struct {
	Routes []Route
}

func Default() *Config {
    return &Config{
        Routes: []Route{
            {
                Prefix:   "/api/users",
                Upstream: "http://localhost:3001",
                Timeout:  5,
            },
            {
                Prefix:   "/api/posts",
                Upstream: "http://localhost:3002",
                Timeout:  5,
            },
        },
    }
}