# light_serve (v1)

A custom HTTP/1.1 server built directly on top of TCP sockets in Go (without `net/http` request handling), following Clean Architecture principles.

Version 1 focuses on a stable server foundation: manual HTTP parsing, routing, middleware, graceful shutdown, HTTPS-only runtime config, and structured logging.

## What v1 can do

- Start an HTTPS-only server on a configurable port.
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
- `LIGHT_SERVE_TLS_CERT_FILE` (required)
- `LIGHT_SERVE_TLS_KEY_FILE` (required)
- `LIGHT_SERVE_TLS_MIN_VERSION` (optional, default: `1.3`, allowed: `1.2`, `1.3`)

Examples:

```bash
# bash/zsh (macOS/Linux)
export LIGHT_SERVE_PORT=8081
export LIGHT_SERVE_REQUEST_TIMEOUT=3s
export LIGHT_SERVE_TLS_CERT_FILE=/absolute/path/cert.pem
export LIGHT_SERVE_TLS_KEY_FILE=/absolute/path/key.pem
```

```powershell
# PowerShell
$env:LIGHT_SERVE_PORT="8081"
$env:LIGHT_SERVE_REQUEST_TIMEOUT="3s"
$env:LIGHT_SERVE_TLS_CERT_FILE="C:\path\to\cert.pem"
$env:LIGHT_SERVE_TLS_KEY_FILE="C:\path\to\key.pem"
```

```cmd
:: cmd.exe
set LIGHT_SERVE_PORT=8081
set LIGHT_SERVE_REQUEST_TIMEOUT=3s
set LIGHT_SERVE_TLS_CERT_FILE=C:\path\to\cert.pem
set LIGHT_SERVE_TLS_KEY_FILE=C:\path\to\key.pem
```

## Generate local TLS key pair (OpenSSL)

Create a local self-signed certificate for `localhost` and `127.0.0.1`.

```powershell
# PowerShell (Windows)
& "C:\Program Files\Git\usr\bin\openssl.exe" req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes `
  -keyout "$PWD\certs\key.pem" `
  -out "$PWD\certs\cert.pem" `
  -subj "/CN=localhost" `
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

```bash
# bash/zsh (Linux/macOS)
mkdir -p certs
openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
  -keyout certs/key.pem \
  -out certs/cert.pem \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

Then set absolute TLS file paths before running the server:

```powershell
$env:LIGHT_SERVE_TLS_CERT_FILE="$PWD\certs\cert.pem"
$env:LIGHT_SERVE_TLS_KEY_FILE="$PWD\certs\key.pem"
$env:LIGHT_SERVE_TLS_MIN_VERSION="1.3"
```

```bash
export LIGHT_SERVE_TLS_CERT_FILE="$(pwd)/certs/cert.pem"
export LIGHT_SERVE_TLS_KEY_FILE="$(pwd)/certs/key.pem"
export LIGHT_SERVE_TLS_MIN_VERSION=1.3
```

## Run the server (HTTPS only)

From the project root:

```sh
go run ./cmd/server
```

You should see startup logs indicating the HTTPS listening address.
For local self-signed certs, use `curl -k` in the test commands below.

## Run with Docker

Build the image from the project root:

```sh
docker build -t light-serve:v1 .
```

Run the container and publish port `8080`:

```sh
docker run --rm -p 8080:8080 \
  -v /absolute/path/certs:/certs:ro \
  -e LIGHT_SERVE_TLS_CERT_FILE=/certs/cert.pem \
  -e LIGHT_SERVE_TLS_KEY_FILE=/certs/key.pem \
  --name light-serve light-serve:v1
```

Run with custom runtime config (example):

```sh
docker run --rm -p 8081:8081 \
  -v /absolute/path/certs:/certs:ro \
  -e LIGHT_SERVE_PORT=8081 \
  -e LIGHT_SERVE_REQUEST_TIMEOUT=3s \
  -e LIGHT_SERVE_TLS_CERT_FILE=/certs/cert.pem \
  -e LIGHT_SERVE_TLS_KEY_FILE=/certs/key.pem \
  --name light-serve light-serve:v1
```

## Test endpoints manually

Open a second terminal while the server is running.

### 1) Health endpoint

```sh
curl -k -i https://127.0.0.1:8080/health
```

Expected:
- status line includes `HTTP/1.1 200 OK`
- body is `ok`

### 2) Hello endpoint

```sh
curl -k -i https://127.0.0.1:8080/hello
```

Expected:
- status line includes `HTTP/1.1 200 OK`
- body is `hello`

### 3) Unknown path (`404`)

```sh
curl -k -i https://127.0.0.1:8080/not-found
```

Expected:
- `HTTP/1.1 404 Not Found`

### 4) Method not allowed (`405`)

```sh
curl -k -i -X POST https://127.0.0.1:8080/health
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

## CI/CD (GitHub Actions -> Azure Container Registry)

This repository includes a workflow at `.github/workflows/ci-cd.yml` that:
- runs CI (`go mod download`, `go test ./...`, `go build ./cmd/server`) on pull requests to `main` and on pushes to `main`
- publishes a container image to ACR only on pushes to `main`

### Required GitHub Secrets

Add these repository secrets before using image publish:
- `ACR_LOGIN_SERVER` (example: `myregistry.azurecr.io`)
- `ACR_USERNAME`
- `ACR_PASSWORD`

### Published image tags

On each successful push to `main`, the workflow pushes:
- `myregistry.azurecr.io/light-serve:main`
- `myregistry.azurecr.io/light-serve:sha-<short-commit-sha>`

Replace `myregistry.azurecr.io` with your `ACR_LOGIN_SERVER` value.

Example pull command:

```sh
docker pull myregistry.azurecr.io/light-serve:main
```

## Notes for v1

- This is a custom learning-oriented HTTP server implementation, not production-hardened web infrastructure yet.
- The architecture is intentionally layered and testable to support incremental additions (real use-case slices, persistence adapters, richer routing, auth, static files, etc.).
