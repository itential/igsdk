// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Response wraps an HTTP response with convenience methods for common operations.
//
// Response provides methods to:
//   - Parse JSON responses into Go types
//   - Access response body as text
//   - Check response status (success/error)
//   - Access headers and status codes
//
// The Response type is returned by all HTTP methods on Platform and Gateway clients.
type Response struct {
	// Raw is the underlying http.Response from the HTTP request.
	Raw *http.Response

	// Body contains the complete response body as bytes.
	Body []byte
}

// NewResponse creates a new Response from an http.Response and body bytes.
func NewResponse(raw *http.Response, body []byte) (*Response, error) {
	if raw == nil {
		return nil, fmt.Errorf("http response cannot be nil")
	}
	return &Response{Raw: raw, Body: body}, nil
}

// StatusCode returns the HTTP status code of the response.
func (r *Response) StatusCode() int {
	if r == nil || r.Raw == nil {
		return 0
	}
	return r.Raw.StatusCode
}

// Headers returns the HTTP headers of the response.
func (r *Response) Headers() http.Header {
	if r == nil || r.Raw == nil {
		return nil
	}
	return r.Raw.Header
}

// Text returns the response body as a string.
func (r *Response) Text() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}

// JSON unmarshals the response body into the provided value.
//
// The value parameter must be a pointer to the target type. JSON uses the
// standard encoding/json package for unmarshaling.
//
// Example with a struct:
//
//	type User struct {
//		ID   string `json:"_id"`
//		Name string `json:"name"`
//	}
//	var user User
//	if err := resp.JSON(&user); err != nil {
//		return err
//	}
//
// Example with a map:
//
//	var result map[string]any
//	if err := resp.JSON(&result); err != nil {
//		return err
//	}
func (r *Response) JSON(v any) error {
	if r == nil || len(r.Body) == 0 {
		return fmt.Errorf("empty response body")
	}
	if err := json.Unmarshal(r.Body, v); err != nil {
		return fmt.Errorf("unmarshal response body: %w", err)
	}
	return nil
}

// IsSuccess returns true if the response status code is 2xx.
func (r *Response) IsSuccess() bool {
	sc := r.StatusCode()
	return sc >= http.StatusOK && sc < http.StatusMultipleChoices
}

// IsError returns true if the response status code is 4xx or 5xx.
func (r *Response) IsError() bool {
	return r.StatusCode() >= http.StatusBadRequest
}

// CheckStatus returns an error if the response status indicates failure.
//
// Returns nil for 2xx and 3xx status codes.
// Returns HTTPStatusError for 4xx and 5xx status codes.
//
// Note: HTTP methods on Platform and Gateway clients automatically call CheckStatus,
// so you typically don't need to call this manually.
func (r *Response) CheckStatus() error {
	if !r.IsError() {
		return nil
	}
	status := ""
	if r.Raw != nil {
		status = r.Raw.Status
	}
	return &HTTPStatusError{StatusCode: r.StatusCode(), Status: status, Response: r}
}

// String returns a string representation of the response.
func (r *Response) String() string {
	if r == nil || r.Raw == nil || r.Raw.Request == nil || r.Raw.Request.URL == nil {
		return fmt.Sprintf("Response(status_code=%d)", r.StatusCode())
	}
	return fmt.Sprintf("Response(status_code=%d, url='%s')", r.StatusCode(), r.Raw.Request.URL.String())
}
