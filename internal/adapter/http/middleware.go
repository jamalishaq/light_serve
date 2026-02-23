package http

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jamalishaq/light_serve/internal/usecase"
)

// LoggingMiddleware logs method, path, status code, and request duration.
func LoggingMiddleware(logger usecase.Logger) Middleware {
	return func(next HandlerAdapter) HandlerAdapter {
		return func(req *Request) *Response {
			startedAt := time.Now()
			resp := safeInvoke(next, req)
			duration := time.Since(startedAt)

			method := ""
			path := ""
			if req != nil {
				method = req.Method
				path = req.Path
			}

			statusCode := resp.StatusCode
			if statusCode == 0 {
				statusCode = 200
			}

			requestID, correlationID := requestIdentifiers(req)
			logInfo(logger, "http request",
				"method", method,
				"path", path,
				"status", statusCode,
				"duration", duration.String(),
				"request_id", requestID,
				"correlation_id", correlationID,
			)
			return resp
		}
	}
}

// RecoveryMiddleware recovers panics from downstream handlers and returns 500.
func RecoveryMiddleware(logger usecase.Logger) Middleware {
	return func(next HandlerAdapter) HandlerAdapter {
		return func(req *Request) (resp *Response) {
			defer func() {
				if recovered := recover(); recovered != nil {
					requestID, correlationID := requestIdentifiers(req)
					logError(logger, "panic recovered",
						"method", requestMethod(req),
						"path", requestPath(req),
						"panic", recovered,
						"request_id", requestID,
						"correlation_id", correlationID,
					)

					resp = NewResponse()
					resp.StatusCode = 500
					resp.SetHeader("Content-Type", "text/plain")
					resp.WriteString("Internal Server Error")
				}
			}()

			return safeInvoke(next, req)
		}
	}
}

// TimeoutMiddleware returns 408 when downstream handling exceeds the timeout.
func TimeoutMiddleware(timeout time.Duration) Middleware {
	return func(next HandlerAdapter) HandlerAdapter {
		return func(req *Request) *Response {
			if timeout <= 0 {
				return safeInvoke(next, req)
			}

			timeoutCtx, cancel := context.WithTimeout(requestContext(req), timeout)
			defer cancel()

			reqWithTimeout := withRequestContext(req, timeoutCtx)
			responseCh := make(chan *Response, 1)
			panicCh := make(chan any, 1)

			go func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						panicCh <- recovered
					}
				}()
				responseCh <- safeInvoke(next, reqWithTimeout)
			}()

			select {
			case recovered := <-panicCh:
				_ = recovered
				resp := NewResponse()
				resp.StatusCode = 500
				resp.SetHeader("Content-Type", "text/plain")
				resp.WriteString("Internal Server Error")
				return resp
			case resp := <-responseCh:
				return safeResponse(resp)
			case <-timeoutCtx.Done():
				if !errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
					return internalServerErrorResponse()
				}
				resp := NewResponse()
				resp.StatusCode = 408
				resp.SetHeader("Content-Type", "text/plain")
				resp.WriteString("Request Timeout")
				return resp
			}
		}
	}
}

// requestContext returns req.Context(), tolerating nil request values.
func requestContext(req *Request) context.Context {
	if req == nil {
		return context.Background()
	}
	return req.Context()
}

// withRequestContext clones req with the provided context.
func withRequestContext(req *Request, ctx context.Context) *Request {
	if req == nil {
		return &Request{Ctx: ctx}
	}
	cloned := *req
	cloned.Ctx = ctx
	return &cloned
}

// safeInvoke executes the next handler and guarantees a non-nil response.
func safeInvoke(next HandlerAdapter, req *Request) *Response {
	if next == nil {
		return internalServerErrorResponse()
	}

	return safeResponse(next(req))
}

// safeResponse normalizes nil responses to 500 Internal Server Error.
func safeResponse(resp *Response) *Response {
	if resp != nil {
		return resp
	}
	return internalServerErrorResponse()
}

// requestMethod extracts the method from the request safely.
func requestMethod(req *Request) string {
	if req == nil {
		return ""
	}
	return req.Method
}

// requestPath extracts the path from the request safely.
func requestPath(req *Request) string {
	if req == nil {
		return ""
	}
	return req.Path
}

// requestIdentifiers extracts request/correlation IDs from headers.
func requestIdentifiers(req *Request) (string, string) {
	if req == nil || req.Headers == nil {
		return "", ""
	}
	return strings.TrimSpace(req.Headers["x-request-id"]), strings.TrimSpace(req.Headers["x-correlation-id"])
}

// logInfo logs an info event when a logger is provided.
func logInfo(logger usecase.Logger, msg string, keysAndValues ...any) {
	if logger == nil {
		return
	}
	logger.Info(msg, keysAndValues...)
}

// logError logs an error event when a logger is provided.
func logError(logger usecase.Logger, msg string, keysAndValues ...any) {
	if logger == nil {
		return
	}
	logger.Error(msg, keysAndValues...)
}
