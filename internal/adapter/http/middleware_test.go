package http

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// stubLogger captures middleware log messages for assertions.
type stubLogger struct {
	entries []string
}

// Info stores info-level log entries for test verification.
func (l *stubLogger) Info(msg string, keysAndValues ...any) {
	l.entries = append(l.entries, fmt.Sprintf("%s %v", msg, keysAndValues))
}

// Error stores error-level log entries for test verification.
func (l *stubLogger) Error(msg string, keysAndValues ...any) {
	l.entries = append(l.entries, fmt.Sprintf("%s %v", msg, keysAndValues))
}

// TestRecoveryMiddleware_RecoversPanic verifies panic recovery to 500 responses.
func TestRecoveryMiddleware_RecoversPanic(t *testing.T) {
	logger := &stubLogger{}
	mw := RecoveryMiddleware(logger)

	handler := mw(func(req *Request) *Response {
		panic("boom")
	})

	resp := handler(&Request{
		Method: "GET",
		Path:   "/panic",
		Headers: map[string]string{
			"x-request-id":     "req-789",
			"x-correlation-id": "corr-789",
		},
	})
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.StatusCode != 500 {
		t.Fatalf("expected status 500, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "Internal Server Error" {
		t.Fatalf("expected internal error body, got %q", string(resp.Body))
	}
	if len(logger.entries) == 0 {
		t.Fatalf("expected panic recovery log entry")
	}
	entry := logger.entries[0]
	if !strings.Contains(entry, "request_id req-789") {
		t.Fatalf("expected request_id in panic log entry, got %q", entry)
	}
	if !strings.Contains(entry, "correlation_id corr-789") {
		t.Fatalf("expected correlation_id in panic log entry, got %q", entry)
	}
}

// TestTimeoutMiddleware_ReturnsTimeout verifies timeout middleware returns 408.
func TestTimeoutMiddleware_ReturnsTimeout(t *testing.T) {
	mw := TimeoutMiddleware(5 * time.Millisecond)
	blockCh := make(chan struct{})

	handler := mw(func(req *Request) *Response {
		<-blockCh
		resp := NewResponse()
		resp.StatusCode = 200
		resp.WriteString("late")
		return resp
	})

	resp := handler(&Request{Method: "GET", Path: "/slow"})
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.StatusCode != 408 {
		t.Fatalf("expected status 408, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "Request Timeout" {
		t.Fatalf("expected timeout body, got %q", string(resp.Body))
	}

	close(blockCh)
}

// TestTimeoutMiddleware_PassesThroughFastHandler verifies non-timed-out requests succeed.
func TestTimeoutMiddleware_PassesThroughFastHandler(t *testing.T) {
	mw := TimeoutMiddleware(100 * time.Millisecond)
	handler := mw(func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 200
		resp.WriteString("ok")
		return resp
	})

	resp := handler(&Request{Method: "GET", Path: "/fast"})
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if string(resp.Body) != "ok" {
		t.Fatalf("expected body ok, got %q", string(resp.Body))
	}
}

// TestTimeoutMiddleware_InjectsTimeoutContext verifies downstream sees timeout cancellation.
func TestTimeoutMiddleware_InjectsTimeoutContext(t *testing.T) {
	mw := TimeoutMiddleware(5 * time.Millisecond)
	handler := mw(func(req *Request) *Response {
		<-req.Context().Done()
		if req.Context().Err() != context.DeadlineExceeded {
			t.Fatalf("expected deadline exceeded, got %v", req.Context().Err())
		}
		resp := NewResponse()
		resp.StatusCode = 200
		resp.WriteString("late")
		return resp
	})

	resp := handler(&Request{Method: "GET", Path: "/ctx-timeout"})
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if resp.StatusCode != 408 {
		t.Fatalf("expected status 408, got %d", resp.StatusCode)
	}
}

// TestLoggingMiddleware_LogsRequest verifies request metadata is logged.
func TestLoggingMiddleware_LogsRequest(t *testing.T) {
	logger := &stubLogger{}
	mw := LoggingMiddleware(logger)

	handler := mw(func(req *Request) *Response {
		resp := NewResponse()
		resp.StatusCode = 201
		resp.WriteString("created")
		return resp
	})

	resp := handler(&Request{
		Method: "POST",
		Path:   "/items",
		Headers: map[string]string{
			"x-request-id":     "req-123",
			"x-correlation-id": "corr-456",
		},
	})
	if resp == nil {
		t.Fatalf("expected non-nil response")
	}
	if len(logger.entries) != 1 {
		t.Fatalf("expected one log entry, got %d", len(logger.entries))
	}
	entry := logger.entries[0]
	if !strings.Contains(entry, "method POST") {
		t.Fatalf("expected method in log entry, got %q", entry)
	}
	if !strings.Contains(entry, "path /items") {
		t.Fatalf("expected path in log entry, got %q", entry)
	}
	if !strings.Contains(entry, "status 201") {
		t.Fatalf("expected status in log entry, got %q", entry)
	}
	if !strings.Contains(entry, "request_id req-123") {
		t.Fatalf("expected request_id in log entry, got %q", entry)
	}
	if !strings.Contains(entry, "correlation_id corr-456") {
		t.Fatalf("expected correlation_id in log entry, got %q", entry)
	}
}
