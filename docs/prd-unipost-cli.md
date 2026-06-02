# UniPost CLI PRD
**开发者 Quickstart CLI + AI Agent Operator CLI**
Status: Planning
Owner: Developer Experience / Platform API
Created: 2026-06-02

---

## 1. Background

UniPost 已经有 REST API、SDK、MCP、Dashboard 和公开文档，但用户第一次接入时仍然需要在 docs、Dashboard、terminal、API examples 之间来回切换。这个过程对人类开发者和 AI coding agent 都不够顺滑：

- 开发者需要先理解 API key、profile、connected account、connect session、post validation、media upload、analytics 等概念，才能发出第一条有效请求。
- AI agent 需要结构化访问用户账号内的数据，但如果只能检索 UniPost docs 或网页，就容易读到过时信息、误解真实账号状态，或在缺少 guardrail 的情况下直接构造发布请求。
- Support / Ops 场景需要快速诊断 API key、workspace、accounts、post delivery、rate limits、webhooks 等状态，目前主要依赖 Dashboard 或手写 cURL。

UniPost CLI 的目标是把 UniPost 的主路径变成可执行、可验证、可复制的终端工作流。它不是替代 docs、SDK 或 MCP，而是把它们连接起来：

- 对开发者：不必频繁检索 docs，也能快速完成 API 接入。
- 对 AI agent：不必读网页，也能用稳定命令读取真实账号、帖子、媒体和 analytics 状态。
- 对团队内部：提供一个统一的调试、演示和 smoke-test 工具。

---

## 2. Product Positioning

UniPost CLI 是 UniPost 的 first-party command line interface，覆盖两条产品线：

1. **Developer Quickstart CLI**
   - 面向人类开发者。
   - 帮用户完成 API onboarding、账号连接、post validation、draft creation、示例生成和诊断。
   - 核心目标是减少文档查找和手写 API 请求。

2. **AI Agent Operator CLI**
   - 面向 Codex、Claude Code、Cursor agent、CI agent 和自动化脚本。
   - 提供稳定 JSON 输出、非交互模式、安全发布 guardrails、agent context 和可审计 metadata。
   - 核心目标是让 agent 可靠、安全地操作用户授权范围内的 UniPost 数据。

同一个 `unipost` binary 同时支持这两条路径。开发者体验通过 human-readable 输出和交互式 wizard 完成；agent 体验通过 `--json`、`--non-interactive`、明确 exit code 和稳定 schema 完成。

---

## 3. Product Goals

1. 让新用户在 10 分钟内完成 CLI 安装、API key 验证、workspace/profile 识别、账号连接引导和第一次 post validation。
2. 让新用户在 15 分钟内创建第一条 draft post，或生成一份可以复制到自己项目中的 SDK/cURL publish 示例。
3. 让 AI agent 能通过 CLI 读取 accounts、posts、media、analytics、workspace context，并在安全限制下执行 dry-run 或 publish。
4. 为所有命令提供稳定的 `--json` 输出、明确 exit code、错误 code、human hint、request id 和 docs link。
5. 让 CLI 成为 UniPost docs、SDK、MCP 的终端入口：能生成 examples、测试 API auth、生成 MCP config、引导用户打开 Dashboard。
6. 将真实发布行为默认设计为安全操作：先 validate / dry-run / draft，再 explicit publish。
7. 让 Support / Ops 可以用 `doctor` 和诊断命令快速定位常见集成问题。

---

## 4. Non-goals

- v1 不做完整 Dashboard 替代品。
- v1 不覆盖所有 REST endpoints 的 CRUD wrapper。
- v1 不默认允许 agent 静默发布真实社交平台内容。
- v1 不让前端或 agent 直接接收 UniPost 内部 secrets、Unleash token、平台 OAuth tokens。
- v1 不把 MCP 替换成 CLI。MCP 仍然是 agent-native 协议；CLI 是终端和脚本入口。
- v1 不强制用户使用 CLI。REST API、SDK、Dashboard 和 MCP 都继续独立可用。
- v1 不承诺所有平台的所有高级 publish options 都通过 flags 暴露；复杂请求可以通过 `--from-file` 输入 JSON。
- v1 不实现长期 daemon、background scheduler 或本地 queue。调度和 delivery 仍由 UniPost API 负责。

---

## 5. Personas And Jobs To Be Done

