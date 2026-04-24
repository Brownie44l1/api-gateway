package proxy

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Brownie44l1/api-gateway/internal/cache"
	"github.com/Brownie44l1/api-gateway/internal/metrics"
)

// Proxy holds a reverse proxy and circuit breaker per upstream route.
type Proxy struct {
	routes   []Route
	breakers map[string]*CircuitBreaker
	proxies  map[string]*httputil.ReverseProxy
	cache    *cache.Cache
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
	Cache          *cache.Cache
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
		cache:          opts.Cache,
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
			metrics.RateLimitHits.WithLabelValues("circuit_breaker").Inc()
			http.Error(w, `{"error":"service temporarily unavailable"}`, http.StatusServiceUnavailable)
			return
		}

		p.serveWithCache(w, r, route.Prefix, rp, breaker)
	}
}

func (p *Proxy) serveWithCache(w http.ResponseWriter, r *http.Request, prefix string, rp *httputil.ReverseProxy, breaker *CircuitBreaker) {
	path := r.URL.Path
	query := r.URL.RawQuery

	if p.cache != nil && r.Method == "GET" {
		if cached, ok := p.cache.Get(r.Context(), path, query); ok {
			metrics.CacheHits.WithLabelValues(path).Inc()
			for k, v := range cached.Headers {
				w.Header().Set(k, v)
			}
			w.WriteHeader(cached.StatusCode)
			w.Write([]byte(cached.Body))
			return
		}
		metrics.CacheMisses.WithLabelValues(path).Inc()
	}

	wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
	upstream := ""
	if rp.Director != nil {
		upstream = prefix
	}
	start := time.Now()
	rp.ServeHTTP(wrapped, r)
	elapsed := time.Since(start).Seconds()

	if upstream != "" {
		metrics.UpstreamRequests.WithLabelValues(upstream, fmt.Sprintf("%d", wrapped.statusCode)).Inc()
		metrics.UpstreamLatency.WithLabelValues(upstream).Observe(elapsed)
	}

	if wrapped.statusCode >= 500 {
		breaker.Failure()
	} else {
		breaker.Success()
	}

	metrics.CircuitBreakerState.WithLabelValues(prefix).Set(float64(breaker.State()))

	if p.cache != nil && r.Method == "GET" && wrapped.statusCode == 200 && wrapped.body != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		p.cache.Set(ctx, path, query, &cache.Response{
			Body:       string(wrapped.body),
			StatusCode: wrapped.statusCode,
			Headers:    map[string]string{},
			Timestamp:  time.Now(),
		})
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

// responseWriter wraps http.ResponseWriter to capture the status code and body.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.body = append(rw.body, b...)
	return rw.ResponseWriter.Write(b)
}
