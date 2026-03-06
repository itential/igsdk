# igsdk

[![Go Reference](https://pkg.go.dev/badge/github.com/itential/igsdk.svg)](https://pkg.go.dev/github.com/itential/igsdk)
[![Build Status](https://github.com/itential/igsdk/actions/workflows/ci.yml/badge.svg)](https://github.com/itential/igsdk/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/License-GPL--3.0-blue.svg)](LICENSE)
[![GitHub release](https://img.shields.io/github/v/release/itential/igsdk)](https://github.com/itential/igsdk/releases)

A Go SDK for the Itential Automation Platform and Itential Automation Gateway. It provides
idiomatic HTTP clients with automatic authentication, TTL-based re-authentication, structured
logging with sensitive-data redaction, and context-aware request tracing.

## Features

- **Dual-client support** - Separate `PlatformClient` and `GatewayClient` for the Itential Automation Platform and Gateway
- **Automatic authentication** - Basic Auth and OAuth 2.0 (Client Credentials) handled transparently on every request
- **TTL-based re-authentication** - Configurable token lifetimes with automatic re-auth before expiry
- **Structured logging** - Text and JSON loggers built on `log/slog` with optional sensitive-data redaction
- **Context-aware tracing** - Attach trace IDs and correlation fields to a `context.Context`; they appear in all SDK log entries for that request
- **Concurrency-safe** - Both clients are safe for use by multiple goroutines

## Requirements

- Go 1.24+

## Installation

```bash
go get github.com/itential/igsdk
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/itential/igsdk"
)

func main() {
    client, err := igsdk.NewPlatformClient("platform.example.com",
        igsdk.WithBasicAuth("admin", "password"),
    )
    if err != nil {
        log.Fatal(err)
    }

    resp, err := client.Get(context.Background(), "/health/server", nil)
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(resp.StatusCode())
}
```

## Usage

### Platform Client

```go
client, err := igsdk.NewPlatformClient("platform.example.com",
    igsdk.WithBasicAuth("admin", "password"),
    igsdk.WithTLS(true),
    igsdk.WithVerify(true),
    igsdk.WithTimeout(30 * time.Second),
    igsdk.WithTTL(15 * time.Minute),
)
```

### Gateway Client

```go
client, err := igsdk.NewGatewayClient("gateway.example.com",
    igsdk.WithBasicAuth("admin@itential", "password"),
)
// Requests are automatically prefixed with /api/v2.0
resp, err := client.Get(ctx, "/devices", nil)
```

### OAuth Authentication (Platform only)

```go
client, err := igsdk.NewPlatformClient("platform.example.com",
    igsdk.WithOAuth("my-client-id", "my-client-secret"),
)
```

### Making Requests

All HTTP methods accept a `context.Context`, a path, optional query parameters, and optional
per-request options:

```go
// GET with query parameters
params := url.Values{"limit": []string{"10"}, "skip": []string{"0"}}
resp, err := client.Get(ctx, "/adapters", params)

// POST with a JSON payload
payload := map[string]any{"name": "MyAdapter", "type": "HTTP"}
resp, err := client.Post(ctx, "/adapters", nil, payload)

// PUT / PATCH / DELETE
resp, err := client.Put(ctx, "/adapters/123", nil, updates)
resp, err := client.Patch(ctx, "/adapters/123", nil, changes)
resp, err := client.Delete(ctx, "/adapters/123", nil)
```

### Handling Responses

```go
resp, err := client.Get(ctx, "/users/me", nil)
if err != nil {
    return err
}

// Parse JSON
var user map[string]any
if err := resp.JSON(&user); err != nil {
    return err
}

// Raw text
body := resp.Text()

// Status inspection
fmt.Println(resp.StatusCode())
fmt.Println(resp.IsSuccess())
fmt.Println(resp.IsError())
```

### Error Handling

HTTP 4xx/5xx responses are returned as `*igsdk.HTTPStatusError`:

```go
resp, err := client.Get(ctx, "/resource/missing", nil)
if err != nil {
    var httpErr *igsdk.HTTPStatusError
    if errors.As(err, &httpErr) {
        switch httpErr.StatusCode {
        case 404:
            fmt.Println("resource not found")
        case 401:
            fmt.Println("authentication failed")
        default:
            fmt.Printf("HTTP error: %s\n", httpErr.Status)
        }
    }
    return err
}
```

### Logging

```go
// Text logger (stderr, Info level by default)
logger := igsdk.NewLogger(
    igsdk.WithLogLevel(slog.LevelDebug),
    igsdk.WithLogOutput(os.Stdout),
)

// JSON logger
logger := igsdk.NewJSONLogger(igsdk.WithLogLevel(slog.LevelDebug))

client, err := igsdk.NewPlatformClient("platform.example.com",
    igsdk.WithLogger(logger),
)
```

### Sensitive Data Redaction

```go
scanner := igsdk.NewScanner()

logger := igsdk.NewLogger(igsdk.WithSensitiveDataRedaction(scanner))

client, err := igsdk.NewPlatformClient("platform.example.com",
    igsdk.WithLogger(logger),
    igsdk.WithScanner(scanner),
)
```

### Request Tracing

Attach correlation fields to a context; the SDK includes them in every log entry
for requests made with that context:

```go
ctx = igsdk.LogContext(ctx, "request_id", reqID, "tenant", tenantID)
resp, err := client.Get(ctx, "/resources", nil)
```

### Custom Request Headers

```go
resp, err := client.Get(ctx, "/report", nil,
    igsdk.WithHeader("Accept", "application/xml"),
)
```

## Documentation

- [API Reference](https://pkg.go.dev/github.com/itential/igsdk) - Full GoDoc reference
- [Contributing Guide](CONTRIBUTING.md) - How to contribute to this project
- [Changelog](CHANGELOG.md) - Version history and release notes

## Contributing

Contributions are welcome! Please read our [Contributing Guide](CONTRIBUTING.md) to get started.

Before contributing, you'll need to sign our [Contributor License Agreement](CLA.md).

## Support

- **Bug Reports**: [Open an issue](https://github.com/itential/igsdk/issues/new)
- **Questions**: [Start a discussion](https://github.com/itential/igsdk/discussions)
- **Maintainer**: [@privateip](https://github.com/privateip)

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️  by the <a href="https://github.com/itential">Itential</a> community
</p>
