# CLAUDE.md

## What You're Building

A Go web application that processes CSV files for regulated gaming data. Users upload CSV files through a browser UI, the app identifies the CSV type, processes rows (hashing player IDs, normalizing data), and uploads results to a remote API. The app has a setup wizard, login/auth, a real-time dashboard with SSE, an archive of processed files, settings management, and a player deduplication database.

## How to Work

1. Read specs in `build-specification/` in numbered order (01 through 15)
2. Each spec is one implementation task -- complete it fully before moving on
3. Use sub-agents for implementation (spawn per spec with the Agent tool, fresh context)
4. After each sub-agent completes: build (`./build.sh -g -G -b`), test (`./build.sh -t`), smoke-test, commit
5. Maintain the `progress` file in the project root (see protocol below)
6. After ALL specs are done: run the self-test protocol described in README.md
7. Don't ask questions -- make reasonable choices

## Tech Stack Rules

- **Router:** `github.com/go-chi/chi/v5` with `github.com/go-chi/render`
- **Templates:** Edit `.templ` files only. Never hand-edit `_templ.go` files
- **Build:** Always use `./build.sh`. Never invoke `go build`, `go test`, `templ generate` directly
- **Frontend:** No npm, node, or JS build tools. htmx vendored under `static/vendor/`
- **CSS:** No CSS frameworks. Use design tokens from `design-system/tokens.css`. Follow `design-system/patterns.md` for component patterns. Follow `design-system/layout.md` for page structure and responsive behavior
- **State:** Goroutine actor pattern for all shared mutable state. No mutexes
- **Config:** CLI flags + TOML config. No environment variables
- **Tests:** Table-driven, `t.TempDir()` for isolation. No testify, no gomock

## Progress File Protocol

Maintain a file called `progress` in the project root. Update it after each spec is completed or started.

```
# Progress

## Spec 01: Project scaffold (01-project-scaffold.md)
Status: DONE
Files: build.sh, go.mod, go.sum, main.go, internal/...
Notes: Basic scaffold with all flags
Testable: ./file-uploader --help shows subcommands

## Spec 03: Configuration (03-configuration.md)
Status: IN PROGRESS
```

## builder_done File Protocol

When done with current work, write a file called `builder_done` with:

```
Agent: Builder
Scenario: <scenario from instructions>
Spec status:
- 01-project-scaffold.md: DONE
- 02-chanutil-and-interfaces.md: DONE
- 03-configuration.md: JUST COMPLETED
- 04-hashing-and-normalization.md: PENDING
- 05-csv-framework.md: PENDING
- 06-csv-type-definitions.md: PENDING
- 07-worker-pool.md: PENDING
- 08-player-db.md: PENDING
- 09-csv-generator.md: PENDING
- 10-application-state-machine.md: PENDING
- 11-auth-and-server.md: PENDING
- 12-setup-wizard.md: PENDING
- 13-dashboard-and-sse.md: PENDING
- 14-archive-settings-playersdb.md: PENDING
- 15-build-and-deploy.md: PENDING
```

## Feedback Directories

When returning from a review/test gate failure, read the appropriate directory for findings to address:

- `code-review/` -- code review findings
- `security-review/` -- security review findings
- `dev-test-results/` -- dev test results
- `uat-feedback/` -- UAT feedback

## Persistent Tooling Directories

These directories contain automation scripts created by review/test agents. Do not clear them between rounds:

- `test-harness/` -- dev test automation
- `security-harness/` -- security probe scripts
- `code-review-tools/` -- code review automation
