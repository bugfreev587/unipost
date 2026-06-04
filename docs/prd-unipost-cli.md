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
2. 让新用户在 15 分钟内创建第一条 draft post，或生成一份可以复制到自己项目中的 cURL/native HTTP publish 示例；SDK 示例在对应 SDK package 发布后启用。
3. 让 AI agent 能通过 CLI 读取 accounts、posts、media、analytics、workspace context，并在安全限制下执行 dry-run 或 publish。
4. 为所有命令提供稳定的 `--json` 输出、明确 exit code、错误 code、human hint、request id 和 docs link。
5. 让 CLI 成为 UniPost docs、SDK、MCP 的终端入口：能生成 examples、测试 API auth、生成 MCP config、引导用户打开 Dashboard。
6. 将真实发布行为默认设计为安全操作：先 validate / dry-run / draft，再 explicit publish。
7. 让 Support / Ops 可以用 `doctor` 和诊断命令快速定位常见集成问题。
8. 让注册后的用户可以把 UniPost 直接交给 Codex / Claude Code 等 agent 接入：用户用白话提出目标，agent 通过 CLI bootstrap、结构化诊断和必要的用户确认完成 API 接入。
9. 让 AI agent 不需要猜测底层 terminal 命令：CLI 应暴露 machine-readable capability catalog、intent planning wrapper、safe execution contract，并在成熟阶段提供 MCP server / Codex skill / Claude Code instructions package。

---

## 3.1 Current Backend Reality And Launch Gates

This PRD includes the target onboarding experience, but some backend dependencies are not implemented yet. The implementation plan must distinguish between fallback launch paths and the full agent-assisted onboarding path.

Hard dependencies for the full post-signup agent onboarding flow:

- **Not implemented yet:** device auth or browser auth endpoint for CLI login.
- **Not implemented yet:** Dashboard-issued setup token (`ust_...`) issuance.
- **Not implemented yet:** setup-token exchange endpoint that creates a named, revocable CLI API key and returns the plaintext key once to the local CLI.
- **Needs backend confirmation or addition:** request metadata fields for CLI/agent audit, such as `source`, `cli_version`, `agent_name`, and `client`.
- **Needs package availability check:** SDK-backed examples require published SDK packages. Until SDK packages are confirmed available, CLI examples must include cURL and dependency-free language examples that use native HTTP/fetch.

Phase implications:

- Phase 1 and the minimal Phase 2 Quickstart can launch with `UNIPOST_API_KEY` as the auth path.
- The polished "paste this Dashboard prompt into Codex / Claude Code" flow is not complete until setup-token or device auth exists.
- If setup-token/device auth is not ready for Phase 2, Phase 2 success criteria should use the fallback path: the user provides `UNIPOST_API_KEY`, then `agent bootstrap` runs diagnostics and context discovery.
- Full agent-assisted onboarding becomes GA only when Dashboard setup token or device auth can create/store a local CLI credential without manual API-key copy/paste.

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
   - `--json` schema、exit codes、command names, agent intent names, capability catalog schema and safety-level enums need careful evolution.
   - `agent capabilities` should include a `catalog_version` so agents can reason about compatibility.
   - Breaking changes 必须通过 major version 或 compatibility mode 处理。

7. **Redaction and auditability**
   - 不输出完整 API key、OAuth tokens、presigned private URLs 中的敏感 query，除非命令明确是创建 secret 的一次性结果。
   - 写操作应带 `source=cli`、`cli_version`、可选 `agent_name`、`idempotency_key` 等 metadata。

