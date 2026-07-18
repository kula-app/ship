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
ship <slug>
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

| Command                           | Description              |
| --------------------------------- | ------------------------ |
| `ship <slug>`                     | Full publish             |
| `ship publish <slug>`             | Full publish             |
| `ship publish metadata <slug>`    | Publish metadata only    |
| `ship publish screenshots <slug>` | Publish screenshots only |
| `ship publish app <slug>`         | Publish app binary only  |
| `ship publish status <slug>`      | Show publish job status  |
| `ship publish validate <slug>`    | Pre-publish validation   |
| `ship publish --app-id <id>`      | Full publish by ID       |

All publish commands accept `--app-id <id>` or `--app-slug <slug>` for explicit targeting, and `--platform ios,android` to target specific platforms.

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
