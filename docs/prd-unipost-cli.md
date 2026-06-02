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
8. 让注册后的用户可以把 UniPost 直接交给 Codex / Claude Code 等 agent 接入：用户用白话提出目标，agent 通过 CLI bootstrap、结构化诊断和必要的用户确认完成 API 接入。

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
unipost agent bootstrap --client codex --json
unipost agent bootstrap --client claude-code --json
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
npx -y @unipost/cli agent bootstrap --client codex
```

Recommended prompt:

```text
请帮我接入 UniPost。运行 npx -y @unipost/cli agent bootstrap --client codex，然后根据 CLI 输出继续操作。
```

The product goal is that the user can speak naturally in Codex / Claude Code:

```text
帮我把我的项目接入 UniPost API，我想先支持 LinkedIn 发帖。
```

The agent should then use the CLI to discover real account state, ask the user only when ambiguity remains, and complete setup steps without requiring the user to search UniPost docs manually.

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
--setup-token <token>
--profile <profile_id>
--account <account_id>
--client <codex|claude-code|cursor|windsurf>
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
- `--setup-token`: passes a short-lived Dashboard-generated setup token for bootstrap only.
- `--client`: tells bootstrap/config commands which agent or IDE should receive tailored instructions.
- `--agent-name`: records the calling agent in CLI metadata when included.
- `--open`: open a generated Dashboard, docs, OAuth or connect URL in the user's browser when the command supports it.

`--open` behavior:

- In interactive terminals, `--open` may launch the user's browser.
- In `--non-interactive` mode, `--open` must not block setup. CLI should return the URL in output and include a warning that browser launch was skipped.
- In headless or unsupported environments, CLI should return the URL and a `browser_open_unavailable` warning instead of failing the command.

### 9.2 Auth Commands

```bash
unipost auth status
unipost auth login --api-key up_live_...
unipost auth login
unipost auth logout
```

Requirements:

- `auth status` verifies that a credential exists and can authenticate.
- `auth login --api-key ...` stores credentials according to user-selected storage mode.
- `auth login` without `--api-key` starts browser/device auth once the backend exists; until then it should explain that `UNIPOST_API_KEY` or `--api-key` is required.
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
8. generate SDK/cURL example using real IDs

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
```

Requirements:

- `connect create` returns a hosted URL and the session ID.
- Human output clearly says to open the URL in a browser.
- Optional `--open` may launch browser when user explicitly asks.
- `--json` output includes URL, session ID, platform, profile ID and expiration if available.

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
unipost posts create --from-file post.json --dry-run
unipost posts publish-draft post_... --yes --idempotency-key demo-002
unipost posts list
unipost posts list --status failed
unipost posts get post_...
unipost posts analytics post_...
```

Requirements:

- `validate` should be the safest first publish-adjacent command.
- `draft` creates a server-side draft by calling `POST /v1/posts` with `status: "draft"`.
- `publish-draft` publishes an existing draft by calling `POST /v1/posts/{id}/publish`.
- Updating an existing draft maps to `PATCH /v1/posts/{id}` and can be added as `posts update-draft` when needed.
- `create` can publish or schedule according to request body.
- `draft` does not require `--yes` in either interactive or non-interactive mode because it does not publish to external social networks.
- Live publish means `posts create` without `status: "draft"` and without `--dry-run`, or `posts publish-draft`.
- Live publish in non-interactive mode must include `--yes` and `--idempotency-key`; otherwise CLI exits with code 3 (`missing required input`).
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
unipost analytics export --from 2026-06-01 --to 2026-06-30 --format csv
```

Requirements:

