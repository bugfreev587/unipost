"use client";

import { useState, useEffect } from "react";
import { X, Send, Calendar, FileText, Pencil } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { PLATFORM_LIMITS, countCharacters, getCountStatus, STATUS_COLORS } from "@/components/tools/platform-limits";
import type { SocialAccount, SocialPost } from "@/lib/api";
import { createSocialPost } from "@/lib/api";

const CSS = `.cpm-overlay{position:fixed;inset:0;background:var(--overlay);z-index:50;animation:cpm-fade .15s ease}
@keyframes cpm-fade{from{opacity:0}to{opacity:1}}
.cpm-modal{position:fixed;top:50%;left:50%;transform:translate(-50%,-50%);width:820px;max-width:95vw;max-height:90vh;background:var(--surface-raised);border:1px solid var(--dborder);border-radius:14px;z-index:51;display:flex;flex-direction:column;animation:cpm-scale .2s ease}
@keyframes cpm-scale{from{opacity:0;transform:translate(-50%,-50%) scale(.96)}to{opacity:1;transform:translate(-50%,-50%) scale(1)}}
.cpm-header{display:flex;align-items:center;justify-content:space-between;padding:18px 24px;border-bottom:1px solid var(--dborder);flex-shrink:0}
.cpm-header-title{font-size:16px;font-weight:700;color:var(--dtext)}
.cpm-header-sub{font-size:12px;color:var(--dmuted);margin-top:2px}
.cpm-close{background:none;border:1px solid var(--dborder);border-radius:6px;padding:5px;cursor:pointer;color:var(--dmuted);display:flex;align-items:center;transition:all .1s}
.cpm-close:hover{background:var(--surface2);color:var(--dtext)}
.cpm-body{display:grid;grid-template-columns:1fr 280px;flex:1;overflow-y:auto}
.cpm-left{padding:24px;border-right:1px solid var(--dborder)}
.cpm-right{padding:24px;display:flex;flex-direction:column}
.cpm-label{font-size:11px;font-weight:600;text-transform:uppercase;letter-spacing:.06em;color:var(--dmuted2);margin-bottom:8px}
.cpm-textarea{width:100%;min-height:100px;background:var(--surface1);border:1px solid var(--dborder);border-radius:8px;padding:12px 14px;font-size:14px;line-height:1.6;color:var(--dtext);font-family:inherit;resize:vertical;outline:none;transition:border-color .15s}
.cpm-textarea:focus{border-color:var(--daccent)}
.cpm-textarea::placeholder{color:var(--dmuted2)}
.cpm-char-count{text-align:right;font-size:11px;color:var(--dmuted2);margin-top:4px;font-family:var(--font-geist-mono),monospace}
.cpm-overrides{margin-top:20px}
.cpm-override-item{margin-bottom:8px}
.cpm-override-header{display:flex;align-items:center;justify-content:space-between;padding:8px 10px;background:var(--surface1);border:1px solid var(--dborder);border-radius:6px;cursor:pointer;transition:all .1s}
.cpm-override-header:hover{border-color:var(--dborder2)}
.cpm-override-left{display:flex;align-items:center;gap:6px;font-size:12.5px;color:var(--dtext)}
.cpm-override-tag{font-size:10.5px;color:var(--dmuted2)}
.cpm-override-edit{font-size:11px;color:var(--daccent);display:flex;align-items:center;gap:3px}
.cpm-override-textarea{width:100%;min-height:60px;background:var(--surface2);border:1px solid var(--dborder);border-radius:0 0 6px 6px;border-top:none;padding:10px 12px;font-size:13px;line-height:1.5;color:var(--dtext);font-family:inherit;resize:vertical;outline:none}
.cpm-override-textarea:focus{border-color:var(--daccent)}
.cpm-override-counter{text-align:right;font-size:10px;padding:2px 12px 6px;background:var(--surface2);border:1px solid var(--dborder);border-top:none;border-radius:0 0 6px 6px;font-family:var(--font-geist-mono),monospace}
.cpm-accounts{flex:1;overflow-y:auto;margin-bottom:16px}
.cpm-account{display:flex;align-items:center;gap:8px;padding:8px 10px;border-radius:6px;cursor:pointer;transition:background .1s;font-size:13px;color:var(--dtext)}
.cpm-account:hover{background:var(--surface2)}
.cpm-account input{accent-color:var(--daccent)}
.cpm-account-name{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.cpm-publish{margin-top:auto}
.cpm-publish-tabs{display:flex;gap:2px;margin-bottom:12px;background:var(--surface1);border-radius:6px;padding:2px}
.cpm-publish-tab{flex:1;padding:6px;text-align:center;font-size:11.5px;font-weight:600;border-radius:5px;cursor:pointer;color:var(--dmuted);transition:all .1s;border:none;background:none;font-family:inherit}
.cpm-publish-tab.active{background:var(--surface2);color:var(--dtext)}
.cpm-publish-tab:hover:not(.active){color:var(--dtext)}
.cpm-schedule{margin-bottom:12px}
.cpm-schedule input{width:100%;height:32px;background:var(--surface1);border:1px solid var(--dborder);border-radius:6px;padding:0 10px;font-size:12.5px;color:var(--dtext);font-family:inherit;outline:none}
.cpm-schedule input:focus{border-color:var(--daccent)}
.cpm-footer{padding:16px 24px;border-top:1px solid var(--dborder);display:flex;justify-content:space-between;align-items:center;flex-shrink:0}`;

