# PRD：UniPost 非品牌自然搜索增长与内容质量治理

## 1. 文档信息

- 状态：Draft for implementation planning
- 产品：UniPost
- 目标市场：全球英语开发者市场，按 US、EMEA、Other 分组观察
- 关联 PRD：[Admin Search Intelligence](./prd-admin-search-intelligence.md)
- 关联既有文档：[SEO / GEO Search Growth](./prd-seo-geo-search-growth.md)、[SEO / GEO Search Execution](./seo-geo-search-execution.md)

本 PRD 是对既有 SEO / GEO 规划的增量收敛，聚焦一个已被真实数据验证的问题：UniPost 可以凭品牌词 `unipost` 获得搜索流量，但尚未通过 `social media posting API`、`unified social media API` 等非品牌商业意图查询稳定获得合格注册与激活。

## 2. 背景与问题

### 2.1 用户问题

开发者在不知道 UniPost 品牌的情况下，会通过以下意图寻找产品：

- social media posting API
- unified social media API
- social media publishing API
- API to post to multiple social media platforms
- TikTok / Instagram / YouTube / X posting API
- white-label social media API

目前，搜索 `unipost` 时官网可获得首位或高位展示；搜索更接近产品品类的查询时，UniPost 很难进入前两页。这意味着现有自然搜索主要在承接已经知道品牌的人，而不是持续创造新需求。

`unified api for publishing post` 可作为问题样例，但不是需要机械匹配的主关键词。页面应覆盖自然、真实、有商业价值的开发者查询，而不是为一个不自然的短语堆词。

### 2.2 2026-07-22 基线

Google Search Console 对 `sc-domain:unipost.dev` 的近 3 个月数据：

| 指标 | 当前值 | 解释 |
| --- | ---: | --- |
| Web clicks | 134 | 总自然搜索点击 |
| Web impressions | 2,979 | 总自然搜索展示 |
| Average CTR | 4.5% | 品牌流量占比较高，整体 CTR 不代表非品牌能力 |
| Average position | 7.9 | 同样被品牌词显著抬高 |
| `unipost` clicks | 88 | 单一品牌查询贡献约 65.7% 总点击 |
| `/` clicks | 121 | 流量高度集中在首页 |
| Indexed pages | 13 | 与当前公开内容规模不匹配 |
| Not indexed pages | 38 | 需要按原因治理，而非继续扩张薄内容 |

Top query 中已经出现 `tiktok upload api`、`tiktok content posting api`、`tiktok posting api` 等非品牌展示，但点击为 0。目标品类页尚未进入主要点击页面。

Google 同时报告以下 URL 形态：

- `https://unipost.dev/`
- `https://www.unipost.dev/`
- `https://www.unipost.dev/tiktok-api`
- `https://unipost.dev/youtube-api`

这表明 `www` / non-`www` 的重定向、canonical、内部链接和 sitemap 一致性需要作为 P0 验证，而不能仅依赖页面声明。

### 2.3 已发现的内容质量问题

公开页面存在会降低用户信任与搜索质量的可见产物：

- 页面正文展示内部编辑字段，如 `Primary query:`、`Meta Description:`、`Slug:`、`H1:`。
- 平台页面展示 `SCREENSHOT PLACEHOLDER`、`DEMO VIDEO PLACEHOLDER`、`ANALYTICS SCREENSHOT PLACEHOLDER`。
- 部分文章包含未用 UniPost 实际 API 验证的泛化或虚构代码示例。
- `/social-media-api`、`/social-media-posting-api`、`/social-media-publishing-api` 意图重叠，存在关键词与权重互相竞争风险。
- sitemap `lastmod` 可能随构建或请求变化，而不是反映内容真实更新时间。
- GSC robots 报告对多个子域显示警告；`clerk.unipost.dev/robots.txt` 还显示 404。需要区分生产营销站问题、非索引子域问题和可忽略的历史状态。
- 外部权威信号主要来自 SDK / 包注册表，缺少高相关开发者生态引用、真实集成案例和客户证据。

