package http

import "context"

// Request is a parsed HTTP request.
type Request struct {
	Ctx     context.Context
	Method  string
	Path    string
	Version string
	Headers map[string]string
	Body    []byte
}

// Context returns the request context or Background when unset.
func (r *Request) Context() context.Context {
	if r == nil || r.Ctx == nil {
		return context.Background()
	}
	return r.Ctx
}