8. **Agent contracts over command guessing**
   - Agent should not need to infer command syntax from docs or memorize endpoint mappings.
   - CLI should expose a small, stable agent surface with capabilities, intent names, input schemas, safety levels, missing-input detection and recommended next prompts.
   - Codex / Claude Code can understand the user's natural language, but UniPost should provide the structured contract that turns that intent into safe UniPost actions.

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
unipost agent bootstrap --client codex --json
unipost agent bootstrap --client claude-code --json
unipost agent capabilities --json
unipost agent guide --client codex
unipost agent context --json
unipost agent plan --intent create_draft_post --json
unipost accounts list --json --non-interactive
unipost accounts health --account sa_... --json
unipost posts list --status failed --json
unipost posts get post_... --json
unipost media upload ./video.mp4 --json
unipost analytics summary --from 2026-06-01 --to 2026-06-30 --json
unipost posts create --from-file post.json --dry-run --json
unipost posts create --from-file post.json --yes --idempotency-key user-action-...
```

### 7.4 Agent-Assisted Onboarding

Agent-assisted onboarding is the ideal post-signup path for users who want Codex or Claude Code to help them integrate UniPost API without reading the full docs.

The Dashboard should expose a clear entry point after signup:

```text
Connect with Claude Code / Codex
```

That entry point should give users either:

1. a one-line command, or
2. a copyable prompt for their coding agent.

Recommended command:

```bash
npm install -g @unipost/cli
unipost agent bootstrap --client codex
```

Recommended prompt:

```text
请帮我接入 UniPost。先确认本地已安装 UniPost CLI，然后运行 unipost agent bootstrap --client codex，根据 CLI 输出继续操作。
```

The product goal is that the user can speak naturally in Codex / Claude Code:

```text
帮我把我的项目接入 UniPost API，我想先支持 LinkedIn 发帖。
```

The agent should then use the CLI to discover real account state, ask the user only when ambiguity remains, and complete setup steps without requiring the user to search UniPost docs manually.

### 7.5 Agent Invocation Layer

Terminal commands alone are not enough for reliable AI-agent usage. Codex / Claude Code can run shell commands, but without a first-party agent contract they must infer command syntax, required inputs and safety rules from docs or examples. The CLI should therefore include an agent invocation layer above the human terminal surface.

Layering model:

1. **Core CLI commands**
   - Human and CI-friendly commands such as `accounts list`, `posts draft`, `posts create`, `media upload` and `analytics summary`.
   - These remain the source of truth for execution.

2. **Agent wrapper commands**
   - `agent capabilities`, `agent guide`, `agent context`, `agent plan`, `agent bootstrap`, and later `agent execute`.
   - These commands expose task-oriented contracts, accepted intent names, JSON schemas, safety levels, required confirmations and next prompts.
   - `agent execute` is not required for Phase 3; it should ship only after a security review proves plan validation is safe.

3. **MCP server**
   - Agent-native tool protocol for Codex, Claude Code and other compatible clients.
   - MCP tools should map to the same conceptual actions as the CLI wrapper, so behavior stays consistent.

4. **Client-specific instruction packages**
   - Codex skill/plugin package.
   - Claude Code instruction package.
   - Optional Cursor/Windsurf instructions.
   - These packages teach the agent when to call UniPost, how to interpret CLI JSON, and when to stop for user confirmation.

The CLI should not rely on an internal free-form LLM to understand every user request. Codex / Claude Code are responsible for understanding the user's natural language. UniPost is responsible for exposing a stable capability catalog and safe planning/execution interface so the agent can map natural language to supported UniPost intents.

Recommended intent examples:

```text
inspect_workspace
connect_account
validate_post
create_draft_post
plan_publish_post
publish_post
schedule_post
wait_for_post
wait_for_connect_session
cancel_scheduled_post
retry_failed_delivery
upload_media
inspect_failed_posts
summarize_analytics
generate_integration_example
```

`agent capabilities --json` should describe each intent, required inputs, optional inputs, safety level and canonical action mapping. `agent guide --client codex` should produce client-tailored instructions that can be installed into a Codex skill/plugin or pasted into a project `AGENTS.md` when appropriate.

---

## 8. Authentication And Configuration

### 8.1 Auth Sources

CLI resolves credentials and bootstrap auth signals in this order:

1. `--api-key` flag
2. `--setup-token` flag for `agent bootstrap` only
3. `UNIPOST_API_KEY` environment variable
4. local CLI config metadata pointing to OS keychain, if configured
5. explicit file-based credential storage, only if the user opted in

Recommended v1 default:

- Prefer `UNIPOST_API_KEY` for developer clarity and CI compatibility.
- Allow `unipost auth login --api-key ...` to store a redacted local config or keychain value.
- For agent-assisted onboarding, prefer browser/device auth or one-time setup tokens over asking users to paste long-lived API keys into Codex / Claude Code.
- Never print the full API key after storing it.

### 8.2 Browser, Device, And Setup Token Auth

The smoothest Codex / Claude Code onboarding should not require users to manually copy an API key into the agent chat. Instead, UniPost should support one or both of these flows:

1. **Device auth flow**
   - CLI prints a short code and activation URL.
   - User opens the URL in a browser and approves CLI access.
   - CLI polls until authorization is complete.
   - Backend creates a named, revocable CLI API key for the user's workspace.
   - CLI stores the returned plaintext key locally and never prints it.

2. **Dashboard setup token flow**
   - User clicks "Connect with Claude Code / Codex" in Dashboard.
   - Dashboard creates a short-lived, single-use setup token scoped to CLI onboarding.
   - User copies the generated command or prompt into the agent.
   - CLI exchanges the setup token for a named, revocable CLI API key.
   - Backend returns the plaintext API key only once to the local CLI.
   - CLI stores it in keychain when available and continues setup.

Example:

```bash
unipost auth login
unipost agent bootstrap --client codex --setup-token ust_...
```

Requirements:

- Setup tokens must expire quickly and be single-use.
- Setup tokens must not be usable for ordinary UniPost API calls, direct publishing, API key listing, API key revocation, or destructive actions.
- The exchanged local credential should be scoped according to the user's UniPost permissions.
- Generated keys should be clearly named, such as `Codex CLI` or `Claude Code CLI`, and visible/revocable in Dashboard -> API Keys.
- CLI must store credentials in OS keychain when available.
- CLI must provide a clear fallback to `UNIPOST_API_KEY` for CI and advanced users.
- Agent bootstrap output must redact setup tokens and local credentials.

### 8.3 Base URL

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

### 8.4 Config File

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

### 8.5 Credential Storage

CLI-created API keys should be stored locally in the user's OS credential store by default, not in plaintext config.

Recommended default storage:

```text
macOS      -> Keychain
Windows    -> Credential Manager
Linux      -> Secret Service / libsecret
CI/headless -> UNIPOST_API_KEY environment variable
```

Recommended keychain shape:

```text
service: unipost
account: <workspace_id>:<client_id_or_default>
secret: up_live_...
```

Multiple local credentials are allowed. `auth list` should enumerate redacted workspace/client entries, and `auth use` should select the default credential by workspace ID or local alias without printing the API key.

The config file should store only non-sensitive metadata:

```json
{
  "base_url": "https://api.unipost.dev",
  "default_profile_id": "pr_...",
  "credential": {
    "storage": "keychain",
    "workspace_id": "ws_...",
    "key_id": "key_...",
    "name": "Codex CLI",
    "prefix": "up_live_abcd",
    "client": "codex"
  }
}
```

Credential storage requirements:

- The full API key is never printed after creation.
- `config show` shows only credential metadata and redacted prefixes.
- If keychain storage is unavailable in interactive mode, CLI may ask whether to use explicit file-based storage.
- If keychain storage is unavailable in `--non-interactive` mode, CLI should fail with a clear hint to use `UNIPOST_API_KEY`.
- CI should prefer `UNIPOST_API_KEY` and avoid persistent local credential storage.
- `auth logout` removes local credentials but does not revoke the remote API key.
- Remote key revocation remains a separate Dashboard/API action.

### 8.6 Networking, Retry, Proxies, And TLS

The CLI should make network behavior predictable for humans, CI and agents.

Requirements:

- Default API requests should have finite connect and response timeouts.
- Read requests and idempotent writes may retry on transient network errors, 429 and 5xx responses with bounded exponential backoff.
- Unsafe writes must not be retried automatically unless an idempotency key is present.
- CLI must respect `Retry-After` when returned by the API and should also surface UniPost rate-limit headers in JSON metadata.
- Polling commands such as `media wait`, `connect wait` and `posts wait` should back off, cap polling frequency and respect rate-limit headers.
- Proxy support should follow standard environment variables: `HTTPS_PROXY`, `HTTP_PROXY` and `NO_PROXY`.
- Enterprise/custom CA support should use standard runtime mechanisms where possible, such as `NODE_EXTRA_CA_CERTS` or `SSL_CERT_FILE`.
- `--insecure` is allowed only for local/dev troubleshooting and should warn in human output and JSON `warnings`; production API hosts should reject or strongly discourage it.

### 8.7 Telemetry And Privacy

Telemetry can improve CLI reliability, but it must be explicit, redact sensitive data and be easy to disable.

Requirements:

- First run should disclose whether telemetry is enabled and what categories are collected.
- Telemetry must never include API keys, setup tokens, OAuth tokens, captions, full media URLs, local file paths, or raw post bodies.
- Users can opt out with `unipost config set telemetry false`, `--no-telemetry`, or `UNIPOST_TELEMETRY=0`.
- CI should be able to disable telemetry deterministically through environment variables.
- Telemetry events should use stable command and outcome names, not localized human text.

---

## 9. Command Taxonomy

### 9.1 Global Flags

```bash
--json
--output <table|json|yaml>
--field <field_or_jsonpath>
--non-interactive
--yes
--quiet
--verbose
--no-color
--base-url <url>
--api-key <key>
--setup-token <token>
--profile <profile_id>
--account <account_id>
--client <codex|claude-code|cursor|windsurf>
--intent <intent_name>
--idempotency-key <key>
--agent-name <name>
--dry-run
--schedule-at <iso8601>
--limit <n>
--cursor <cursor>
--all
--lang <locale>
--no-telemetry
--open
--insecure
```

Behavior:

- `--json`: output stable JSON envelope.
- `--output`: choose `table`, `json`, or `yaml`; `--json` is an alias for `--output json`.
- `--field`: print one field or JSONPath-like selector for scripting; never prints unredacted secrets.
- `--non-interactive`: never prompt; fail with actionable error when required input is missing.
- `--yes`: confirms a write/destructive action in non-interactive contexts.
- `--no-color`: disables ANSI color; `NO_COLOR=1` must have the same effect.
- `--dry-run`: validate and preview request without creating or publishing when the API supports it.
- `--schedule-at`: schedules a publish-capable post at an RFC3339/ISO-8601 timestamp when used by posts commands.
- `--limit`, `--cursor`, `--all`: standard pagination controls for list commands.
- `--setup-token`: passes a short-lived Dashboard-generated setup token for bootstrap only.
- `--client`: tells bootstrap/config commands which agent or IDE should receive tailored instructions.
- `--intent`: tells `agent plan` which UniPost task the agent believes the user requested.
- `--agent-name`: records the calling agent in CLI metadata when included.
- `--lang`: requests localized human-facing text while keeping machine fields stable.
- `--no-telemetry`: disables CLI telemetry for the current run.
- `--open`: open a generated Dashboard, docs, OAuth or connect URL in the user's browser when the command supports it.
- `--insecure`: disables TLS verification only for explicit local/dev troubleshooting; it must warn loudly and should be rejected for production `api.unipost.dev`.

`--open` behavior:

- In interactive terminals, `--open` may launch the user's browser.
- In `--non-interactive` mode, `--open` must not block setup. CLI should return the URL in output and include a warning that browser launch was skipped.
- In headless or unsupported environments, CLI should return the URL and a `browser_open_unavailable` warning instead of failing the command.

### 9.2 Auth Commands

```bash
unipost auth status
unipost auth login --api-key up_live_...
unipost auth login
unipost auth list
unipost auth use ws_...
unipost auth logout
```

Requirements:

- `auth status` verifies that a credential exists and can authenticate.
- `auth login --api-key ...` stores credentials according to user-selected storage mode.
- `auth login` without `--api-key` starts browser/device auth once the backend exists; until then it should explain that `UNIPOST_API_KEY` or `--api-key` is required.
- `auth list` shows locally known credentials/workspaces with key names and prefixes redacted.
- `auth use <workspace_id_or_alias>` switches the default local credential when multiple workspace credentials exist.
- `auth logout` removes local credentials but never revokes API keys on the server.

### 9.3 Doctor Commands

```bash
unipost doctor
unipost doctor --json
unipost doctor --check auth,workspace,profiles,accounts,rate-limit
```

Checks:

- CLI version
- minimum supported CLI version / upgrade hint when API requires newer CLI behavior
- API reachability
- API key validity
- workspace access
- profile availability
- connected account availability
- rate limit headers
- API response request id
- common environment misconfiguration

Human output should group results as pass/warn/fail. JSON output should include check IDs and normalized status.

### 9.4 Config Commands

```bash
unipost config show
unipost config path
unipost config set base_url https://dev-api.unipost.dev
unipost config set default_profile_id pr_...
```

Requirements:

- `config show` prints effective configuration with secrets redacted.
- `config path` prints the config file location.
- `config set` supports non-secret preferences such as `base_url` and `default_profile_id`.
- CLI must not store plaintext API keys in config unless the user explicitly opts into file-based storage.

### 9.5 Quickstart Commands

```bash
unipost init
unipost init --force
unipost init --reauth
unipost quickstart
unipost quickstart --platform linkedin
unipost quickstart --lang node
```

`init` responsibilities:

- detect existing local credential or `UNIPOST_API_KEY`
- call `auth status`
- fetch workspace
- fetch profiles
- save default profile if user chooses one
- print next command
- avoid creating a new API key when an existing local credential is valid
- restart browser/device/setup-token auth only when no valid credential exists or the user explicitly passes `--reauth` / `--replace-key`

`quickstart` responsibilities:

1. verify auth
2. select or create profile
3. create connect URL/session for selected platform
4. guide user to complete OAuth
5. list accounts
6. validate a sample post
7. optionally create a draft
8. generate cURL/native HTTP/fetch example using real IDs; generate SDK example only when the corresponding SDK package is published

Quickstart should prefer draft/validate over live publish.

Init recovery requirements:

- `unipost init` is resumable and can be run repeatedly.
- A declined key-creation prompt is not terminal; users can rerun `init` or `agent bootstrap`.
- An expired or used setup token requires a new Dashboard setup token or a fresh device-auth flow.
- `--force` reruns setup checks and refreshes local non-secret config, but does not replace a valid credential by default.
- `--reauth` or `--replace-key` starts credential replacement and may create a new named API key after user confirmation.

### 9.6 Profiles Commands

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

### 9.7 Connect Commands

```bash
unipost connect create --platform linkedin --profile pr_...
unipost connect create --platform youtube --profile pr_... --return-url https://example.com/return
unipost connect get cs_...
unipost connect wait cs_... --timeout 300 --json
```

Requirements:

- `connect create` returns a hosted URL and the session ID.
- Human output clearly says to open the URL in a browser.
- Optional `--open` may launch browser when user explicitly asks.
- `--json` output includes URL, session ID, platform, profile ID and expiration if available.
- `connect wait` polls `GET /v1/connect/sessions/{id}` until `completed`, `expired`, `canceled`, or timeout.
- `connect wait --json` returns `completed_social_account_id` / `managed_account_id` when the session completes, so agents do not need to diff `accounts list`.
- `connect wait` should use bounded exponential backoff, respect `Retry-After` and rate-limit headers, and exit with code 10 on timeout.

### 9.8 Accounts Commands

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
- `accounts get` is a CLI-side helper implemented by calling `GET /v1/accounts` and selecting the matching account ID locally; the public API does not currently expose `GET /v1/accounts/{id}`.
- Human output should show platform, handle/name, profile, status, account ID.
- JSON output should preserve raw API fields.
- `health`, `capabilities`, and `metrics` help diagnose platform-specific issues.

### 9.9 Posts Commands

```bash
unipost posts validate --account sa_... --caption "Hello"
unipost posts draft --account sa_... --caption "Hello"
unipost posts create --account sa_... --caption "Hello" --yes --idempotency-key demo-001
unipost posts create --account sa_... --caption "Hello" --schedule-at 2026-06-10T09:00:00Z --yes --idempotency-key demo-002
unipost posts schedule --account sa_... --caption "Hello" --at 2026-06-10T09:00:00Z --yes --idempotency-key demo-003
unipost posts create --from-file post.json --dry-run
unipost posts publish-draft post_... --yes --idempotency-key demo-004
unipost posts wait post_... --timeout 120 --json
unipost posts cancel post_... --yes
unipost posts retry post_... --result <result_id> --yes
unipost posts list
unipost posts list --status failed
unipost posts list --status scheduled
unipost posts get post_...
unipost posts analytics post_...
```

Requirements:

- `validate` should be the safest first publish-adjacent command.
- `draft` creates a server-side draft by calling `POST /v1/posts` with `status: "draft"`.
- `publish-draft` publishes an existing draft by calling `POST /v1/posts/{id}/publish`.
- Updating an existing draft maps to `PATCH /v1/posts/{id}` and can be added as `posts update-draft` when needed.
- `create` can publish immediately or schedule according to request body / `--schedule-at`.
- `schedule` is an intent-first alias for `posts create` with top-level `scheduled_at`; it maps to the same create endpoint.
- `draft` does not require `--yes` in either interactive or non-interactive mode because it does not publish to external social networks.
- Live publish means `posts create` without `status: "draft"` and without `--dry-run`, `posts create --schedule-at`, `posts schedule`, or `posts publish-draft`.
- Scheduled publish is treated as `live_write` because it will eventually publish to external social networks.
- Live publish and scheduled publish in non-interactive mode must include `--yes` and `--idempotency-key`; missing `--yes` exits with code 9 (`unsafe action blocked`), and missing `--idempotency-key` exits with code 3 (`missing required input`), as defined in §11.1.
- `posts wait` polls the post and per-platform delivery results until a terminal status such as `published`, `failed`, `partial`, `canceled`, or timeout; it exits with code 10 on timeout.
- `posts cancel` cancels a draft or scheduled post by calling `POST /v1/posts/{id}/cancel` or the canonical lifecycle update path when that becomes preferred.
- `posts retry --result <result_id>` retries a failed per-platform delivery by calling `POST /v1/posts/{id}/results/{resultID}/retry`.
- The implementation must confirm the canonical `social_post_results` ID prefix before publishing examples; the PRD uses `<result_id>` to avoid implying one prefix.
- Delivery-job level retry/cancel can be exposed later as `posts jobs retry|cancel` if support workflows need direct job IDs.
- `--from-file` accepts full API-shaped JSON for advanced platform options.
- v1 supports multi-account/cross-post payloads through `account_ids[]` in `post.json`. Bulk scheduling/draft semantics beyond the current API-supported top-level `scheduled_at` should remain behind `--from-file` and are not a separate v1 resource.

Example `post.json`:

```json
{
  "account_ids": ["sa_..."],
  "caption": "Shipping with UniPost CLI.",
  "media_ids": ["med_..."],
  "scheduled_at": "2026-06-10T09:00:00Z",
  "idempotency_key": "launch-2026-06-02-001"
}
```

### 9.10 Media Commands

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
- `media wait` should poll with bounded exponential backoff, respect rate-limit headers when present, and exit with code 10 on timeout.

### 9.11 Analytics Commands

```bash
unipost analytics summary --from 2026-06-01 --to 2026-06-30
unipost analytics posts --from 2026-06-01 --to 2026-06-30
unipost analytics platforms
unipost analytics platform tiktok --from 2026-06-01 --to 2026-06-30
unipost analytics export --from 2026-06-01 --to 2026-06-30 --format csv   # post-full-V1 unless pulled forward
```

Requirements:

- Analytics commands are important for agents because they let agents reason from real performance data.
- `analytics platforms` maps to `GET /v1/analytics/platforms`.
- `analytics platform <platform>` maps to `GET /v1/analytics/platforms/{platform}`.
- Legacy/internal `GET /v1/analytics/by-platform` can remain outside the CLI v1 surface unless a later implementation plan needs it.
- Human output can summarize key metrics.
- JSON output preserves full API response.
- `analytics export` is post-full-V1 unless pulled forward explicitly. Export may write a file when user provides `--out-file`; otherwise it prints to stdout.

### 9.12 Examples Commands

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
- v1 must support cURL and dependency-free Node.js `fetch` examples even if SDK packages are not published.
- SDK-backed examples for Node.js, Python, Go and Java are enabled only after the corresponding SDK package is published and the install command is confirmed.

### 9.13 Agent Commands

```bash
unipost agent bootstrap --client codex --json
unipost agent bootstrap --client claude-code --json
unipost agent bootstrap --client codex --setup-token ust_... --json
unipost agent capabilities --json
unipost agent guide --client codex
unipost agent guide --client claude-code
unipost agent context --json
unipost agent plan --intent create_draft_post --from-file request.json --json
unipost agent plan --intent plan_publish_post --from-file post.json --json
unipost agent plan-publish --from-file post.json --json
unipost agent execute --plan plan.json --json        # deferred to Phase 5 / security review
unipost agent install --client codex                 # deferred to Phase 5
unipost agent install --client claude-code           # deferred to Phase 5
unipost agent mcp-config codex --json
unipost agent mcp-config claude-code --json
```

`agent execute` and `agent install` are target commands, not Phase 3 launch blockers. `agent execute` should remain optional until the plan trust model has been security-reviewed.

`agent bootstrap` should:

- detect whether the CLI is installed and current
- detect the requested client (`codex`, `claude-code`, `cursor`, `windsurf`, or `generic`)
- complete browser/device/setup-token auth when needed
- run `doctor` checks
- fetch `agent context`
- identify missing setup steps
- return next actions in a shape the agent can follow
- provide user-facing questions when the agent needs clarification
- generate MCP or CLI instructions tailored to the selected client
- avoid publish or destructive writes

`agent capabilities` should:

- return a machine-readable catalog of supported agent intents
- include `catalog_version`
- include required inputs, optional inputs, output schemas, safety level and canonical action mapping
- include whether an intent is read-only, draft-only, dry-run-only, or live-write capable
- include whether the intent can run without user confirmation
- include client-specific notes when `--client` is provided

`agent guide` should:

- output concise client-specific usage guidance for Codex, Claude Code, Cursor, Windsurf or generic agents
- include recommended command order for onboarding and common workflows
- include guardrails for publish, destructive actions, credential storage and user confirmation
- be suitable for Dashboard copy, `AGENTS.md`, Codex skills, Claude Code instructions or MCP server instructions
- avoid including secrets or user-specific credentials

`agent context` response should include:

- workspace summary
- default profile
- profiles
- connected accounts
- recent posts summary
- failed posts summary
- analytics availability
- recommended next commands

`agent plan` should:

- accept an explicit `--intent` selected by the agent from the capabilities catalog
- accept structured inputs through flags or `--from-file`
- validate available local context and identify missing inputs
- return an ordered plan containing structured actions and arguments, not raw shell command strings
- include optional display-only command strings only for human readability; those strings are not executable authority
- classify each step by safety level
- return `required_user_confirmations` for ambiguous choices, live publish, destructive actions or credential replacement
- avoid executing writes by default
- return `safe_to_execute_without_user: true` only for read-only, validate-only or draft-only steps that do not require a user choice

`agent plan-publish` should remain a convenience alias for:

```bash
unipost agent plan --intent plan_publish_post --from-file post.json --json
```

It should:

- validate post payload
- identify required confirmations
- return a publish plan
- avoid writing unless user later calls `posts create --yes`

`agent execute` should:

- be deferred until Phase 5 or an explicit post-Phase-3 security review; Phase 3 should rely on `agent plan` plus normal CLI/MCP execution paths
- accept only a plan created by `agent plan`
- treat the plan file as untrusted input
- revalidate the plan before running it by recomputing allowed actions, arguments and safety levels from the current capabilities catalog
- never trust `safety_level`, `safe_to_execute_without_user`, `display_command`, or any other plan field as execution authority
- never shell-exec or eval command strings from the plan file
- execute only structured actions through internal CLI handlers or a fixed action registry
- execute read-only, validate and draft steps when confirmation rules allow
- never execute `live_write` steps; live publish must go through explicit `posts create --yes --idempotency-key` or `posts publish-draft --yes --idempotency-key`
- fail if the plan references unknown commands, unsupported intent names or stale resource IDs

`agent install` should:

- install or print client-specific setup instructions for Codex / Claude Code when supported
- prefer non-destructive config changes and ask before editing user config files
- support a dry-run mode in implementation
- explain manual fallback steps when automatic installation is not available

Bootstrap responses should distinguish between:

- `already_configured`: a valid local credential already exists; no setup token is exchanged and no new API key is created.
- `ready`: CLI is authenticated and usable.
- `needs_user_auth`: user must open an activation URL or approve Dashboard setup.
- `needs_profile`: user has no profile or must choose one.
- `needs_account_connection`: user must connect a social account.
- `needs_user_choice`: multiple valid resources exist and user should choose.
- `blocked`: setup cannot proceed without support, credentials, or a product decision.

Example next-action shape:

```json
{
  "ok": true,
  "data": {
    "status": "needs_account_connection",
    "recommended_prompt": "你的 UniPost workspace 里还没有 LinkedIn 账号。要我现在创建 LinkedIn 连接链接吗？",
    "next_commands": [
      "unipost connect create --platform linkedin --open --json",
      "unipost accounts list --platform linkedin --json"
    ],
    "safe_to_continue_without_user": false
  },
  "warnings": [],
  "meta": {
    "cli_version": "0.1.0",
    "command": "agent bootstrap",
    "source": "cli"
  }
}
```

Example capabilities shape:

```json
{
  "ok": true,
  "data": {
    "catalog_version": "2026-06-02.1",
    "intents": [
      {
        "name": "create_draft_post",
        "description": "Create a server-side draft without publishing to social networks.",
        "safety_level": "draft_write",
        "requires_user_confirmation": false,
        "required_inputs": ["account_id", "caption"],
        "optional_inputs": ["media_ids", "profile_id"],
        "canonical_actions": [
          {
            "action": "posts.draft",
            "display_command": "unipost posts draft --account <account_id> --caption <caption> --json"
          }
        ]
      },
      {
        "name": "publish_post",
        "description": "Publish or schedule social content after explicit user approval.",
        "safety_level": "live_write",
        "requires_user_confirmation": true,
        "required_inputs": ["account_id", "caption", "idempotency_key"],
        "optional_inputs": ["media_ids", "profile_id", "scheduled_at"],
        "canonical_actions": [
          {
            "action": "posts.create_live",
            "display_command": "unipost posts create --from-file post.json --yes --idempotency-key <key> --json --non-interactive"
          },
          {
            "action": "posts.schedule",
            "display_command": "unipost posts schedule --account <account_id> --caption <caption> --at <scheduled_at> --yes --idempotency-key <key> --json --non-interactive"
          }
        ]
      }
    ]
  },
  "warnings": [],
  "meta": {
    "cli_version": "0.1.0",
    "command": "agent capabilities",
    "source": "cli"
  }
}
```

Example plan shape:

```json
{
  "ok": true,
  "data": {
    "intent": "create_draft_post",
    "status": "ready_to_execute",
    "safe_to_execute_without_user": true,
    "required_user_confirmations": [],
    "missing_inputs": [],
    "steps": [
      {
        "id": "validate_post",
        "safety_level": "validate_only",
        "action": "posts.validate",
        "args": {
          "account_id": "sa_...",
          "caption": "..."
        },
        "display_command": "unipost posts validate --account sa_... --caption \"...\" --json --non-interactive"
      },
      {
        "id": "create_draft",
        "safety_level": "draft_write",
        "action": "posts.draft",
        "args": {
          "account_id": "sa_...",
          "caption": "..."
        },
        "display_command": "unipost posts draft --account sa_... --caption \"...\" --json --non-interactive"
      }
    ]
  },
  "warnings": [],
  "meta": {
    "cli_version": "0.1.0",
    "command": "agent plan",
    "source": "cli"
  }
}
```

### 9.14 Completion Commands

```bash
unipost completion bash
unipost completion zsh
unipost completion fish
```

Requirements:

- Completion scripts should include stable command names, flags and known enum values such as `--client` and supported platforms.
- Completion should not call the UniPost API or require authentication.
- Human docs should include installation snippets for common shells.

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

### 10.4 Backend Error Mapping

The backend returns `error.normalized_code` and `request_id` on many API errors. CLI should preserve `normalized_code` in JSON output and map it to stable exit codes.

| Backend `normalized_code` | CLI exit code | Default CLI hint |
| --- | ---: | --- |
| `unauthorized` | 4 | Check `UNIPOST_API_KEY`, local auth, or run `unipost auth status`. |
| `forbidden` | 5 | The current credential lacks permission for this action. |
| `validation_error`, `invalid_request` | 6 | Fix request fields; run validate/dry-run before write commands. |
| `not_found` | 6 | Check the resource ID and current workspace/profile context. |
| `account_already_connected` | 6 | Use `accounts list` or choose a different platform/account. |
| `account_disconnected` | 6 | Reconnect the account before publishing or fetching metrics. |
| `not_supported` | 6 | This platform or endpoint does not support the requested operation. |
| `plan_post_quota_exceeded` | 6 | Upgrade plan, reduce publish units, or retry after quota resets. |
| `request_rate_limited`, `enqueue_rate_limited`, `queue_depth_exceeded` | 7 | Respect rate-limit headers and retry later with backoff. |
| `upstream_error` | 7 | UniPost reached the platform but the upstream platform failed. |
| `internal_error` | 7 | Capture `request_id` and retry or contact support. |

If the backend returns an unknown `normalized_code`, CLI should map by HTTP status first, include the raw `normalized_code` in JSON output, and use exit code 7 for 5xx or exit code 6 for 4xx.

### 10.5 Agent Capability Catalog Contract

`agent capabilities --json` is a stable agent contract. It must include a `catalog_version` field and use additive evolution whenever possible.

Contract requirements:

- `catalog_version` identifies the capability catalog schema and intent set version.
- Intent names are stable identifiers; do not rename or remove them within a major CLI version.
- `required_inputs`, `optional_inputs`, `safety_level`, `canonical_actions` and output schema fields are part of the public agent contract.
- New intents, optional inputs and additive fields may be added in minor versions.
- Removing an intent, changing required inputs, changing a safety level to a less restrictive value, or changing action semantics requires a major version or explicit compatibility mode.
- MCP tools, Codex skill/plugin instructions and Claude Code instructions should reference the same intent names and safety levels.

### 10.6 Status Enum Contract

CLI-facing status fields are part of the agent contract. Agents and CI will branch on exact string equality, so the CLI must expose one canonical spelling per state.

Canonical status values:

| Resource | CLI-facing status values |
| --- | --- |
| Post | `draft`, `scheduled`, `publishing`, `published`, `partial`, `failed`, `canceled` |
| Connect session | `pending`, `completed`, `expired`, `canceled` |
| Media | `pending`, `processing`, `ready`, `failed` |

Requirements:

- CLI JSON output must normalize backend aliases before returning status fields that agents branch on.
- Backend `cancelled` must be normalized to CLI `canceled`.
- Human output may use localized labels, but JSON `status` fields must use the canonical values above.
- If CLI exposes raw backend payloads for debugging, the raw status should be nested under a field such as `raw.status` or `status_original`; agents should not branch on raw values.
- Adding a new status value requires a documented minor-version change and capability catalog update. Renaming or removing a status value is a breaking change.

### 10.7 Pagination Contract

List commands must use one predictable pagination model.

Commands covered:

- `accounts list`
- `posts list`
- `analytics posts`
- `analytics platforms` when the backend paginates
- future list/export commands

Flags:

```bash
--limit <n>
--cursor <cursor>
--all
```

Requirements:

- `--limit` controls page size within API-supported bounds.
- `--cursor` requests a specific page cursor.
- `--all` follows pagination until no next page remains or a documented safety cap is reached.
- JSON responses include pagination metadata when available.
- CLI should derive `next_cursor` from response body metadata or the HTTP `Link` header, depending on endpoint behavior.
- Human output should clearly show when more pages exist and print the next command.
- Agent/CI flows should prefer explicit `--limit`/`--cursor` unless the user intentionally asks for all records.

Example:

```json
{
  "ok": true,
  "data": [],
  "warnings": [],
  "meta": {
    "pagination": {
      "limit": 50,
      "next_cursor": "cur_...",
      "has_more": true
    }
  }
}
```

### 10.8 Output, Field Selection, And Localization Contract

Output requirements:

- Default human output should be concise tables or task-oriented summaries.
- `--output json` and `--json` produce the standard JSON envelope.
- `--output yaml` is optional for v1 but useful for config/debug workflows.
- `--field <field_or_jsonpath>` prints a single redacted value for shell scripting and CI.
- Color must be disabled by `--no-color` or `NO_COLOR=1`.

Localization requirements:

- Machine fields are always stable English identifiers: `code`, `normalized_code`, `intent`, `safety_level`, `status`, exit codes, command names and enum values.
- Human-facing strings such as `message`, `hint`, `recommended_prompt` and table labels may be localized.
- CLI locale can follow system locale, `LANG`, or explicit `--lang <locale>`.
- Agents should branch only on stable machine fields and may display localized prompts to users.

---

## 11. Safety, Permissions, And Audit

### 11.1 Publish Guardrails

Default behavior:

- `posts validate` is safe and can run without confirmation.
- `posts draft` is a write action but does not publish to social networks.
- `posts create` that publishes immediately requires confirmation in human mode.
- `posts create --schedule-at` and `posts schedule` require confirmation in human mode because the scheduled job will eventually publish externally.
- `posts draft` does not require `--yes`, including in `--non-interactive` mode.
- `posts create --dry-run` does not require `--yes`.
- `posts create --non-interactive` for live publish requires `--yes` and `--idempotency-key`.
- `posts create --schedule-at --non-interactive` and `posts schedule --non-interactive` require `--yes` and `--idempotency-key`.
- `posts publish-draft --non-interactive` requires `--yes` and `--idempotency-key`.
- Missing `--yes` for non-interactive live publish exits with code 9 (`unsafe action blocked`).
- Missing `--idempotency-key` for non-interactive live publish exits with code 3 (`missing required input`).

Recommended policy:

```text
human interactive publish: confirmation prompt required
human --yes publish: allowed
non-interactive live publish: --yes + --idempotency-key required
non-interactive scheduled publish: --yes + --idempotency-key required
dry-run: allowed without --yes
draft creation: allowed without --yes
```

### 11.2 Destructive Actions

Destructive commands such as account disconnect, webhook delete, profile delete, or API key revoke should not be part of the initial agent beta. Post lifecycle operations have narrower rules:

- `posts cancel` is allowed for draft/scheduled posts when the user provides an explicit post ID and `--yes`.
- `posts retry` is allowed for failed delivery results when the user provides an explicit post ID/result ID and `--yes`.
- Agent usage of cancel/retry should come from real CLI context (`posts get`, `posts list --status failed`, or `posts wait`), not invented IDs.

For broader destructive commands added later:

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

### 11.4 Agent Plan And Execute Guardrails

`agent plan` and `agent execute` must follow a strict trust model.

Plan requirements:

- Plans are advisory artifacts, not permission grants.
- Plan steps must use structured `action` and `args` fields.
- Plan steps may include `display_command` for human readability, but `display_command` is never executable authority.
- `safety_level` in a plan is informational and must not be trusted by `agent execute`.

Execute requirements:

- `agent execute` treats the plan file as untrusted input.
- `agent execute` must recompute each step's safety level from the current capability catalog and internal action registry.
- `agent execute` must never shell-exec or eval a command string from a plan file.
- `agent execute` may run read-only, validate-only and draft-write actions only when required inputs are present and no user choice is outstanding.
- `agent execute` must reject `live_write` steps and return a structured error such as `requires_explicit_publish_command`.
- Live publish must always use explicit publish commands with `--yes` and `--idempotency-key`, not automatic plan execution.

---

## 12. UX Flows

### 12.1 Unified `unipost init` Decision Flow

```bash
unipost init
```

`unipost init` should use one standard decision flow for first setup, repeat setup, Codex/Claude Code reinstall, CLI reinstall and new-machine setup.

Principle:

```text
Find first.
Validate second.
Repair if possible.
Create only when missing, invalid, or explicitly replacing.
```

Decision flow:

```text
Start: unipost init
  |
  v
