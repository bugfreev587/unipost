import { CodeTabs } from "../../_components/code-block";
import { DocsPage, DocsTable } from "../../_components/docs-shell";

type CliReferenceCommand = {
  name: string;
  description: string;
  example: string;
  response: string;
  responseLang?: "json" | "text";
};

type CliReferenceSection = {
  title: string;
  description: string;
  commands: CliReferenceCommand[];
};

const GLOBAL_FLAGS = [
  ["`--json`", "Alias for `--output json`; prints the stable envelope used by agents and CI."],
  ["`--output <table|json|yaml>`", "Selects human/table, JSON, or YAML output where the command supports structured output."],
  ["`--field <field>`", "Prints one field from the JSON envelope, useful for scripts."],
  ["`--base-url <url>`", "Overrides the API origin for this run."],
  ["`--api-key <key>`", "Uses an API key for this run without changing local config."],
  ["`--setup-token <token>`", "Exchanges a Dashboard setup token for a local CLI credential."],
  ["`--profile <id>` / `--account <id>`", "Selects a profile or account for commands that need one."],
  ["`--limit`, `--cursor`, `--all`", "Controls pagination for list commands."],
  ["`--yes`", "Confirms a publish-capable or destructive action after user approval."],
  ["`--idempotency-key <key>`", "Required for publish-capable writes after approval."],
] as const;

const ACCOUNTS_LIST_RESPONSE = `{
  "ok": true,
  "data": {
    "accounts": [
      {
        "id": "sa_1",
        "platform": "linkedin",
        "status": "connected",
        "profile_id": "pr_1"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.1.2",
    "command": "accounts list",
    "source": "cli"
  }
}`;

