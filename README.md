# ar-go-common

Shared authentication, caching and database helper utilities for Go projects.

This repository contains a small library of authentication-related handlers and utilities used across services: registration, login, email verification, password reset, caching helpers and database cursor utilities.

## Key files

- `authentication.go`: authentication helpers and middleware
- `authorization.go`: authorization utilities
- `cache.go`: cache implementation and helpers
- `cache_responses.go`: response caching helpers
- `cache_test.go`: tests for cache functionality
- `cursor.go`: database cursor helpers
- `database.go`: database connection and utilities
- `email_service.go`: email sending utilities
- `email_verification.go`: email verification flows
- `errors.go`: common error definitions
- `login.go`: login handler and helpers
- `middlewares.go`: HTTP middlewares used by the package
- `password_reset.go`: password reset flow
- `register.go`: registration handler and helpers
- `user.go`: user model and helpers
- `utils.go`: miscellaneous helpers

## Purpose

- Provide reusable, well-scoped authentication flows and helpers for small services.
- Offer a lightweight caching layer and database cursor utilities to reduce duplication.

Getting started

- Clone or add this module to your Go project and import the package functions you need.

Quick start (local)

```pwsh
# build the module (from repository root)
go build ./...

# run tests
go test ./...
```

Usage notes

- This repo is intended as a library, not a standalone server. Import the package into your service and wire the handlers/middlewares into your router.
- Check `go.mod` for required dependency versions.

Contributing

- Open an issue or PR with small, focused changes. Keep changes scoped to utilities and tests.

License

- If you want a license, add a `LICENSE` file to the repository.

---

If you'd like, I can also: update the module path in `go.mod`, add an example `example/main.go`, or add a short test runner. Which would you prefer next?
