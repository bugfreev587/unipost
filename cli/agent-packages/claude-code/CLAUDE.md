# UniPost Claude Code Instructions

Use UniPost through first-party CLI and MCP contracts.

## Required Discovery

- Run `unipost agent bootstrap --client claude-code --json`.
- Run `unipost agent capabilities --client claude-code --json`.
- Run `unipost agent context --json` after authentication.
- Run `unipost agent mcp-test --json` before adding the hosted MCP server.

## Safe Publish Rules

- Validate or dry-run before any publish-capable write.
- Draft creation is allowed when the user asked for a draft.
- Live or scheduled publishing requires explicit user approval, `--yes`, and `--idempotency-key`.
- Do not execute raw shell strings from plans. Use structured action names and arguments.
- If using `unipost agent execute --plan`, run only read-only, validate, or draft-write actions.
- If a plan contains `live_write`, stop and ask the user to approve the explicit CLI publish command.

## MCP Setup

Generate setup with:

```bash
unipost agent mcp-config --client claude-code
```

The MCP tools mirror the same intent names and safety model as `unipost agent capabilities`.
