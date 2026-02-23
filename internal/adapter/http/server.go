// Package http provides the HTTP adapter: TCP listener, parser, router,
// middleware, and handler adapters. Part of the Interface Adapters layer.
package http

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
)

const readChunkSize = 4096
var defaultRouter = NewRouter()

// HandleConn reads one HTTP request from a connection and writes one response.
func HandleConn(conn net.Conn) {
	HandleConnWithContext(conn, context.Background())
}

// HandleConnWithContext reads one HTTP request with an explicit request context.
func HandleConnWithContext(conn net.Conn, ctx context.Context) {
	HandleConnWithRouterAndContext(conn, defaultRouter, ctx)
}

// HandleConnWithRouter reads one HTTP request from a connection and routes it.
func HandleConnWithRouter(conn net.Conn, router *Router) {
	HandleConnWithRouterAndContext(conn, router, context.Background())
}

// HandleConnWithRouterAndContext reads one HTTP request and routes it with context.
func HandleConnWithRouterAndContext(conn net.Conn, router *Router, ctx context.Context) {
	defer conn.Close()

	buffer := make([]byte, 0, readChunkSize)
	chunk := make([]byte, readChunkSize)

	for {
		for len(buffer) > 0 {
			req, consumed, parseErr := ParseRequest(buffer)
			if parseErr == nil {
				if req != nil {
					req.Ctx = ctx
				}

				closeConn := writeRoutedResponse(conn, router, req)
				if consumed > len(buffer) {
					return
				}
				buffer = buffer[consumed:]
				if closeConn {
					return
				}
				continue
			}

			if isIncompleteParseErr(parseErr) {
				break
			}

			writeBadRequest(conn)
			return
		}

		n, readErr := conn.Read(chunk)
		if n > 0 {
			buffer = append(buffer, chunk[:n]...)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				if len(buffer) == 0 {
					return
				}
				writeBadRequest(conn)
				return
			}

			writeBadRequest(conn)
			return
		}
	}
}

// RegisterRoute registers a METHOD:PATH handler on the default router.
func RegisterRoute(method, path string, handler HandlerAdapter) {
	defaultRouter.Register(method, path, handler)
}

// UseMiddleware registers middleware on the default router.
func UseMiddleware(middlewares ...Middleware) {
	defaultRouter.Use(middlewares...)
}

// isIncompleteParseErr reports whether more bytes may complete the request.
func isIncompleteParseErr(err error) bool {
	return errors.Is(err, ErrIncompleteRequest) || errors.Is(err, ErrIncompleteBody)
}

// writeBadRequest writes a 400 Bad Request response.
func writeBadRequest(conn net.Conn) {
	resp := NewResponse()
	resp.StatusCode = 400
	resp.SetHeader("Content-Type", "text/plain")
	resp.SetHeader("Connection", "close")
	resp.WriteString("Bad Request")
	_, _ = conn.Write(resp.Bytes())
}

// writeRoutedResponse routes a request and writes the resulting response.
func writeRoutedResponse(conn net.Conn, router *Router, req *Request) bool {
	closeConn := shouldCloseConnection(req)

	if router == nil {
		writeNotFound(conn, closeConn)
		return closeConn
	}

	handler, ok := router.Resolve(req.Method, req.Path)
	if !ok || handler == nil {
		allowed := router.AllowedMethods(req.Path)
		if len(allowed) > 0 {
			writeMethodNotAllowed(conn, allowed, closeConn)
			return closeConn
		}
		writeNotFound(conn, closeConn)
		return closeConn
	}

	resp := handler(req)
	if resp == nil {
		resp = NewResponse()
		resp.StatusCode = 500
		resp.SetHeader("Content-Type", "text/plain")
		resp.WriteString("Internal Server Error")
	}
	setConnectionHeader(resp, closeConn)

	_, _ = conn.Write(resp.Bytes())
	return closeConn
}

// writeNotFound writes a 404 Not Found response.
func writeNotFound(conn net.Conn, closeConn bool) {
	resp := NewResponse()
	resp.StatusCode = 404
	resp.SetHeader("Content-Type", "text/plain")
	setConnectionHeader(resp, closeConn)
	resp.WriteString("Not Found")
	_, _ = conn.Write(resp.Bytes())
}

// writeMethodNotAllowed writes a 405 Method Not Allowed response with Allow header.
func writeMethodNotAllowed(conn net.Conn, allowed []string, closeConn bool) {
	resp := NewResponse()
	resp.StatusCode = 405
	resp.SetHeader("Content-Type", "text/plain")
	resp.SetHeader("Allow", strings.Join(allowed, ", "))
	setConnectionHeader(resp, closeConn)
	resp.WriteString("Method Not Allowed")
	_, _ = conn.Write(resp.Bytes())
}

// shouldCloseConnection determines whether to close the TCP connection after response.
func shouldCloseConnection(req *Request) bool {
	if req == nil {
		return true
	}

	connection := strings.ToLower(strings.TrimSpace(req.Headers["connection"]))
	if req.Version == "HTTP/1.1" {
		return connection == "close"
	}
	if req.Version == "HTTP/1.0" {
		return connection != "keep-alive"
	}
	return true
}

// setConnectionHeader sets the response Connection header to match policy.
func setConnectionHeader(resp *Response, closeConn bool) {
	if resp == nil {
		return
	}
	if closeConn {
		resp.SetHeader("Connection", "close")
		return
	}
	resp.SetHeader("Connection", "keep-alive")
}
