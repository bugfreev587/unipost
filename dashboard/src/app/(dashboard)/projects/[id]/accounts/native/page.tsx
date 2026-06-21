"use client";

import { useCallback, useEffect, useState, type ChangeEvent } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { ImageIcon, Lock, Trash2, Upload } from "lucide-react";
import {
  getProfile,
  updateProfile,
  updateWorkspace,
  uploadProfileLogo,
  deleteProfileLogo,
  getApiLimits,
  type Profile,
} from "@/lib/api";

const LOGO_MAX_BYTES = 2 * 1024 * 1024;
const HEX_COLOR_RE = /^#[0-9a-fA-F]{6}$/;
const CUSTOM_PLATFORM_OPTIONS = [
  { id: "twitter", name: "X / Twitter" },
  { id: "linkedin", name: "LinkedIn" },
  { id: "bluesky", name: "Bluesky" },
  { id: "youtube", name: "YouTube" },
  { id: "tiktok", name: "TikTok" },
  { id: "instagram", name: "Instagram" },
  { id: "threads", name: "Threads" },
  { id: "facebook", name: "Facebook Page" },
  { id: "pinterest", name: "Pinterest" },
];

export default function NativeModePage() {
  const { id: profileId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [profile, setProfile] = useState<Profile | null>(null);
  const [loading, setLoading] = useState(true);
  const [pageError, setPageError] = useState("");
  const [brandingDisplayName, setBrandingDisplayName] = useState("");
  const [brandingPrimaryColor, setBrandingPrimaryColor] = useState("#111111");
  const [brandingHidePoweredBy, setBrandingHidePoweredBy] = useState(false);
  const [brandingSaving, setBrandingSaving] = useState(false);
  const [logoUploading, setLogoUploading] = useState(false);
  const [brandingError, setBrandingError] = useState("");
  const [brandingMessage, setBrandingMessage] = useState("");
  const [planAllowsHostedConnectBranding, setPlanAllowsHostedConnectBranding] = useState<boolean | null>(null);
  const [planAllowsHidePoweredBy, setPlanAllowsHidePoweredBy] = useState<boolean | null>(null);
  const [whiteLabelPlatformLimit, setWhiteLabelPlatformLimit] = useState<number | null>(null);
  const [customPlatformSlot, setCustomPlatformSlot] = useState("");

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
      const [profileRes, limitsRes] = await Promise.all([
        getProfile(token, profileId),
        getApiLimits(token).catch(() => null),
      ]);
      applyProfile(profileRes.data);
      if (limitsRes) {
        setPlanAllowsHostedConnectBranding(limitsRes.data.plan_allows_hosted_connect_branding);
        setPlanAllowsHidePoweredBy(limitsRes.data.plan_allows_hide_powered_by);
        setWhiteLabelPlatformLimit(limitsRes.data.white_label_platform_limit);
        setCustomPlatformSlot(limitsRes.data.custom_platform_slot || "");
      }
    } catch (e) {
      setPageError((e as Error).message || "Failed to load Hosted Connect settings");
    }
    finally { setLoading(false); }
  }, [applyProfile, getToken, profileId]);

  useEffect(() => { loadPage(); }, [loadPage]);

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
    if (whiteLabelPlatformLimit === 1 && !customPlatformSlot) {
      setBrandingError("Choose the platform this Basic plan should customize.");
      return;
    }

    setBrandingSaving(true);
    setBrandingError("");
    setBrandingMessage("");
    try {
      const token = await getToken();
      if (!token) return;
      if (whiteLabelPlatformLimit === 1) {
        const workspaceRes = await updateWorkspace(token, { custom_platform_slot: customPlatformSlot });
        setCustomPlatformSlot(workspaceRes.data.custom_platform_slot || "");
      }
      const res = await updateProfile(token, profile.id, {
        branding_display_name: displayName,
        branding_primary_color: primaryColor,
        branding_hide_powered_by: planAllowsHidePoweredBy === true ? brandingHidePoweredBy : false,
      });
      applyProfile(res.data);
      setBrandingMessage(whiteLabelPlatformLimit === 1 ? "Profile branding and platform slot saved." : "Profile branding saved.");
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

  const brandingLocked = planAllowsHostedConnectBranding === false;
  const attributionLocked = planAllowsHidePoweredBy === false;
  const requiresCustomPlatformSlot = whiteLabelPlatformLimit === 1;
  const customSlotMissing = requiresCustomPlatformSlot && !customPlatformSlot;
  const previewName = brandingDisplayName.trim() || profile?.name || "Your product";
  const previewColor = HEX_COLOR_RE.test(brandingPrimaryColor.trim()) ? brandingPrimaryColor.trim() : "#111111";
  const effectiveHidePoweredBy = planAllowsHidePoweredBy === true && brandingHidePoweredBy;
  const brandingReady = Boolean(profile?.branding_logo_url) && Boolean(brandingDisplayName.trim()) && HEX_COLOR_RE.test(brandingPrimaryColor.trim()) && !customSlotMissing;
  const brandingBadgeClass = brandingLocked ? "dbadge-gray" : brandingReady ? "dbadge-green" : "dbadge-amber";
  const brandingBadgeLabel = brandingLocked ? "Locked" : brandingReady ? "Ready" : "Needs setup";

  if (loading) return <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Hosted Connect</div>
          <div style={{ fontSize: 14, color: "var(--dmuted)", marginTop: 6 }}>
            Configure the branded profile your customers see before they continue to each platform&apos;s OAuth screen.
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
          Hosted Connect is the customer-facing connection flow
        </div>
        <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
          Use this page for the logo, name, color, and attribution on the hosted connection page. OAuth app credentials and quota ownership live in{" "}
          <Link href={`/projects/${profileId}/credentials`} style={{ color: "var(--daccent)", textDecoration: "none" }}>
            Platform Credentials
          </Link>
          . Create sessions through the{" "}
          <Link href="/docs/api/connect/sessions/create" style={{ color: "var(--daccent)", textDecoration: "none" }}>
            Connect Sessions
          </Link>{" "}
          API when your app needs end users to connect their own accounts.
        </div>
      </div>

      {brandingLocked && (
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
              Hosted Connect profile starts on Basic
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.55 }}>
              Your current plan can still use UniPost&apos;s shared OAuth apps. Upgrade to Basic ($19/mo) to brand the Hosted Connect profile,
              or Growth ($59/mo) / Team ($149/mo) to remove attribution.
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
              <div>Hosted Connect profile branding starts on Basic. OAuth credentials and quota settings are managed separately in Platform Credentials.</div>
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

              {whiteLabelPlatformLimit !== null && (
                <div style={{ marginBottom: 16 }}>
                  <label>
                    <span className="dform-label">Custom platform scope</span>
                    {whiteLabelPlatformLimit === 1 ? (
                      <select
                        className="dform-input"
                        value={customPlatformSlot}
                        onChange={(e) => setCustomPlatformSlot(e.target.value)}
                        disabled={brandingLocked}
                        style={{ height: 46 }}
                      >
                        <option value="">Choose one platform</option>
                        {CUSTOM_PLATFORM_OPTIONS.map((platform) => (
                          <option key={platform.id} value={platform.id}>
                            {platform.name}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <div
                        className="dform-input"
                        style={{
                          minHeight: 46,
                          display: "flex",
                          alignItems: "center",
                          color: "var(--dmuted)",
                        }}
                      >
                        {whiteLabelPlatformLimit === -1 ? "All supported platforms" : "Available on Basic or higher"}
                      </div>
                    )}
                  </label>
                  <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5, marginTop: 7 }}>
                    {whiteLabelPlatformLimit === 1
                      ? "Basic uses one shared platform slot for Hosted Connect branding and Platform Credentials."
                      : whiteLabelPlatformLimit === -1
                        ? "Growth, Team, and Enterprise apply Hosted Connect branding across all supported platforms."
                        : "Hosted Connect branding and Platform Credentials start on Basic."}
                  </div>
                </div>
              )}

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
                    {attributionLocked ? "Available on Growth, Team, and Enterprise." : "Applies to the hosted Connect page footer."}
                  </span>
                </span>
              </label>

              <button
                type="button"
                className="dbtn dbtn-primary"
                onClick={() => void handleBrandingSave()}
                disabled={brandingLocked || brandingSaving || !profile || customSlotMissing}
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

    </>
  );
}
