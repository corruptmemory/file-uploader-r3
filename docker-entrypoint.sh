#!/usr/bin/env bash
set -e
exec /usr/local/bin/file-uploader -c /etc/file-uploader/file-uploader.toml "$@"