- Analytics commands are important for agents because they let agents reason from real performance data.
- `analytics platforms` maps to `GET /v1/analytics/platforms`.
- `analytics platform <platform>` maps to `GET /v1/analytics/platforms/{platform}`.
- Legacy/internal `GET /v1/analytics/by-platform` can remain outside the CLI v1 surface unless a later implementation plan needs it.
- Human output can summarize key metrics.
- JSON output preserves full API response.
- `analytics export` is Phase 4 / post-GA unless pulled forward explicitly. Export may write a file when user provides `--output`; otherwise it prints to stdout.

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
unipost agent context --json
unipost agent plan-publish --from-file post.json --json
unipost agent mcp-config codex --json
unipost agent mcp-config claude-code --json
```

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

---

## 11. Safety, Permissions, And Audit

### 11.1 Publish Guardrails

Default behavior:

- `posts validate` is safe and can run without confirmation.
- `posts draft` is a write action but does not publish to social networks.
- `posts create` that publishes immediately requires confirmation in human mode.
- `posts draft` does not require `--yes`, including in `--non-interactive` mode.
- `posts create --dry-run` does not require `--yes`.
- `posts create --non-interactive` for live publish requires `--yes` and `--idempotency-key`.
- `posts publish-draft --non-interactive` requires `--yes` and `--idempotency-key`.
- Missing `--yes` for non-interactive live publish exits with code 9 (`unsafe action blocked`).
- Missing `--idempotency-key` for non-interactive live publish exits with code 3 (`missing required input`).

Recommended policy:

```text
human interactive publish: confirmation prompt required
human --yes publish: allowed
non-interactive live publish: --yes + --idempotency-key required
dry-run: allowed without --yes
draft creation: allowed without --yes
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
unipost accounts list --platform linkedin
```

5. CLI shows account ID and next validate command.

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
npx -y @unipost/cli agent bootstrap --client codex --setup-token ust_...
```

Example prompt shown in Dashboard:

```text
请帮我接入 UniPost API。运行 npx -y @unipost/cli agent bootstrap --client codex，然后根据 CLI 输出继续操作。如果 CLI 需要我选择 profile、平台或账号，请用白话问我确认。
```

The setup token must be short-lived, single-use and scoped to onboarding. If setup-token auth is not available, Dashboard should show the device auth flow instead:

