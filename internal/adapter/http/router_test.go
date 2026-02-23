package http

import (
	"reflect"
	"testing"
)

// TestRouter_RegisterLookup verifies METHOD:PATH mapping behavior.
func TestRouter_RegisterLookup(t *testing.T) {
	router := NewRouter()
	router.Register("get", "/users", func(req *Request) *Response {
		resp := NewResponse()
		resp.WriteString("ok")
		return resp
	})

	handler, ok := router.Lookup("GET", "/users")
	if !ok || handler == nil {
		t.Fatalf("expected handler for GET:/users")
	}

	if _, ok := router.Lookup("POST", "/users"); ok {
		t.Fatalf("did not expect handler for POST:/users")
	}
}

// TestRouter_ResolveAppliesMiddlewareOrder verifies middleware wrapping order.
func TestRouter_ResolveAppliesMiddlewareOrder(t *testing.T) {
	router := NewRouter()
	order := make([]string, 0, 5)

	router.Use(
		func(next HandlerAdapter) HandlerAdapter {
			return func(req *Request) *Response {
				order = append(order, "mw1-before")
				resp := next(req)
				order = append(order, "mw1-after")
				return resp
			}
		},
		func(next HandlerAdapter) HandlerAdapter {
			return func(req *Request) *Response {
				order = append(order, "mw2-before")
				resp := next(req)
				order = append(order, "mw2-after")
				return resp
			}
		},
	)

	router.Register("GET", "/x", func(req *Request) *Response {
		order = append(order, "handler")
		resp := NewResponse()
		resp.WriteString("ok")
		return resp
	})

	handler, ok := router.Resolve("GET", "/x")
	if !ok || handler == nil {
		t.Fatalf("expected resolved handler")
	}

	resp := handler(&Request{Method: "GET", Path: "/x"})
	if string(resp.Body) != "ok" {
		t.Fatalf("expected handler response body, got %q", string(resp.Body))
	}

	want := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("unexpected middleware order: got %v, want %v", order, want)
	}
}

// TestRouter_ResolveMiddlewareCanShortCircuit verifies middleware may skip downstream handlers.
func TestRouter_ResolveMiddlewareCanShortCircuit(t *testing.T) {
	router := NewRouter()

	router.Use(func(next HandlerAdapter) HandlerAdapter {
		return func(req *Request) *Response {
			resp := NewResponse()
			resp.StatusCode = 401
			resp.WriteString("blocked")
			return resp
		}
	})

	handlerCalled := false
	router.Register("GET", "/secure", func(req *Request) *Response {
		handlerCalled = true
		resp := NewResponse()
		resp.WriteString("should not happen")
		return resp
	})

	handler, ok := router.Resolve("GET", "/secure")
	if !ok || handler == nil {
		t.Fatalf("expected resolved handler")
	}

	resp := handler(&Request{Method: "GET", Path: "/secure"})
	if resp.StatusCode != 401 {
		t.Fatalf("expected short-circuit status 401, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "blocked" {
		t.Fatalf("expected short-circuit body blocked, got %q", string(resp.Body))
	}
	if handlerCalled {
		t.Fatalf("expected route handler not to be called")
	}
}

// TestRouter_AllowedMethods verifies registered methods are discovered by path.
func TestRouter_AllowedMethods(t *testing.T) {
	router := NewRouter()
	router.Register("GET", "/users", func(req *Request) *Response { return NewResponse() })
	router.Register("POST", "/users", func(req *Request) *Response { return NewResponse() })
	router.Register("DELETE", "/other", func(req *Request) *Response { return NewResponse() })

	got := router.AllowedMethods("/users")
	want := []string{"GET", "POST"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected allowed methods: got %v, want %v", got, want)
	}
}
