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
        YouTube Analytics V2 uses the YouTube Analytics API for richer date-ranged channel reporting. It is separate from basic
        account metrics: use <Link href="/docs/api/accounts/metrics">Get account metrics</Link> for live subscriber count, public
        video count, and lifetime channel views.
      </p>
      <p>
        These endpoints require the connected account to include <code>https://www.googleapis.com/auth/yt-analytics.readonly</code>.
        Monetary reports are not included and do not require <code>yt-analytics-monetary.readonly</code>.
      </p>

      <h2 id="endpoints">Endpoints</h2>
      <div className="docs-next-grid">
        <Link href="/docs/api/analytics/youtube/summary" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">Summary</div>
          <div className="docs-next-body">Channel-level totals for a date range.</div>
        </Link>
        <Link href="/docs/api/analytics/youtube/trend" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">Trend</div>
          <div className="docs-next-body">Daily channel metrics for a date range.</div>
        </Link>
        <Link href="/docs/api/analytics/youtube/videos" className="docs-next-card">
          <div className="docs-next-kicker">GET</div>
          <div className="docs-next-title">Top Videos</div>
          <div className="docs-next-body">Top video rows ranked by views.</div>
        </Link>
      </div>
    </DocsPage>
  );
}
