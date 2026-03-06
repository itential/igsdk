// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

/*
Package igsdk provides a Go SDK for the Itential Automation Platform and Automation Gateway APIs.

# Overview

The igsdk package offers idiomatic Go clients for interacting with Itential's automation
infrastructure. It provides HTTP clients with automatic authentication, structured logging,
sensitive data redaction, and comprehensive error handling.

# Quick Start

Create a Platform client:

	import "github.com/itential/igsdk"

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithBasicAuth("admin", "password"),
		igsdk.WithTLS(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	resp, err := client.Get(context.Background(), "/health", nil)
	if err != nil {
		log.Fatal(err)
	}

	var result map[string]any
	if err := resp.JSON(&result); err != nil {
		log.Fatal(err)
	}

Create a Gateway client:

	gateway, err := igsdk.NewGatewayClient("gateway.example.com",
		igsdk.WithBasicAuth("admin@itential", "password"),
	)
	if err != nil {
		log.Fatal(err)
	}

	devices, err := gateway.Get(context.Background(), "/devices", nil)

# Authentication

The SDK supports two authentication methods:

Basic Authentication (default):

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithBasicAuth("username", "password"),
	)

OAuth 2.0 Client Credentials (Platform only):

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithOAuth("client-id", "client-secret"),
	)

Authentication tokens are automatically managed with optional TTL-based re-authentication:

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithBasicAuth("admin", "password"),
		igsdk.WithTTL(30*time.Minute), // Re-authenticate after 30 minutes
	)

# HTTP Methods

All clients provide standard HTTP methods:

	// GET request
	resp, err := client.Get(ctx, "/adapters", params)

	// POST request with JSON body
	payload := map[string]string{"name": "myAdapter"}
	resp, err := client.Post(ctx, "/adapters", nil, payload)

	// PUT, PATCH, DELETE
	resp, err := client.Put(ctx, "/adapters/id", nil, payload)
	resp, err := client.Patch(ctx, "/adapters/id", nil, payload)
	resp, err := client.Delete(ctx, "/adapters/id", nil)

# Query Parameters

Use url.Values for type-safe query parameters:

	import "net/url"

	params := url.Values{
		"limit":  []string{"10"},
		"offset": []string{"0"},
		"filter": []string{"name=test"},
	}
	resp, err := client.Get(ctx, "/adapters", params)

# Response Handling

The Response type provides convenient methods for handling responses:

	resp, err := client.Get(ctx, "/health", nil)
	if err != nil {
		log.Fatal(err)
	}

	// Check status
	if resp.IsSuccess() {
		fmt.Println("Success!")
	}

	// Parse JSON into a struct
	var health HealthStatus
	if err := resp.JSON(&health); err != nil {
		log.Fatal(err)
	}

	// Access raw body
	fmt.Println(resp.Text())

	// Check status code
	fmt.Println(resp.StatusCode())

# Error Handling

HTTP methods return errors only for transport-level failures (network errors,
serialization failures, authentication failures). HTTP 4xx and 5xx status codes
are returned as responses, not errors — use IsError, CheckStatus, or StatusCode
to inspect them.

	resp, err := client.Get(ctx, "/resource", nil)
	if err != nil {
		// Transport or auth error (network failure, bad credentials, etc.)
		return err
	}
	if resp.IsError() {
		// HTTP 4xx or 5xx — inspect and handle
		fmt.Printf("HTTP error: %d\n", resp.StatusCode())
		var body map[string]any
		_ = resp.JSON(&body)
		return fmt.Errorf("request failed: %d", resp.StatusCode())
	}

To use Go's errors.As for status code handling, call CheckStatus explicitly:

	if err := resp.CheckStatus(); err != nil {
		var httpErr *igsdk.HTTPStatusError
		if errors.As(err, &httpErr) {
			fmt.Printf("HTTP error: %d\n", httpErr.StatusCode)
		}
	}

Authentication errors (wrong credentials, OAuth failure) are always returned
as errors regardless of this behavior:

# Structured Logging

The SDK uses Go's standard log/slog package for structured logging. At Debug
level, every request logs method, URL, headers, body, status code, and round-trip
duration. Sensitive values are redacted before they reach the log output.

	import "log/slog"

	logger := igsdk.NewJSONLogger(
		igsdk.WithLogOutput(os.Stdout),
		igsdk.WithLogLevel(slog.LevelDebug),
	)

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithLogger(logger),
	)

For sensitive data redaction, attach a Scanner to the logger:

	scanner := igsdk.NewScanner()
	logger := igsdk.NewLogger(
		igsdk.WithLogOutput(os.Stdout),
		igsdk.WithSensitiveDataRedaction(scanner),
	)

For request tracing and correlation IDs, use LogContext to attach fields to the
context. The SDK includes these fields in every log entry for that request:

	ctx = igsdk.LogContext(ctx, "request_id", reqID, "tenant", tenantID)
	resp, err := client.Get(ctx, "/adapters", nil)
	// Debug log entries for this request include request_id and tenant.

# Configuration Options

Clients support extensive configuration via functional options:

	client, err := igsdk.NewPlatformClient("api.example.com",
		igsdk.WithPort(8443),
		igsdk.WithTLS(true),
		igsdk.WithVerify(false),                    // Disable cert verification
		igsdk.WithTimeout(60*time.Second),          // Request timeout
		igsdk.WithTTL(30*time.Minute),             // Auth token TTL
		igsdk.WithBasicAuth("admin", "password"),
		igsdk.WithHTTPClient(customClient),         // Custom http.Client
		igsdk.WithLogger(logger),                   // Custom logger
		igsdk.WithScanner(scanner),                 // Custom scanner
	)

# Version Information

Access SDK version and build metadata:

	info := igsdk.GetInfo()
	fmt.Println(info.FullVersion())  // "igsdk v1.0.0 (abc123)"

	if info.IsRelease() {
		fmt.Printf("Version: %s\n", info.Version)
		fmt.Printf("Build: %s\n", info.Build)
	}

# Context Support

All HTTP methods accept a context for cancellation and timeouts:

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := client.Get(ctx, "/slow-endpoint", nil)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Println("Request timed out")
		}
		return err
	}

# Sensitive Data Redaction

The Scanner type automatically redacts sensitive data from logs:

	scanner := igsdk.NewScanner()

	// Built-in patterns detect:
	// - Passwords, API keys, tokens
	// - Secrets, bearer tokens
	// - URLs with credentials
	// - Email addresses in auth contexts

	// Add custom patterns
	scanner.AddPattern("custom_token", `token=[a-zA-Z0-9]+`)

	// Use with logger
	logger := igsdk.NewLogger(
		igsdk.WithSensitiveDataRedaction(scanner),
	)

# Platform vs Gateway

Platform Client:
  - Connects to Itential Automation Platform
  - Supports OAuth 2.0 and Basic Auth
  - Base path: / (root)
  - Default user: "admin"

Gateway Client:
  - Connects to Itential Automation Gateway
  - Supports Basic Auth only
  - Base path: /api/v2.0
  - Default user: "admin@itential"

# Best Practices

1. Always use context for cancellation and timeouts
2. Check response status before parsing JSON
3. Use url.Values for query parameters (type-safe)
4. Enable structured logging in production
5. Use sensitive data redaction for security
6. Set appropriate timeouts for your use case
7. Handle HTTPStatusError explicitly for better error messages
8. Reuse clients - they are safe for concurrent use

# Thread Safety

All client types are safe for concurrent use. You can share a single client
across multiple goroutines:

	client, _ := igsdk.NewPlatformClient("api.example.com")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client.Get(context.Background(), "/health", nil)
		}()
	}
	wg.Wait()
*/
package igsdk
