// File: api-gateway/internal/httpserver/adapter.go
package httpserver

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/Brownie44l1/http/internal/headers"
	httpserver "github.com/Brownie44l1/http/internal/server"
	"github.com/Brownie44l1/http/internal/response"
)

// ResponseWriter adapts our HTTP response.Writer to work like net/http.ResponseWriter
type ResponseWriter struct {
	writer  *response.Writer
	headers *headers.Headers
	status  int
	written bool
}

// NewResponseWriter creates a new adapter
func NewResponseWriter(w *response.Writer) *ResponseWriter {
	return &ResponseWriter{
		writer:  w,
		headers: headers.NewHeaders(),
		status:  200, // Default status
		written: false,
	}
}

// Header returns the header map
func (rw *ResponseWriter) Header() *headers.Headers {
	return rw.headers
}

// Write writes the response body
func (rw *ResponseWriter) Write(data []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(rw.status)
	}
	
	if err := rw.writer.WriteBody(data); err != nil {
		return 0, err
	}
	return len(data), nil
}

// WriteHeader writes the status code and headers
func (rw *ResponseWriter) WriteHeader(statusCode int) {
	if rw.written {
		return // Already written
	}
	
	rw.status = statusCode
	rw.written = true
	
	// Write status line
	if err := rw.writer.WriteStatusLine(response.StatusCode(statusCode)); err != nil {
		return
	}
	
	// Add Content-Length if not already set
	if _, ok := rw.headers.Get("content-length"); !ok {
		// For now, we'll handle this in Write() if needed
	}
	
	// Write headers
	rw.writer.WriteHeaders(rw.headers)
}

// WriteJSON is a convenience method to write JSON responses
func (rw *ResponseWriter) WriteJSON(statusCode int, data interface{}) error {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	
	rw.headers.Set("Content-Type", "application/json")
	rw.headers.Set("Content-Length", strconv.Itoa(len(jsonData)))
	rw.WriteHeader(statusCode)
	
	_, err = rw.Write(jsonData)
	return err
}

// Error writes an error response
func (rw *ResponseWriter) Error(message string, statusCode int) {
	rw.headers.Set("Content-Type", "text/plain; charset=utf-8")
	rw.headers.Set("Content-Length", strconv.Itoa(len(message)))
	rw.WriteHeader(statusCode)
	rw.Write([]byte(message))
}

// Request wraps our request to provide a familiar interface
type Request struct {
	ctx *httpserver.Context
}

// NewRequest creates a new request wrapper
func NewRequest(ctx *httpserver.Context) *Request {
	return &Request{ctx: ctx}
}

// Method returns the HTTP method
func (r *Request) Method() string {
	return r.ctx.Method()
}

// URL returns the request path (simplified for now)
func (r *Request) URL() string {
	return r.ctx.Path()
}

// Header returns a request header
func (r *Request) Header(key string) string {
	return r.ctx.Header(key)
}

// Body returns the request body
func (r *Request) Body() []byte {
	return r.ctx.Body()
}

// Context returns the underlying context
func (r *Request) Context() *httpserver.Context {
	return r.ctx
}

// HandlerAdapter adapts a gateway handler to work with our HTTP server
type HandlerAdapter struct {
	handler func(w *ResponseWriter, r *Request)
}

// NewHandlerAdapter creates a new handler adapter
func NewHandlerAdapter(handler func(w *ResponseWriter, r *Request)) *HandlerAdapter {
	return &HandlerAdapter{handler: handler}
}

// ServeHTTP implements the server.Handler interface
func (ha *HandlerAdapter) ServeHTTP(ctx *httpserver.Context) {
	w := NewResponseWriter(ctx.Response)
	r := NewRequest(ctx)
	ha.handler(w, r)
}

// Middleware type for chaining
type Middleware func(HandlerFunc) HandlerFunc

// HandlerFunc is a function that handles requests
type HandlerFunc func(w *ResponseWriter, r *Request)

// Router provides HTTP routing with middleware support
type Router struct {
	routes      map[string]map[string]HandlerFunc // method -> path -> handler
	middlewares []Middleware
}

// NewRouter creates a new router
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]map[string]HandlerFunc),
	}
}

// Use adds middleware to the router
func (router *Router) Use(mw Middleware) {
	router.middlewares = append(router.middlewares, mw)
}

// Handle registers a handler for a method and path
func (router *Router) Handle(method, path string, handler HandlerFunc) {
	if router.routes[method] == nil {
		router.routes[method] = make(map[string]HandlerFunc)
	}
	
	// Apply all middlewares to the handler
	for i := len(router.middlewares) - 1; i >= 0; i-- {
		handler = router.middlewares[i](handler)
	}
	
	router.routes[method][path] = handler
}

// GET registers a GET handler
func (router *Router) GET(path string, handler HandlerFunc) {
	router.Handle("GET", path, handler)
}

// POST registers a POST handler
func (router *Router) POST(path string, handler HandlerFunc) {
	router.Handle("POST", path, handler)
}

// PUT registers a PUT handler
func (router *Router) PUT(path string, handler HandlerFunc) {
	router.Handle("PUT", path, handler)
}

// DELETE registers a DELETE handler
func (router *Router) DELETE(path string, handler HandlerFunc) {
	router.Handle("DELETE", path, handler)
}

// ServeHTTP implements the server.Handler interface
func (router *Router) ServeHTTP(ctx *httpserver.Context) {
	method := ctx.Method()
	path := ctx.Path()
	
	// Find handler
	if handlers, ok := router.routes[method]; ok {
		if handler, ok := handlers[path]; ok {
			w := NewResponseWriter(ctx.Response)
			r := NewRequest(ctx)
			handler(w, r)
			return
		}
	}
	
	// No handler found - 404
	ctx.Error(response.StatusNotFound, "Not Found")
}

// Example middleware implementations

// LoggingMiddleware logs each request
func LoggingMiddleware(next HandlerFunc) HandlerFunc {
	return func(w *ResponseWriter, r *Request) {
		fmt.Printf("%s %s\n", r.Method(), r.URL())
		next(w, r)
	}
}

// RecoveryMiddleware recovers from panics
func RecoveryMiddleware(next HandlerFunc) HandlerFunc {
	return func(w *ResponseWriter, r *Request) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Printf("PANIC: %v\n", err)
				w.Error("Internal Server Error", 500)
			}
		}()
		next(w, r)
	}
}