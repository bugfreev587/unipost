# UniPost CLI

Phase 5 beta source package for the planned UniPost CLI.

The current implementation supports API-key fallback through `UNIPOST_API_KEY`.
Browser/device auth and Dashboard setup-token exchange are still backend
dependencies; until those endpoints exist, init/bootstrap diagnose setup and
reuse the API key already available in the environment.

```bash
UNIPOST_API_KEY=up_live_... unipost init --json
UNIPOST_API_KEY=up_live_... unipost quickstart --json
UNIPOST_API_KEY=up_live_... unipost auth status --json
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
unipost completion zsh
```

The CLI stores non-secret local defaults such as the selected profile in
`~/.unipost/config.json`. API keys are not written to that file.

Phase 5 supports agent planning, dry-run publish validation, scheduled publish,
post lifecycle waits, cancel/retry operations, account diagnostics, local media
upload/readiness waits, analytics reads, MCP client setup generation, MCP auth
testing, Codex/Claude Code instruction packages, and a limited structured
`agent execute` beta.
Publish-capable writes require explicit user approval through `--yes` and a
stable `--idempotency-key`; draft creation and dry-run validation remain safe
without live publishing.
`agent execute` only accepts structured plan actions for read-only, validate, or
draft-write flows. It rejects live publish actions and never executes raw command
strings from a plan.
