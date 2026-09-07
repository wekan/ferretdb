#!/usr/bin/env bash
# Static release guard plus the command the published image must pass on its
# native architecture after it starts.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
workflow="$root/.github/workflows/docker.yml"
dockerfile="$root/Dockerfile.release"
grep -q 'MONGOSH_REPO: wekan/mongosh-patches' "$workflow"
grep -q 'gh release download.*--repo "$MONGOSH_REPO"' "$workflow"
grep -q 'sha256sum -c.*mongosh-' "$workflow"
grep -q 'mongosh-${m}.tgz' "$dockerfile"
grep -q 'arm/v7).*m=armv7' "$dockerfile"
grep -q 'ENV PATH=/opt/mongosh:' "$dockerfile"
if [ "${FERRETDB_MONGOSH_CONTAINER:-}" ]; then
  docker exec "$FERRETDB_MONGOSH_CONTAINER" mongosh \
    'mongodb://127.0.0.1:27017/?directConnection=true' \
    --quiet --eval 'if (db.runCommand({ping:1}).ok !== 1) quit(2)'
fi
echo 'FerretDB Docker mongosh wiring passed'
