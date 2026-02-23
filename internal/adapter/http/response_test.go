package http

import (
	"bytes"
	"strings"
	"testing"
)

// TestNewResponse_Defaults verifies default response values.
func TestNewResponse_Defaults(t *testing.T) {
	resp := NewResponse()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	if resp.Headers == nil {
		t.Fatalf("expected headers map to be initialized")
	}
	if len(resp.Headers) != 0 {
		t.Fatalf("expected empty headers map, got %#v", resp.Headers)
	}
	if len(resp.Body) != 0 {
		t.Fatalf("expected empty body, got %q", string(resp.Body))
	}
}

// TestResponse_SetHeader verifies headers are set and map initialization is nil-safe.
func TestResponse_SetHeader(t *testing.T) {
	resp := &Response{StatusCode: 200}
	resp.SetHeader("Content-Type", "text/plain")

	if resp.Headers == nil {
		t.Fatalf("expected headers map to be initialized")
	}
	if got := resp.Headers["Content-Type"]; got != "text/plain" {
		t.Fatalf("expected Content-Type=text/plain, got %q", got)
	}

	resp.SetHeader("Content-Type", "application/json")
	if got := resp.Headers["Content-Type"]; got != "application/json" {
		t.Fatalf("expected Content-Type overwrite to application/json, got %q", got)
	}
}

// TestResponse_WriteBytes verifies byte body writes and copy semantics.
func TestResponse_WriteBytes(t *testing.T) {
	resp := NewResponse()
	original := []byte("hello")

	resp.WriteBytes(original)
	if string(resp.Body) != "hello" {
		t.Fatalf("expected body hello, got %q", string(resp.Body))
	}

	original[0] = 'H'
	if string(resp.Body) != "hello" {
		t.Fatalf("expected body to be copied, got %q", string(resp.Body))
	}
}

// TestResponse_WriteString verifies string body writes.
func TestResponse_WriteString(t *testing.T) {
	resp := NewResponse()
	resp.WriteString("world")

	if string(resp.Body) != "world" {
		t.Fatalf("expected body world, got %q", string(resp.Body))
	}
}

// TestResponse_Bytes_WireFormat verifies HTTP/1.1 wire serialization layout.
func TestResponse_Bytes_WireFormat(t *testing.T) {
	resp := NewResponse()
	resp.StatusCode = 200
	resp.SetHeader("Content-Type", "text/plain")
	resp.WriteString("Hello")

	wire := string(resp.Bytes())

	if !strings.HasPrefix(wire, "HTTP/1.1 200 OK\r\n") {
		t.Fatalf("expected status line prefix, got %q", wire)
	}
	if !strings.Contains(wire, "Content-Type: text/plain\r\n") {
		t.Fatalf("expected Content-Type header in wire output, got %q", wire)
	}
	if !strings.Contains(wire, "Content-Length: 5\r\n") {
		t.Fatalf("expected Content-Length header in wire output, got %q", wire)
	}
	if !strings.Contains(wire, "\r\n\r\nHello") {
		t.Fatalf("expected header/body separator and body, got %q", wire)
	}
}

// TestResponse_Bytes_AutoContentLength verifies Content-Length is auto-added when missing.
func TestResponse_Bytes_AutoContentLength(t *testing.T) {
	resp := NewResponse()
	resp.WriteBytes([]byte("abc"))

	_ = resp.Bytes()
	if got := resp.Headers["Content-Length"]; got != "3" {
		t.Fatalf("expected auto Content-Length=3, got %q", got)
	}
}

// TestResponse_Bytes_DoesNotOverwriteContentLength verifies explicit Content-Length is preserved.
func TestResponse_Bytes_DoesNotOverwriteContentLength(t *testing.T) {
	resp := NewResponse()
	resp.SetHeader("Content-Length", "999")
	resp.WriteString("abc")

	_ = resp.Bytes()
	if got := resp.Headers["Content-Length"]; got != "999" {
		t.Fatalf("expected Content-Length to remain 999, got %q", got)
	}
}

// TestResponse_Bytes_ContentLengthCaseInsensitive verifies key matching ignores case.
func TestResponse_Bytes_ContentLengthCaseInsensitive(t *testing.T) {
	resp := NewResponse()
	resp.SetHeader("content-length", "777")
	resp.WriteString("abc")

	wire := string(resp.Bytes())
	if strings.Contains(wire, "Content-Length: 3\r\n") {
		t.Fatalf("expected no auto Content-Length overwrite, got %q", wire)
	}
	if got := resp.Headers["content-length"]; got != "777" {
		t.Fatalf("expected original lowercase content-length header to remain, got %q", got)
	}
}

// TestResponse_Bytes_UnknownStatus verifies unknown codes use a fallback reason phrase.
func TestResponse_Bytes_UnknownStatus(t *testing.T) {
	resp := NewResponse()
	resp.StatusCode = 599
	wire := string(resp.Bytes())

	if !strings.HasPrefix(wire, "HTTP/1.1 599 Unknown\r\n") {
		t.Fatalf("expected unknown status line, got %q", wire)
	}
}

// TestResponse_Bytes_BodyBytesUnchanged verifies body bytes are written verbatim.
func TestResponse_Bytes_BodyBytesUnchanged(t *testing.T) {
	resp := NewResponse()
	body := []byte{0x00, 0x01, 0x02, 0x41}
	resp.WriteBytes(body)

	wire := resp.Bytes()
	sep := []byte("\r\n\r\n")
	idx := bytes.Index(wire, sep)
	if idx == -1 {
		t.Fatalf("expected header/body separator in wire output")
	}

	gotBody := wire[idx+len(sep):]
	if !bytes.Equal(gotBody, body) {
		t.Fatalf("expected body %v, got %v", body, gotBody)
	}
}
