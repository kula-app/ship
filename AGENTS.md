# Agent Instructions

## Package Manager

- **Go** with Makefile
- Run `make help` to discover all available commands
- Always use Makefile targets — never raw `go` or other tool commands

## Commit Attribution

- Do NOT include `Co-Authored-By`, AI assistant names, or `Generated-with` in commits, PRs, or code comments

## Commit Messages

- [Conventional Commits 1.0.0](https://www.conventionalcommits.org/)
- Format: `<type>[optional scope]: <description>`
- Types: `feat`, `fix`, `build`, `chore`, `ci`, `docs`, `style`, `refactor`, `perf`, `test`
- Breaking changes: `feat!:` or `BREAKING CHANGE:` footer

## PR Workflow

- No stacked PRs — always branch from `main`
- Feature branches preferred; multi-feature PRs must use rebase merge
- Branch prefixes: `feature/`, `fix/`, `refactor/`, `docs/`, `chore/`
- Use `gh pr create --body-file` — avoid inline `--body` with backticks

## Key Conventions

### File Naming — Platform-Safe Suffixes

- Go treats `_<GOOS>.go` suffixes as platform-specific (only compiled for that OS)
- **Never** end filenames with `_ios.go`, `_android.go`, `_darwin.go`, `_linux.go`, `_windows.go` unless genuinely platform-specific
- Use `_ios_patch.go`, `_android_config.go` patterns instead
- Verify with: `make build` (platform-specific files will cause build failures)

### Logging

- Use `InfoContext`, `ErrorContext`, `WarnContext`, `DebugContext` when `context.Context` is available
- Bare logger calls only in init code where no request context exists
- Required for Sentry trace association via `sentryslog`
- Never use `fmt.Print` in services — always use injected `slog.Logger`

### CLI Output Modes

- **Text mode** (default): logs to stderr via slog, results to stdout
- **JSON mode** (`--log-format json`): silent logger, only JSON to stdout
- **stdout** = structured output, **stderr** = diagnostics

### JSON Parsing

- Always use `jq` for JSON parsing — never `python`, `node`, or other tools

### CLI Flags

- `--output` / `-o` = file path (never format)
- `--log-format` = output format (`text` or `json`)
- Env var fallbacks: `SHIPABLE_<FIELD>` (flag takes priority)

### Control Flow

- Prefer early-exit: `if cond { return }` — no `else`/`else if` after a returning `if` block
- Avoid variable scoping in `if` initializers when it forces `else if` chains

### Validation

- Use `github.com/go-playground/validator/v10` struct tags — no custom validation logic

### Local Database (SQLite)

- Database file: `~/.ship/cli.db`
- Uses `github.com/mattn/go-sqlite3` with ent ORM
- Schema managed via ent auto-migration in `ent/schema/`

### Import Organization

- Group: stdlib, external deps, internal packages (separated by blank lines)

## Infrastructure (Pulumi)

- `deploy/` is a **separate Go module** — run `cd deploy && go build ./...` to verify
- Use `deploy/scripts/pulumi.sh` wrapper for all Pulumi commands
