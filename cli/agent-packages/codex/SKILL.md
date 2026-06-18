---
name: unipost-agent
description: Use when integrating UniPost API, CLI, or MCP into a project, or when operating UniPost accounts, posts, media, analytics, and safe publish workflows through Codex.
---

# UniPost Agent Skill

Use the UniPost CLI as the source of truth. Do not infer command syntax from docs prose when the CLI can expose a structured contract.

## Startup

1. Run `unipost auth status --json`. If auth is `missing` or `metadata_only`,
   ask the user to run `unipost init` or the Dashboard-generated setup command
   before continuing.
2. Run `unipost agent bootstrap --client codex --json`.
3. Run `unipost agent capabilities --client codex --json`.
4. Run `unipost agent context --json` when authenticated.
5. Use `unipost agent mcp-test --json` before configuring MCP.

## Safety

- Prefer `posts validate`, `posts draft`, and `posts create --dry-run`.
- Do not live-publish from an `agent plan`.
- For live or scheduled publish, stop for explicit user approval and use the normal publish command with `--yes` and `--idempotency-key`.
- Treat `display_command` as human-readable only; use structured `canonical_action` and `args`.
- If using `unipost agent execute --plan`, run only read-only, validate, or draft-write actions; reject `live_write` plans.
- Never invent account IDs, profile IDs, media IDs, post IDs, or result IDs.

## MCP

Use `unipost agent mcp-config --client codex` for config and `unipost examples mcp.claude-code` for Claude Code setup examples.
