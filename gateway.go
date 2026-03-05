// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import "fmt"

// NewGatewayClient creates a new client for the Itential Automation Gateway.
//
// The host parameter specifies the Gateway hostname (without protocol or path).
// The Gateway client automatically uses the /api/v2.0 base path.
//
// Default configuration:
//   - Port: 443 (HTTPS) or 80 (HTTP)
//   - TLS: Enabled
//   - Certificate verification: Enabled
//   - Timeout: 30 seconds
//   - Base path: /api/v2.0
//   - Authentication: Basic auth (username: "admin@itential", password: "admin")
//
// Returns an error if OAuth credentials are provided (not supported for Gateway),
// or if the username or password is empty.
//
// Example:
//
//	client, err := igsdk.NewGatewayClient("gateway.example.com",
//		igsdk.WithBasicAuth("admin@itential", "password"),
//	)
func NewGatewayClient(host string, opts ...ClientOption) (*GatewayClient, error) {
	cfg := defaultGatewayConfig(host)
	for _, opt := range opts {
		opt(&cfg)
	}
	if err := validateGatewayConfig(cfg); err != nil {
		return nil, err
	}
	base, err := newBaseClient(kindGateway, "/api/v2.0", cfg)
	if err != nil {
		return nil, fmt.Errorf("create gateway client: %w", err)
	}
	return &GatewayClient{client: base}, nil
}

func validateGatewayConfig(cfg clientConfig) error {
	if cfg.clientID != "" || cfg.clientSecret != "" {
		return fmt.Errorf("OAuth is not supported for Gateway authentication")
	}
	if cfg.user == "" || cfg.password == "" {
		return fmt.Errorf("Gateway authentication requires both username and password")
	}
	return nil
}
