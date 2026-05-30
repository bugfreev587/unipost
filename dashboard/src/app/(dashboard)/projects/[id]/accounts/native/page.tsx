"use client";

import { useCallback, useEffect, useState, type ChangeEvent } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { ExternalLink, ImageIcon, Lock, Trash2, Upload } from "lucide-react";
import { ConfirmModal } from "@/components/confirm-modal";
import { WhiteLabelStats } from "@/components/dashboard/connection-stats";
import {
  getProfile,
  updateProfile,
  uploadProfileLogo,
  deleteProfileLogo,
  listPlatformCredentials,
  createPlatformCredential,
  deletePlatformCredential,
  getApiLimits,
  type Profile,
  type PlatformCredential,
} from "@/lib/api";

const LOGO_MAX_BYTES = 2 * 1024 * 1024;
const HEX_COLOR_RE = /^#[0-9a-fA-F]{6}$/;

const CRED_PLATFORMS = [
  { id: "instagram", name: "Meta (Instagram / Threads)", idLabel: "App ID", secretLabel: "App Secret", whiteLabelDocs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "facebook", name: "Facebook Page", idLabel: "App ID", secretLabel: "App Secret", whiteLabelDocs: "/docs/white-label/meta", developerPortal: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/linkedin", developerPortal: "https://developer.linkedin.com" },
  { id: "pinterest", name: "Pinterest", idLabel: "App ID", secretLabel: "App Secret", whiteLabelDocs: null, developerPortal: "https://developers.pinterest.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/tiktok", developerPortal: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/youtube", developerPortal: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", whiteLabelDocs: "/docs/white-label/twitter", developerPortal: "https://developer.x.com" },
];

export default function NativeModePage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [creds, setCreds] = useState<PlatformCredential[]>([]);
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState("");
  const [credForms, setCredForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [credSaving, setCredSaving] = useState<string | null>(null);
  const [credError, setCredError] = useState("");
  const [removeTarget, setRemoveTarget] = useState<string | null>(null);
  const [brandingDisplayName, setBrandingDisplayName] = useState("");
  const [brandingPrimaryColor, setBrandingPrimaryColor] = useState("#111111");
  const [brandingHidePoweredBy, setBrandingHidePoweredBy] = useState(false);
  const [brandingSaving, setBrandingSaving] = useState(false);
  const [logoUploading, setLogoUploading] = useState(false);
  const [brandingError, setBrandingError] = useState("");
  const [brandingMessage, setBrandingMessage] = useState("");
  const [planAllowsWhiteLabel, setPlanAllowsWhiteLabel] = useState<boolean | null>(null);
  const [whiteLabelPlatformLimit, setWhiteLabelPlatformLimit] = useState<number | null>(null);
  const [planAllowsHostedConnectBranding, setPlanAllowsHostedConnectBranding] = useState<boolean | null>(null);
  const [planAllowsHidePoweredBy, setPlanAllowsHidePoweredBy] = useState<boolean | null>(null);

  const applyProfile = useCallback((next: Profile) => {
    setProfile(next);
    setBrandingDisplayName(next.branding_display_name || next.name || "");
    setBrandingPrimaryColor(next.branding_primary_color || "#111111");
    setBrandingHidePoweredBy(Boolean(next.branding_hide_powered_by));
  }, []);

  const loadPage = useCallback(async () => {
    try {
      setPageError("");
      const token = await getToken();
      if (!token) return;
      const [profileRes, credsRes, limitsRes] = await Promise.all([
        getProfile(token, profileId),
        listPlatformCredentials(token),
        getApiLimits(token).catch(() => null),
      ]);
      applyProfile(profileRes.data);
      setCreds(credsRes.data ?? []);
      if (limitsRes) {
        setPlanAllowsWhiteLabel(limitsRes.data.white_label_platform_limit !== 0);
        setWhiteLabelPlatformLimit(limitsRes.data.white_label_platform_limit);
        setPlanAllowsHostedConnectBranding(limitsRes.data.plan_allows_hosted_connect_branding);
        setPlanAllowsHidePoweredBy(limitsRes.data.plan_allows_hide_powered_by);
      }
    } catch (e) {
      setPageError((e as Error).message || "Failed to load white-label settings");
    }
    finally { setLoading(false); }
  }, [applyProfile, getToken, profileId]);

  useEffect(() => { loadPage(); }, [loadPage]);

  function updateCredForm(platform: string, field: "clientId" | "clientSecret", value: string) {
    setCredForms((prev) => ({ ...prev, [platform]: { ...prev[platform], [field]: value } }));
  }

  async function handleCredSave(platform: string) {
    const form = credForms[platform];
    if (!form?.clientId || !form?.clientSecret) return;
    setCredSaving(platform); setCredError("");
    try {
      const token = await getToken();
      if (!token) return;
      await createPlatformCredential(token, {
        platform,
        client_id: form.clientId,
        client_secret: form.clientSecret,
      });
      setCredForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      loadPage();
    } catch (e) {
      setCredError((e as Error).message || "Failed to save");
    } finally {
      setCredSaving(null);
    }
  }

  async function handleCredDelete(platform: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await deletePlatformCredential(token, platform);
      loadPage();
    } catch { /* silent */ }
    finally { setRemoveTarget(null); }
  }

  async function handleBrandingSave() {
    if (!profile) return;
    const displayName = brandingDisplayName.trim();
    const primaryColor = brandingPrimaryColor.trim();
    if (displayName.length > 60) {
      setBrandingError("Display name must be 60 characters or fewer.");
      return;
    }
    if (!HEX_COLOR_RE.test(primaryColor)) {
      setBrandingError("Primary color must be a 6-digit hex value, for example #111111.");
      return;
    }

    setBrandingSaving(true);
    setBrandingError("");
    setBrandingMessage("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateProfile(token, profile.id, {
        branding_display_name: displayName,
        branding_primary_color: primaryColor,
        branding_hide_powered_by: planAllowsHidePoweredBy === true ? brandingHidePoweredBy : false,
      });
      applyProfile(res.data);
      setBrandingMessage("Profile branding saved.");
    } catch (e) {
      setBrandingError((e as Error).message || "Failed to save profile branding");
    } finally {
      setBrandingSaving(false);
    }
  }

  async function handleLogoUpload(event: ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    if (!file || !profile) return;
    if (file.type && file.type !== "image/png" && file.type !== "image/jpeg") {
      setBrandingError("Logo must be a PNG or JPG image.");
      event.currentTarget.value = "";
      return;
    }
    if (file.size > LOGO_MAX_BYTES) {
      setBrandingError("Logo must be 2 MB or smaller.");
      event.currentTarget.value = "";
      return;
    }

    setLogoUploading(true);
    setBrandingError("");
    setBrandingMessage("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await uploadProfileLogo(token, profile.id, file);
      applyProfile(res.data);
      setBrandingMessage("Logo uploaded.");
    } catch (e) {
      setBrandingError((e as Error).message || "Failed to upload logo");
    } finally {
      setLogoUploading(false);
      event.currentTarget.value = "";
    }
  }

  async function handleLogoDelete() {
    if (!profile?.branding_logo_url) return;
    setLogoUploading(true);
    setBrandingError("");
    setBrandingMessage("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await deleteProfileLogo(token, profile.id);
      applyProfile(res.data);
      setBrandingMessage("Logo removed.");
    } catch (e) {
      setBrandingError((e as Error).message || "Failed to remove logo");
    } finally {
      setLogoUploading(false);
    }
  }

  const configuredPlatforms = new Set(creds.map((c) => c.platform));
  const brandingLocked = planAllowsHostedConnectBranding === false;
  const attributionLocked = planAllowsHidePoweredBy === false;
  const previewName = brandingDisplayName.trim() || profile?.name || "Your product";
  const previewColor = HEX_COLOR_RE.test(brandingPrimaryColor.trim()) ? brandingPrimaryColor.trim() : "#111111";
  const effectiveHidePoweredBy = planAllowsHidePoweredBy === true && brandingHidePoweredBy;
  const brandingReady = Boolean(profile?.branding_logo_url) && Boolean(brandingDisplayName.trim()) && HEX_COLOR_RE.test(brandingPrimaryColor.trim());
  const brandingBadgeClass = brandingLocked ? "dbadge-gray" : brandingReady ? "dbadge-green" : "dbadge-amber";
  const brandingBadgeLabel = brandingLocked ? "Locked" : brandingReady ? "Ready" : "Needs setup";

  if (loading) return <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>White-label Profile</div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", marginTop: 6 }}>
            Configure the hosted Connect profile your customers see before OAuth, plus the platform credentials that make OAuth consent use your own developer apps.
          </div>
        </div>
      </div>

      {pageError && (
        <div style={{ padding: "10px 14px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          {pageError}
        </div>
      )}

      <div
        style={{
          padding: "14px 16px",
          marginBottom: 20,
          borderRadius: 10,
          background: "color-mix(in srgb, var(--surface2) 88%, white)",
          border: "1px solid var(--dborder)",
        }}
      >
        <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 4 }}>
          White-label credentials are separate from Quickstart
        </div>
        <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
          Quickstart always uses UniPost&apos;s shared OAuth apps. Save credentials here only if you want to onboard end users through{" "}
          <Link href="/docs/api/connect/sessions/create" style={{ color: "var(--daccent)", textDecoration: "none" }}>
            Connect Sessions
          </Link>{" "}
          or other white-label flows where the platform consent screen should show your own app name. Hosted onboarding keeps <em>Powered by UniPost</em> on Basic and makes it optional on Growth / Team.
        </div>
      </div>

      {whiteLabelPlatformLimit !== null && (
        <div
          style={{
            padding: "14px 16px",
            marginBottom: 20,
            borderRadius: 10,
            background: "color-mix(in srgb, var(--surface2) 88%, white)",
            border: "1px solid var(--dborder)",
          }}
        >
          <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 4 }}>
            White-label capacity
          </div>
          <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
            {whiteLabelPlatformLimit === 0 && "Your current plan uses UniPost's shared OAuth apps only."}
            {whiteLabelPlatformLimit === 1 && `Your current plan includes white-label for 1 platform. ${configuredPlatforms.size}/1 platform slot configured.`}
            {whiteLabelPlatformLimit === -1 && "Your current plan includes white-label across all supported platforms."}
            {" "}
            {planAllowsHidePoweredBy ? "This plan can also hide “Powered by UniPost” from hosted onboarding." : "“Powered by UniPost” remains visible on hosted onboarding at this tier."}
          </div>
        </div>
      )}

      {planAllowsWhiteLabel === false && (
        <div
          style={{
            display: "flex",
            gap: 14,
            alignItems: "flex-start",
            padding: "16px 18px",
            marginBottom: 20,
            background: "color-mix(in srgb, var(--daccent) 9%, transparent)",
            border: "1px solid color-mix(in srgb, var(--daccent) 30%, transparent)",
            borderRadius: 10,
          }}
        >
          <div
            style={{
              width: 34,
              height: 34,
              flexShrink: 0,
              borderRadius: 8,
              background: "color-mix(in srgb, var(--daccent) 18%, transparent)",
              color: "var(--daccent)",
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Lock size={16} />
          </div>
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)", marginBottom: 3 }}>
              White-label starts on Basic
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
              Your current plan uses UniPost&apos;s shared OAuth credentials (Quickstart mode).
              Upgrade to Basic ($19/mo) for 1 branded platform, or Growth ($59/mo) / Team ($149/mo) for all supported platforms plus optional attribution removal.
            </div>
          </div>
          <Link
            href="/settings/billing"
            className="dbtn dbtn-primary"
            style={{ fontSize: 13, padding: "8px 16px", whiteSpace: "nowrap", flexShrink: 0 }}
          >
            Upgrade plan
          </Link>
        </div>
      )}

      <div className="settings-section" style={{ marginBottom: 20 }}>
        <div className="settings-section-header" style={{ gap: 16 }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)" }}>Hosted Connect profile</div>
            <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5, marginTop: 2 }}>
              This is shown to your customers before they continue to each platform&apos;s OAuth screen.
            </div>
          </div>
          <span className={`dbadge ${brandingBadgeClass}`} style={{ flexShrink: 0 }}>
            <span className="dbadge-dot" />
            {brandingBadgeLabel}
          </span>
        </div>
        <div className="settings-section-body">
          {brandingLocked && (
            <div
              style={{
                display: "flex",
                alignItems: "flex-start",
                gap: 10,
                padding: "12px 14px",
                borderRadius: 8,
                background: "color-mix(in srgb, var(--daccent) 8%, transparent)",
                border: "1px solid color-mix(in srgb, var(--daccent) 24%, transparent)",
                color: "var(--dmuted)",
                fontSize: 13,
                lineHeight: 1.55,
                marginBottom: 16,
              }}
            >
              <Lock size={15} style={{ color: "var(--daccent)", flexShrink: 0, marginTop: 2 }} />
              <div>Hosted Connect profile branding starts on Basic. Quickstart still uses UniPost&apos;s shared OAuth apps.</div>
            </div>
          )}

          {(brandingError || brandingMessage) && (
            <div
              style={{
                padding: "10px 14px",
                borderRadius: 8,
                background: brandingError ? "var(--danger-soft)" : "var(--success-soft)",
                border: `1px solid ${brandingError ? "color-mix(in srgb, var(--danger) 24%, transparent)" : "color-mix(in srgb, var(--success) 24%, transparent)"}`,
                fontSize: 13,
                color: brandingError ? "var(--danger)" : "var(--success)",
                marginBottom: 16,
              }}
            >
              {brandingError || brandingMessage}
            </div>
          )}

          <div style={{ display: "flex", gap: 22, alignItems: "stretch", flexWrap: "wrap" }}>
            <div style={{ flex: "1 1 390px", minWidth: 0 }}>
              <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 14, marginBottom: 16 }}>
                <label>
                  <span className="dform-label">Display name</span>
                  <input
                    className="dform-input"
                    value={brandingDisplayName}
                    onChange={(e) => setBrandingDisplayName(e.target.value)}
                    maxLength={60}
                    disabled={brandingLocked}
                    placeholder={profile?.name || "Your product"}
                  />
                </label>
                <label>
                  <span className="dform-label">Primary color</span>
                  <div style={{ display: "flex", gap: 8 }}>
                    <input
                      aria-label="Primary color swatch"
                      type="color"
                      value={HEX_COLOR_RE.test(brandingPrimaryColor) ? brandingPrimaryColor : "#111111"}
                      onChange={(e) => setBrandingPrimaryColor(e.target.value)}
                      disabled={brandingLocked}
                      style={{
                        width: 46,
                        height: 46,
                        padding: 3,
                        borderRadius: 8,
                        border: "1px solid var(--dborder2)",
                        background: "var(--surface2)",
                        cursor: brandingLocked ? "not-allowed" : "pointer",
                      }}
                    />
                    <input
                      className="dform-input"
                      value={brandingPrimaryColor}
                      onChange={(e) => setBrandingPrimaryColor(e.target.value)}
                      disabled={brandingLocked}
                      placeholder="#111111"
                      style={{ fontFamily: "var(--font-geist-mono), monospace", fontSize: 13 }}
                    />
                  </div>
                </label>
              </div>

              <div style={{ display: "flex", alignItems: "center", gap: 14, flexWrap: "wrap", marginBottom: 16 }}>
                <div
                  style={{
                    width: 58,
                    height: 58,
                    borderRadius: 8,
                    border: "1px solid var(--dborder2)",
                    background: "var(--surface2)",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "center",
                    overflow: "hidden",
                    flexShrink: 0,
                  }}
                >
                  {profile?.branding_logo_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={profile.branding_logo_url} alt={`${previewName} logo`} style={{ maxWidth: "100%", maxHeight: "100%", objectFit: "contain" }} />
                  ) : (
                    <ImageIcon size={21} style={{ color: "var(--dmuted2)" }} />
                  )}
                </div>
                <div style={{ minWidth: 220, flex: 1 }}>
                  <div style={{ fontSize: 13, fontWeight: 650, color: "var(--dtext)", marginBottom: 3 }}>Logo</div>
                  <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5 }}>PNG or JPG, 2 MB max. Stored in UniPost R2 and served from the public asset URL.</div>
                </div>
                <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                  <label
                    className="dbtn dbtn-ghost"
                    title="Upload logo"
                    style={{
                      fontSize: 13,
                      opacity: brandingLocked || logoUploading ? 0.55 : 1,
                      cursor: brandingLocked || logoUploading ? "not-allowed" : "pointer",
                    }}
                  >
                    <Upload size={13} />
                    {logoUploading ? "Uploading..." : "Upload"}
                    <input
                      type="file"
                      accept="image/png,image/jpeg"
                      disabled={brandingLocked || logoUploading}
                      onChange={handleLogoUpload}
                      style={{ display: "none" }}
                    />
                  </label>
                  <button
                    type="button"
                    className="dbtn dbtn-danger"
                    onClick={() => void handleLogoDelete()}
                    disabled={brandingLocked || logoUploading || !profile?.branding_logo_url}
                    title="Remove logo"
                    style={{ fontSize: 13 }}
                  >
                    <Trash2 size={13} />
                    Remove
                  </button>
                </div>
              </div>

              <label style={{ display: "flex", alignItems: "flex-start", gap: 9, marginBottom: 18, opacity: attributionLocked ? 0.68 : 1 }}>
                <input
                  type="checkbox"
                  checked={effectiveHidePoweredBy}
                  disabled={brandingLocked || attributionLocked}
                  onChange={(e) => setBrandingHidePoweredBy(e.target.checked)}
                  style={{ marginTop: 3 }}
                />
                <span>
                  <span style={{ display: "block", fontSize: 13, fontWeight: 650, color: "var(--dtext)" }}>Hide Powered by UniPost</span>
                  <span style={{ display: "block", fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5 }}>
                    {attributionLocked ? "Available on Growth and Team." : "Applies to the hosted Connect page footer."}
                  </span>
                </span>
              </label>

              <button
                type="button"
                className="dbtn dbtn-primary"
                onClick={() => void handleBrandingSave()}
                disabled={brandingLocked || brandingSaving || !profile}
                style={{ fontSize: 13, padding: "8px 16px" }}
              >
                {brandingSaving ? "Saving..." : "Save profile"}
              </button>
            </div>

            <div style={{ flex: "1 1 280px", minWidth: 260 }}>
              <div
                style={{
                  border: "1px solid var(--dborder)",
                  borderRadius: 8,
                  background: "#fff",
                  color: "#111",
                  padding: 22,
                  minHeight: 228,
                  boxShadow: "0 16px 36px -30px rgba(15, 23, 42, 0.45)",
                }}
              >
                <div style={{ display: "flex", alignItems: "center", gap: 12, marginBottom: 18 }}>
                  <div
                    style={{
                      width: 42,
                      height: 42,
                      borderRadius: 8,
                      border: "1px solid #e5e5e5",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      overflow: "hidden",
                      background: "#fafafa",
                    }}
                  >
                    {profile?.branding_logo_url ? (
                      // eslint-disable-next-line @next/next/no-img-element
                      <img src={profile.branding_logo_url} alt="" style={{ maxWidth: "100%", maxHeight: "100%", objectFit: "contain" }} />
                    ) : (
                      <ImageIcon size={18} color="#888" />
                    )}
                  </div>
                  <div style={{ fontSize: 15, fontWeight: 700, color: "#111", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {previewName}
                  </div>
                </div>
                <div style={{ border: "1px solid #e5e5e5", borderRadius: 8, padding: 18 }}>
                  <div style={{ fontSize: 20, fontWeight: 700, letterSpacing: -0.2, marginBottom: 6 }}>Connect account</div>
                  <div style={{ fontSize: 13, color: "#555", lineHeight: 1.55, marginBottom: 14 }}>
                    {previewName} wants to publish posts to your account on your behalf.
                  </div>
                  <div
                    style={{
                      width: "100%",
                      borderRadius: 8,
                      background: previewColor,
                      color: "#fff",
                      textAlign: "center",
                      fontSize: 14,
                      fontWeight: 650,
                      padding: "11px 12px",
                    }}
                  >
                    Authorize platform
                  </div>
                </div>
                {!effectiveHidePoweredBy && (
                  <div style={{ color: "#888", textAlign: "center", fontSize: 12, marginTop: 18 }}>
                    Powered by UniPost
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>

      <WhiteLabelStats configuredCount={configuredPlatforms.size} totalPlatforms={CRED_PLATFORMS.length} />

      {credError && (
        <div style={{ padding: "10px 14px", borderRadius: 8, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          {credError}
        </div>
      )}

      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {CRED_PLATFORMS.map((p) => {
          const configured = configuredPlatforms.has(p.id);
          const cred = creds.find((c) => c.platform === p.id);
          const form = credForms[p.id] || { clientId: "", clientSecret: "" };
          return (
            <div key={p.id} className="settings-section" style={{ marginBottom: 0 }}>
              <div className="settings-section-header">
                <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                  <span style={{ fontSize: 14, fontWeight: 600, color: "var(--dtext)" }}>{p.name}</span>
                  {configured && (
                    <span className="dbadge dbadge-green" style={{ fontSize: 10 }}>Configured</span>
                  )}
                </div>
                <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                  {p.whiteLabelDocs && (
                    <Link href={p.whiteLabelDocs} style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
                      Docs
                    </Link>
                  )}
                  <a href={p.developerPortal} target="_blank" rel="noopener noreferrer" style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 12, color: "var(--dmuted)", textDecoration: "none" }}>
                    Developer Portal <ExternalLink style={{ width: 11, height: 11 }} />
                  </a>
                  {configured && (
                    <button className="dbtn dbtn-danger" style={{ padding: "4px 10px", fontSize: 11 }} onClick={() => setRemoveTarget(p.id)}>Remove</button>
                  )}
                </div>
              </div>
              <div className="settings-section-body">
                {configured ? (
                  <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
                    Client ID: <span className="mono">{cred?.client_id}</span>
                  </div>
                ) : (
                  <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr auto", gap: 10 }}>
                    <input className="dform-input" placeholder={p.idLabel} value={form.clientId} onChange={(e) => updateCredForm(p.id, "clientId", e.target.value)} />
                    <input className="dform-input" type="password" placeholder={p.secretLabel} value={form.clientSecret} onChange={(e) => updateCredForm(p.id, "clientSecret", e.target.value)} />
                    <button className="dbtn dbtn-primary" onClick={() => handleCredSave(p.id)} disabled={credSaving === p.id || !form.clientId || !form.clientSecret} style={{ padding: "8px 16px" }}>
                      {credSaving === p.id ? "..." : "Save"}
                    </button>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      <ConfirmModal
        open={!!removeTarget}
        title="Remove Credentials"
        message="Remove these platform credentials? White-label Connect flows for this platform will fall back to UniPost credentials. Quickstart already uses UniPost credentials."
        confirmLabel="Remove"
        variant="danger"
        onConfirm={() => removeTarget && handleCredDelete(removeTarget)}
        onCancel={() => setRemoveTarget(null)}
      />
    </>
  );
}
