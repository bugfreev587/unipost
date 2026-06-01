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
  createTikTokReviewDemoPlan,
  createReviewDomain,
  createReviewJob,
  createReviewKit,
  getTikTokReviewScopeTemplates,
  getReviewJob,
  getReviewState,
  listPlatformCredentials,
  verifyReviewDomain,
  type PlatformCredential,
  type ReviewDNSRecord,
  type ReviewDomain,
  type ReviewJob,
  type ReviewKit,
  type TikTokDemoPlan,
  type TikTokScopeTemplate,
} from "@/lib/api";

const CONTENT_POSTING_SCOPES = ["user.info.basic", "video.upload", "video.publish"];
const ANALYTICS_BASIC_SCOPES = ["user.info.profile", "user.info.stats"];
const ANALYTICS_VIDEO_LIST_SCOPES = ["user.info.profile", "user.info.stats", "video.list"];
const DEFAULT_TIKTOK_SCOPES = CONTENT_POSTING_SCOPES;
const SCOPE_PRESETS = [
  { key: "content", label: "Content Posting API", scopes: CONTENT_POSTING_SCOPES },
  { key: "analytics", label: "Analytics Basic", scopes: ANALYTICS_BASIC_SCOPES },
  { key: "analytics-video", label: "Analytics + Video List", scopes: ANALYTICS_VIDEO_LIST_SCOPES },
];
const FALLBACK_SCOPE_TEMPLATES: TikTokScopeTemplate[] = [
  { scope: "user.info.basic", label: "Basic user info", use_case: "content_posting", primary_surface: "connection_flow,create_post_drawer", required_evidence: "Connected account identity and creator info-driven posting settings." },
  { scope: "video.upload", label: "Upload videos", use_case: "content_posting", primary_surface: "create_post_drawer", required_evidence: "Video file upload, validation, and preview before publish." },
  { scope: "video.publish", label: "Publish videos", use_case: "content_posting", primary_surface: "create_post_drawer,posts_list,tiktok_profile", required_evidence: "Publish action, status, and TikTok-side verification." },
  { scope: "user.info.profile", label: "Profile info", use_case: "analytics", primary_surface: "tiktok_analytics", required_evidence: "Profile card, avatar, display name, and username." },
  { scope: "user.info.stats", label: "Account stats", use_case: "analytics", primary_surface: "tiktok_analytics", required_evidence: "Followers, following, likes, and video count." },
  { scope: "video.list", label: "Video list", use_case: "analytics", primary_surface: "tiktok_analytics,tiktok_profile", required_evidence: "Video list compared with TikTok profile." },
];
const TIKTOK_DEVELOPER_PORTAL = "https://developers.tiktok.com";
const API_BASE_URL = (process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev").replace(/\/+$/, "");

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
  const [scopeTemplates, setScopeTemplates] = useState<TikTokScopeTemplate[]>([]);
  const [selectedScopes, setSelectedScopes] = useState<string[]>(DEFAULT_TIKTOK_SCOPES);
  const [demoPlan, setDemoPlan] = useState<TikTokDemoPlan | null>(null);
  const [redirectAttested, setRedirectAttested] = useState(false);
  const [oauthResetConfirmed, setOauthResetConfirmed] = useState(false);
  const [copied, setCopied] = useState<string | null>(null);
  const [loadingCreds, setLoadingCreds] = useState(true);
  const [loadingState, setLoadingState] = useState(true);
  const [loadingScopes, setLoadingScopes] = useState(true);
  const [loadingPlan, setLoadingPlan] = useState(false);
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

  const loadScopeTemplates = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getTikTokReviewScopeTemplates(token);
      setScopeTemplates(res.data ?? []);
    } catch {
      // The fixed presets still allow the page to render while the API is unavailable.
    } finally {
      setLoadingScopes(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadScopeTemplates();
  }, [loadScopeTemplates]);

  const loadReviewState = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await getReviewState(token);
      if (res.data.domain) {
        setReviewDomain(res.data.domain);
        setDomain(res.data.domain.domain);
      }
      if (res.data.kit) {
        setReviewKit(res.data.kit);
        setRedirectAttested(true);
        if (res.data.kit.required_scopes?.length) {
          setSelectedScopes(res.data.kit.required_scopes);
        }
      }
      if (res.data.job) {
        setReviewJob(res.data.job);
      }
    } catch {
      // Users can still recreate or verify the setup manually from this page.
    } finally {
      setLoadingState(false);
    }
  }, [getToken]);

  useEffect(() => {
    loadReviewState();
  }, [loadReviewState]);

  useEffect(() => {
    let cancelled = false;
    const buildPlan = async () => {
      if (selectedScopes.length === 0) {
        setDemoPlan(null);
        return;
      }
      setLoadingPlan(true);
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await createTikTokReviewDemoPlan(token, { scopes: selectedScopes });
        if (!cancelled) {
          setDemoPlan(res.data);
        }
      } catch (err) {
        if (!cancelled) {
          setDemoPlan(null);
          setError((err as Error).message || "Failed to generate TikTok demo plan");
        }
      } finally {
        if (!cancelled) setLoadingPlan(false);
      }
    };
    void buildPlan();
    return () => {
      cancelled = true;
    };
  }, [getToken, selectedScopes]);

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
  const domainMatchesCurrentSetup = Boolean(reviewDomain && normalizedDomain && reviewDomain.domain === normalizedDomain);
  const redirectURI = `${API_BASE_URL}/v1/connect/callback/tiktok`;
  const artifactDownloadURL = reviewJob?.artifacts ? `data:application/json;charset=utf-8,${encodeURIComponent(JSON.stringify(reviewJob.artifacts, null, 2))}` : "";
  const videoArtifacts = reviewJob?.video_artifacts ?? [];
  const selectedScopeSet = useMemo(() => new Set(selectedScopes), [selectedScopes]);
  const visibleScopeTemplates = scopeTemplates.length > 0 ? scopeTemplates : FALLBACK_SCOPE_TEMPLATES;
  const planReady = Boolean(demoPlan && demoPlan.requested_scopes.length > 0);
  const aiGuidedRecording = Boolean(reviewJob?.agent_command?.includes("--ai-guided"));
  const reviewSurface = reviewSurfaceForUseCase(demoPlan?.use_case ?? reviewKit?.use_case, selectedScopes);
  const reviewSessionToken = reviewSessionTokenFromCommand(reviewJob?.agent_command);
  const manualReviewURL = reviewDomain && reviewJob && reviewSessionToken
    ? buildReviewLaunchURL(reviewDomain.domain, reviewSurface, reviewSessionToken)
    : "";

  const steps = useMemo(() => [
    { label: "TikTok credentials", detail: "Client Key and Client Secret saved in Platform Credentials.", state: hasTikTokCredential ? "done" : "blocked" as StepState },
    { label: "API scopes", detail: demoPlan ? `${demoPlan.requested_scopes.length} scopes mapped to ${demoPlan.segments.length} video segments.` : loadingPlan ? "Generating scope-based recording plan." : "Select the scopes requested in TikTok Developer Portal.", state: demoPlan ? "done" : selectedScopes.length ? "ready" : "blocked" as StepState },
    { label: "Review domain", detail: reviewDomain ? `${reviewDomain.domain} (${reviewDomain.status})` : loadingState ? "Loading existing review host." : "Create the customer-domain review host.", state: domainReady ? "done" : reviewDomain ? "ready" : "blocked" as StepState },
    { label: "Redirect URI", detail: "Added in the TikTok developer portal.", state: redirectAttested ? "done" : "blocked" as StepState },
    { label: "OAuth reset", detail: "Remove existing app access in TikTok mobile settings so the consent screen appears.", state: oauthResetConfirmed ? "done" : "blocked" as StepState },
    { label: "Recording kit", detail: reviewKit ? `Ready: ${reviewKit.id}` : loadingState ? "Loading existing recording kit." : "Create after all checks pass.", state: reviewKit ? "done" : domainReady && hasTikTokCredential && redirectAttested && oauthResetConfirmed && planReady ? "ready" : "blocked" as StepState },
  ], [demoPlan, domainReady, hasTikTokCredential, loadingPlan, loadingState, oauthResetConfirmed, planReady, redirectAttested, reviewDomain, reviewKit, selectedScopes.length]);

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

  function applyScopePreset(scopes: string[]) {
    setReviewKit(null);
    setReviewJob(null);
    setSelectedScopes(scopes);
  }

  function toggleScope(scope: string) {
    setReviewKit(null);
    setReviewJob(null);
    setSelectedScopes((current) => {
      if (current.includes(scope)) {
        return current.filter((item) => item !== scope);
      }
      return [...current, scope].sort((a, b) => a.localeCompare(b));
    });
  }

  async function handleCreateKit() {
    if (!reviewDomain || !redirectAttested || !demoPlan || !oauthResetConfirmed) return;
    setWorking("kit");
    setError("");
    setReviewJob(null);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createReviewKit(token, {
        platform: "tiktok",
        use_case: demoPlan.use_case,
        review_domain_id: reviewDomain.id,
        redirect_uri_attested: redirectAttested,
        required_scopes: demoPlan.requested_scopes,
        brand_snapshot: { review_domain: reviewDomain.domain, selected_scope_count: demoPlan.requested_scopes.length },
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
            Prepare a TikTok app-review recording kit with your platform credentials, customer-domain review page, required scopes, and a pinned local agent command.
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
                <button className="dbtn dbtn-primary" onClick={handleCreateDomain} disabled={!normalizedDomain || domainMatchesCurrentSetup || working === "domain"} style={{ minWidth: 142 }}>
                  {working === "domain" ? <ButtonLoading label="Preparing" /> : domainMatchesCurrentSetup ? "Domain prepared" : "Prepare domain"}
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
                {selectedScopes.map((scope) => (
                  <span key={scope} className="dbadge dbadge-gray" style={{ fontSize: 11 }}>{scope}</span>
                ))}
              </div>
            </div>
          </section>

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">
              <span>TikTok API scopes</span>
              <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{demoPlan?.use_case ?? "select"}</span>
            </div>
            <div className="settings-section-body" style={{ display: "grid", gap: 14 }}>
              <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                {SCOPE_PRESETS.map((preset) => (
                  <button
                    key={preset.key}
                    className="dbtn"
                    onClick={() => applyScopePreset(preset.scopes)}
                    style={{ display: "inline-flex", alignItems: "center", gap: 6, padding: "7px 10px" }}
                  >
                    {preset.label}
                  </button>
                ))}
              </div>
              <div style={{ display: "grid", gap: 8 }}>
                {visibleScopeTemplates.map((template) => (
                  <label
                    key={template.scope}
                    style={{
                      display: "grid",
                      gridTemplateColumns: "auto minmax(0, 1fr)",
                      gap: 10,
                      padding: 10,
                      borderRadius: 8,
                      border: "1px solid var(--dborder)",
                      background: selectedScopeSet.has(template.scope) ? "color-mix(in srgb, var(--daccent) 7%, var(--surface2))" : "var(--surface2)",
                      color: "var(--dtext)",
                    }}
                  >
                    <input
                      type="checkbox"
                      checked={selectedScopeSet.has(template.scope)}
                      onChange={() => toggleScope(template.scope)}
                      style={{ marginTop: 3 }}
                    />
                    <span style={{ minWidth: 0 }}>
                      <span style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8, marginBottom: 3 }}>
                        <strong style={{ fontSize: 13 }}>{template.scope}</strong>
                        <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{template.use_case}</span>
                      </span>
                      <span style={{ display: "block", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.45 }}>
                        {template.required_evidence}
                      </span>
                    </span>
                  </label>
                ))}
              </div>
              {loadingScopes && (
                <div style={{ fontSize: 12, color: "var(--dmuted)" }}>Loading scope templates...</div>
              )}
            </div>
          </section>

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">
              <span>Generated demo plan</span>
              <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{demoPlan?.recording.resolution ?? "1080p"}</span>
            </div>
            <div className="settings-section-body" style={{ display: "grid", gap: 12 }}>
              {loadingPlan && (
                <div style={{ fontSize: 13, color: "var(--dmuted)" }}>Generating reviewer-visible evidence steps...</div>
              )}
              {demoPlan ? (
                <>
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 8 }}>
                    <PlanMetric label="Files" value={`${demoPlan.segments.length}`} />
                    <PlanMetric label="Limit" value={`<${demoPlan.recording.max_file_size_mb}MB`} />
                    <PlanMetric label="FPS" value={`${demoPlan.recording.fps}`} />
                  </div>
                  <div style={{ display: "grid", gap: 8 }}>
                    {demoPlan.segments.map((segment) => (
                      <div key={segment.key} style={{ padding: 10, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)" }}>
                        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 10 }}>
                          <div style={{ minWidth: 0 }}>
                            <div style={{ fontSize: 13, fontWeight: 650, color: "var(--dtext)", marginBottom: 3 }}>{segment.title}</div>
                            <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.45 }}>{segment.description}</div>
                          </div>
                          <span className="dbadge dbadge-gray" style={{ fontSize: 10, flexShrink: 0 }}>{Math.ceil(segment.estimated_duration_sec / 60)}m</span>
                        </div>
                        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginTop: 8 }}>
                          {segment.scopes.map((scope) => (
                            <span key={scope} className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{scope}</span>
                          ))}
                        </div>
                        {segment.steps.length > 0 && (
                          <div style={{ display: "grid", gap: 7, marginTop: 10, paddingTop: 10, borderTop: "1px solid var(--dborder)" }}>
                            {segment.steps.map((step, index) => (
                              <div key={step.key} style={{ display: "grid", gridTemplateColumns: "24px minmax(0, 1fr)", gap: 8, alignItems: "start" }}>
                                <span className="mono" style={{ width: 24, height: 24, borderRadius: 7, display: "grid", placeItems: "center", background: "var(--surface)", border: "1px solid var(--dborder)", color: "var(--dmuted)", fontSize: 11 }}>
                                  {index + 1}
                                </span>
                                <span style={{ minWidth: 0 }}>
                                  <span style={{ display: "block", color: "var(--dtext)", fontSize: 12, fontWeight: 650 }}>{step.title}</span>
                                  <span style={{ display: "block", color: "var(--dmuted)", fontSize: 12, lineHeight: 1.45, marginTop: 2 }}>{step.evidence}</span>
                                </span>
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                  {demoPlan.warnings.length > 0 && (
                    <div style={{ display: "grid", gap: 6 }}>
                      {demoPlan.warnings.map((warning) => (
                        <div key={warning} style={{ fontSize: 12, color: "var(--warning)", lineHeight: 1.45 }}>
                          {warning}
                        </div>
                      ))}
                    </div>
                  )}
                </>
              ) : !loadingPlan ? (
                <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
                  Select at least one supported TikTok scope to generate the recording plan.
                </div>
              ) : null}
            </div>
          </section>

          <section className="settings-section" style={{ marginBottom: 0 }}>
            <div className="settings-section-header">OAuth consent reset</div>
            <div className="settings-section-body" style={{ display: "grid", gap: 10 }}>
              <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
                Before recording, remove existing access in TikTok mobile app: Settings and privacy, Security and permissions, Apps and services permissions, then remove UniPost or your customer app. This forces TikTok to show the authorization scope page in the demo.
              </div>
              <label style={{ display: "flex", alignItems: "flex-start", gap: 10, fontSize: 13, color: "var(--dtext)", lineHeight: 1.5 }}>
                <input type="checkbox" checked={oauthResetConfirmed} onChange={(event) => setOauthResetConfirmed(event.target.checked)} style={{ marginTop: 2 }} />
                <span>I removed existing TikTok app authorization from TikTok mobile settings for this test account.</span>
              </label>
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
                    <Link href={`/projects/${profileId}/credentials`} style={{ display: "inline-flex", alignItems: "center", gap: 5, marginTop: 8, fontSize: 12, color: "var(--daccent)", textDecoration: "none" }}>
                      Open Platform Credentials <ExternalLink size={11} />
                    </Link>
                  )}
                </div>
              </div>

              <button className="dbtn dbtn-primary" onClick={handleCreateKit} disabled={!domainReady || !hasTikTokCredential || !redirectAttested || !oauthResetConfirmed || !demoPlan || Boolean(reviewKit) || working === "kit"} style={{ width: "100%", justifyContent: "center" }}>
                {working === "kit" ? <ButtonLoading label="Creating kit" /> : reviewKit ? "Kit ready" : "Create review kit"}
              </button>
              <button className="dbtn" onClick={handleCreateJob} disabled={!reviewKit || working === "job"} style={{ width: "100%", justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7 }}>
                {working === "job" ? <ButtonLoading label="Starting" /> : <><Play size={14} /> {reviewJob ? "Re-record" : "Start recording"}</>}
              </button>
              {manualReviewURL && (
                <div style={{ display: "grid", gap: 9, padding: 12, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)" }}>
                  <div style={{ fontSize: 13, fontWeight: 700, color: "var(--dtext)" }}>Manual review workspace</div>
                  <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.55 }}>
                    Open this customer-domain page in a clean Chrome or Incognito window to validate the flow before automatic recording. The page starts with no video selected so you can upload, remove, preview, connect TikTok, open policy links, and publish manually.
                  </div>
                  <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) auto", gap: 8 }}>
                    <a className="dbtn dbtn-primary" href={manualReviewURL} target="_blank" rel="noopener noreferrer" style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7, textDecoration: "none" }}>
                      <ExternalLink size={14} /> Open review page
                    </a>
                    <button className="dbtn" onClick={() => copyText("manual-review", manualReviewURL)} style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                      {copied === "manual-review" ? <Check size={14} /> : <Clipboard size={14} />} Copy
                    </button>
                  </div>
                </div>
              )}
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
                <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                  {aiGuidedRecording && <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>AI-guided recording</span>}
                  <span className="dbadge dbadge-gray" style={{ fontSize: 10 }}>{formatJobStatus(reviewJob.status)}</span>
                </span>
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
                {videoArtifacts.length > 0 ? (
                  <div style={{ display: "grid", gap: 8 }}>
                    {videoArtifacts.map((artifact, index) => (
                      <div key={artifact.file_id || artifact.segment_key || index} style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) auto", gap: 10, alignItems: "center", padding: 10, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)" }}>
                        <div style={{ minWidth: 0 }}>
                          <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 700, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                            {artifact.filename || artifact.segment_key || `Review video part ${index + 1}`}
                          </div>
                          <div style={{ display: "flex", gap: 8, flexWrap: "wrap", marginTop: 5, fontSize: 11, color: "var(--dmuted)" }}>
                            {artifact.segment_key && <span>{artifact.segment_key}</span>}
                            {artifact.size_bytes ? <span>{formatBytes(artifact.size_bytes)}</span> : null}
                            {artifact.duration_sec ? <span>{Math.round(artifact.duration_sec)}s</span> : null}
                            {artifact.scopes?.length ? <span>{artifact.scopes.join(", ")}</span> : null}
                          </div>
                        </div>
                        {artifact.download_url && (
                          <a className="dbtn dbtn-primary" href={artifact.download_url} download style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7, textDecoration: "none" }}>
                            <Download size={14} /> Part {index + 1}
                          </a>
                        )}
                      </div>
                    ))}
                  </div>
                ) : reviewJob.video_download_url && (
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
                      {aiGuidedRecording && (
                        <div style={{ padding: "10px 12px", borderRadius: 8, background: "var(--success-soft)", border: "1px solid color-mix(in srgb, var(--success) 22%, transparent)", color: "var(--success)", fontSize: 12, lineHeight: 1.55 }}>
                          UniPost AI will guide the local browser through the review plan. Passwords, QR scans, and verification codes remain manual and are not sent to AI.
                        </div>
                      )}
                      <div style={{ padding: "10px 12px", borderRadius: 8, background: "var(--warning-soft)", border: "1px solid color-mix(in srgb, var(--warning) 24%, transparent)", color: "var(--warning)", fontSize: 12, lineHeight: 1.55 }}>
                        macOS may require granting Screen Recording permission to Terminal or iTerm once. If the agent stops at preflight, approve the permission in System Settings, restart the terminal, and run this same command again.
                      </div>
                      <pre style={{ margin: 0, padding: 12, borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", color: "var(--dtext)", fontSize: 12, lineHeight: 1.6, overflowX: "auto" }}><code>{reviewJob.agent_command}</code></pre>
                      <button className="dbtn dbtn-primary" onClick={() => copyText("agent", reviewJob.agent_command || "")} style={{ justifyContent: "center", display: "inline-flex", alignItems: "center", gap: 7 }}>
                        {copied === "agent" ? <Check size={14} /> : <Terminal size={14} />} Copy command
                      </button>
                    </>
                  )}
                </div>
              ) : (
                <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.65 }}>
                  When recording starts, UniPost creates a single-use token and pins the CLI version. First validate the manual review page, then run the local command for automatic recording. On macOS, the first run may ask you to grant Screen Recording permission to Terminal, then restart the command once.
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

