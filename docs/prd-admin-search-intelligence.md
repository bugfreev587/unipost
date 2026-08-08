# PRD：UniPost Admin Search Intelligence

## 1. 文档信息

- 状态：Draft for implementation planning
- 产品区域：UniPost Admin
- 关联 PRD：[非品牌自然搜索增长与内容质量治理](./prd-nonbrand-organic-search-growth.md)
- 数据源：Google Search Console、Google Analytics 4、UniPost first-party data

本 PRD 定义 UniPost 如何在 Admin 中通过 OAuth 直接连接 GSC 与 GA4，持续读取搜索、流量、注册、激活和付费数据，并形成可解释的 SEO 经营闭环。

## 2. 背景与现状

### 2.1 为什么需要产品内连接

当前 GSC、GA4 和 UniPost 产品数据分散在三个系统中：

- GSC 能看到 query、page、country、device、impressions、clicks、CTR、position；
- GA4 能看到 landing page、source / medium、session 和前端行为；
- UniPost 能确认注册、API key、账号连接、API 调用、成功发布、订阅和收入。

单独看任何一个系统都不能回答“哪个非品牌搜索主题带来了合格注册和激活”。人工导出可以完成一次分析，但无法支撑持续迭代、数据新鲜度监控和稳定口径。

### 2.2 2026-07-22 Google 配置审计

已通过指定 Chrome 账号进行只读核验：

| 配置项 | 当前状态 |
| --- | --- |
| GSC property | `sc-domain:unipost.dev` |
| GSC ownership | 指定管理员为 verified owner |
| GA4 account | `400142017` |
| GA4 property | `544348649`，显示名 `UniPost` |
| GA4 web stream | `15210734183`，`https://unipost.dev`，过去 48 小时有流量 |
| GA4 permission | 当前管理员满足至少 Editor 权限 |
| GA4 ↔ GSC native link | 尚未建立 |
| GA4 key events | 当前首页显示 0 |
| Google Cloud project | `unipost-492219`，显示名 `UniPost` |
| OAuth branding | 已验证，home / privacy / terms 与 `unipost.dev` 一致 |
| OAuth audience | External、In production |
| Existing OAuth client | `UniPost-OAuth`，Web application，仅配置 YouTube dev / staging / production callbacks |
| Existing OAuth scopes | YouTube / YouTube Analytics 相关，不含 GSC / GA4 |
| Required APIs | Search Console API、Google Analytics Data API、Google Analytics Admin API 均未启用 |

### 2.3 与 CiteLoop 的关系

CiteLoop 已通过 OAuth 读取 GSC / GA4，并且 GSC 权限列表中存在 CiteLoop 相关主体。UniPost 可以复用其连接模式和工程经验，但必须遵守：

- 不复制、迁移或共享 CiteLoop refresh token；
- 不依赖 CiteLoop runtime 才能读取数据；
- UniPost 使用自己的 OAuth client、redirect URI、token storage、审计与撤销流程；
- CiteLoop 现有 GSC 权限是否保留不影响本项目上线。

## 3. 目标

### 3.1 主目标

让授权管理员在 UniPost Admin 内完成一次 Google OAuth 连接后，可以持续查看：

- 哪些非品牌 query cluster 获得展示与点击；
- Google Organic 流量落到哪些页面；
- 这些 landing page 带来多少注册、7 天激活和付费；
- US、EMEA、Other 的表现差异；
- 哪些 URL 存在 cannibalization、索引或规范化异常；
- 数据何时同步、是否过期、为何与 Google UI 可能存在差异。

### 3.2 成功标准

- 管理员无需离开 UniPost，即可重现 28 / 90 天 SEO 基线；
- Google access token 过期时由后端 refresh token 自动续期；
- 页面加载不直接依赖 Google API 实时响应；
- GSC、GA4 与 UniPost 数据按合法粒度组合，不制造个人级 query 归因；
- 连接失败、授权撤销、配额或数据过期在 Admin 中可见且可恢复。

