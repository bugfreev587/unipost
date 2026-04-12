"use client";

import { useEffect, useRef } from "react";
import { X, ExternalLink, Copy } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import type { SocialPost } from "@/lib/api";

const CSS = `.pdd-overlay{position:fixed;inset:0;background:var(--overlay);z-index:50;animation:pdd-fade-in .15s ease}
@keyframes pdd-fade-in{from{opacity:0}to{opacity:1}}
.pdd-drawer{position:fixed;top:0;right:0;bottom:0;width:420px;max-width:90vw;background:var(--surface-raised);border-left:1px solid var(--dborder);z-index:51;overflow-y:auto;animation:pdd-slide-in .2s ease;display:flex;flex-direction:column}
@keyframes pdd-slide-in{from{transform:translateX(100%)}to{transform:translateX(0)}}
.pdd-header{display:flex;align-items:center;justify-content:space-between;padding:20px 24px;border-bottom:1px solid var(--dborder);flex-shrink:0}
.pdd-header-title{font-size:16px;font-weight:700;color:var(--dtext)}
.pdd-close{background:none;border:1px solid var(--dborder);border-radius:6px;padding:5px;cursor:pointer;color:var(--dmuted);transition:all .1s;display:flex;align-items:center}
.pdd-close:hover{background:var(--surface2);color:var(--dtext)}
.pdd-body{padding:24px;flex:1}
.pdd-section{margin-bottom:24px}
.pdd-section-title{font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:var(--dmuted2);margin-bottom:10px}
.pdd-caption{font-size:14px;color:var(--dtext);line-height:1.65;white-space:pre-wrap;word-break:break-word;background:var(--surface1);border:1px solid var(--dborder);border-radius:8px;padding:14px 16px}
.pdd-result{display:flex;align-items:flex-start;gap:10px;padding:10px 0;border-bottom:1px solid var(--dborder)}
.pdd-result:last-child{border-bottom:none}
.pdd-result-info{flex:1;min-width:0}
.pdd-result-name{font-size:13px;font-weight:600;color:var(--dtext)}
.pdd-result-link{font-size:11px;color:var(--daccent);text-decoration:none;display:inline-flex;align-items:center;gap:3px;margin-top:2px}
.pdd-result-link:hover{text-decoration:underline}
.pdd-result-error{font-size:12px;color:var(--danger);margin-top:3px;line-height:1.4}
.pdd-meta-row{display:flex;justify-content:space-between;align-items:center;padding:6px 0;font-size:13px}
.pdd-meta-label{color:var(--dmuted)}
.pdd-meta-value{color:var(--dtext);font-weight:500}
.pdd-footer{padding:16px 24px;border-top:1px solid var(--dborder);display:flex;gap:8px;flex-shrink:0}`;

const RESULT_BADGE: Record<string, { color: string; label: string }> = {
  published: { color: "var(--success)", label: "published" },
  failed: { color: "var(--danger)", label: "failed" },
  pending: { color: "var(--info)", label: "pending" },
};

function externalUrl(platform: string | undefined, externalId: string): string | null {
  if (!platform || !externalId) return null;
  switch (platform) {
    case "twitter": return `https://x.com/i/web/status/${externalId}`;
    case "linkedin": return `https://www.linkedin.com/feed/update/${externalId}`;
    case "bluesky": return `https://bsky.app/profile/${externalId}`;
    default: return null;
  }
}

interface Props {
  post: SocialPost;
  onClose: () => void;
  onDuplicate?: (post: SocialPost) => void;
}

export function PostDetailDrawer({ post, onClose, onDuplicate }: Props) {
  const overlayRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleKey(e: KeyboardEvent) { if (e.key === "Escape") onClose(); }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [onClose]);

  function publishMode(): string {
    if (post.status === "draft") return "Draft";
    if (post.scheduled_at) return "Scheduled";
    return "Immediate";
  }

  function formatDate(d: string | undefined | null): string {
    if (!d) return "—";
    return new Date(d).toLocaleString("en-US", { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
  }

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="pdd-overlay" ref={overlayRef} onClick={(e) => { if (e.target === overlayRef.current) onClose(); }} />
      <div className="pdd-drawer">
        <div className="pdd-header">
          <span className="pdd-header-title">Post detail</span>
          <button className="pdd-close" onClick={onClose}><X style={{ width: 16, height: 16 }} /></button>
        </div>

        <div className="pdd-body">
          {/* Caption */}
          <div className="pdd-section">
            <div className="pdd-section-title">Caption</div>
            <div className="pdd-caption">{post.caption || "(no caption)"}</div>
          </div>

          {/* Platform results */}
          {post.results && post.results.length > 0 && (
            <div className="pdd-section">
              <div className="pdd-section-title">Platform results</div>
              {post.results.map((r, i) => {
                const badge = RESULT_BADGE[r.status] || RESULT_BADGE.pending;
                const url = externalUrl(r.platform, r.external_id || "");
                return (
                  <div key={i} className="pdd-result">
                    <PlatformIcon platform={r.platform || ""} size={16} />
                    <div className="pdd-result-info">
                      <div className="pdd-result-name">
                        {r.account_name || r.platform || "Unknown"}
                        <span style={{ marginLeft: 8, fontSize: 11, fontWeight: 400, color: badge.color }}>{badge.label}</span>
                      </div>
                      {url && r.status === "published" && (
                        <a href={url} target="_blank" rel="noopener noreferrer" className="pdd-result-link">
                          View on {r.platform} <ExternalLink style={{ width: 10, height: 10 }} />
                        </a>
                      )}
                      {r.error_message && <div className="pdd-result-error">{r.error_message}</div>}
                    </div>
                  </div>
                );
              })}
            </div>
          )}

          {/* Metadata */}
          <div className="pdd-section">
            <div className="pdd-section-title">Metadata</div>
            <div className="pdd-meta-row">
              <span className="pdd-meta-label">Mode</span>
              <span className="pdd-meta-value">{publishMode()}</span>
            </div>
            <div className="pdd-meta-row">
              <span className="pdd-meta-label">Status</span>
              <span className="pdd-meta-value">{post.status}</span>
            </div>
            <div className="pdd-meta-row">
              <span className="pdd-meta-label">Created</span>
              <span className="pdd-meta-value">{formatDate(post.created_at)}</span>
            </div>
            {post.published_at && (
              <div className="pdd-meta-row">
                <span className="pdd-meta-label">Published</span>
                <span className="pdd-meta-value">{formatDate(post.published_at)}</span>
              </div>
            )}
            {post.scheduled_at && (
              <div className="pdd-meta-row">
                <span className="pdd-meta-label">Scheduled for</span>
                <span className="pdd-meta-value">{formatDate(post.scheduled_at)}</span>
              </div>
            )}
          </div>
        </div>

        {/* Footer actions */}
        <div className="pdd-footer">
          {onDuplicate && (
            <button className="dbtn dbtn-ghost" style={{ gap: 5, fontSize: 12 }} onClick={() => onDuplicate(post)}>
              <Copy style={{ width: 13, height: 13 }} /> Duplicate
            </button>
          )}
        </div>
      </div>
    </>
  );
}