### 5.1 Developer Integrator

用户：正在把 UniPost 接入自己的 SaaS、workflow、CMS、AI agent 或 internal tool 的开发者。

Jobs:

- 验证 API key 是否可用。
- 快速知道自己的 workspace、profiles 和 connected accounts。
- 创建或选择 profile。
- 生成 OAuth connect URL，把社交账号连进 UniPost。
- 找到 `account_id` 并发起第一条 validate / draft / publish 请求。
- 生成 Node.js、Python、Go、Java 或 cURL 示例。
- 遇到错误时知道下一步，而不是只得到 raw API error。

### 5.2 AI Coding Agent

用户：Codex、Claude Code、Cursor、Windsurf 等 coding agent。

Jobs:

- 获取当前 workspace 的结构化 context。
- 列出可用 social accounts，选择正确平台和 profile。
- 读取最近 posts、失败 posts、post delivery results 和 analytics。
- 上传或检查 media。
- 在明确授权后创建 draft 或 publish。
- 将所有输出作为 JSON 继续推理，不依赖解析 human text。

### 5.3 Ops / Support Engineer

用户：UniPost 内部或客户侧负责诊断集成问题的人。

Jobs:

- 快速检查 API key、workspace、rate limit、account health、connect session、post delivery。
- 复现用户 API 调用。
- 导出最小诊断信息给 support，不泄露 secrets。
- 用 request id 关联 API logs。

### 5.4 CI Automation

用户：需要在 CI/CD 中验证 UniPost 集成的工程团队。

Jobs:

- 在部署前验证 API key 和 workspace 可用。
- 检查至少一个目标 account 存在且健康。
- 执行 validate-only 或 draft-only smoke test。
- 用 exit code 判断通过/失败。

---

## 6. Product Principles

1. **Intent-first, endpoint-second**
   - 命令按用户任务命名，而不是机械映射 REST endpoint。
   - 例如 `unipost doctor` 比 `GET /v1/workspace` 更符合 onboarding 目标。

2. **Human-readable by default, machine-readable on demand**
   - 默认输出适合人读。
   - `--json` 输出稳定 schema，供 agent 和 CI 使用。

3. **Safe writes**
   - Read commands 默认安全。
   - Draft / validate 优先于 live publish。
   - Non-interactive live publish 必须显式 `--yes`，并推荐或要求 `--idempotency-key`。

4. **No hidden docs dependency**
   - CLI 应该能告诉用户下一步做什么。
   - 每个常见错误都提供 `hint` 和 `docs_url`。

5. **Real account grounding**
   - CLI 和 agent 不应该只依赖示例 ID。
   - Quickstart 和 agent context 应尽量基于用户真实 workspace/profile/account 返回建议。

6. **Stable contracts for agents**
   - `--json` schema、exit codes、command names 需要谨慎演进。
   - Breaking changes 必须通过 major version 或 compatibility mode 处理。

7. **Redaction and auditability**
   - 不输出完整 API key、OAuth tokens、presigned private URLs 中的敏感 query，除非命令明确是创建 secret 的一次性结果。
   - 写操作应带 `source=cli`、`cli_version`、可选 `agent_name`、`idempotency_key` 等 metadata。

---

## 7. Product Surface Overview

### 7.1 Shared CLI Foundation

所有命令共享以下基础能力：

- API client
- config management
- auth resolution
- output formatting
- JSON envelope
- error normalization
- request id display
- safe logging
- telemetry hooks
- version checks
- base URL override
- environment selection

Recommended install:

```bash
npm install -g @unipost/cli
```

Recommended package/binary:

```bash
unipost
```

Optional future distribution:

```bash
brew install unipost-dev/tap/unipost
```

### 7.2 Developer Quickstart CLI

Developer Quickstart CLI 是默认用户看到的产品路径，重点是“第一次顺利接入 UniPost API”。

Representative commands:

```bash
unipost init
unipost doctor
unipost quickstart
unipost profiles list
unipost profiles create --name "Quickstart"
unipost connect create --platform linkedin --profile pr_...
unipost accounts list
unipost posts validate --account sa_... --caption "Hello"
unipost posts draft --account sa_... --caption "Hello"
unipost examples posts.create --lang node
```

### 7.3 AI Agent Operator CLI

