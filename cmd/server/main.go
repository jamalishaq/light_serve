// Package main runs the TCP entrypoint for the HTTP adapter server.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	httpadapter "github.com/jamalishaq/light_serve/internal/adapter/http"
	logadapter "github.com/jamalishaq/light_serve/internal/adapter/logging"
	"github.com/jamalishaq/light_serve/internal/usecase"
)

const (
	defaultPort             = 8080
	defaultReadTimeout      = 5 * time.Second
	defaultWriteTimeout     = 5 * time.Second
	defaultShutdownDeadline = 10 * time.Second
	defaultRequestTimeout   = 2 * time.Second
)

// serverConfig configures runtime behavior from environment values.
type serverConfig struct {
	ListenAddress    string
	ReadTimeout      time.Duration
	WriteTimeout     time.Duration
	ShutdownDeadline time.Duration
	RequestTimeout   time.Duration
	TLSCertFile      string
	TLSKeyFile       string
	TLSMinVersion    uint16
}

// main starts the TCP listener and accepts incoming HTTP connections.
func main() {
	cfg, err := loadServerConfigFromEnv()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	structuredLogger := logadapter.NewStdLogger(log.Default())
	httpadapter.UseMiddleware(
		httpadapter.LoggingMiddleware(structuredLogger),
		httpadapter.TimeoutMiddleware(cfg.RequestTimeout),
		httpadapter.RecoveryMiddleware(structuredLogger),
	)

	httpadapter.RegisterRoute("GET", "/health", func(req *httpadapter.Request) *httpadapter.Response {
		resp := httpadapter.NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("ok")
		return resp
	})

	httpadapter.RegisterRoute("GET", "/hello", func(req *httpadapter.Request) *httpadapter.Response {
		resp := httpadapter.NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("hello")
		return resp
	})

	httpadapter.RegisterRoute("GET", "/", func(req *httpadapter.Request) *httpadapter.Response {
		resp := httpadapter.NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("ok")
		return resp
	})

	tlsCertificate, err := tls.LoadX509KeyPair(cfg.TLSCertFile, cfg.TLSKeyFile)
	if err != nil {
		log.Fatalf("tls certificate: %v", err)
	}
	tlsConfig := &tls.Config{
		MinVersion:   cfg.TLSMinVersion,
		Certificates: []tls.Certificate{tlsCertificate},
	}

	listener, err := tls.Listen("tcp", cfg.ListenAddress, tlsConfig)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	structuredLogger.Info("https adapter server listening", "address", cfg.ListenAddress, "tls_min_version", tlsVersionName(cfg.TLSMinVersion))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runtime := newServerRuntime(listener, structuredLogger, cfg.ReadTimeout, cfg.WriteTimeout, cfg.ShutdownDeadline)
	if err := runtime.serve(ctx); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

// loadServerConfigFromEnv loads runtime configuration from LIGHT_SERVE_* vars.
func loadServerConfigFromEnv() (serverConfig, error) {
	port, err := parsePortEnv("LIGHT_SERVE_PORT", defaultPort)
	if err != nil {
		return serverConfig{}, err
	}

	readTimeout, err := parseDurationEnv("LIGHT_SERVE_READ_TIMEOUT", defaultReadTimeout)
	if err != nil {
		return serverConfig{}, err
	}
	writeTimeout, err := parseDurationEnv("LIGHT_SERVE_WRITE_TIMEOUT", defaultWriteTimeout)
	if err != nil {
		return serverConfig{}, err
	}
	shutdownDeadline, err := parseDurationEnv("LIGHT_SERVE_SHUTDOWN_DEADLINE", defaultShutdownDeadline)
	if err != nil {
		return serverConfig{}, err
	}
	requestTimeout, err := parseDurationEnv("LIGHT_SERVE_REQUEST_TIMEOUT", defaultRequestTimeout)
	if err != nil {
		return serverConfig{}, err
	}
	tlsCertFile, err := parseRequiredFileEnv("LIGHT_SERVE_TLS_CERT_FILE")
	if err != nil {
		return serverConfig{}, err
	}
	tlsKeyFile, err := parseRequiredFileEnv("LIGHT_SERVE_TLS_KEY_FILE")
	if err != nil {
		return serverConfig{}, err
	}
	tlsMinVersion, err := parseTLSMinVersionEnv("LIGHT_SERVE_TLS_MIN_VERSION", tls.VersionTLS13)
	if err != nil {
		return serverConfig{}, err
	}

	return serverConfig{
		ListenAddress:    ":" + strconv.Itoa(port),
		ReadTimeout:      readTimeout,
		WriteTimeout:     writeTimeout,
		ShutdownDeadline: shutdownDeadline,
		RequestTimeout:   requestTimeout,
		TLSCertFile:      tlsCertFile,
		TLSKeyFile:       tlsKeyFile,
		TLSMinVersion:    tlsMinVersion,
	}, nil
}

// parseDurationEnv reads a duration env var with fallback default.
func parseDurationEnv(envKey string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", envKey, raw, err)
	}
	if value <= 0 {
		return 0, fmt.Errorf("%s: duration must be > 0", envKey)
	}
	return value, nil
}

