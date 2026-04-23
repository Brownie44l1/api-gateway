package proxy

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Proxy holds a reverse proxy and circuit breaker per upstream route.
type Proxy struct {
	routes   []Route
	breakers map[string]*CircuitBreaker
	proxies  map[string]*httputil.ReverseProxy
	mu       sync.RWMutex

	cbMaxFailures  int
	cbResetTimeout time.Duration
	defaultTimeout time.Duration
}

// Options configures Proxy behaviour sourced from config.
type Options struct {
	CBMaxFailures  int
	CBResetTimeout time.Duration
	DefaultTimeout time.Duration // fallback when route.Timeout == 0
}

// New builds a Proxy from a Config.
// Each route gets its own circuit breaker and reverse proxy instance.
func New(cfg *Config, opts Options) (*Proxy, error) {
	if opts.DefaultTimeout == 0 {
		opts.DefaultTimeout = 5 * time.Second
	}
	if opts.CBMaxFailures == 0 {
		opts.CBMaxFailures = 5
	}
	if opts.CBResetTimeout == 0 {
		opts.CBResetTimeout = 30 * time.Second
	}

	p := &Proxy{
		routes:         cfg.Routes,
		breakers:       make(map[string]*CircuitBreaker),
		proxies:        make(map[string]*httputil.ReverseProxy),
		cbMaxFailures:  opts.CBMaxFailures,
		cbResetTimeout: opts.CBResetTimeout,
		defaultTimeout: opts.DefaultTimeout,
	}

	for _, route := range cfg.Routes {
		target, err := url.Parse(route.Upstream)
		if err != nil {
			return nil, fmt.Errorf("invalid upstream URL %s: %w", route.Upstream, err)
		}

		timeout := time.Duration(route.Timeout) * time.Second
		if timeout == 0 {
			timeout = p.defaultTimeout
		}

		rp := httputil.NewSingleHostReverseProxy(target)

		rp.Transport = &http.Transport{
			ResponseHeaderTimeout: timeout,
		}

		// capture route for closure
		r := route
		rp.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
			slog.Error("upstream error", "upstream", r.Upstream, "err", err)
			p.breakers[r.Prefix].Failure()
			http.Error(w, `{"error":"upstream service unavailable"}`, http.StatusBadGateway)
		}

		rp.Director = func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			req.URL.Path = strings.TrimPrefix(req.URL.Path, r.Prefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}

			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
			req.Header.Set("X-Forwarded-Host", req.Header.Get("Host"))
		}

		p.breakers[route.Prefix] = NewCircuitBreaker(opts.CBMaxFailures, opts.CBResetTimeout)
		p.proxies[route.Prefix] = rp
	}

	return p, nil
}

// Handler returns an http.HandlerFunc that matches the request path
// to a route and forwards it.
func (p *Proxy) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		route, rp, breaker := p.match(r.URL.Path)

		if route == nil {
			http.Error(w, `{"error":"no route found"}`, http.StatusNotFound)
			return
		}

		if !breaker.Allow() {
			http.Error(w, `{"error":"service temporarily unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		rp.ServeHTTP(wrapped, r)

		if wrapped.statusCode >= 500 {
			breaker.Failure()
		} else {
			breaker.Success()
		}
	}
}

// match finds the longest prefix that matches the request path.
func (p *Proxy) match(path string) (*Route, *httputil.ReverseProxy, *CircuitBreaker) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var matched *Route
	for i := range p.routes {
		route := &p.routes[i]
		if strings.HasPrefix(path, route.Prefix) {
			if matched == nil || len(route.Prefix) > len(matched.Prefix) {
				matched = route
			}
		}
	}

	if matched == nil {
		return nil, nil, nil
	}

	return matched, p.proxies[matched.Prefix], p.breakers[matched.Prefix]
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
