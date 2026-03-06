// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/itential/igsdk"
)

func main() {
	// Build a JSON logger that writes to stdout at Debug level so every
	// request and response is visible. A Scanner is attached so passwords,
	// tokens, and other sensitive values are redacted before they reach the log.
	scanner := igsdk.NewScanner()
	logger := igsdk.NewJSONLogger(
		igsdk.WithLogOutput(os.Stdout),
		igsdk.WithLogLevel(slog.LevelDebug),
		igsdk.WithSensitiveDataRedaction(scanner),
	)

	client, err := igsdk.NewPlatformClient("platform.itential.dev",
		igsdk.WithBasicAuth("admin@pronghorn", "admin"),
		igsdk.WithLogger(logger),
		igsdk.WithScanner(scanner),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}

	// Attach a correlation ID to the context so it appears in every log
	// entry for this request.
	ctx := igsdk.LogContext(context.Background(), "request_id", "example-001")

	resp, err := client.Get(ctx, "/whoami", nil)
	if err != nil {
		// err covers transport failures and authentication errors only.
		log.Fatalf("request failed: %v", err)
	}

	// HTTP 4xx/5xx are returned as responses, not errors.
	if resp.IsError() {
		log.Fatalf("HTTP %d: %s", resp.StatusCode(), resp.Text())
	}

	var data map[string]any
	if err := resp.JSON(&data); err != nil {
		log.Fatalf("failed to parse response: %v", err)
	}

	output, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		log.Fatalf("failed to format output: %v", err)
	}

	fmt.Println(string(output))
}