## 3. 产品目标

### 3.1 主目标

通过非品牌自然搜索持续获得合格开发者注册，并在注册后 7 天内推动可验证的产品激活。

### 3.2 指标优先级

1. Qualified non-brand organic registrations
2. 7-day activated non-brand organic registrations
3. Non-brand impressions、clicks、CTR、目标 URL 平均排名
4. 30 / 60 / 90 天付费转化，按 US、EMEA、Other 分组观察

排名是领先指标，不是最终结果。付费转化是业务结果，但当前付费样本过小，不作为第一阶段发布门槛。

### 3.3 合格注册定义

同时满足以下条件：

- 新注册用户；
- first-touch 或可验证 acquisition source 为 Google Organic；
- 首次自然搜索落地页属于开发者/API/集成评估意图；
- 不属于已知 bot、测试账号、员工账号或重复注册；
- 若 GSC 无法提供用户级 query，则以 query cluster 的聚合数据与用户级 landing/source 数据分别归因，不虚构一对一关键词归因。

### 3.4 7 天激活定义

注册后 7 天内至少完成以下任一事件：

- 创建 API key；
- 连接至少一个社交账号；
- 完成一次成功的核心 API 调用；
- 完成一次成功发布。

激活事件的统一采集和 Admin 呈现由关联的 Admin Search Intelligence PRD 负责。

## 4. 非目标

- 不承诺某个关键词在固定日期达到 Google 第一名。
- 不通过关键词堆砌、隐藏文字、批量低质量页面或自动拼接文章追求展示。
- 不购买链接，不进行与产品无关的客座文章交换。
- 不在本 PRD 中改变核心 API、发布流程或授权逻辑。
- 不把 Google Ads 或其他付费投放计入自然搜索成果。
- 不尝试将 GSC 的聚合 query 数据错误绑定到具体个人。
- 不在第一阶段建设多语言站点；先验证全球英语内容模型。

## 5. 用户与使用场景

### 5.1 主要用户

- 正在为 SaaS 产品集成多平台发布能力的开发者；
- 需要 white-label social publishing 的产品负责人或技术负责人；
- 需要减少 TikTok、Instagram、YouTube、X 等平台 API 集成成本的工程团队；
- 正在比较 Ayrshare、Late、Postiz 等方案的技术买家。

### 5.2 地域策略

当前美国注册最多，但已知两个付费用户分别来自匈牙利和以色列，近期注册还包括波兰及其他欧洲国家。因此：

- 内容语言保持英文；
- 示例、定价表达、隐私与合规信息需适合全球开发者；
- 数据必须按 US、EMEA、Other 分组，避免美国总量掩盖 EMEA 的更高商业价值；
- 不因样本小而单独创建国家薄页。

## 6. 产品原则

1. **一个意图，一个主页面**：每个核心商业查询集群只能有一个主要排名 URL。
2. **先修质量，再扩内容**：有占位符、内部标记、事实错误或重复意图时，不新增相同主题页面。
3. **证据优先**：页面必须展示真实 API、实际请求/响应、支持的平台限制和可验证产品能力。
4. **搜索到激活闭环**：不以展示量替代注册与激活结果。
5. **聚合数据诚实归因**：GSC、GA4、UniPost 一方数据按各自粒度使用。
6. **全球英语、分区分析**：内容统一，商业结果按区域分组。

## 7. 信息架构与查询所有权

### 7.1 建议所有权

| URL / 页面类型 | 主查询意图 | 决策 |
| --- | --- | --- |
| `/` | UniPost 品牌 + one API for social publishing | 保留品牌主入口，不承载所有非品牌长尾 |
| `/social-media-api` | unified social media API | 核心品类页 |
| `/social-media-posting-api` | social media posting API / post to multiple platforms API | 核心商业页 |
| `/social-media-publishing-api` | SaaS / white-label social publishing workflow | 仅在内容、受众、证据确实独立时保留；否则 301 合并 |
| `/{platform}-api` | 单平台 posting API | 对应平台专页，必须包含真实平台限制与 UniPost 实现 |
| `/solutions/*` | 产品使用场景 | 面向任务和买家角色，不复制品类页 |
| `/compare/*`、alternatives 内容 | 供应商比较 | 基于可验证功能、价格和迁移场景 |
| `/docs/*` | 实施与 API 参考 | 解决“如何实现”，不替代商业落地页 |
| `/blog/*` | 教育、趋势、问题解决 | 支持集群，不与商业页抢同一主意图 |

