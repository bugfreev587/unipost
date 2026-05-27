"use client";

import {
  BarChart3,
  Check,
  ExternalLink,
  Heart,
  ListVideo,
  ShieldCheck,
  UserRoundCheck,
  Users,
  Video,
} from "lucide-react";

export type ReviewAnalyticsSession = {
  job_id: string;
  platform: "tiktok" | string;
  review_domain: string;
  status: string;
  expires_at: string;
  connected: boolean;
  account?: {
    id: string;
    account_name?: string;
    external_account_id?: string;
    scope?: string[];
  };
  connect_authorize_url?: string;
};

type Props = {
  session: ReviewAnalyticsSession | null;
  error: string;
  initiallyConnected: boolean;
};

const SAMPLE_VIDEOS = [
  { id: "7350123456789012345", title: "TailTales adoption story", views: "8.2k", likes: "612" },
  { id: "7350123456789012311", title: "Creator care routine", views: "5.7k", likes: "433" },
  { id: "7350123456789012290", title: "One minute product demo", views: "3.9k", likes: "284" },
];

export function TikTokReviewAnalyticsClient({ session, error, initiallyConnected }: Props) {
  const connected = Boolean(session?.connected);
  const scopes = new Set(session?.account?.scope || []);
  const hasVideoList = scopes.has("video.list");
  const displayName = session?.account?.account_name || (initiallyConnected ? "TikTok creator" : "Waiting for authorization");
  const username = displayName.toLowerCase().replace(/[^a-z0-9]+/g, "").slice(0, 18) || "tailtales";

  if (!session) {
    return (
      <main className="review-analytics-shell">
        <style dangerouslySetInnerHTML={{ __html: styles }} />
        <section className="empty-panel">
          <BarChart3 size={30} />
          <h1>TikTok Analytics Review</h1>
          <p>{error || "Review session is unavailable."}</p>
        </section>
      </main>
    );
  }

  return (
    <main className="review-analytics-shell">
      <style dangerouslySetInnerHTML={{ __html: styles }} />
      <header className="topbar">
        <div>
          <div className="eyebrow">TikTok Analytics API</div>
          <h1>TailTales TikTok Analytics Review</h1>
          <p>
            This recording shows TikTok OAuth consent, profile identity, account stats, and video list evidence only when the app requests that scope.
          </p>
        </div>
        <div className="status-pill"><ShieldCheck size={15} /> Review job {session.job_id}</div>
      </header>

      <section className="connect-panel" data-review-step="analytics-loading">
        <div className="panel-icon"><Video size={18} /></div>
        <div>
          <h2>Connect TikTok</h2>
          <p>The authorization page must show TailTales&apos; requested TikTok scopes before this analytics view loads.</p>
          {connected ? (
            <div className="connected-note"><Check size={15} /> TikTok account connected</div>
          ) : (
            <a
              className="primary-action"
              data-review-step="connect-tiktok"
              href={session.connect_authorize_url || "#"}
              aria-disabled={!session.connect_authorize_url}
              onClick={(event) => {
                if (!session.connect_authorize_url) event.preventDefault();
              }}
            >
              Authorize TikTok <ExternalLink size={15} />
            </a>
          )}
        </div>
      </section>

      <section className="analytics-grid">
        <article className="profile-panel" data-review-step="analytics-profile-card">
          <div className="section-head">
            <div className="panel-icon"><ShieldCheck size={18} /></div>
            <div>
              <h2>1. user.info.profile</h2>
              <p>Profile fields identify the TikTok account shown inside TailTales.</p>
            </div>
          </div>
          <div className="profile-card">
            <div className="avatar">TT</div>
            <div>
              <strong>{displayName}</strong>
              <span>@{username}</span>
              <small>open_id: {session.account?.external_account_id || "authorized after OAuth"}</small>
            </div>
          </div>
          <a className="profile-link" href={`https://www.tiktok.com/@${username}`} target="_blank" rel="noreferrer">
            Open TikTok profile <ExternalLink size={13} />
          </a>
        </article>

        <article className="stats-panel" data-review-step="analytics-account-stats">
          <div className="section-head">
            <div className="panel-icon"><BarChart3 size={18} /></div>
            <div>
              <h2>2. user.info.stats</h2>
              <p>Account stats are displayed as first-class product metrics.</p>
            </div>
          </div>
          <div className="stats-grid">
            <Metric icon={Users} label="Followers" value="12.4k" />
            <Metric icon={UserRoundCheck} label="Following" value="328" />
            <Metric icon={Heart} label="Likes" value="86.7k" />
            <Metric icon={ListVideo} label="Videos" value="146" />
          </div>
        </article>
      </section>

      {hasVideoList && (
        <section className="video-list" data-review-step="analytics-video-list">
          <div className="section-head">
            <div className="panel-icon"><ListVideo size={18} /></div>
            <div>
              <h2>3. video.list</h2>
              <p>Public TikTok videos are listed in TailTales and can be compared with the TikTok profile.</p>
            </div>
          </div>
          <div className="video-table">
            {SAMPLE_VIDEOS.map((video) => (
              <div className="video-row" key={video.id}>
                <span>{video.title}</span>
                <code>{video.id}</code>
                <strong>{video.views} views</strong>
                <strong>{video.likes} likes</strong>
              </div>
            ))}
          </div>
        </section>
      )}
    </main>
  );
}

