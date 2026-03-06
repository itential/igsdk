// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- Factory / constructor tests ---

func TestNewPlatformClientDefaults(t *testing.T) {
	c, err := NewPlatformClient("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.BaseURL(); got != "https://example.com" {
		t.Fatalf("unexpected base url: %s", got)
	}
}

func TestFactoryEmptyHostDefaults(t *testing.T) {
	p, err := NewPlatformClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.BaseURL() != "https://localhost" {
		t.Fatalf("unexpected platform base url: %s", p.BaseURL())
	}
	g, err := NewGatewayClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.BaseURL() != "https://localhost/api/v2.0" {
		t.Fatalf("unexpected gateway base url: %s", g.BaseURL())
	}
}

func TestNewGatewayClientDefaults(t *testing.T) {
	c, err := NewGatewayClient("example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.BaseURL(); got != "https://example.com/api/v2.0" {
		t.Fatalf("unexpected base url: %s", got)
	}
}

func TestNewGatewayClientRejectsOAuth(t *testing.T) {
	_, err := NewGatewayClient("example.com", WithOAuth("id", "secret"))
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Auth validation at construction time ---

func TestPlatformClientRejectsPartialOAuth(t *testing.T) {
	_, err := NewPlatformClient("x", WithOAuth("id", ""))
	if err == nil {
		t.Fatal("expected error for partial OAuth credentials")
	}
	_, err = NewPlatformClient("x", WithOAuth("", "secret"))
	if err == nil {
		t.Fatal("expected error for partial OAuth credentials")
	}
}

func TestPlatformClientRejectsEmptyBasicAuth(t *testing.T) {
	_, err := NewPlatformClient("x", WithBasicAuth("", ""))
	if err == nil {
		t.Fatal("expected error for empty basic auth credentials")
	}
	_, err = NewPlatformClient("x", WithBasicAuth("user", ""))
	if err == nil {
		t.Fatal("expected error for empty password")
	}
	_, err = NewPlatformClient("x", WithBasicAuth("", "pass"))
	if err == nil {
		t.Fatal("expected error for empty username")
	}
}

func TestGatewayClientRejectsEmptyBasicAuth(t *testing.T) {
	_, err := NewGatewayClient("x", WithBasicAuth("", ""))
	if err == nil {
		t.Fatal("expected error for empty basic auth credentials")
	}
}

// --- makeBaseURL ---

func TestMakeBaseURLPortBehavior(t *testing.T) {
	// TLS default (port 0 → 443, omitted)
	if got := makeBaseURL("localhost", 0, "", true); got != "https://localhost" {
		t.Fatalf("got %s", got)
	}
	// Non-standard port included
	if got := makeBaseURL("localhost", 8080, "", true); got != "https://localhost:8080" {
		t.Fatalf("got %s", got)
	}
	// HTTP default (port 0 → 80, omitted)
	if got := makeBaseURL("localhost", 80, "", false); got != "http://localhost" {
		t.Fatalf("got %s", got)
	}
	// Cross-scheme: TLS + port 80 must include port
	if got := makeBaseURL("localhost", 80, "", true); got != "https://localhost:80" {
		t.Fatalf("got %s", got)
	}
	// Cross-scheme: HTTP + port 443 must include port
	if got := makeBaseURL("localhost", 443, "", false); got != "http://localhost:443" {
		t.Fatalf("got %s", got)
	}
}

func TestMakeBaseURLHTTPDefaultPort(t *testing.T) {
	got := makeBaseURL("localhost", 0, "/api", false)
	if got != "http://localhost/api" {
		t.Fatalf("unexpected url: %s", got)
	}
}

// --- Basic auth flow ---

func TestPlatformBasicAuthAndGet(t *testing.T) {
	var loginCalls int32
	var apiCalls int32
	var authHeader string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			atomic.AddInt32(&loginCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/health/server":
			atomic.AddInt32(&apiCalls, 1)
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)), WithBasicAuth("admin", "admin"))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	res, err := c.Get(context.Background(), "/health/server", nil)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if !res.IsSuccess() {
		t.Fatal("expected success")
	}
	if got := atomic.LoadInt32(&loginCalls); got != 1 {
		t.Fatalf("expected 1 login call, got %d", got)
	}
	if got := atomic.LoadInt32(&apiCalls); got != 1 {
		t.Fatalf("expected 1 api call, got %d", got)
	}
	if authHeader != "" {
		t.Fatalf("did not expect bearer auth header for basic auth, got %q", authHeader)
	}
}