## 4. 非目标

- V1 不支持任意客户连接自己的 GSC / GA4；仅连接 UniPost 自有资源。
- V1 只允许一个 active Google connection、一个 GSC property 和一个 GA4 property。
- 不从 Admin 修改 GSC、GA4 报表、过滤器、转化设置或数据保留政策。
- 不通过 Search Console API 自动请求大量 URL 索引。
- 不把 GSC query 绑定到具体用户、邮箱或 user ID。
- 不让浏览器直接持有 Google refresh token 或 client secret。
- 不复用现有 YouTube OAuth client。
- 不在 Preview 环境连接真实生产 Google 账号。

## 5. OAuth 与 Google 配置决策

### 5.1 连接方式

V1 使用 UniPost-owned OAuth 2.0 Web Server flow：

- OAuth state 防 CSRF；
- PKCE 防授权码拦截；
- `access_type=offline` 获取可持续刷新的 refresh token；
- 强制显式 consent 仅用于首次或丢失 refresh token 的重连；
- Google API 调用全部由后端执行。

请求范围仅为：

- `https://www.googleapis.com/auth/webmasters.readonly`
- `https://www.googleapis.com/auth/analytics.readonly`

不请求写权限，不请求 YouTube 权限，不请求 Gmail、Drive 或用户联系人权限。

### 5.2 独立项目与客户端

不得向当前 `unipost-492219` 的 `UniPost-OAuth` 客户端追加 GSC / GA4 callback 或 scopes。原因：

- 该客户端正在服务 YouTube 连接；
- 新增敏感范围可能触发 OAuth 再审核并扩大既有 consent screen；
- 搜索数据访问与社交账号发布属于不同安全边界；
- 独立客户端可以单独轮换、撤销和审计。

实施时创建：

1. `UniPost Search Intelligence Non-Production` Google Cloud project
   - 用于 dev、staging；
   - 使用独立 Web OAuth client；
   - 仅授权指定管理员；
   - Preview 使用 mock provider，不接入此项目。
2. `UniPost Search Intelligence Production` Google Cloud project
   - 仅用于 production；
   - 使用独立 Web OAuth client；
   - 独立 client secret、redirect URI、token 与审计记录。

实际 Google project ID 和 OAuth client ID 在配置完成后写入受控运维 runbook；client secret 只进入环境 secret store，不进入文档、日志、前端或 Git。

### 5.3 部署回调

Non-production client 允许：

- `https://dev-api.unipost.dev/v1/admin/search-intelligence/google/callback`
- `https://staging-api.unipost.dev/v1/admin/search-intelligence/google/callback`

Production client 仅允许：

- `https://api.unipost.dev/v1/admin/search-intelligence/google/callback`

不使用通配符，不把 production callback 配入 non-production client，也不把 localhost 加入 production client。

### 5.4 必须启用的 API

两个新项目均启用：

- Google Search Console API (`searchconsole.googleapis.com`)
- Google Analytics Data API (`analyticsdata.googleapis.com`)
- Google Analytics Admin API (`analyticsadmin.googleapis.com`)

### 5.5 OAuth consent 与验证

- App name：UniPost Search Intelligence；
- Homepage：`https://unipost.dev`；
- Privacy：`https://unipost.dev/privacy`；
- Terms：`https://unipost.dev/terms`；
- Authorized domain：`unipost.dev`；
- User type：External（当前管理员为个人 Google 账号，无法依赖 Workspace Internal）；
- Publishing status：production 连接必须为 In production，避免 Testing 状态 refresh token 的短生命周期；
- Non-production 项目在进入持续 dev / staging 验收前也应切换为 In production；若仍处于 Testing，则只允许短期开发调试，并在 UI 明确提示需要重新授权；
- Data Access 仅声明上述两个 read-only scopes；
- 如果 Google 将 scope 标记为需审核，则提交 scope justification、隐私政策和功能演示；未完成必要审核时不得向非指定管理员开放连接。

