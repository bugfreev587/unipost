"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  Check,
  Clipboard,
  Download,
  ExternalLink,
  KeyRound,
  Loader2,
  Play,
  RefreshCw,
  ShieldCheck,
  Terminal,
  Video,
} from "lucide-react";
import { FeatureFlagGate } from "@/components/feature-flag-gate";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";
import {
  createReviewDomain,
  createReviewJob,
  createReviewKit,
  getReviewJob,
  listPlatformCredentials,
  verifyReviewDomain,
  type PlatformCredential,
  type ReviewDNSRecord,
  type ReviewDomain,
  type ReviewJob,
  type ReviewKit,
} from "@/lib/api";

const REQUIRED_TIKTOK_SCOPES = ["user.info.basic", "video.publish", "video.upload"];
const TIKTOK_DEVELOPER_PORTAL = "https://developers.tiktok.com";

type StepState = "ready" | "blocked" | "done";

export default function AppReviewAutopilotPage() {
  return (
    <FeatureFlagGate
      flag={FEATURE_FLAG_KEYS.appReviewAutopilotV1}
      title="App Review Autopilot is not enabled"
      description="This workspace does not have access to the review recording beta yet."
    >
      <AppReviewAutopilotContent />
    </FeatureFlagGate>
  );
}

function AppReviewAutopilotContent() {
  const { getToken } = useAuth();
  const { id: profileId } = useParams<{ id: string }>();
  const [domain, setDomain] = useState("");
  const [reviewDomain, setReviewDomain] = useState<ReviewDomain | null>(null);
  const [reviewKit, setReviewKit] = useState<ReviewKit | null>(null);
  const [reviewJob, setReviewJob] = useState<ReviewJob | null>(null);
  const [credentials, setCredentials] = useState<PlatformCredential[]>([]);
  const [redirectAttested, setRedirectAttested] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [loadingCreds, setLoadingCreds] = useState(true);
  const [working, setWorking] = useState<"domain" | "verify" | "kit" | "job" | null>(null);
  const [pollingJob, setPollingJob] = useState(false);
  const [error, setError] = useState("");
  const copyTimerRef = useRef<number | null>(null);

  const loadCredentials = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listPlatformCredentials(token);
      setCredentials(res.data ?? []);
    } catch {
      // The readiness panel can still render and point users to the credential page.
    } finally {
      setLoadingCreds(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadCredentials();
  }, [loadCredentials]);

  const refreshReviewJob = useCallback(async (jobId: string) => {
    const token = await getToken();
    if (!token) return;
    const res = await getReviewJob(token, jobId);
    setReviewJob((current) => ({
      ...res.data,
      agent_command: res.data.agent_command ?? current?.agent_command,
      token_expires_at: res.data.token_expires_at ?? current?.token_expires_at,
    }));
  }, [getToken]);

  useEffect(() => {
    if (!reviewJob?.id) return;
    let cancelled = false;
    let interval: number | null = null;
    const tick = async () => {
      if (cancelled) return;
      setPollingJob(true);
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getReviewJob(token, reviewJob.id);
        if (cancelled) return;
        setReviewJob((current) => ({
          ...res.data,
          agent_command: res.data.agent_command ?? current?.agent_command,
          token_expires_at: res.data.token_expires_at ?? current?.token_expires_at,
        }));
        if ((res.data.status === "completed" || res.data.status === "failed") && interval !== null) {
          window.clearInterval(interval);
          interval = null;
        }
      } catch {
        // Keep the local command visible; users can refresh explicitly.
      } finally {
        if (!cancelled) setPollingJob(false);
      }
    };
    void tick();
    interval = window.setInterval(tick, 5000);
    return () => {
      cancelled = true;
      if (interval !== null) window.clearInterval(interval);
    };
  }, [getToken, reviewJob?.id]);

  useEffect(() => () => {
    if (copyTimerRef.current !== null) {
      window.clearTimeout(copyTimerRef.current);
    }
  }, []);

  const hasTikTokCredential = credentials.some((cred) => cred.platform === "tiktok");
  const domainReady = reviewDomain?.status === "ready" && (!reviewDomain.tls_status || reviewDomain.tls_status === "issued");
  const normalizedDomain = domain.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*/, "");
  const redirectURI = normalizedDomain ? `https://${normalizedDomain}/v1/connect/callback/tiktok` : "https://your-review-domain/v1/connect/callback/tiktok";
  const artifactDownloadURL = reviewJob?.artifacts ? `data:application/json;charset=utf-8,${encodeURIComponent(JSON.stringify(reviewJob.artifacts, null, 2))}` : "";

  const steps = useMemo(() => [
    { label: "TikTok credentials", detail: "Client Key and Client Secret saved in White-label.", state: hasTikTokCredential ? "done" : "blocked" as StepState },
    { label: "Review domain", detail: reviewDomain ? `${reviewDomain.domain} (${reviewDomain.status})` : "Create the customer-domain review host.", state: domainReady ? "done" : reviewDomain ? "ready" : "blocked" as StepState },
    { label: "Redirect URI", detail: "Added in the TikTok developer portal.", state: redirectAttested ? "done" : "blocked" as StepState },
    { label: "Recording kit", detail: reviewKit ? `Ready: ${reviewKit.id}` : "Create after all checks pass.", state: reviewKit ? "done" : domainReady && hasTikTokCredential && redirectAttested ? "ready" : "blocked" as StepState },
  ], [domainReady, hasTikTokCredential, redirectAttested, reviewDomain, reviewKit]);

  async function handleCreateDomain() {
    if (!normalizedDomain) return;
    setWorking("domain");
    setError("");
    setReviewKit(null);
    setReviewJob(null);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createReviewDomain(token, { domain: normalizedDomain, provider: "manual" });
      setReviewDomain(res.data);
    } catch (err) {
      setError((err as Error).message || "Failed to create review domain");
    } finally {
      setWorking(null);
    }
  }

  async function handleVerifyDomain() {
    if (!reviewDomain) return;
    setWorking("verify");
    setError("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await verifyReviewDomain(token, reviewDomain.id);
      setReviewDomain(res.data);
    } catch (err) {
      setError((err as Error).message || "DNS is not ready yet");
    } finally {
      setWorking(null);
    }
  }

  async function handleCreateKit() {
    if (!reviewDomain || !redirectAttested) return;
    setWorking("kit");
    setError("");
    setReviewJob(null);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createReviewKit(token, {
        platform: "tiktok",
        use_case: "content_posting",
        review_domain_id: reviewDomain.id,
        redirect_uri_attested: redirectAttested,
        required_scopes: REQUIRED_TIKTOK_SCOPES,
        brand_snapshot: { review_domain: reviewDomain.domain },
        profile_id: profileId,
      });
      setReviewKit(res.data);
    } catch (err) {
      setError((err as Error).message || "Failed to create review kit");
    } finally {
      setWorking(null);
    }
  }

  async function handleCreateJob() {
    if (!reviewKit) return;
    setWorking("job");
    setError("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createReviewJob(token, { review_kit_id: reviewKit.id });
      setReviewJob(res.data);
      void refreshReviewJob(res.data.id);
    } catch (err) {
      setError((err as Error).message || "Failed to create recording job");
    } finally {
      setWorking(null);
    }
  }

  async function handleRefreshJob() {
    if (!reviewJob?.id) return;
    setError("");
    setPollingJob(true);
    try {
      await refreshReviewJob(reviewJob.id);
    } catch (err) {
      setError((err as Error).message || "Failed to refresh recording job");
    } finally {
      setPollingJob(false);
    }
  }

  async function copyText(key: string, value: string) {
    await navigator.clipboard.writeText(value);
    setCopied(key);
    if (copyTimerRef.current !== null) {
      window.clearTimeout(copyTimerRef.current);
    }
    copyTimerRef.current = window.setTimeout(() => {
      setCopied((current) => current === key ? null : current);
      copyTimerRef.current = null;
    }, 1600);
  }

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 24, marginBottom: 28 }}>
        <div style={{ maxWidth: 760 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
            <div style={{ width: 34, height: 34, borderRadius: 8, background: "color-mix(in srgb, var(--daccent) 12%, transparent)", border: "1px solid color-mix(in srgb, var(--daccent) 24%, transparent)", color: "var(--daccent)", display: "grid", placeItems: "center" }}>
              <Video size={17} strokeWidth={1.8} />
            </div>
            <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>App Review Autopilot</div>
          </div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", lineHeight: 1.65 }}>
            Prepare a TikTok app-review recording kit with your white-label credentials, customer-domain review page, required scopes, and a pinned local agent command.
          </div>
        </div>
        <a className="dbtn" href={TIKTOK_DEVELOPER_PORTAL} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 7, whiteSpace: "nowrap" }}>
          TikTok portal <ExternalLink size={13} />
        </a>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(min(100%, 360px), 1fr))", gap: 18, alignItems: "start" }}>
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">
              <span>Readiness</span>
              <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>TikTok</span>
            </div>
            <div className="settings-section-body" style={{ display: "grid", gap: 12 }}>
              {steps.map((step) => (
                <StatusRow key={step.label} state={step.state} label={step.label} detail={step.detail} />
              ))}
            </div>
          </section>

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">Customer domain</div>
            <div className="settings-section-body">
              <label className="dform-label">Review domain</label>
              <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) auto", gap: 10 }}>
                <input
                  className="dform-input"
                  placeholder="review.customer.com"
                  value={domain}
                  onChange={(event) => setDomain(event.target.value)}
                />
                <button className="dbtn dbtn-primary" onClick={handleCreateDomain} disabled={!normalizedDomain || working === "domain"} style={{ minWidth: 142 }}>
                  {working === "domain" ? <ButtonLoading label="Preparing" /> : "Prepare domain"}
                </button>
              </div>
              <div style={{ marginTop: 8, fontSize: 12, color: "var(--dmuted)", lineHeight: 1.55 }}>
                Use the same domain you want visible in the recording address bar. DNS automation will attach here; manual records are shown until provider authorization is connected.
              </div>
            </div>
          </section>

          {reviewDomain && (
            <section className="settings-section" style={{ marginBottom: 0 }}>
              <div className="settings-section-header">
                <span>DNS records</span>
                <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{reviewDomain.status}</span>
              </div>
              <div className="settings-section-body" style={{ display: "grid", gap: 10 }}>
                {reviewDomain.dns_records.map((record, index) => (
                  <DNSRecordRow
                    key={`${record.type}-${record.name}-${index}`}
                    record={record}
                    copied={copied === `dns-${index}`}
                    onCopy={() => copyText(`dns-${index}`, `${record.type} ${record.name} ${record.value}`)}
                  />
                ))}
                <button className="dbtn" onClick={handleVerifyDomain} disabled={working === "verify" || domainReady} style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7 }}>
                  {working === "verify" ? <ButtonLoading label="Checking DNS" /> : domainReady ? <><Check size={14} /> Domain ready</> : "Check DNS"}
                </button>
                <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.55 }}>
                  Once DNS and certificate issuance are ready, create the review kit. If propagation is still pending, you can leave this page and check again later without changing the TikTok setup.
                </div>
              </div>
            </section>
          )}

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">TikTok developer app</div>
            <div className="settings-section-body" style={{ display: "grid", gap: 14 }}>
              <div style={{ display: "grid", gap: 8 }}>
                <label className="dform-label">OAuth redirect URI</label>
                <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) auto", gap: 8 }}>
                  <code className="mono" style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", padding: "9px 10px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", color: "var(--dtext)" }}>
                    {redirectURI}
                  </code>
                  <button className="dbtn" onClick={() => copyText("redirect", redirectURI)} style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                    {copied === "redirect" ? <Check size={14} /> : <Clipboard size={14} />} Copy
                  </button>
                </div>
              </div>
              <label style={{ display: "flex", alignItems: "flex-start", gap: 10, fontSize: 13, color: "var(--dtext)", lineHeight: 1.5 }}>
                <input type="checkbox" checked={redirectAttested} onChange={(event) => setRedirectAttested(event.target.checked)} style={{ marginTop: 2 }} />
                <span>I added this redirect URI to the TikTok developer app.</span>
              </label>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                {REQUIRED_TIKTOK_SCOPES.map((scope) => (
                  <span key={scope} className="dbadge dbadge-gray" style={{ fontSize: 11 }}>{scope}</span>
                ))}
              </div>
            </div>
          </section>
        </div>

        <aside style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">Recording kit</div>
            <div className="settings-section-body" style={{ display: "grid", gap: 12 }}>
              <div style={{ display: "flex", gap: 12, alignItems: "flex-start" }}>
                <div style={{ width: 34, height: 34, borderRadius: 8, background: hasTikTokCredential ? "var(--success-soft)" : "var(--warning-soft)", color: hasTikTokCredential ? "var(--success)" : "var(--warning)", display: "grid", placeItems: "center", flexShrink: 0 }}>
                  <KeyRound size={16} />
                </div>
                <div style={{ minWidth: 0 }}>
                  <div style={{ fontSize: 14, fontWeight: 650, color: "var(--dtext)", marginBottom: 4 }}>
                    {loadingCreds ? "Checking credentials" : hasTikTokCredential ? "TikTok credentials found" : "TikTok credentials missing"}
                  </div>
                  <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
                    {hasTikTokCredential ? "The review flow will use your own TikTok app credentials." : "Save your TikTok Client Key and Client Secret before creating a recording kit."}
                  </div>
                  {!hasTikTokCredential && (
                    <Link href={`/projects/${profileId}/accounts/native`} style={{ display: "inline-flex", alignItems: "center", gap: 5, marginTop: 8, fontSize: 12, color: "var(--daccent)", textDecoration: "none" }}>
                      Open White-label <ExternalLink size={11} />
                    </Link>
                  )}
                </div>
              </div>

              <button className="dbtn dbtn-primary" onClick={handleCreateKit} disabled={!domainReady || !hasTikTokCredential || !redirectAttested || working === "kit"} style={{ width: "100%", justifyContent: "center" }}>
                {working === "kit" ? <ButtonLoading label="Creating kit" /> : reviewKit ? "Kit ready" : "Create review kit"}
              </button>
              <button className="dbtn" onClick={handleCreateJob} disabled={!reviewKit || working === "job"} style={{ width: "100%", justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7 }}>
                {working === "job" ? <ButtonLoading label="Starting" /> : <><Play size={14} /> {reviewJob ? "Re-record" : "Start recording"}</>}
              </button>
              {error && (
                <div style={{ padding: "10px 12px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", color: "var(--danger)", fontSize: 13, lineHeight: 1.5 }}>
                  {error}
                </div>
              )}
            </div>
          </section>

          {reviewJob && (
            <section className="settings-section" style={{ marginBottom: 0 }}>
              <div className="settings-section-header">
                <span>Recording status</span>
                <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{formatJobStatus(reviewJob.status)}</span>
              </div>
              <div className="settings-section-body" style={{ display: "grid", gap: 12 }}>
                <div style={{ display: "flex", justifyContent: "space-between", gap: 12, alignItems: "center" }}>
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 650 }}>{reviewJob.id}</div>
                    <div style={{ fontSize: 12, color: "var(--dmuted)", marginTop: 3 }}>{recordingStatusDetail(reviewJob)}</div>
                  </div>
                  <button className="dbtn" onClick={handleRefreshJob} disabled={pollingJob} style={{ display: "inline-flex", alignItems: "center", gap: 6, flexShrink: 0 }}>
                    <RefreshCw size={13} className={pollingJob ? "animate-spin" : ""} /> Refresh
                  </button>
                </div>
                {reviewJob.failure_reason && (
                  <div style={{ padding: "10px 12px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", color: "var(--danger)", fontSize: 13, lineHeight: 1.5 }}>
                    {reviewJob.failure_reason}
                  </div>
                )}
                {reviewJob.video_download_url && (
                  <div style={{ display: "grid", gap: 10 }}>
                    <video src={reviewJob.video_download_url} controls style={{ width: "100%", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", aspectRatio: "16 / 9" }} />
                    <a className="dbtn dbtn-primary" href={reviewJob.video_download_url} download style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7, textDecoration: "none" }}>
                      <Download size={14} /> Download video
                    </a>
                  </div>
                )}
                {artifactDownloadURL && (
                  <a className="dbtn" href={artifactDownloadURL} download={`unipost-review-${reviewJob.id}-artifacts.json`} style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7, textDecoration: "none" }}>
                    <Download size={14} /> Download artifacts
                  </a>
                )}
              </div>
            </section>
          )}

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">Local agent</div>
            <div className="settings-section-body">
              {reviewJob ? (
                <div style={{ display: "grid", gap: 10 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--success)", fontSize: 13, fontWeight: 650 }}>
                    <ShieldCheck size={15} /> Token minted, expires {formatTime(reviewJob.token_expires_at || "")}
                  </div>
                  {reviewJob.agent_command && (
                    <>
                      <pre style={{ margin: 0, padding: 12, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", color: "var(--dtext)", fontSize: 12, lineHeight: 1.6, overflowX: "auto" }}><code>{reviewJob.agent_command}</code></pre>
                      <button className="dbtn dbtn-primary" onClick={() => copyText("agent", reviewJob.agent_command || "")} style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7 }}>
                        {copied === "agent" ? <Check size={14} /> : <Terminal size={14} />} Copy command
                      </button>
                    </>
                  )}
                </div>
              ) : (
                <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.65 }}>
                  When recording starts, UniPost creates a single-use token and pins the CLI version. The local browser and recorder run on the user machine.
                </div>
              )}
            </div>
          </section>
        </aside>
      </div>
    </>
  );
}

