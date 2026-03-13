# 15 — Build, Embed, and Deploy

**Dependencies:** 01-project-scaffold.md (build.sh, settables.go, project layout), 11-auth-and-server.md (WebApp, Server).

**Produces:** Complete build.sh script, static asset embedding, Air config, Dockerfile, systemd unit, install script.

---

## 1. build.sh — Central Build Script

All build operations go through `./build.sh`. Never invoke `go build`, `go test`, or `templ generate` directly.

### Prerequisites

Checked at startup with helpful error messages:
- Bash 5+
- `git`, `realpath`, `dirname`, `which`
- Go toolchain in PATH

### Flags

| Flag | Long | Description |
|---|---|---|
| `-h` | `--help` | Print usage and exit |
| `-c` | `--clean` | Clean: `go clean .`, remove binary |
| `-C` | `--clean-all` | Deep clean: also `go clean --modcache` |
| `-g` | `--generate` | Run `go generate -v ./...` (auto-installs `stringer` if missing) |
| `-G` | `--templ-generate` | Run `templ generate` (auto-installs `templ` if missing) |
| `-b` | `--build` | Build binary with ldflags |
| `-t` | `--test` | Run `go test -race ./...` |
| `-D` | `--docker-build` | Full build + Docker image (implies -c -g -G -b) |
| `-d` | `--dist` | Build distribution archive (zip + tarball) |
| `--api-env` | | API environment name for Docker/dist |
| `--api-endpoint` | | API endpoint URL for Docker/dist |
| `--jwt-key` | | JWT signing key (auto-generated if omitted) |
| `--tagname` | | Docker image tag (default: `latest`) |

### Common Invocations

```bash
./build.sh -g -G -b           # Safe default for any change
./build.sh -G -b               # After editing .templ files
./build.sh -b                   # After editing .go files only
./build.sh -t                   # Run all tests (with race detector)
./build.sh -C                   # Deep clean
```

### Auto-Install Dependencies

When `-g` passed and `stringer` not found:
```bash
go install golang.org/x/tools/cmd/stringer@latest
```

When `-G` passed and `templ` not found:
```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### Build Command

```bash
CGO_ENABLED=0 go build -ldflags "
  -X main.PublicAPIEnvironment=${api_env}
  -X main.PublicAPIEndpoint=${api_endpoint}
  -X main.ConfigVersion=1
  -X main.GitSHA=${sha}
  -X main.DirtyBuild=${dirty}
  -X 'main.GitFullVersionDescription=${describe_all}'
  -X 'main.GitDescribeVersion=${describe_version}'
  -X 'main.GitLastCommitDate=${last_commit_date}'
  -X main.GitVersion=${head_version_tag}
  -X main.SnapshotBuild=${snapshot}
" -v -o file-uploader .
```

### Git Metadata Collection

At script start, collect:
- `sha` — `git rev-parse HEAD`
- `head_version_tag` — `git tag --list 'v*' --points-at HEAD`
- `dirty` — `"True"` if `git diff --stat` non-empty, else `"False"`
- `describe_version` — `git describe --tags --long --match 'v*'` (empty if no tags)
- `describe_all` — `git describe --all --long`
- `last_commit_date` — `git log -1 --format=%cd`
- `snapshot` — `"False"` only if clean AND HEAD has valid semver tag, else `"True"`

### Distribution Build (`-d`)

1. Verify `--api-env` and `--api-endpoint` provided.
2. Resolve version (HEAD tag if present, otherwise prompt).
3. Generate random JWT key if not provided.
4. Build binary with version metadata.
5. Generate starter TOML config.
6. Copy: Dockerfile, docker-entrypoint.sh, install-systemd.sh, INSTALL.md.
7. Package into `dist/file-uploader-{version}.tar.gz` and `.zip`.

---

## 2. Static Asset Embedding

### Embed from Project Root

Place embed declarations in the main package where they can reach `static/`:

```go
// File: embed.go (project root)
package main

import "embed"

