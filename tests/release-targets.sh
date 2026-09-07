#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
build="$root/build.sh"
workflow="$root/.github/workflows/release-all.yml"

targets="$(sed -n '/^FERRETDB_DIST_TARGETS=(/,/^)/p' "$build" |
  sed -n 's/^  "\([^ " ]*\) .*/\1/p')"
expected="amd64 arm64 armhf armv6 armel i386 ppc64le s390x riscv64 loong64 win64 win-arm64 win32 mac-amd64 mac-arm64 freebsd-amd64 freebsd-i386 freebsd-armel freebsd-armv6 freebsd-armv7 freebsd-arm64 netbsd-amd64 openbsd-amd64 openbsd-arm64 android-arm64"

[ "$(printf '%s\n' "$targets" | wc -l | tr -d ' ')" = 25 ]
[ "$(printf '%s\n' "$targets" | sort -u | wc -l | tr -d ' ')" = 25 ]
for target in $expected; do
  printf '%s\n' "$targets" | grep -qxF "$target"
done
order="$(sed -n 's/^          order="\([^"]*\)"/\1/p' "$workflow")"
[ "$order" = "$expected" ]

echo 'release-targets: 25 unique build targets and release-note order agree'
