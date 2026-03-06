// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type kind int

const (
	kindPlatform kind = iota + 1
	kindGateway
)

// ClientOption is a functional option for configuring Platform and Gateway clients.
// Options can be passed to NewPlatformClient or NewGatewayClient to customize
// the client's behavior, authentication, timeouts, and logging.
type ClientOption func(*clientConfig)

type clientConfig struct {
	host         string
	port         int
	useTLS       bool
	verify       bool
	timeout      time.Duration
	ttl          time.Duration
	user         string
	password     string
	clientID     string
	clientSecret string
	httpClient   *http.Client
	logger       *slog.Logger
	scanner      *Scanner
}

func defaultPlatformConfig(host string) clientConfig {
	if host == "" {
		host = "localhost"
	}
	return clientConfig{
		host:     host,
		port:     0,
		useTLS:   true,
		verify:   true,
		timeout:  30 * time.Second,
		ttl:      0,
		user:     "admin",
		password: "admin",
		logger:   NewDiscardLogger(),
		scanner:  NewScanner(),
	}
}

func defaultGatewayConfig(host string) clientConfig {
	if host == "" {
		host = "localhost"
	}
	return clientConfig{
		host:     host,
		port:     0,
		useTLS:   true,
		verify:   true,
		timeout:  30 * time.Second,
		ttl:      0,
		user:     "admin@itential",
		password: "admin",
		logger:   NewDiscardLogger(),
		scanner:  NewScanner(),
	}
}

// WithPort sets the port for the client.
func WithPort(port int) ClientOption {
	return func(c *clientConfig) { c.port = port }
}

// WithTLS enables or disables TLS for the client.
func WithTLS(enabled bool) ClientOption {
	return func(c *clientConfig) { c.useTLS = enabled }
}

// WithVerify enables or disables TLS certificate verification.
func WithVerify(enabled bool) ClientOption {
	return func(c *clientConfig) { c.verify = enabled }
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *clientConfig) { c.timeout = timeout }
}

// WithTTL sets the authentication token TTL.
func WithTTL(ttl time.Duration) ClientOption {
	return func(c *clientConfig) { c.ttl = ttl }
}

// WithBasicAuth sets basic authentication credentials.
func WithBasicAuth(user, password string) ClientOption {
	return func(c *clientConfig) {
		c.user = user
		c.password = password
	}
}

// WithOAuth sets OAuth client credentials.
func WithOAuth(clientID, clientSecret string) ClientOption {
	return func(c *clientConfig) {
		c.clientID = clientID
		c.clientSecret = clientSecret
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *clientConfig) {
		c.httpClient = client
	}
}

// WithLogger sets a custom structured logger for the client.
// The logger should be created with NewLogger or NewJSONLogger.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *clientConfig) {
		c.logger = logger
	}
}

// WithScanner sets a custom sensitive data scanner for the client.
func WithScanner(scanner *Scanner) ClientOption {
	return func(c *clientConfig) {
		c.scanner = scanner
	}
}

// RequestOption configures a single HTTP request. Options are applied after
// SDK defaults, so they can override headers like Accept or Content-Type.
type RequestOption func(*requestConfig)

type requestConfig struct {
	headers http.Header
}

// WithHeader sets a custom HTTP header on the request, overriding any SDK default
// for that header key. Call multiple times to set multiple headers.
//
// Example — request XML instead of JSON:
//
//	resp, err := client.Get(ctx, "/report", nil, igsdk.WithHeader("Accept", "application/xml"))
func WithHeader(key, value string) RequestOption {
	return func(rc *requestConfig) {
		if rc.headers == nil {
			rc.headers = make(http.Header)
		}
		rc.headers.Set(key, value)
	}
}

// PlatformClient provides HTTP API access to the Itential Automation Platform.
//
// PlatformClient is safe for concurrent use by multiple goroutines.
// All methods accept a context.Context for cancellation and timeout control.
//
// Supported HTTP methods: Get, Post, Put, Patch, Delete
//
// Authentication is handled automatically using either Basic Auth or OAuth 2.0,
// depending on the options provided to NewPlatformClient.
type PlatformClient struct {
	client *baseClient
}

// GatewayClient provides HTTP API access to the Itential Automation Gateway.
//
// GatewayClient is safe for concurrent use by multiple goroutines.
// All methods accept a context.Context for cancellation and timeout control.
//
// The Gateway client automatically prepends /api/v2.0 to all request paths.
// Supported HTTP methods: Get, Post, Put, Patch, Delete
//
// Authentication is handled automatically using Basic Auth.
// OAuth is not supported for Gateway clients.
type GatewayClient struct {
	client *baseClient
}

