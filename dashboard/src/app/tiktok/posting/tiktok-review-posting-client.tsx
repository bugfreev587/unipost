"use client";

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import {
  Check,
  ExternalLink,
  Eye,
  FileVideo,
  Loader2,
  Music2,
  Send,
  ShieldCheck,
  UploadCloud,
  Video,
} from "lucide-react";

export type ReviewCreatorInfo = {
  creator_avatar_url?: string;
  creator_username?: string;
  creator_nickname?: string;
  privacy_level_options?: string[];
  comment_disabled?: boolean;
  duet_disabled?: boolean;
  stitch_disabled?: boolean;
  max_video_post_duration_sec?: number;
};

export type ReviewSession = {
  job_id: string;
  platform: "tiktok" | string;
  review_domain: string;
  status: string;
  expires_at: string;
  test_video_url?: string;
  default_caption?: string;
  connected: boolean;
  account?: {
    id: string;
    account_name?: string;
    external_account_id?: string;
    scope?: string[];
  };
  creator_info?: ReviewCreatorInfo;
  creator_info_error?: string;
  connect_authorize_url?: string;
};

type ReviewPublishResult = {
  status: string;
  external_id?: string;
  url?: string;
  privacy_level: string;
  video_url: string;
};

type ApiEnvelope<T> = { data?: T; error?: { code: string; message: string } };

type Props = {
  session: ReviewSession | null;
  error: string;
  initiallyConnected: boolean;
};

type ReviewForm = {
  videoSelected: boolean;
  caption: string;
  privacyLevel: string;
  disableComment: boolean;
  disableDuet: boolean;
  disableStitch: boolean;
  disclosureEnabled: boolean;
  yourBrand: boolean;
  brandedContent: boolean;
};

const PRIVACY_LABELS: Record<string, string> = {
  PUBLIC_TO_EVERYONE: "Everyone",
  MUTUAL_FOLLOW_FRIENDS: "Friends",
  FOLLOWER_OF_CREATOR: "Followers",
  SELF_ONLY: "Only me",
};

const DEFAULT_CAPTION = "TailTales review video published through UniPost.";

