# light_serve (v1)

A custom HTTP/1.1 server built directly on top of TCP sockets in Go (without `net/http` request handling), following Clean Architecture principles.

Version 1 focuses on a stable server foundation: manual HTTP parsing, routing, middleware, graceful shutdown, env-based runtime config, and structured logging.

## What v1 can do

- Start a TCP HTTP server on a configurable port.
- Parse raw HTTP/1.1 requests into structured request objects.
- Route handlers by `METHOD:PATH`.
- Serve two starter endpoints:
  - `GET /health` -> `200 OK`, body `ok`
  - `GET /hello` -> `200 OK`, body `hello`
- Return protocol-correct fallback responses:
  - `404 Not Found` for unknown paths
  - `405 Method Not Allowed` (+ `Allow` header) when path exists but method does not
  - `400 Bad Request` for malformed requests
- Apply middleware for logging, panic recovery, and request timeout.
- Propagate request context and cancellation into handler/use-case flow.
- Gracefully shut down on `Ctrl+C` (`SIGINT`) / `SIGTERM`.

## Project layout

```text
light_serve/
├── cmd/server/main.go            # Composition root and runtime lifecycle
├── internal/
│   ├── domain/                   # Domain errors/entities
│   ├── usecase/                  # Use-case contracts and ports
│   └── adapter/
│       ├── http/                 # Parser, router, middleware, HTTP server adapter
│       ├── logging/              # Logger adapter(s)
│       └── persistence/          # Persistence adapter placeholder
└── docs/architecture.md          # Architecture design document
```

## Requirements

- Go 1.21+
- Any shell/terminal (PowerShell, cmd, bash, zsh, etc.)

## Runtime configuration (environment variables)

All values are optional. If unset, defaults are used.

- `LIGHT_SERVE_PORT` (default: `8080`)
- `LIGHT_SERVE_READ_TIMEOUT` (default: `5s`)
- `LIGHT_SERVE_WRITE_TIMEOUT` (default: `5s`)
- `LIGHT_SERVE_SHUTDOWN_DEADLINE` (default: `10s`)
- `LIGHT_SERVE_REQUEST_TIMEOUT` (default: `2s`)

Examples:

```bash
# bash/zsh (macOS/Linux)
export LIGHT_SERVE_PORT=8081
export LIGHT_SERVE_REQUEST_TIMEOUT=3s
```

```powershell
# PowerShell
$env:LIGHT_SERVE_PORT="8081"
$env:LIGHT_SERVE_REQUEST_TIMEOUT="3s"
```

```cmd
:: cmd.exe
set LIGHT_SERVE_PORT=8081
set LIGHT_SERVE_REQUEST_TIMEOUT=3s
```

## Run the server

From the project root:

```sh
go run ./cmd/server
```

You should see startup logs indicating the listening address.

## Run with Docker

Build the image from the project root:

```sh
docker build -t light-serve:v1 .
```

Run the container and publish port `8080`:

```sh
docker run --rm -p 8080:8080 --name light-serve light-serve:v1
```

Run with custom runtime config (example):

```sh
docker run --rm -p 8081:8081 \
  -e LIGHT_SERVE_PORT=8081 \
  -e LIGHT_SERVE_REQUEST_TIMEOUT=3s \
  --name light-serve light-serve:v1
```

## Test endpoints manually

Open a second terminal while the server is running.

### 1) Health endpoint

```sh
curl -i http://127.0.0.1:8080/health
```

Expected:
- status line includes `HTTP/1.1 200 OK`
- body is `ok`

### 2) Hello endpoint

```sh
curl -i http://127.0.0.1:8080/hello
```

Expected:
- status line includes `HTTP/1.1 200 OK`
- body is `hello`

### 3) Unknown path (`404`)

```sh
curl -i http://127.0.0.1:8080/not-found
```

Expected:
- `HTTP/1.1 404 Not Found`

### 4) Method not allowed (`405`)

```sh
curl -i -X POST http://127.0.0.1:8080/health
```

Expected:
- `HTTP/1.1 405 Method Not Allowed`
- `Allow: GET`

The same endpoint checks work when running in Docker (using the published host port).

## Run tests

Run all tests:

```sh
go test ./...
```

Run selected packages:

```sh
go test ./cmd/server
go test ./internal/adapter/http
go test ./internal/adapter/logging
```

If your PowerShell setup aliases `curl` to a different command, use `curl.exe` explicitly.

## Notes for v1

- This is a custom learning-oriented HTTP server implementation, not production-hardened web infrastructure yet.
- The architecture is intentionally layered and testable to support incremental additions (real use-case slices, persistence adapters, richer routing, auth, static files, etc.).