### 7.2 URL 处置流程

所有可抓取 URL 在实施前进入统一 inventory，并标记为：

- Keep：意图独立、质量达标；
- Improve：意图独立但内容或技术信号不足；
- Merge + 301：与更强页面重复；
- Noindex：对用户有用但不应参与搜索；
- Delete / 410：无用户价值、无迁移目标的历史内容。

处置决策必须结合 GSC 的 query → page、外链、转化和 canonical 数据，不能只按 URL 名称判断。

## 8. 功能需求

### 8.1 P0：内容质量清场

#### FR-SEO-001 页面产物清理

- 所有公开可索引页面不得出现内部编辑标签、SEO brief 字段或占位符。
- 所有截图、视频和分析图区域必须满足以下之一：真实资产、对用户有意义的可运行示例、完全移除。
- 代码示例必须使用当前 UniPost API endpoint、字段、认证方式和真实响应结构。
- 每个改动页面必须经过产品事实、API 技术、SEO/编辑三类检查。

#### FR-SEO-002 规范化与索引一致性

- 选定 `https://unipost.dev` 为唯一营销站主机。
- 所有 `http`、`www` 和非主机变体以单跳永久重定向到唯一 HTTPS non-`www` URL。
- canonical、Open Graph URL、结构化数据 URL、hreflang（如未来存在）、内部链接和 sitemap 使用同一主机与路径。
- 非生产子域、app、API、Clerk 等非营销内容按实际用途配置 noindex / robots，不得污染营销站索引诊断。
- 迁移前后保留重定向映射和回滚方案。

#### FR-SEO-003 sitemap 与 robots

- sitemap 只包含 canonical、200、indexable URL。
- `lastmod` 仅在正文或影响搜索结果的元数据真实变化时更新。
- robots.txt 不声明无法由 Google 支持或解析的规则；所有警告必须记录为已修复、预期行为或历史等待重抓取。
- GSC 中的 sitemap、robots 和 index coverage 状态纳入 Admin 告警。

### 8.2 P1：查询所有权与内容合并

#### FR-SEO-004 Query-to-page map

- Admin 中维护 query cluster、primary URL、secondary URLs、当前状态和负责人。
- 同一 query cluster 不得同时将两个商业页标为 primary。
- 系统标记近 28 天同一 query 在多个 URL 产生显著展示的 cannibalization 候选。

#### FR-SEO-005 核心品类页重写

核心页至少包含：

- 用户问题与 UniPost 的明确定位；
- 支持的平台与每个平台的真实能力差异；
- API 请求/响应示例；
- 认证、媒体、排程、状态回调和错误处理概览；
- 适用与不适用场景；
- 指向精确 docs 的内部链接；
- 可验证 CTA：查看文档、开始构建、注册。

页面内容不要求机械出现所有关键词变体，但 title、H1、intro 和主体应自然覆盖主意图。

#### FR-SEO-006 内部链接体系

- 首页、平台页、解决方案页、比较页、博客和文档形成明确的 hub → spoke 关系。
- 锚文本描述目标主题，不使用大量泛化 `learn more`。
- 新内容发布前必须声明其 primary cluster 与要支持的核心页。

### 8.3 P2：原创证据与权威建设

#### FR-SEO-007 原创开发者资产

优先制作可被引用的资产：

- 各平台发布能力与限制矩阵；
- 多平台发布 API 的错误处理、重试和状态模型；
- 真实 SDK 示例仓库；
- white-label 连接流程架构图；
- 平台审核、媒体规格或限额的持续更新页面；
- 经客户许可的集成案例与量化结果。

