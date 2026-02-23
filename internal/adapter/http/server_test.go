package http

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jamalishaq/light_serve/internal/usecase"
)

// TestHandleConn_UnknownRouteReturns404 verifies unknown route responses are 404.
func TestHandleConn_UnknownRouteReturns404(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	go HandleConn(serverConn)

	request := "GET /hello HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 404 Not Found\r\n") {
		t.Fatalf("expected 404 status line, got %q", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nNot Found") {
		t.Fatalf("expected not found body, got %q", resp)
	}
}

// TestHandleConn_MalformedRequest verifies malformed requests return 400.
func TestHandleConn_MalformedRequest(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	go HandleConn(serverConn)

	request := "GET /bad HTTP/1.1\r\nInvalidHeader\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 400 Bad Request\r\n") {
		t.Fatalf("expected 400 status line, got %q", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nBad Request") {
		t.Fatalf("expected bad request response body, got %q", resp)
	}
}

// TestHandleConn_KeepAliveProcessesMultipleRequests verifies basic keep-alive.
func TestHandleConn_KeepAliveProcessesMultipleRequests(t *testing.T) {
	router := NewRouter()
	router.Register("GET", "/one", func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("one")
		return resp
	})
	router.Register("GET", "/two", func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("two")
		return resp
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	go HandleConnWithRouter(serverConn, router)

	request := "GET /one HTTP/1.1\r\nHost: example.com\r\n\r\nGET /two HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if strings.Count(resp, "HTTP/1.1 200 OK\r\n") != 2 {
		t.Fatalf("expected exactly two 200 responses, got %q", resp)
	}
	if !strings.Contains(resp, "\r\n\r\none") || !strings.Contains(resp, "\r\n\r\ntwo") {
		t.Fatalf("expected both response bodies, got %q", resp)
	}
}

// TestHandleConnWithRouter_RoutedHandler verifies METHOD:PATH routing to handler adapters.
func TestHandleConnWithRouter_RoutedHandler(t *testing.T) {
	router := NewRouter()
	router.Register("GET", "/routed", func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("routed handler")
		return resp
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouter(serverConn, router)

	request := "GET /routed HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("expected 200 status line, got %q", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nrouted handler") {
		t.Fatalf("expected routed handler body, got %q", resp)
	}
}

// TestHandleConnWithRouter_MiddlewareApplied verifies middleware is executed in routed path.
func TestHandleConnWithRouter_MiddlewareApplied(t *testing.T) {
	router := NewRouter()
	router.Use(func(next HandlerAdapter) HandlerAdapter {
		return func(req *Request) *Response {
			resp := next(req)
			resp.SetHeader("X-Middleware", "applied")
			return resp
		}
	})

	router.Register("GET", "/mw", func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("middleware path")
		return resp
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouter(serverConn, router)

	request := "GET /mw HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.Contains(resp, "X-Middleware: applied\r\n") {
		t.Fatalf("expected middleware header in response, got %q", resp)
	}
}

// TestHandleConnWithRouter_RecoveryMiddleware verifies panic recovery in routed handling.
func TestHandleConnWithRouter_RecoveryMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(RecoveryMiddleware(nil))
	router.Register("GET", "/panic", func(req *Request) *Response {
		panic("boom")
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouter(serverConn, router)

	request := "GET /panic HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 500 Internal Server Error\r\n") {
		t.Fatalf("expected 500 status line, got %q", resp)
	}
}

// TestHandleConnWithRouter_TimeoutMiddleware verifies timeout handling in routed path.
func TestHandleConnWithRouter_TimeoutMiddleware(t *testing.T) {
	router := NewRouter()
	router.Use(TimeoutMiddleware(5 * time.Millisecond))

	blockCh := make(chan struct{})
	router.Register("GET", "/slow", func(req *Request) *Response {
		<-blockCh
		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("late")
		return resp
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouter(serverConn, router)

	request := "GET /slow HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	close(blockCh)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 408 Request Timeout\r\n") {
		t.Fatalf("expected 408 status line, got %q", resp)
	}
	if !strings.Contains(resp, "\r\n\r\nRequest Timeout") {
		t.Fatalf("expected timeout response body, got %q", resp)
	}
}

// TestHandleConnWithRouter_MethodNotAllowed verifies 405 and Allow response behavior.
func TestHandleConnWithRouter_MethodNotAllowed(t *testing.T) {
	router := NewRouter()
	router.Register("GET", "/users", func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.WriteString("users")
		return resp
	})

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouter(serverConn, router)

	request := "POST /users HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 405 Method Not Allowed\r\n") {
		t.Fatalf("expected 405 status line, got %q", resp)
	}
	if !strings.Contains(resp, "Allow: GET\r\n") {
		t.Fatalf("expected Allow header, got %q", resp)
	}
}

type cancelAwareUseCase struct {
	ctxErrCh chan error
}

// Handle records cancellation signal from propagated request context.
func (u *cancelAwareUseCase) Handle(ctx context.Context, input usecase.RequestInput) (usecase.ResponseOutput, error) {
	<-ctx.Done()
	u.ctxErrCh <- ctx.Err()
	return usecase.ResponseOutput{}, ctx.Err()
}

// TestHandleConnWithRouterAndContext_PropagatesCancel verifies context reaches use case.
func TestHandleConnWithRouterAndContext_PropagatesCancel(t *testing.T) {
	router := NewRouter()
	uc := &cancelAwareUseCase{ctxErrCh: make(chan error, 1)}
	router.Register("GET", "/cancel", AdaptUseCaseHandler(uc))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go HandleConnWithRouterAndContext(serverConn, router, ctx)

	request := "GET /cancel HTTP/1.1\r\nHost: example.com\r\nConnection: close\r\n\r\n"
	if _, err := clientConn.Write([]byte(request)); err != nil {
		t.Fatalf("write request failed: %v", err)
	}

	respBytes, err := io.ReadAll(clientConn)
	if err != nil {
		t.Fatalf("read response failed: %v", err)
	}
	resp := string(respBytes)

	if !strings.HasPrefix(resp, "HTTP/1.1 500 Internal Server Error\r\n") {
		t.Fatalf("expected 500 status line, got %q", resp)
	}

	select {
	case ctxErr := <-uc.ctxErrCh:
		if !errors.Is(ctxErr, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", ctxErr)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected use case to observe cancellation")
	}
}
