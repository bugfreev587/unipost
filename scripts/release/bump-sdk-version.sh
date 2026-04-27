#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
VERSION="${1:-}"

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>" >&2
  exit 64
fi

if [[ ! "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid version: $VERSION" >&2
  exit 64
fi

export ROOT_DIR VERSION

python3 <<'PY'
import json
import os
from pathlib import Path
import re

root = Path(os.environ["ROOT_DIR"])
version = os.environ["VERSION"]

def replace(path: Path, pattern: str, repl: str) -> None:
    text = path.read_text()
    new_text, count = re.subn(pattern, repl, text, count=1, flags=re.MULTILINE)
    if count != 1:
        raise SystemExit(f"failed to update {path}")
    path.write_text(new_text)

package_json = root / "sdk/javascript/package.json"
package = json.loads(package_json.read_text())
package["version"] = version
package_json.write_text(json.dumps(package, indent=2) + "\n")

replace(
    root / "sdk/javascript/dist/index.mjs",
    r'const SDK_VERSION = "@unipost/sdk/[^"]+";',
    f'const SDK_VERSION = "@unipost/sdk/{version}";',
)

replace(
    root / "sdk/python/pyproject.toml",
    r'^version = "[^"]+"$',
    f'version = "{version}"',
)

replace(
    root / "sdk/python/unipost/__init__.py",
    r'^__version__ = "[^"]+"$',
    f'__version__ = "{version}"',
)

replace(
    root / "sdk/go/unipost/sdk.go",
    r'^\s*sdkVersion\s*=\s*"[^"]+"$',
    f'\tsdkVersion     = "{version}"',
)
PY

echo "Updated SDK versions to ${VERSION}"
