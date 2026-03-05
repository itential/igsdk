// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import "fmt"

// HTTPStatusError is returned when an HTTP request results in a 4xx or 5xx status code.
//
// This error type allows callers to inspect the status code, status text, and
// full response body to determine the appropriate action.
//
// Example:
//
//	resp, err := client.Get(ctx, "/resource", nil)
//	if err != nil {
//		var httpErr *igsdk.HTTPStatusError
//		if errors.As(err, &httpErr) {
//			switch httpErr.StatusCode {
//			case 404:
//				fmt.Println("Resource not found")
//			case 401:
//				fmt.Println("Authentication failed")
//			default:
//				fmt.Printf("HTTP error: %d\n", httpErr.StatusCode)
//			}
//		}
//		return err
//	}
type HTTPStatusError struct {
	// StatusCode is the HTTP status code (e.g., 404, 500).
	StatusCode int

	// Status is the HTTP status text (e.g., "404 Not Found").
	Status string

	// Response contains the full HTTP response including body.
	// Useful for extracting error details from the response body.
	Response *Response
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return "http status error"
	}
	if e.Status != "" {
		return fmt.Sprintf("http request failed: %s", e.Status)
	}
	return fmt.Sprintf("http request failed with status code %d", e.StatusCode)
}