### 5.6 GA4 与 GSC 原生关联

除 UniPost 自己的 API OAuth 外，还需在 GA4 建立原生 Search Console link：

- GA4 property：`544348649`；
- GSC property：`sc-domain:unipost.dev`；
- Web stream：`15210734183`。

这两个连接不是同一件事：

- **GA4 ↔ GSC native link** 让 GA4 内出现 Google Organic Search Queries / Traffic 报表；
- **UniPost OAuth** 让 UniPost 后端自主同步 GSC 和 GA4 数据。

原生关联不能替代 UniPost OAuth，UniPost OAuth 也不会自动创建 GA4 原生关联。

## 6. 用户与权限

### 6.1 角色

| 操作 | Super Admin | Admin / Marketing viewer | 其他用户 |
| --- | --- | --- | --- |
| 查看 Search Intelligence | 是 | 按现有 Admin RBAC 授权 | 否 |
| Connect / Reconnect Google | 是 | 否 | 否 |
| 选择 GSC / GA4 property | 是 | 否 | 否 |
| Manual sync | 是 | 可选只读角色不开放 | 否 |
| 修改 query map / brand rules | 是 | 按现有内容管理权限 | 否 |
| Disconnect / delete synced data | 是，二次确认 | 否 | 否 |

所有配置操作写入审计日志，包含 actor、action、resource、timestamp、result；不得写入 token 或 secret。

## 7. 用户流程

### 7.1 首次连接

1. Super Admin 打开 `Admin > Search Intelligence > Settings`。
2. 点击 `Connect Google`。
3. 后端生成 state、PKCE 和一次性连接事务。
4. Google consent screen 只展示 GSC read-only 与 GA read-only 权限。
5. 回调后，后端交换 token 并加密保存 refresh token。
6. 后端调用 GSC Sites API 和 Analytics Admin API 列出可访问资源。
7. 管理员选择 `sc-domain:unipost.dev` 与 property `544348649`。
8. 系统验证权限并保存选择。
9. 系统启动 90 天 initial backfill。
10. 页面展示同步进度、已完成日期范围和任何部分失败。

### 7.2 日常使用

1. Viewer 打开 Search Intelligence。
2. 页面从 UniPost 本地快照读取，不同步阻塞 UI。
3. 默认展示最近 28 个完整数据日，并显示 GSC / GA4 各自 last successful sync。
4. 用户可切换 7 / 28 / 90 天、US / EMEA / Other、brand / non-brand、query cluster 与 landing page。
5. 页面将 Google 聚合数据与 UniPost landing / cohort 数据并列展示。

### 7.3 重新授权

当 refresh token 被撤销、过期或权限不足：

- 同步状态变为 `Action required`；
- 历史数据仍可读；
- 只有 Super Admin 可点击 `Reconnect`；
- 成功后继续增量同步，不重复制造历史记录。

### 7.4 断开连接

- 明确展示将停止新数据同步；
- 二次确认后调用 Google token revocation；
- 立即删除本地 refresh / access token；
- 保留历史聚合数据并标记 disconnected；
- 另提供独立的“删除已同步历史数据”高风险操作，不与普通 disconnect 捆绑。

## 8. Admin 信息架构

V1 为一个 Admin 页面，使用以下 tab：

### 8.1 Overview

- 非品牌 impressions、clicks、CTR、average position；
- Google Organic sessions、新注册、qualified registrations、7-day activations；
- 30 / 60 / 90 天 paid cohorts；
- US、EMEA、Other 对比；
- 数据 freshness 与异常摘要。

### 8.2 Queries

- query、cluster、brand / non-brand、country group；
- impressions、clicks、CTR、position；
- primary URL 与出现展示的其他 URL；
- cannibalization candidate 标记；
- 7 / 28 / 90 天趋势。

### 8.3 Pages

