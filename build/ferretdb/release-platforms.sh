#!/usr/bin/env bash
# Debian trixie-slim runtime platforms, intersected with release assets.
# ARMv6 and Loong64 have standalone binaries but no matching official base.
set -euo pipefail
dist="${1:?release asset directory required}"
platforms=""
for binary in amd64 arm64 armhf i386 ppc64le s390x riscv64; do
  [ -f "$dist/ferretdb-$binary" ] || continue
  shell="$binary"
  case "$binary" in
    armhf) platform=linux/arm/v7; shell=armv7 ;;
    i386) platform=linux/386 ;;
    *) platform="linux/$binary" ;;
  esac
  if [ ! -s "$dist/mongosh-$shell.tgz" ]; then
    echo "Missing mongosh-$shell.tgz for ferretdb-$binary" >&2
    exit 1
  fi
  platforms="${platforms:+$platforms,}$platform"
done
for binary in armv6 armel loong64; do
  if [ -f "$dist/ferretdb-$binary" ]; then
    echo "Omitting $binary container: no supported Debian runtime/mongosh pair; standalone binary remains available" >&2
  fi
done
if [ -z "$platforms" ]; then
  echo 'No supported FerretDB runtime platforms with matching mongosh packages' >&2
  exit 1
fi
printf '%s\n' "$platforms"