function StatusRow({ state, label, detail }: { state: StepState; label: string; detail: string }) {
  const color = state === "done" ? "var(--success)" : state === "ready" ? "var(--warning)" : "var(--dmuted2)";
  const background = state === "done" ? "var(--success-soft)" : state === "ready" ? "var(--warning-soft)" : "var(--surface2)";
  return (
    <div style={{ display: "flex", gap: 10, alignItems: "flex-start", padding: "10px 0", borderBottom: "1px solid var(--dborder)" }}>
      <div style={{ width: 24, height: 24, borderRadius: 7, background, color, display: "grid", placeItems: "center", flexShrink: 0 }}>
        {state === "done" ? <Check size={14} /> : <span style={{ width: 6, height: 6, borderRadius: 999, background: color }} />}
      </div>
      <div style={{ minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 650, color: "var(--dtext)", marginBottom: 2 }}>{label}</div>
        <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.45 }}>{detail}</div>
      </div>
    </div>
  );
}

function DNSRecordRow({ record, copied, onCopy }: { record: ReviewDNSRecord; copied: boolean; onCopy: () => void }) {
  return (
    <div style={{ display: "grid", gridTemplateColumns: "70px minmax(0, 1fr) auto", gap: 8, alignItems: "center", padding: 10, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)" }}>
      <span className="dbadge dbadge-gray" style={{ justifySelf: "start", fontSize: 10 }}>{record.type}</span>
      <div style={{ minWidth: 0 }}>
        <div className="mono" style={{ fontSize: 12, color: "var(--dtext)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{record.name}</div>
        <div className="mono" style={{ fontSize: 12, color: "var(--dmuted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", marginTop: 2 }}>{record.value}</div>
      </div>
      <button className="dbtn" onClick={onCopy} style={{ padding: "6px 10px", display: "inline-flex", alignItems: "center", gap: 5 }}>
        {copied ? <Check size={13} /> : <Clipboard size={13} />} Copy
      </button>
    </div>
  );
}

function ButtonLoading({ label }: { label: string }) {
  return <span style={{ display: "inline-flex", alignItems: "center", gap: 7 }}><Loader2 size={14} className="animate-spin" /> {label}</span>;
}

function formatJobStatus(status: string) {
  return status.replace(/_/g, " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

function recordingStatusDetail(job: ReviewJob) {
  if (job.status === "completed") return "Recording completed. Review the generated video and artifact bundle before submitting.";
  if (job.status === "failed") return "Recording failed. Re-recording will reuse the same domain, credentials, and kit setup.";
  if (job.status === "waiting_for_user") return "Waiting for the user-controlled TikTok login or OAuth consent step.";
  if (job.status === "running") return "The local agent is recording and reporting progress.";
  return "Run the local command to start recording.";
}

function formatTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "soon";
  return date.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}
