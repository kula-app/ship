# Ship

> CLI for Shipable

Ship is the command-line interface for [Shipable](https://shipable.app), providing authentication, app management, and publishing workflows from the terminal.

## Installation

### Download Pre-built Binary (Recommended)

```bash
# macOS (Apple Silicon)
curl -L -o ship \
  https://github.com/kula-app/ship/releases/latest/download/ship-darwin-arm64
chmod +x ship
sudo mv ship /usr/local/bin/
```

### Build from Source

```bash
make build
# Binary will be at ./dist/ship
```

## Quick Start

```bash
# Authenticate with Shipable
ship auth login

# List your apps
ship apps list

# Publish an app
ship publish --app-id <uuid>
```

## Commands

### Authentication

| Command            | Description               |
| ------------------ | ------------------------- |
| `ship auth login`  | Authenticate via browser  |
| `ship auth logout` | Remove stored credentials |

### Apps

| Command          | Description   |
| ---------------- | ------------- |
| `ship apps list` | List all apps |

### Publishing

| Command                                  | Description              |
| ---------------------------------------- | ------------------------ |
| `ship publish --app-id <id>`             | Full publish             |
| `ship publish metadata --app-id <id>`    | Publish metadata only    |
| `ship publish screenshots --app-id <id>` | Publish screenshots only |
| `ship publish app --app-id <id>`         | Publish app binary only  |
| `ship publish status --app-id <id>`      | Show publish job status  |
| `ship publish validate --app-id <id>`    | Pre-publish validation   |

All publish commands accept `--platform ios,android` to target specific platforms.

### Raw API Access

| Command               | Description                       |
| --------------------- | --------------------------------- |
| `ship api <endpoint>` | Make an authenticated API request |

The endpoint is relative to `/api/` — the prefix is added automatically. Works like `gh api`:

```bash
ship api apps/                                        # GET /api/apps/
ship api apps/ -F limit=10                            # field flags become query params on GET
ship api app/<id>/publish -X POST -F platforms[]=ios  # and a JSON body on other methods
ship api app/<id>/publish -X POST -d '{"platforms":["ios"]}'
ship api app/<id>/publish -X POST -i body.json        # body from a file, or "-" for stdin
```

Use `-F/--field` for values parsed as JSON (`key=value`, `key[sub]=value`, `key[]=value`, `key[]`),
`-f/--raw-field` to send them as plain strings, `-H/--header` to add request headers, `--verbose`
for a curl-style request/response transcript, `--silent` to drop the response body, and
`-n/--dry-run` to print the resolved request without sending it.

Use `--log-format json` on any command for machine-readable JSON output.

## Development

### Prerequisites

- Go 1.26 or later
- Make

### Getting Started

```bash
# Install dependencies
make init

# Set up environment
cp .env.example .env

# Build and run
make build
./dist/ship --help
```

### Makefile Commands

Run `make help` for the full list. Key commands:

| Command         | Description                         |
| --------------- | ----------------------------------- |
| `make build`    | Build CLI binary                    |
| `make run`      | Build and run CLI                   |
| `make test`     | Run tests                           |
| `make analyze`  | Static analysis and security checks |
| `make format`   | Format code                         |
| `make generate` | Generate ent ORM code               |

## Acknowledgments

This project's structure and tooling is heavily inspired by [sentry-cli](https://github.com/getsentry/sentry-cli) :heart:
