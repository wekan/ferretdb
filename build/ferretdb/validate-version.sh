#!/usr/bin/env bash
# Release artifacts from this SQLite fork must never use FerretDB v2 tags.
set -euo pipefail
if [[ ! "${1:-}" =~ ^v1\.[0-9]+\.[0-9]+$ ]]; then
  echo "Expected a FerretDB v1 release tag (v1.MINOR.PATCH), got: ${1:-}" >&2
  exit 1
fi
