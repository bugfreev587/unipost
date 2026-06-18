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
    id: "sdk-javascript-0-4-0",
    date: "2026-06-18",
    displayDate: "June 18, 2026",
    title: "JavaScript SDK 0.4.0",
    summary:
      "The official JavaScript package reached 0.4.0 on npm. Use this entry as the SDK version source for JavaScript examples and install snippets.",
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
    ],
    links: [
      { label: "SDK docs", href: "/docs/sdk" },
      { label: "npm package", href: "https://www.npmjs.com/package/@unipost/sdk" },
    ],
    sourceLinks: [
      { label: "npm version", href: "https://www.npmjs.com/package/@unipost/sdk/v/0.4.0" },
      { label: "SDK docs source", href: "https://github.com/bugfreev587/unipost/blob/dev/dashboard/src/app/docs/sdk/page.tsx" },
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
      "TikTok analytics docs now cover profile, account metrics, public video inventory, and UniPost-published post analytics for production-ready TikTok reporting flows.",
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
