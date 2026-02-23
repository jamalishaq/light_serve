package http

import (
	"errors"
	"strconv"
	"strings"
)

const (
	maxRequestLineBytes = 4096
	maxHeadersBytes     = 16 * 1024
	maxHeaderCount      = 50
	maxBodyBytes        = 256 * 1024
)

var (
	// ErrEmptyRequest indicates the input buffer has no bytes.
	ErrEmptyRequest         = errors.New("empty request")
	// ErrIncompleteRequest indicates request headers are not fully available yet.
	ErrIncompleteRequest    = errors.New("incomplete request")
	// ErrIncompleteBody indicates the body is shorter than Content-Length.
	ErrIncompleteBody       = errors.New("incomplete body")
	// ErrMalformedRequestLine indicates an invalid request line format.
	ErrMalformedRequestLine = errors.New("malformed request line")
	// ErrInvalidHTTPVersion indicates an unsupported HTTP version.
	ErrInvalidHTTPVersion   = errors.New("invalid HTTP version")
	// ErrInvalidHeader indicates an invalid header line format.
	ErrInvalidHeader        = errors.New("invalid header")
	// ErrInvalidContentLength indicates an invalid Content-Length value.
	ErrInvalidContentLength = errors.New("invalid Content-Length")
	// ErrRequestLineTooLong indicates request line exceeds parser limits.
	ErrRequestLineTooLong   = errors.New("request line too long")
	// ErrHeadersTooLarge indicates headers exceed parser limits.
	ErrHeadersTooLarge      = errors.New("headers too large")
	// ErrTooManyHeaders indicates header count exceeds parser limits.
	ErrTooManyHeaders       = errors.New("too many headers")
	// ErrBodyTooLarge indicates body size exceeds parser limits.
	ErrBodyTooLarge         = errors.New("body too large")
)

// ParseRequest parses a raw HTTP request from bytes.
// It returns the parsed request, bytes consumed, and an error.
func ParseRequest(data []byte) (*Request, int, error) {
	if len(data) == 0 {
		return nil, 0, ErrEmptyRequest
	}
	headerEnd, delimiterLen := findHeaderDelimiter(data)
	if len(data) > maxHeadersBytes && headerEnd < 0 {
		return nil, 0, ErrHeadersTooLarge
	}
	if headerEnd < 0 {
		return nil, 0, ErrIncompleteRequest
	}
	if headerEnd > maxHeadersBytes {
		return nil, 0, ErrHeadersTooLarge
	}

	head := string(data[:headerEnd])
	lines := splitLines(head)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) == "" {
		return nil, 0, ErrMalformedRequestLine
	}
	if len(lines[0]) > maxRequestLineBytes {
		return nil, 0, ErrRequestLineTooLong
	}

	method, path, version, err := parseRequestLine(lines[0])
	if err != nil {
		return nil, 0, err
	}

	headers := make(map[string]string)
	headerCount := 0
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		headerCount++
		if headerCount > maxHeaderCount {
			return nil, 0, ErrTooManyHeaders
		}

		colon := strings.Index(line, ":")
		if colon <= 0 {
			return nil, 0, ErrInvalidHeader
		}

		key := strings.ToLower(strings.TrimSpace(line[:colon]))
		value := strings.TrimSpace(line[colon+1:])
		if key == "" {
			return nil, 0, ErrInvalidHeader
		}

		headers[key] = value
	}

	bodyStart := headerEnd + delimiterLen
	if bodyStart > len(data) {
		return nil, 0, ErrIncompleteRequest
	}

	contentLength := 0
	if rawLen, ok := headers["content-length"]; ok {
		if rawLen == "" {
			return nil, 0, ErrInvalidContentLength
		}

		n, convErr := strconv.Atoi(rawLen)
		if convErr != nil || n < 0 {
			return nil, 0, ErrInvalidContentLength
		}
		if n > maxBodyBytes {
			return nil, 0, ErrBodyTooLarge
		}
		contentLength = n
	}

	if len(data)-bodyStart < contentLength {
		return nil, 0, ErrIncompleteBody
	}

	body := make([]byte, contentLength)
	copy(body, data[bodyStart:bodyStart+contentLength])

	req := &Request{
		Method:  method,
		Path:    path,
		Version: version,
		Headers: headers,
		Body:    body,
	}

	return req, bodyStart + contentLength, nil
}

// findHeaderDelimiter locates the end of the HTTP headers and delimiter length.
func findHeaderDelimiter(data []byte) (int, int) {
	crlf := strings.Index(string(data), "\r\n\r\n")
	lf := strings.Index(string(data), "\n\n")

	switch {
	case crlf >= 0 && lf >= 0:
		if crlf < lf {
			return crlf, 4
		}
		return lf, 2
	case crlf >= 0:
		return crlf, 4
	case lf >= 0:
		return lf, 2
	default:
		return -1, 0
	}
}

// splitLines normalizes line endings and splits the header block into lines.
func splitLines(head string) []string {
	normalized := strings.ReplaceAll(head, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}

// parseRequestLine parses and validates an HTTP request line.
func parseRequestLine(line string) (string, string, string, error) {
	parts := strings.Fields(line)
	if len(parts) != 3 {
		return "", "", "", ErrMalformedRequestLine
	}

	method := parts[0]
	path := parts[1]
	version := parts[2]

	if version != "HTTP/1.1" && version != "HTTP/1.0" {
		return "", "", "", ErrInvalidHTTPVersion
	}

	return method, path, version, nil
}