每项资产必须有维护负责人和“最后事实核验日期”。

#### FR-SEO-008 合规外部获取

- 通过 SDK registry、GitHub 示例、集成伙伴、客户技术文章、开发者目录和社区回答获得相关引用。
- 不以链接数量为目标；记录来源相关性、目标页面、referral sessions、注册与激活。
- 所有比较或生态页面必须披露事实来源与核验日期。

### 8.4 P3：持续实验

#### FR-SEO-009 SEO 实验单元

每个实验记录：

- 假设；
- query cluster 与 primary URL；
- 变更 SHA / 发布日期；
- 28 天前基线；
- 预期领先指标；
- 观察窗口；
- 结果与下一步。

不得通过每天查询个人化 Google SERP 作为唯一判断；排名以 GSC 聚合数据和受控验证为主。

## 9. 测量与归因

### 9.1 数据源职责

| 数据源 | 负责回答 | 不负责回答 |
| --- | --- | --- |
| GSC | query、page、country、device 的展示、点击、CTR、平均排名；索引状态 | 某个具体 query 属于哪个注册用户 |
| GA4 | landing page、source / medium、session、用户行为与前端关键事件 | UniPost 后端最终业务真相 |
| UniPost DB | 注册、地区、API key、连接、API 调用、发布、订阅与收入 | Google 搜索 query 的个人级归因 |
| Admin Search Intelligence | 按日期、landing、区域做可解释汇总 | 制造不存在的 user-query join |

### 9.2 时间窗口

- Baseline：上线前连续 28 天，并保留可用的 90 天 GSC 历史。
- Leading review：上线后 30 天。
- Outcome review：上线后 90 天。
- Paid cohorts：注册后 30 / 60 / 90 天。
- GSC 数据默认标记 2–3 天延迟，不与 GA4 当日实时数据强行对齐。

### 9.3 目标值

#### 30 天领先目标

- 所有 P0 目标页完成质量、canonical、重定向、sitemap 与结构化数据验收；
- GSC 已重抓核心页，且没有新增由本项目导致的 canonical / noindex / 404 问题；
- 至少一个目标非品牌 cluster 的 primary URL 进入平均 Top 20；
- 非品牌展示和合格注册不低于 28 天基线；
- `www` 与 non-`www` 的新增点击和展示持续向 canonical 主机收敛。

#### 90 天业务目标

- 若基线合格非品牌自然注册少于 5 / 28 天，则达到至少 5 / 28 天；否则较基线提升至少 50%；
- 7 天激活率不低于自然搜索注册总体基线；
- 至少一个主目标或高相关近义 cluster 的 primary URL 达到平均 Top 10；
- 获得至少一个来自相关开发者生态、集成伙伴或客户内容的自然引用，并产生可验证 referral 或品牌发现价值；
- 付费结果按 US、EMEA、Other 汇报，但不因短期样本不足判定项目失败。

这些是观察与经营目标，不是对搜索引擎排名的发布承诺。

## 10. Admin 需求接口

本 PRD 依赖 [Admin Search Intelligence](./prd-admin-search-intelligence.md) 提供：

- GSC 与 GA4 OAuth 连接状态；
- GSC query / page / country 数据同步；
- GA4 landing / source / key event 数据同步；
- UniPost 注册、激活、订阅 cohort 聚合；
- query-to-page map 与 cannibalization 提示；
- 索引、sitemap、robots、同步失败告警；
- 数据 freshness、口径和不可归因限制的可见说明。

没有 Admin 集成时，可以人工导出完成第一次 baseline，但不得长期以人工表格作为唯一事实源。

## 11. 发布阶段

### Phase 0：测量与冻结

- 保存 28 / 90 天基线；
- 建立 URL inventory；
- 暂停新增相同主题的批量内容；
- 确认 canonical host 与非生产子域索引政策。

### Phase 1：P0 修复

