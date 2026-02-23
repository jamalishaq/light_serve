package main

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	logadapter "github.com/jamalishaq/light_serve/internal/adapter/logging"
)

// TestServerRuntime_ServeStopsOnContextCancel verifies serve exits after cancellation.
func TestServerRuntime_ServeStopsOnContextCancel(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	runtime := newServerRuntime(listener, logadapter.NewStdLogger(log.New(io.Discard, "", 0)), 0, 0, 100*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runtime.serve(ctx)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil serve error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("serve did not stop after context cancellation")
	}
}

// TestServerRuntime_ServeForcesCloseOnShutdownDeadline verifies active conns are closed at deadline.
func TestServerRuntime_ServeForcesCloseOnShutdownDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	runtime := newServerRuntime(listener, logadapter.NewStdLogger(log.New(io.Discard, "", 0)), 0, 0, 50*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- runtime.serve(ctx)
	}()

	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer clientConn.Close()

	waitForActiveConn(t, runtime, time.Second)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil serve error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("serve did not finish after shutdown deadline")
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	buf := make([]byte, 1)
	_, readErr := clientConn.Read(buf)
	if readErr == nil {
		t.Fatalf("expected closed client connection after forced shutdown")
	}
}

// TestServerRuntime_HandleConnSetsDeadlines verifies configured deadlines are applied.
func TestServerRuntime_HandleConnSetsDeadlines(t *testing.T) {
	conn := &spyConn{}
	runtime := newServerRuntime(nil, logadapter.NewStdLogger(log.New(io.Discard, "", 0)), time.Second, 2*time.Second, time.Second)

	runtime.wg.Add(1)
	runtime.trackConn(conn)
	runtime.handleConn(context.Background(), conn)

	if conn.readDeadline.IsZero() {
		t.Fatalf("expected read deadline to be set")
	}
	if conn.writeDeadline.IsZero() {
		t.Fatalf("expected write deadline to be set")
	}
}

// TestLoadServerConfigFromEnv_Defaults verifies defaults when env vars are unset.
func TestLoadServerConfigFromEnv_Defaults(t *testing.T) {
	t.Setenv("LIGHT_SERVE_PORT", "")
	t.Setenv("LIGHT_SERVE_READ_TIMEOUT", "")
	t.Setenv("LIGHT_SERVE_WRITE_TIMEOUT", "")
	t.Setenv("LIGHT_SERVE_SHUTDOWN_DEADLINE", "")
	t.Setenv("LIGHT_SERVE_REQUEST_TIMEOUT", "")

	cfg, err := loadServerConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}

	if cfg.ListenAddress != ":8080" {
		t.Fatalf("expected default listen address :8080, got %q", cfg.ListenAddress)
	}
	if cfg.ReadTimeout != defaultReadTimeout {
		t.Fatalf("expected default read timeout %s, got %s", defaultReadTimeout, cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != defaultWriteTimeout {
		t.Fatalf("expected default write timeout %s, got %s", defaultWriteTimeout, cfg.WriteTimeout)
	}
	if cfg.ShutdownDeadline != defaultShutdownDeadline {
		t.Fatalf("expected default shutdown deadline %s, got %s", defaultShutdownDeadline, cfg.ShutdownDeadline)
	}
	if cfg.RequestTimeout != defaultRequestTimeout {
		t.Fatalf("expected default request timeout %s, got %s", defaultRequestTimeout, cfg.RequestTimeout)
	}
}

// TestLoadServerConfigFromEnv_Overrides verifies valid env overrides are parsed.
func TestLoadServerConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("LIGHT_SERVE_PORT", "9090")
	t.Setenv("LIGHT_SERVE_READ_TIMEOUT", "7s")
	t.Setenv("LIGHT_SERVE_WRITE_TIMEOUT", "8s")
	t.Setenv("LIGHT_SERVE_SHUTDOWN_DEADLINE", "12s")
	t.Setenv("LIGHT_SERVE_REQUEST_TIMEOUT", "3s")

	cfg, err := loadServerConfigFromEnv()
	if err != nil {
		t.Fatalf("unexpected config error: %v", err)
	}

	if cfg.ListenAddress != ":9090" {
		t.Fatalf("expected listen address :9090, got %q", cfg.ListenAddress)
	}
	if cfg.ReadTimeout != 7*time.Second {
		t.Fatalf("expected read timeout 7s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 8*time.Second {
		t.Fatalf("expected write timeout 8s, got %s", cfg.WriteTimeout)
	}
	if cfg.ShutdownDeadline != 12*time.Second {
		t.Fatalf("expected shutdown deadline 12s, got %s", cfg.ShutdownDeadline)
	}
	if cfg.RequestTimeout != 3*time.Second {
		t.Fatalf("expected request timeout 3s, got %s", cfg.RequestTimeout)
	}
}

// TestLoadServerConfigFromEnv_InvalidValues verifies invalid env values fail fast.
func TestLoadServerConfigFromEnv_InvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		value  string
		expect string
	}{
		{name: "invalid port", key: "LIGHT_SERVE_PORT", value: "abc", expect: "invalid port"},
		{name: "port out of range", key: "LIGHT_SERVE_PORT", value: "70000", expect: "between 1 and 65535"},
		{name: "invalid duration", key: "LIGHT_SERVE_READ_TIMEOUT", value: "bad", expect: "invalid duration"},
		{name: "non-positive duration", key: "LIGHT_SERVE_REQUEST_TIMEOUT", value: "0s", expect: "must be > 0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LIGHT_SERVE_PORT", "")
			t.Setenv("LIGHT_SERVE_READ_TIMEOUT", "")
			t.Setenv("LIGHT_SERVE_WRITE_TIMEOUT", "")
			t.Setenv("LIGHT_SERVE_SHUTDOWN_DEADLINE", "")
			t.Setenv("LIGHT_SERVE_REQUEST_TIMEOUT", "")
			t.Setenv(tt.key, tt.value)

			_, err := loadServerConfigFromEnv()
			if err == nil {
				t.Fatalf("expected config error")
			}
			if !strings.Contains(err.Error(), tt.expect) {
				t.Fatalf("expected error containing %q, got %q", tt.expect, err.Error())
			}
		})
	}
}

// waitForActiveConn blocks until one connection is tracked or timeout is reached.
func waitForActiveConn(t *testing.T, runtime *serverRuntime, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		runtime.mu.Lock()
		active := len(runtime.conns)
		runtime.mu.Unlock()
		if active > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for active tracked connection")
}

// spyConn records deadlines and supports minimal net.Conn behavior for tests.
type spyConn struct {
	mu            sync.Mutex
	readDeadline  time.Time
	writeDeadline time.Time
	closed        bool
}

// Read returns EOF to let handlers complete without blocking.
func (c *spyConn) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

// Write discards bytes and reports success.
func (c *spyConn) Write(p []byte) (int, error) {
	return len(p), nil
}

// Close marks the connection as closed.
func (c *spyConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

// LocalAddr returns a dummy local address.
func (c *spyConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// RemoteAddr returns a dummy remote address.
func (c *spyConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0}
}

// SetDeadline sets both read and write deadlines.
func (c *spyConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	c.writeDeadline = t
	return nil
}

// SetReadDeadline records read deadline.
func (c *spyConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	return nil
}

// SetWriteDeadline records write deadline.
func (c *spyConn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return nil
}