AI Agent Operator CLI 是同一个 binary 的 agent-friendly 使用方式，重点是“结构化读取 + 安全操作”。

Representative commands:

```bash
unipost agent context --json
unipost accounts list --json --non-interactive
unipost accounts health --account sa_... --json
unipost posts list --status failed --json
unipost posts get post_... --json
unipost media upload ./video.mp4 --json
unipost analytics summary --from 2026-06-01 --to 2026-06-30 --json
unipost posts create --from-file post.json --dry-run --json
unipost posts create --from-file post.json --yes --idempotency-key user-action-...
```

---

## 8. Authentication And Configuration

### 8.1 Auth Sources

CLI resolves credentials in this order:

1. `--api-key` flag
2. `UNIPOST_API_KEY` environment variable
3. local CLI config
4. OS keychain entry, if implemented

Recommended v1 default:

- Prefer `UNIPOST_API_KEY` for developer clarity and CI compatibility.
- Allow `unipost auth login --api-key ...` to store a redacted local config or keychain value.
- Never print the full API key after storing it.

### 8.2 Base URL

Default:

```text
https://api.unipost.dev
```

Supported override:

```bash
unipost doctor --base-url https://dev-api.unipost.dev
UNIPOST_BASE_URL=https://dev-api.unipost.dev unipost accounts list
```

Local development:

```bash
UNIPOST_BASE_URL=http://localhost:8080 unipost doctor
```

### 8.3 Config File

Recommended path:

```text
~/.config/unipost/config.json
```

Example:

```json
{
  "base_url": "https://api.unipost.dev",
  "default_output": "human",
  "default_profile_id": "pr_...",
  "last_workspace_id": "ws_...",
  "telemetry": true
}
```

The config file must not contain unredacted API keys unless the user explicitly chooses file-based storage. Keychain storage is preferred where available.

---

## 9. Command Taxonomy

### 9.1 Global Flags

```bash
--json
--non-interactive
--yes
--quiet
--verbose
--base-url <url>
--api-key <key>
--profile <profile_id>
--account <account_id>
--idempotency-key <key>
--agent-name <name>
--dry-run
--open
```

Behavior:

- `--json`: output stable JSON envelope.
- `--non-interactive`: never prompt; fail with actionable error when required input is missing.
- `--yes`: confirms a write/destructive action in non-interactive contexts.
- `--dry-run`: validate and preview request without creating or publishing when the API supports it.
- `--agent-name`: records the calling agent in CLI metadata when included.
- `--open`: open a generated Dashboard, docs, OAuth or connect URL in the user's browser when the command supports it.

### 9.2 Auth Commands

```bash
unipost auth status
unipost auth login --api-key up_live_...
unipost auth logout
```

Requirements:

- `auth status` verifies that a credential exists and can authenticate.
- `auth login` stores credentials according to user-selected storage mode.
- `auth logout` removes local credentials but never revokes API keys on the server.

### 9.3 Doctor Commands

```bash
unipost doctor
unipost doctor --json
unipost doctor --check auth,workspace,profiles,accounts,rate-limit
```

Checks:

- CLI version
- API reachability
- API key validity
- workspace access
- profile availability
- connected account availability
- rate limit headers
- API response request id
- common environment misconfiguration

Human output should group results as pass/warn/fail. JSON output should include check IDs and normalized status.

### 9.4 Quickstart Commands

```bash
unipost init
unipost quickstart
unipost quickstart --platform linkedin
unipost quickstart --lang node
```

`init` responsibilities:

- detect API key
- call `auth status`
- fetch workspace
- fetch profiles
- save default profile if user chooses one
- print next command

`quickstart` responsibilities:

1. verify auth
2. select or create profile
3. create connect URL/session for selected platform
4. guide user to complete OAuth
5. list accounts
6. validate a sample post
7. optionally create a draft
8. generate SDK/cURL example using real IDs

Quickstart should prefer draft/validate over live publish.

### 9.5 Profiles Commands

```bash
unipost profiles list
unipost profiles get pr_...
unipost profiles create --name "Brand"
unipost profiles use pr_...
```

Requirements:

- `profiles use` sets local default profile.
- `profiles list --json` includes enough fields for agent grounding.
- Profile update/delete can be deferred unless developer demand is high.

### 9.6 Connect Commands

```bash
unipost connect create --platform linkedin --profile pr_...
unipost connect create --platform youtube --profile pr_... --return-url https://example.com/return
unipost connect get cs_...
```

