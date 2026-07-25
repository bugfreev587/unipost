#!/usr/bin/env bash
#
# Down-sync one leg of the branch pull chain by merging an upper (source)
# branch into the lower (target) branch and pushing the result.
#
# Direction of flow is DOWN the environment chain:
#   prod (main) -> staging -> dev
# i.e. lower environments pull in whatever landed higher up (hotfixes,
# production merges) so they never fall behind.
#
# Usage:
#   scripts/sync/pull-chain.sh <source-upper-branch> <target-lower-branch>
#
# Examples:
#   scripts/sync/pull-chain.sh main staging
#   scripts/sync/pull-chain.sh staging dev
#
# Exit codes:
#   0  success (pushed) or already up to date (nothing to do)
#   1  merge conflict or unexpected git failure -> resolve manually
#
set -euo pipefail

SOURCE="${1:?source (upper) branch is required, e.g. main}"
TARGET="${2:?target (lower) branch is required, e.g. staging}"

echo "==> Pull-chain sync: ${SOURCE} -> ${TARGET}"

git fetch --prune origin "${SOURCE}" "${TARGET}"

# Start the target branch exactly at its remote tip.
git checkout -B "${TARGET}" "origin/${TARGET}"

# If the target already contains every commit from the source there is
# nothing to merge; report and exit cleanly so callers can chain legs.
if git merge-base --is-ancestor "origin/${SOURCE}" "HEAD"; then
  echo "==> ${TARGET} already contains all of ${SOURCE}; nothing to sync."
  exit 0
fi

MERGE_MSG="chore(sync): merge ${SOURCE} into ${TARGET} [pull-chain]"

if ! git merge --no-ff --no-edit -m "${MERGE_MSG}" "origin/${SOURCE}"; then
  echo "::error::Merge conflict syncing ${SOURCE} -> ${TARGET}. Resolve manually and re-run." >&2
  git merge --abort || true
  exit 1
fi

git push origin "HEAD:${TARGET}"
echo "==> Pushed ${TARGET} (merged ${SOURCE})."