// --- OAuth flow ---

func TestPlatformOAuthAuthAndBearer(t *testing.T) {
	var loginCalls int32
	var apiCalls int32
	var authHeader string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			atomic.AddInt32(&loginCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"abc123token"}`))
		case "/adapters":
			atomic.AddInt32(&apiCalls, 1)
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)), WithOAuth("cid", "csecret"))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/adapters", nil)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got := atomic.LoadInt32(&loginCalls); got != 1 {
		t.Fatalf("expected 1 oauth call, got %d", got)
	}
	if got := atomic.LoadInt32(&apiCalls); got != 1 {
		t.Fatalf("expected 1 api call, got %d", got)
	}
	if authHeader != "Bearer abc123token" {
		t.Fatalf("unexpected authorization header: %q", authHeader)
	}
}

// --- Gateway auth and base path ---

func TestGatewayAuthAndBasePath(t *testing.T) {
	var loginCalls int32
	var apiCalls int32

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/login":
			atomic.AddInt32(&loginCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/v2.0/devices":
			atomic.AddInt32(&apiCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"devices":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c, err := NewGatewayClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/devices", nil)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if got := atomic.LoadInt32(&loginCalls); got != 1 {
		t.Fatalf("expected 1 login call, got %d", got)
	}
	if got := atomic.LoadInt32(&apiCalls); got != 1 {
		t.Fatalf("expected 1 api call, got %d", got)
	}
}

// --- TTL re-authentication ---

func TestTTLForcesReauth(t *testing.T) {
	var loginCalls int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			atomic.AddInt32(&loginCalls, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"access_token":"tok"}`))
		case "/adapters":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	c, err := NewPlatformClient(
		"localhost",
		WithTLS(false),
		WithPort(serverPort(t, ts.URL)),
		WithOAuth("cid", "csecret"),
		WithTTL(1*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if _, err := c.Get(context.Background(), "/adapters", nil); err != nil {
		t.Fatalf("first get error: %v", err)
	}
	time.Sleep(3 * time.Millisecond)
	if _, err := c.Get(context.Background(), "/adapters", nil); err != nil {
		t.Fatalf("second get error: %v", err)
	}
	if got := atomic.LoadInt32(&loginCalls); got < 2 {
		t.Fatalf("expected reauth to trigger, got %d login calls", got)
	}
}

// --- HTTP status errors are responses, not errors ---

func TestHTTPStatusErrorIsResponse(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	resp, err := c.Get(context.Background(), "/broken", nil)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected response, got nil")
	}
	if !resp.IsError() {
		t.Fatalf("expected IsError() true, got false for status %d", resp.StatusCode())
	}
	if resp.StatusCode() != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", resp.StatusCode())
	}
	// Body should be accessible
	if resp.Text() == "" {
		t.Fatal("expected non-empty body")
	}
	// CheckStatus still works for callers who want it
	if err := resp.CheckStatus(); err == nil {
		t.Fatal("expected CheckStatus to return error")
	}
	var hs *HTTPStatusError
	if !errors.As(resp.CheckStatus(), &hs) {
		t.Fatal("expected HTTPStatusError from CheckStatus")
	}
	if hs.StatusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code: %d", hs.StatusCode)
	}
}

// --- Auth failures still return errors (server-side) ---

