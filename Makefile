# Copyright (c) 2026 Itential, Inc
# GNU General Public License v3.0+ (see LICENSE or https://www.gnu.org/licenses/gpl-3.0.txt)
# SPDX-License-Identifier: GPL-3.0-or-later

# Make configuration
SHELL := /bin/bash
.DEFAULT_GOAL := help
.SHELLFLAGS := -eu -o pipefail -c

# Go build configuration
# Note: CGO_ENABLED is set to 0 by default for static builds, but is temporarily
# enabled for race detector tests which require it
export CGO_ENABLED := 0

# Go tools
GO := go
GOFMT := gofmt
GOLINT := golangci-lint

# Build metadata
GIT_SHA := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
LDFLAGS := -X 'github.com/itential/igsdk.Version=$(VERSION)' \
           -X 'github.com/itential/igsdk.Build=$(GIT_SHA)'

# Directories and files
COVER_DIR := cover
COVERAGE_OUT := coverage.out
COVERAGE_TXT := coverage.txt

# Test configuration
PKG := ./...
TEST_CACHE ?= enabled
CACHE_FLAG := $(if $(filter-out enabled,$(TEST_CACHE)),-count=1,)

# Phony targets
.PHONY: help config clean ci test test-race coverage \
        fmt vet lint tidy verify tools

##@ General

help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } \
		/^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

config: ## Display the build configuration
	@echo "Build Configuration:"
	@echo "  VERSION            = $(VERSION)"
	@echo "  GIT_SHA            = $(GIT_SHA)"
	@echo "  CGO_ENABLED        = $(CGO_ENABLED)"
	@echo ""
	@echo "Go Configuration:"
	@echo "  GO                 = $(GO)"
	@echo "  GOFMT              = $(GOFMT)"
	@echo ""
	@echo "Linker Flags:"
	@echo "  LDFLAGS            = $(LDFLAGS)"

##@ Development

fmt: ## Format all Go files
	@echo "Formatting Go files..."
	@$(GOFMT) -w .
	@echo "Formatting complete"

vet: ## Run go vet
	@echo "Running go vet..."
	@$(GO) vet $(PKG)
	@echo "Vet complete"

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running golangci-lint..."
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run $(PKG); \
		echo "Lint complete"; \
	else \
		echo "Error: golangci-lint not found. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
		exit 1; \
	fi

tidy: ## Run go mod tidy
	@echo "Running go mod tidy..."
	@$(GO) mod tidy
	@echo "Tidy complete"

verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	@$(GO) mod verify
	@echo "Verification complete"

tools: ## Install development tools (golangci-lint)
	@echo "Installing development tools..."
	@if ! command -v $(GOLINT) >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin; \
	else \
		echo "golangci-lint already installed"; \
	fi
	@echo "Tools installation complete"

clean: ## Clean build artifacts and coverage files
	@echo "Cleaning build artifacts..."
	@rm -rf $(COVER_DIR) $(COVERAGE_OUT) $(COVERAGE_TXT)
	@$(GO) clean -cache -testcache
	@echo "Clean complete"

##@ Testing

test: ## Run all tests
	@echo "Running tests..."
	@$(GO) test $(CACHE_FLAG) $(PKG)
	@echo "Tests complete"

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@CGO_ENABLED=1 $(GO) test -race $(CACHE_FLAG) $(PKG)
	@echo "Race detector tests complete"

coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@mkdir -p $(COVER_DIR)
	@$(GO) test -cover $(CACHE_FLAG) $(PKG)
	@echo ""
	@echo "Generating coverage profile..."
	@$(GO) test -coverprofile=$(COVERAGE_OUT) $(CACHE_FLAG) $(PKG)
	@$(GO) tool cover -func=$(COVERAGE_OUT)
	@# Generate coverage.txt artifact for CI
	@if [ -f $(COVERAGE_OUT) ]; then \
		total=$$($(GO) tool cover -func=$(COVERAGE_OUT) | grep total | awk '{print $$3}'); \
		echo "Total coverage: $$total" | tee $(COVERAGE_TXT); \
	fi

##@ CI/CD

ci: fmt vet test-race coverage ## Run all CI checks (fmt, vet, test-race, coverage)
	@echo ""
	@echo "✓ All CI checks passed!"