1. Discover local credential
   - UNIPOST_API_KEY
   - config metadata -> OS keychain / credential store
   - keychain / credential-store scan: service=unipost
   - supported password manager / secret manager, if configured
   - explicit file storage, only if user opted in
  |
  v
2. Is any credential found?
   |
   |-- yes --> 3. Validate credential with auth status / workspace
   |             |
   |             |-- valid --> 4. Repair local config if needed
   |             |              - restore workspace_id
   |             |              - restore key_id/name/prefix
   |             |              - restore default profile if possible
   |             |              - continue doctor/profile/account checks
   |             |              -> configured
   |             |
   |             |-- invalid --> 5. Can recover from another local credential?
   |                            - try next discovered credential
   |                            - otherwise ask for reauth or fallback
   |                            -> if no usable credential, go to 6
   |
   |-- no --> 6. Start authorization
                 - setup token from Dashboard, or
                 - device/browser auth, or
                 - fallback manual UNIPOST_API_KEY
  |
  v
7. User authorizes CLI access
  |
  v
8. Backend creates named, revocable CLI API key
   - name: Codex CLI - MacBook Pro
   - client: codex / claude-code / generic
   - source: cli_bootstrap
  |
  v
9. CLI stores credential locally
   - macOS Keychain
   - Windows Credential Manager
   - Linux Secret Service / libsecret
   - supported password manager / secret manager, if configured
   - CI/headless: env only
   - config stores metadata only
  |
  v
