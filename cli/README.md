# UniPost CLI

Phase 2 source package for the planned UniPost CLI.

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
UNIPOST_API_KEY=up_live_... unipost posts validate --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... unipost posts draft --account sa_... --caption "Hello" --json
UNIPOST_API_KEY=up_live_... unipost agent bootstrap --client codex --json
UNIPOST_API_KEY=up_live_... unipost agent capabilities --json
UNIPOST_API_KEY=up_live_... unipost examples posts.create --lang node --account sa_...
UNIPOST_API_KEY=up_live_... unipost doctor --json
unipost completion zsh
```

The CLI stores non-secret local defaults such as the selected profile in
`~/.unipost/config.json`. API keys are not written to that file.

Phase 2 intentionally creates drafts and validation results only. Live publish,
scheduled publish, media workflows, analytics, and first-party MCP packaging are
planned in later PRD phases.
