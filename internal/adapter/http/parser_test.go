package http

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// TestParseRequest_ValidMinimal verifies parsing a minimal valid request.
func TestParseRequest_ValidMinimal(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\n\r\n")

	req, consumed, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if consumed != len(raw) {
		t.Fatalf("expected consumed %d, got %d", len(raw), consumed)
	}
	if req.Method != "GET" || req.Path != "/" || req.Version != "HTTP/1.1" {
		t.Fatalf("unexpected request line: %+v", req)
	}
	if len(req.Headers) != 0 {
		t.Fatalf("expected no headers, got %#v", req.Headers)
	}
	if len(req.Body) != 0 {
		t.Fatalf("expected empty body, got %q", string(req.Body))
	}
}

// TestParseRequest_ValidWithHeadersAndBody verifies parsing headers and body.
func TestParseRequest_ValidWithHeadersAndBody(t *testing.T) {
	raw := []byte("POST /echo HTTP/1.1\r\nHost: localhost\r\nContent-Type: text/plain\r\nContent-Length: 5\r\n\r\nhello")

	req, consumed, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consumed != len(raw) {
		t.Fatalf("expected consumed %d, got %d", len(raw), consumed)
	}
	if req.Method != "POST" || req.Path != "/echo" {
		t.Fatalf("unexpected method/path: %s %s", req.Method, req.Path)
	}
	if req.Headers["host"] != "localhost" {
		t.Fatalf("expected host header, got %#v", req.Headers)
	}
	if req.Headers["content-type"] != "text/plain" {
		t.Fatalf("expected content-type header, got %#v", req.Headers)
	}
	if string(req.Body) != "hello" {
		t.Fatalf("expected body hello, got %q", string(req.Body))
	}
}

// TestParseRequest_PathWithQuery verifies query strings are preserved in path.
func TestParseRequest_PathWithQuery(t *testing.T) {
	raw := []byte("GET /users?id=1 HTTP/1.1\r\n\r\n")
	req, _, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Path != "/users?id=1" {
		t.Fatalf("unexpected path: %q", req.Path)
	}
}

// TestParseRequest_LFOnlyLineEndings verifies LF-only requests are accepted.
func TestParseRequest_LFOnlyLineEndings(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\nHost: localhost\n\n")
	req, _, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Headers["host"] != "localhost" {
		t.Fatalf("expected host header, got %#v", req.Headers)
	}
}

// TestParseRequest_HeaderNormalizationAndLastWins verifies normalized keys and overwrite behavior.
func TestParseRequest_HeaderNormalizationAndLastWins(t *testing.T) {
	raw := []byte("GET / HTTP/1.1\r\nHost: a\r\nhost: b\r\n\r\n")
	req, _, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Headers["host"] != "b" {
		t.Fatalf("expected last host header to win, got %q", req.Headers["host"])
	}
}

// TestParseRequest_MultipleRequestsConsumedLength verifies consumed bytes for buffered streams.
func TestParseRequest_MultipleRequestsConsumedLength(t *testing.T) {
	first := "GET /one HTTP/1.1\r\n\r\n"
	second := "GET /two HTTP/1.1\r\n\r\n"
	raw := []byte(first + second)

	req, consumed, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Path != "/one" {
		t.Fatalf("expected first request to be parsed, got %q", req.Path)
	}
	if consumed != len(first) {
		t.Fatalf("expected consumed %d, got %d", len(first), consumed)
	}
}

// TestParseRequest_ContentLengthZero verifies empty bodies with Content-Length zero.
func TestParseRequest_ContentLengthZero(t *testing.T) {
	raw := []byte("POST /empty HTTP/1.1\r\nContent-Length: 0\r\n\r\n")
	req, _, err := ParseRequest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(req.Body) != 0 {
		t.Fatalf("expected empty body, got %q", string(req.Body))
	}
}

// TestParseRequest_Errors verifies malformed and incomplete request error handling.
func TestParseRequest_Errors(t *testing.T) {
	tests := []struct {
		name string
		raw  []byte
		want error
	}{
		{
			name: "empty input",
			raw:  []byte(""),
			want: ErrEmptyRequest,
		},
		{
			name: "incomplete request line",
			raw:  []byte("GET /"),
			want: ErrIncompleteRequest,
		},
		{
			name: "malformed request line too few parts",
			raw:  []byte("GET\r\n\r\n"),
			want: ErrMalformedRequestLine,
		},
		{
			name: "malformed request line too many parts",
			raw:  []byte("GET / HTTP/1.1 extra\r\n\r\n"),
			want: ErrMalformedRequestLine,
		},
		{
			name: "invalid version",
			raw:  []byte("GET / HTTP/2.0\r\n\r\n"),
			want: ErrInvalidHTTPVersion,
		},
		{
			name: "malformed header missing colon",
			raw:  []byte("GET / HTTP/1.1\r\nHost localhost\r\n\r\n"),
			want: ErrInvalidHeader,
		},
		{
			name: "malformed header empty key",
			raw:  []byte("GET / HTTP/1.1\r\n: value\r\n\r\n"),
			want: ErrInvalidHeader,
		},
		{
			name: "invalid content-length non-numeric",
			raw:  []byte("POST / HTTP/1.1\r\nContent-Length: abc\r\n\r\n"),
			want: ErrInvalidContentLength,
		},
		{
			name: "content-length mismatch incomplete body",
			raw:  []byte("POST / HTTP/1.1\r\nContent-Length: 5\r\n\r\nhey"),
			want: ErrIncompleteBody,
		},
		{
			name: "request line too long",
			raw:  []byte("GET /" + strings.Repeat("a", maxRequestLineBytes) + " HTTP/1.1\r\n\r\n"),
			want: ErrRequestLineTooLong,
		},
		{
			name: "too many headers",
			raw:  []byte("GET / HTTP/1.1\r\n" + buildHeaders(maxHeaderCount+1) + "\r\n\r\n"),
			want: ErrTooManyHeaders,
		},
		{
			name: "body too large",
			raw:  []byte("POST / HTTP/1.1\r\nContent-Length: 300000\r\n\r\n"),
			want: ErrBodyTooLarge,
		},
		{
			name: "headers too large before delimiter",
			raw:  []byte(strings.Repeat("a", maxHeadersBytes+1)),
			want: ErrHeadersTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ParseRequest(tt.raw)
			if !errors.Is(err, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, err)
			}
		})
	}
}

// buildHeaders builds a list of test header lines.
func buildHeaders(count int) string {
	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, "X-"+strconv.Itoa(i)+": v")
	}
	return strings.Join(lines, "\r\n")
}
