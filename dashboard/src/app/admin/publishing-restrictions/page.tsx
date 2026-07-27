"use client";

import { useAuth } from "@clerk/nextjs";
import {
  Clock3,
  LoaderCircle,
  Mail,
  RotateCcw,
  ShieldAlert,
  ToggleLeft,
  ToggleRight,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  createAdminPublishingRestrictionCampaign,
  listAdminPublishingRestrictionCampaigns,
  listAdminPublishingRestrictions,
  previewAdminPublishingRestrictionCampaign,
  retryFailedAdminPublishingRestrictionCampaign,
  updateAdminPublishingRestriction,
  type AdminPublishingRestriction,
  type AdminPublishingRestrictionCampaign,
  type AdminPublishingRestrictionCampaignPreview,
  type ApiFetchError,
  type PublishingRestrictionCampaignType,
} from "@/lib/api";

import { AdminShell } from "../_components/admin-ui";

type PendingChange = {
  restriction: AdminPublishingRestriction;
  enabled: boolean;
};

type PreviewState = {
  platform: string;
  preview: AdminPublishingRestrictionCampaignPreview;
};

const CAMPAIGN_LABELS: Record<PublishingRestrictionCampaignType, string> = {
  restriction_notice: "Restriction notice",
  recovery_notice: "Recovery notice",
};

const ACTIVE_CAMPAIGN_STATUSES = new Set(["queued", "running"]);