//go:embed static
var StaticFS embed.FS
```

Pass `StaticFS` to the server constructor. The server uses `fs.Sub(staticFS, "static/js")` etc. to extract subdirectories.

### Route Registration

| URL Path | Embedded Directory | Content-Type |
|---|---|---|
| `/js/*` | `static/js/` | `application/javascript` |
| `/css/*` | `static/css/` | `text/css` |
| `/img/*` | `static/img/` | auto-detected |
| `/vendor/*` | `static/vendor/` | auto-detected |
| `/favicon.ico` | `static/img/favicon.ico` | `image/x-icon` |

Use `http.FileServer(http.FS(...))` with `http.StripPrefix`.

### Static File Inventory

| File | Purpose |
|---|---|
| `static/css/tokens.css` | Design token CSS custom properties |
| `static/css/app.css` | Application styles |
| `static/js/app.js` | Session management, file upload, UI behavior |
| `static/js/sse.js` | Custom htmx SSE extension with reconnection |
| `static/vendor/htmx.min.js` | htmx library (vendored, not CDN) |
| `static/img/favicon.ico` | Favicon |

---

## 3. Development with Air

Config file: `.air.toml` in project root.

```toml
root = "."
testdata_dir = "testdata"
tmp_dir = "tmp"

[build]
  bin = "./file-uploader -p 8080"
  cmd = "./build.sh -G -b"
  delay = 1000
  exclude_dir = ["assets", "tmp", "vendor", "testdata", "data-processing", "players-db"]
  exclude_file = ["file-uploader.toml"]
  exclude_regex = ["_test.go"]
  exclude_unchanged = false
  follow_symlink = false
  full_bin = ""
  include_dir = []
  include_ext = ["go", "js", "css", "jpg", "gif", "ico", "png", "toml", "txt", "svg"]
  include_file = []
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
  poll_interval = 0
  rerun = false
  rerun_delay = 500
  send_interrupt = false
  stop_on_error = false

[color]
  app = ""
  build = "yellow"
  main = "magenta"
  runner = "green"
  watcher = "cyan"

[log]
  main_only = false
  time = false

[misc]
  clean_on_exit = false

[screen]
  clear_on_rebuild = false
  keep_scroll = true
```

Key behaviors:
- Build command: `./build.sh -G -b` (regenerate templ + compile).
- Binary runs with: `-p 8080`.
- Watches: `.go`, `.js`, `.css`, `.toml`, image files.
- Excludes: test files, data directories, config file.
- 1 second delay before rebuilding.

Install: `go install github.com/air-verse/air@latest`

---

## 4. Docker Build

### Dockerfile

Alpine base (not scratch — entrypoint script needs bash):

```dockerfile
FROM alpine:latest

RUN apk add --no-cache bash ca-certificates curl

RUN mkdir -p /etc/file-uploader

COPY file-uploader /usr/local/bin/file-uploader
COPY file-uploader.toml.tmp /etc/file-uploader/file-uploader.toml
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p /data
VOLUME /data

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
```

Binary is built OUTSIDE Docker by `build.sh`. Dockerfile just copies it in.

### docker-entrypoint.sh

```bash
#!/usr/bin/env bash
set -e
exec /usr/local/bin/file-uploader -c /etc/file-uploader/file-uploader.toml "$@"
```

---

## 5. Systemd Service

### install-systemd.sh

Run as root. Does:

1. Create `file-uploader` system user (no login shell, no home).
2. Create directory structure under `/opt/file-uploader/`:
   - `data/players-db/work/`
   - `data/data-processing/upload/`
   - `data/data-processing/processing/`
   - `data/data-processing/uploading/`
   - `data/data-processing/archive/`
3. Copy binary and config to `/opt/file-uploader/`.
4. Set ownership to `file-uploader` user.
5. Install systemd unit file.
6. Reload systemd daemon.

### Unit File

```ini
[Unit]
Description=File Uploader - CSV Processing and Data Synchronization
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=file-uploader
Group=file-uploader
WorkingDirectory=/opt/file-uploader
ExecStart=/opt/file-uploader/file-uploader -c /opt/file-uploader/file-uploader.toml
Restart=on-failure
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=file-uploader

NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/file-uploader/data
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

---

## Tests

| Test | Description |
|------|-------------|
| build.sh -b produces binary | Binary exists and is executable |
| build.sh -t runs tests | All tests execute with race detector |
| Static assets embedded | Binary serves /js/app.js, /css/tokens.css correctly |
| Embedded files have correct MIME types | .js → application/javascript, .css → text/css |
| Version metadata populated | PrintVersion shows non-"<unknown>" values after build |
| Health endpoint in binary | Start binary, GET /health → 200 "ok" |

## Acceptance Criteria

- [ ] build.sh supports all listed flags
- [ ] Auto-installs stringer and templ when missing
- [ ] Binary built with CGO_ENABLED=0 (static)
- [ ] Git metadata embedded via ldflags
- [ ] Static assets embedded via go:embed and served with correct MIME types
- [ ] Air config enables hot-reload development
- [ ] Docker image builds and runs correctly
- [ ] Systemd unit file includes security hardening
- [ ] Distribution archive contains binary + config + deploy files
- [ ] All tests pass
