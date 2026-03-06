// Copyright (c) 2026 Itential, Inc
// GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
// SPDX-License-Identifier: GPL-3.0-or-later

package igsdk

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Scanner detects and redacts sensitive data patterns in text.
//
// Scanner uses regular expressions to identify sensitive information such as:
//   - Passwords and secrets
//   - API keys and access tokens
//   - Bearer tokens
//   - URLs with embedded credentials
//   - Email addresses in authentication contexts
//
// Scanners are safe for concurrent use. Custom patterns can be added with AddPattern.
//
// Example:
//
//	scanner := igsdk.NewScanner()
//	scanner.AddPattern("custom_token", `token=[a-zA-Z0-9]+`)
//	redacted := scanner.ScanAndRedact("password=secret123")
//	// Returns: "[REDACTED_PASSWORD]"
type Scanner struct {
	mu       sync.RWMutex
	patterns map[string]*regexp.Regexp
}

// NewScanner creates a new scanner with built-in sensitive data detection patterns.
//
// The default patterns detect:
//   - API keys (api_key, apikey)
//   - Bearer tokens
//   - Access tokens
//   - Passwords (password, passwd, pwd)
//   - Secrets (secret, client_secret)
//   - URLs with credentials (http://user:pass@host)
//   - Email addresses in auth contexts
//
// Additional patterns can be added with AddPattern.
func NewScanner() *Scanner {
	s := &Scanner{patterns: map[string]*regexp.Regexp{}}
	defaults := map[string]string{
		"api_key":       `(?i)\b(?:api[_-]?key|apikey)\s*[=:]\s*["']?([a-zA-Z0-9_\-]{16,})["']?`,
		"bearer_token":  `(?i)\bbearer\s+([a-zA-Z0-9_\-\.]{20,})`,
		"access_token":  `(?i)\b(?:access[_-]?token|accesstoken)\s*[=:]\s*["']?([a-zA-Z0-9_\-]{20,})["']?`,
		"password":      `(?i)\b(?:password|passwd|pwd)\s*[=:]\s*["']?([^\s"']{6,})["']?`,
		"secret":        `(?i)\b(?:secret|client_secret)\s*[=:]\s*["']?([a-zA-Z0-9_\-]{16,})["']?`,
		"auth_url":      `https?://[a-zA-Z0-9_\-]+:[a-zA-Z0-9_\-]+@[^\s]+`,
		"email_in_auth": `(?i)(?:username|user|email)\s*[=:]\s*["']?([a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,})["']?`,
	}
	for name, pattern := range defaults {
		_ = s.AddPattern(name, pattern)
	}
	return s
}

// AddPattern adds a new pattern to the scanner.
func (s *Scanner) AddPattern(name, pattern string) error {
	r, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern for '%s': %w", name, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns[name] = r
	return nil
}

// RemovePattern removes a pattern from the scanner.
func (s *Scanner) RemovePattern(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.patterns[name]; !ok {
		return false
	}
	delete(s.patterns, name)
	return true
}

// ListPatterns returns all pattern names in sorted order.
func (s *Scanner) ListPatterns() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.patterns))
	for k := range s.patterns {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ScanAndRedact scans text for sensitive data patterns and redacts matches.
func (s *Scanner) ScanAndRedact(text string) string {
	if text == "" {
		return text
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := text
	for name, pattern := range s.patterns {
		replacement := fmt.Sprintf("[REDACTED_%s]", strings.ToUpper(name))
		out = pattern.ReplaceAllString(out, replacement)
	}
	return out
}

// HasSensitiveData checks if text contains any sensitive data patterns.
func (s *Scanner) HasSensitiveData(text string) bool {
	if text == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, pattern := range s.patterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}