func TestAuthServerReturnsErrorOnLogin(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/anything", nil)
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestOAuthServerReturnsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client"}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)), WithOAuth("id", "secret"))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/anything", nil)
	if err == nil {
		t.Fatal("expected OAuth error")
	}
	var httpErr *HTTPStatusError
	if !errors.As(err, &httpErr) {
		t.Fatalf("expected HTTPStatusError wrapped in auth error, got %T", err)
	}
	if httpErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", httpErr.StatusCode)
	}
}

// --- Network / infrastructure errors ---

func TestRequestError(t *testing.T) {
	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(1))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/x", nil)
	if err == nil {
		t.Fatal("expected error for refused connection")
	}
}

func TestSendSerializationFailure(t *testing.T) {
	type bad struct {
		C chan int `json:"c"`
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Post(context.Background(), "/x", nil, bad{C: make(chan int)})
	if err == nil {
		t.Fatal("expected serialization error")
	}
}

func TestBuildURLInvalidPath(t *testing.T) {
	c, err := NewPlatformClient("localhost")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "http://[::1", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Options ---

func TestWithVerifyAndTimeoutAndHTTPClientOptions(t *testing.T) {
	custom := &http.Client{Timeout: 123 * time.Millisecond}
	c, err := NewPlatformClient(
		"example.com",
		WithHTTPClient(custom),
		WithVerify(false),
		WithTimeout(2*time.Second),
	)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if c.client.httpClient != custom {
		t.Fatal("expected custom client")
	}
}

func TestWithVerifyFalseBuildsInsecureClient(t *testing.T) {
	c, err := NewPlatformClient("example.com", WithVerify(false))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if c.client.httpClient == nil {
		t.Fatal("expected http client")
	}
}

func TestWithLoggerAndScannerClientOptions(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(WithLogOutput(&buf))
	scanner := NewScanner()

	c, err := NewPlatformClient("example.com",
		WithLogger(logger),
		WithScanner(scanner),
	)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if c.client.config.logger != logger {
		t.Fatal("expected custom logger")
	}
	if c.client.config.scanner != scanner {
		t.Fatal("expected custom scanner")
	}
}

// --- HTTP methods, query params, and headers ---

func TestMethodsAndParamsAndJSONHeaders(t *testing.T) {
	paths := map[string]bool{}
	var query string
	var contentType string
	var acceptHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		paths[r.Method+":"+r.URL.Path] = true
		if r.URL.Path == "/post" {
			contentType = r.Header.Get("Content-Type")
		}
		if r.URL.Path == "/get" {
			acceptHeader = r.Header.Get("Accept")
		}
		query = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	params := url.Values{"a": []string{"1"}}
	_, _ = c.Get(context.Background(), "/get", params)
	_, _ = c.Delete(context.Background(), "/delete", params)
	_, _ = c.Post(context.Background(), "/post", params, map[string]string{"x": "y"})
	_, _ = c.Put(context.Background(), "/put", nil, map[string]string{"x": "y"})
	_, _ = c.Patch(context.Background(), "/patch", nil, map[string]string{"x": "y"})

	for _, key := range []string{"GET:/get", "DELETE:/delete", "POST:/post", "PUT:/put", "PATCH:/patch"} {
		if !paths[key] {
			t.Fatalf("not all methods seen: %#v", paths)
		}
	}
	if query != "" && query != "a=1" {
		t.Fatalf("unexpected query: %s", query)
	}
	if contentType != "application/json" {
		t.Fatalf("expected json content-type for POST, got %q", contentType)
	}
	if acceptHeader != "application/json" {
		t.Fatalf("expected Accept: application/json on GET, got %q", acceptHeader)
	}
}

// --- GatewayClient HTTP methods ---

func TestGatewayClientMethods(t *testing.T) {
	seen := map[string]bool{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2.0/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			seen[r.Method+":"+r.URL.Path] = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer ts.Close()

	c, err := NewGatewayClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	payload := map[string]string{"x": "y"}
	_, _ = c.Post(context.Background(), "/items", nil, payload)
	_, _ = c.Put(context.Background(), "/items/1", nil, payload)
	_, _ = c.Patch(context.Background(), "/items/1", nil, payload)
	_, _ = c.Delete(context.Background(), "/items/1", nil)

	for _, key := range []string{"POST:/api/v2.0/items", "PUT:/api/v2.0/items/1", "PATCH:/api/v2.0/items/1", "DELETE:/api/v2.0/items/1"} {
		if !seen[key] {
			t.Errorf("expected %s to be called", key)
		}
	}
}

func TestGatewayClientBaseURL(t *testing.T) {
	c, err := NewGatewayClient("gw.example.com")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if got := c.BaseURL(); got != "https://gw.example.com/api/v2.0" {
		t.Fatalf("unexpected base url: %s", got)
	}
}

// --- Response helpers ---

func TestResponseJSONAndString(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	raw := &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{}, Request: req}
	res, err := NewResponse(raw, []byte(`{"x":1}`))
	if err != nil {
		t.Fatalf("new response error: %v", err)
	}
	var parsed map[string]any
	if err := res.JSON(&parsed); err != nil {
		t.Fatalf("json error: %v", err)
	}
	if parsed["x"].(float64) != 1 {
		t.Fatal("unexpected json parse")
	}
	if !res.IsSuccess() || res.IsError() {
		t.Fatal("expected success")
	}
	if got := res.String(); got == "" {
		t.Fatal("expected string repr")
	}
}

func TestResponseHeadersAndJSONFailureAndBytesReader(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	raw := &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Header: http.Header{"X-Test": []string{"v"}}, Request: req}
	res, err := NewResponse(raw, []byte(`{bad}`))
	if err != nil {
		t.Fatalf("new response error: %v", err)
	}
	if res.Headers().Get("X-Test") != "v" {
		t.Fatal("expected header passthrough")
	}
	var parsed map[string]any
	if err := res.JSON(&parsed); err == nil {
		t.Fatal("expected json error")
	}
	if bytesToReader(nil).Len() != 0 {
		t.Fatal("expected empty reader")
	}
	if bytesToReader([]byte("abc")).Len() != 3 {
		t.Fatal("expected non-empty reader")
	}
}

func TestResponseNilCases(t *testing.T) {
	_, err := NewResponse(nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	var res *Response
	if res.StatusCode() != 0 || res.Text() != "" {
		t.Fatal("unexpected nil behavior")
	}
	if res.String() == "" {
		t.Fatal("expected string")
	}
}

func TestNewResponseCheckStatus(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	res, _ := NewResponse(&http.Response{StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error", Request: req, Header: http.Header{}}, []byte(`{}`))
	if err := res.CheckStatus(); err == nil {
		t.Fatal("expected status error")
	}
}

func TestResponseHeadersNilRaw(t *testing.T) {
	r := &Response{Raw: nil, Body: []byte("x")}
	if r.Headers() != nil {
		t.Fatal("expected nil headers")
	}
}

func TestResponseTextNilRaw(t *testing.T) {
	r := &Response{Raw: nil, Body: []byte("hello")}
	if r.Text() != "hello" {
		t.Fatalf("expected body text, got %q", r.Text())
	}
}

func TestResponseJSONEmptyBody(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	r, _ := NewResponse(&http.Response{StatusCode: 200, Status: "200 OK", Header: http.Header{}, Request: req}, []byte{})
	var v map[string]any
	if err := r.JSON(&v); err == nil {
		t.Fatal("expected error for empty body")
	}
}

// --- Error types ---

func TestErrorTypes(t *testing.T) {
	if (&HTTPStatusError{StatusCode: 400}).Error() == "" {
		t.Fatal("expected message")
	}
}

func TestHTTPStatusErrorMessageStatus(t *testing.T) {
	e := &HTTPStatusError{Status: "401 Unauthorized"}
	if e.Error() == "" {
		t.Fatal("expected message")
	}
	var nilErr *HTTPStatusError
	if nilErr.Error() == "" {
		t.Fatal("expected message")
	}
}

// --- Logging and scanner ---

func TestLoggerAndScanner(t *testing.T) {
	var buf bytes.Buffer
	scanner := NewScanner()

	logger := NewLogger(
		WithLogOutput(&buf),
		WithLogLevel(slog.LevelInfo),
		WithSensitiveDataRedaction(scanner),
	)

	logger.Info("processing request", "credentials", "password=secretpass123")
	if got := buf.String(); got == "" || !strings.Contains(got, "REDACTED") {
		t.Fatalf("expected redacted output, got %q", got)
	}

	if !scanner.HasSensitiveData("password=foo123") {
		t.Fatal("expected sensitive data detection")
	}
	if scanner.ScanAndRedact("password=foo123") == "password=foo123" {
		t.Fatal("expected redaction")
	}

	if err := scanner.AddPattern("custom", `token=[a-z]+`); err != nil {
		t.Fatalf("add pattern error: %v", err)
	}
	if !scanner.RemovePattern("custom") {
		t.Fatal("expected removed")
	}
	if scanner.RemovePattern("missing") {
		t.Fatal("expected false")
	}
	if len(scanner.ListPatterns()) == 0 {
		t.Fatal("expected default patterns")
	}

	buf.Reset()
	logger.Debug("debug message") // filtered at Info level
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	jsonBuf := bytes.Buffer{}
	jsonLogger := NewJSONLogger(WithLogOutput(&jsonBuf), WithLogLevel(slog.LevelDebug))
	jsonLogger.Info("json test", "key", "value")
	if got := jsonBuf.String(); !strings.Contains(got, `"msg":"json test"`) {
		t.Fatalf("expected JSON output, got %q", got)
	}

	discard := NewDiscardLogger()
	discard.Info("should be discarded")
	discard.Error("should also be discarded")

	srcBuf := bytes.Buffer{}
	srcLogger := NewLogger(WithLogOutput(&srcBuf), WithLogSource(true))
	srcLogger.Info("test with source")
	if got := srcBuf.String(); !strings.Contains(got, "source=") {
		t.Fatalf("expected source location in output, got %q", got)
	}
}

func TestNewLoggerDefaultsToStderr(t *testing.T) {
	// NewLogger with no WithLogOutput should not panic and should not discard.
	// We can't easily capture stderr here, so just verify it doesn't panic.
	l := NewLogger()
	l.Info("default logger test")
}

func TestScannerEmptyAndList(t *testing.T) {
	s := NewScanner()
	if s.ScanAndRedact("") != "" {
		t.Fatal("expected empty")
	}
	if s.HasSensitiveData("") {
		t.Fatal("expected false")
	}
	if len(s.ListPatterns()) == 0 {
		t.Fatal("expected default patterns")
	}
}

func TestScannerAddPatternSuccess(t *testing.T) {
	s := NewScanner()
	if err := s.AddPattern("x", "x+"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s.ListPatterns()) == 0 {
		t.Fatal("expected patterns")
	}
}

func TestScannerAddPatternInvalidRegex(t *testing.T) {
	s := NewScanner()
	if err := s.AddPattern("bad", `[invalid`); err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

// --- LogContext request tracing ---

func TestLogContextPropagatesInRequests(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLogOutput(&buf),
		WithLogLevel(slog.LevelDebug),
	)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost",
		WithTLS(false),
		WithPort(serverPort(t, ts.URL)),
		WithLogger(logger),
	)
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	ctx := LogContext(context.Background(), "request_id", "trace-abc-123", "tenant", "acme")
	_, err = c.Get(ctx, "/health", nil)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "trace-abc-123") {
		t.Fatalf("expected request_id in log output, got:\n%s", output)
	}
	if !strings.Contains(output, "acme") {
		t.Fatalf("expected tenant in log output, got:\n%s", output)
	}
}

func TestLogContextMergesMultipleCalls(t *testing.T) {
	ctx := context.Background()
	ctx = LogContext(ctx, "a", "1")
	ctx = LogContext(ctx, "b", "2")

	attrs := logAttrsFromContext(ctx)
	if len(attrs) != 4 {
		t.Fatalf("expected 4 elements (2 pairs), got %d: %v", len(attrs), attrs)
	}
}

func TestLogContextOddArgs(t *testing.T) {
	ctx := LogContext(context.Background(), "a", "1", "orphan")
	attrs := logAttrsFromContext(ctx)
	if len(attrs) != 2 {
		t.Fatalf("expected 2 elements (1 pair), got %d: %v", len(attrs), attrs)
	}
}

func TestLogContextEmptyContext(t *testing.T) {
	attrs := logAttrsFromContext(context.Background())
	if attrs != nil {
		t.Fatalf("expected nil attrs on plain context, got %v", attrs)
	}
}

// --- URL building ---

func TestBuildURLAbsoluteAndAuthJSONBuildURLError(t *testing.T) {
	c, err := NewPlatformClient("example.com")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	params := url.Values{"a": []string{"1"}}
	u, err := c.client.buildURL("https://other.example.com/path", params)
	if err != nil {
		t.Fatalf("build url error: %v", err)
	}
	if !strings.HasPrefix(u, "https://other.example.com/path") {
		t.Fatalf("unexpected url: %s", u)
	}
	bc := &baseClient{baseURL: "http://[::1"}
	_, err = bc.sendAuthJSON(context.Background(), "/x", map[string]string{"a": "b"})
	if err == nil {
		t.Fatal("expected build url error")
	}
}

func TestBuildURLWithParams(t *testing.T) {
	c, err := NewPlatformClient("example.com")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	params := url.Values{"b": []string{"1"}}
	u, err := c.client.buildURL("/x", params)
	if err != nil {
		t.Fatalf("build url error: %v", err)
	}
	if !strings.Contains(u, "b=1") {
		t.Fatalf("unexpected query in %s", u)
	}
}

func TestBuildURLAbsoluteHTTPS(t *testing.T) {
	c, err := NewPlatformClient("example.com")
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	u, err := c.client.buildURL("https://other.example.com/path", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != "https://other.example.com/path" {
		t.Fatalf("unexpected url: %s", u)
	}
}

// --- Internal state / edge cases ---

func TestNeedsReauthBranches(t *testing.T) {
	bc := &baseClient{config: clientConfig{ttl: 0}}
	if bc.needsReauth(time.Now()) {
		t.Fatal("expected false")
	}
	bc.config.ttl = time.Second
	if bc.needsReauth(time.Now()) {
		t.Fatal("expected false with zero ts")
	}
	bc.authTS = time.Now().Add(-2 * time.Second)
	if !bc.needsReauth(time.Now()) {
		t.Fatal("expected true")
	}
}

func TestUnknownKind(t *testing.T) {
	bc := &baseClient{kind: 999, config: clientConfig{logger: NewDiscardLogger(), scanner: NewScanner()}}
	_, err := bc.ensureAuth(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthJSONSerializationFailure(t *testing.T) {
	bc := &baseClient{}
	_, err := bc.sendAuthJSON(context.Background(), "/x", map[string]any{"x": func() {}})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendNilContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
		}
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	//nolint:staticcheck // intentionally passing nil context to test fallback
	_, err = c.client.send(nil, http.MethodGet, "/x", nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOAuthMissingTokenDoesNotFail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"no_token":true}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()
	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)), WithOAuth("id", "secret"))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	_, err = c.Get(context.Background(), "/adapters", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.client.token != "" {
		t.Fatal("expected empty token")
	}
}

func TestPlatformAuthInvalidOAuthJSONStillSucceeds(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/oauth/token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{bad}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)), WithOAuth("id", "secret"))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}
	if _, err := c.Get(context.Background(), "/x", nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RequestOption: per-request headers ---

func TestWithHeaderSetsArbitraryHeader(t *testing.T) {
	var customHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		customHeader = r.Header.Get("X-Custom-Header")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	_, err = c.Get(context.Background(), "/resource", nil, WithHeader("X-Custom-Header", "myvalue"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if customHeader != "myvalue" {
		t.Fatalf("expected X-Custom-Header: myvalue, got %q", customHeader)
	}
}

func TestWithHeaderOverridesAcceptDefault(t *testing.T) {
	var acceptHeader string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		acceptHeader = r.Header.Get("Accept")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`data`))
	}))
	defer ts.Close()

	c, err := NewPlatformClient("localhost", WithTLS(false), WithPort(serverPort(t, ts.URL)))
	if err != nil {
		t.Fatalf("factory error: %v", err)
	}

	_, err = c.Get(context.Background(), "/export", nil, WithHeader("Accept", "text/csv"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if acceptHeader != "text/csv" {
		t.Fatalf("expected Accept: text/csv, got %q", acceptHeader)
	}
}

// --- Logger options ---

func TestWithReplaceAttr(t *testing.T) {
	var buf bytes.Buffer
	called := false
	logger := NewLogger(
		WithLogOutput(&buf),
		WithReplaceAttr(func(groups []string, a slog.Attr) slog.Attr {
			called = true
			return a
		}),
	)
	logger.Info("test")
	if !called {
		t.Fatal("expected ReplaceAttr to be called")
	}
}

// --- Metadata ---

func TestMetadata(t *testing.T) {
	if Name == "" || Author == "" {
		t.Fatal("metadata missing")
	}
}

func TestGetInfo(t *testing.T) {
	info := GetInfo()
	if info.Name != "igsdk" {
		t.Fatalf("expected Name to be 'igsdk', got %q", info.Name)
	}
	if info.Author != "Itential" {
		t.Fatalf("expected Author to be 'Itential', got %q", info.Author)
	}
}

func TestInfoIsRelease(t *testing.T) {
	tests := []struct {
		name    string
		info    Info
		wantRel bool
	}{
		{"release build", Info{Version: "v1.0.0", Build: "abc123"}, true},
		{"dev build - no version", Info{Version: "", Build: "abc123"}, false},
		{"dev build - no build", Info{Version: "v1.0.0", Build: ""}, false},
		{"dev build - neither", Info{Version: "", Build: ""}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.IsRelease(); got != tt.wantRel {
				t.Errorf("IsRelease() = %v, want %v", got, tt.wantRel)
			}
		})
	}
}

func TestInfoShortVersion(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{"with version", Info{Version: "v1.2.3"}, "v1.2.3"},
		{"without version", Info{Version: ""}, "devel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.ShortVersion(); got != tt.want {
				t.Errorf("ShortVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfoFullVersion(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{"release build", Info{Name: "igsdk", Version: "v1.2.3", Build: "abc123"}, "igsdk v1.2.3 (abc123)"},
		{"dev build", Info{Name: "igsdk", Version: "", Build: ""}, "igsdk devel"},
		{"partial build info", Info{Name: "igsdk", Version: "v1.2.3", Build: ""}, "igsdk v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.info.FullVersion(); got != tt.want {
				t.Errorf("FullVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInfoString(t *testing.T) {
	info := Info{Name: "igsdk", Version: "v1.0.0", Build: "def456"}
	want := "igsdk v1.0.0 (def456)"
	if got := info.String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
	if info.String() != info.FullVersion() {
		t.Error("String() should equal FullVersion()")
	}
}

// --- Helpers ---

func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	return p
}
