#!/usr/bin/env bash
# This fork is FerretDB v1. FerretDB v2 is different software, not the next
# release after v1.99.0. Minor versions grow without a two-digit rollover.
set -euo pipefail
version="${1:-}"
bash "$(dirname "$0")/validate-version.sh" "$version"
[[ "$version" =~ ^v1\.([0-9]+)\.[0-9]+$ ]]
minor="${BASH_REMATCH[1]}"
printf 'v1.%s.0\n' "$((10#$minor + 1))"