type baseClient struct {
	kind          kind
	baseURL       string
	basePath      string
	config        clientConfig
	httpClient    *http.Client
	token         string
	authenticated bool
	authTS        time.Time
	authMu        sync.Mutex
}

func newHTTPClient(cfg clientConfig) (*http.Client, error) {
	if cfg.httpClient != nil {
		return cfg.httpClient, nil
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if !cfg.verify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}
	return &http.Client{
		Timeout:   cfg.timeout,
		Transport: transport,
		Jar:       jar,
	}, nil
}

func makeBaseURL(host string, port int, basePath string, useTLS bool) string {
	if port == 0 {
		if useTLS {
			port = 443
		} else {
			port = 80
		}
	}
	netloc := host
	isDefaultPort := (useTLS && port == 443) || (!useTLS && port == 80)
	if !isDefaultPort {
		netloc = net.JoinHostPort(host, strconv.Itoa(port))
	}
	scheme := "http"
	if useTLS {
		scheme = "https"
	}
	u := url.URL{Scheme: scheme, Host: netloc, Path: basePath}
	return u.String()
}

func newBaseClient(k kind, basePath string, cfg clientConfig) (*baseClient, error) {
	hc, err := newHTTPClient(cfg)
	if err != nil {
		return nil, err
	}
	return &baseClient{
		kind:       k,
		baseURL:    makeBaseURL(cfg.host, cfg.port, basePath, cfg.useTLS),
		basePath:   basePath,
		config:     cfg,
		httpClient: hc,
	}, nil
}

func (c *baseClient) userAgent() string {
	return fmt.Sprintf("igsdk/%s", Version)
}

func (c *baseClient) needsReauth(now time.Time) bool {
	if c.config.ttl <= 0 || c.authTS.IsZero() {
		return false
	}
	return now.Sub(c.authTS) >= c.config.ttl
}

// ensureAuth authenticates if necessary and returns the current bearer token.
// The token is returned while the lock is still held (via defer), ensuring
// the caller receives a consistent snapshot without a data race.
func (c *baseClient) ensureAuth(ctx context.Context) (string, error) {
	c.authMu.Lock()
	defer c.authMu.Unlock()

	if c.needsReauth(time.Now()) {
		c.authenticated = false
		c.token = ""
	}

	if c.authenticated {
		return c.token, nil
	}

	var err error
	switch c.kind {
	case kindPlatform:
		err = c.authenticatePlatform(ctx)
	case kindGateway:
		err = c.authenticateGateway(ctx)
	default:
		err = fmt.Errorf("unknown client type")
	}

	if err != nil {
		return "", fmt.Errorf("authentication: %w", err)
	}

	c.authenticated = true
	c.authTS = time.Now()
	return c.token, nil
}

func (c *baseClient) buildURL(path string, params url.Values) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		u, err := url.Parse(path)
		if err != nil {
			return "", fmt.Errorf("parse absolute URL: %w", err)
		}
		if len(params) > 0 {
			q := u.Query()
			for key, values := range params {
				for _, val := range values {
					q.Add(key, val)
				}
			}
			u.RawQuery = q.Encode()
		}
		return u.String(), nil
	}

	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}

	finalPath := path
	if c.basePath != "" {
		finalPath = strings.TrimRight(c.basePath, "/") + "/" + strings.TrimLeft(path, "/")
	}

	ref, err := url.Parse(finalPath)
	if err != nil {
		return "", fmt.Errorf("parse path: %w", err)
	}

	full := base.ResolveReference(ref)
	if len(params) > 0 {
		q := full.Query()
		for key, values := range params {
			for _, val := range values {
				q.Add(key, val)
			}
		}
		full.RawQuery = q.Encode()
	}
	return full.String(), nil
}

// formatHeaders formats HTTP headers as a single redacted string for logging.
func formatHeaders(h http.Header, s *Scanner) string {
	if len(h) == 0 {
		return ""
	}
	var sb strings.Builder
	for k, vals := range h {
		for _, v := range vals {
			if sb.Len() > 0 {
				sb.WriteString("; ")
			}
			sb.WriteString(k)
			sb.WriteString(": ")
			sb.WriteString(s.ScanAndRedact(v))
		}
	}
	return sb.String()
}