10. Run setup checks
    - workspace
    - profiles
    - accounts
    - base_url
    - rate limits
  |
  v
Done: configured
```

If a valid credential is found, CLI skips setup-token exchange and key creation. It may print metadata:

```text
Using existing CLI key: Codex CLI (up_live_abcd...), verified now.
```

If no valid credential is found, CLI starts auth/key creation only after user authorization. If setup-token/device auth is not available, CLI falls back to `UNIPOST_API_KEY` or `auth login --api-key`.

### 12.2 Init Scenario Decision Table

| Scenario | `unipost init` behavior |
| --- | --- |
| First init on this machine, no credential | Start setup-token/device auth or fallback to `UNIPOST_API_KEY`; create one named CLI API key after user authorization. |
| Codex or Claude Code reinstalled, secure local credential still exists | Discover keychain/credential-store/password-manager credential, validate it, repair config if needed, reuse existing key. |
| UniPost CLI reinstalled, config missing but keychain credential exists | Scan credential store for `service=unipost`, validate credential, rebuild config metadata, reuse existing key. |
| Config exists but keychain secret is missing | Treat metadata as stale, try other discovered credentials, then reauth if none are valid. |
| API key was revoked in Dashboard | Validation fails; prompt reauth/setup-token/device auth or `UNIPOST_API_KEY` fallback. |
| New computer | No local credential exists; run setup-token/device auth and create a new named CLI key for that device. |
| User provides `UNIPOST_API_KEY` | Use env key for this run; do not create a new key by default. |
| User passes setup token but a valid local credential exists | Return `already_configured`; do not exchange token or create a new key unless `--replace-key` is present. |
| User passes `--force` | Rerun discovery/validation/config repair; do not replace a valid credential. |
| User passes `--reauth` or `--replace-key` | Start authorization and create a replacement key after explicit confirmation. |

Replacement flow:

```bash
unipost init --reauth
unipost auth login --replace-key
```

Expected behavior:

1. CLI detects existing credential metadata.
2. CLI asks for explicit confirmation before replacing the local credential.
3. CLI starts device/setup-token auth.
4. Backend creates a new named API key only after user authorization.
5. CLI stores the new key locally.
6. CLI asks whether to revoke the old key when remote revocation is available; otherwise it links to Dashboard -> API Keys.

If a user passes a new setup token while already configured:

```bash
unipost agent bootstrap --client codex --setup-token ust_...
```

Expected behavior:

- If the existing credential is valid and `--replace-key` is not present, CLI returns `already_configured`, does not exchange the setup token, and does not create a new API key.
- If `--replace-key` is present, CLI may exchange the setup token and replace the local credential after explicit confirmation.

### 12.3 Decline, Failure, And Retry

Onboarding must be resumable.

If the user declines API-key creation:

- CLI writes no destructive state.
- CLI may record only non-sensitive local status such as `last_bootstrap_status: "user_declined_key_creation"`.
- Dashboard should continue showing the AI tools / CLI connect entry point.
- User can rerun `unipost init` or `unipost agent bootstrap` later.

If API-key creation fails:

- CLI returns a structured error with `request_id` when available.
- CLI explains the retry path.
- CLI does not mark setup as permanently failed.

Common failure statuses:

```text
setup_token_expired       -> generate a new setup token or restart device auth
setup_token_used          -> generate a new setup token
permission_denied         -> ask a workspace owner/admin to authorize setup
api_key_quota_exceeded    -> revoke an old key or use an existing valid key
keychain_unavailable      -> use UNIPOST_API_KEY or explicit file storage in interactive mode
network_error             -> retry when connectivity is restored
internal_error            -> retry or contact support with request_id
```

State model:

```text
not_configured
needs_user_auth
auth_approved
key_creation_declined
key_creation_failed
key_created
configured
```

The setup token is single-use, but the init/bootstrap process is not. Used or expired setup tokens require a new setup token or a fresh device-auth flow.

### 12.4 First Connected Account

```bash
unipost connect create --platform linkedin --profile pr_...
```

Expected flow:

1. CLI creates connect session or OAuth URL.
2. CLI prints hosted URL.
3. User opens URL and completes OAuth.
4. User runs:

```bash
unipost connect wait cs_... --timeout 300 --json
```

5. CLI returns `completed_social_account_id` when the session completes.
6. CLI shows account ID and next validate command.

### 12.5 First Safe Post

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

### 12.6 Post-Signup Agent-Assisted Onboarding

After signup, Dashboard should offer:

```text
Connect with Claude Code / Codex
```

Expected Dashboard flow:

1. User signs up or logs in to UniPost.
2. User opens Dashboard setup card for AI agents.
3. User selects a client: Codex, Claude Code, Cursor, Windsurf, or generic terminal.
4. Dashboard generates a short-lived setup token or device-auth instruction.
5. Dashboard shows a one-line command and a copyable agent prompt.
6. User pastes the prompt into Codex / Claude Code.
7. Agent runs `unipost agent bootstrap --client ... --json`.
8. CLI authenticates, runs diagnostics, fetches context and returns next actions.
9. Agent asks the user only for unresolved decisions.
10. Agent continues until `doctor` and `agent context` both pass.

Example command shown in Dashboard:

```bash
unipost agent bootstrap --client codex --setup-token ust_...
```

Example prompt shown in Dashboard:

```text
请帮我接入 UniPost API。先确认本地已安装 UniPost CLI，然后运行 unipost agent bootstrap --client codex，根据 CLI 输出继续操作。如果 CLI 需要我选择 profile、平台或账号，请用白话问我确认。
```

The setup token must be short-lived, single-use and scoped to onboarding. If setup-token auth is not available, Dashboard should show the device auth flow instead:

```bash
unipost agent bootstrap --client codex
```

The CLI then returns an activation URL and short code for the user to approve in browser.

### 12.7 Natural-Language Agent Setup

Target user experience:

```text
User: 帮我把我的项目接入 UniPost API，我想先支持 LinkedIn 发帖。
Agent: 我先检查 UniPost CLI 和你的 workspace 状态。
```

Agent runs:

```bash
unipost agent bootstrap --client codex --json
unipost agent capabilities --client codex --json
unipost agent guide --client codex
unipost doctor --json
unipost agent context --json
```

The agent should use `agent capabilities` and `agent guide` to map the user's natural-language request to a supported UniPost intent. The expected reasoning path is:

1. User speaks naturally.
2. Codex / Claude Code identifies the likely UniPost intent from `agent capabilities`.
3. Agent asks the user for clarification only if the intent or required inputs are ambiguous.
4. Agent calls `agent plan --intent ... --json` with structured inputs.
5. Agent executes only the safe steps allowed by the returned plan.
6. Agent stops for explicit confirmation before live publish, destructive actions or credential replacement.

If auth is missing, CLI returns:

```json
{
  "ok": true,
  "data": {
    "status": "needs_user_auth",
    "activation_url": "https://unipost.dev/cli/activate",
    "user_code": "ABCD-EFGH",
    "recommended_prompt": "UniPost 需要你授权本机 CLI。请打开链接并输入验证码 ABCD-EFGH。"
  },
  "warnings": [],
  "meta": {
    "command": "agent bootstrap",
    "source": "cli"
  }
}
```

If no LinkedIn account exists, the agent asks:

```text
你的 UniPost workspace 里还没有 LinkedIn 账号。要我现在创建 LinkedIn 连接链接吗？
```

After user confirmation, agent runs:

```bash
unipost connect create --platform linkedin --open --json
unipost connect wait cs_... --timeout 300 --json
```

After account connection succeeds, agent runs:

```bash
unipost posts validate --account sa_... --caption "Test from UniPost" --json
unipost agent plan --intent generate_integration_example --json
unipost examples posts.create --lang node --account sa_... --json
```

The agent can then modify the user's project code to add native HTTP/fetch usage, SDK usage when the package is published, `.env` examples and a first API call. The CLI should provide enough context and command guidance that the agent does not need to browse UniPost docs unless the user asks for deeper explanation.

### 12.8 Agent Intent Planning

Target user experience:

```text
User: 帮我给 LinkedIn 创建一条草稿，文案是 "Shipping with UniPost"。
Agent: 我会先用 UniPost 的能力目录确认安全流程，然后创建 draft，不会直接发布。
```

Agent runs:

```bash
unipost agent capabilities --client codex --json
unipost agent context --json --non-interactive
unipost agent plan --intent create_draft_post --from-file request.json --json --non-interactive
```

If the plan is safe to execute without more user input, the Phase 3 agent should use the structured `action` and `args` fields to call the corresponding explicit CLI command or MCP tool. It must not blindly copy and execute `display_command`.

If the optional `agent execute` beta is enabled after security review, the agent can run:

```bash
unipost agent execute --plan plan.json --json --non-interactive --agent-name codex   # Phase 5 only
```

This remains outside the Phase 3 acceptance path; Phase 3 agents should call explicit CLI commands or MCP tools after reading the structured plan.

If the user's request is ambiguous, `agent plan` returns missing inputs:

```json
{
  "ok": true,
  "data": {
    "intent": "create_draft_post",
    "status": "needs_user_choice",
    "safe_to_execute_without_user": false,
    "missing_inputs": [
      {
        "name": "account_id",
        "reason": "Multiple LinkedIn accounts are available.",
        "choices": ["sa_123", "sa_456"],
        "recommended_prompt": "你想用哪个 LinkedIn 账号创建草稿？"
      }
    ],
    "steps": []
  },
  "warnings": [],
  "meta": {
    "command": "agent plan",
    "source": "cli"
  }
}
```

Agent behavior requirements:

- Do not invent account IDs, profile IDs, media IDs or post IDs.
- Do not parse human-readable command output when `--json` is available.
- Do not execute live publish from an `agent plan`; after explicit user confirmation, use the normal `posts create --yes --idempotency-key`, `posts schedule --yes --idempotency-key`, or `posts publish-draft --yes --idempotency-key` command.
- Prefer `posts validate` and `posts draft` over live publish.
- If `agent capabilities` does not include a requested intent, tell the user the action is unsupported and suggest the closest safe action.

### 12.9 Agent Context Grounding

```bash
unipost agent context --json --non-interactive
```

Expected flow:

1. CLI validates auth.
2. CLI fetches workspace, profiles, accounts and recent post summary.
3. CLI returns JSON with recommended next commands.
4. Agent uses account IDs from actual context instead of invented IDs.

### 12.10 Agent Safe Publish

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
- Live or scheduled publish is blocked unless confirmation requirements are satisfied.
- Response includes post ID, result IDs, status and request ID.
- Agent/CI can call `posts wait post_... --timeout 120 --json` after create/publish to observe terminal per-platform delivery state instead of guessing whether the asynchronous publish finished.

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
- `--json`, `--output`, `--field`, `--non-interactive`, error envelope and exit codes.
- Standard pagination flags for list commands.
- Standard network retry/backoff behavior, including `Retry-After` handling.
- Telemetry opt-out and first-run privacy notice.
- Shell completion generation for bash/zsh/fish.
- `unipost doctor` with auth/workspace/API reachability checks.

Success criteria:

- A user with `UNIPOST_API_KEY` can verify API access without reading docs.
- CI can fail reliably on missing/invalid auth.

### Phase 2: Developer Quickstart GA

Deliverables:

- `unipost init`.
- `unipost quickstart`.
- `agent bootstrap --client codex`.
- `agent bootstrap --client claude-code`.
- `agent capabilities --json` with the first supported intent catalog.
- `agent guide --client codex|claude-code`.
- `agent context --json` with read-only workspace/account grounding.
- `agent mcp-config claude-code`.
- `agent mcp-config codex`.
- `profiles list/create/use`.
- `auth list/use` for local multi-workspace credentials.
- `connect create/get/wait`.
- `accounts list/get`.
- `posts validate`.
- `posts draft`.
- `examples posts.create` with cURL and native Node.js `fetch` support that does not depend on published SDK packages.

Success criteria:

- New developer can create or select profile, connect account, find account ID and create first draft from terminal.
- With `UNIPOST_API_KEY` fallback, Codex / Claude Code can run `agent bootstrap`, diagnose setup state and discover workspace/profile/account context.
- With setup-token/device auth backend implemented, a new user can paste a Dashboard-generated prompt into Codex / Claude Code and let the agent complete CLI/API setup without manual API-key copy/paste.
- Quickstart does not require live publish.
- Docs page provides complete install and first-run flow.
- Agent bootstrap can diagnose missing auth, missing profile and missing connected account without requiring the agent to browse docs.
- Agent can ask the CLI for supported intents and client guidance instead of inferring command syntax from docs.

### Phase 3: AI Agent Operator Beta

Deliverables:

- Stable `--json` output for accounts/posts/media/analytics.
- `agent plan --intent ...`.
- `agent plan-publish` as an alias for `agent plan --intent plan_publish_post`.
- `posts create --from-file --dry-run`.
- `posts create --schedule-at` and `posts schedule`.
- `posts wait`.
- `posts cancel` for draft/scheduled posts.
- `posts retry --result` for failed per-platform deliveries.
- Agent publish guardrails with `--yes` and `--idempotency-key`.
- Audit metadata for write commands.

Success criteria:

- Codex / Claude Code can inspect real UniPost account context without browsing docs.
- Agent can safely dry-run publish payloads.
- Agent can schedule approved content and wait for post terminal state with structured per-platform results.
- Agent can cancel or retry eligible post deliveries only when using explicit IDs and confirmation flags.
- Agent can turn a supported intent into a structured plan with missing inputs, required confirmations and canonical actions.
- Agent can use the plan to call normal CLI commands or MCP tools without guessing command syntax.
- Live publish is impossible by accident in non-interactive mode.
- Agent can move from read-only setup into draft/publish workflows only after explicit user confirmation.

### Phase 4: Advanced Ops And Diagnostics

Deliverables:

- `accounts health`.
- `accounts capabilities`.
- `accounts metrics`.
- `posts list --status failed`.
- `analytics summary/posts/platforms`.
- `analytics export` stays post-full-V1 unless pulled forward explicitly.
- `media upload/get/wait`.
- webhook diagnostics if demand is confirmed.

Success criteria:

- Support can diagnose common account and post delivery issues from CLI.
- Users can upload local media and publish with `media_id`.
- Analytics can be consumed by agents and CI workflows.

### Phase 5: MCP Bridge And Agent Ecosystem

Deliverables:

- `examples mcp.claude-code`.
- `agent mcp-config claude-code`.
- `agent mcp-config cursor`.
- MCP auth test command.
- First-party MCP server or MCP wrapper package that mirrors the CLI agent intents.
- Codex skill/plugin package with UniPost instructions, capability usage and safe publish rules.
- Claude Code instruction package with equivalent guidance.
- `agent install --client codex|claude-code` or equivalent generated setup instructions.
- Optional `agent execute --plan plan.json` beta after security review, limited to structured read-only, validate-only and draft-write actions.
- Optional Homebrew distribution.

Success criteria:

- CLI can bootstrap MCP setup without replacing MCP.
- MCP tools and CLI agent wrapper use the same intent names and safety model.
- Codex / Claude Code can call UniPost through installed instructions or MCP tools without guessing terminal syntax.
- If `agent execute` is enabled, it rejects `live_write` steps and never executes raw command strings from plan files.
- Agent users can choose CLI, MCP or client-specific instruction packages based on workflow.

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
- Number of `agent capabilities` and `agent guide` runs by client.
- Number of `agent plan` runs by intent and outcome status.
- Number of `agent execute` runs by safety level, if `agent execute` is enabled.
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
- A first `unipost init` can create/store a named CLI API key after user authorization when setup-token/device auth backend exists.
- Re-running `unipost init` with a valid local credential reuses the existing credential and does not create a new API key.
- `unipost init --reauth` or `auth login --replace-key` can intentionally replace the local credential after explicit confirmation.
- If key creation is declined or fails, the user can retry init/bootstrap later without destructive state.
- A user can list profiles and accounts.
- A user can create a connect URL/session for a supported platform.
- A user can run `connect wait` and receive the completed social account ID after OAuth.
- A user can validate a post without publishing.
- A user can create a draft post.
- A user can schedule a post after explicit confirmation and idempotency key.
- A user can wait for a post to reach terminal delivery state.
- A user can cancel an eligible draft/scheduled post after explicit confirmation.
- A user can retry a failed per-platform delivery after explicit confirmation.
- A user can publish an existing draft through `posts publish-draft` after explicit confirmation.
- A user can generate at least cURL and native Node.js `fetch` examples using their real account ID.
- SDK-backed examples are available only for SDK packages that are published and installable.
- Missing API key produces a clear hint and docs URL.

### 15.2 AI Agent Operator

- `agent bootstrap --client codex --json` returns a clear setup status and next actions.
- `agent bootstrap --client claude-code --json` returns client-specific setup instructions.
- `agent capabilities --json` returns supported intent names, schemas, safety levels and canonical action mappings.
- `agent guide --client codex|claude-code` returns client-specific operating guidance without secrets.
- `agent plan --intent ... --json` returns a structured plan, missing inputs and required confirmations without executing writes.
- If `agent execute --plan ...` is enabled, it treats plan files as untrusted input, runs only structured safe actions, and cannot bypass live-publish guardrails.
- Before setup-token/device auth exists, `agent bootstrap` supports the `UNIPOST_API_KEY` fallback path.
- After setup-token/device auth exists, Dashboard-generated setup tokens can be exchanged without exposing long-lived API keys to the agent.
- When auth, profile or connected account is missing, bootstrap returns a recommended plain-language prompt for the agent to ask the user.
- Every agent-relevant read command supports `--json`.
- JSON output uses the standard envelope.
- Error output uses the standard error envelope.
- `agent context --json` returns workspace, profiles, accounts and recent post summary.
- `connect wait --json` lets agents observe OAuth completion and receive `completed_social_account_id`.
- `posts wait --json` lets agents observe asynchronous post delivery state and per-platform results.
- Non-interactive live publish fails without `--yes`.
- Non-interactive live publish fails without `--idempotency-key`.
- Non-interactive scheduled publish fails without `--yes` and `--idempotency-key`.
- Dry-run publish returns a validation result and plan without publishing.
- Post cancel/retry commands require explicit IDs and `--yes`.
- Write commands include source metadata only after the backend accepts those fields.
- Agent can map common user requests to supported UniPost intents by using `agent capabilities` and `agent guide`, without browsing docs or guessing terminal syntax.

### 15.3 Agent-Assisted Onboarding

- A newly registered user can see a "Connect with Claude Code / Codex" entry point in Dashboard.
- Dashboard can produce either a setup-token command or device-auth command for the selected client.
- User can paste the generated prompt into Codex / Claude Code and have the agent continue without reading UniPost docs.
- Agent can use CLI output to ask clear user-confirmation questions when multiple profiles/accounts/platforms are possible.
- Agent can complete auth check, workspace check, account discovery and first post validation through CLI.
- Agent can use the capabilities catalog and intent planner to decide whether a user request is supported, missing inputs, or blocked by safety policy.
- The flow avoids asking the user to paste long-lived API keys into agent chat by default.
- The flow ends with a generated cURL/native HTTP example, SDK example when available, or project code change using real UniPost IDs.

### 15.4 Reliability And Security

- CLI redacts API keys in logs and output.
- CLI never prints OAuth tokens.
- CLI redacts setup tokens after exchange.
- CLI stores created API keys in OS keychain by default and stores only metadata in config.
- CLI fails safely in non-interactive mode when keychain storage is unavailable.
- CLI surfaces UniPost `request_id` when available.
- Exit codes match the documented contract.
- Pagination, output formatting, color disabling and field selection behave consistently across list/read commands.
- CLI-facing status fields match the documented canonical enums.
- CLI retries only safe/idempotent requests automatically and respects `Retry-After`.
- Telemetry can be disabled with config, environment variable, or per-command flag.
- Base URL override works for local/dev/staging validation.
- CI can use CLI without prompts by passing `--non-interactive`.

---

## 16. Dependencies

API dependencies:

- API key auth through existing UniPost public API.
- Workspace endpoint.
- Profiles endpoints.
- Accounts list/health/capabilities/metrics endpoints.
- Connect session or OAuth connect endpoint.
- Connect session get endpoint that returns status, `completed_social_account_id` and `completed_at`.
- Posts validate/create/list/get/publish-draft endpoints; draft creation uses `POST /v1/posts` with `status: "draft"`.
- Posts scheduling through top-level `scheduled_at` on create/validate payloads.
- Post lifecycle endpoints for canceling eligible posts and retrying failed per-platform results.
- Post delivery job retry/cancel endpoints when direct job-level support workflows are pulled into CLI.
- Media reserve/upload/get endpoints.
- Analytics summary/posts/platforms endpoints, plus export when `analytics export` is included.
- Pagination signals for list endpoints, including `Link` header or response metadata.
- Rate-limit headers and `Retry-After` for retry/backoff behavior.

New hard backend dependencies for full agent-assisted onboarding:

- Browser/device auth endpoint for CLI login.
- Short-lived, single-use setup token issuance from Dashboard.
- Setup token exchange endpoint that creates a named, revocable CLI API key and returns the plaintext key once to the local CLI.
- Backend support for generated key metadata such as key name, client type and source.
- Backend error codes for setup token expired/used, user declined, permission denied, API key quota exceeded, and key creation failure.

Backend behavior that improves CLI quality:

- Consistent `request_id` in responses.
- Consistent `error.normalized_code`.
- Rate limit headers.
- Idempotency key handling for create post.
- Optional request metadata field for CLI/agent audit; current backend acceptance must be confirmed before CLI sends these fields.
- Draft creation path that does not publish.
- Validation endpoint that catches platform-specific constraints.
- Minimum supported CLI version signal so `doctor` can warn users when a CLI upgrade is required.

Docs dependencies:

- CLI docs page.
- Quickstart docs.
- MCP docs.
- API reference.
- SDK docs.
- Published SDK package docs only when SDK-backed examples are enabled.

Agent ecosystem dependencies:

- Stable CLI agent capability catalog and intent names before MCP/tool packages mirror them.
- First-party MCP server or wrapper package when Phase 5 is started.
- Codex skill/plugin packaging and installation path when Phase 5 is started.
- Claude Code instruction package format and installation path when Phase 5 is started.
- Client-specific setup documentation for Codex, Claude Code, Cursor and Windsurf.

These ecosystem dependencies should not block Phase 1 or Phase 2. They become launch dependencies only for Phase 5 or for any release that claims first-party MCP / Codex plugin / Claude Code package support.

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
- Treat scheduled publish as live publish because it eventually posts externally.
- Keep broad destructive commands such as account disconnect, API key revoke and profile delete out of early agent beta.

### Risk: Scheduled posts, retries, or cancels create unexpected external effects

Mitigation:

- Require explicit resource IDs and `--yes` for cancel/retry.
- Require `--yes` and `--idempotency-key` for scheduled publish in non-interactive mode.
- Make `posts wait` available so agents can observe terminal status instead of issuing repeated retries blindly.
- Surface per-platform result IDs and statuses in JSON output.
- Respect rate-limit and queue-depth signals before retrying.

### Risk: Pagination or `--all` causes runaway API usage

Mitigation:

- Use bounded page sizes and a documented maximum for `--all`.
- Show `next_cursor` and recommended next commands in human output.
- Prefer explicit cursor pagination in agent/CI flows.
- Respect `Retry-After` and rate-limit headers during pagination.

### Risk: JSON contract changes break agents

Mitigation:

- Version the CLI.
- Keep envelope stable.
- Add fields without removing existing fields.
- Version `agent capabilities` with `catalog_version`.
- Keep intent names, safety-level enums and capability schema stable within a major version.
- Add intents and optional inputs additively; remove or weaken safety only in a major version or explicit compatibility mode.
- Reserve breaking changes for major version.

### Risk: Secrets leak in diagnostics

Mitigation:

- Redact API keys and tokens.
- Avoid printing full presigned URLs unless the command explicitly requires it.
- Add automated redaction tests.

### Risk: Telemetry or localized output breaks trust or agent parsing

Mitigation:

- Provide first-run telemetry notice and deterministic opt-out.
- Keep machine fields stable English identifiers.
- Localize only human-facing strings such as messages, hints and prompts.
- Redact sensitive data before telemetry is emitted.

### Risk: Setup tokens become a new credential leak path

Mitigation:

- Make setup tokens short-lived and single-use.
- Scope setup tokens to CLI bootstrap only.
- Allow setup tokens only on the dedicated exchange endpoint that creates the one named CLI key; do not allow setup tokens to publish, list keys, revoke keys, create arbitrary keys, or delete resources.
- Redact setup tokens in CLI output, telemetry and logs after exchange.
- Prefer browser/device confirmation before issuing durable local credentials.

### Risk: Agent asks confusing or unsafe follow-up questions

Mitigation:

- Have `agent bootstrap` return `recommended_prompt` and `safe_to_continue_without_user`.
- Have `agent capabilities` and `agent plan` return required inputs, supported choices and recommended clarification prompts.
- Treat missing profile/account/platform choices as explicit user-confirmation points.
- Keep live publish out of bootstrap.
- Keep live publish out of automatic `agent execute` unless all publish guardrails and user confirmation requirements are satisfied.
- Document expected agent phrasing in CLI docs and Dashboard prompt templates.

### Risk: CLI conflicts with MCP positioning

Mitigation:

- Position CLI as terminal/script interface and MCP as agent-native protocol.
- Let CLI generate and test MCP configs.
- Use the same conceptual tool names where possible.
- Use the same intent names and safety levels across CLI agent wrapper, MCP tools, Codex skill/plugin and Claude Code instructions.

### Risk: Agent wrapper becomes an unreliable natural-language parser

Mitigation:

- Do not make free-form natural-language parsing the required path for v1.
- Let Codex / Claude Code interpret user language, then pass explicit `--intent` and structured inputs to the CLI.
- Keep `agent capabilities` as the source of truth for supported intents and schemas.
- If a future `agent plan "natural language"` shortcut is added, treat it as best-effort convenience and return `needs_user_choice` rather than guessing.

---

## 18. Documentation Requirements

The CLI docs page should move from "Coming soon" to a concrete guide with:

- installation
- auth
- quickstart
- command reference
- agent mode
- agent capabilities, guide, planning and execution flow
- JSON output contract
- status enum contract
- pagination and output formatting contract
- networking, retry and proxy behavior
- telemetry and privacy controls
- publish safety rules
- scheduling, wait, cancel and retry workflows
- examples
- MCP bridge
- Codex skill/plugin and Claude Code instruction package setup once available
- troubleshooting

Docs should explicitly explain:

- CLI helps developers avoid frequent docs lookup during first integration.
- CLI helps AI agents avoid browsing UniPost webpages for account state.
- Registered users can start from Dashboard by choosing "Connect with Claude Code / Codex" and pasting a generated prompt into their agent.
- CLI does not replace SDK, API or MCP.
- CLI defaults to validation/draft before live publish.
- Agent-assisted onboarding should use setup token or device auth by default, not manual long-lived API key sharing.
- After user authorization, UniPost should create a named, revocable CLI API key automatically and return it once to the local CLI for secure storage.
- `unipost init` is safe to rerun: valid existing credentials are reused, and new keys are created only on first setup, missing/invalid credentials, or explicit replacement.
- Created keys are stored in OS keychain by default; config stores only key metadata.
- AI agents should use `agent capabilities`, `agent guide`, `agent context`, `agent plan`, and later `agent execute` when enabled, instead of guessing raw terminal commands.
- MCP, Codex skill/plugin and Claude Code instruction packages should mirror the CLI intent names and safety model.
- Scheduled publish is a live-write action and requires the same non-interactive guardrails as immediate publish.
- Wait commands (`connect wait`, `posts wait`, `media wait`) are the supported agent/CI way to observe asynchronous workflows.
- Machine-readable fields stay stable English even when human messages are localized.
- CLI JSON normalizes backend status aliases to canonical values, especially `canceled`.

---

## 19. Recommended V1 Command Set

Full V1 command set delivered across phases:

```text
unipost --version
unipost completion
unipost auth status
unipost auth login
unipost auth list
unipost auth use
unipost auth logout
unipost config show
unipost config path
unipost doctor
unipost init
unipost quickstart
unipost profiles list
unipost profiles create
unipost profiles use
unipost connect create
unipost connect get
unipost connect wait
unipost accounts list
unipost accounts get
unipost accounts health
unipost accounts capabilities
unipost accounts metrics
unipost posts validate
unipost posts draft
unipost posts create
unipost posts schedule
unipost posts publish-draft
unipost posts wait
unipost posts cancel
unipost posts retry
unipost posts list
unipost posts get
unipost posts analytics
unipost media upload
unipost media get
unipost media wait
unipost analytics summary
unipost analytics posts
unipost analytics platforms
unipost analytics platform
unipost examples posts.create
unipost examples mcp.claude-code
unipost agent bootstrap
unipost agent capabilities
unipost agent guide
unipost agent context
unipost agent plan
unipost agent plan-publish
unipost agent mcp-config
```

Commands that can wait until after full V1 GA:

```text
unipost accounts disconnect
unipost profiles delete
unipost webhooks create/update/delete
unipost api-keys create/revoke
unipost analytics export
unipost posts update/delete
unipost posts jobs retry/cancel
```

Ecosystem deliverables that can wait until Phase 5 or after full V1 GA:

```text
unipost agent execute
unipost agent install
@unipost/mcp or equivalent first-party MCP package
Codex skill/plugin package
Claude Code instruction package
```

---

## 20. Product Decisions

Recommended decisions for first implementation:

- Use one binary: `unipost`.
- Use npm package first: `@unipost/cli`.
- Keep Homebrew as a later distribution channel.
- Use `UNIPOST_API_KEY` as the clearest first fallback auth path.
- Use browser/device auth or Dashboard setup tokens as the default agent-assisted onboarding auth path once backend support exists.
- Add optional local credential storage after the basic flow works, with keychain preferred over file storage.
- Repeat `init` should reuse an existing valid credential; create a new key only when no valid credential exists or the user explicitly requests replacement.
- Default first post path to `validate` then `draft`, not live publish.
- Treat scheduled posts as live-write operations because they eventually publish externally.
- Add `connect wait`, `posts wait` and `media wait` as first-class async primitives for agents and CI.
- Support post lifecycle operations for schedule/cancel/retry only with explicit IDs and confirmation guardrails.
- Support `--json` on all read commands from the beginning.
- Support standard pagination and field selection early, so agents do not scrape tables or guess pages.
- Keep machine fields stable English; localize only human-facing strings.
- Normalize CLI-facing status fields to the documented enum values, including `canceled` instead of backend aliases.
- Respect `Retry-After` and do not retry unsafe writes without an idempotency key.
- Add `agent bootstrap` and `agent context` early because they are the highest-value AI-agent onboarding primitives.
- Add `agent capabilities` and `agent guide` early so agents can discover supported intents without reading docs.
- Use `agent plan --intent ...` as the primary AI-agent planning wrapper; keep `agent plan-publish` as a publish-specific alias.
- Let Codex / Claude Code interpret natural language, then pass explicit intent and structured inputs to UniPost CLI.
- Defer `agent execute` to Phase 5 or a post-Phase-3 security review. Phase 3 should provide plan-only guidance and let agents execute normal CLI commands or MCP tools explicitly.
- If `agent execute` ships, treat it as a safe structured-action runner, not a permission bypass.
- Require `--yes` and `--idempotency-key` for non-interactive live publish.
- Keep MCP separate but let CLI generate MCP config snippets and mirror CLI agent intents in MCP tools.
- Defer first-party Codex skill/plugin and Claude Code instruction package to the ecosystem phase unless pulled forward for launch.

---

## 21. Definition Of Done For The CLI Product

The CLI product is considered successfully launched when:

1. A new developer can install it and complete Quickstart without reading the full API docs.
2. The same developer can generate a usable cURL/native HTTP example containing their real profile/account IDs, plus SDK examples where published packages exist.
3. An AI coding agent can run `agent context --json` and get enough data to choose real accounts instead of inventing IDs.
4. An AI coding agent can run `agent capabilities --json` and `agent guide --client ...` to understand supported UniPost intents without browsing docs.
5. An AI coding agent can run `agent plan --intent ... --json` and receive missing inputs, required confirmations and canonical safe actions.
6. An AI coding agent can dry-run a post and receive a structured publish plan.
7. A developer or agent can schedule approved content, wait for terminal post state and inspect per-platform results.
8. Live or scheduled publish cannot happen accidentally in non-interactive agent usage.
9. Support can ask users to run `unipost doctor --json` and receive safe diagnostic output.
10. CLI docs clearly explain when to use CLI, SDK, raw API, MCP and client-specific agent instruction packages.
11. Full agent-assisted onboarding can create/store a named, revocable CLI API key through setup-token or device auth without manual API-key copy/paste.