Requirements:

- `connect create` returns a hosted URL and the session ID.
- Human output clearly says to open the URL in a browser.
- Optional `--open` may launch browser when user explicitly asks.
- `--json` output includes URL, session ID, platform, profile ID and expiration if available.

### 9.7 Accounts Commands

```bash
unipost accounts list
unipost accounts list --platform youtube
unipost accounts get sa_...
unipost accounts health --account sa_...
unipost accounts capabilities --account sa_...
unipost accounts metrics --account sa_...
```

Requirements:

- `accounts list` is one of the most important commands for both developers and agents.
- Human output should show platform, handle/name, profile, status, account ID.
- JSON output should preserve raw API fields.
- `health`, `capabilities`, and `metrics` help diagnose platform-specific issues.

### 9.8 Posts Commands

```bash
unipost posts validate --account sa_... --caption "Hello"
unipost posts draft --account sa_... --caption "Hello"
unipost posts create --account sa_... --caption "Hello" --yes --idempotency-key demo-001
unipost posts create --from-file post.json --dry-run
unipost posts list
unipost posts list --status failed
unipost posts get post_...
unipost posts analytics post_...
```

Requirements:

- `validate` should be the safest first publish-adjacent command.
- `draft` creates a server-side draft when supported.
- `create` can publish or schedule according to request body.
- In non-interactive mode, `create` requires `--yes` for live publish.
- In agent mode, live publish should require `--idempotency-key`.
- `--from-file` accepts full API-shaped JSON for advanced platform options.

Example `post.json`:

```json
{
  "account_ids": ["sa_..."],
  "caption": "Shipping with UniPost CLI.",
  "media_ids": ["med_..."],
  "idempotency_key": "launch-2026-06-02-001"
}
```

### 9.9 Media Commands

```bash
unipost media upload ./video.mp4
unipost media get med_...
unipost media wait med_... --timeout 120
```

Requirements:

- CLI should support local file upload because local binary transfer is awkward in MCP.
- Upload flow should reserve media through UniPost, upload bytes to storage, then poll media status.
- Human output should show `media_id` and next publish command.
- JSON output should include media ID, status, MIME type, size and readiness.

### 9.10 Analytics Commands

```bash
unipost analytics summary --from 2026-06-01 --to 2026-06-30
unipost analytics posts --from 2026-06-01 --to 2026-06-30
unipost analytics platforms
unipost analytics platform tiktok --from 2026-06-01 --to 2026-06-30
unipost analytics export --from 2026-06-01 --to 2026-06-30 --format csv
```

Requirements:

- Analytics commands are important for agents because they let agents reason from real performance data.
- Human output can summarize key metrics.
- JSON output preserves full API response.
- Export command may write a file when user provides `--output`; otherwise prints to stdout.

### 9.11 Examples Commands

```bash
unipost examples posts.create --lang curl
unipost examples posts.create --lang node --account sa_...
unipost examples connect.create --lang python --platform linkedin
unipost examples mcp.claude-code
```

Requirements:

- Examples should use real profile/account IDs when available.
- Generated code must redact API keys.
- Examples should include the docs URL for deeper reference.
- Supported languages initially: cURL, Node.js, Python, Go, Java.

### 9.12 Agent Commands

```bash
unipost agent context --json
unipost agent plan-publish --from-file post.json --json
unipost agent mcp-config claude-code --json
```

`agent context` response should include:

- workspace summary
- default profile
- profiles
- connected accounts
- recent posts summary
- failed posts summary
- analytics availability
- recommended next commands

`agent plan-publish` should:

- validate post payload
- identify required confirmations
- return a publish plan
- avoid writing unless user later calls `posts create --yes`

---

## 10. JSON Output Contract

All commands that support `--json` must return a stable envelope.

### 10.1 Success Envelope

```json
{
  "ok": true,
  "data": {},
  "warnings": [],
  "meta": {
    "request_id": "req_...",
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.1.0",
    "command": "accounts list",
    "source": "cli"
  }
}
```

### 10.2 Error Envelope