- 清理公开占位符和编辑字段；
- 修复 www / non-www / http 规范化；
- 修复 sitemap lastmod 与 robots 问题；
- 对核心页面执行事实和 API 验证。

### Phase 2：所有权收敛

- 建立 query-to-page map；
- 合并或重新定位重叠核心页；
- 重写核心品类页与内部链接；
- 提交 sitemap 并观察重抓取。

### Phase 3：权威增长

- 发布原创资产、示例仓库和真实案例；
- 开展相关生态引用；
- 以 28 天实验周期迭代。

## 12. 验收标准

### 12.1 内容与技术

- [ ] 所有 indexable 页面自动扫描不到禁止词和已知占位符。
- [ ] 核心代码示例通过契约或文档测试验证。
- [ ] 每个核心 query cluster 只有一个 primary URL。
- [ ] `http` / `www` 变体单跳 301/308 到 HTTPS non-`www` canonical。
- [ ] sitemap 仅含 canonical 200 indexable URL，`lastmod` 可追溯到真实变更。
- [ ] 结构化数据通过自动化校验，且内容与页面可见信息一致。
- [ ] 非生产与应用子域的索引策略有明确验收结果。

### 12.2 数据

- [ ] 28 天与 90 天基线可在 Admin 重现。
- [ ] Brand / non-brand 分类规则有版本并支持回溯。
- [ ] 初始品牌规则至少覆盖大小写与空格变体 `unipost`、`uni post`、`uniposts`，并允许经审核添加误拼或产品专有词。
- [ ] US、EMEA、Other 三个区域口径一致。
- [ ] 注册、激活与付费 cohort 可按 landing page 汇总。
- [ ] 页面明确说明 GSC query 不可与个人用户直接关联。

### 12.3 发布门禁

- [ ] 本地 CI 与相关 dashboard regression 通过。
- [ ] Preview Acceptance 在 exact PR head SHA 通过。
- [ ] Railway PR Environment 与 Vercel Preview 互相指向正确。
- [ ] 合并 `dev` 后，在真实 dev 域名验证 canonical、重定向、sitemap、robots 与关键页面。
- [ ] 标准发布时按 `dev → staging → main` 完成每一环境的部署与浏览器验收。

## 13. 风险与缓解

| 风险 | 缓解 |
| --- | --- |
| 合并页面造成短期波动 | 只在意图和证据支持时合并；保留 301 映射和监控 |
| 内容增长再次产生薄页 | 发布前强制 query ownership、事实核验和差异化证据 |
| GSC 与 GA4 数字不一致 | 页面展示数据延迟、范围和归因口径，不要求逐行相等 |
| EMEA 样本小导致误判 | 分区展示但用较长 cohort，不按单周波动改策略 |
| OAuth 或 API 同步失败导致盲区 | 数据 freshness、重试、告警和人工重新授权 |
| 过度关注排名 | 评审始终先看合格注册与激活，再解释排名 |

## 14. 依赖与责任边界

- Marketing / SEO：query map、内容 brief、页面质量与外部引用。
- Product / Engineering：页面实现、重定向、canonical、数据事件与 Admin。
- Developer Experience：API 示例与技术事实核验。
- Data / Analytics：指标口径、brand 分类、cohort 和数据质量。
- Design：真实产品资产、图示与页面可读性。
- Legal / Privacy：OAuth 数据用途、隐私政策和数据保留说明。

## 15. 官方参考

- [Google Search Essentials](https://developers.google.com/search/docs/essentials)
- [Creating helpful, reliable, people-first content](https://developers.google.com/search/docs/fundamentals/creating-helpful-content)
- [Consolidate duplicate URLs](https://developers.google.com/search/docs/crawling-indexing/consolidate-duplicate-urls)
- [Build and submit a sitemap](https://developers.google.com/search/docs/crawling-indexing/sitemaps/build-sitemap)
- [Search Console Search Analytics API](https://developers.google.com/webmaster-tools/v1/searchanalytics/query)
