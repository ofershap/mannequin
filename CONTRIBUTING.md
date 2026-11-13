# Contributing

Issues and pull requests are welcome.

## Setup

Go 1.26 or newer. No other dependencies.

## Checks

Run everything CI runs before opening a PR:

```bash
go vet ./...
gofmt -l .
go test -race ./...
golangci-lint run ./...
cd compat && go test ./...
```

The `compat/` module pins the SDK versions mannequin is tested against.
If you change the wire format, extend the compat tests, not just the unit tests.

## Rules

- Keep the public API small. A new entry point needs a real project that needs it.
- Claims in the README must match what the tests demonstrate.
- Cassettes and examples must never contain real credentials, prompts, or user data.
