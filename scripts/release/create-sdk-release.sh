#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-}"
PUSH="${2:-}"
TAG="v${VERSION}"

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version> [--push]" >&2
  exit 64
fi

if [[ -n "$PUSH" && "$PUSH" != "--push" ]]; then
  echo "unknown option: $PUSH" >&2
  exit 64
fi

cd "$ROOT_DIR"

if [[ -n "$(git status --short)" ]]; then
  echo "working tree is not clean; commit or stash changes before creating a release" >&2
  exit 1
fi

if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "tag already exists: $TAG" >&2
  exit 1
fi

scripts/release/bump-sdk-version.sh "$VERSION"

node --check scripts/sdk-validation/js/unipost-sdk-test.mjs
python3 -c "import sys; sys.path.insert(0, 'sdk/python'); import unipost; print(unipost.__version__)"
(
  cd sdk/go
  go test ./...
)

git add \
  sdk/javascript/package.json \
  sdk/javascript/dist/index.mjs \
  sdk/python/pyproject.toml \
  sdk/python/unipost/__init__.py \
  sdk/go/unipost/sdk.go

git commit -m "Release SDKs v${VERSION}"
git tag "$TAG"

if [[ "$PUSH" == "--push" ]]; then
  git push origin main
  git push origin "$TAG"
  echo "Release commit and tag pushed: ${TAG}"
else
  echo "Created release commit and tag locally: ${TAG}"
  echo "Next:"
  echo "  git push origin main"
  echo "  git push origin ${TAG}"
fi
