# ============================================================================
# SHIP CLI MAKEFILE
# ============================================================================
# This Makefile provides automation for building, testing, and developing
# the Ship CLI. Run 'make help' to see all available commands.
# ============================================================================

# Default target - show help when running 'make' without arguments
.DEFAULT_GOAL := help

# ============================================================================
# BUILD CONFIGURATION
# ============================================================================

# Configurable build variables (can be overridden)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0

# Build directories
DIST_DIR := dist
TMP_DIR := tmp

# Binary output paths
CLI_BINARY := $(DIST_DIR)/ship
CLI_LINUX_AMD64 := $(DIST_DIR)/ship-linux-amd64
CLI_LINUX_ARM64 := $(DIST_DIR)/ship-linux-arm64

# Source file enumeration using find
GO_SOURCES := $(shell find . -type f -name '*.go' -not -path './deploy/*' -not -path './vendor/*')
GO_MOD_FILES := go.mod go.sum

# Ensure directories exist
$(DIST_DIR):
	@mkdir -p $(DIST_DIR)

$(TMP_DIR):
	@mkdir -p $(TMP_DIR)

# ============================================================================
# SETUP & INSTALLATION
# ============================================================================

## Initialize project for development (installs all dependencies)
#
# This command sets up your development environment by:
# - Installing system dependencies via Homebrew (if available)
# - Installing Go module dependencies
# - Preparing the project for development
#
# Run this once when you first clone the repository.
.PHONY: init
init:
	@if [ "$$(uname)" = "Darwin" ]; then \
		echo "Darwin detected."; \
		$(MAKE) init-darwin; \
	elif [ "$$(uname)" = "Linux" ]; then \
		echo "Linux detected."; \
		$(MAKE) init-linux; \
	else \
		echo "Not running on Darwin or Linux."; \
		exit 1; \
	fi
	$(MAKE) install

.PHONY: init-darwin
init-darwin:
	@if ! command -v brew >/dev/null 2>&1; then \
		echo "Homebrew not detected. Skipping system dependency installation."; \
		exit 1; \
	fi
	echo "Homebrew detected. Installing system dependencies..."; \
	brew bundle

.PHONY: init-linux
init-linux:
	@if ! command -v dprint >/dev/null 2>&1; then \
		echo "dprint not detected. Installing it using: curl -fsSL https://dprint.dev/install.sh | sh"; \
		exit 1; \
	fi

## Install and tidy Go module dependencies
#
# Downloads and installs all Go module dependencies and removes
# unused modules. Safe to run multiple times.
.PHONY: install
install:
	go mod tidy

# ============================================================================
# BUILDING
# ============================================================================

## Build the CLI binary
#
# Creates an optimized binary in dist/ship suitable for distribution.
# This target rebuilds only when sources change.
.PHONY: build
build: $(CLI_BINARY)

$(CLI_BINARY): $(GO_SOURCES) $(GO_MOD_FILES) | $(DIST_DIR)
	go build -o $@ ./cmd/cli

## Generate code using ent ORM code generator
#
# Runs code generation for the ent ORM framework, which generates:
# - Database schema and migration code
# - Type-safe database client code
# - Query builders and mutations
#
# Run this after making changes to ent schema files in ./ent/schema/
# Generated files will be placed in ./ent/
.PHONY: generate
generate:
	go generate ./ent

## Build Linux binaries for containerization
#
# Creates statically-linked Linux binaries for both amd64 and arm64.
# These are used for Docker images and CI/CD environments.
# This target rebuilds only when sources change.
.PHONY: build-linux
build-linux: $(CLI_LINUX_AMD64) $(CLI_LINUX_ARM64)

$(CLI_LINUX_AMD64): $(GO_SOURCES) $(GO_MOD_FILES) | $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags "-s -w -extldflags '-static'" \
		-o $@ ./cmd/cli

$(CLI_LINUX_ARM64): $(GO_SOURCES) $(GO_MOD_FILES) | $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-ldflags "-s -w -extldflags '-static'" \
		-o $@ ./cmd/cli

