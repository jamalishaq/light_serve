// Package usecase contains application business rules and ports (interfaces).
// Use cases depend on these interfaces, not concrete implementations.
package usecase

import "context"

// Logger is a port for logging. Adapters implement this interface.
type Logger interface {
	Info(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}

// UserRepository is a port for user persistence. Adapters implement this interface.
// Placeholder for future DB implementations.
type UserRepository interface {
	GetByID(ctx context.Context, id string) (interface{}, error)
}