- landing page / canonical URL；
- GSC impressions、clicks、CTR、position；
- GA4 Organic Search sessions、engagement；
- UniPost registrations、qualified registrations、7-day activations、paid cohorts；
- www / non-www、HTTP / HTTPS 等 host variant 诊断。

### 8.4 Conversions

- 按 landing page 和区域聚合 first-party funnel；
- Registration → API key → Connected account → Successful API call → Successful publish → Paid；
- 不能显示“query → individual user”；
- 在所有 query 相关表格上展示归因限制提示。

### 8.5 Technical Health

- GSC indexed / not indexed 趋势；
- sitemap 状态；
- robots 状态；
- canonical host variant；
- sync failures 与 quota；
- URL Inspection 仅针对有限核心 URL 的按需诊断，不做批量索引请求。

### 8.6 Settings

- OAuth connected / disconnected / action required；
- selected GSC property、GA4 property 和 stream；
- granted scopes；
- last sync、next scheduled sync、last error；
- Connect、Reconnect、Manual sync、Disconnect；
- brand query rules、query-to-page map 的入口；
- 数据保留和删除说明。

## 9. 数据采集需求

### 9.1 GSC

使用 Search Analytics API，按单日同步并分页：

- 单独同步仅含 `date` 的 top-line totals，作为总 clicks / impressions 的事实口径；
- 另同步 `date`、`query`、`page`、`country`、`device` 明细，用于主题、页面、地区和设备分析；
- metrics：`clicks`、`impressions`、`ctr`、`position`；
- type：`web`；
- `rowLimit` 最高 25,000，使用 `startRow` 分页；
- 记录 API 返回的完整日期范围和数据状态。

已知限制必须在产品中说明：

- GSC 通常有 2–3 天延迟；
- query / page 维度可能因隐私阈值省略部分数据；
- Search Analytics API 不保证返回所有行，按 query/page 聚合时总数可能与顶层总数不同；
- UI 不得把明细行重新求和后冒充 top-line totals；二者差值应标记为 anonymized / omitted data；
- API 每天每 search type 最多返回约 50,000 行；UniPost 当前规模远低于上限，但同步器仍需检测截断。

另使用：

- Sites API 列出资源与权限；
- Sitemaps API 读取 sitemap 状态；
- URL Inspection API 对核心 URL 做有限、按需检查。

### 9.2 GA4

使用 Analytics Admin API：

- `accountSummaries.list` 列出可访问 account / property；
- 读取 property 元数据和 web stream 元数据。

使用 Analytics Data API：

- 标准报表：`runReport` / `batchRunReports`；
- dimensions 至少包括 `date`、`landingPagePlusQueryString`、`sessionDefaultChannelGroup`、`firstUserSourceMedium`、`country`、`deviceCategory`；
- metrics 至少包括 `sessions`、`totalUsers`、`newUsers`、`engagedSessions`、`engagementRate`、`keyEvents`；
- 仅提取 Google Organic 与必要对照组；
- 页面需要展示 GA4 identity、thresholding、sampling 或 `(not set)` 对结果的影响。

V1 不要求页面使用 realtime API；实时流量不是 SEO 决策核心，避免不必要配额与产品复杂度。

### 9.3 UniPost first-party data

以 UniPost 数据库为注册、激活和付费事实源：

- 注册时间、first-touch source / medium、first landing page、country；
- API key created；
- social account connected；
- successful core API call；
- successful publish；
- subscription started / paid；
- test / employee / bot exclusion标记。

若当前注册链路尚未可靠保存 first landing / source，则该采集是本项目上线前置条件。GA4 client ID 可以用于会话分析，但不得替代 UniPost 用户与业务事件事实。

## 10. 数据模型

建议逻辑实体：

- `search_google_connections`
  - status、encrypted_refresh_token、granted_scopes、connected_by、connected_at、last_refresh_at；
- `search_properties`
  - source、external_id、display_name、selected、permission_level；
- `search_sync_runs`
  - source、job_type、date_range、status、rows_read、rows_written、quota、error_code、started_at、completed_at；
