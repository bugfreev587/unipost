#!/usr/bin/env bash

# Bump the version of all three SDKs in /Users/xiaoboyu/unipost-dev/.
# Operates on the working tree only — no git commit/tag/push.
#
# Usage:
#   scripts/release/bump-sdk-version.sh <version>
#
# Example:
#   scripts/release/bump-sdk-version.sh 0.2.5

set -euo pipefail

VERSION="${1:-}"
SDKS_ROOT="${UNIPOST_DEV_ROOT:-/Users/xiaoboyu/unipost-dev}"

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>" >&2
  exit 64
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid version: $VERSION" >&2
  exit 64
fi

for repo in sdk-js sdk-python sdk-go; do
  if [[ ! -d "$SDKS_ROOT/$repo" ]]; then
    echo "missing repo: $SDKS_ROOT/$repo" >&2
    exit 1
  fi
done

export SDKS_ROOT VERSION

python3 <<'PY'
import json
import os
from pathlib import Path
import re

root = Path(os.environ["SDKS_ROOT"])
version = os.environ["VERSION"]


def replace(path: Path, pattern: str, repl: str) -> None:
    text = path.read_text()
    new_text, count = re.subn(pattern, repl, text, count=1, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"failed to update {path}")
    path.write_text(new_text)


# --- JavaScript ---
package_json = root / "sdk-js/package.json"
package = json.loads(package_json.read_text())
package["version"] = version
package_json.write_text(json.dumps(package, indent=2) + "\n")

replace(
    root / "sdk-js/src/http.ts",
    r'^const SDK_VERSION = "[^"]+";$',
    f'const SDK_VERSION = "{version}";',
)

# --- Python ---
replace(
    root / "sdk-python/pyproject.toml",
    r'^version = "[^"]+"$',
    f'version = "{version}"',
)

replace(
    root / "sdk-python/unipost/__init__.py",
    r'^__version__ = "[^"]+"$',
    f'__version__ = "{version}"',
)

replace(
    root / "sdk-python/unipost/http.py",
    r'^SDK_VERSION = "[^"]+"$',
    f'SDK_VERSION = "{version}"',
)

replace(
    root / "sdk-python/unipost/async_client.py",
    r'^SDK_VERSION = "[^"]+"$',
    f'SDK_VERSION = "{version}"',
)

# --- Go ---
replace(
    root / "sdk-go/unipost/client.go",
    r'^\s*sdkVersion\s*=\s*"[^"]+"$',
    f'\tsdkVersion     = "{version}"',
)
PY

echo "Updated SDK source versions to ${VERSION} in ${SDKS_ROOT}/{sdk-js,sdk-python,sdk-go}"
