package logging

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

// TestStdLogger_InfoWithoutFields verifies no trailing empty field segment is emitted.
func TestStdLogger_InfoWithoutFields(t *testing.T) {
	var buffer bytes.Buffer
	logger := NewStdLogger(log.New(&buffer, "", 0))

	logger.Info("startup complete")

	entry := strings.TrimSpace(buffer.String())
	expected := `level=INFO msg="startup complete"`
	if entry != expected {
		t.Fatalf("expected %q, got %q", expected, entry)
	}
}

// TestFormatKeyValues_OddPairCountUsesMissingValue verifies missing values are explicit.
func TestFormatKeyValues_OddPairCountUsesMissingValue(t *testing.T) {
	fields := formatKeyValues("method", "GET", "status")
	if !strings.Contains(fields, "method=GET") {
		t.Fatalf("expected method field, got %q", fields)
	}
	if !strings.Contains(fields, "status=<missing>") {
		t.Fatalf("expected missing placeholder field, got %q", fields)
	}
}

// TestFormatKeyValues_EmptyKeyUsesIndexedFallback verifies deterministic fallback keys.
func TestFormatKeyValues_EmptyKeyUsesIndexedFallback(t *testing.T) {
	fields := formatKeyValues("", "first", "", "second")
	if !strings.Contains(fields, "field_0=first") {
		t.Fatalf("expected field_0 fallback, got %q", fields)
	}
	if !strings.Contains(fields, "field_1=second") {
		t.Fatalf("expected field_1 fallback, got %q", fields)
	}
}

// TestFormatKeyValues_NonStringKeyIsStringified verifies non-string keys are handled.
func TestFormatKeyValues_NonStringKeyIsStringified(t *testing.T) {
	fields := formatKeyValues(42, "answer")
	if !strings.Contains(fields, "42=answer") {
		t.Fatalf("expected stringified numeric key, got %q", fields)
	}
}
