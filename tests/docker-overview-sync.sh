#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
workflow="$root/.github/workflows/docker.yml"

# The overview-sync step must exist, run after the image push step, and read
# README.md as the single source of truth for both registries' descriptions.
grep -q 'name: Sync the repository overview to each registry from README.md' "$workflow"
push_line="$(grep -n 'name: Build and push multi-arch image to each registry' "$workflow" | cut -d: -f1)"
sync_line="$(grep -n 'name: Sync the repository overview to each registry' "$workflow" | cut -d: -f1)"
[[ "$sync_line" -gt "$push_line" ]]
grep -q "readme=\"\$(cat README.md)\"" "$workflow"

# Docker Hub: login-for-JWT then PATCH full_description, exactly as
# hub.docker.com's own API expects (confirmed against peter-evans/dockerhub-description).
grep -q 'https://hub.docker.com/v2/users/login' "$workflow"
grep -q 'https://hub.docker.com/v2/repositories/wekanteam/ferretdb' "$workflow"
grep -q 'full_description' "$workflow"
grep -q 'Authorization: JWT \$jwt' "$workflow"

# Quay.io: PUT description with a Bearer token (the RepoUpdate schema from
# quay.io's own /api/v1/discovery document).
grep -q 'https://quay.io/api/v1/repository/wekan/ferretdb' "$workflow"
grep -q "'{description:\$d}'" "$workflow"
grep -q 'Authorization: Bearer \$quay_token' "$workflow"

# GHCR needs no sync call (it mirrors the linked repo's README automatically) —
# the step must say so rather than silently doing nothing for it.
grep -q 'GHCR needs no such step' "$workflow"

# Credentials must never be echoed or logged in plaintext anywhere in the step.
sync_body="$(awk '
  /name: Sync the repository overview to each registry/ {selected=1}
  selected && /run: \|/ {body=1; next}
  body && /^      - / {exit}
  body
' "$workflow")"
! grep -qE 'echo.*\$(token|jwt|quay_token|DOCKERHUB_AUTH|QUAY_AUTH)\b' <<<"$sync_body"
! grep -q 'set -x' <<<"$sync_body"

echo 'docker-overview-sync: docker.yml syncs README.md to Docker Hub and Quay.io without leaking credentials'