```json
{
  "ok": false,
  "error": {
    "code": "unauthorized",
    "message": "API key is missing or invalid.",
    "hint": "Set UNIPOST_API_KEY or run unipost auth login.",
    "docs_url": "https://unipost.dev/docs/quickstart"
  },
  "warnings": [],
  "meta": {
    "request_id": "req_...",
    "base_url": "https://api.unipost.dev",
    "cli_version": "0.1.0",
    "command": "auth status",
    "source": "cli"
  }
}
```

### 10.3 Exit Codes

```text
0   success
1   generic error
2   invalid arguments
3   missing required input
4   authentication failure
5   authorization failure
6   validation failure
7   upstream UniPost API failure
8   network failure
9   unsafe action blocked
10  timeout
```

Exit codes must remain stable for CI and agent usage.

---

## 11. Safety, Permissions, And Audit

### 11.1 Publish Guardrails

Default behavior:

- `posts validate` is safe and can run without confirmation.
- `posts draft` is a write action but does not publish to social networks.
- `posts create` that publishes immediately requires confirmation in human mode.
- `posts create --non-interactive` requires `--yes`.
- `posts create --json --non-interactive --yes` should require or strongly enforce `--idempotency-key`.

Recommended policy:

```text
human interactive publish: confirmation prompt required
human --yes publish: allowed
agent non-interactive publish: --yes + --idempotency-key required
agent dry-run: allowed
draft creation: --yes recommended but not mandatory in interactive mode
```

### 11.2 Destructive Actions

Destructive commands such as account disconnect, webhook delete, profile delete, or API key revoke should not be part of the initial agent beta. If added later:

- require `--yes`
- require explicit resource ID
- require `--non-interactive` callers to include a reason string
- emit audit metadata

### 11.3 Audit Metadata

Write commands should include metadata where the API supports it:

```json
{
  "source": "cli",
  "cli_version": "0.1.0",
  "agent_name": "codex",
  "idempotency_key": "..."
}
```

The backend remains the authority for authorization and sensitive decisions.

---

## 12. UX Flows

### 12.1 Developer First Run

```bash
unipost init
```

Expected flow:

1. CLI checks whether `UNIPOST_API_KEY` exists.
2. If missing, CLI asks for API key or links to Dashboard API Keys.
3. CLI calls workspace endpoint.
4. CLI lists profiles and lets user choose or create one.
5. CLI prints next command:

```bash
unipost connect create --platform linkedin --profile pr_...
```

### 12.2 First Connected Account

```bash
unipost connect create --platform linkedin --profile pr_...
```

Expected flow:

1. CLI creates connect session or OAuth URL.
2. CLI prints hosted URL.
3. User opens URL and completes OAuth.
4. User runs:

```bash
unipost accounts list --platform linkedin
```

5. CLI shows account ID and next validate command.

### 12.3 First Safe Post

```bash
unipost posts validate --account sa_... --caption "Shipping with UniPost CLI."
```

Expected flow:

1. CLI validates platform-specific constraints.
2. CLI prints pass/fail and warnings.
3. CLI offers draft command:

```bash
unipost posts draft --account sa_... --caption "Shipping with UniPost CLI."
```

### 12.4 Agent Context Grounding

```bash
unipost agent context --json --non-interactive
```

Expected flow:

1. CLI validates auth.
2. CLI fetches workspace, profiles, accounts and recent post summary.
3. CLI returns JSON with recommended next commands.
4. Agent uses account IDs from actual context instead of invented IDs.

### 12.5 Agent Safe Publish

```bash
unipost posts create \
  --from-file post.json \
  --dry-run \
  --json \
  --non-interactive
```

Then, after explicit user approval:

```bash
unipost posts create \
  --from-file post.json \
  --yes \
  --idempotency-key user-approved-2026-06-02-001 \
  --json \
  --non-interactive \
  --agent-name codex
```

Expected behavior:

- Dry-run returns validation result and publish plan.
- Live publish is blocked unless confirmation requirements are satisfied.
- Response includes post ID, result IDs, status and request ID.

---

## 13. Phased Delivery Plan

### Phase 0: PRD And Command Contract

Deliverables:

- This PRD.
- Initial command taxonomy.
- JSON envelope contract.
- Exit code contract.
- Safety policy.
- Docs page update from "Coming soon" to planned command reference.

Success criteria:

- Product, API and DX agree on first command set.
- No ambiguous publish safety requirement remains.

### Phase 1: CLI Foundation

Deliverables:

