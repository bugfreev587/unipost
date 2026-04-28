#!/usr/bin/env bash

# Cut a release across all three SDKs in /Users/xiaoboyu/unipost-dev/:
#   - bumps version files in each repo
#   - rebuilds the JS dist so it ships the new SDK_VERSION header
#   - runs basic local checks (Go test, Python import, JS dist node --check)
#   - runs the three source-validation suites before it will tag a release
#   - commits and tags v<version> in each repo
#   - optionally pushes commit + tag to origin/main
#
# Usage:
#   scripts/release/create-sdk-release.sh <version> [--push]
#
# Each unipost-dev repo must be on `main` with a clean working tree.

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-}"
PUSH="${2:-}"
TAG="v${VERSION}"
SDKS_ROOT="${UNIPOST_DEV_ROOT:-/Users/xiaoboyu/unipost-dev}"
REPOS=(sdk-js sdk-python sdk-go)
UNIPOST_API_KEY="${UNIPOST_API_KEY:-}"
BASE_URL="${BASE_URL:-https://api.unipost.dev}"
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID:-}"
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW:-false}"
SOURCE_VALIDATION_LOG_DIR="${SOURCE_VALIDATION_LOG_DIR:-${ROOT_DIR}/artifacts/sdk-source-validation-release}"

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version> [--push]" >&2
  exit 64
fi

if [[ -n "$PUSH" && "$PUSH" != "--push" ]]; then
  echo "unknown option: $PUSH" >&2
  exit 64
fi

if [[ -z "$UNIPOST_API_KEY" ]]; then
  echo "UNIPOST_API_KEY is required so source validation can run before release" >&2
  exit 64
fi

# Pre-flight: each repo must exist, be on main, be clean, and not already
# have the target tag.
for repo in "${REPOS[@]}"; do
  dir="${SDKS_ROOT}/${repo}"
  if [[ ! -d "$dir/.git" ]]; then
    echo "not a git repo: $dir" >&2
    exit 1
  fi
  branch=$(git -C "$dir" branch --show-current)
  if [[ "$branch" != "main" ]]; then
    echo "$repo is on branch '$branch'; expected 'main'" >&2
    exit 1
  fi
  if [[ -n "$(git -C "$dir" status --short)" ]]; then
    echo "$repo working tree is not clean; commit or stash first" >&2
    exit 1
  fi
  if git -C "$dir" rev-parse "$TAG" >/dev/null 2>&1; then
    echo "$repo already has tag $TAG" >&2
    exit 1
  fi
done

# Bump versions in each repo.
"${ROOT_DIR}/scripts/release/bump-sdk-version.sh" "$VERSION"

# Rebuild JS dist so the published bundle carries the new SDK_VERSION.
(
  cd "${SDKS_ROOT}/sdk-js"
  if [[ -f package-lock.json ]]; then
    npm ci --silent
  else
    npm install --silent
  fi
  npm run build --silent
)

# Smoke checks before tagging.
node --check "${SDKS_ROOT}/sdk-js/dist/index.mjs"
python3 -c "import sys; sys.path.insert(0, '${SDKS_ROOT}/sdk-python'); import unipost; assert unipost.__version__ == '${VERSION}', unipost.__version__"
(
  cd "${SDKS_ROOT}/sdk-go"
  go test ./... >/dev/null
)

# Hard gate: all three source-validation suites must pass before we tag.
LOG_DIR="${SOURCE_VALIDATION_LOG_DIR}" \
UNIPOST_DEV_ROOT="${SDKS_ROOT}" \
UNIPOST_API_KEY="${UNIPOST_API_KEY}" \
BASE_URL="${BASE_URL}" \
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID}" \
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW}" \
  bash "${ROOT_DIR}/scripts/sdk-source-validation/run-suite.sh" sdk-js

LOG_DIR="${SOURCE_VALIDATION_LOG_DIR}" \
UNIPOST_DEV_ROOT="${SDKS_ROOT}" \
UNIPOST_API_KEY="${UNIPOST_API_KEY}" \
BASE_URL="${BASE_URL}" \
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID}" \
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW}" \
  bash "${ROOT_DIR}/scripts/sdk-source-validation/run-suite.sh" sdk-python

LOG_DIR="${SOURCE_VALIDATION_LOG_DIR}" \
UNIPOST_DEV_ROOT="${SDKS_ROOT}" \
UNIPOST_API_KEY="${UNIPOST_API_KEY}" \
BASE_URL="${BASE_URL}" \
TEST_ACCOUNT_ID="${TEST_ACCOUNT_ID}" \
TEST_PUBLISH_NOW="${TEST_PUBLISH_NOW}" \
  bash "${ROOT_DIR}/scripts/sdk-source-validation/run-suite.sh" sdk-go

# Commit + tag in each repo.
for repo in "${REPOS[@]}"; do
  dir="${SDKS_ROOT}/${repo}"
  case "$repo" in
    sdk-js)
      git -C "$dir" add package.json src/http.ts dist/
      ;;
    sdk-python)
      git -C "$dir" add pyproject.toml unipost/__init__.py unipost/http.py unipost/async_client.py
      ;;
    sdk-go)
      git -C "$dir" add unipost/client.go
      ;;
  esac
  if [[ -z "$(git -C "$dir" diff --cached --name-only)" ]]; then
    echo "$repo: no version-related changes detected; skipping commit" >&2
    continue
  fi
  git -C "$dir" commit -m "Release v${VERSION}"
  git -C "$dir" tag "$TAG"
done

if [[ "$PUSH" == "--push" ]]; then
  for repo in "${REPOS[@]}"; do
    dir="${SDKS_ROOT}/${repo}"
    git -C "$dir" push origin main
    git -C "$dir" push origin "$TAG"
    echo "${repo}: pushed main + ${TAG}"
  done
else
  echo
  echo "Created release commits and tags locally. To publish, run:"
  for repo in "${REPOS[@]}"; do
    echo "  git -C ${SDKS_ROOT}/${repo} push origin main && git -C ${SDKS_ROOT}/${repo} push origin ${TAG}"
  done
fi
