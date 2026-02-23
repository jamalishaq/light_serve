package http

import (
	"errors"

	"github.com/jamalishaq/light_serve/internal/domain"
	"github.com/jamalishaq/light_serve/internal/usecase"
)

// AdaptUseCaseHandler translates HTTP requests to use case input and back to HTTP responses.
func AdaptUseCaseHandler(handler usecase.Handler) HandlerAdapter {
	return func(req *Request) *Response {
		if handler == nil {
			return internalServerErrorResponse()
		}

		input := toUseCaseInput(req)
		output, err := handler.Handle(req.Context(), input)
		if err != nil {
			return mapUseCaseError(err)
		}

		resp := NewResponse()
		resp.StatusCode = 200
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteBytes(output.Body)
		return resp
	}
}

// toUseCaseInput converts an HTTP request into transport-agnostic use case input.
func toUseCaseInput(req *Request) usecase.RequestInput {
	input := usecase.RequestInput{}

	if req != nil {
		input.Path = req.Path
		input.Headers = copyHeaders(req.Headers)
		input.Body = copyBody(req.Body)
	}

	return input
}

// copyHeaders clones header values to avoid sharing mutable maps across layers.
func copyHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}

	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		cloned[key] = value
	}
	return cloned
}

// copyBody clones request body bytes to preserve adapter/use-case boundaries.
func copyBody(body []byte) []byte {
	if body == nil {
		return nil
	}

	cloned := make([]byte, len(body))
	copy(cloned, body)
	return cloned
}

// mapUseCaseError maps domain and application errors to HTTP responses.
func mapUseCaseError(err error) *Response {
	resp := NewResponse()
	resp.SetHeader("Content-Type", "text/plain")

	switch {
	case errors.Is(err, domain.ErrBadRequest):
		resp.StatusCode = 400
		resp.WriteString("Bad Request")
	case errors.Is(err, domain.ErrUnauthorized):
		resp.StatusCode = 401
		resp.WriteString("Unauthorized")
	case errors.Is(err, domain.ErrNotFound):
		resp.StatusCode = 404
		resp.WriteString("Not Found")
	default:
		resp.StatusCode = 500
		resp.WriteString("Internal Server Error")
	}

	return resp
}

// internalServerErrorResponse returns a generic 500 response.
func internalServerErrorResponse() *Response {
	resp := NewResponse()
	resp.StatusCode = 500
	resp.SetHeader("Content-Type", "text/plain")
	resp.WriteString("Internal Server Error")
	return resp
}