function PlanMetric({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ padding: "9px 10px", borderRadius: 8, border: "1px solid var(--dborder)", background: "var(--surface2)", minWidth: 0 }}>
      <div style={{ fontSize: 10, textTransform: "uppercase", letterSpacing: 0.4, color: "var(--dmuted)", marginBottom: 3 }}>{label}</div>
      <div className="mono" style={{ fontSize: 13, color: "var(--dtext)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{value}</div>
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

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "";
  if (value < 1024 * 1024) return `${Math.round(value / 1024)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}

function reviewSurfaceForUseCase(useCase: string | undefined, scopes: string[]): "posting" | "analytics" {
  if (useCase === "analytics") return "analytics";
  if (useCase === "content_posting") return "posting";
  const hasPostingScope = scopes.some((scope) => scope === "video.upload" || scope === "video.publish");
  return hasPostingScope ? "posting" : "analytics";
}

function reviewSessionTokenFromCommand(command: string | undefined) {
  if (!command) return "";
  const match = command.match(/--session-token(?:=|\s+)(["']?)([^\s"']+)\1/);
  return match?.[2] ?? "";
}

function buildReviewLaunchURL(domain: string, surface: "posting" | "analytics", token: string) {
  const host = domain.trim().toLowerCase().replace(/^https?:\/\//, "").replace(/\/.*/, "");
  if (!host || !token) return "";
  const url = new URL(surface === "analytics" ? "/tiktok/analytics/session" : "/tiktok/posting/session", `https://${host}`);
  url.searchParams.set("token", token);
  return url.toString();
}