// parsePortEnv reads and validates a TCP port env var.
func parsePortEnv(envKey string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return fallback, nil
	}

	raw = strings.TrimPrefix(raw, ":")
	port, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid port %q", envKey, raw)
	}
	if port < 1 || port > 65535 {
		return 0, fmt.Errorf("%s: port must be between 1 and 65535", envKey)
	}
	return port, nil
}

// parseRequiredFileEnv reads a required file path env var and checks existence.
func parseRequiredFileEnv(envKey string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return "", fmt.Errorf("%s: value is required", envKey)
	}
	if _, err := os.Stat(raw); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s: file does not exist: %s", envKey, raw)
		}
		return "", fmt.Errorf("%s: file check failed: %w", envKey, err)
	}
	return raw, nil
}

// parseTLSMinVersionEnv reads TLS minimum version from env with fallback.
func parseTLSMinVersionEnv(envKey string, fallback uint16) (uint16, error) {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(envKey)))
	if raw == "" {
		return fallback, nil
	}
	switch raw {
	case "1.2", "tls1.2":
		return tls.VersionTLS12, nil
	case "1.3", "tls1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("%s: invalid value %q (allowed: 1.2, 1.3)", envKey, raw)
	}
}

// tlsVersionName renders TLS version constants for structured logs.
func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "1.3"
	case tls.VersionTLS12:
		return "1.2"
	default:
		return fmt.Sprintf("0x%x", version)
	}
}

// serverRuntime owns accept loop and graceful shutdown lifecycle.
type serverRuntime struct {
	listener         net.Listener
	logger           usecase.Logger
	readTimeout      time.Duration
	writeTimeout     time.Duration
	shutdownDeadline time.Duration

	wg    sync.WaitGroup
	mu    sync.Mutex
	conns map[net.Conn]struct{}
}

// newServerRuntime constructs a runtime with lifecycle and timeout settings.
func newServerRuntime(listener net.Listener, logger usecase.Logger, readTimeout, writeTimeout, shutdownDeadline time.Duration) *serverRuntime {
	return &serverRuntime{
		listener:         listener,
		logger:           logger,
		readTimeout:      readTimeout,
		writeTimeout:     writeTimeout,
		shutdownDeadline: shutdownDeadline,
		conns:            make(map[net.Conn]struct{}),
	}
}

// serve accepts connections until context cancellation, then drains active work.
func (s *serverRuntime) serve(ctx context.Context) error {
	defer s.listener.Close()

	go func() {
		<-ctx.Done()
		logRuntimeInfo(s.logger, "shutdown signal received", "action", "stop_accepts")
		_ = s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			logRuntimeError(s.logger, "accept failed", "error", err)
			continue
		}

		s.trackConn(conn)
		s.wg.Add(1)
		go s.handleConn(ctx, conn)
	}

	logRuntimeInfo(s.logger, "waiting for in-flight connections")
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logRuntimeInfo(s.logger, "shutdown complete")
	case <-time.After(s.shutdownDeadline):
		logRuntimeError(s.logger, "shutdown deadline reached", "deadline", s.shutdownDeadline.String(), "action", "force_close_active_connections")
		s.closeTrackedConns()
		<-done
		logRuntimeInfo(s.logger, "shutdown complete after forced close")
	}

	return nil
}

// handleConn sets per-connection deadlines and delegates request handling.
func (s *serverRuntime) handleConn(ctx context.Context, conn net.Conn) {
	defer s.wg.Done()
	defer s.untrackConn(conn)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-done:
		}
	}()

	if s.readTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(s.readTimeout))
	}
	if s.writeTimeout > 0 {
		_ = conn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
	}

	httpadapter.HandleConnWithContext(conn, ctx)
}

// trackConn adds a connection to the active set.
func (s *serverRuntime) trackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conns[conn] = struct{}{}
}

// untrackConn removes a connection from the active set.
func (s *serverRuntime) untrackConn(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.conns, conn)
}

// closeTrackedConns force closes all currently tracked active connections.
func (s *serverRuntime) closeTrackedConns() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for conn := range s.conns {
		_ = conn.Close()
	}
}

// logRuntimeInfo logs runtime lifecycle events when a logger is configured.
func logRuntimeInfo(logger usecase.Logger, msg string, keysAndValues ...any) {
	if logger == nil {
		return
	}
	logger.Info(msg, keysAndValues...)
}

// logRuntimeError logs runtime errors when a logger is configured.
func logRuntimeError(logger usecase.Logger, msg string, keysAndValues ...any) {
	if logger == nil {
		return
	}
	logger.Error(msg, keysAndValues...)
}
