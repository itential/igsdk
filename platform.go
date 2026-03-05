// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import "fmt"

// NewPlatformClient creates a new client for the Itential Automation Platform.
//
// The host parameter specifies the Platform hostname (without protocol or path).
// Additional configuration can be provided via functional options.
//
// Default configuration:
//   - Port: 443 (HTTPS) or 80 (HTTP)
//   - TLS: Enabled
//   - Certificate verification: Enabled
//   - Timeout: 30 seconds
//   - Authentication: Basic auth (username: "admin", password: "admin")
//
// Returns an error if the provided credentials are incomplete:
//   - OAuth: both client ID and client secret must be set
//   - Basic auth: both username and password must be set
//
// Example:
//
//	client, err := igsdk.NewPlatformClient("api.example.com",
//		igsdk.WithBasicAuth("admin", "password"),
//		igsdk.WithTimeout(60*time.Second),
//	)
func NewPlatformClient(host string, opts ...ClientOption) (*PlatformClient, error) {
	cfg := defaultPlatformConfig(host)
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := validatePlatformConfig(cfg); err != nil {
		return nil, err
	}
	base, err := newBaseClient(kindPlatform, "", cfg)
	if err != nil {
		return nil, fmt.Errorf("create platform client: %w", err)
	}
	return &PlatformClient{client: base}, nil
}

func validatePlatformConfig(cfg clientConfig) error {
	switch {
	case cfg.clientID != "" && cfg.clientSecret != "":
		return nil // valid OAuth
	case cfg.clientID != "" || cfg.clientSecret != "":
		return fmt.Errorf("OAuth authentication requires both client ID and client secret")
	case cfg.user == "" || cfg.password == "":
		return fmt.Errorf("basic authentication requires both username and password")
	}
	return nil
}