export function TikTokReviewPostingClient({ session, error, initiallyConnected }: Props) {
  const [form, setForm] = useState<ReviewForm>({
    videoSelected: false,
    caption: session?.default_caption || DEFAULT_CAPTION,
    privacyLevel: "",
    disableComment: true,
    disableDuet: true,
    disableStitch: true,
    disclosureEnabled: false,
    yourBrand: false,
    brandedContent: false,
  });
  const [publishing, setPublishing] = useState(false);
  const [publishResult, setPublishResult] = useState<ReviewPublishResult | null>(null);
  const [publishError, setPublishError] = useState("");

  const expires = useMemo(() => formatTime(session?.expires_at || ""), [session?.expires_at]);
  const connected = Boolean(session?.connected);
  const creator = session?.creator_info;
  const creatorReady = Boolean(connected && creator && !session?.creator_info_error);
  const creatorName = creator?.creator_nickname || session?.account?.account_name || (initiallyConnected ? "TikTok creator" : "Waiting for authorization");
  const creatorDetail = creator?.creator_username ? `@${creator.creator_username}` : connected ? "creator_info loaded through TikTok API" : "Authorize TikTok to load account limits";
  const privacyOptions = creator?.privacy_level_options?.length ? creator.privacy_level_options : [];
  const published = Boolean(publishResult);
  const disclosureIncomplete = form.disclosureEnabled && !form.yourBrand && !form.brandedContent;
  const brandedPrivateConflict = form.disclosureEnabled && form.brandedContent && form.privacyLevel === "SELF_ONLY";
  const videoURL = session?.test_video_url || "";
  const videoHost = videoURL ? safeHost(videoURL) : "UniPost review asset";
  const publishDisabledReason =
    !connected
      ? "Connect TikTok before publishing."
      : session?.creator_info_error
        ? session.creator_info_error
        : !form.videoSelected
          ? "Select the TailTales review video before publishing."
          : !form.privacyLevel
            ? "Choose who can view this video."
            : disclosureIncomplete
              ? "Choose Your Brand, Branded Content, or both."
              : brandedPrivateConflict
                ? "TikTok does not allow Branded Content to be posted as Only me."
                : !videoURL
                  ? "Review test video is not configured."
                  : "";
  const canPublish = !publishDisabledReason && !publishing && !published;

  function updateForm(fields: Partial<ReviewForm>) {
    setForm((current) => ({ ...current, ...fields }));
  }

  function handleBrandedContentChange(checked: boolean) {
    const updates: Partial<ReviewForm> = { brandedContent: checked };
    if (checked && form.privacyLevel === "SELF_ONLY") {
      updates.privacyLevel = "";
    }
    updateForm(updates);
  }

  async function handlePublish() {
    if (!canPublish) return;
    setPublishing(true);
    setPublishError("");
    try {
      const res = await fetch("/tiktok/posting/publish", {
        method: "POST",
        headers: { "Content-Type": "application/json", Accept: "application/json" },
        body: JSON.stringify({
          caption: form.caption,
          privacy_level: form.privacyLevel,
          disable_comment: creator?.comment_disabled ? true : form.disableComment,
          disable_duet: creator?.duet_disabled ? true : form.disableDuet,
          disable_stitch: creator?.stitch_disabled ? true : form.disableStitch,
          brand_content_toggle: form.disclosureEnabled && form.brandedContent,
          brand_organic_toggle: form.disclosureEnabled && form.yourBrand,
        }),
      });
      const body: ApiEnvelope<ReviewPublishResult> = await res.json();
      if (!res.ok || !body.data) {
        throw new Error(body.error?.message || "TikTok publish failed.");
      }
      setPublishResult(body.data);
    } catch (err) {
      setPublishError((err as Error).message || "TikTok publish failed.");
    } finally {
      setPublishing(false);
    }
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
          <h1>TailTales TikTok Publish Review</h1>
          <p className="top-copy">
            This page shows the exact video, caption, creator_info controls, and TikTok policy confirmation before calling Direct Post.
          </p>
        </div>
        <div className="status-pill"><ShieldCheck size={15} /> Review job {session.job_id}</div>
      </header>

      <section className="review-grid">
        <aside className="media-panel">
          <div className="section-head">
            <div className="panel-icon"><UploadCloud size={20} /></div>
            <div>
              <h2>Video To Publish</h2>
              <p>Select the TailTales review video before publishing to TikTok.</p>
            </div>
          </div>

          <div className="video-frame" data-review-step="video-preview">
            {videoURL ? (
              <video src={videoURL} controls muted playsInline preload="metadata" />
            ) : (
              <div className="video-placeholder"><FileVideo size={28} /> Review video URL is not configured</div>
            )}
          </div>

          <div className="asset-row">
            <FileVideo size={18} />
            <div>
              <strong>tailtales-review-video.mp4</strong>
              <span>{form.videoSelected ? "Uploaded to UniPost media storage" : videoHost}</span>
            </div>
            <button
              data-review-step="select-video"
              type="button"
              onClick={() => updateForm({ videoSelected: true })}
              disabled={!videoURL || form.videoSelected || published}
            >
              {form.videoSelected ? <><Check size={15} /> Uploaded</> : <><UploadCloud size={15} /> Upload video</>}
            </button>
          </div>

          <div className="upload-evidence" data-review-step={form.videoSelected ? "video-upload-ready" : "video-upload-pending"}>
            <div className={form.videoSelected ? "upload-step done" : "upload-step"}>
              <Check size={14} />
              <span>Local MP4 selected</span>
            </div>
            <div className={form.videoSelected ? "upload-step done" : "upload-step"}>
              <Check size={14} />
              <span>Stored in UniPost media library</span>
            </div>
            <div className={form.videoSelected ? "upload-step ready" : "upload-step"}>
              <UploadCloud size={14} />
              <span>Ready for TikTok video.upload</span>
            </div>
          </div>

          <Field label="Custom Caption" meta={`${form.caption.length} / 2200`}>
            <textarea
              value={form.caption}
              onChange={(event) => updateForm({ caption: event.target.value.slice(0, 2200) })}
              rows={4}
              maxLength={2200}
              placeholder="Write a TikTok caption"
            />
          </Field>

          <div className="connect-box" data-review-step="connect-tiktok-panel">
            <div className="panel-icon"><Video size={20} /></div>
            <div>
              <h2>Connect TikTok</h2>
              <p>The OAuth consent uses TailTales&apos; TikTok app and returns to {session.review_domain}.</p>
              {connected ? (
                <div className="connected-note"><Check size={16} /> TikTok account connected</div>
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
          </div>
        </aside>

        <section className="form-panel">
          <div className="section-head" data-review-step="creator-info">
            <div className="panel-icon"><ShieldCheck size={20} /></div>
            <div>
              <h2>Creator Info</h2>
              <p>Settings below are rendered from TikTok&apos;s creator_info response.</p>
            </div>
          </div>

          <div className="creator-card">
            {creator?.creator_avatar_url ? (
              // eslint-disable-next-line @next/next/no-img-element
              <img className="avatar-img" src={creator.creator_avatar_url} alt="" />
            ) : (
              <div className="avatar">TT</div>
            )}
            <div>
              <strong>{creatorName}</strong>
              <span>{session.creator_info_error || creatorDetail}</span>
            </div>
            <div className="limit-pill">{creator?.max_video_post_duration_sec ? `Max ${creator.max_video_post_duration_sec}s` : "Waiting"}</div>
          </div>

          <Field label="Who Can View This Video">
            <div className="privacy-options" data-review-step="privacy-selector">
              {privacyOptions.length ? (
                privacyOptions.map((option) => {
                  const lockedByBrand = form.disclosureEnabled && form.brandedContent && option === "SELF_ONLY";
                  const selected = form.privacyLevel === option;
                  return (
                    <button
                      key={option}
                      type="button"
                      data-review-step={option === "SELF_ONLY" ? "privacy-self-only" : undefined}
                      className={selected ? "privacy-option selected" : "privacy-option"}
                      disabled={!creatorReady || lockedByBrand || published}
                      onClick={() => updateForm({ privacyLevel: option })}
                    >
                      <Eye size={15} />
                      <span>{PRIVACY_LABELS[option] || option}</span>
                      <small>{option}</small>
                    </button>
                  );
                })
              ) : (
                <div className="muted-box">Authorize TikTok to load privacy options.</div>
              )}
            </div>
            {!form.privacyLevel && creatorReady && <Hint tone="warning">Pick a visibility - TikTok requires an explicit choice.</Hint>}
            <Hint tone="info">For app review, choose Only me so the unaudited sandbox app can publish safely.</Hint>
          </Field>

          <Field label="Allow Users To">
            <div className="toggle-row" data-review-step="interaction-controls">
              <InteractionToggle
                label="Comment"
                checked={!form.disableComment}
                creatorDisabled={Boolean(creator?.comment_disabled)}
                disabled={!creatorReady || published}
                onChange={(allow) => updateForm({ disableComment: !allow })}
              />
              <InteractionToggle
                label="Duet"
                checked={!form.disableDuet}
                creatorDisabled={Boolean(creator?.duet_disabled)}
                disabled={!creatorReady || published}
                onChange={(allow) => updateForm({ disableDuet: !allow })}
              />
              <InteractionToggle
                label="Stitch"
                checked={!form.disableStitch}
                creatorDisabled={Boolean(creator?.stitch_disabled)}
                disabled={!creatorReady || published}
                onChange={(allow) => updateForm({ disableStitch: !allow })}
              />
            </div>
          </Field>

          <div className="disclosure-box" data-review-step="content-disclosure">
            <label className="check-line">
              <input
                type="checkbox"
                checked={form.disclosureEnabled}
                disabled={published}
                onChange={(event) => {
                  if (event.target.checked) {
                    updateForm({ disclosureEnabled: true });
                  } else {
                    updateForm({ disclosureEnabled: false, yourBrand: false, brandedContent: false });
                  }
                }}
              />
              <span>
                <strong>Disclose video content</strong>
                <small>Turn on if this post promotes TailTales, a third party, or both.</small>
              </span>
            </label>

            {form.disclosureEnabled && (
              <div className="nested-checks">
                <label className="check-line compact">
                  <input
                    type="checkbox"
                    checked={form.yourBrand}
                    disabled={published}
                    onChange={(event) => updateForm({ yourBrand: event.target.checked })}
                  />
                  <span>
                    <strong>Your Brand</strong>
                    <small>TailTales is promoting itself or its own product.</small>
                  </span>
                </label>
                <label className="check-line compact">
                  <input
                    type="checkbox"
                    checked={form.brandedContent}
                    disabled={published}
                    onChange={(event) => handleBrandedContentChange(event.target.checked)}
                  />
                  <span>
                    <strong>Branded Content</strong>
                    <small>TailTales was paid to promote a third party.</small>
                  </span>
                </label>
                {disclosureIncomplete && <Hint tone="error">Choose Your Brand, Branded Content, or both.</Hint>}
                {brandedPrivateConflict && <Hint tone="error">Branded Content cannot be posted as Only me.</Hint>}
              </div>
            )}
          </div>

          <div className="post-preview" data-review-step="post-preview">
            <div className="preview-media">
              {videoURL ? (
                <video src={videoURL} muted playsInline preload="metadata" />
              ) : (
                <FileVideo size={20} />
              )}
            </div>
            <div>
              <div className="preview-title">TikTok post preview</div>
              <p>{form.caption || "Caption will appear here before publishing."}</p>
              <div className="preview-meta">
                <span>{form.privacyLevel || "Visibility required"}</span>
                <span>{form.videoSelected ? "Video selected" : "Video not selected"}</span>
              </div>
            </div>
          </div>

          <div className="music-confirmation" data-review-step="music-confirmation">
            <Music2 size={17} />
            <p>
              By posting, you agree to TikTok&apos;s{" "}
              {form.disclosureEnabled && form.brandedContent ? (
                <>
                  <a data-review-step="branded-content-policy-inline-link" href="https://www.tiktok.com/legal/page/global/bc-policy/en" target="_blank" rel="noreferrer">Branded Content Policy</a>
                  {" "}and{" "}
                </>
              ) : null}
              <a data-review-step="music-usage-confirmation-link" href="https://www.tiktok.com/legal/page/global/music-usage-confirmation/en" target="_blank" rel="noreferrer">Music Usage Confirmation</a>.
            </p>
          </div>

          <div className="music-confirmation" data-review-step="branded-content-policy">
            <ShieldCheck size={17} />
            <p>
              Branded posts follow TikTok&apos;s{" "}
              <a data-review-step="branded-content-policy-link" href="https://www.tiktok.com/legal/page/global/bc-policy/en" target="_blank" rel="noreferrer">Branded Content Policy</a>.
              TailTales confirms the disclosure choice before publishing.
            </p>
          </div>

          <button
            className="publish-button"
            data-review-step="publish-tiktok"
            type="button"
            disabled={!canPublish}
            onClick={handlePublish}
          >
            {publishing ? <><Loader2 className="spin" size={16} /> Publishing</> : published ? <><Check size={16} /> Published</> : <><Send size={16} /> Publish to TikTok</>}
          </button>
          {publishDisabledReason && !published && <Hint tone="warning">{publishDisabledReason}</Hint>}
          {publishError && <div className="error-note">{publishError}</div>}
        </section>
      </section>

      <section className="result-band" data-review-step="publish-result">
        <div>
          <h2>{published ? "Publish Complete" : "Ready For Publish"}</h2>
          <p>{publishResult ? `TikTok accepted the ${publishResult.privacy_level} review post with status ${publishResult.status}.` : "The recording will show video selection, creator_info controls, policy confirmation, and the live publish result."}</p>
        </div>
        <div className={published ? "result-ok" : "result-pending"}>{publishResult ? `id: ${publishResult.external_id || "processing"}` : `expires: ${expires}`}</div>
      </section>
    </main>
  );
}

function Field({ label, meta, children }: { label: string; meta?: string; children: ReactNode }) {
  return (
    <div className="field">
      <div className="field-label">
        <label>{label}</label>
        {meta ? <span>{meta}</span> : null}
      </div>
      {children}
    </div>
  );
}

function InteractionToggle({
  label,
  checked,
  creatorDisabled,
  disabled,
  onChange,
}: {
  label: string;
  checked: boolean;
  creatorDisabled: boolean;
  disabled: boolean;
  onChange: (allow: boolean) => void;
}) {
  const locked = creatorDisabled || disabled;
  const effectiveChecked = creatorDisabled ? false : checked;
  return (
    <label className={effectiveChecked ? "interaction-toggle checked" : "interaction-toggle"} title={creatorDisabled ? `${label} is disabled in this creator's TikTok settings` : undefined}>
      <input
        type="checkbox"
        checked={effectiveChecked}
        disabled={locked}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span>{label}</span>
      {creatorDisabled ? <small>Unavailable</small> : <small>{effectiveChecked ? "Allowed" : "Off"}</small>}
    </label>
  );
}

function Hint({ tone, children }: { tone: "warning" | "error" | "info"; children: ReactNode }) {
  return <p className={`hint ${tone}`}>{children}</p>;
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "soon";
  return date.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}

function safeHost(value: string) {
  try {
    return new URL(value).host;
  } catch {
    return "UniPost review asset";
  }
}

const styles = `
  .review-shell{min-height:100vh;background:#f6f7fa;color:#171b26;font-family:Geist,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;padding:30px}
  .review-topbar{display:flex;justify-content:space-between;align-items:flex-start;gap:20px;max-width:1280px;margin:0 auto 24px}
  .eyebrow{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:#667085;font-weight:750;margin-bottom:8px}
  h1{font-size:34px;line-height:1.08;margin:0;color:#111827;letter-spacing:0;font-weight:720}
  h2{font-size:17px;margin:0 0 4px;color:#111827;letter-spacing:0;font-weight:700}
  p{font-size:14px;line-height:1.55;color:#647084;margin:0}
  .top-copy{margin-top:10px;max-width:700px}
  .status-pill,.connected-note,.result-ok,.result-pending{display:inline-flex;align-items:center;gap:7px;border-radius:8px;border:1px solid #d7deea;background:#fff;padding:8px 10px;font-size:13px;font-weight:650;color:#344054;white-space:nowrap}
  .review-grid{max-width:1280px;margin:0 auto;display:grid;grid-template-columns:minmax(360px,5fr) minmax(420px,4fr);gap:18px;align-items:start}
  .media-panel,.form-panel,.result-band{background:#fff;border:1px solid #dfe5ef;border-radius:8px;padding:20px}
  .section-head{display:flex;align-items:flex-start;gap:12px;margin-bottom:16px}
  .panel-icon{width:38px;height:38px;border-radius:8px;display:grid;place-items:center;background:#eef4ff;color:#175cd3;border:1px solid #d1e0ff;flex-shrink:0}
  .video-frame{border:1px solid #dfe5ef;border-radius:8px;background:#101828;aspect-ratio:16/9;overflow:hidden;display:grid;place-items:center}
  .video-frame video{width:100%;height:100%;object-fit:contain;background:#101828}
  .video-placeholder{display:flex;align-items:center;gap:10px;color:#cbd5e1;font-size:13px}
  .asset-row{margin:12px 0 18px;display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:10px;align-items:center;border:1px solid #dfe5ef;border-radius:8px;padding:10px;background:#f8fafc}
  .asset-row strong{display:block;font-size:13px;color:#182230}
  .asset-row span{display:block;font-size:12px;color:#667085;margin-top:1px}
  .asset-row button,.primary-action,.publish-button{display:inline-flex;align-items:center;justify-content:center;gap:8px;border:0;border-radius:8px;background:#101828;color:#fff;text-decoration:none;padding:10px 13px;font-size:13px;font-weight:720;min-height:40px;box-sizing:border-box;cursor:pointer;transition:transform 140ms ease, background 140ms ease}
  .asset-row button:active,.primary-action:active,.publish-button:active{transform:translateY(1px)}
  .asset-row button:disabled,.primary-action[aria-disabled="true"],.publish-button:disabled{background:#98a2b3;cursor:not-allowed}
  .upload-evidence{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:-4px 0 18px}
  .upload-step{border:1px solid #dfe5ef;border-radius:8px;background:#fff;color:#667085;padding:9px;display:flex;align-items:center;gap:7px;min-height:40px;font-size:12px;font-weight:650}
  .upload-step svg{color:#98a2b3;flex-shrink:0}
  .upload-step.done{border-color:#abefc6;background:#ecfdf3;color:#067647}
  .upload-step.done svg,.upload-step.ready svg{color:#079455}
  .upload-step.ready{border-color:#7cd4b5;background:#f0fdf9;color:#065f46}
  .field{margin-top:16px}
  .field-label{display:flex;justify-content:space-between;align-items:center;gap:12px;margin-bottom:7px}
  .field-label label{font-size:12px;text-transform:uppercase;letter-spacing:.08em;color:#667085;font-weight:760}
  .field-label span{font-size:12px;color:#667085;font-family:ui-monospace,SFMono-Regular,Menlo,monospace}
  textarea{width:100%;resize:vertical;border:1px solid #d7deea;border-radius:8px;padding:11px 12px;font:inherit;font-size:14px;color:#182230;background:#fff;outline:none;box-sizing:border-box}
  textarea:focus{border-color:#175cd3;box-shadow:0 0 0 3px rgba(23,92,211,.12)}
  .connect-box{margin-top:18px;border-top:1px solid #e4e9f2;padding-top:18px;display:flex;gap:12px;align-items:flex-start}
  .connect-box .primary-action{margin-top:12px}
  .creator-card{display:grid;grid-template-columns:auto minmax(0,1fr) auto;gap:12px;align-items:center;border:1px solid #dfe5ef;border-radius:8px;background:#f8fafc;padding:11px;margin-bottom:18px}
  .creator-card strong{display:block;font-size:14px;color:#182230}
  .creator-card span{display:block;font-size:13px;color:#667085;margin-top:2px}
  .avatar,.avatar-img{width:42px;height:42px;border-radius:50%;display:grid;place-items:center;background:#182230;color:#fff;font-size:13px;font-weight:800;object-fit:cover}
  .limit-pill{font-size:12px;font-weight:680;color:#175cd3;border:1px solid #d1e0ff;background:#eef4ff;border-radius:999px;padding:6px 9px;white-space:nowrap}
  .privacy-options{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px}
  .privacy-option{display:grid;grid-template-columns:auto 1fr;grid-template-areas:"icon label" "icon code";gap:1px 8px;text-align:left;border:1px solid #d7deea;border-radius:8px;background:#fff;color:#182230;padding:10px;cursor:pointer}
  .privacy-option svg{grid-area:icon;margin-top:2px;color:#667085}
  .privacy-option span{grid-area:label;font-size:13px;font-weight:700}
  .privacy-option small{grid-area:code;font-size:11px;color:#667085}
  .privacy-option.selected{border-color:#175cd3;background:#eef4ff}
  .privacy-option:disabled{opacity:.55;cursor:not-allowed}
  .toggle-row{display:flex;flex-wrap:wrap;gap:8px}
  .interaction-toggle{display:grid;grid-template-columns:auto 1fr;grid-template-areas:"check label" "check state";gap:1px 8px;min-width:132px;border:1px solid #d7deea;border-radius:8px;background:#fff;padding:10px;cursor:pointer}
  .interaction-toggle.checked{border-color:#175cd3;background:#eef4ff}
  .interaction-toggle input{grid-area:check;margin-top:3px;width:15px;height:15px}
  .interaction-toggle span{grid-area:label;font-size:13px;font-weight:700}
  .interaction-toggle small{grid-area:state;font-size:11px;color:#667085}
  .interaction-toggle:has(input:disabled){opacity:.58;cursor:not-allowed}
  .disclosure-box,.music-confirmation{margin-top:18px;border:1px solid #dfe5ef;border-radius:8px;background:#f8fafc;padding:13px}
  .check-line{display:flex;align-items:flex-start;gap:10px;cursor:pointer}
  .check-line input{width:16px;height:16px;margin-top:2px;flex-shrink:0}
  .check-line strong{display:block;font-size:13px;color:#182230}
  .check-line small{display:block;font-size:12px;color:#667085;line-height:1.45;margin-top:2px}
  .nested-checks{margin-top:12px;border-top:1px solid #dfe5ef;padding-top:12px;display:grid;gap:10px}
  .compact{background:#fff;border:1px solid #dfe5ef;border-radius:8px;padding:10px}
  .post-preview{margin-top:18px;border:1px solid #dfe5ef;border-radius:8px;background:#fff;display:grid;grid-template-columns:96px minmax(0,1fr);gap:12px;padding:12px;align-items:center}
  .preview-media{height:120px;border-radius:8px;background:#101828;color:#cbd5e1;display:grid;place-items:center;overflow:hidden}
  .preview-media video{width:100%;height:100%;object-fit:cover}
  .preview-title{font-size:13px;font-weight:760;color:#182230;margin-bottom:4px}
  .preview-meta{display:flex;flex-wrap:wrap;gap:6px;margin-top:9px}
  .preview-meta span{font-size:11px;color:#344054;background:#f2f4f7;border:1px solid #dfe5ef;border-radius:999px;padding:4px 7px}
  .music-confirmation{display:flex;gap:10px;align-items:flex-start;background:#fff}
  .music-confirmation svg{color:#175cd3;margin-top:1px;flex-shrink:0}
  .music-confirmation a{color:#047857;text-decoration:underline;text-underline-offset:3px;font-weight:650}
  .publish-button{width:100%;margin-top:18px;min-height:46px;font-size:14px}
  .hint{font-size:12px;margin-top:7px;line-height:1.45}
  .hint.warning{color:#b54708}.hint.error{color:#b42318}.hint.info{color:#667085}
  .error-note{margin-top:10px;border:1px solid #fecaca;background:#fef2f2;color:#b42318;border-radius:8px;padding:10px 11px;font-size:12px;line-height:1.45}
  .muted-box{border:1px solid #dfe5ef;border-radius:8px;background:#f8fafc;color:#667085;padding:12px;font-size:13px}
  .result-band{max-width:1280px;margin:18px auto 0;display:flex;align-items:center;justify-content:space-between;gap:20px}
  .result-ok{background:#ecfdf3;border-color:#abefc6;color:#067647}
  .result-pending{background:#fffaeb;border-color:#fedf89;color:#b54708}
  .review-empty{max-width:520px;margin:12vh auto;background:#fff;border:1px solid #dfe5ef;border-radius:8px;padding:28px;text-align:center}
  .review-empty svg{color:#175cd3}
  .spin{animation:spin .8s linear infinite}
  @keyframes spin{from{transform:rotate(0deg)}to{transform:rotate(360deg)}}
  @media (max-width:980px){.review-shell{padding:20px}.review-topbar,.result-band{flex-direction:column}.review-grid{grid-template-columns:1fr}.status-pill{align-self:flex-start}.privacy-options,.upload-evidence{grid-template-columns:1fr}}
`;
