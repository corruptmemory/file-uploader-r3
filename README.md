# File Uploader

A Go web application for CSV file processing in regulated gaming. Users upload CSV files, the app identifies the type, processes rows (hashing player IDs, normalizing data), and uploads results to a remote API.

## Tech Stack

- **Router:** chi (`github.com/go-chi/chi/v5`) with `github.com/go-chi/render`
- **Templates:** templ (`github.com/a-h/templ`) -- type-safe HTML components
- **Frontend:** htmx (vendored, no npm/node), plain browser JS
- **CLI:** go-flags (`github.com/jessevdk/go-flags`) with subcommands
- **Config:** TOML via `github.com/BurntSushi/toml`, CLI flag overrides
- **State:** Goroutine actor pattern (no mutexes) for all shared mutable state
- **Storage:** File-based only, no database
- **Binary:** Single static binary with all assets embedded via `go:embed`

## Building

```bash
./build.sh -g -G -b    # Generate templ, download deps, build binary
./build.sh -t           # Run tests (includes -race detector)
./build.sh -g -G -b -t # Build and test
```

Always use `./build.sh`. Never invoke `go build`, `go test`, or `templ generate` directly.

## Running

```bash
./file-uploader --mock -p 8080    # Run in mock mode on port 8080
./file-uploader gen-config        # Print default TOML config to stdout
./file-uploader gen-csv --help    # CSV test data generator
```

## Constraints

- Build with `./build.sh` only
- No database -- all state is file-based
- No npm/node -- htmx is vendored under `static/vendor/`
- Actor pattern for shared state (no mutexes)
- Edit `.templ` files only, never `_templ.go`
- No CSS frameworks -- use design tokens from `design-system/tokens.css`
- Browser for Playwright testing: `/usr/bin/brave`

## Self-Test Protocol

Before declaring the build done, run through every item:

1. **All tests pass:** `./build.sh -t`
2. **Race detector clean:** `./build.sh -t` includes `-race` by default
3. **Smoke-test in mock mode:**
   - Start: `./file-uploader --mock -p 8080`
   - Exercise every feature with `curl` AND Playwright (browser: `/usr/bin/brave`)
   - Upload CSV files, verify processing pipeline works end-to-end
   - Check SSE dashboard updates in real time
   - Test login/logout, session expiry, session extension
   - Test settings page, archive page, players DB page
   - Test setup wizard (start without `--mock`)
4. **Code review:** Architecture, patterns, dead code, consistency
5. **Security review:** Auth, sessions, token handling, session replay attacks, cookie flags
6. **Fix any issues found, then re-test from step 1**
