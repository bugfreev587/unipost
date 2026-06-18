# UniPost CLI

Installable UniPost CLI for developer quickstarts and AI agent operator workflows.

The current implementation supports Dashboard-generated setup tokens for
keychain-backed CLI auth, plus API-key fallback through `UNIPOST_API_KEY`.
Browser/device auth is still a later auth surface; setup-token login creates a
named revocable API key and stores the plaintext secret in OS keychain, not in
the local config file.

Install once:

```bash
npm install -g @unipost/cli
unipost --help
```

Use `unipost ...` by default after installation. The Dashboard setup-token
command assumes this global install and returns a `unipost auth login ...`
command for the current environment.

Update with `unipost upgrade`, then run `unipost --version` to confirm the
installed version. Use `unipost self help` for install, update, version, and
help commands.

The Dashboard setup-token command signs in the UniPost CLI only. It does not
install or configure Codex, Claude Code, Cursor, or any other local AI agent.

To let a local AI agent use UniPost:

1. Finish CLI auth first:
   `unipost auth login --setup-token ust_... --client terminal --base-url https://api.unipost.dev --json`
2. Add the UniPost instruction package for that agent:
   `unipost agent install --client codex --json`
3. In the agent session, have it run:
   `unipost agent bootstrap --client codex --json`

```bash
unipost auth login --setup-token ust_... --client terminal --base-url https://api.unipost.dev --json
unipost config path --json
unipost config set base_url https://dev-api.unipost.dev --json
unipost config set default_profile_id pr_... --json
unipost config show --json
unipost auth login --api-key up_live_... --json
unipost auth logout --json
unipost auth status --json
UNIPOST_API_KEY=up_live_... unipost init --json
UNIPOST_API_KEY=up_live_... unipost quickstart --json
UNIPOST_API_KEY=up_live_... unipost profiles list --json
UNIPOST_API_KEY=up_live_... unipost connect create --platform linkedin --json
UNIPOST_API_KEY=up_live_... unipost accounts list --json
UNIPOST_API_KEY=up_live_... unipost accounts health --account sa_... --json
UNIPOST_API_KEY=up_live_... unipost accounts capabilities --account sa_... --json
UNIPOST_API_KEY=up_live_... unipost accounts metrics --account sa_... --json
UNIPOST_API_KEY=up_live_... unipost posts validate --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... unipost posts draft --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... unipost posts create --from-file post.json --dry-run --json
UNIPOST_API_KEY=up_live_... unipost posts schedule --account sa_... --caption "Hello" --at 2026-06-10T09:00:00Z --yes --idempotency-key demo-001 --json
UNIPOST_API_KEY=up_live_... unipost posts wait post_... --json
UNIPOST_API_KEY=up_live_... unipost posts cancel post_... --yes --json
UNIPOST_API_KEY=up_live_... unipost posts retry post_... --result result_... --yes --json
UNIPOST_API_KEY=up_live_... unipost media upload ./video.mp4 --json
UNIPOST_API_KEY=up_live_... unipost media get med_... --json
UNIPOST_API_KEY=up_live_... unipost media wait med_... --json
UNIPOST_API_KEY=up_live_... unipost analytics summary --from 2026-06-01 --to 2026-06-30 --json
UNIPOST_API_KEY=up_live_... unipost agent bootstrap --client codex --json
UNIPOST_API_KEY=up_live_... unipost agent capabilities --json
UNIPOST_API_KEY=up_live_... unipost agent plan --intent plan_publish_post --from-file post.json --json
UNIPOST_API_KEY=up_live_... unipost agent mcp-test --json
UNIPOST_API_KEY=up_live_... unipost agent mcp-config --client claude-code --json
UNIPOST_API_KEY=up_live_... unipost agent mcp-config --client cursor --json
UNIPOST_API_KEY=up_live_... unipost agent install --client codex --json
UNIPOST_API_KEY=up_live_... unipost agent install --client claude-code --json
UNIPOST_API_KEY=up_live_... unipost agent execute --plan plan.json --json
UNIPOST_API_KEY=up_live_... unipost examples posts.create --lang node --account sa_...
UNIPOST_API_KEY=up_live_... unipost examples mcp.claude-code --json
UNIPOST_API_KEY=up_live_... unipost doctor --json
UNIPOST_API_KEY=up_live_... unipost doctor diagnose --json
UNIPOST_API_KEY=up_live_... unipost doctor explain --request-id req_... --json
UNIPOST_API_KEY=up_live_... unipost logs list --status error --since 2h --json
UNIPOST_API_KEY=up_live_... unipost doctor verify --json
UNIPOST_API_KEY=up_live_... unipost doctor support-bundle --json
unipost upgrade
unipost self help
unipost completion zsh
```

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
The Agent Debug Kit adds `doctor diagnose`, `doctor explain`, `doctor verify`,
`doctor support-bundle`, and `logs list/get`. `doctor diagnose` returns
`doctor.v1` under the standard JSON envelope and includes local project repair
hints for common auth, payload, media, SDK, and environment mistakes without
reading real `.env` contents.
Publish-capable writes require explicit user approval through `--yes` and a
stable `--idempotency-key`; draft creation and dry-run validation remain safe
without live publishing.
`agent execute` only accepts structured plan actions for read-only, validate, or
draft-write flows from a current `agent plan --json` envelope with a matching
`catalog_version`. It rejects stale plans, live publish actions, pending
confirmations, and raw command strings from a plan.