- `gsc_daily_metrics`
  - date、query、page、country、device、clicks、impressions、ctr、position；
- `ga4_daily_metrics`
  - date、landing、channel、source_medium、country、device、sessions、users、engaged_sessions、key_events；
- `search_query_clusters`
  - cluster、brand_classification、primary_url、status、owner、rule_version；
- `search_landing_cohorts`
  - cohort_date、landing、region、registrations、qualified_registrations、activated_7d、paid_30d、paid_60d、paid_90d；
- `search_admin_audit_log`
  - actor、action、resource、result、timestamp、safe_metadata。

所有日粒度记录同时保存 `source_date`、source timezone 与 ingestion timestamp。GSC 日期按其官方报表时区解释，GA4 按 property timezone 解释；跨源比较按完整 source date 对齐，不把 UTC 截断日期直接连接。

数据库迁移必须遵守现有 tenant / admin isolation 规则。虽然 V1 只连接 UniPost 自有资源，数据访问仍需显式 Admin 权限，不得暴露到普通客户 API。

## 11. 同步与可靠性

### 11.1 调度

- Initial backfill：连接成功后同步最近 90 个可用数据日；
- Daily GSC：每天一次，重同步最近 7 天以吸收延迟修正；
- Daily GA4：每天一次，重同步最近 3 天；
- Manual sync：每个 source 最多每 15 分钟一次，仅 Super Admin；
- Page load：只读本地数据，不触发 Google live fetch。

### 11.2 幂等与重试

- 所有聚合表使用稳定自然键 upsert；
- job 使用分布式互斥，避免同一 source / date range 并发；
- 可重试错误采用有上限的指数退避；
- OAuth invalid_grant、permission denied 和 scope missing 不无限重试，直接进入 Action required；
- 部分日期失败时保留成功分区，并支持从失败日期恢复。

### 11.3 Freshness

- GA4 最后成功同步超过 48 小时显示 stale warning；
- GSC 最后成功同步超过 72 小时显示 stale warning；
- 页面同时显示 source data date 与 ingestion time；
- 一个 source stale 时，另一个 source 的数据仍可读，但跨源结论标记为 incomplete。

### 11.4 保留

- 聚合搜索和分析数据默认保留 24 个月；
- Admin 审计日志至少保留 12 个月；
- token 保留至 disconnect、撤销或管理员删除；
- access token 仅缓存必要生命周期，refresh token 加密持久化；
- 数据删除必须可审计且与普通断开连接分离。

## 12. API 需求

建议 Admin API：

- `POST /v1/admin/search-intelligence/google/connect`
- `GET /v1/admin/search-intelligence/google/callback`
- `POST /v1/admin/search-intelligence/google/reconnect`
- `POST /v1/admin/search-intelligence/google/disconnect`
- `GET /v1/admin/search-intelligence/properties`
- `PUT /v1/admin/search-intelligence/properties/selection`
- `POST /v1/admin/search-intelligence/sync`
- `GET /v1/admin/search-intelligence/status`
- `GET /v1/admin/search-intelligence/overview`
- `GET /v1/admin/search-intelligence/queries`
- `GET /v1/admin/search-intelligence/pages`
- `GET /v1/admin/search-intelligence/conversions`
- `GET /v1/admin/search-intelligence/technical-health`
- `GET /v1/admin/search-intelligence/query-map`
- `PUT /v1/admin/search-intelligence/query-map`

所有 endpoint：

- 使用现有 Admin authentication / authorization；
- 拒绝普通用户和 API key 客户端；
- 对连接、同步、断开和映射修改写审计日志；
- 返回稳定 error code，不返回 Google token 或原始 provider 错误中的敏感信息。

## 13. 安全与隐私

- refresh token 使用 authenticated encryption 加密后存储；
- encryption key 与数据库分离，存入各环境 secret store；
- client secret、access token、refresh token 不写日志、trace、error monitoring 或 analytics；
- production 与 non-production secrets 完全隔离；
- OAuth callback 校验 state、PKCE、issuer、redirect URI 与 transaction expiration；
- Connect / Disconnect 使用 CSRF 防护与最近登录校验；
- scope 必须精确匹配 allowlist，检测到扩大权限时拒绝保存并告警；
- 数据表与 Admin API 使用最小权限；
- 隐私政策明确说明读取 GSC / GA4 的目的、保留和撤销方式；
- 不存储 GSC 用户级数据，因为 GSC 本身仅提供聚合搜索数据。

## 14. Google 外部配置交付清单

实施工作不仅包含代码，还必须由 Codex 在用户已授权的 Google 账号中完成并记录以下配置：

### 14.1 Google Cloud

- [ ] 创建 non-production 与 production Search Intelligence projects。
- [ ] 启用 Search Console API、Analytics Data API、Analytics Admin API。
- [ ] 配置 OAuth branding、authorized domain、support / developer contact。
- [ ] 添加且仅添加两个 read-only scopes。
- [ ] 分别创建 Web OAuth client。
- [ ] 注册本 PRD 指定的 dev、staging、production callbacks。
- [ ] 将 client ID / secret 写入对应环境 secret store；不通过聊天或 Git 传递 secret。
- [ ] 在需要时完成 Google OAuth verification 材料与提交。

### 14.2 Search Console

- [ ] 确认 `sc-domain:unipost.dev` ownership 仍有效。
- [ ] 确认 OAuth 管理员可通过 Sites API 读取该 property。
- [ ] 核验 sitemap、robots 和核心 URL inspection 数据可读取。
- [ ] 不删除 CiteLoop 或其他现有主体，除非用户另行授权清理。

### 14.3 GA4

- [ ] 确认 account `400142017` / property `544348649` / stream `15210734183`。
- [ ] 建立 GA4 property 与 `sc-domain:unipost.dev` 的 Search Console link。
- [ ] 确认 Google Organic Search Queries / Traffic 报表在 Google 处理完成后可用。
- [ ] 审计 GA4 `sign_up` 与其他关键事件；当前 key events 为 0，至少确保注册衡量口径明确。
- [ ] 确认 UniPost OAuth 可通过 Admin / Data API 读取正确 property。

### 14.4 Runbook

记录但不记录 secret：

- Google project display name / project ID；
- OAuth client display name / client ID；
- callback URI；
- scopes；
- GSC / GA4 property ID；
- 环境变量名与 secret 所在系统；
- 上次验证时间；
- token 撤销、client secret rotation、重新授权和故障排查步骤。

## 15. 验收标准

### 15.1 OAuth

- [ ] 未授权 Admin 被拒绝连接、重连和断开。
- [ ] state mismatch、PKCE mismatch、过期 callback 被拒绝。
- [ ] consent screen 只显示两个 read-only 业务 scopes。
- [ ] refresh token 加密存储，日志和 API 响应中不可见。
- [ ] token 撤销后状态在下一次同步中变为 Action required。
- [ ] Disconnect 撤销 Google token 并立即删除本地 token。

### 15.2 数据

- [ ] 初始 90 天 backfill 可暂停、重试并幂等恢复。
- [ ] 对相同日期范围与过滤条件，GSC click / impression 与 GSC UI 在同一数据更新时间内差异不超过 1%。
- [ ] GA4 session / user 与 GA4 UI 在同一 property、identity、date range 和 filter 下差异不超过 1%，或页面能明确解释 thresholding / freshness 差异。
- [ ] `sc-domain:unipost.dev`、property `544348649` 和 stream `15210734183` 被正确选中。
- [ ] GSC 延迟、匿名 query 缺失和行数上限在 UI 中可见。
- [ ] query 不能下钻到个人注册用户。

### 15.3 业务