- Installable `@unipost/cli` package.
- `unipost --version`.
- `unipost auth status`.
- API client with base URL override.
- `--json`, `--non-interactive`, error envelope and exit codes.
- `unipost doctor` with auth/workspace/API reachability checks.

Success criteria:

- A user with `UNIPOST_API_KEY` can verify API access without reading docs.
- CI can fail reliably on missing/invalid auth.

### Phase 2: Developer Quickstart GA

Deliverables:

- `unipost init`.
- `unipost quickstart`.
- `profiles list/create/use`.
- `connect create/get`.
- `accounts list/get`.
- `posts validate`.
- `posts draft`.
- `examples posts.create`.

Success criteria:

- New developer can create or select profile, connect account, find account ID and create first draft from terminal.
- Quickstart does not require live publish.
- Docs page provides complete install and first-run flow.

### Phase 3: AI Agent Operator Beta

Deliverables:

- Stable `--json` output for accounts/posts/media/analytics.
- `agent context`.
- `agent plan-publish`.
- `posts create --from-file --dry-run`.
- Agent publish guardrails with `--yes` and `--idempotency-key`.
- Audit metadata for write commands.

Success criteria:

- Codex / Claude Code can inspect real UniPost account context without browsing docs.
- Agent can safely dry-run publish payloads.
- Live publish is impossible by accident in non-interactive mode.

### Phase 4: Advanced Ops And Diagnostics

Deliverables:

- `accounts health`.
- `accounts capabilities`.
- `accounts metrics`.
- `posts list --status failed`.
- `analytics summary/posts/platforms/export`.
- `media upload/get/wait`.
- webhook diagnostics if demand is confirmed.

Success criteria:

- Support can diagnose common account and post delivery issues from CLI.
- Users can upload local media and publish with `media_id`.
- Analytics can be consumed by agents and CI workflows.

### Phase 5: MCP Bridge And Ecosystem

Deliverables:

- `examples mcp.claude-code`.
- `agent mcp-config claude-code`.
- `agent mcp-config cursor`.
- MCP auth test command.
- Optional Homebrew distribution.

Success criteria:

- CLI can bootstrap MCP setup without replacing MCP.
- Agent users can choose CLI or MCP based on workflow.

---

## 14. Metrics

Product metrics:

- CLI installs.
- `init` starts and completions.
- `quickstart` starts and completions.
- Time from install to successful `doctor`.
- Time from install to first `accounts list`.
- Time from install to first `posts validate`.
- Time from install to first draft.
- Number of generated examples.
- Number of `--json` command runs.
- Number of agent dry-runs before publish.

Quality metrics:

- CLI command error rate.
- API auth failure rate after `init`.
- `doctor` warning/failure categories.
- Support tickets mentioning API key, account IDs, connect sessions or first post.
- Publish blocked by safety policy.
- Request IDs surfaced in support conversations.

Telemetry must avoid recording secrets, captions, full media URLs, or full API keys.

---

## 15. Acceptance Criteria

### 15.1 Developer Quickstart

- A user can install CLI and run `unipost auth status`.
- A user can run `unipost doctor` and see pass/warn/fail results.
- A user can list profiles and accounts.
- A user can create a connect URL/session for a supported platform.
- A user can validate a post without publishing.
- A user can create a draft post.
- A user can generate at least cURL and Node.js examples using their real account ID.
- Missing API key produces a clear hint and docs URL.

### 15.2 AI Agent Operator

- Every agent-relevant read command supports `--json`.
- JSON output uses the standard envelope.
- Error output uses the standard error envelope.
- `agent context --json` returns workspace, profiles, accounts and recent post summary.
- Non-interactive live publish fails without `--yes`.
- Non-interactive live publish fails without idempotency key when agent mode is detected or `--agent-name` is provided.
- Dry-run publish returns a validation result and plan without publishing.
- All write commands include source metadata when backend supports it.

### 15.3 Reliability And Security

- CLI redacts API keys in logs and output.
- CLI never prints OAuth tokens.
- CLI surfaces UniPost `request_id` when available.
- Exit codes match the documented contract.
- Base URL override works for local/dev/staging validation.
- CI can use CLI without prompts by passing `--non-interactive`.

---

## 16. Dependencies

API dependencies:

