# Branch Pull Chain Sync

The environment branches form a promotion chain that flows **up**:

```
dev  ->  staging  ->  main (prod)
```

Feature work is promoted upward via PRs (see `AGENTS.md`).

The **pull chain** flows the other way — **down** — so each lower environment
"pulls" from the layer above it and never falls behind hotfixes or production
merges:

```
main (prod)  ->  staging  ->  dev
```

- `staging` pulls from `main` (prod)
- `dev` pulls from `staging`

## How to run it

It is a **manual-only** GitHub Actions workflow:
`.github/workflows/branch-pull-chain-sync.yml`.

1. Actions tab → **Branch Pull Chain Sync** → **Run workflow**.
2. Choose the leg:
   - `full-chain` — `main` → `staging`, then `staging` → `dev` (default)
   - `prod-to-staging` — `main` → `staging` only
   - `staging-to-dev` — `staging` → `dev` only

Each leg merges the upper branch into the lower one (`--no-ff`) and pushes.
If a leg is already up to date, it is a no-op. On a merge conflict the leg
fails with a clear error and pushes nothing — resolve manually and re-run.

## Underlying script

Both legs call `scripts/sync/pull-chain.sh <upper> <lower>`, so you can run a
sync locally too:

```bash
scripts/sync/pull-chain.sh main staging
scripts/sync/pull-chain.sh staging dev
```

## Caveats

- This performs a **direct merge-and-push** to the shared `staging`/`dev`
  branches, bypassing the normal PR + Preview Acceptance gate in `AGENTS.md`.
  It is intentionally manual so it only runs when a down-sync is intended.
- If branch protection blocks the `GITHUB_TOKEN` from pushing to `staging`/
  `dev`, allow the Actions bot to push to those branches (or grant an
  exception), otherwise the push step will fail.