func (c *baseClient) send(ctx context.Context, method string, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	token, err := c.ensureAuth(ctx)
	if err != nil {
		return nil, err
	}

	fullURL, err := c.buildURL(path, params)
	if err != nil {
		return nil, fmt.Errorf("build URL: %w", err)
	}

	var bodyBytes []byte
	if payload != nil {
		bodyBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal request payload: %w", err)
		}
	}

	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytesToReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Accept", "application/json")
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply per-request options after SDK defaults so callers can override.
	rc := &requestConfig{}
	for _, opt := range opts {
		opt(rc)
	}
	for k, vals := range rc.headers {
		req.Header[k] = vals
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	logger := c.config.logger
	scanner := c.config.scanner
	traceAttrs := logAttrsFromContext(ctx)

	logger.DebugContext(ctx, "request",
		buildLogArgs(traceAttrs,
			"method", method,
			"url", scanner.ScanAndRedact(fullURL),
			"headers", formatHeaders(req.Header, scanner),
			"body", scanner.ScanAndRedact(string(bodyBytes)),
		)...,
	)

	start := time.Now()
	raw, err := c.httpClient.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		logger.ErrorContext(ctx, "request failed",
			buildLogArgs(traceAttrs,
				"method", method,
				"url", scanner.ScanAndRedact(fullURL),
				"elapsed_ms", elapsed.Milliseconds(),
				"error", err,
			)...,
		)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer raw.Body.Close()

	data, err := io.ReadAll(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	res, err := NewResponse(raw, data)
	if err != nil {
		return nil, fmt.Errorf("create response: %w", err)
	}

	logger.DebugContext(ctx, "response",
		buildLogArgs(traceAttrs,
			"method", method,
			"url", scanner.ScanAndRedact(fullURL),
			"status_code", raw.StatusCode,
			"elapsed_ms", elapsed.Milliseconds(),
			"headers", formatHeaders(raw.Header, scanner),
			"body", scanner.ScanAndRedact(string(data)),
		)...,
	)

	return res, nil
}

// buildLogArgs merges trace attributes with additional key-value pairs into a
// single []any slice suitable for slog variadic log calls.
func buildLogArgs(traceAttrs []any, keyvals ...any) []any {
	if len(traceAttrs) == 0 {
		return keyvals
	}
	merged := make([]any, 0, len(traceAttrs)+len(keyvals))
	merged = append(merged, traceAttrs...)
	merged = append(merged, keyvals...)
	return merged
}

func (c *baseClient) authenticatePlatform(ctx context.Context) error {
	if c.config.clientID != "" {
		// OAuth 2.0 Client Credentials
		form := url.Values{}
		form.Set("grant_type", "client_credentials")
		form.Set("client_id", c.config.clientID)
		form.Set("client_secret", c.config.clientSecret)

		authURL, _ := c.buildURL("/oauth/token", nil)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, authURL, strings.NewReader(form.Encode()))
		if err != nil {
			return fmt.Errorf("create OAuth request: %w", err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", c.userAgent())

		raw, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("execute OAuth request: %w", err)
		}
		defer raw.Body.Close()

		body, err := io.ReadAll(raw.Body)
		if err != nil {
			return fmt.Errorf("read OAuth response: %w", err)
		}

		res, err := NewResponse(raw, body)
		if err != nil {
			return fmt.Errorf("create OAuth response: %w", err)
		}

		if err := res.CheckStatus(); err != nil {
			return err
		}

		var tokenResp map[string]any
		if err := res.JSON(&tokenResp); err == nil {
			if tok, ok := tokenResp["access_token"].(string); ok {
				c.token = tok
			}
		}

		return nil
	}

	// Basic auth
	data := map[string]map[string]string{"user": {"username": c.config.user, "password": c.config.password}}
	_, err := c.sendAuthJSON(ctx, "/login", data)
	return err
}

func (c *baseClient) authenticateGateway(ctx context.Context) error {
	data := map[string]string{"username": c.config.user, "password": c.config.password}
	_, err := c.sendAuthJSON(ctx, "/login", data)
	return err
}

func (c *baseClient) sendAuthJSON(ctx context.Context, path string, payload any) (*Response, error) {
	fullURL, err := c.buildURL(path, nil)
	if err != nil {
		return nil, fmt.Errorf("build auth URL: %w", err)
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal auth payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytesToReader(body))
	if err != nil {
		return nil, fmt.Errorf("create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent())

	raw, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute auth request: %w", err)
	}
	defer raw.Body.Close()

	data, err := io.ReadAll(raw.Body)
	if err != nil {
		return nil, fmt.Errorf("read auth response: %w", err)
	}

	res, err := NewResponse(raw, data)
	if err != nil {
		return nil, fmt.Errorf("create auth response: %w", err)
	}

	if err := res.CheckStatus(); err != nil {
		return nil, err
	}

	return res, nil
}

// bytesToReader converts a byte slice to a bytes.Reader for use as an io.Reader.
func bytesToReader(b []byte) *bytes.Reader {
	return bytes.NewReader(b)
}

// Get performs an HTTP GET request to the specified path.
//
// The path parameter specifies the API endpoint (e.g., "/health/server").
// Optional query parameters can be provided via params (use nil for no parameters).
//
// The request automatically includes authentication headers and will re-authenticate
// if the token has expired (when TTL is configured).
//
// Example:
//
//	params := url.Values{"limit": []string{"10"}}
//	resp, err := client.Get(ctx, "/adapters", params)
func (c *PlatformClient) Get(ctx context.Context, path string, params url.Values, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodGet, path, params, nil, opts...)
}

// Delete performs an HTTP DELETE request to the specified path.
//
// The path parameter specifies the API endpoint (e.g., "/adapters/123").
// Optional query parameters can be provided via params (use nil for no parameters).
//
// Example:
//
//	resp, err := client.Delete(ctx, "/adapters/123", nil)
func (c *PlatformClient) Delete(ctx context.Context, path string, params url.Values, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodDelete, path, params, nil, opts...)
}

