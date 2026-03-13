#!/usr/bin/env bash
set -euo pipefail

# --- Prerequisites ---
if [[ "${BASH_VERSINFO[0]}" -lt 5 ]]; then
    echo "ERROR: Bash 5+ required (found ${BASH_VERSION})" >&2
    exit 1
fi

for cmd in git realpath dirname go; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: '$cmd' not found in PATH" >&2
        exit 1
    fi
done

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# --- Defaults ---
do_help=false
do_clean=false
do_clean_all=false
do_generate=false
do_templ=false
do_build=false
do_test=false
do_docker=false
do_dist=false
do_refresh_htmx=false

api_env=""
api_endpoint=""
jwt_key=""
tagname="latest"

# --- Parse flags ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        -h|--help)          do_help=true; shift ;;
        -c|--clean)         do_clean=true; shift ;;
        -C|--clean-all)     do_clean_all=true; shift ;;
        -g|--generate)      do_generate=true; shift ;;
        -G|--templ-generate) do_templ=true; shift ;;
        -b|--build)         do_build=true; shift ;;
        -t|--test)          do_test=true; shift ;;
        -D|--docker-build)  do_docker=true; shift ;;
        -d|--dist)          do_dist=true; shift ;;
        --refresh-htmx)     do_refresh_htmx=true; shift ;;
        --api-env)          api_env="$2"; shift 2 ;;
        --api-endpoint)     api_endpoint="$2"; shift 2 ;;
        --jwt-key)          jwt_key="$2"; shift 2 ;;
        --tagname)          tagname="$2"; shift 2 ;;
        *)
            echo "Unknown flag: $1" >&2
            exit 1
            ;;
    esac
done

# --- Help ---
if $do_help; then
    cat <<'USAGE'
Usage: ./build.sh [flags]

Flags:
  -h, --help             Print this help and exit
  -c, --clean            Clean generated artifacts
  -C, --clean-all        Deep clean (includes module cache)
  -g, --generate         Run go generate (auto-installs stringer)
  -G, --templ-generate   Run templ generate (auto-installs templ)
  -b, --build            Build binary with ldflags
  -t, --test             Run tests
  -D, --docker-build     Full build + Docker image (implies -c -g -G -b)
  -d, --dist             Build distribution archive
  --refresh-htmx         Download latest htmx.min.js
  --api-env VALUE        API environment name
  --api-endpoint VALUE   API endpoint URL
  --jwt-key VALUE        JWT signing key
  --tagname VALUE        Docker image tag (default: latest)
USAGE
    exit 0
fi

# --- Docker implies -c -g -G -b ---
if $do_docker; then
    do_clean=true
    do_generate=true
    do_templ=true
    do_build=true
fi

# --- Git metadata ---
sha="$(git rev-parse HEAD 2>/dev/null || echo "unknown")"
head_version_tag="$(git tag --list 'v*' --points-at HEAD 2>/dev/null | head -1 || echo "")"
dirty="False"
if [[ -n "$(git diff --stat 2>/dev/null)" ]]; then
    dirty="True"
fi
describe_version="$(git describe --tags --long --match 'v*' 2>/dev/null || echo "")"
describe_all="$(git describe --all --long 2>/dev/null || echo "")"
last_commit_date="$(git log -1 --format=%cd 2>/dev/null || echo "")"

snapshot="True"
if [[ "$dirty" == "False" && -n "$head_version_tag" ]]; then
    snapshot="False"
fi

# --- Refresh htmx ---
if $do_refresh_htmx; then
    echo "==> Downloading htmx.min.js"
    curl -sL https://unpkg.com/htmx.org/dist/htmx.min.js -o static/vendor/htmx.min.js
    echo "    Done ($(wc -c < static/vendor/htmx.min.js) bytes)"
fi

# --- Clean ---
if $do_clean; then
    echo "==> Cleaning"
    go clean .
    rm -f file-uploader
fi

if $do_clean_all; then
    echo "==> Deep cleaning"
    go clean .
    rm -f file-uploader
    go clean --modcache
fi

# --- Generate ---
if $do_generate; then
    if ! command -v stringer &>/dev/null; then
        echo "==> Installing stringer"
        go install golang.org/x/tools/cmd/stringer@latest
    fi
    echo "==> Running go generate"
    go generate -v ./...
fi

# --- Templ generate ---
if $do_templ; then
    if ! command -v templ &>/dev/null; then
        echo "==> Installing templ"
        go install github.com/a-h/templ/cmd/templ@latest
    fi
    echo "==> Running templ generate"
    templ generate
fi

# --- Build ---
if $do_build; then
    echo "==> Building"
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
fi

# --- Test ---
if $do_test; then
    echo "==> Running tests"
    go test -race ./...
fi

# --- Dist ---
if $do_dist; then
    echo "==> Building distribution archive"
    # Placeholder for distribution build
    echo "    Distribution build not yet implemented"
fi

# --- Docker ---
if $do_docker; then
    echo "==> Building Docker image"
    # Placeholder for Docker build
    echo "    Docker build not yet implemented"
fi
