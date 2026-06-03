# UniPost CLI

Installable UniPost CLI for developer quickstarts and AI agent operator workflows.

The current implementation supports Dashboard-generated setup tokens for
keychain-backed CLI auth, plus API-key fallback through `UNIPOST_API_KEY`.
Browser/device auth is still a later auth surface; setup-token login creates a
named revocable API key and stores the plaintext secret in OS keychain, not in
the local config file.

```bash
npx -y @unipost/cli agent bootstrap --setup-token ust_... --client codex --json
npx -y @unipost/cli config path --json
npx -y @unipost/cli config set base_url https://dev-api.unipost.dev --json
npx -y @unipost/cli config set default_profile_id pr_... --json
npx -y @unipost/cli config show --json
npx -y @unipost/cli auth login --api-key up_live_... --json
npx -y @unipost/cli auth logout --json
npx -y @unipost/cli auth status --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli init --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli quickstart --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli profiles list --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli connect create --platform linkedin --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli accounts list --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli accounts health --account sa_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli accounts capabilities --account sa_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli accounts metrics --account sa_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts validate --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts draft --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts create --from-file post.json --dry-run --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts schedule --account sa_... --caption "Hello" --at 2026-06-10T09:00:00Z --yes --idempotency-key demo-001 --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts wait post_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts cancel post_... --yes --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli posts retry post_... --result result_... --yes --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli media upload ./video.mp4 --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli media get med_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli media wait med_... --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli analytics summary --from 2026-06-01 --to 2026-06-30 --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent bootstrap --client codex --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent capabilities --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent plan --intent plan_publish_post --from-file post.json --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent mcp-test --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent mcp-config --client claude-code --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent mcp-config --client cursor --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent install --client codex --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent install --client claude-code --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli agent execute --plan plan.json --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli examples posts.create --lang node --account sa_...
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli examples mcp.claude-code --json
UNIPOST_API_KEY=up_live_... npx -y @unipost/cli doctor --json
npx -y @unipost/cli completion zsh
```

If you prefer a persistent shell command, run `npm install -g @unipost/cli`
once, then replace `npx -y @unipost/cli` with `unipost`.

The CLI stores non-secret local defaults such as the selected profile in
`~/.unipost/config.json`. `auth login --api-key` validates the key against
`/v1/workspace`, then stores redacted credential metadata only. `auth login
--setup-token` exchanges a short-lived Dashboard token for a named API key and
stores that secret in OS keychain; the config file stores only the keychain
locator and redacted metadata. `auth logout` removes local keychain/config
credentials only; revoke the named key from Dashboard if it should stop working
remotely.

Phase 5 supports agent planning, dry-run publish validation, scheduled publish,
post lifecycle waits, cancel/retry operations, account diagnostics, local media
upload/readiness waits, analytics reads, MCP client setup generation, MCP auth
testing, Codex/Claude Code instruction packages, and a limited structured
`agent execute` beta.
Publish-capable writes require explicit user approval through `--yes` and a
stable `--idempotency-key`; draft creation and dry-run validation remain safe
without live publishing.
`agent execute` only accepts structured plan actions for read-only, validate, or
draft-write flows from a current `agent plan --json` envelope with a matching
`catalog_version`. It rejects stale plans, live publish actions, pending
confirmations, and raw command strings from a plan.