- API key auth through existing UniPost public API.
- Workspace endpoint.
- Profiles endpoints.
- Accounts list/get/health/capabilities/metrics endpoints.
- Connect session or OAuth connect endpoint.
- Posts validate/draft/create/list/get endpoints.
- Media reserve/upload/get endpoints.
- Analytics summary/posts/platforms/export endpoints.

Backend behavior that improves CLI quality:

- Consistent `request_id` in responses.
- Consistent `error.normalized_code`.
- Rate limit headers.
- Idempotency key handling for create post.
- Optional metadata field for CLI/agent audit.
- Draft creation path that does not publish.
- Validation endpoint that catches platform-specific constraints.

Docs dependencies:

- CLI docs page.
- Quickstart docs.
- MCP docs.
- API reference.
- SDK docs.

---

## 17. Risks And Mitigations

### Risk: CLI becomes a full REST wrapper too early

Mitigation:

- Prioritize intent-based commands.
- Keep advanced API shapes behind `--from-file`.
- Use docs/API reference for long-tail endpoints.

### Risk: Agent accidentally publishes real content

Mitigation:

- Validate/dry-run first.
- Require explicit `--yes`.
- Require idempotency key for non-interactive agent publish.
- Keep destructive commands out of early agent beta.

### Risk: JSON contract changes break agents

Mitigation:

- Version the CLI.
- Keep envelope stable.
- Add fields without removing existing fields.
- Reserve breaking changes for major version.

### Risk: Secrets leak in diagnostics

Mitigation:

- Redact API keys and tokens.
- Avoid printing full presigned URLs unless the command explicitly requires it.
- Add automated redaction tests.

### Risk: CLI conflicts with MCP positioning

Mitigation:

- Position CLI as terminal/script interface and MCP as agent-native protocol.
- Let CLI generate and test MCP configs.
- Use the same conceptual tool names where possible.

---

## 18. Documentation Requirements

The CLI docs page should move from "Coming soon" to a concrete guide with:

- installation
- auth
- quickstart
- command reference
- agent mode
- JSON output contract
- publish safety rules
- examples
- MCP bridge
- troubleshooting

Docs should explicitly explain:

- CLI helps developers avoid frequent docs lookup during first integration.
- CLI helps AI agents avoid browsing UniPost webpages for account state.
- CLI does not replace SDK, API or MCP.
- CLI defaults to validation/draft before live publish.

---

## 19. Recommended V1 Command Set

Minimum GA command set:

```text
unipost --version
unipost auth status
unipost auth login
unipost auth logout
unipost doctor
unipost init
unipost quickstart
unipost profiles list
unipost profiles create
unipost profiles use
unipost connect create
unipost connect get
unipost accounts list
unipost accounts get
unipost posts validate
unipost posts draft
unipost posts create
unipost posts list
unipost posts get
unipost media upload
unipost media get
unipost analytics summary
unipost examples posts.create
unipost agent context
unipost agent plan-publish
unipost agent mcp-config
```

Commands that can wait until after GA:

```text
unipost accounts disconnect
unipost profiles delete
unipost webhooks create/update/delete
unipost api-keys create/revoke
unipost analytics export
unipost posts update/delete
```

---

## 20. Product Decisions

Recommended decisions for first implementation:

- Use one binary: `unipost`.
- Use npm package first: `@unipost/cli`.
- Keep Homebrew as a later distribution channel.
- Use `UNIPOST_API_KEY` as the clearest first auth path.
- Add optional local credential storage after the basic flow works.
- Default first post path to `validate` then `draft`, not live publish.
- Support `--json` on all read commands from the beginning.
- Add `agent context` early because it is the highest-value AI-agent primitive.
- Require `--yes` and `--idempotency-key` for non-interactive agent publish.
- Keep MCP separate but let CLI generate MCP config snippets.

---

## 21. Definition Of Done For The CLI Product

The CLI product is considered successfully launched when:

1. A new developer can install it and complete Quickstart without reading the full API docs.
2. The same developer can generate a usable SDK/cURL example containing their real profile/account IDs.
3. An AI coding agent can run `agent context --json` and get enough data to choose real accounts instead of inventing IDs.
4. An AI coding agent can dry-run a post and receive a structured publish plan.
5. Live publish cannot happen accidentally in non-interactive agent usage.
6. Support can ask users to run `unipost doctor --json` and receive safe diagnostic output.
7. CLI docs clearly explain when to use CLI, SDK, raw API and MCP.
