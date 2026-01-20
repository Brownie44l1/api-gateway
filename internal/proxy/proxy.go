package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Service interface {
	GetName() string
	GetPathPrefix() string
	GetUpstreamURL() string
	GetTimeout() time.Duration
}

type Proxy struct {
	client *http.Client
}

func NewProxy() *Proxy {
	return &Proxy{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// Forward proxies the request to the upstream service
func (p *Proxy) Forward(w http.ResponseWriter, r *http.Request, service Service) {
	// Build upstream URL
	upstreamPath := strings.TrimPrefix(r.URL.Path, service.GetPathPrefix())
	upstreamURL := fmt.Sprintf("%s%s", service.GetUpstreamURL(), upstreamPath)
	
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	// Create new request
	ctx, cancel := context.WithTimeout(r.Context(), service.GetTimeout())
	defer cancel()

	proxyReq, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL, r.Body)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Copy headers (excluding hop-by-hop headers)
	p.copyHeaders(proxyReq.Header, r.Header)
	
	// Add X-Forwarded headers
	p.addForwardedHeaders(proxyReq, r)

	// Execute request
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		log.Printf("Error proxying request to %s: %v", upstreamURL, err)
		if ctx.Err() == context.DeadlineExceeded {
			http.Error(w, "Gateway timeout", http.StatusGatewayTimeout)
		} else {
			http.Error(w, "Bad gateway", http.StatusBadGateway)
		}
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	p.copyHeaders(w.Header(), resp.Header)
	
	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	if _, err := io.Copy(w, resp.Body); err != nil {
		log.Printf("Error copying response body: %v", err)
	}
}

// copyHeaders copies HTTP headers, excluding hop-by-hop headers
func (p *Proxy) copyHeaders(dst, src http.Header) {
	hopByHopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"Te":                  true,
		"Trailers":            true,
		"Transfer-Encoding":   true,
		"Upgrade":             true,
	}

	for key, values := range src {
		if hopByHopHeaders[key] {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

// addForwardedHeaders adds X-Forwarded-* headers to the proxy request
func (p *Proxy) addForwardedHeaders(proxyReq *http.Request, originalReq *http.Request) {
	// X-Forwarded-For
	clientIP := originalReq.RemoteAddr
	if prior := originalReq.Header.Get("X-Forwarded-For"); prior != "" {
		clientIP = prior + ", " + clientIP
	}
	proxyReq.Header.Set("X-Forwarded-For", clientIP)

	// X-Forwarded-Proto
	proto := "http"
	if originalReq.TLS != nil {
		proto = "https"
	}
	proxyReq.Header.Set("X-Forwarded-Proto", proto)

	// X-Forwarded-Host
	proxyReq.Header.Set("X-Forwarded-Host", originalReq.Host)
}