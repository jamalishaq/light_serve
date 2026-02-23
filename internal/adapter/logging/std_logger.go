// Package logging provides concrete logger adapters.
package logging

import (
	"fmt"
	"log"
	"strings"

	"github.com/jamalishaq/light_serve/internal/usecase"
)

// stdLogger adapts log.Logger to the usecase.Logger port.
type stdLogger struct {
	base *log.Logger
}

// NewStdLogger creates a logger adapter backed by a standard logger.
func NewStdLogger(base *log.Logger) usecase.Logger {
	return &stdLogger{base: base}
}

// Info logs informational events.
func (l *stdLogger) Info(msg string, keysAndValues ...any) {
	if l == nil || l.base == nil {
		return
	}
	fields := formatKeyValues(keysAndValues...)
	if fields == "" {
		l.base.Printf("level=INFO msg=%q", msg)
		return
	}
	l.base.Printf("level=INFO msg=%q %s", msg, fields)
}

// Error logs error events.
func (l *stdLogger) Error(msg string, keysAndValues ...any) {
	if l == nil || l.base == nil {
		return
	}
	fields := formatKeyValues(keysAndValues...)
	if fields == "" {
		l.base.Printf("level=ERROR msg=%q", msg)
		return
	}
	l.base.Printf("level=ERROR msg=%q %s", msg, fields)
}

// formatKeyValues renders key/value pairs into a log-friendly string.
func formatKeyValues(keysAndValues ...any) string {
	if len(keysAndValues) == 0 {
		return ""
	}

	parts := make([]string, 0, len(keysAndValues)/2+1)
	for i := 0; i < len(keysAndValues); i += 2 {
		key := fmt.Sprintf("field_%d", i/2)
		value := any("<missing>")
		if i < len(keysAndValues) {
			key = fmt.Sprint(keysAndValues[i])
		}
		if i+1 < len(keysAndValues) {
			value = keysAndValues[i+1]
		}
		key = sanitizeKey(key, i/2)
		parts = append(parts, fmt.Sprintf("%s=%v", key, value))
	}
	return strings.Join(parts, " ")
}

// sanitizeKey normalizes logging keys and applies deterministic fallbacks.
func sanitizeKey(key string, index int) string {
	normalized := strings.TrimSpace(strings.ToLower(strings.ReplaceAll(key, " ", "_")))
	if normalized == "" {
		return fmt.Sprintf("field_%d", index)
	}
	return normalized
}
