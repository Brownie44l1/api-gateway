package registry

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	ErrServiceNotFound = errors.New("service not found")
)

type Service struct {
	Name string
	PathPrefix string
	UpstreamURL string
	Timeout time.Duration
	Healthy bool
}

type Registry struct {
	mu sync.RWMutex
	services map[string]*Service
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*Service),
	}
}

// Register adds a service to the registry
func (r *Registry) Register(service *Service) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.services[service.PathPrefix] = service
}

// FindService finds a service by matching the request path
func (r *Registry) FindService(path string) (*Service, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find the longest matching prefix
	var matchedService *Service
	var longestMatch string

	for prefix, service := range r.services {
		if strings.HasPrefix(path, prefix) && len(prefix) > len(longestMatch) {
			longestMatch = prefix
			matchedService = service
		}
	}

	if matchedService == nil {
		return nil, ErrServiceNotFound
	}

	if !matchedService.Healthy {
		return nil, errors.New("service is unhealthy")
	}

	return matchedService, nil
}

// GetAllServices returns all registered services
func (r *Registry) GetAllServices() []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*Service, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	return services
}

// UpdateHealthStatus updates the health status of a service
func (r *Registry) UpdateHealthStatus(pathPrefix string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if service, exists := r.services[pathPrefix]; exists {
		service.Healthy = healthy
	}
}

// Deregister removes a service from the registry
func (r *Registry) Deregister(pathPrefix string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.services, pathPrefix)
}