## Build Docker image locally for testing
#
# Builds the CLI Docker image for linux/amd64 platform using buildx.
# The image will be loaded into your local Docker daemon.
#
# Prerequisites:
# - Docker with buildx support
.PHONY: docker-build
docker-build: $(CLI_LINUX_AMD64)
	@echo "Building Docker image with $(CLI_LINUX_AMD64)..."
	docker buildx build \
		--platform linux/amd64 \
		-t kula/ship:latest \
		--load \
		.
	@echo "Docker image built successfully: kula/ship:latest"

## Run Docker image
#
# Runs the ship CLI inside a Docker container with --help.
# Useful for verifying the Docker image works correctly.
.PHONY: docker-run
docker-run:
	docker run --rm kula/ship:latest --help

## Test Docker image
#
# Runs a quick smoke test to verify the Docker image is functional.
.PHONY: docker-test
docker-test:
	docker run --rm kula/ship:latest --version

# ============================================================================
# DEVELOPMENT & RUNNING
# ============================================================================

## Build and run the CLI
#
# Compiles the CLI binary and then runs it directly.
# Useful for quick local testing of CLI changes.
#
# Pass additional arguments via ARGS variable:
#   make run ARGS="auth login"
.PHONY: run
run: $(CLI_BINARY)
	@echo "Running the Ship CLI..."
	@echo ""
	$(CLI_BINARY) $(ARGS)

# ============================================================================
# TESTING & QUALITY ASSURANCE
# ============================================================================

## Run all tests in the project
#
# Executes all unit tests, integration tests, and benchmarks.
# Tests are run with Go's built-in testing framework.
#
# Use 'go test -v ./...' for verbose output.
# Use 'go test -race ./...' to check for race conditions.
.PHONY: test
test:
	go test ./...

## Run comprehensive static analysis and security checks
#
# Performs multiple code quality checks:
# - go vet: Examines Go source code for suspicious constructs
# - staticcheck: Advanced static analysis for bugs and performance issues
# - govulncheck: Scans for known security vulnerabilities
#
# Fix any issues reported before committing code.
.PHONY: analyze
analyze:
	go vet ./...
	go tool staticcheck ./...
	go tool govulncheck ./...

## Format code and organize imports
#
# Automatically formats all code in the project:
# - go mod tidy: Cleans up module dependencies
# - go fmt: Formats Go source code to standard style
# - dprint fmt: Formats other files (JSON, YAML, etc.) using dprint
#
# Run this before committing to ensure consistent code style.
.PHONY: format
format:
	go mod tidy
	go fmt ./...
	dprint fmt

# ============================================================================
# MAINTENANCE
# ============================================================================

## Clean all build artifacts
#
# Removes all generated binaries and temporary files:
# - dist/ directory (production binaries)
# - tmp/ directory (temporary files)
#
# Run this to force a full rebuild or clean up the workspace.
.PHONY: clean
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(DIST_DIR) $(TMP_DIR)
	@echo "Clean complete."

## Update all dependencies to latest compatible versions
#
# Updates all Go module dependencies to their latest minor/patch versions
# while respecting semantic versioning constraints. After updating:
# - Dependencies are updated to latest compatible versions
# - Code is automatically formatted
# - Module files are tidied
#
# Review changes carefully before committing dependency updates.
.PHONY: upgrade-deps
upgrade-deps:
	go get -u ./...
	$(MAKE) format

# ============================================================================
# HELP & DOCUMENTATION
# ============================================================================

## Show this help message with all available commands
#
# Displays a formatted list of all available make targets with descriptions.
# Commands are organized by topic for easy navigation.
.PHONY: help
help:
	@echo "=============================================="
	@echo "SHIP CLI DEVELOPMENT COMMANDS"
	@echo "=============================================="
	@echo ""
	@awk 'BEGIN { desc = ""; target = "" } \
	/^## / { desc = substr($$0, 4) } \
	/^\.PHONY: / && desc != "" { \
		target = $$2; \
		printf "\033[36m%-20s\033[0m %s\n", target, desc; \
		desc = ""; target = "" \
	}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Use 'make <command>' to run any command above."
	@echo "For detailed information, see comments in the Makefile."
	@echo ""
