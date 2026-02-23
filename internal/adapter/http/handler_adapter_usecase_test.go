package http

import (
	"context"
	"errors"
	"testing"

	"github.com/jamalishaq/light_serve/internal/domain"
	"github.com/jamalishaq/light_serve/internal/usecase"
)

type stubUseCaseHandler struct {
	output usecase.ResponseOutput
	err    error
	got    usecase.RequestInput
	gotCtx context.Context
}

// Handle records input and returns configured output/error.
func (s *stubUseCaseHandler) Handle(ctx context.Context, input usecase.RequestInput) (usecase.ResponseOutput, error) {
	s.got = input
	s.gotCtx = ctx
	return s.output, s.err
}

// TestAdaptUseCaseHandler_ValidFlow verifies request-to-usecase-to-response translation.
func TestAdaptUseCaseHandler_ValidFlow(t *testing.T) {
	stub := &stubUseCaseHandler{
		output: usecase.ResponseOutput{Body: []byte("usecase ok")},
	}
	adapter := AdaptUseCaseHandler(stub)

	req := &Request{
		Method: "GET",
		Path:   "/users",
		Headers: map[string]string{
			"host": "example.com",
		},
		Body: []byte("input"),
	}

	resp := adapter(req)

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "usecase ok" {
		t.Fatalf("expected response body from use case, got %q", string(resp.Body))
	}
	if stub.got.Path != "/users" {
		t.Fatalf("expected mapped path /users, got %q", stub.got.Path)
	}
	if stub.got.Headers["host"] != "example.com" {
		t.Fatalf("expected mapped header host=example.com, got %#v", stub.got.Headers)
	}
	if string(stub.got.Body) != "input" {
		t.Fatalf("expected mapped body input, got %q", string(stub.got.Body))
	}
	if stub.gotCtx == nil {
		t.Fatalf("expected non-nil context to be passed")
	}
}

// TestAdaptUseCaseHandler_UsesRequestContext verifies request context is propagated.
func TestAdaptUseCaseHandler_UsesRequestContext(t *testing.T) {
	stub := &stubUseCaseHandler{
		output: usecase.ResponseOutput{Body: []byte("ok")},
	}
	adapter := AdaptUseCaseHandler(stub)

	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := adapter(&Request{Path: "/ctx", Ctx: reqCtx})
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if stub.gotCtx != reqCtx {
		t.Fatalf("expected adapter to pass request context through")
	}
}

// TestAdaptUseCaseHandler_ErrorMapping verifies domain error to HTTP status mapping.
func TestAdaptUseCaseHandler_ErrorMapping(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		body   string
	}{
		{name: "bad request", err: domain.ErrBadRequest, status: 400, body: "Bad Request"},
		{name: "unauthorized", err: domain.ErrUnauthorized, status: 401, body: "Unauthorized"},
		{name: "not found", err: domain.ErrNotFound, status: 404, body: "Not Found"},
		{name: "unknown", err: errors.New("boom"), status: 500, body: "Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &stubUseCaseHandler{err: tt.err}
			adapter := AdaptUseCaseHandler(stub)

			resp := adapter(&Request{Path: "/x"})
			if resp.StatusCode != tt.status {
				t.Fatalf("expected status %d, got %d", tt.status, resp.StatusCode)
			}
			if string(resp.Body) != tt.body {
				t.Fatalf("expected body %q, got %q", tt.body, string(resp.Body))
			}
		})
	}
}

// TestAdaptUseCaseHandler_NilHandler verifies nil use case handler results in 500.
func TestAdaptUseCaseHandler_NilHandler(t *testing.T) {
	adapter := AdaptUseCaseHandler(nil)
	resp := adapter(&Request{Path: "/x"})
	if resp.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
}

