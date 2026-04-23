import { DocsCodeTabs, DocsPage, DocsTable } from "../../_components/docs-shell";

const ANALYTICS_SNIPPETS = [
  {
    label: "Node.js",
    code: `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const rollup = await client.analytics.rollup({
  from: "2026-04-01T00:00:00Z",
  to: "2026-04-30T00:00:00Z",
  granularity: "day",
});

const postAnalytics = await client.posts.analytics("post_abc123");`,
  },
  {
    label: "Python",
    code: `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

rollup = client.analytics.rollup(
  from_date="2026-04-01T00:00:00Z",
  to_date="2026-04-30T00:00:00Z",
  granularity="day",
)

post_analytics = client.posts.analytics("post_abc123")`,
  },
  {
    label: "Go",
    code: `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  rollup, err := client.Analytics.Rollup(context.Background(), &unipost.RollupParams{
    From:        "2026-04-01T00:00:00Z",
    To:          "2026-04-30T00:00:00Z",
    Granularity: "day",
  })
  if err != nil {
    log.Fatal(err)
  }

  postAnalytics, err := client.Posts.Analytics(context.Background(), "post_abc123")
  if err != nil {
    log.Fatal(err)
  }

  _, _ = rollup, postAnalytics
}`,
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

      <h2 id="examples">SDK examples</h2>
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
