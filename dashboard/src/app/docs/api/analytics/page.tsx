import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const ANALYTICS_SNIPPETS = [
  {
    label: "Summary",
    code: `curl "https://api.unipost.dev/v1/analytics/summary?start_date=2026-04-01&end_date=2026-04-30" \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
  {
    label: "By platform",
    code: `curl "https://api.unipost.dev/v1/analytics/by-platform?start_date=2026-04-01&end_date=2026-04-30" \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
  {
    label: "Per-post analytics",
    code: `curl https://api.unipost.dev/v1/social-posts/post_abc123/analytics \\
  -H "Authorization: Bearer up_live_xxxx"`,
  },
];

export default function AnalyticsPage() {
  return (
    <DocsPage
      eyebrow="API Reference"
      title="Analytics"
      lead="UniPost analytics covers both per-post performance and workspace-level rollups. The goal is to let you power dashboards, reports, or agent feedback loops from one layer without stitching together platform-specific analytics APIs."
    >
      <h2 id="metric-model">Unified metric model</h2>
      <p>UniPost normalizes platform metrics into a shared shape: impressions, reach, likes, comments, shares, saves, clicks, and video views. Metrics a platform does not expose are returned as zero.</p>

      <h2 id="endpoints">Key endpoints</h2>
      <DocsTable
        columns={["Endpoint", "Use case"]}
        rows={[
          ["/v1/analytics/summary", "Totals and period-over-period overview"],
          ["/v1/analytics/trend", "Time-series trend charting"],
          ["/v1/analytics/by-platform", "Breakdown by destination network"],
          ["/v1/social-posts/{id}/analytics", "Detailed metrics for one post"],
        ]}
      />

      <h2 id="examples">Examples</h2>
      <DocsCodeTabs snippets={ANALYTICS_SNIPPETS} />

      <h2 id="behavior">Behavior notes</h2>
      <ul className="docs-list">
        <li>Analytics is cached and refreshed in the background for supported platforms.</li>
        <li>Soft-deleted posts are excluded from rollups and analytics views.</li>
        <li>Archived posts still appear in analytics because archive is an organizational state, not a reporting delete.</li>
      </ul>
    </DocsPage>
  );
}