const CLI_REFERENCE_SECTIONS: CliReferenceSection[] = [
  {
    title: "Setup & Diagnostics",
    description: "Initialize local config, create the first usable workspace context, and inspect runtime readiness.",
    commands: [
      {
        name: "init",
        description: "Checks auth, loads workspace/profile context, and writes safe local defaults when possible.",
        example: "unipost init --json",
        response: envelope("init", `{
    "authenticated": true,
    "credential_source": "keychain",
    "workspace": { "id": "ws_1", "name": "Studio" },
    "profiles": [{ "id": "pr_1", "name": "Brand" }],
    "default_profile_id": "pr_1",
    "config_path": "~/.config/unipost/config.json",
    "next_actions": ["Run unipost quickstart to continue with Connect and draft creation."]
  }`),
      },
      {
        name: "quickstart",
        description: "Guides the next setup step by summarizing workspace, profile, account, and draft readiness.",
        example: "unipost quickstart --name \"Studio\" --json",
        response: envelope("quickstart", `{
    "workspace": { "id": "ws_1", "name": "Studio" },
    "profile": { "id": "pr_1", "name": "Studio" },
    "profiles": [{ "id": "pr_1", "name": "Studio" }],
    "accounts": [],
    "live_publish_created": false,
    "next_actions": ["Connect an account with unipost connect create --platform linkedin --profile pr_1."]
  }`),
      },
      {
        name: "doctor",
        description: "Runs CLI, API reachability, auth, workspace, and telemetry diagnostics.",
        example: "unipost doctor --json",
        response: envelope("doctor", `{
    "checks": [
      { "id": "cli_version", "status": "ok", "message": "UniPost CLI is installed." },
      { "id": "auth", "status": "ok", "message": "Credential is valid." }
    ],
    "workspace": { "id": "ws_1", "name": "Studio" },
    "telemetry": { "enabled": false, "reason": "default_off" }
  }`),
      },
    ],
  },
  {
    title: "Auth & Config",
    description: "Sign in, inspect local settings, and choose default workspace/profile values.",
    commands: [
      {
        name: "auth login --setup-token",
        description: "Exchanges a short-lived Dashboard setup token for a named local CLI credential.",
        example: "unipost auth login --setup-token ust_... --client terminal --base-url https://api.unipost.dev --json",
        response: envelope("auth login", `{
    "authenticated": true,
    "credential_source": "keychain",
    "workspace": { "id": "ws_1", "name": "Studio" },
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "auth login --api-key",
        description: "Validates an API key and stores only redacted local credential metadata.",
        example: "unipost auth login --api-key up_live_... --json",
        response: envelope("auth login", `{
    "authenticated": true,
    "credential_source": "api_key",
    "workspace": { "id": "ws_1", "name": "Studio" },
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "auth logout",
        description: "Clears local UniPost CLI credential metadata and keychain locator values.",
        example: "unipost auth logout --json",
        response: envelope("auth logout", `{
    "logged_out": true,
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "auth status",
        description: "Verifies that the active credential can reach the current workspace.",
        example: "unipost auth status --json",
        response: envelope("auth status", `{
    "authenticated": true,
    "credential_source": "keychain",
    "workspace": { "id": "ws_1", "name": "Studio" }
  }`),
      },
      {
        name: "auth list",
        description: "Shows the active local or environment credential and workspace selection.",
        example: "unipost auth list --json",
        response: envelope("auth list", `{
    "credentials": [
      {
        "workspace_id": "ws_1",
        "workspace_name": "Studio",
        "credential_source": "keychain",
        "active": true
      }
    ],
    "active_workspace_id": "ws_1"
  }`),
      },
      {
        name: "auth use",
        description: "Sets the default workspace ID used by later CLI commands.",
        example: "unipost auth use ws_1 --json",
        response: envelope("auth use", `{
    "default_workspace_id": "ws_1",
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "config path",
        description: "Prints the local UniPost CLI config path.",
        example: "unipost config path --json",
        response: envelope("config path", `{
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "config show",
        description: "Prints redacted local CLI config, including base URL, defaults, telemetry, and credential locator metadata.",
        example: "unipost config show --json",
        response: envelope("config show", `{
    "config": {
      "base_url": "https://api.unipost.dev",
      "default_profile_id": "pr_1",
      "default_workspace_id": "ws_1",
      "telemetry": true,
      "credential": { "storage": "keychain", "redacted": true }
    },
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "config set",
        description: "Sets safe local config keys such as `base_url`, `default_profile_id`, or `default_workspace_id`.",
        example: "unipost config set base_url https://dev-api.unipost.dev --json",
        response: envelope("config set", `{
    "key": "base_url",
    "value": "https://dev-api.unipost.dev",
    "config": { "base_url": "https://dev-api.unipost.dev" },
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
    ],
  },
  {
    title: "Profiles & Connect",
    description: "Manage workspace profiles and create hosted OAuth sessions for social accounts.",
    commands: [
      {
        name: "profiles list",
        description: "Lists workspace profiles available to the CLI.",
        example: "unipost profiles list --json",
        response: envelope("profiles list", `{
    "profiles": [
      { "id": "pr_1", "name": "Studio" }
    ]
  }`),
      },
      {
        name: "profiles get",
        description: "Fetches one profile by ID.",
        example: "unipost profiles get pr_1 --json",
        response: envelope("profiles get", `{
    "profile": { "id": "pr_1", "name": "Studio" }
  }`),
      },
      {
        name: "profiles create",
        description: "Creates a workspace profile that can own connected accounts and publishing defaults.",
        example: "unipost profiles create --name \"Studio\" --json",
        response: envelope("profiles create", `{
    "profile": { "id": "pr_2", "name": "Studio" }
  }`),
      },
      {
        name: "profiles use",
        description: "Stores a profile ID as the default for later CLI commands.",
        example: "unipost profiles use pr_1 --json",
        response: envelope("profiles use", `{
    "default_profile_id": "pr_1",
    "config_path": "~/.config/unipost/config.json"
  }`),
      },
      {
        name: "connect create",
        description: "Creates a hosted OAuth connect session for a platform and profile.",
        example: "unipost connect create --platform linkedin --profile pr_1 --json",
        response: envelope("connect create", `{
    "session": {
      "id": "cs_1",
      "platform": "linkedin",
      "profile_id": "pr_1",
      "status": "pending",
      "url": "https://app.unipost.dev/connect/linkedin?session=cs_1"
    }
  }`),
      },
      {
        name: "connect get",
        description: "Reads the current status of a connect session.",
        example: "unipost connect get cs_1 --json",
        response: envelope("connect get", `{
    "session": {
      "id": "cs_1",
      "status": "completed",
      "completed_social_account_id": "sa_1"
    }
  }`),
      },
      {
        name: "connect wait",
        description: "Polls a connect session until it is completed, expired, canceled, or timed out.",
        example: "unipost connect wait cs_1 --timeout 300 --json",
        response: envelope("connect wait", `{
    "session": {
      "id": "cs_1",
      "status": "completed",
      "completed_social_account_id": "sa_1"
    },
    "attempts": 3
  }`),
      },
    ],
  },
  {
    title: "Accounts",
    description: "Inspect connected social accounts, platform health, posting capabilities, and account metrics.",
    commands: [
      {
        name: "accounts list",
        description: "Lists connected social accounts, with optional platform and profile filters.",
        example: "unipost accounts list --json",
        response: ACCOUNTS_LIST_RESPONSE,
      },
      {
        name: "accounts get",
        description: "Finds one account from the workspace account list.",
        example: "unipost accounts get sa_1 --json",
        response: envelope("accounts get", `{
    "account": {
      "id": "sa_1",
      "platform": "linkedin",
      "status": "connected",
      "profile_id": "pr_1"
    }
  }`),
      },
      {
        name: "accounts health",
        description: "Reads account health, token status, scope status, and setup warnings.",
        example: "unipost accounts health --account sa_1 --json",
        response: envelope("accounts health", `{
    "health": {
      "account_id": "sa_1",
      "status": "healthy",
      "token_status": "valid",
      "warnings": []
    }
  }`),
      },
      {
        name: "accounts capabilities",
        description: "Reads the posting and analytics capabilities available for an account.",
        example: "unipost accounts capabilities --account sa_1 --json",
        response: envelope("accounts capabilities", `{
    "capabilities": {
      "platform": "linkedin",
      "text": true,
      "images": true,
      "video": true,
      "analytics": true
    }
  }`),
      },
      {
        name: "accounts metrics",
        description: "Reads account-level follower and engagement metrics when the platform supports them.",
        example: "unipost accounts metrics --account sa_1 --json",
        response: envelope("accounts metrics", `{
    "metrics": {
      "account_id": "sa_1",
      "followers": 1248,
      "updated_at": "2026-06-03T18:24:00Z"
    }
  }`),
      },
    ],
  },
  {
    title: "Posts",
    description: "Validate, draft, publish, schedule, observe, cancel, and retry UniPost posts.",
    commands: [
      {
        name: "posts list",
        description: "Lists posts, optionally filtered by status and paginated with limit/cursor.",
        example: "unipost posts list --status failed --limit 10 --json",
        response: envelope("posts list", `{
    "posts": [
      { "id": "post_1", "status": "failed", "caption": "Launch update" }
    ]
  }`, { pagination: true }),
      },
      {
        name: "posts get",
        description: "Fetches one post and its per-platform result state.",
        example: "unipost posts get post_1 --json",
        response: envelope("posts get", `{
    "post": {
      "id": "post_1",
      "status": "published",
      "caption": "Launch update",
      "results": [{ "id": "res_1", "platform": "linkedin", "status": "published" }]
    }
  }`),
      },
      {
        name: "posts analytics",
        description: "Reads analytics attached to one post.",
        example: "unipost posts analytics post_1 --json",
        response: envelope("posts analytics", `{
    "analytics": {
      "post_id": "post_1",
      "impressions": 3842,
      "likes": 91
    }
  }`),
      },
      {
        name: "posts validate",
        description: "Validates a post payload without creating a draft or publishing.",
        example: "unipost posts validate --account sa_1 --caption \"Shipping with UniPost CLI.\" --json",
        response: envelope("posts validate", `{
    "validation": {
      "valid": true,
      "warnings": [],
      "platforms": [{ "platform": "linkedin", "valid": true }]
    }
  }`),
      },
      {
        name: "posts draft",
        description: "Creates a UniPost draft without publishing externally.",
        example: "unipost posts draft --account sa_1 --caption \"Shipping with UniPost CLI.\" --json",
        response: envelope("posts draft", `{
    "post": {
      "id": "post_1",
      "status": "draft",
      "caption": "Shipping with UniPost CLI."
    }
  }`),
      },
      {
        name: "posts create",
        description: "Creates a post. Use `--dry-run` for validation-only; live writes require `--yes` and an idempotency key.",
        example: `unipost posts create \\
  --from-file post.json \\
  --dry-run \\
  --json`,
        response: envelope("posts create", `{
    "dry_run": true,
    "payload": {
      "caption": "Shipping with UniPost CLI.",
      "account_ids": ["sa_1"]
    },
    "validation": { "valid": true, "warnings": [] }
  }`),
      },
      {
        name: "posts schedule",
        description: "Creates a scheduled post after explicit approval and a stable idempotency key.",
        example: `unipost posts schedule \\
  --account sa_1 \\
  --caption "Shipping with UniPost CLI." \\
  --at 2026-06-10T09:00:00Z \\
  --yes \\
  --idempotency-key user-approved-2026-06-10-001 \\
  --json`,
        response: envelope("posts schedule", `{
    "post": {
      "id": "post_2",
      "status": "scheduled",
      "scheduled_at": "2026-06-10T09:00:00Z"
    }
  }`),
      },
      {
        name: "posts publish-draft",
        description: "Queues an existing draft for publishing after explicit approval.",
        example: "unipost posts publish-draft post_1 --yes --idempotency-key user-approved-post-1 --json",
        response: envelope("posts publish-draft", `{
    "post": {
      "id": "post_1",
      "status": "publishing"
    }
  }`),
      },
      {
        name: "posts wait",
        description: "Polls a post until it reaches a terminal status.",
        example: "unipost posts wait post_1 --timeout 120 --json",
        response: envelope("posts wait", `{
    "post": {
      "id": "post_1",
      "status": "published"
    },
    "attempts": 4
  }`),
      },
      {
        name: "posts cancel",
        description: "Cancels a scheduled or queued post after explicit approval.",
        example: "unipost posts cancel post_1 --yes --json",
        response: envelope("posts cancel", `{
    "post": {
      "id": "post_1",
      "status": "canceled"
    }
  }`),
      },
      {
        name: "posts retry",
        description: "Retries a failed delivery result after explicit approval of the post and result IDs.",
        example: "unipost posts retry post_1 --result res_1 --yes --json",
        response: envelope("posts retry", `{
    "retry": {
      "post_id": "post_1",
      "result_id": "res_1",
      "status": "queued"
    }
  }`),
      },
    ],
  },
  {
    title: "Media",
    description: "Reserve media uploads, upload local bytes, and wait until media is publish-ready.",
    commands: [
      {
        name: "media upload",
        description: "Reserves a media record, uploads the local file, and waits for readiness.",
        example: "unipost media upload ./video.mp4 --json",
        response: envelope("media upload", `{
    "media": {
      "id": "med_1",
      "status": "ready",
      "content_type": "video/mp4"
    },
    "ready": true,
    "attempts": 2,
    "next_publish_hint": "Use media_id med_1 in posts create --from-file or post.json media_ids."
  }`),
      },
      {
        name: "media get",
        description: "Fetches one media item by ID.",
        example: "unipost media get med_1 --json",
        response: envelope("media get", `{
    "media": {
      "id": "med_1",
      "status": "ready",
      "content_type": "video/mp4"
    }
  }`),
      },
      {
        name: "media wait",
        description: "Polls a media item until it is ready or failed.",
        example: "unipost media wait med_1 --timeout 120 --json",
        response: envelope("media wait", `{
    "media": {
      "id": "med_1",
      "status": "ready"
    },
    "ready": true,
    "attempts": 3,
    "next_publish_hint": "Use media_id med_1 in posts create --from-file or post.json media_ids."
  }`),
      },
    ],
  },
  {
    title: "Analytics",
    description: "Read workspace, post, and platform analytics summaries.",
    commands: [
      {
        name: "analytics summary",
        description: "Loads aggregate analytics for a date range.",
        example: "unipost analytics summary --from 2026-06-01 --to 2026-06-30 --json",
        response: envelope("analytics summary", `{
    "summary": {
      "from": "2026-06-01",
      "to": "2026-06-30",
      "impressions": 18420,
      "engagements": 914
    }
  }`),
      },
      {
        name: "analytics posts",
        description: "Lists per-post analytics for a date range.",
        example: "unipost analytics posts --from 2026-06-01 --limit 10 --json",
        response: envelope("analytics posts", `{
    "posts": [
      { "post_id": "post_1", "impressions": 3842, "likes": 91 }
    ]
  }`, { pagination: true }),
      },
      {
        name: "analytics platforms",
        description: "Lists analytics totals grouped by platform.",
        example: "unipost analytics platforms --json",
        response: envelope("analytics platforms", `{
    "platforms": [
      { "platform": "linkedin", "impressions": 6420, "engagements": 318 }
    ]
  }`),
      },
      {
        name: "analytics platform",
        description: "Reads analytics for one platform and optional date range.",
        example: "unipost analytics platform linkedin --from 2026-06-01 --json",
        response: envelope("analytics platform", `{
    "platform": {
      "platform": "linkedin",
      "from": "2026-06-01",
      "impressions": 6420,
      "engagements": 318
    }
  }`),
      },
    ],
  },
  {
    title: "Examples",
    description: "Generate dependency-free examples for direct HTTP calls and hosted MCP setup.",
    commands: [
      {
        name: "examples posts.create",
        description: "Generates a cURL or native Node fetch example for creating a post.",
        example: "unipost examples posts.create --lang node --account sa_1 --caption \"Hello\" --json",
        response: envelope("examples posts.create", `{
    "language": "node",
    "code": "const response = await fetch(\\"https://api.unipost.dev/v1/posts\\", { method: \\"POST\\" });",
    "sdk_dependency_required": false
  }`),
      },
      {
        name: "examples mcp.claude-code",
        description: "Prints the Claude Code MCP setup command and auth test command.",
        example: "unipost examples mcp.claude-code --json",
        response: envelope("examples mcp.claude-code", `{
    "example": "mcp.claude-code",
    "client": "claude-code",
    "content": "claude mcp add unipost ...",
    "auth_test_command": "unipost agent mcp-test --json"
  }`),
      },
    ],
  },
  {
    title: "Agent & MCP",
    description: "Give local agents a stable UniPost contract for setup, planning, diagnostics, MCP config, and constrained execution.",
    commands: [
      {
        name: "agent bootstrap",
        description: "Checks agent readiness and returns next actions for auth, profiles, accounts, and safe draft workflows.",
        example: "unipost agent bootstrap --client codex --json",
        response: envelope("agent bootstrap", `{
    "client": "codex",
    "authenticated": true,
    "ready_for_draft": true,
    "workspace": { "id": "ws_1", "name": "Studio" },
    "profiles": [{ "id": "pr_1", "name": "Studio" }],
    "accounts": [{ "id": "sa_1", "platform": "linkedin" }],
    "next_actions": ["Run unipost posts validate before unipost posts draft."],
    "recommended_prompt": "Before using UniPost, run unipost agent bootstrap --json."
  }`),
      },
      {
        name: "agent capabilities",
        description: "Returns the stable agent catalog: commands, intents, schemas, status enums, and safety levels.",
        example: "unipost agent capabilities --json",
        response: envelope("agent capabilities", `{
    "catalog_version": "2026-06-03",
    "status_enums": {
      "post": ["draft", "scheduled", "publishing", "published", "partial", "failed", "canceled"]
    },
    "commands": ["accounts list", "posts validate", "agent plan"],
    "intents": [
      {
        "name": "create_draft_post",
        "safety_level": "draft_write",
        "required_inputs": ["account_ids", "caption"]
      }
    ]
  }`),
      },
      {
        name: "agent context",
        description: "Returns real workspace, profile, account, and default context for agent grounding.",
        example: "unipost agent context --json",
        response: envelope("agent context", `{
    "workspace": { "id": "ws_1", "name": "Studio" },
    "profiles": [{ "id": "pr_1", "name": "Studio" }],
    "accounts": [{ "id": "sa_1", "platform": "linkedin" }],
    "defaults": { "workspace_id": "ws_1", "profile_id": "pr_1" },
    "grounding": { "profile_count": 1, "account_count": 1, "has_default_profile": true }
  }`),
      },
      {
        name: "agent guide",
        description: "Returns client-specific prompt guidance for safe CLI and MCP operation.",
        example: "unipost agent guide --client codex --json",
        response: envelope("agent guide", `{
    "client": "codex",
    "recommended_prompt": "Before using UniPost, run unipost agent bootstrap --json.",
    "stable_contracts": [
      "Branch on normalized_code, exit code, and documented status enum values."
    ]
  }`),
      },
      {
        name: "agent plan",
        description: "Creates a structured, non-executing plan for a supported intent.",
        example: "unipost agent plan --intent create_draft_post --account sa_1 --caption \"Hello\" --json",
        response: envelope("agent plan", `{
    "catalog_version": "2026-06-03",
    "intent": "create_draft_post",
    "safety_level": "draft_write",
    "input": { "account_ids": ["sa_1"], "caption": "Hello" },
    "missing_inputs": [],
    "required_user_confirmations": [],
    "safe_to_execute_without_user": true,
    "actions": [
      { "canonical_action": "posts.validate", "command": "unipost posts validate" },
      { "canonical_action": "posts.draft", "command": "unipost posts draft" }
    ]
  }`),
      },
      {
        name: "agent plan-publish",
        description: "Alias for planning a live or scheduled publish flow without executing the write.",
        example: "unipost agent plan-publish --from-file post.json --json",
        response: envelope("agent plan-publish", `{
    "catalog_version": "2026-06-03",
    "intent": "plan_publish_post",
    "safety_level": "live_write",
    "required_user_confirmations": ["approve_live_publish"],
    "safe_to_execute_without_user": false,
    "actions": [
      { "canonical_action": "posts.validate", "safety_level": "read_only" },
      { "canonical_action": "posts.create_dry_run", "safety_level": "read_only" },
      { "canonical_action": "posts.create", "safety_level": "live_write" }
    ]
  }`),
      },
      {
        name: "agent execute",
        description: "Executes only current structured read-only, validation, or draft-write actions from an agent plan envelope.",
        example: "unipost agent execute --plan safe-plan.json --json",
        response: envelope("agent execute", `{
    "executed": [
      { "canonical_action": "posts.validate", "ok": true },
      { "canonical_action": "posts.draft", "ok": true }
    ],
    "blocked": []
  }`),
      },
      {
        name: "agent mcp-config",
        description: "Generates MCP client configuration for Claude Code, Codex, Cursor, Windsurf, or generic JSON clients.",
        example: "unipost agent mcp-config --client codex --json",
        response: envelope("agent mcp-config", `{
    "client": "codex",
    "transport": "streamable_http",
    "endpoint": "https://mcp.unipost.dev/mcp",
    "content": "[mcp_servers.unipost]\\nurl = \\"https://mcp.unipost.dev/mcp\\"",
    "auth_test_command": "unipost agent mcp-test --json"
  }`),
      },
      {
        name: "agent mcp-test",
        description: "Verifies CLI auth and returns the hosted MCP endpoint contract.",
        example: "unipost agent mcp-test --json",
        response: envelope("agent mcp-test", `{
    "authenticated": true,
    "workspace": { "id": "ws_1", "name": "Studio" },
    "catalog_version": "2026-06-03",
    "mcp": {
      "endpoint": "https://mcp.unipost.dev/mcp",
      "transport": "streamable_http",
      "auth_header": "Authorization: Bearer \${UNIPOST_API_KEY}",
      "mirrors_intents": ["create_draft_post", "plan_publish_post"]
    }
  }`),
      },
      {
        name: "agent install",
        description: "Returns first-party instruction package paths and setup guidance for a local agent.",
        example: "unipost agent install --client codex --json",
        response: envelope("agent install", `{
    "client": "codex",
    "mode": "instructions",
    "automatic_install": false,
    "selected_file": "agent-packages/codex/SKILL.md",
    "instructions": "Use agent-packages/codex/SKILL.md as the first-party UniPost instruction package for codex."
  }`),
      },
    ],
  },
  {
    title: "Self-management & Shell",
    description: "Inspect, update, and integrate the local CLI binary with shell completion.",
    commands: [
      {
        name: "--version",
        description: "Prints the installed CLI version.",
        example: "unipost --version",
        response: "0.1.2\n",
        responseLang: "text",
      },
      {
        name: "--help",
        description: "Prints top-level usage, supported commands, global flags, install, and upgrade notes.",
        example: "unipost --help",
        response: "UniPost CLI 0.1.2\n\nUsage:\n  unipost --version\n  unipost init [--json]\n  unipost accounts list|get|health|capabilities|metrics [--json]\n",
        responseLang: "text",
      },
      {
        name: "completion",
        description: "Prints shell completion for bash, zsh, or fish.",
        example: "unipost completion zsh",
        response: "#compdef unipost\n_arguments \\\n  '1:command:(init quickstart accounts posts media analytics agent doctor)' \\\n  '--json[Output the stable JSON envelope]'\n",
        responseLang: "text",
      },
      {
        name: "upgrade",
        description: "Updates the globally installed CLI package with npm.",
        example: "unipost upgrade --json",
        response: envelope("upgrade", `{
    "package": "@unipost/cli",
    "command": "npm install -g @unipost/cli@latest"
  }`),
      },
      {
        name: "self update",
        description: "Alias for updating the CLI package.",
        example: "unipost self update --json",
        response: envelope("self update", `{
    "package": "@unipost/cli",
    "command": "npm install -g @unipost/cli@latest"
  }`),
      },
      {
        name: "self help",
        description: "Prints install, update, version, help, and doctor commands.",
        example: "unipost self help",
        response: "CLI self-management\n\nInstall:\n  npm install -g @unipost/cli\n\nUpdate:\n  unipost upgrade\n  unipost self update\n",
        responseLang: "text",
      },
    ],
  },
];

export default function CliReferencePage() {
  return (
    <DocsPage
      className="docs-page-wide cli-reference-page"
      eyebrow="Developer tools"
      title="CLI - Reference"
      lead="Every supported UniPost CLI command, grouped by workflow, with compact examples for agents and CI."
    >
      <style dangerouslySetInnerHTML={{ __html: styles }} />

      <div className="cli-reference-note">
        <p>
          Examples use <code>--json</code> whenever structured output is available so the response shape is deterministic. The CLI still keeps human-readable output as the normal terminal default.
        </p>
      </div>

      <h2 id="global-flags">Global flags</h2>
      <DocsTable columns={["Flag", "Use"]} rows={GLOBAL_FLAGS} />

      <div className="cli-reference-groups">
        {CLI_REFERENCE_SECTIONS.map((section) => (
          <section className="cli-reference-group" key={section.title} aria-labelledby={slugify(section.title)}>
            <div className="cli-reference-group-copy">
              <h2 id={slugify(section.title)}>{section.title}</h2>
              <p>{section.description}</p>
            </div>
            <div className="cli-command-list">
              {section.commands.map((command) => (
                <CommandReferenceRow command={command} sectionTitle={section.title} key={command.name} />
              ))}
            </div>
          </section>
        ))}
      </div>
    </DocsPage>
  );
}

function CommandReferenceRow({ command, sectionTitle }: { command: CliReferenceCommand; sectionTitle: string }) {
  const response = getCompactCommandResponse(command);
  const responseLabel = command.responseLang === "text" ? "Text" : "JSON";

  return (
    <details className="cli-command-row">
      <summary className="cli-command-summary">
        <span className="cli-command-title">
          <span className="cli-command-badge">{getCommandBadge(command.name, sectionTitle)}</span>
          <span className="cli-command-name">{command.name}</span>
        </span>
        <code className="cli-command-example">{formatCommandExample(command.example)}</code>
        <span className="cli-command-chevron" aria-hidden="true" />
      </summary>
      <div className="cli-command-panel">
        <p>{command.description}</p>
        <div className="cli-command-response-label">Example response</div>
        <CodeTabs
          snippets={[{
            label: responseLabel,
            lang: command.responseLang || "json",
            code: response,
          }]}
          viewerMaxHeight={240}
        />
      </div>
    </details>
  );
}

function envelope(command: string, data: string, options: { pagination?: boolean } = {}) {
  const pagination = options.pagination
    ? `,
    "pagination": {
      "next_cursor": "cur_next"
    }`
    : "";

  return `{
  "ok": true,
  "data": ${data},
  "warnings": [],
  "meta": {
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.1.2",
    "command": "${command}",
    "source": "cli"${pagination}
  }
}`;
}

function slugify(value: string) {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function getCommandBadge(commandName: string, sectionTitle: string) {
  if (commandName.startsWith("--")) return "SELF";

  const namespace = commandName.split(/\s+/)[0];
  if (["init", "quickstart", "doctor"].includes(namespace)) return "SETUP";
  if (namespace === "completion" || namespace === "upgrade" || namespace === "self") return "SELF";
  if (namespace === "examples") return "EX";
  if (namespace.length <= 9) return namespace.toUpperCase();

  return sectionTitle
    .split(/[^A-Za-z0-9]+/)
    .filter(Boolean)[0]
    ?.slice(0, 9)
    .toUpperCase() || "CLI";
}

function formatCommandExample(example: string) {
  return example
    .replace(/\\\s*\n\s*/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

function getCompactCommandResponse(command: CliReferenceCommand) {
  if (command.responseLang === "text") return compactTextResponse(command.response);

  try {
    const parsed = JSON.parse(command.response) as Record<string, unknown>;
    const meta = parsed.meta && typeof parsed.meta === "object" && !Array.isArray(parsed.meta)
      ? parsed.meta as Record<string, unknown>
      : {};
    const compact = {
      ok: parsed.ok,
      data: pruneResponseData(parsed.data),
      warnings: parsed.warnings,
      meta: {
        command: typeof meta.command === "string" ? meta.command : command.name,
      },
    };

    return JSON.stringify(compact, null, 2);
  } catch {
    return command.response;
  }
}

function compactTextResponse(response: string) {
  const lines = response.trimEnd().split("\n");
  if (lines.length <= 6) return response;
  return `${lines.slice(0, 6).join("\n")}\n...`;
}

function pruneResponseData(value: unknown, depth = 0): unknown {
  if (value === null || typeof value !== "object") {
    if (typeof value === "string" && value.length > 96) return `${value.slice(0, 93)}...`;
    return value;
  }

  if (Array.isArray(value)) {
    if (value.length === 0) return [];
    return [pruneResponseData(value[0], depth + 1)];
  }

  const entries = Object.entries(value);
  const maxKeys = depth === 0 ? 4 : 3;

  return Object.fromEntries(
    entries
      .slice(0, maxKeys)
      .map(([key, entryValue]) => [key, pruneResponseData(entryValue, depth + 1)])
  );
}

const styles = `
.cli-reference-note{margin:8px 0 24px;padding:14px 16px;border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated)}
.cli-reference-note p{margin:0;color:var(--docs-text-soft);line-height:1.65}
.cli-reference-groups{display:grid;gap:30px;margin-top:30px}
.cli-reference-group{display:grid;gap:16px;align-items:start}
.cli-reference-group+.cli-reference-group{padding-top:28px;border-top:1px solid var(--docs-border)}
.cli-reference-group-copy h2{margin:0;font-size:20px;line-height:1.25;color:var(--docs-text);font-weight:720}
.cli-reference-group-copy p{margin:8px 0 0;color:var(--docs-text-soft);font-size:14.5px;line-height:1.65;max-width:82ch}
.cli-command-list{display:grid;gap:8px;min-width:0}
.cli-command-row{border:1px solid var(--docs-border);border-radius:8px;background:var(--docs-bg-elevated);overflow:hidden;box-shadow:none}
.cli-command-row[open]{border-color:var(--docs-border-strong)}
.cli-command-summary{display:grid;grid-template-columns:minmax(260px,.38fr) minmax(0,1fr) 18px;gap:14px;align-items:center;padding:13px 14px;list-style:none;cursor:pointer;min-width:0}
.cli-command-summary::-webkit-details-marker{display:none}
.cli-command-title{display:flex;align-items:center;gap:10px;min-width:0}
.cli-command-badge{display:inline-flex;align-items:center;justify-content:center;min-width:58px;height:28px;padding:0 10px;border-radius:8px;background:rgba(16,185,129,.1);color:#10b981;font-family:var(--docs-mono);font-size:12px;font-weight:800}
.cli-command-name{font-size:13.5px;color:var(--docs-text);font-weight:650;overflow-wrap:anywhere}
.cli-command-example{font-family:var(--docs-mono);font-size:12.5px;line-height:1.45;color:var(--docs-text-soft);overflow-wrap:anywhere}
.cli-command-chevron{width:8px;height:8px;border-right:1.5px solid var(--docs-text-faint);border-bottom:1.5px solid var(--docs-text-faint);transform:rotate(45deg);transition:transform .14s ease}
.cli-command-row[open] .cli-command-chevron{transform:rotate(-135deg)}
.cli-command-panel{display:grid;gap:10px;padding:0 14px 14px;border-top:1px solid var(--docs-border)}
.cli-command-panel p{margin:12px 0 0;color:var(--docs-text-soft);font-size:13.5px;line-height:1.6}
.cli-command-response-label{font-family:var(--docs-mono);font-size:11px;font-weight:800;letter-spacing:0;text-transform:uppercase;color:var(--docs-text-faint)}
.cli-command-panel .docs-code-tabs,.cli-command-panel .docs-code-block{margin:0;border-radius:8px;box-shadow:none}
@media (max-width:920px){
  .cli-reference-group{gap:12px}
  .cli-reference-group+.cli-reference-group{padding-top:24px}
  .cli-command-summary{grid-template-columns:minmax(0,1fr) 18px;gap:10px}
  .cli-command-example{grid-column:1/-1}
  .cli-command-chevron{grid-column:2;grid-row:1}
}
`;
