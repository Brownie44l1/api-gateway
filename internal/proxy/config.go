package proxy

import (
	"fmt"
	"strconv"
	"strings"
)

// Route maps an incoming path prefix to an upstream service URL.
type Route struct {
	Prefix   string
	Upstream string
	Timeout  int // seconds
}

// Config holds all route definitions for the gateway.
type Config struct {
	Routes []Route
}

// ParseRoutes parses the ROUTES env var format:
// "prefix:upstream:timeout,prefix:upstream:timeout"
// e.g. "/api/users:http://localhost:3001:5,/api/posts:http://localhost:3002:5"
func ParseRoutes(raw string) (*Config, error) {
	cfg := &Config{}
	if raw == "" {
		return cfg, nil
	}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		// split on ":" but be careful — upstream URLs contain ":"
		// format is always: prefix:scheme://host:port:timeout
		// so we split from the right to get the timeout, then rejoin the rest
		parts := strings.Split(entry, ":")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid route %q: want prefix:upstream:timeout", entry)
		}

		// last part is timeout, everything before is "prefix:upstream"
		timeoutStr := parts[len(parts)-1]
		timeout, err := strconv.Atoi(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout in route %q: %w", entry, err)
		}

		// rejoin prefix and upstream (upstream may contain ":" in the URL)
		prefixAndUpstream := strings.Join(parts[:len(parts)-1], ":")

		// prefix is first segment, upstream is the rest
		idx := strings.Index(prefixAndUpstream, ":")
		if idx == -1 {
			return nil, fmt.Errorf("invalid route %q: missing upstream", entry)
		}

		prefix := prefixAndUpstream[:idx]
		upstream := prefixAndUpstream[idx+1:]

		cfg.Routes = append(cfg.Routes, Route{
			Prefix:   prefix,
			Upstream: upstream,
			Timeout:  timeout,
		})
	}

	return cfg, nil
}
