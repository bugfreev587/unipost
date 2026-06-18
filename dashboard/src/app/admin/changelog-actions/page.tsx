"use client";

import { Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { useSearchParams } from "next/navigation";

import { AdminShell, PanelRow, fmtDate } from "../_components/admin-ui";
import {
  confirmAdminChangelogCandidateAction,
  getAdminChangelogCandidate,
  type AdminChangelogAction,
  type AdminChangelogActionResult,
  type AdminChangelogCandidatePreview,
  type ApiFetchError,
} from "@/lib/api";

const ACTION_LABELS: Record<AdminChangelogAction, string> = {
  publish: "Publish",
  save: "Save for later",
  discard: "Discard",
};

const ACTION_DESCRIPTIONS: Record<AdminChangelogAction, string> = {
  publish: "Start the guarded release workflow for dev, staging, and production.",
  save: "Keep this candidate for later review without publishing it.",
  discard: "Suppress this candidate for the same source hash.",
};

function isAction(value: string | null): value is AdminChangelogAction {
  return value === "publish" || value === "save" || value === "discard";
}

export default function AdminChangelogActionsPage() {
  return (
    <Suspense fallback={<AdminShell title="Changelog Action" loading requireSuperAdmin><div /></AdminShell>}>
      <AdminChangelogActionsContent />
    </Suspense>
  );
}

function AdminChangelogActionsContent() {
  const { getToken } = useAuth();
  const searchParams = useSearchParams();
  const candidateId = searchParams.get("candidate_id") || "";
  const actionParam = searchParams.get("action");
  const expires = searchParams.get("expires") || "";
  const signature = searchParams.get("signature") || "";
  const action = isAction(actionParam) ? actionParam : null;

  const [preview, setPreview] = useState<AdminChangelogCandidatePreview | null>(null);
  const [result, setResult] = useState<AdminChangelogActionResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [alreadyHandled, setAlreadyHandled] = useState(false);

  const release = preview?.candidate.payload.candidate;

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    setAlreadyHandled(false);
    try {
      if (!candidateId || !action || !expires || !signature) {
        throw new Error("This changelog action link is incomplete.");
      }
      const token = await getToken();
      if (!token) throw new Error("Not authenticated.");
      const response = await getAdminChangelogCandidate(token, candidateId, { action, expires, signature });
      setPreview(response.data);
    } catch (err) {
      const fetchError = err as ApiFetchError;
      if (fetchError.code === "ALREADY_HANDLED") {
        setAlreadyHandled(true);
        setError("Already handled");
      } else {
        setError(err instanceof Error ? err.message : "Failed to load changelog candidate.");
      }
    } finally {
      setLoading(false);
    }
  }, [action, candidateId, expires, getToken, signature]);

  useEffect(() => {
    load();
  }, [load]);

  const actionLabel = useMemo(() => (action ? ACTION_LABELS[action] : "Review"), [action]);

  const confirm = useCallback(async () => {
    if (!action || !candidateId || !expires || !signature) return;
    setSubmitting(true);
    setError(null);
    setAlreadyHandled(false);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated.");
      const response = await confirmAdminChangelogCandidateAction(token, candidateId, { action, expires, signature });
      setResult(response.data);
    } catch (err) {
      const fetchError = err as ApiFetchError;
      if (fetchError.code === "ALREADY_HANDLED") {
        setAlreadyHandled(true);
        setError("Already handled");
      } else {
        setError(err instanceof Error ? err.message : "Failed to confirm changelog action.");
      }
    } finally {
      setSubmitting(false);
    }
  }, [action, candidateId, expires, getToken, signature]);

  return (
    <AdminShell title="Changelog Action" loading={loading || submitting} onRefresh={load} requireSuperAdmin>
      <div style={{ display: "grid", gap: 14, maxWidth: 820 }}>
        <div className="ad-section-header">
          <div>
            <div className="ad-section-title">{actionLabel} changelog candidate</div>
            <div className="ad-section-meta">
              {action ? ACTION_DESCRIPTIONS[action] : "Review the signed changelog action."}
            </div>
          </div>
        </div>

        {error && (
          <div style={{ background: alreadyHandled ? "var(--info-soft)" : "var(--danger-soft)", border: `1px solid ${alreadyHandled ? "color-mix(in srgb, var(--info) 22%, transparent)" : "color-mix(in srgb, var(--danger) 22%, transparent)"}`, borderRadius: 8, padding: 12, color: alreadyHandled ? "var(--info)" : "var(--danger)", fontSize: 13 }}>
            {error}
          </div>
        )}

        {result && (
          <div style={{ background: "var(--success-soft)", border: "1px solid color-mix(in srgb, var(--success) 22%, transparent)", borderRadius: 8, padding: 12, color: "var(--success)", fontSize: 13 }}>
            {result.message}
            {result.workflow_run_url ? (
              <>
                {" "}
                <a className="ad-link" href={result.workflow_run_url} target="_blank" rel="noreferrer">Open workflow</a>
              </>
            ) : null}
          </div>
        )}

        <div className="ad-tbl-wrap ad-tbl-static" style={{ padding: 16 }}>
          {loading ? (
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>Loading candidate...</div>
          ) : release ? (
            <div style={{ display: "grid", gap: 14 }}>
              <div>
                <div style={{ fontSize: 18, fontWeight: 650, letterSpacing: "-0.01em" }}>{release.title}</div>
                <div style={{ marginTop: 6, color: "var(--dmuted)", maxWidth: 680 }}>{release.summary}</div>
              </div>

              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(170px, 1fr))", gap: 8 }}>
                <PanelRow k="Date" v={release.displayDate || fmtDate(release.date)} />
                <PanelRow k="Area" v={<span className="ad-badge ad-b-gray">{release.category}</span>} />
                <PanelRow k="Impact" v={<span className="ad-badge ad-b-blue">{release.impact}</span>} />
                <PanelRow k="Status" v={<span className="ad-badge ad-b-gray">{preview?.candidate.status}</span>} />
              </div>

              <div>
                <div className="ad-panel-section-title">Why user visible</div>
                <div style={{ color: "var(--dtext)", fontSize: 13 }}>{release.whyUserVisible}</div>
              </div>

              {release.sdkVersions?.length ? (
                <div>
                  <div className="ad-panel-section-title">SDK versions</div>
                  <div className="ad-stack">
                    {release.sdkVersions.map((sdk) => (
                      <div key={`${sdk.ecosystem}-${sdk.packageName}-${sdk.version}`} className="ad-failure-card">
                        <div className="ad-failure-head">
                          <div>
                            <div className="ad-failure-title">{sdk.packageName}</div>
                            <div className="ad-mono">{sdk.ecosystem} {sdk.version}</div>
                          </div>
                          <a className="ad-link" href={sdk.href} target="_blank" rel="noreferrer">Registry</a>
                        </div>
                        {sdk.installCommand ? <div className="ad-failure-debug">{sdk.installCommand}</div> : null}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              <div>
                <div className="ad-panel-section-title">Source links</div>
                <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                  {release.sourceLinks.map((link) => (
                    <a key={`${link.label}-${link.href}`} className="ad-btn ad-btn-ghost" href={link.href} target="_blank" rel="noreferrer">
                      {link.label}
                    </a>
                  ))}
                </div>
              </div>

              <div style={{ display: "flex", gap: 8, flexWrap: "wrap", paddingTop: 4 }}>
                <button className="ad-btn" style={{ background: action === "discard" ? "var(--danger-soft)" : "var(--daccent)", color: action === "discard" ? "var(--danger)" : "var(--primary-foreground)", borderColor: action === "discard" ? "color-mix(in srgb, var(--danger) 22%, transparent)" : "transparent", padding: "8px 14px" }} type="button" onClick={confirm} disabled={submitting || !!result || alreadyHandled}>
                  {submitting ? "Confirming..." : actionLabel}
                </button>
                <button className="ad-btn ad-btn-ghost" style={{ padding: "8px 14px" }} type="button" onClick={load} disabled={submitting}>
                  Refresh
                </button>
              </div>
            </div>
          ) : (
            <div style={{ color: "var(--dmuted)", fontSize: 13 }}>No candidate loaded.</div>
          )}
        </div>
      </div>
    </AdminShell>
  );
}
