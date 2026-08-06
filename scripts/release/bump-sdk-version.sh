#!/usr/bin/env bash

# Bump the version of all SDKs in /Users/xiaoboyu/unipost-dev/.
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

for repo in sdk-js sdk-python sdk-go sdk-java; do
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


def replace_all(path: Path, pattern: str, repl: str, expected_count: int) -> None:
    text = path.read_text()
    new_text, count = re.subn(pattern, repl, text, flags=re.MULTILINE)
    if count != expected_count:
        raise SystemExit(
            f"failed to update {path}: expected {expected_count} matches, found {count}"
        )
    path.write_text(new_text)


# --- JavaScript ---
package_json = root / "sdk-js/package.json"
package = json.loads(package_json.read_text())
package["version"] = version
package_json.write_text(json.dumps(package, indent=2) + "\n")

package_lock_json = root / "sdk-js/package-lock.json"
package_lock = json.loads(package_lock_json.read_text())
package_lock["version"] = version
package_lock["packages"][""]["version"] = version
package_lock_json.write_text(json.dumps(package_lock, indent=2) + "\n")

replace(
    root / "sdk-js/src/http.ts",
    r'^const SDK_VERSION = "[^"]+";$',
    f'const SDK_VERSION = "{version}";',
)

replace(
    root / "sdk-js/tests/release-workflows.test.ts",
    r'expect\(packageJson\.version\)\.toBe\("[^"]+"\);',
    f'expect(packageJson.version).toBe("{version}");',
)

replace(
    root / "sdk-js/tests/users.test.ts",
    r'"@unipost/sdk/[0-9]+\.[0-9]+\.[0-9]+"',
    f'"@unipost/sdk/{version}"',
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

replace(
    root / "sdk-python/README.md",
    r'^## Latest release: v[^\s]+$',
    f'## Latest release: v{version}',
)

replace(
    root / "sdk-python/tests/test_release.py",
    r'assert \'version = "[^"]+"\' in pyproject',
    f'assert \'version = "{version}"\' in pyproject',
)

replace_all(
    root / "sdk-python/tests/test_release.py",
    r'assert (unipost\.__version__|http\.SDK_VERSION|async_client\.SDK_VERSION) == "[^"]+"',
    lambda match: f'assert {match.group(1)} == "{version}"',
    3,
)

# --- Go ---
replace(
    root / "sdk-go/unipost/client.go",
    r'^\s*sdkVersion\s*=\s*"[^"]+"$',
    f'\tsdkVersion     = "{version}"',
)

replace(
    root / "sdk-go/README.md",
    r'^## Latest release: v[^\s]+$',
    f'## Latest release: v{version}',
)

replace(
    root / "sdk-go/README.md",
    r'go get github\.com/unipost-dev/sdk-go@v[^\s]+',
    f'go get github.com/unipost-dev/sdk-go@v{version}',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'if sdkVersion != "[0-9]+\.[0-9]+\.[0-9]+"',
    f'if sdkVersion != "{version}"',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'sdkVersion = %q, want [0-9]+\.[0-9]+\.[0-9]+',
    f'sdkVersion = %q, want {version}',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'if userAgent != "unipost-go/[0-9]+\.[0-9]+\.[0-9]+"',
    f'if userAgent != "unipost-go/{version}"',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'userAgent = %q, want unipost-go/[0-9]+\.[0-9]+\.[0-9]+',
    f'userAgent = %q, want unipost-go/{version}',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'Latest release: v[0-9]+\.[0-9]+\.[0-9]+',
    f'Latest release: v{version}',
)

replace(
    root / "sdk-go/unipost/release_test.go",
    r'go get github\.com/unipost-dev/sdk-go@v[0-9]+\.[0-9]+\.[0-9]+',
    f'go get github.com/unipost-dev/sdk-go@v{version}',
)

# --- Java ---
replace(
    root / "sdk-java/build.gradle.kts",
    r'^version = "[^"]+"$',
    f'version = "{version}"',
)

replace(
    root / "sdk-java/README.md",
    r'^## Latest release: v[^\s]+$',
    f'## Latest release: v{version}',
)

replace(
    root / "sdk-java/README.md",
    r'<version>[^<]+</version>',
    f'<version>{version}</version>',
)

replace(
    root / "sdk-java/README.md",
    r'implementation\("dev\.unipost:sdk-java:[^"]+"\)',
    f'implementation("dev.unipost:sdk-java:{version}")',
)

replace(
    root / "sdk-java/src/main/java/dev/unipost/UniPost.java",
    r'^    public static final String SDK_VERSION = "[^"]+";$',
    f'    public static final String SDK_VERSION = "{version}";',
)

replace(
    root / "sdk-java/src/main/java/dev/unipost/ApiHttpClient.java",
    r'"unipost-java/[^"]+"',
    f'"unipost-java/{version}"',
)

replace(
    root / "sdk-java/pom.xml",
    r'<version>[^<]+</version>',
    f'<version>{version}</version>',
)

replace_all(
    root / "sdk-java/src/test/java/dev/unipost/InboxTest.java",
    r'"unipost-java/[0-9]+\.[0-9]+\.[0-9]+"',
    f'"unipost-java/{version}"',
    2,
)

replace(
    root / "sdk-java/src/test/java/dev/unipost/InboxTest.java",
    r'assertEquals\("[0-9]+\.[0-9]+\.[0-9]+", UniPost\.SDK_VERSION\);',
    f'assertEquals("{version}", UniPost.SDK_VERSION);',
)
PY

echo "Updated SDK source versions to ${VERSION} in ${SDKS_ROOT}/{sdk-js,sdk-python,sdk-go,sdk-java}"
