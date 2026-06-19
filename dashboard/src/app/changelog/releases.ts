export type ChangelogCategory = "api" | "sdk" | "dashboard" | "platform" | "dx" | "reliability";
export type ChangelogImpact = "new" | "improved" | "changed" | "fixed";
export type SdkEcosystem = "npm" | "pip" | "go" | "maven";

export type ChangelogLink = {
  label: string;
  href: string;
};

export type SdkVersion = {
  ecosystem: SdkEcosystem;
  packageName: string;
  version: string;
  href: string;
  installCommand?: string;
};

export type ChangelogRelease = {
  id: string;
  date: string;
  displayDate?: string;
  title: string;
  summary: string;
  category: ChangelogCategory;
  impact: ChangelogImpact;
  isBreaking: boolean;
  sdkVersions?: SdkVersion[];
  links: ChangelogLink[];
  sourceLinks: ChangelogLink[];
};

export const categoryLabels: Record<ChangelogCategory, string> = {
  api: "API",
  sdk: "SDK",
  dashboard: "Dashboard",
  platform: "Platform",
  dx: "DX",
  reliability: "Reliability",
};

export const impactLabels: Record<ChangelogImpact, string> = {
  new: "New",
  improved: "Improved",
  changed: "Changed",
  fixed: "Fixed",
};

