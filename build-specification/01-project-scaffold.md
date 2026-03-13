# 01 — Project Scaffold

**Dependencies:** None (this is the first task).

**Produces:** Build system, project skeleton, main entry point, static asset structure.

---

## 1. Initialize the Project

```bash
go mod init github.com/corruptmemory/file-uploader-r3
```

Create the directory structure from CLAUDE.md. All directories must exist even if initially empty (use `.gitkeep` files for empty dirs that need to be tracked).

## 2. build.sh

Central build script. All build operations go through `./build.sh`.

### Prerequisites Check

At script start, verify:
- Bash 5+ (print helpful error if not)
- `git`, `realpath`, `dirname`, `which` are available
- Go toolchain is in PATH

### Flags

| Flag | Long | Description |
|---|---|---|
| `-h` | `--help` | Print usage and exit |
| `-c` | `--clean` | Clean generated artifacts (`go clean .`, remove binary) |
| `-C` | `--clean-all` | Deep clean: also `go clean --modcache` |
| `-g` | `--generate` | Run `go generate -v ./...` (auto-install `stringer` if missing) |
| `-G` | `--templ-generate` | Run `templ generate` (auto-install `templ` if missing) |
| `-b` | `--build` | Build binary with ldflags |
| `-t` | `--test` | Run `go test ./...` |
| `-D` | `--docker-build` | Full build + Docker image (implies -c -g -G -b) |
| `-d` | `--dist` | Build distribution archive |
| `--api-env` | | API environment name for Docker/dist builds |
| `--api-endpoint` | | API endpoint URL for Docker/dist builds |
| `--jwt-key` | | JWT signing key (auto-generated if omitted) |
| `--tagname` | | Docker image tag (default: `latest`) |

### Auto-Install Dependencies

When `-g` and `stringer` not found:
```bash
go install golang.org/x/tools/cmd/stringer@latest
```

When `-G` and `templ` not found:
```bash
go install github.com/a-h/templ/cmd/templ@latest
```

### Git Metadata Collection

Collect at script start:
- `sha` — `git rev-parse HEAD`
- `head_version_tag` — `git tag --list 'v*' --points-at HEAD`
- `dirty` — `"True"` if `git diff --stat` is non-empty, else `"False"`
- `describe_version` — `git describe --tags --long --match 'v*'` (empty if no tags)
- `describe_all` — `git describe --all --long`
- `last_commit_date` — `git log -1 --format=%cd`
- `snapshot` — `"False"` only if clean AND HEAD has a valid semver tag, else `"True"`

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

### Common Invocations

```bash
./build.sh -g -G -b    # Full build
./build.sh -G -b        # After .templ edits
./build.sh -b            # After .go edits only
./build.sh -t            # Run tests
```

## 3. settables.go — Build-Time Variables

File: `settables.go` in the main package (project root).

```go
package main

var (
    PublicAPIEnvironment       string
    PublicAPIEndpoint          string
    ConfigVersion              string
    GitSHA                     string = "<unknown>"
    DirtyBuild                 string = "<unknown>"
    GitFullVersionDescription  string = "<unknown>"
    GitDescribeVersion         string = "<unknown>"
    GitLastCommitDate          string = "<unknown>"
    GitVersion                 string = "<unknown>"
    SnapshotBuild              string = "<unknown>"
)

func PrintVersion() {
    println("Version: " + GitVersion)
    println("GitSHA: " + GitSHA)
    println("GitFullVersionDescription: " + GitFullVersionDescription)
    println("GitDescribeVersion: " + GitDescribeVersion)
    println("GitLastCommitDate: " + GitLastCommitDate)
    println("SnapshotBuild: " + SnapshotBuild)
    println("DirtyBuild: " + DirtyBuild)
    println("ConfigVersion: " + ConfigVersion)
    println("PublicAPIEnvironment: " + PublicAPIEnvironment)
    println("PublicAPIEndpoint: " + PublicAPIEndpoint)
}
```

## 4. main.go — Entry Point Skeleton

File: `main.go` in the project root.

```go
package main

import (
    "fmt"
    "os"

    flags "github.com/jessevdk/go-flags"
)

func main() {
    var args Args
    parser := flags.NewParser(&args, flags.Default)

    // Register subcommands (implemented in later specs)
    // parser.AddCommand("gen-config", ...)
    // parser.AddCommand("gen-csv", ...)

    if _, err := parser.Parse(); err != nil {
        if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
            os.Exit(0)
        }
        os.Exit(1)
    }

    if args.Version {
        PrintVersion()
        os.Exit(0)
    }

    // Server startup (implemented in spec 11)
    fmt.Println("Server startup not yet implemented")
}
```

The `Args` struct (defined in spec 03) will be added when the configuration spec is implemented. For now, use a minimal placeholder:

```go
type Args struct {
    Version    bool   `short:"v" long:"version" description:"Show version and exit"`
    ConfigFile string `short:"c" long:"config-file" description:"Config file path" default:"./file-uploader.toml"`
}
```

## 5. Static Asset Embedding

File: `embed.go` in the project root.

```go
package main

import "embed"

//go:embed static
var StaticFS embed.FS
```

Create placeholder static files:
- `static/css/tokens.css` — empty (populated in spec 13)
- `static/css/app.css` — empty
- `static/js/app.js` — empty
- `static/js/sse.js` — empty
- `static/vendor/htmx.min.js` — download htmx v1.9.x from the official CDN and vendor it
- `static/img/favicon.ico` — create a minimal placeholder

### Downloading htmx

In `build.sh`, add a `--refresh-htmx` flag that downloads the latest htmx:
```bash
curl -sL https://unpkg.com/htmx.org/dist/htmx.min.js -o static/vendor/htmx.min.js
```

For the initial build, download it manually or include it in the repo.

## 6. Air Configuration

File: `.air.toml` in the project root.

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
  include_ext = ["go", "js", "css", "jpg", "gif", "ico", "png", "toml", "txt", "svg"]
  kill_delay = "0s"
  log = "build-errors.log"
  poll = false
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

## 7. .gitignore

```
file-uploader
tmp/
*.log
data-processing/
players-db/
mock-output/
file-uploader.toml
```

---

## Tests

After completing this spec:
- `./build.sh -b` compiles successfully
- `./file-uploader -v` prints version info and exits
- `./file-uploader -h` prints help and exits

## Acceptance Criteria

- [ ] `go mod init` creates a valid module
- [ ] `build.sh` compiles with all flags working
- [ ] Build-time ldflags inject version metadata
- [ ] Static assets are embedded via `go:embed`
- [ ] Air config enables hot-reload development
- [ ] `.gitignore` excludes generated/data files
