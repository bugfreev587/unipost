import Link from "next/link";
import { DocsPage } from "../../../_components/docs-shell";

export default function YouTubeAnalyticsOverviewPage() {
  return (
    <DocsPage
      eyebrow="Analytics API"
      title="YouTube Analytics"
      lead="Owner-authorized YouTube Analytics reports for connected YouTube channels."
      className="docs-page-wide"
    >
      <h2 id="overview">Overview</h2>
      <p>
        YouTube analytics in UniPost has two layers. V1 is the shared basic account metrics endpoint,{" "}
        <code>{"/v1/accounts/{account_id}/metrics"}</code>, for live subscriber count, public video count, and lifetime channel
        views. V2 uses the YouTube Analytics API for richer date-ranged channel reporting.
      </p>
      <p>
        V1 requires <code>youtube.readonly</code>. V2 requires the connected account to include{" "}
        <code>https://www.googleapis.com/auth/yt-analytics.readonly</code>. Monetary reports are not included and do not require{" "}
        <code>yt-analytics-monetary.readonly</code>.
      </p>
      <p>
        V2 starts with <code>{"/v1/accounts/{account_id}/youtube/analytics/summary"}</code>, then adds daily trend and top video
        endpoints under the same YouTube Analytics route family.
      </p>
      <p>
        In the dashboard, use Analytics - Platforms - YouTube to see V1 channel metrics beside V2 summary, daily trend, and top video
        reports for a connected YouTube channel.
      </p>

      <h2 id="endpoints">Endpoints</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/accounts/metrics" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">V1 account metrics</div>
          <div className="docs-next-body">Subscriber count, public video count, and lifetime channel views.</div>
        </Link>
        <Link href="/docs/api/analytics/youtube/summary" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">V2 summary</div>
          <div className="docs-next-body">Channel-level totals for a date range.</div>
        </Link>
        <Link href="/docs/api/analytics/youtube/trend" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">V2 trend</div>
          <div className="docs-next-body">Daily channel metrics for a date range.</div>
        </Link>
        <Link href="/docs/api/analytics/youtube/videos" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">V2 top videos</div>
          <div className="docs-next-body">Top video rows ranked by views.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
