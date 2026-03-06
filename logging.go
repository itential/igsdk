// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import (
	"context"
	"io"
	"log/slog"
	"os"
)

// LoggerOption configures a logger instance.
type LoggerOption func(*loggerConfig)

type loggerConfig struct {
	level           slog.Level
	output          io.Writer
	scanner         *Scanner
	redactSensitive bool
	addSource       bool
	replaceAttr     func(groups []string, a slog.Attr) slog.Attr
}

// ctxKey is an unexported type for context keys in this package.
// Using a package-local type avoids key collisions with other packages.
type ctxKey int

const logCtxKey ctxKey = 0

// WithLogLevel sets the minimum log level.
func WithLogLevel(level slog.Level) LoggerOption {
	return func(c *loggerConfig) {
		c.level = level
	}
}

// WithLogOutput sets the log output writer.
func WithLogOutput(w io.Writer) LoggerOption {
	return func(c *loggerConfig) {
		c.output = w
	}
}

// WithSensitiveDataRedaction enables redaction of sensitive data in log messages.
func WithSensitiveDataRedaction(scanner *Scanner) LoggerOption {
	return func(c *loggerConfig) {
		c.scanner = scanner
		c.redactSensitive = true
	}
}

// WithLogSource enables source code location in log messages.
func WithLogSource(enabled bool) LoggerOption {
	return func(c *loggerConfig) {
		c.addSource = enabled
	}
}

// WithReplaceAttr sets a custom attribute replacer for the logger.
func WithReplaceAttr(fn func(groups []string, a slog.Attr) slog.Attr) LoggerOption {
	return func(c *loggerConfig) {
		c.replaceAttr = fn
	}
}

// NewLogger creates a new structured logger with text output.
// Defaults to writing to os.Stderr at Info level. Use WithLogOutput to redirect output.
// For JSON output use NewJSONLogger. For no output use NewDiscardLogger.
func NewLogger(opts ...LoggerOption) *slog.Logger {
	cfg := &loggerConfig{
		level:  slog.LevelInfo,
		output: os.Stderr,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       cfg.level,
		AddSource:   cfg.addSource,
		ReplaceAttr: buildReplaceAttr(cfg),
	}

	return slog.New(slog.NewTextHandler(cfg.output, handlerOpts))
}

// NewJSONLogger creates a new structured logger with JSON output.
// Defaults to writing to os.Stderr at Info level. Use WithLogOutput to redirect output.
func NewJSONLogger(opts ...LoggerOption) *slog.Logger {
	cfg := &loggerConfig{
		level:  slog.LevelInfo,
		output: os.Stderr,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	handlerOpts := &slog.HandlerOptions{
		Level:       cfg.level,
		AddSource:   cfg.addSource,
		ReplaceAttr: buildReplaceAttr(cfg),
	}

	return slog.New(slog.NewJSONHandler(cfg.output, handlerOpts))
}

// buildReplaceAttr creates a ReplaceAttr function that handles sensitive data redaction
// and chains with any user-provided ReplaceAttr.
func buildReplaceAttr(cfg *loggerConfig) func(groups []string, a slog.Attr) slog.Attr {
	return func(groups []string, a slog.Attr) slog.Attr {
		if cfg.redactSensitive && cfg.scanner != nil && a.Value.Kind() == slog.KindString {
			original := a.Value.String()
			if cfg.scanner.HasSensitiveData(original) {
				a.Value = slog.StringValue(cfg.scanner.ScanAndRedact(original))
			}
		}
		if cfg.replaceAttr != nil {
			a = cfg.replaceAttr(groups, a)
		}
		return a
	}
}

// NewDiscardLogger creates a logger that discards all output.
// Useful for testing or when logging should be explicitly disabled.
func NewDiscardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// LogContext attaches key-value pairs to the context for request tracing.
// The SDK automatically includes these values in all log entries generated
// during requests made with this context, enabling correlation across calls.
//
// Keys must be strings; non-string keys are silently skipped.
// If an odd number of arguments is provided the last one is dropped.
//
// Example:
//
//	ctx = igsdk.LogContext(ctx, "request_id", reqID, "tenant", tenantID)
//	resp, err := client.Get(ctx, "/resources", nil)
//	// SDK debug logs for this request will include request_id and tenant fields.
func LogContext(ctx context.Context, keyvals ...any) context.Context {
	if len(keyvals)%2 != 0 {
		keyvals = keyvals[:len(keyvals)-1]
	}
	existing, _ := ctx.Value(logCtxKey).([]any)
	merged := make([]any, len(existing)+len(keyvals))
	copy(merged, existing)
	copy(merged[len(existing):], keyvals)
	return context.WithValue(ctx, logCtxKey, merged)
}

// logAttrsFromContext returns the key-value pairs stored by LogContext, or nil.
func logAttrsFromContext(ctx context.Context) []any {
	attrs, _ := ctx.Value(logCtxKey).([]any)
	return attrs
}
