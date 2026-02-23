// Package domain contains enterprise business rules: entities and domain errors.
// It has no knowledge of HTTP, TCP, or any transport.
package domain

import "errors"

// Domain errors are transport-agnostic. Adapters map these to HTTP status codes.
var (
	// ErrNotFound indicates a requested domain resource was not found.
	ErrNotFound      = errors.New("not found")
	// ErrUnauthorized indicates the caller is not authorized to perform the action.
	ErrUnauthorized  = errors.New("unauthorized")
	// ErrBadRequest indicates invalid domain input.
	ErrBadRequest    = errors.New("bad request")
)
