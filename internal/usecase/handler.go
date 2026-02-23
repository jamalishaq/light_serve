package usecase

import "context"

// Handler is a transport-agnostic handler interface.
// HTTP adapters translate between HTTP and this interface.
type Handler interface {
	Handle(ctx context.Context, input RequestInput) (ResponseOutput, error)
}

// RequestInput is the input to a use case. Transport-agnostic.
type RequestInput struct {
	Path    string
	Headers map[string]string
	Body    []byte
}

// ResponseOutput is the output from a use case. Transport-agnostic.
type ResponseOutput struct {
	Body []byte
}
