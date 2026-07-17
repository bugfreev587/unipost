#!/usr/bin/env bash

set -euo pipefail

SUITE="${1:-}"
UNIPOST_DEV_ROOT="${UNIPOST_DEV_ROOT:-/Users/xiaoboyu/unipost-dev}"

case "$SUITE" in
  sdk-js)
    cd "${UNIPOST_DEV_ROOT}/sdk-js"
    npm ci
    npm test
    npm run typecheck
    npm run build
    ;;
  sdk-python)
    cd "${UNIPOST_DEV_ROOT}/sdk-python"
    python3 -m pip install --disable-pip-version-check -e '.[dev]'
    pytest -q
    mypy unipost
    ruff check unipost tests
    ;;
  sdk-go)
    cd "${UNIPOST_DEV_ROOT}/sdk-go"
    go test ./...
    go vet ./...
    ;;
  sdk-java)
    cd "${UNIPOST_DEV_ROOT}/sdk-java"
    ./gradlew test
    ;;
  *)
    echo "usage: $0 <sdk-js|sdk-python|sdk-go|sdk-java>" >&2
    exit 64
    ;;
esac
