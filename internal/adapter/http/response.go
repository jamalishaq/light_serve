package http

import (
	"bytes"
	"strconv"
	"strings"
)

// Response is an HTTP response model used by the HTTP adapter layer.
type Response struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// NewResponse creates a response with default values.
func NewResponse() *Response {
	return &Response{
		StatusCode: 200,
		Headers:    make(map[string]string),
		Body:       []byte{},
	}
}

// SetHeader sets a response header value, initializing the map if needed.
func (r *Response) SetHeader(key, value string) {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}
	r.Headers[key] = value
}

// WriteBytes replaces the response body with the provided bytes.
func (r *Response) WriteBytes(body []byte) {
	r.Body = make([]byte, len(body))
	copy(r.Body, body)
}

// WriteString replaces the response body with the provided string.
func (r *Response) WriteString(body string) {
	r.Body = []byte(body)
}

// Bytes serializes the response to HTTP/1.1 wire format.
func (r *Response) Bytes() []byte {
	if r.Headers == nil {
		r.Headers = make(map[string]string)
	}

	if !hasHeaderIgnoreCase(r.Headers, "Content-Length") {
		r.Headers["Content-Length"] = strconv.Itoa(len(r.Body))
	}

	var buf bytes.Buffer
	buf.WriteString("HTTP/1.1 ")
	buf.WriteString(strconv.Itoa(r.StatusCode))
	buf.WriteString(" ")
	buf.WriteString(statusText(r.StatusCode))
	buf.WriteString("\r\n")

	for key, value := range r.Headers {
		buf.WriteString(key)
		buf.WriteString(": ")
		buf.WriteString(value)
		buf.WriteString("\r\n")
	}

	buf.WriteString("\r\n")
	buf.Write(r.Body)
	return buf.Bytes()
}

// statusText returns a reason phrase for a status code.
func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 408:
		return "Request Timeout"
	case 500:
		return "Internal Server Error"
	default:
		return "Unknown"
	}
}

// hasHeaderIgnoreCase reports whether a header exists by case-insensitive key.
func hasHeaderIgnoreCase(headers map[string]string, target string) bool {
	for key := range headers {
		if strings.EqualFold(key, target) {
			return true
		}
	}
	return false
}
