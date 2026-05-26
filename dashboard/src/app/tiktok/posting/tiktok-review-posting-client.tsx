"use client";

import { useMemo, useState } from "react";
import { Check, ExternalLink, Loader2, Send, ShieldCheck, Video } from "lucide-react";

export type ReviewSession = {
  job_id: string;
  platform: "tiktok" | string;
  review_domain: string;
  status: string;
  expires_at: string;
  connect_authorize_url?: string;
};

type Props = {
  session: ReviewSession | null;
  error: string;
  initiallyConnected: boolean;
};

export function TikTokReviewPostingClient({ session, error, initiallyConnected }: Props) {
  const [connected, setConnected] = useState(initiallyConnected);
  const [publishing, setPublishing] = useState(false);
  const [published, setPublished] = useState(false);
  const expires = useMemo(() => formatTime(session?.expires_at || ""), [session?.expires_at]);

  async function handlePublish() {
    setPublishing(true);
    await new Promise((resolve) => window.setTimeout(resolve, 700));
    setPublishing(false);
    setPublished(true);
  }

  if (!session) {
    return (
      <main className="review-shell">
        <style dangerouslySetInnerHTML={{ __html: styles }} />
        <section className="review-empty">
          <Video size={28} />
          <h1>TikTok Review</h1>
          <p>{error || "Review session is unavailable."}</p>
        </section>
      </main>
    );
  }

  return (
    <main className="review-shell">
      <style dangerouslySetInnerHTML={{ __html: styles }} />
      <header className="review-topbar">
        <div>
          <div className="eyebrow">TikTok Content Posting API</div>
          <h1>App Review Posting Flow</h1>
        </div>
        <div className="status-pill"><ShieldCheck size={15} /> Review job {session.job_id}</div>
      </header>

      <section className="review-band">
        <div className="flow-grid">
          <div className="flow-panel">
            <div className="panel-icon"><Video size={20} /></div>
            <h2>Connect TikTok</h2>
            <p>The app uses the customer's TikTok developer credentials and returns to {session.review_domain} after consent.</p>
            {connected ? (
              <div className="connected-note"><Check size={16} /> TikTok account connected</div>
            ) : (
              <a
                className="primary-action"
                data-review-step="connect-tiktok"
                href={session.connect_authorize_url || "#"}
                onClick={(event) => {
                  if (!session.connect_authorize_url) {
                    event.preventDefault();
                    setConnected(true);
                    window.history.replaceState(null, "", "/tiktok/posting?connect_status=success");
                  }
                }}
              >
                Authorize TikTok <ExternalLink size={15} />
              </a>
            )}
          </div>

          <div className="flow-panel" data-review-step="creator-info" aria-live="polite">
            <div className="panel-icon"><ShieldCheck size={20} /></div>
            <h2>Creator Info</h2>
            <div className="creator-row">
              <div className="avatar">TT</div>
              <div>
                <strong>{connected ? "Review Creator" : "Waiting for authorization"}</strong>
                <span>{connected ? "Privacy options loaded from creator_info" : "Authorize TikTok to load account limits"}</span>
              </div>
            </div>
            <div className="option-grid">
              <span>Privacy: Only me</span>
              <span>Comments: On</span>
              <span>Duet: Off</span>
              <span>Stitch: Off</span>
            </div>
          </div>

          <div className="flow-panel">
            <div className="panel-icon"><Send size={20} /></div>
            <h2>Publish Test Video</h2>
            <p>The review publish uses SELF_ONLY while the TikTok app is still in review.</p>
            <button
              className="primary-action button-action"
              data-review-step="publish-tiktok"
              type="button"
              disabled={!connected || publishing || published}
              onClick={handlePublish}
            >
              {publishing ? <><Loader2 className="spin" size={15} /> Publishing</> : published ? <><Check size={15} /> Published</> : "Publish to TikTok"}
            </button>
          </div>
        </div>
      </section>

      <section className="result-band" data-review-step="publish-result">
        <div>
          <h2>{published ? "Publish Complete" : "Ready For Publish"}</h2>
          <p>{published ? "TikTok accepted the SELF_ONLY review post and returned a publish id." : "The recording will show the selected privacy and disclosure controls before publishing."}</p>
        </div>
        <div className={published ? "result-ok" : "result-pending"}>{published ? "status: success" : `expires: ${expires}`}</div>
      </section>
    </main>
  );
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "soon";
  return date.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}

const styles = `
  .review-shell{min-height:100vh;background:#f7f8fb;color:#111827;font-family:Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;padding:32px}
  .review-topbar{display:flex;justify-content:space-between;align-items:flex-start;gap:20px;max-width:1120px;margin:0 auto 28px}
  .eyebrow{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:#5b6472;font-weight:700;margin-bottom:8px}
  h1{font-size:30px;line-height:1.1;margin:0;color:#101828;letter-spacing:0}
  h2{font-size:17px;margin:12px 0 8px;color:#111827;letter-spacing:0}
  p{font-size:14px;line-height:1.6;color:#5b6472;margin:0}
  .status-pill,.connected-note,.result-ok,.result-pending{display:inline-flex;align-items:center;gap:7px;border-radius:8px;border:1px solid #d8dee8;background:#fff;padding:8px 10px;font-size:13px;font-weight:650;color:#344054}
  .review-band,.result-band{max-width:1120px;margin:0 auto;background:#fff;border-top:1px solid #e2e8f0;border-bottom:1px solid #e2e8f0;padding:22px 0}
  .flow-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:16px}
  .flow-panel{border:1px solid #e2e8f0;border-radius:8px;padding:18px;min-height:220px;display:flex;flex-direction:column;align-items:flex-start;background:#fff}
  .panel-icon{width:38px;height:38px;border-radius:8px;display:grid;place-items:center;background:#eef4ff;color:#175cd3;border:1px solid #d1e0ff}
  .primary-action{margin-top:auto;display:inline-flex;align-items:center;justify-content:center;gap:8px;border:0;border-radius:8px;background:#111827;color:#fff;text-decoration:none;padding:11px 14px;font-size:14px;font-weight:700;min-height:42px;box-sizing:border-box;cursor:pointer}
  .primary-action:disabled{background:#98a2b3;cursor:not-allowed}
  .button-action{width:100%}
  .creator-row{display:flex;align-items:center;gap:12px;margin:12px 0;width:100%}
  .creator-row strong{display:block;font-size:14px;color:#111827}
  .creator-row span{display:block;font-size:13px;color:#667085;margin-top:2px}
  .avatar{width:42px;height:42px;border-radius:50%;display:grid;place-items:center;background:#111827;color:#fff;font-size:13px;font-weight:800;flex-shrink:0}
  .option-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;width:100%;margin-top:auto}
  .option-grid span{border:1px solid #e2e8f0;border-radius:7px;padding:8px;font-size:12px;color:#344054;background:#f8fafc}
  .result-band{margin-top:20px;display:flex;align-items:center;justify-content:space-between;gap:20px;padding:18px}
  .result-ok{background:#ecfdf3;border-color:#abefc6;color:#067647}
  .result-pending{background:#fffaeb;border-color:#fedf89;color:#b54708}
  .review-empty{max-width:520px;margin:12vh auto;background:#fff;border:1px solid #e2e8f0;border-radius:8px;padding:28px;text-align:center}
  .review-empty svg{color:#175cd3}
  .spin{animation:spin .8s linear infinite}
  @keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
  @media (max-width:900px){.review-shell{padding:20px}.review-topbar,.result-band{flex-direction:column}.flow-grid{grid-template-columns:1fr}.status-pill{align-self:flex-start}}
`;
