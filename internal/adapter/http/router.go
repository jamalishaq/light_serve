package http

import (
	"sort"
	"strings"
	"sync"
)

// HandlerAdapter adapts a parsed HTTP request into an HTTP response.
type HandlerAdapter func(*Request) *Response

// Middleware wraps a handler adapter to provide cross-cutting behavior.
type Middleware func(HandlerAdapter) HandlerAdapter

// Router maps METHOD:PATH keys to handler adapters.
type Router struct {
	mu          sync.RWMutex
	routes      map[string]HandlerAdapter
	middlewares []Middleware
}

// NewRouter creates an empty router.
func NewRouter() *Router {
	return &Router{
		routes: make(map[string]HandlerAdapter),
	}
}

// Use appends middleware to the router chain in registration order.
func (r *Router) Use(middlewares ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middlewares = append(r.middlewares, middlewares...)
}

// Register maps a method/path pair to a handler adapter.
func (r *Router) Register(method, path string, handler HandlerAdapter) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes[routeKey(method, path)] = handler
}

// Lookup returns the handler adapter for a method/path pair.
func (r *Router) Lookup(method, path string) (HandlerAdapter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, ok := r.routes[routeKey(method, path)]
	return handler, ok
}

// Resolve returns a route handler wrapped with the registered middleware chain.
func (r *Router) Resolve(method, path string) (HandlerAdapter, bool) {
	r.mu.RLock()
	handler, ok := r.routes[routeKey(method, path)]
	if !ok {
		r.mu.RUnlock()
		return nil, false
	}

	middlewares := make([]Middleware, len(r.middlewares))
	copy(middlewares, r.middlewares)
	r.mu.RUnlock()

	wrapped := applyMiddleware(handler, middlewares)
	return wrapped, true
}

// AllowedMethods returns sorted HTTP methods registered for a path.
func (r *Router) AllowedMethods(path string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]struct{})
	suffix := ":" + path
	for key := range r.routes {
		if strings.HasSuffix(key, suffix) {
			method := strings.TrimSuffix(key, suffix)
			if method != "" {
				seen[method] = struct{}{}
			}
		}
	}

	methods := make([]string, 0, len(seen))
	for method := range seen {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

// applyMiddleware wraps a handler with middlewares from outermost to innermost.
func applyMiddleware(handler HandlerAdapter, middlewares []Middleware) HandlerAdapter {
	wrapped := handler
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

// routeKey builds the router lookup key in METHOD:PATH format.
func routeKey(method, path string) string {
	return strings.ToUpper(method) + ":" + path
}