type PublishMode = "now" | "schedule" | "draft";

interface Props {
  accounts: SocialAccount[];
  workspaceId: string;
  getToken: () => Promise<string | null>;
  onClose: () => void;
  onCreated: () => void;
  editDraft?: SocialPost | null;
}

type CreatePostPayload = {
  caption?: string;
  account_ids?: string[];
  platform_posts?: Array<{ account_id: string; caption: string }>;
  scheduled_at?: string;
  draft?: boolean;
};

export function CreatePostModal({ accounts, workspaceId, getToken, onClose, onCreated, editDraft }: Props) {
  const [caption, setCaption] = useState(editDraft?.caption || "");
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => {
    if (editDraft?.results) return new Set(editDraft.results.map(r => r.social_account_id));
    const saved = typeof window !== "undefined" ? localStorage.getItem("agentpost_last_accounts") : null;
    return saved ? new Set(JSON.parse(saved)) : new Set(accounts.filter(a => a.status === "active").map(a => a.id));
  });
  const [overrides, setOverrides] = useState<Record<string, string>>({});
  const [expandedOverride, setExpandedOverride] = useState<string | null>(null);
  const [mode, setMode] = useState<PublishMode>("now");
  const [scheduledAt, setScheduledAt] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const activeAccounts = accounts.filter(a => a.status === "active");

  function toggleAccount(id: string) {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  // Remember last selected accounts
  useEffect(() => {
    if (selectedIds.size > 0) {
      localStorage.setItem("agentpost_last_accounts", JSON.stringify([...selectedIds]));
    }
  }, [selectedIds]);

  async function handleSubmit() {
    const ids = [...selectedIds];
    if (ids.length === 0 || !caption.trim()) return;
    setSubmitting(true);
    try {
      const token = await getToken();
      if (!token) return;

      // Build platform_posts if any overrides exist
      const hasOverrides = Object.values(overrides).some(v => v.trim());
      const payload: CreatePostPayload = {};

      if (hasOverrides) {
        payload.platform_posts = ids.map(id => ({
          account_id: id,
          caption: overrides[id]?.trim() || caption.trim(),
        }));
      } else {
        payload.caption = caption.trim();
        payload.account_ids = ids;
      }

      if (mode === "schedule" && scheduledAt) {
        payload.scheduled_at = new Date(scheduledAt).toISOString();
      }
      if (mode === "draft") {
        payload.status = "draft";
      }

      await createSocialPost(token, workspaceId, payload);
      onCreated();
      onClose();
    } catch (err) {
      console.error("Create failed:", err);
    } finally {
      setSubmitting(false);
    }
  }

  function getOverrideCount(accountId: string): { count: number; max: number; color: string } | null {
    const account = accounts.find(a => a.id === accountId);
    if (!account) return null;
    const pl = PLATFORM_LIMITS.find(p => p.platform === account.platform);
    if (!pl) return null;
    const text = overrides[accountId]?.trim() || caption;
    const count = countCharacters(text, pl.countingMethod);
    const status = getCountStatus(count, pl.maxLength);
    return { count, max: pl.maxLength, color: STATUS_COLORS[status] };
  }

  const submitLabel = mode === "now" ? "Publish now" : mode === "schedule" ? "Schedule post" : "Save draft";
  const submitIcon = mode === "now" ? <Send style={{ width: 13, height: 13 }} /> : mode === "schedule" ? <Calendar style={{ width: 13, height: 13 }} /> : <FileText style={{ width: 13, height: 13 }} />;
  const canSubmit = caption.trim() && selectedIds.size > 0 && !submitting && (mode !== "schedule" || scheduledAt);

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="cpm-overlay" onClick={onClose} />
      <div className="cpm-modal">
        {/* Header */}
        <div className="cpm-header">
          <div>
            <div className="cpm-header-title">{editDraft ? "Edit draft" : "Create post"}</div>
            <div className="cpm-header-sub">Compose and publish to multiple platforms</div>
          </div>
          <button className="cpm-close" onClick={onClose}><X style={{ width: 16, height: 16 }} /></button>
        </div>

        {/* Body: two columns */}
        <div className="cpm-body">
          {/* Left: content */}
          <div className="cpm-left">
            <div className="cpm-label">Content</div>
            <textarea
              className="cpm-textarea"
              placeholder="What would you like to share?"
              value={caption}
              onChange={(e) => setCaption(e.target.value)}
              autoFocus
            />
            <div className="cpm-char-count">{caption.length} chars</div>

            {/* Per-platform overrides */}
            {selectedIds.size > 0 && (
              <div className="cpm-overrides">
                <div className="cpm-label">Per-platform overrides</div>
                {[...selectedIds].map(id => {
                  const acc = accounts.find(a => a.id === id);
                  if (!acc) return null;
                  const expanded = expandedOverride === id;
                  const hasOverride = (overrides[id] || "").trim();
                  const cc = getOverrideCount(id);
                  return (
                    <div key={id} className="cpm-override-item">
                      <div
                        className="cpm-override-header"
                        onClick={() => setExpandedOverride(expanded ? null : id)}
                        style={expanded ? { borderRadius: "6px 6px 0 0" } : undefined}
                      >
                        <span className="cpm-override-left">
                          <PlatformIcon platform={acc.platform} size={13} />
                          {acc.account_name || acc.platform}
                          {!hasOverride && <span className="cpm-override-tag">Same caption</span>}
                        </span>
                        <span className="cpm-override-edit">
                          {cc && <span style={{ color: cc.color, marginRight: 4 }}>{cc.count}/{cc.max}</span>}
                          <Pencil style={{ width: 11, height: 11 }} />
                        </span>
                      </div>
                      {expanded && (
                        <>
                          <textarea
                            className="cpm-override-textarea"
                            placeholder={`Custom caption for ${acc.platform}... (leave empty to use main caption)`}
                            value={overrides[id] || ""}
                            onChange={(e) => setOverrides(o => ({ ...o, [id]: e.target.value }))}
                          />
                          {cc && (
                            <div className="cpm-override-counter" style={{ color: cc.color }}>
                              {cc.count} / {cc.max}
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Right: destinations + publish */}
          <div className="cpm-right">
            <div className="cpm-label">Post to</div>
            <div className="cpm-accounts">
              {activeAccounts.length === 0 ? (
                <div style={{ fontSize: 12, color: "var(--dmuted2)", padding: 10 }}>
                  No active accounts. Connect one first.
                </div>
              ) : (
                activeAccounts.map(a => (
                  <label key={a.id} className="cpm-account">
                    <input type="checkbox" checked={selectedIds.has(a.id)} onChange={() => toggleAccount(a.id)} />
                    <PlatformIcon platform={a.platform} size={14} />
                    <span className="cpm-account-name">{a.account_name || a.platform}</span>
                  </label>
                ))
              )}
            </div>

            <div className="cpm-publish">
              <div className="cpm-label">Publish</div>
              <div className="cpm-publish-tabs">
                {(["now", "schedule", "draft"] as PublishMode[]).map(m => (
                  <button key={m} className={`cpm-publish-tab${mode === m ? " active" : ""}`} onClick={() => setMode(m)}>
                    {m === "now" ? "Now" : m === "schedule" ? "Schedule" : "Draft"}
                  </button>
                ))}
              </div>

              {mode === "schedule" && (
                <div className="cpm-schedule">
                  <input
                    type="datetime-local"
                    value={scheduledAt}
                    onChange={(e) => setScheduledAt(e.target.value)}
                    min={new Date().toISOString().slice(0, 16)}
                  />
                </div>
              )}

              {mode === "now" && (
                <div style={{ fontSize: 12, color: "var(--dmuted)", marginBottom: 12 }}>
                  Posts immediately to all selected accounts.
                </div>
              )}
              {mode === "draft" && (
                <div style={{ fontSize: 12, color: "var(--dmuted)", marginBottom: 12 }}>
                  Saves without publishing. Publish later from the posts list.
                </div>
              )}
            </div>
          </div>
        </div>

        {/* Footer */}
        <div className="cpm-footer">
          <button className="dbtn dbtn-ghost" onClick={onClose}>Cancel</button>
          <div style={{ display: "flex", gap: 8 }}>
            {mode !== "draft" && (
              <button className="dbtn dbtn-ghost" style={{ fontSize: 12 }} onClick={() => { setMode("draft"); handleSubmit(); }}>
                <FileText style={{ width: 12, height: 12 }} /> Save draft
              </button>
            )}
            <button className="dbtn dbtn-primary" disabled={!canSubmit} onClick={handleSubmit} style={{ gap: 5 }}>
              {submitIcon} {submitting ? "Sending..." : submitLabel}
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