export const changelogReleases: ChangelogRelease[] = [
  {
    id: "cli-0-3-1-agent-context-auth-reuse",
    date: "2026-06-18",
    displayDate: "June 18, 2026",
    title: "CLI 0.3.1 setup-token reuse and agent context",
    summary:
      "The UniPost CLI now reuses a valid local binding instead of consuming a Dashboard setup token, adds explicit --replace-key/--reauth replacement wording, and includes recent posts plus a status summary in agent context output.",
    category: "dx",
    impact: "fixed",
    isBreaking: false,
    sdkVersions: [
      {
        ecosystem: "npm",
        packageName: "@unipost/cli",
        version: "0.3.1",
        href: "https://www.npmjs.com/package/@unipost/cli",
        installCommand: "npm install -g @unipost/cli@0.3.1",
      },
    ],
    links: [
      { label: "CLI overview", href: "/docs/cli" },
      { label: "CLI reference", href: "/docs/cli/reference" },
    ],
    sourceLinks: [
      { label: "npm package", href: "https://www.npmjs.com/package/@unipost/cli" },
    ],
  },
  {
    id: "cli-0-3-0-auth-onboarding",
    date: "2026-06-18",
    displayDate: "June 18, 2026",
    title: "CLI 0.3.0 auth onboarding cleanup",
    summary:
      "The UniPost CLI now treats auth as a single local binding: API-key login stores a secure local credential on macOS, metadata-only mode is explicit, replacing an existing binding requires confirmation, auth status classifies missing and metadata-only setups, and agent debug skills start with an auth readiness check.",
    category: "dx",
    impact: "improved",
    isBreaking: false,
    sdkVersions: [
      {
        ecosystem: "npm",
        packageName: "@unipost/cli",
        version: "0.3.0",
        href: "https://www.npmjs.com/package/@unipost/cli",
        installCommand: "npm install -g @unipost/cli@0.3.0",
      },
    ],
    links: [
      { label: "CLI overview", href: "/docs/cli" },
      { label: "CLI reference", href: "/docs/cli/reference" },
      { label: "AI-assisted debugging", href: "/docs/cli/agent-debug" },
    ],
    sourceLinks: [
      { label: "npm package", href: "https://www.npmjs.com/package/@unipost/cli" },
    ],
  },
  {
    id: "sdk-0-4-0",
    date: "2026-06-18",
    displayDate: "June 18, 2026",
    title: "Multi-language SDKs 0.4.0",
    summary:
      "The official JavaScript, Python, Go, and Java SDKs reached 0.4.0 across their public registries, aligning client releases around Analytics Explorer, Developer Logs, and the latest API coverage.",
    category: "sdk",
    impact: "improved",
    isBreaking: false,
    sdkVersions: [
      {
        ecosystem: "npm",
        packageName: "@unipost/sdk",
        version: "0.4.0",
        href: "https://www.npmjs.com/package/@unipost/sdk",
        installCommand: "npm install @unipost/sdk@0.4.0",
      },
      {
        ecosystem: "pip",
        packageName: "unipost",
        version: "0.4.0",
        href: "https://pypi.org/project/unipost/0.4.0/",
        installCommand: "pip install unipost==0.4.0",
      },
      {
        ecosystem: "go",
        packageName: "github.com/unipost-dev/sdk-go",
        version: "0.4.0",
        href: "https://pkg.go.dev/github.com/unipost-dev/sdk-go@v0.4.0",
        installCommand: "go get github.com/unipost-dev/sdk-go@v0.4.0",
      },
      {
        ecosystem: "maven",
        packageName: "dev.unipost:sdk-java",
        version: "0.4.0",
        href: "https://repo1.maven.org/maven2/dev/unipost/sdk-java/0.4.0/",
        installCommand: 'implementation("dev.unipost:sdk-java:0.4.0")',
      },
    ],
    links: [
      { label: "SDK docs", href: "/docs/sdk" },
      { label: "Coverage matrix", href: "https://github.com/bugfreev587/unipost/blob/dev/docs/sdk-api-coverage-matrix.md" },
      { label: "npm package", href: "https://www.npmjs.com/package/@unipost/sdk" },
      { label: "PyPI package", href: "https://pypi.org/project/unipost/0.4.0/" },
      { label: "Go package", href: "https://pkg.go.dev/github.com/unipost-dev/sdk-go@v0.4.0" },
      { label: "Maven artifact", href: "https://repo1.maven.org/maven2/dev/unipost/sdk-java/0.4.0/" },
    ],
    sourceLinks: [
      { label: "npm version", href: "https://www.npmjs.com/package/@unipost/sdk/v/0.4.0" },
      { label: "PyPI release", href: "https://pypi.org/project/unipost/0.4.0/" },
      { label: "Go module tag", href: "https://github.com/unipost-dev/sdk-go/tree/v0.4.0" },
      { label: "Maven Central", href: "https://repo1.maven.org/maven2/dev/unipost/sdk-java/0.4.0/" },
    ],
  },
  {
    id: "agent-debug-kit",
    date: "2026-06-17",
    displayDate: "June 17, 2026",
    title: "Agent Debug Kit and CLI 0.2.0",
    summary:
      "The UniPost CLI added doctor diagnose, verify, explain, support-bundle, and logs commands with a stable doctor.v1 JSON contract, plus first-party agent skill packages for Claude Code and Codex.",
    category: "dx",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "Agent Debug Kit", href: "/docs/cli/agent-debug" },
      { label: "CLI reference", href: "/docs/cli/reference" },
      { label: "AI Agent Guide", href: "/docs/cli/agents" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/b06b7bcd" },
      { label: "Redaction hardening", href: "https://github.com/bugfreev587/unipost/commit/1501e81a" },
    ],
  },
  {
    id: "local-connect-testing",
    date: "2026-06-17",
    displayDate: "June 17, 2026",
    title: "Local Connect testing guide and URL helper",
    summary:
      "Developer docs and tooling now make local OAuth testing easier by generating Connect Session URLs, clarifying dev origins, and documenting the callback flow for localhost integrations.",
    category: "dx",
    impact: "improved",
    isBreaking: false,
    links: [
      { label: "Local Connect testing", href: "/docs/local-connect-test" },
      { label: "Connect Sessions", href: "/docs/connect-sessions" },
      { label: "Create session API", href: "/docs/api/connect/sessions/create" },
    ],
    sourceLinks: [
      { label: "URL helper", href: "https://github.com/bugfreev587/unipost/commit/dfd1b690" },
      { label: "Testing guide", href: "https://github.com/bugfreev587/unipost/commit/c59b542f" },
    ],
  },
  {
    id: "developer-logs-api",
    date: "2026-06-17",
    displayDate: "June 17, 2026",
    title: "Developer Logs API",
    summary:
      "Workspace-scoped developer logs are available over REST with cursor pagination and over SSE for near real-time ingestion. Logs stay isolated to the authenticated workspace and include redacted payload details on the detail endpoint.",
    category: "reliability",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "Logs overview", href: "/docs/api/logs" },
      { label: "List logs", href: "/docs/api/logs/list" },
      { label: "Stream logs", href: "/docs/api/logs/stream" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/1f8d4cf0" },
      { label: "Docs source", href: "https://github.com/bugfreev587/unipost/blob/dev/dashboard/src/app/docs/api/logs/page.tsx" },
    ],
  },
  {
    id: "tiktok-analytics-api",
    date: "2026-06-17",
    displayDate: "June 17, 2026",
    title: "TikTok Analytics API",
    summary:
      "TikTok analytics docs now cover profile details, account metrics, public video inventory, and UniPost-published post analytics for reporting flows when connected accounts grant the required analytics permissions.",
    category: "api",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "TikTok analytics", href: "/docs/api/analytics/tiktok" },
      { label: "Account metrics", href: "/docs/api/analytics/tiktok/account-metrics" },
      { label: "Public videos", href: "/docs/api/analytics/tiktok/videos" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/6fdc9afa" },
      { label: "Docs source", href: "https://github.com/bugfreev587/unipost/blob/dev/dashboard/src/app/docs/api/analytics/tiktok/page.tsx" },
    ],
  },
  {
    id: "api-metrics-surfaces",
    date: "2026-06-09",
    displayDate: "June 9, 2026",
    title: "API Metrics surfaces",
    summary:
      "API metrics endpoints and dashboard surfaces give teams a clearer view of API-key-authenticated traffic, status code mix, trends, and usage health.",
    category: "reliability",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "API metrics", href: "/docs/api/api-metrics/overall" },
      { label: "Summary", href: "/docs/api/api-metrics/summary" },
      { label: "Trend", href: "/docs/api/api-metrics/trend" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/57207f65" },
      { label: "Docs split", href: "https://github.com/bugfreev587/unipost/commit/0def99fd" },
    ],
  },
  {
    id: "cli-agent-guide",
    date: "2026-06-04",
    displayDate: "June 4, 2026",
    title: "CLI agent guide and command reference",
    summary:
      "The CLI docs now separate overview, grouped command reference, and AI agent guidance so teams can install once, bootstrap safely, and hand structured UniPost context to local coding agents.",
    category: "dx",
    impact: "improved",
    isBreaking: false,
    links: [
      { label: "CLI overview", href: "/docs/cli" },
      { label: "AI Agent Guide", href: "/docs/cli/agents" },
      { label: "CLI Reference", href: "/docs/cli/reference" },
    ],
    sourceLinks: [
      { label: "Agent guide commit", href: "https://github.com/bugfreev587/unipost/commit/10f432f8" },
      { label: "Reference commit", href: "https://github.com/bugfreev587/unipost/commit/4688bd0a" },
    ],
  },
  {
    id: "platform-credentials-hosted-connect",
    date: "2026-06-01",
    displayDate: "June 1, 2026",
    title: "Hosted Connect and Platform Credentials split",
    summary:
      "Developer docs now separate hosted Connect branding from workspace-owned platform credentials, making it clearer when teams should use UniPost-hosted OAuth and when they should bring their own app credentials.",
    category: "platform",
    impact: "improved",
    isBreaking: false,
    links: [
      { label: "Platform Credentials", href: "/docs/platform-credentials" },
      { label: "Hosted Connect", href: "/docs/white-label" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/b1db496b" },
      { label: "Platform docs source", href: "https://github.com/bugfreev587/unipost/blob/dev/dashboard/src/app/docs/platform-credentials/page.tsx" },
    ],
  },
  {
    id: "posts-calendar-editing",
    date: "2026-05-31",
    displayDate: "May 31, 2026",
    title: "Posts calendar and scheduled-post editing",
    summary:
      "The dashboard gained a posts calendar view and scheduled-post editing, giving teams a clearer planning surface for upcoming campaigns without leaving the publishing workflow.",
    category: "dashboard",
    impact: "improved",
    isBreaking: false,
    links: [
      { label: "Update post API", href: "/docs/api/posts/update" },
      { label: "List posts API", href: "/docs/api/posts/list" },
    ],
    sourceLinks: [
      { label: "Calendar view", href: "https://github.com/bugfreev587/unipost/commit/e2f542df" },
      { label: "Scheduled editing", href: "https://github.com/bugfreev587/unipost/commit/54f31942" },
    ],
  },
  {
    id: "white-label-logo-uploads",
    date: "2026-05-30",
    displayDate: "May 30, 2026",
    title: "White-label profile logo uploads",
    summary:
      "White-label hosted Connect setup added profile logo uploads so customer-facing OAuth surfaces can carry workspace branding alongside platform connection flows.",
    category: "platform",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "Hosted Connect", href: "/docs/white-label" },
      { label: "Platform Credentials", href: "/docs/platform-credentials" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/5d2b20f8" },
      { label: "Implementation plan", href: "https://github.com/bugfreev587/unipost/blob/dev/docs/prd-white-label-profile-branding-logo-upload.md" },
    ],
  },
  {
    id: "analytics-explorer-api",
    date: "2026-05-26",
    displayDate: "May 26, 2026",
    title: "Analytics Explorer API",
    summary:
      "The analytics API surface expanded to summary, trend, rollup, post-level rows, CSV export, platform availability, and refresh endpoints, with docs regression coverage added for the new surface.",
    category: "api",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "Analytics overview", href: "/docs/api/analytics" },
      { label: "Post analytics", href: "/docs/api/analytics/posts" },
      { label: "CSV export", href: "/docs/api/analytics/posts/export" },
      { label: "Platforms", href: "/docs/api/analytics/platforms" },
    ],
    sourceLinks: [
      { label: "API expansion", href: "https://github.com/bugfreev587/unipost/commit/609f11ac" },
      { label: "Docs regression", href: "https://github.com/bugfreev587/unipost/commit/ec615239" },
    ],
  },
  {
    id: "connect-sessions-guide",
    date: "2026-05-24",
    displayDate: "May 24, 2026",
    title: "Connect Sessions guide",
    summary:
      "The public docs added a dedicated Connect Sessions guide for hosted OAuth onboarding, including how customer-owned accounts move from connection to usable social account IDs.",
    category: "dx",
    impact: "new",
    isBreaking: false,
    links: [
      { label: "Connect Sessions", href: "/docs/connect-sessions" },
      { label: "Create session API", href: "/docs/api/connect/sessions/create" },
    ],
    sourceLinks: [
      { label: "Release commit", href: "https://github.com/bugfreev587/unipost/commit/06c69c7e" },
      { label: "Docs source", href: "https://github.com/bugfreev587/unipost/blob/dev/dashboard/src/app/docs/connect-sessions/page.tsx" },
    ],
  },
];

export const changelogCategories: ChangelogCategory[] = ["api", "sdk", "dashboard", "platform", "dx", "reliability"];