- [ ] Overview 可重现 28 / 90 天基线。
- [ ] Query / Page / Region / Brand filters 一致作用于所有 tab。
- [ ] US、EMEA、Other 分组与 first-party country 规则一致。
- [ ] landing cohort 可展示注册、qualified registration、7-day activation、paid 30 / 60 / 90。
- [ ] www / non-www 分裂可被 Pages / Technical Health 识别。
- [ ] cannibalization 候选能显示 query、primary URL 与其他 ranking URLs。

### 15.4 外部配置

- [ ] GA4 Search Console link 显示已连接。
- [ ] 三个 Google API 在正确 Cloud project 中显示 Enabled。
- [ ] dev / staging 无法使用 production client secret。
- [ ] production callback 列表不包含 dev、staging、localhost 或通配符。
- [ ] 现有 YouTube `UniPost-OAuth` client 与 scopes 未被改变。

### 15.5 发布门禁

- [ ] Backend：`GOCACHE=/tmp/unipost-go-build go test ./...` 通过。
- [ ] Dashboard：`npm run build` 通过。
- [ ] Admin routing / auth / analytics regression 通过。
- [ ] Preview 使用 mock OAuth provider 完成 connected、expired、permission denied、disconnect 状态验收。
- [ ] exact PR head SHA 的 CI、Railway PR Environment、Vercel Preview、deployed regression 与 Codex browser acceptance 全部成功。
- [ ] 合并 `dev` 后在真实 dev 环境完成真实 non-production OAuth 连接和同步验收。
- [ ] 标准发布时在 staging 重验后，才配置和验证 production OAuth。

## 16. 监控与告警

至少记录：

- sync success / failure / duration / rows / quota；
- token refresh success / categorized failure；
- latest source date 与 ingestion lag；
- Google 429、5xx、permission denied、invalid_grant；
- detected row cap / partial data；
- manual sync rate limit；
- admin connect / reconnect / disconnect / property change。

告警优先级：

- P1：疑似 token / secret 泄露、未授权数据访问；
- P2：production 同步连续失败、授权撤销、错误 property；
- P3：数据 stale、单日期部分失败、配额接近阈值。

## 17. 回滚与恢复

- UI 或聚合逻辑异常：停止 scheduler，保留已同步快照，Admin 显示数据暂停更新；
- OAuth 异常：断开并撤销 Search Intelligence token，不影响 YouTube OAuth；
- 错选 property：停止同步，选择正确 property，按日期重新 backfill；历史错误数据隔离后再删除；
- GA4 native link 配错：在 GA4 中删除错误 link，并按确认的 property / stream 重建；
- client secret 需要轮换：新增 secret、双写验证、切换环境、撤销旧 secret，并更新 runbook；
- 任何回滚均不得删除 first-party 注册、激活或付费事实数据。

## 18. 依赖

- 现有 Admin RBAC 与审计能力；
- 后端 scheduler / worker 与安全 secret store；
- first-touch source / landing page 的可靠采集；
- 注册、API key、连接、API 调用、发布、订阅事件的统一口径；
- Google OAuth verification 与指定管理员授权；
- [非品牌自然搜索增长 PRD](./prd-nonbrand-organic-search-growth.md) 中的 query map、brand 分类与 URL inventory。

## 19. 官方参考

- [Search Console API authorization](https://developers.google.com/webmaster-tools/v1/how-tos/authorizing)
- [Search Analytics API: getting all available data](https://developers.google.com/webmaster-tools/v1/how-tos/all-your-data)
- [Search Console API reference](https://developers.google.com/webmaster-tools/v1/api_reference_index)
- [Google Analytics Data API](https://developers.google.com/analytics/devguides/reporting/data/v1)
- [Analytics Data API quotas](https://developers.google.com/analytics/devguides/reporting/data/v1/quotas)
- [OAuth 2.0 for web server applications](https://developers.google.com/identity/protocols/oauth2/web-server)
- [OAuth app verification](https://support.google.com/cloud/answer/13464323)
- [OAuth production readiness best practices](https://support.google.com/cloud/answer/15549945)