// Post performs an HTTP POST request to the specified path with a JSON payload.
//
// The payload is automatically marshaled to JSON and sent with appropriate
// Content-Type and Accept headers.
//
// Example:
//
//	payload := map[string]any{"name": "MyAdapter", "type": "HTTP"}
//	resp, err := client.Post(ctx, "/adapters", nil, payload)
func (c *PlatformClient) Post(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPost, path, params, payload, opts...)
}

// Put performs an HTTP PUT request to the specified path with a JSON payload.
//
// The payload is automatically marshaled to JSON and sent with appropriate headers.
// PUT typically replaces the entire resource at the specified path.
//
// Example:
//
//	updates := map[string]any{"name": "UpdatedName", "enabled": true}
//	resp, err := client.Put(ctx, "/adapters/123", nil, updates)
func (c *PlatformClient) Put(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPut, path, params, payload, opts...)
}

// Patch performs an HTTP PATCH request to the specified path with a JSON payload.
//
// The payload is automatically marshaled to JSON and sent with appropriate headers.
// PATCH typically applies partial modifications to a resource.
//
// Example:
//
//	changes := map[string]any{"enabled": false}
//	resp, err := client.Patch(ctx, "/adapters/123", nil, changes)
func (c *PlatformClient) Patch(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPatch, path, params, payload, opts...)
}

// BaseURL returns the base URL used by the client.
//
// The base URL includes the scheme (http/https), host, and port,
// but does not include any path components.
//
// Example return value: "https://api.example.com:8443"
func (c *PlatformClient) BaseURL() string {
	return c.client.baseURL
}

// Get performs an HTTP GET request to the specified path.
//
// The path is automatically prefixed with /api/v2.0 for Gateway requests.
// Optional query parameters can be provided via params (use nil for no parameters).
//
// Example:
//
//	resp, err := client.Get(ctx, "/devices", nil)
//	// Actual request: GET /api/v2.0/devices
func (c *GatewayClient) Get(ctx context.Context, path string, params url.Values, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodGet, path, params, nil, opts...)
}

// Delete performs an HTTP DELETE request to the specified path.
//
// The path is automatically prefixed with /api/v2.0 for Gateway requests.
//
// Example:
//
//	resp, err := client.Delete(ctx, "/devices/dev123", nil)
func (c *GatewayClient) Delete(ctx context.Context, path string, params url.Values, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodDelete, path, params, nil, opts...)
}

// Post performs an HTTP POST request to the specified path with a JSON payload.
//
// The path is automatically prefixed with /api/v2.0 for Gateway requests.
// The payload is automatically marshaled to JSON.
//
// Example:
//
//	device := map[string]any{"name": "Router1", "ip": "10.0.0.1"}
//	resp, err := client.Post(ctx, "/devices", nil, device)
func (c *GatewayClient) Post(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPost, path, params, payload, opts...)
}

// Put performs an HTTP PUT request to the specified path with a JSON payload.
//
// The path is automatically prefixed with /api/v2.0 for Gateway requests.
func (c *GatewayClient) Put(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPut, path, params, payload, opts...)
}

// Patch performs an HTTP PATCH request to the specified path with a JSON payload.
//
// The path is automatically prefixed with /api/v2.0 for Gateway requests.
func (c *GatewayClient) Patch(ctx context.Context, path string, params url.Values, payload any, opts ...RequestOption) (*Response, error) {
	return c.client.send(ctx, http.MethodPatch, path, params, payload, opts...)
}

// BaseURL returns the base URL used by the client, including the /api/v2.0 path.
//
// Example return value: "https://gateway.example.com/api/v2.0"
func (c *GatewayClient) BaseURL() string {
	return c.client.baseURL
}
