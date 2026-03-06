# Repository Guidelines

## Project Structure & Module Organization
This repository is a Go module: `github.com/itential/igsdk`.

- Root package files contain SDK functionality:
  - `client.go`: core HTTP client, auth flow, TTL re-auth, request methods
  - `platform.go`, `gateway.go`: client constructors
  - `errors.go`, `http_types.go`, `jsonutils.go`: shared types/utilities
  - `logging.go`, `heuristics.go`: logging and sensitive-data redaction
  - `metadata.go`, `init.go`: package metadata and initialization
- Tests live in `client_test.go` (table-driven and behavior-focused tests).
- Module metadata is in `go.mod`.

Keep new code in the root package unless a clear subpackage boundary emerges.

## Build, Test, and Development Commands
Use standard Go tooling:

- `go test ./...` runs all tests.
- `go test ./... -cover` reports package coverage.
- `go test ./... -coverprofile=cover.out && go tool cover -func=cover.out` shows per-function coverage.
- `gofmt -w *.go` formats all Go files in this repository.

Run formatting and tests before opening a PR.

## Coding Style & Naming Conventions
- Follow idiomatic Go and `gofmt` output (tabs, standard formatting).
- Exported APIs use `PascalCase` (`NewPlatformClient`); internal helpers use `camelCase` (`makeBaseURL`).
- Keep constructors `New...`-prefixed.
- Prefer explicit error wrapping/types over string-only errors.
- Keep functions focused and small; use comments only where behavior is non-obvious.

## Testing Guidelines
- Use Go’s built-in `testing` package.
- Name tests as `TestXxxBehavior` (e.g., `TestNewGatewayClientDefaults`).
- Cover success paths, edge cases, and failure branches (auth errors, HTTP errors, serialization failures).
- Coverage target: strive for very high coverage; avoid untested exported behavior.

## Commit & Pull Request Guidelines
- Write commit messages in imperative mood: `rename factory constructors`, `add gateway auth tests`.
- Keep commits focused (one logical change per commit).
- PRs should include:
  - What changed and why
  - Any API changes or breaking behavior
  - Test evidence (`go test ./...` output summary)
  - Linked issue/ticket when applicable

## Security & Configuration Notes
- Never commit credentials, tokens, or real endpoint secrets.
- Preserve sensitive-data redaction behavior in logging paths.
- Prefer `context.Context`-aware APIs for cancellation and timeout control.