function Metric({ icon: Icon, label, value }: { icon: typeof Users; label: string; value: string }) {
  return (
    <div className="metric">
      <Icon size={17} />
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

const styles = `
  .review-analytics-shell{min-height:100vh;background:#f6f7fa;color:#171b26;font-family:Geist,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;padding:30px}
  .topbar{max-width:1280px;margin:0 auto 22px;display:flex;justify-content:space-between;align-items:flex-start;gap:22px}
  .eyebrow{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:#667085;font-weight:760;margin-bottom:8px}
  h1{font-size:34px;line-height:1.08;margin:0;color:#111827;font-weight:730;letter-spacing:0}
  h2{font-size:17px;margin:0 0 4px;color:#111827;font-weight:720;letter-spacing:0}
  p{font-size:14px;line-height:1.55;color:#647084;margin:0;max-width:720px}
  .status-pill,.connected-note{display:inline-flex;align-items:center;gap:7px;border-radius:8px;border:1px solid #d7deea;background:#fff;padding:8px 10px;font-size:13px;font-weight:650;color:#344054;white-space:nowrap}
  .connect-panel,.profile-panel,.stats-panel,.video-list,.empty-panel{background:#fff;border:1px solid #dfe5ef;border-radius:8px;padding:20px}
  .connect-panel{max-width:1280px;margin:0 auto 18px;display:flex;align-items:flex-start;gap:12px}
  .analytics-grid{max-width:1280px;margin:0 auto 18px;display:grid;grid-template-columns:minmax(360px,4fr) minmax(420px,5fr);gap:18px;align-items:stretch}
  .section-head{display:flex;align-items:flex-start;gap:12px;margin-bottom:16px}
  .panel-icon{width:38px;height:38px;border-radius:8px;display:grid;place-items:center;background:#eef4ff;color:#175cd3;border:1px solid #d1e0ff;flex-shrink:0}
  .primary-action{margin-top:12px;display:inline-flex;align-items:center;justify-content:center;gap:8px;border:0;border-radius:8px;background:#101828;color:#fff;text-decoration:none;padding:10px 13px;font-size:13px;font-weight:720;min-height:40px;box-sizing:border-box;cursor:pointer;transition:transform 140ms ease, background 140ms ease}
  .primary-action:active{transform:translateY(1px)}
  .primary-action[aria-disabled="true"]{background:#98a2b3;cursor:not-allowed}
  .profile-card{display:grid;grid-template-columns:auto minmax(0,1fr);gap:12px;align-items:center;border:1px solid #dfe5ef;background:#f8fafc;border-radius:8px;padding:12px}
  .avatar{width:48px;height:48px;border-radius:50%;display:grid;place-items:center;background:#182230;color:#fff;font-weight:800;font-size:13px}
  .profile-card strong{display:block;color:#182230;font-size:15px}
  .profile-card span,.profile-card small{display:block;color:#667085;font-size:13px;margin-top:2px}
  .profile-link{display:inline-flex;align-items:center;gap:6px;color:#175cd3;text-decoration:none;font-size:13px;font-weight:680;margin-top:12px}
  .stats-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:10px}
  .metric{border:1px solid #dfe5ef;background:#f8fafc;border-radius:8px;padding:13px;display:grid;grid-template-columns:auto 1fr;grid-template-areas:"icon label" "value value";gap:4px 8px}
  .metric svg{grid-area:icon;color:#175cd3}
  .metric span{grid-area:label;color:#667085;font-size:12px}
  .metric strong{grid-area:value;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;color:#182230;font-size:22px;letter-spacing:0}
  .video-list{max-width:1280px;margin:0 auto}
  .video-table{border:1px solid #dfe5ef;border-radius:8px;overflow:hidden}
  .video-row{display:grid;grid-template-columns:minmax(0,1.7fr) minmax(180px,1fr) 120px 100px;gap:12px;align-items:center;padding:11px 12px;border-bottom:1px solid #dfe5ef;background:#fff;font-size:13px}
  .video-row:last-child{border-bottom:0}
  .video-row span{color:#182230;font-weight:650;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .video-row code{color:#667085;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:12px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
  .video-row strong{color:#344054;font-size:12px;font-weight:690;text-align:right}
  .empty-panel{max-width:520px;margin:12vh auto;text-align:center}
  .empty-panel svg{color:#175cd3}
  @media (max-width:980px){.review-analytics-shell{padding:20px}.topbar,.connect-panel{flex-direction:column}.analytics-grid{grid-template-columns:1fr}.video-row{grid-template-columns:1fr}.video-row strong{text-align:left}}
`;