```bash
npx -y @unipost/cli agent bootstrap --client codex
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
npx -y @unipost/cli agent bootstrap --client codex --json
unipost doctor --json
unipost agent context --json
```

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
unipost accounts list --platform linkedin --json
```

After account connection succeeds, agent runs:

```bash
unipost posts validate --account sa_... --caption "Test from UniPost" --json
unipost examples posts.create --lang node --account sa_... --json
```

The agent can then modify the user's project code to add SDK usage, `.env` examples and a first API call. The CLI should provide enough context that the agent does not need to browse UniPost docs unless the user asks for deeper explanation.

### 12.8 Agent Context Grounding

```bash
unipost agent context --json --non-interactive
```

Expected flow:

1. CLI validates auth.
2. CLI fetches workspace, profiles, accounts and recent post summary.
3. CLI returns JSON with recommended next commands.
4. Agent uses account IDs from actual context instead of invented IDs.

### 12.9 Agent Safe Publish

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
- `agent bootstrap --client codex`.
- `agent bootstrap --client claude-code`.
- `agent context --json` with read-only workspace/account grounding.
- `agent mcp-config claude-code`.
- `agent mcp-config codex`.
- `profiles list/create/use`.
- `connect create/get`.
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

### Phase 3: AI Agent Operator Beta

Deliverables:

- Stable `--json` output for accounts/posts/media/analytics.
- `agent plan-publish`.
- `posts create --from-file --dry-run`.
- Agent publish guardrails with `--yes` and `--idempotency-key`.
- Audit metadata for write commands.

Success criteria:

- Codex / Claude Code can inspect real UniPost account context without browsing docs.
- Agent can safely dry-run publish payloads.
- Live publish is impossible by accident in non-interactive mode.
- Agent can move from read-only setup into draft/publish workflows only after explicit user confirmation.

### Phase 4: Advanced Ops And Diagnostics

Deliverables:

- `accounts health`.
- `accounts capabilities`.
- `accounts metrics`.
- `posts list --status failed`.
- `analytics summary/posts/platforms/export`.
- `analytics export` can stay post-GA within Phase 4 if analytics read commands are needed earlier.
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
- A first `unipost init` can create/store a named CLI API key after user authorization when setup-token/device auth backend exists.
- Re-running `unipost init` with a valid local credential reuses the existing credential and does not create a new API key.
- `unipost init --reauth` or `auth login --replace-key` can intentionally replace the local credential after explicit confirmation.
- If key creation is declined or fails, the user can retry init/bootstrap later without destructive state.
- A user can list profiles and accounts.
- A user can create a connect URL/session for a supported platform.
- A user can validate a post without publishing.
- A user can create a draft post.
- A user can publish an existing draft through `posts publish-draft` after explicit confirmation.
- A user can generate at least cURL and native Node.js `fetch` examples using their real account ID.
- SDK-backed examples are available only for SDK packages that are published and installable.
- Missing API key produces a clear hint and docs URL.

### 15.2 AI Agent Operator

- `agent bootstrap --client codex --json` returns a clear setup status and next actions.
- `agent bootstrap --client claude-code --json` returns client-specific setup instructions.
- Before setup-token/device auth exists, `agent bootstrap` supports the `UNIPOST_API_KEY` fallback path.
- After setup-token/device auth exists, Dashboard-generated setup tokens can be exchanged without exposing long-lived API keys to the agent.
- When auth, profile or connected account is missing, bootstrap returns a recommended plain-language prompt for the agent to ask the user.
- Every agent-relevant read command supports `--json`.
- JSON output uses the standard envelope.
- Error output uses the standard error envelope.
- `agent context --json` returns workspace, profiles, accounts and recent post summary.
- Non-interactive live publish fails without `--yes`.
- Non-interactive live publish fails without `--idempotency-key`.
- Dry-run publish returns a validation result and plan without publishing.
- Write commands include source metadata only after the backend accepts those fields.

### 15.3 Agent-Assisted Onboarding

- A newly registered user can see a "Connect with Claude Code / Codex" entry point in Dashboard.
- Dashboard can produce either a setup-token command or device-auth command for the selected client.
- User can paste the generated prompt into Codex / Claude Code and have the agent continue without reading UniPost docs.
- Agent can use CLI output to ask clear user-confirmation questions when multiple profiles/accounts/platforms are possible.
- Agent can complete auth check, workspace check, account discovery and first post validation through CLI.
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
- Posts validate/create/list/get/publish-draft endpoints; draft creation uses `POST /v1/posts` with `status: "draft"`.
- Media reserve/upload/get endpoints.
- Analytics summary/posts/platforms endpoints, plus export when `analytics export` is included.

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

Docs dependencies:

- CLI docs page.
- Quickstart docs.
- MCP docs.
- API reference.
- SDK docs.
- Published SDK package docs only when SDK-backed examples are enabled.

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
- Treat missing profile/account/platform choices as explicit user-confirmation points.
- Keep live publish out of bootstrap.
- Document expected agent phrasing in CLI docs and Dashboard prompt templates.

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
- Registered users can start from Dashboard by choosing "Connect with Claude Code / Codex" and pasting a generated prompt into their agent.
- CLI does not replace SDK, API or MCP.
- CLI defaults to validation/draft before live publish.
- Agent-assisted onboarding should use setup token or device auth by default, not manual long-lived API key sharing.
- After user authorization, UniPost should create a named, revocable CLI API key automatically and return it once to the local CLI for secure storage.
- `unipost init` is safe to rerun: valid existing credentials are reused, and new keys are created only on first setup, missing/invalid credentials, or explicit replacement.
- Created keys are stored in OS keychain by default; config stores only key metadata.

---

## 19. Recommended V1 Command Set

Minimum GA command set:

```text
unipost --version
unipost auth status
unipost auth login
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
unipost accounts list
unipost accounts get
unipost posts validate
unipost posts draft
unipost posts create
unipost posts publish-draft
unipost posts list
unipost posts get
unipost media upload
unipost media get
unipost media wait
unipost analytics summary
unipost examples posts.create
unipost agent bootstrap
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
- Use `UNIPOST_API_KEY` as the clearest first fallback auth path.
- Use browser/device auth or Dashboard setup tokens as the default agent-assisted onboarding auth path once backend support exists.
- Add optional local credential storage after the basic flow works, with keychain preferred over file storage.
- Repeat `init` should reuse an existing valid credential; create a new key only when no valid credential exists or the user explicitly requests replacement.
- Default first post path to `validate` then `draft`, not live publish.
- Support `--json` on all read commands from the beginning.
- Add `agent bootstrap` and `agent context` early because they are the highest-value AI-agent onboarding primitives.
- Require `--yes` and `--idempotency-key` for non-interactive live publish.
- Keep MCP separate but let CLI generate MCP config snippets.

---

## 21. Definition Of Done For The CLI Product

The CLI product is considered successfully launched when:

1. A new developer can install it and complete Quickstart without reading the full API docs.
2. The same developer can generate a usable cURL/native HTTP example containing their real profile/account IDs, plus SDK examples where published packages exist.
3. An AI coding agent can run `agent context --json` and get enough data to choose real accounts instead of inventing IDs.
4. An AI coding agent can dry-run a post and receive a structured publish plan.
5. Live publish cannot happen accidentally in non-interactive agent usage.
6. Support can ask users to run `unipost doctor --json` and receive safe diagnostic output.
7. CLI docs clearly explain when to use CLI, SDK, raw API and MCP.
8. Full agent-assisted onboarding can create/store a named, revocable CLI API key through setup-token or device auth without manual API-key copy/paste.