function formatDate(value?: string): string {
  if (!value) return "Not set";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

function platformLabel(platform: string): string {
  return platform === "tiktok"
    ? "TikTok"
    : platform.charAt(0).toUpperCase() + platform.slice(1);
}

function campaignKey(platform: string, type: PublishingRestrictionCampaignType): string {
  return `${platform}:${type}`;
}

export default function AdminPublishingRestrictionsPage() {
  const { getToken } = useAuth();
  const [restrictions, setRestrictions] = useState<AdminPublishingRestriction[]>([]);
  const [campaignsByPlatform, setCampaignsByPlatform] = useState<Record<string, AdminPublishingRestrictionCampaign[]>>({});
  const [campaignErrors, setCampaignErrors] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [conflict, setConflict] = useState<string | null>(null);
  const [pendingChange, setPendingChange] = useState<PendingChange | null>(null);
  const [previewState, setPreviewState] = useState<PreviewState | null>(null);
  const [sendPreview, setSendPreview] = useState<PreviewState | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [dialogError, setDialogError] = useState<string | null>(null);

  const refreshCampaigns = useCallback(async (
    currentRestrictions: AdminPublishingRestriction[],
    tokenOverride?: string,
  ) => {
    const token = tokenOverride || await getToken();
    if (!token) throw new Error("Not authenticated");
    const settled = await Promise.allSettled(
      currentRestrictions.map(async (restriction) => ({
        platform: restriction.platform,
        response: await listAdminPublishingRestrictionCampaigns(token, restriction.platform),
      })),
    );
    const nextCampaigns: Record<string, AdminPublishingRestrictionCampaign[]> = {};
    const nextErrors: Record<string, string> = {};
    settled.forEach((result, index) => {
      const platform = currentRestrictions[index]?.platform;
      if (!platform) return;
      if (result.status === "fulfilled") {
        nextCampaigns[platform] = result.value.response.data || [];
      } else {
        nextCampaigns[platform] = [];
        nextErrors[platform] = result.reason instanceof Error
          ? result.reason.message
          : "Campaign status is unavailable";
      }
    });
    setCampaignsByPlatform(nextCampaigns);
    setCampaignErrors(nextErrors);
  }, [getToken]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await listAdminPublishingRestrictions(token);
      const loaded = response.data || [];
      setRestrictions(loaded);
      await refreshCampaigns(loaded, token);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load publishing restrictions");
    } finally {
      setLoading(false);
    }
  }, [getToken, refreshCampaigns]);

  useEffect(() => {
    void load();
  }, [load]);

  const hasActiveCampaign = useMemo(
    () => Object.values(campaignsByPlatform).some((campaigns) =>
      campaigns.some((campaign) => ACTIVE_CAMPAIGN_STATUSES.has(campaign.status))),
    [campaignsByPlatform],
  );

  useEffect(() => {
    if (!hasActiveCampaign || restrictions.length === 0) return;
    const timer = window.setTimeout(() => {
      void refreshCampaigns(restrictions).catch(() => undefined);
    }, 3000);
    return () => window.clearTimeout(timer);
  }, [hasActiveCampaign, restrictions, refreshCampaigns, campaignsByPlatform]);

  function requestChange(restriction: AdminPublishingRestriction, enabled: boolean) {
    setDialogError(null);
    setPendingChange({ restriction, enabled });
  }

  async function confirmPendingChange() {
    if (!pendingChange || busyKey) return;
    const key = `restriction:${pendingChange.restriction.platform}`;
    setBusyKey(key);
    setDialogError(null);
    setConflict(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await updateAdminPublishingRestriction(
        token,
        pendingChange.restriction.platform,
        {
          enabled: pendingChange.enabled,
          expected_version: pendingChange.restriction.version,
        },
      );
      setRestrictions((current) => current.map((restriction) =>
        restriction.platform === response.data.restriction.platform
          ? response.data.restriction
          : restriction));
      setPendingChange(null);
    } catch (err) {
      if ((err as ApiFetchError)?.status === 409) {
        await load();
        setConflict("This restriction changed in another Admin session. Current state was reloaded; review it before confirming again.");
        setPendingChange(null);
      } else {
        setDialogError(err instanceof Error ? err.message : "Failed to update publishing restriction");
      }
    } finally {
      setBusyKey(null);
    }
  }

  async function requestCampaignPreview(
    restriction: AdminPublishingRestriction,
    campaignType: PublishingRestrictionCampaignType,
  ) {
    const key = campaignKey(restriction.platform, campaignType);
    setBusyKey(key);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await previewAdminPublishingRestrictionCampaign(token, restriction.platform, {
        campaign_type: campaignType,
      });
      setPreviewState({ platform: restriction.platform, preview: response.data });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to preview campaign");
    } finally {
      setBusyKey(null);
    }
  }

  async function confirmCampaignSend() {
    if (!sendPreview || busyKey) return;
    const { platform, preview } = sendPreview;
    const key = campaignKey(platform, preview.campaign_type);
    setBusyKey(key);
    setDialogError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await createAdminPublishingRestrictionCampaign(token, platform, {
        campaign_type: preview.campaign_type,
        preview_token: preview.preview_token,
        confirmation: "SEND",
      });
      setCampaignsByPlatform((current) => ({
        ...current,
        [platform]: [
          response.data,
          ...(current[platform] || []).filter((campaign) => campaign.id !== response.data.id),
        ],
      }));
      setPreviewState(null);
      setSendPreview(null);
    } catch (err) {
      setDialogError(err instanceof Error ? err.message : "Failed to start campaign");
    } finally {
      setBusyKey(null);
    }
  }

  async function retryFailedRecipients(
    platform: string,
    campaign: AdminPublishingRestrictionCampaign,
  ) {
    const key = `retry:${campaign.id}`;
    setBusyKey(key);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const response = await retryFailedAdminPublishingRestrictionCampaign(token, platform, campaign.id);
      setCampaignsByPlatform((current) => ({
        ...current,
        [platform]: (current[platform] || []).map((item) =>
          item.id === response.data.id ? response.data : item),
      }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to retry recipients");
    } finally {
      setBusyKey(null);
    }
  }

  return (
    <AdminShell title="Publishing Restrictions" loading={loading} onRefresh={load} requireSuperAdmin>
      <style>{pageCss}</style>

      <div className="ad-section-header apr-header">
        <div>
          <div className="ad-section-title">Operational publishing policy</div>
          <div className="ad-section-meta">
            Platform-and-plan restrictions enforced by the API and delivery workers.
          </div>
        </div>
        <span className="ad-badge ad-b-gray">Super Admin only</span>
      </div>

      <div className="apr-separation-note">
        <Mail aria-hidden="true" />
        <div>
          <strong>Email campaigns are separate manual actions.</strong>
          <span>Enabling or disabling never sends email, and campaign completion never retries posts.</span>
        </div>
      </div>

      {error ? <div className="apr-alert" role="alert">{error}</div> : null}
      {conflict ? <div className="apr-alert is-conflict" role="alert">{conflict}</div> : null}

      <div className="apr-list" aria-busy={loading}>
        {loading && restrictions.length === 0 ? (
          <div className="apr-loading" aria-label="Loading publishing restrictions">
            <span />
            <span />
          </div>
        ) : restrictions.length === 0 ? (
          <div className="apr-empty">
            <ShieldAlert aria-hidden="true" />
            <strong>No publishing restrictions configured</strong>
            <span>New platform policies will appear here after they are configured by the API.</span>
          </div>
        ) : restrictions.map((restriction) => {
          const saving = busyKey === `restriction:${restriction.platform}`;
          const campaigns = campaignsByPlatform[restriction.platform] || [];
          return (
            <article className="apr-restriction" key={restriction.id || restriction.platform}>
              <header className="apr-restriction-head">
                <div>
                  <div className="apr-title-row">
                    <h2>{platformLabel(restriction.platform)}</h2>
                    <span className={`ad-badge ${restriction.enabled ? "ad-b-red" : "ad-b-gray"}`}>
                      {restriction.enabled ? "ENABLED" : "DISABLED"}
                    </span>
                  </div>
                  <code>{restriction.reason_code}</code>
                </div>
                <button
                  type="button"
                  className={`apr-toggle ${restriction.enabled ? "is-on" : ""}`}
                  aria-pressed={restriction.enabled}
                  aria-label={`${restriction.enabled ? "Disable" : "Enable"} ${platformLabel(restriction.platform)} restriction`}
                  disabled={saving}
                  onClick={() => requestChange(restriction, !restriction.enabled)}
                >
                  {saving
                    ? <LoaderCircle className="apr-spin" aria-hidden="true" />
                    : restriction.enabled
                      ? <ToggleRight aria-hidden="true" />
                      : <ToggleLeft aria-hidden="true" />}
                  {restriction.enabled ? "Disable" : "Enable"}
                </button>
              </header>

              <p className="apr-message">{restriction.user_message}</p>

              <dl className="apr-metrics">
                <div><dt>Restricted plans</dt><dd>{restriction.restricted_plan_ids.join(", ") || "None"}</dd></div>
                <div><dt>Affected workspaces</dt><dd>{(restriction.affected_workspace_count ?? 0).toLocaleString()}</dd></div>
                <div><dt>Active accounts</dt><dd>{(restriction.affected_account_count ?? 0).toLocaleString()}</dd></div>
                <div><dt>Current-cycle retained objects</dt><dd>{(restriction.current_cycle_retained_object_count ?? 0).toLocaleString()}</dd></div>
                <div><dt>Current-cycle retained bytes</dt><dd>{(restriction.current_cycle_retained_bytes ?? 0).toLocaleString()}</dd></div>
                <div><dt>Current-cycle projected 60-day storage</dt><dd>${(restriction.current_cycle_projected_60_day_storage_cost_usd ?? 0).toFixed(4)}</dd></div>
                <div><dt>Version</dt><dd>{restriction.version}</dd></div>
                <div><dt>Cycle</dt><dd>{restriction.cycle_id || "No activation yet"}</dd></div>
                <div><dt>Latest actor</dt><dd>{restriction.updated_by_user_id || "System"}</dd></div>
                <div><dt>Enabled</dt><dd>{formatDate(restriction.enabled_at)}</dd></div>
                <div><dt>Disabled</dt><dd>{formatDate(restriction.disabled_at)}</dd></div>
                <div><dt>Created</dt><dd>{formatDate(restriction.created_at)}</dd></div>
                <div><dt>Updated</dt><dd>{formatDate(restriction.updated_at)}</dd></div>
              </dl>

              <section className="apr-subsection">
                <div className="apr-subsection-head">
                  <h3>Recent audit events</h3>
                  <span>{restriction.recent_events?.length || 0} shown</span>
                </div>
                {restriction.recent_events?.length ? (
                  <ol className="apr-events">
                    {restriction.recent_events.map((event) => (
                      <li key={event.id}>
                        <span className={`apr-event-dot ${event.event_type === "enabled" ? "is-enabled" : ""}`} />
                        <div>
                          <strong>{event.event_type}</strong>
                          <span>{event.actor_user_id || "System"} · v{event.resulting_version} · {formatDate(event.created_at)}</span>
                        </div>
                      </li>
                    ))}
                  </ol>
                ) : <div className="apr-inline-empty">No audit events returned.</div>}
              </section>

              <section className="apr-subsection">
                <div className="apr-subsection-head">
                  <h3>Customer email campaigns</h3>
                  <span>Manual, fixed-copy service notices</span>
                </div>
                {campaignErrors[restriction.platform] ? (
                  <div className="apr-inline-error" role="alert">
                    Campaign status unavailable: {campaignErrors[restriction.platform]}
                  </div>
                ) : null}
                <div className="apr-campaign-grid">
                  {(["restriction_notice", "recovery_notice"] as const).map((campaignType) => (
                    <CampaignPanel
                      key={campaignType}
                      restriction={restriction}
                      campaignType={campaignType}
                      campaigns={campaigns}
                      preview={
                        previewState?.platform === restriction.platform &&
                        previewState.preview.campaign_type === campaignType
                          ? previewState.preview
                          : null
                      }
                      busyKey={busyKey}
                      onPreview={() => void requestCampaignPreview(restriction, campaignType)}
                      onConfirm={(preview) => setSendPreview({ platform: restriction.platform, preview })}
                      onRetry={(campaign) => void retryFailedRecipients(restriction.platform, campaign)}
                    />
                  ))}
                </div>
              </section>
            </article>
          );
        })}
      </div>

      <Dialog
        open={pendingChange !== null}
        onOpenChange={(open) => {
          if (!open && !busyKey) {
            setPendingChange(null);
            setDialogError(null);
          }
        }}
      >
        <DialogContent className="apr-dialog" showCloseButton={!busyKey}>
          {pendingChange ? (
            <>
              <DialogHeader>
                <DialogTitle>
                  {pendingChange.enabled ? "Enable" : "Disable"} {platformLabel(pendingChange.restriction.platform)} restriction?
                </DialogTitle>
                <DialogDescription>
                  This operational change is immediate for new admission and dispatch checks.
                </DialogDescription>
              </DialogHeader>
              <ul className="apr-confirm-list">
                {pendingChange.enabled ? (
                  <>
                    <li>Free TikTok admission and dispatch will be blocked.</li>
                    <li>Paid plans and other platforms remain available.</li>
                    <li>Already in-flight calls are not canceled.</li>
                    <li>No customer email is sent.</li>
                    <li>No post is automatically retried.</li>
                  </>
                ) : (
                  <>
                    <li>New eligible customer actions may publish again.</li>
                    <li>Previous failures will not publish automatically.</li>
                    <li>Customers must use result-level Retry.</li>
                    <li>No recovery email is sent unless separately previewed and confirmed.</li>
                  </>
                )}
              </ul>
              {dialogError ? <div className="apr-inline-error" role="alert">{dialogError}</div> : null}
              <DialogFooter>
                <button className="dbtn dbtn-ghost" type="button" disabled={Boolean(busyKey)} onClick={() => setPendingChange(null)}>
                  Cancel
                </button>
                <button className="dbtn dbtn-primary" type="button" disabled={Boolean(busyKey)} onClick={() => void confirmPendingChange()}>
                  {busyKey ? <LoaderCircle className="apr-spin" aria-hidden="true" /> : null}
                  Confirm {pendingChange.enabled ? "enable" : "disable"}
                </button>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>

      <Dialog
        open={sendPreview !== null}
        onOpenChange={(open) => {
          if (!open && !busyKey) {
            setSendPreview(null);
            setDialogError(null);
          }
        }}
      >
        <DialogContent className="apr-dialog" showCloseButton={!busyKey}>
          {sendPreview ? (
            <>
              <DialogHeader>
                <div className="apr-irreversible">Irreversible send</div>
                <DialogTitle>Send {CAMPAIGN_LABELS[sendPreview.preview.campaign_type]}?</DialogTitle>
                <DialogDescription>
                  This snapshots the current audience and queues delivery. Sent recipients cannot be recalled.
                </DialogDescription>
              </DialogHeader>
              <div className="apr-send-summary">
                <strong>{sendPreview.preview.recipient_count.toLocaleString()} previewed recipients</strong>
                <span>{sendPreview.preview.subject}</span>
              </div>
              {dialogError ? <div className="apr-inline-error" role="alert">{dialogError}</div> : null}
              <DialogFooter>
                <button className="dbtn dbtn-ghost" type="button" disabled={Boolean(busyKey)} onClick={() => setSendPreview(null)}>
                  Cancel
                </button>
                <button className="dbtn dbtn-primary" type="button" disabled={Boolean(busyKey)} onClick={() => void confirmCampaignSend()}>
                  {busyKey ? <LoaderCircle className="apr-spin" aria-hidden="true" /> : null}
                  Send — irreversible
                </button>
              </DialogFooter>
            </>
          ) : null}
        </DialogContent>
      </Dialog>
    </AdminShell>
  );
}

function CampaignPanel({
  restriction,
  campaignType,
  campaigns,
  preview,
  busyKey,
  onPreview,
  onConfirm,
  onRetry,
}: {
  restriction: AdminPublishingRestriction;
  campaignType: PublishingRestrictionCampaignType;
  campaigns: AdminPublishingRestrictionCampaign[];
  preview: AdminPublishingRestrictionCampaignPreview | null;
  busyKey: string | null;
  onPreview: () => void;
  onConfirm: (preview: AdminPublishingRestrictionCampaignPreview) => void;
  onRetry: (campaign: AdminPublishingRestrictionCampaign) => void;
}) {
  const cycleCampaigns = campaigns.filter((item) => item.cycle_id === restriction.cycle_id);
  const campaign = cycleCampaigns.find((item) => item.campaign_type === campaignType);
  const restrictionSent = cycleCampaigns.some((item) =>
    item.campaign_type === "restriction_notice" && item.sent_count > 0);
  const eligible = campaignType === "restriction_notice"
    ? restriction.enabled && !campaign
    : !restriction.enabled && restrictionSent && !campaign;
  const busy = busyKey === campaignKey(restriction.platform, campaignType) || busyKey === `retry:${campaign?.id}`;

  return (
    <div className="apr-campaign-panel">
      <div className="apr-campaign-head">
        <div>
          <h4>{CAMPAIGN_LABELS[campaignType]}</h4>
          <span>{campaignType === "restriction_notice" ? "Current eligible Free owners" : "Successfully notified owners still eligible"}</span>
        </div>
        {campaign ? <span className="ad-badge ad-b-gray">{campaign.status}</span> : null}
      </div>

      {campaign ? (
        <>
          <div className="apr-progress" aria-label={`${CAMPAIGN_LABELS[campaignType]} progress`}>
            <div><span>Previewed</span><strong>{campaign.previewed_count}</strong></div>
            <div><span>Snapshotted</span><strong>{campaign.snapshotted_count}</strong></div>
            <div><span>Pending</span><strong>{campaign.pending_count}</strong></div>
            <div><span>Sent</span><strong>{campaign.sent_count}</strong></div>
            <div><span>Failed</span><strong>{campaign.failed_count}</strong></div>
            <div><span>Skipped</span><strong>{campaign.skipped_count}</strong></div>
          </div>
          {campaign.previewed_count !== campaign.snapshotted_count ? (
            <div className="apr-count-drift">
              Preview showed {campaign.previewed_count}; final snapshot contains {campaign.snapshotted_count}.
            </div>
          ) : null}
          {ACTIVE_CAMPAIGN_STATUSES.has(campaign.status) ? (
            <div className="apr-live"><Clock3 aria-hidden="true" /> Progress refreshes while delivery is active.</div>
          ) : null}
          {campaign.failed_count > 0 ? (
            <button type="button" className="apr-secondary" disabled={busy} onClick={() => onRetry(campaign)}>
              {busy ? <LoaderCircle className="apr-spin" aria-hidden="true" /> : <RotateCcw aria-hidden="true" />}
              Retry failed recipients
            </button>
          ) : null}
        </>
      ) : preview ? (
        <div className="apr-preview">
          <div className="apr-preview-count">{preview.recipient_count.toLocaleString()} recipients</div>
          <strong>{preview.subject}</strong>
          <pre>{preview.body}</pre>
          <button type="button" className="apr-primary" onClick={() => onConfirm(preview)}>
            Continue to irreversible confirmation
          </button>
        </div>
      ) : (
        <>
          <p className="apr-campaign-help">
            {eligible
              ? "Preview exact copy and the current deduplicated recipient count before sending."
              : campaignType === "restriction_notice"
                ? "Available only while the current cycle is enabled and before a notice exists."
                : "Available only after disable, after at least one restriction notice was sent."}
          </p>
          <button type="button" className="apr-secondary" disabled={!eligible || busy} onClick={onPreview}>
            {busy ? <LoaderCircle className="apr-spin" aria-hidden="true" /> : <Mail aria-hidden="true" />}
            Preview {CAMPAIGN_LABELS[campaignType].toLowerCase()}
          </button>
        </>
      )}
    </div>
  );
}

const pageCss = `
.apr-header{align-items:flex-start}.apr-separation-note{display:flex;gap:12px;align-items:flex-start;margin-bottom:18px;padding:14px 16px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface2)}
.apr-separation-note>svg{width:18px;height:18px;margin-top:1px;color:var(--daccent);flex:0 0 auto}.apr-separation-note>div{display:grid;gap:3px}.apr-separation-note strong{font-size:13px}.apr-separation-note span{color:var(--dmuted);font-size:12px;line-height:1.55}
.apr-alert,.apr-inline-error{border:1px solid color-mix(in srgb,var(--danger) 30%,var(--dborder));border-radius:9px;background:var(--danger-soft);color:var(--danger);font-size:12px;line-height:1.55;padding:10px 12px}.apr-alert{margin-bottom:14px}.apr-alert.is-conflict{border-color:color-mix(in srgb,var(--warning) 35%,var(--dborder));background:color-mix(in srgb,var(--warning) 9%,var(--surface2));color:var(--warning)}
.apr-list{display:grid;gap:16px}.apr-loading{display:grid;gap:12px}.apr-loading span{display:block;height:112px;border:1px solid var(--dborder);border-radius:12px;background:linear-gradient(90deg,var(--surface2),var(--surface3),var(--surface2));background-size:200% 100%;animation:apr-shimmer 1.4s ease-in-out infinite}.apr-empty{display:grid;justify-items:start;gap:8px;padding:34px;border:1px dashed var(--dborder2);border-radius:12px;color:var(--dmuted)}.apr-empty>svg{width:24px;height:24px;color:var(--dmuted2)}.apr-empty strong{color:var(--dtext);font-size:14px}.apr-empty span{font-size:12px}
.apr-restriction{border:1px solid var(--dborder);border-radius:12px;background:var(--surface1);overflow:hidden}.apr-restriction-head{display:flex;align-items:flex-start;justify-content:space-between;gap:18px;padding:20px}.apr-title-row{display:flex;align-items:center;gap:10px}.apr-title-row h2{margin:0;color:var(--dtext);font-size:20px;letter-spacing:-.025em}.apr-title-row+code,.apr-restriction-head code{display:block;margin-top:6px;color:var(--dmuted2);font-family:var(--font-geist-mono),monospace;font-size:11px}.apr-toggle,.apr-secondary,.apr-primary{display:inline-flex;align-items:center;justify-content:center;gap:7px;border-radius:8px;font:inherit;font-size:12px;font-weight:650;cursor:pointer;transition:background .14s,border-color .14s,transform .14s}.apr-toggle:active:not(:disabled),.apr-secondary:active:not(:disabled),.apr-primary:active:not(:disabled){transform:translateY(1px)}.apr-toggle{min-width:106px;padding:8px 11px;border:1px solid var(--dborder);background:var(--surface2);color:var(--dtext)}.apr-toggle.is-on{border-color:color-mix(in srgb,var(--danger) 35%,var(--dborder));color:var(--danger)}.apr-toggle svg,.apr-secondary svg{width:15px;height:15px}.apr-toggle:disabled,.apr-secondary:disabled,.apr-primary:disabled{opacity:.5;cursor:not-allowed}.apr-message{margin:0;padding:0 20px 18px;color:var(--dmuted);font-size:13px;line-height:1.65;max-width:82ch}
.apr-metrics{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));margin:0;border-top:1px solid var(--dborder);border-bottom:1px solid var(--dborder)}.apr-metrics>div{display:grid;grid-template-columns:minmax(120px,.55fr) minmax(0,1fr);gap:12px;padding:10px 20px;border-bottom:1px solid var(--dborder)}.apr-metrics>div:nth-child(odd){border-right:1px solid var(--dborder)}.apr-metrics>div:nth-last-child(-n+2){border-bottom:0}.apr-metrics dt{color:var(--dmuted2);font-size:10.5px;font-weight:650;letter-spacing:.07em;text-transform:uppercase}.apr-metrics dd{margin:0;color:var(--dtext);font-family:var(--font-geist-mono),monospace;font-size:11.5px;overflow-wrap:anywhere}
.apr-subsection{padding:18px 20px}.apr-subsection+.apr-subsection{border-top:1px solid var(--dborder)}.apr-subsection-head{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:12px}.apr-subsection-head h3{margin:0;color:var(--dtext);font-size:13px}.apr-subsection-head span{color:var(--dmuted2);font-size:11px}.apr-events{display:grid;gap:8px;margin:0;padding:0;list-style:none}.apr-events li{display:flex;align-items:flex-start;gap:9px}.apr-event-dot{width:7px;height:7px;margin-top:5px;border-radius:999px;background:var(--dmuted2)}.apr-event-dot.is-enabled{background:var(--danger)}.apr-events li>div{display:grid;gap:2px}.apr-events strong{font-size:12px;text-transform:capitalize}.apr-events span,.apr-inline-empty{color:var(--dmuted2);font-size:11px}
.apr-campaign-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px}.apr-campaign-panel{min-width:0;padding:14px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface2)}.apr-campaign-head{display:flex;align-items:flex-start;justify-content:space-between;gap:10px;margin-bottom:12px}.apr-campaign-head h4{margin:0 0 3px;color:var(--dtext);font-size:13px}.apr-campaign-head>div>span,.apr-campaign-help{color:var(--dmuted);font-size:11.5px;line-height:1.5}.apr-progress{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));border-top:1px solid var(--dborder);border-left:1px solid var(--dborder);margin-bottom:10px}.apr-progress>div{display:grid;gap:2px;padding:8px;border-right:1px solid var(--dborder);border-bottom:1px solid var(--dborder)}.apr-progress span{color:var(--dmuted2);font-size:9.5px;text-transform:uppercase}.apr-progress strong{font-family:var(--font-geist-mono),monospace;font-size:12px}.apr-count-drift,.apr-live{margin-bottom:9px;color:var(--warning);font-size:11px;line-height:1.45}.apr-live{display:flex;align-items:center;gap:6px;color:var(--dmuted)}.apr-live svg{width:12px;height:12px}.apr-secondary,.apr-primary{padding:7px 10px}.apr-secondary{border:1px solid var(--dborder2);background:var(--surface3);color:var(--dtext)}.apr-primary{width:100%;border:1px solid color-mix(in srgb,var(--primary) 40%,var(--dborder));background:var(--primary);color:var(--primary-foreground)}.apr-campaign-help{min-height:36px;margin:0 0 10px}.apr-preview{display:grid;gap:9px}.apr-preview-count{color:var(--daccent);font-family:var(--font-geist-mono),monospace;font-size:12px}.apr-preview>strong{font-size:12px}.apr-preview pre{max-height:240px;margin:0;padding:10px;overflow:auto;white-space:pre-wrap;border:1px solid var(--dborder);border-radius:8px;background:var(--surface1);color:var(--dmuted);font-family:var(--font-geist),sans-serif;font-size:11.5px;line-height:1.55}
.apr-confirm-list{display:grid;gap:8px;margin:0;padding:14px 14px 14px 32px;border:1px solid var(--dborder);border-radius:9px;background:var(--surface2);color:var(--dtext);font-size:12px;line-height:1.5}.apr-irreversible{width:max-content;padding:4px 7px;border-radius:6px;background:var(--danger-soft);color:var(--danger);font-size:10px;font-weight:750;text-transform:uppercase;letter-spacing:.08em}.apr-send-summary{display:grid;gap:4px;padding:12px;border:1px solid var(--dborder);border-radius:9px;background:var(--surface2)}.apr-send-summary strong{font-size:13px}.apr-send-summary span{color:var(--dmuted);font-size:12px}.apr-spin{animation:apr-spin .8s linear infinite}
@keyframes apr-spin{to{transform:rotate(360deg)}}@keyframes apr-shimmer{to{background-position:-200% 0}}
@media(prefers-reduced-motion:reduce){.apr-loading span,.apr-spin{animation:none}.apr-toggle,.apr-secondary,.apr-primary{transition:none}}
@media(max-width:900px){.apr-metrics,.apr-campaign-grid{grid-template-columns:minmax(0,1fr)}.apr-metrics>div:nth-child(odd){border-right:0}.apr-metrics>div:nth-last-child(2){border-bottom:1px solid var(--dborder)}.apr-restriction-head,.apr-subsection-head{align-items:flex-start;flex-direction:column}.apr-toggle{width:100%}}
`;
