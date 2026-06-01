"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams, useSearchParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";
import {
  listSocialAccounts, connectSocialAccount, disconnectSocialAccount, getOAuthConnectURL, listProfiles, getActivation, getMe,
  type SocialAccount, type Profile,
} from "@/lib/api";
import { isFacebookEnabledForMe } from "@/components/dashboard/shell";
import { FacebookPagePicker } from "@/components/accounts/facebook-page-picker";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import { Plus, ExternalLink, CheckCircle2, XCircle } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { ConfirmModal } from "@/components/confirm-modal";
import { ConnectionStats } from "@/components/dashboard/connection-stats";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";
import { humanizeConnectError } from "@/lib/connect-errors";
import { clearStoredReplay, readStoredReplay, writeStoredReplay } from "@/components/tutorials/replay-storage";
import { writeStoredQuickstartSelectedAccountId } from "@/components/tutorials/quickstart-selection-storage";

// BASE_PLATFORMS is the always-available set. Feature-flagged platforms
// (currently just Facebook during audit) are appended at render time
// via the isFacebookEnabled() check so a flag-off deploy doesn't show a
// broken entry.
const BASE_PLATFORMS = [
  { id: "bluesky", name: "Bluesky", type: "credentials" as const },
  { id: "linkedin", name: "LinkedIn", type: "oauth" as const },
  { id: "instagram", name: "Instagram", type: "oauth" as const },
  { id: "threads", name: "Threads", type: "oauth" as const },
  { id: "pinterest", name: "Pinterest", type: "oauth" as const },
  { id: "tiktok", name: "TikTok", type: "oauth" as const },
  { id: "youtube", name: "YouTube", type: "oauth" as const },
  { id: "twitter", name: "X / Twitter", type: "oauth" as const },
];

const FACEBOOK_PLATFORM = { id: "facebook", name: "Facebook Page", type: "oauth" as const };

// Platforms that work with text-only or simple image posts. These are the
// most reliable choices for a first-time Connect during activation — video
// platforms (TikTok, YouTube) are slower and more failure-prone, so we
// filter them out by default when the user arrives via the activation
// modal (?first=1). A "Show all platforms" link escape-hatches.
const FIRST_TIME_PLATFORM_IDS = new Set(["bluesky", "linkedin", "instagram", "threads", "twitter", "pinterest"]);

function accountSourceLabel(platform: string) {
  if (platform === "youtube") return "YouTube channel";
  if (platform === "facebook") return "Facebook Page";
  if (platform === "twitter") return "X / Twitter account";
  return `${platform.charAt(0).toUpperCase()}${platform.slice(1)} account`;
}

export default function AccountsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  // isSuperAdmin comes from /v1/me and authoritatively gates
  // in-development features. Starts
  // undefined so we don't flash the Facebook button before the
  // /me round-trip resolves — the splice below only runs when the
  // value lands true.
  const [isSuperAdmin, setIsSuperAdmin] = useState<boolean>(false);

  const PLATFORMS = (() => {
    const list = [...BASE_PLATFORMS];
    if (isFacebookEnabledForMe(isSuperAdmin)) list.splice(3, 0, FACEBOOK_PLATFORM);
    return list;
  })();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [profileFilter, setProfileFilter] = useState<string>("all");
  const [loading, setLoading] = useState(true);
  // Auto-open the connect flow when arriving from activation modal
  // (?action=new). See activation-modal.tsx STEP_META.connect_account.
  const [connectOpen, setConnectOpen] = useState(searchParams.get("action") === "new");
  // First-time activation mode: filter out video-heavy platforms to keep
  // the first connect fast and low-friction. Toggleable via "Show all".
  const [firstTimeMode, setFirstTimeMode] = useState(searchParams.get("first") === "1");
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
  const [handle, setHandle] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState("");
  const [connectProfileId, setConnectProfileId] = useState(profileId);
  const [accountsError, setAccountsError] = useState<{ message: string; topic: string } | null>(null);

  const router = useRouter();
  const workspaceId = useWorkspaceId();
  const callbackStatus = searchParams.get("status");
  const callbackAccount = searchParams.get("account_name");
  const callbackError = humanizeConnectError(searchParams.get("error") || searchParams.get("reason"));
  // Facebook detour lands here with ?pending=<id>; mount the picker
  // when present, clear the URL param on close so a refresh doesn't
  // re-open an already-finalized pending row.
  const pendingFacebookId = searchParams.get("pending");

  const closePicker = useCallback(() => {
    // Strip the `pending` param from the URL without a full nav so the
    // success banner + account list don't re-render from scratch.
    const url = new URL(window.location.href);
    url.searchParams.delete("pending");
    router.replace(url.pathname + (url.search ? url.search : ""));
  }, [router]);

  const clearTutorialParams = useCallback(() => {
    const url = new URL(window.location.href);
    url.searchParams.delete("action");
    url.searchParams.delete("first");
    url.searchParams.delete("template");
    router.replace(url.pathname + (url.search ? url.search : ""));
  }, [router]);

  useEffect(() => {
    if (callbackStatus !== "success") return;
    if (loading) return;

    const activeProfileAccounts = accounts
      .filter((account) => account.profile_id === profileId && account.status === "active")
      .sort((a, b) => Date.parse(b.connected_at || "") - Date.parse(a.connected_at || ""));

    const matchedByName = callbackAccount
      ? activeProfileAccounts.find((account) => account.account_name?.trim() === callbackAccount.trim())
      : undefined;
    const selected = matchedByName || activeProfileAccounts[0];
    if (!selected) return;

    const stored = readStoredReplay();
    if (stored) {
      if (stored.selectedAccountId !== selected.id) {
        writeStoredReplay({ ...stored, selectedAccountId: selected.id });
      }
      router.replace(`/projects/${profileId}`);
      return;
    }

    if (!firstTimeMode) return;

    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getActivation(token);
        if (cancelled) return;
        if (!res.data.completed && !res.data.dismissed) {
          writeStoredQuickstartSelectedAccountId(selected.id);
          router.replace(`/projects/${profileId}`);
        }
      } catch {
        /* silent — stay on accounts page */
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [accounts, callbackAccount, callbackStatus, firstTimeMode, getToken, loading, profileId, router]);

  const loadAccounts = useCallback(async () => {
    try {
      setAccountsError(null);
      const token = await getToken();
      if (!token) return;
      const profRes = await listProfiles(token);
      setProfiles(profRes.data);
      // Load accounts from all profiles
      const allAccounts = await Promise.all(
        profRes.data.map((p) => listSocialAccounts(token, p.id))
      );
      setAccounts(allAccounts.flatMap((r) => r.data));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load accounts";
      console.error("Failed to load accounts:", err);
      setAccountsError({ message, topic: "accounts-load-failure" });
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => { loadAccounts(); }, [loadAccounts]);

  // Resolve super-admin status once per mount. If the /me call fails
  // we fall back to "not super admin" — this page is the only
  // Facebook Pages entry point, so a failed fetch simply hides the
  // in-development button until the user refreshes.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getMe(token);
        if (!cancelled) setIsSuperAdmin(!!res.data.is_super_admin);
      } catch { /* silent — leave the FB button hidden */ }
    })();
    return () => { cancelled = true; };
  }, [getToken]);

  const byoAccounts = useMemo(
    () => accounts.filter((a) => a.connection_type === "byo"),
    [accounts]
  );

  const visibleAccounts = useMemo(
    () =>
      (profileFilter === "all"
        ? byoAccounts
        : byoAccounts.filter((a) => a.profile_id === profileFilter)),
    [byoAccounts, profileFilter]
  );

  async function handleBlueskyConnect() {
    if (!handle.trim() || !appPassword.trim()) return;
    setConnecting(true); setConnectError(""); setAccountsError(null);
    try {
      const token = await getToken();
      if (!token) return;
      await connectSocialAccount(token, connectProfileId, { platform: "bluesky", credentials: { handle: handle.trim(), app_password: appPassword.trim() } });
      setConnectOpen(false); setSelectedPlatform(null); setHandle(""); setAppPassword(""); loadAccounts();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to connect";
      setConnectError(message);
      setAccountsError({ message, topic: "account-connect-failure" });
    } finally { setConnecting(false); }
  }

  async function handleOAuthConnect(platform: string) {
    setConnecting(true); setConnectError(""); setAccountsError(null);
    try {
      const token = await getToken();
      if (!token) return;
      const redirectUrl = window.location.href.split("?")[0];
      const res = await getOAuthConnectURL(token, connectProfileId, platform, redirectUrl);
      window.location.href = res.data.auth_url;
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to start OAuth";
      setConnectError(message);
      setAccountsError({ message, topic: "account-oauth-failure" });
      setConnecting(false);
    }
  }

  async function handleDisconnect(accountId: string) {
    // Find the account's profile_id — accounts may belong to different profiles
    const account = accounts.find((a) => a.id === accountId);
    const ownerProfileId = account?.profile_id || profileId;
    try {
      setAccountsError(null);
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, ownerProfileId, accountId);
      loadAccounts();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to disconnect account";
      console.error("Failed to disconnect:", err);
      setAccountsError({ message, topic: "account-disconnect-failure" });
    }
    finally { setDisconnectTarget(null); }
  }

  return (
    <>
      {accountsError && (
        <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16, padding: "10px 14px", borderRadius: 6, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
          <div>
            <div style={{ fontWeight: 600, marginBottom: 4 }}>Account action failed</div>
            <div>{accountsError.message}</div>
          </div>
          <div style={{ display: "flex", gap: 8, flexShrink: 0 }}>
            <a
              href={buildSupportMailto({
                subject: "Connection action failed in dashboard",
                intro: "I ran into a connection-related failure in the dashboard.",
                details: [
                  `Profile ID: ${profileId}`,
                  `Topic: ${accountsError.topic}`,
                  `Error: ${accountsError.message}`,
                ],
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Contact support
            </a>
            <a
              href={buildContactPageHref({
                topic: accountsError.topic,
                source: "accounts-page",
                profile: profileId,
                error: accountsError.message,
              })}
              className="dbtn dbtn-ghost"
              style={{ fontSize: 12 }}
            >
              Open help center
            </a>
          </div>
        </div>
      )}

      {callbackStatus === "success" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "12px 14px", borderRadius: 8, background: "color-mix(in srgb, var(--success-soft) 82%, white)", border: "1px solid color-mix(in srgb, var(--success) 26%, transparent)", fontSize: 13, fontWeight: 500, color: "color-mix(in srgb, var(--success) 86%, var(--dtext))", marginBottom: 20 }}>
          <CheckCircle2 style={{ width: 14, height: 14 }} /> Connected {callbackAccount || "account"} successfully.
        </div>
      )}
      {callbackStatus === "error" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "12px 14px", borderRadius: 8, background: "color-mix(in srgb, var(--danger-soft) 82%, white)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, fontWeight: 500, color: "color-mix(in srgb, var(--danger) 86%, var(--dtext))", marginBottom: 20 }}>
          <XCircle style={{ width: 14, height: 14 }} /> {callbackError}
        </div>
      )}

      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div className="dt-page-title">Connections</div>
          <div className="dt-subtitle" style={{ maxWidth: 620, lineHeight: 1.6 }}>
            Connect your own publishing accounts from the dashboard. UniPost can use shared OAuth apps by default,
            or your workspace platform credentials when you configure them.
          </div>
          <div
            style={{
              maxWidth: 720,
              marginTop: 14,
              padding: "10px 12px",
              borderRadius: 8,
              background: "color-mix(in srgb, var(--success-soft) 58%, var(--surface1))",
              border: "1px solid color-mix(in srgb, var(--success) 18%, var(--dborder))",
              color: "color-mix(in srgb, var(--success) 78%, var(--dtext))",
              fontSize: 12,
              lineHeight: 1.55,
            }}
          >
            Quickstart connection and health fields are managed by UniPost. The Source platform column identifies the external social account,
            for example a YouTube channel connected through UniPost-managed OAuth.
          </div>
          {profiles.length > 1 && (
            <div style={{ display: "flex", gap: 6, marginTop: 14, flexWrap: "wrap" }}>
              <button
                className={`dbtn ${profileFilter === "all" ? "dbtn-primary" : "dbtn-ghost"}`}
                style={{ padding: "4px 12px", fontSize: 12 }}
                onClick={() => setProfileFilter("all")}
              >
                All
              </button>
              {profiles.map((p) => (
                <button
                  key={p.id}
                  className={`dbtn ${profileFilter === p.id ? "dbtn-primary" : "dbtn-ghost"}`}
                  style={{ padding: "4px 12px", fontSize: 12 }}
                  onClick={() => setProfileFilter(p.id)}
                >
                  {p.name}
                </button>
              ))}
            </div>
          )}
        </div>
        <Dialog
          open={connectOpen}
          onOpenChange={(open) => {
            setConnectOpen(open);
            if (!open) {
              setSelectedPlatform(null);
              setConnectError("");
              if (searchParams.get("action") === "new") {
                clearStoredReplay();
                clearTutorialParams();
              }
            } else {
              setConnectProfileId(profileId);
            }
          }}
        >
          <DialogTrigger render={<button className="dbtn dbtn-primary" />}>
            <Plus style={{ width: 13, height: 13 }} /> Connect Account
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{selectedPlatform ? `Connect ${PLATFORMS.find((p) => p.id === selectedPlatform)?.name}` : "Connect Account"}</DialogTitle>
              <DialogDescription>{selectedPlatform === "bluesky" ? "Enter your handle and App Password." : selectedPlatform ? "Redirecting to authorize." : "Choose a platform."}</DialogDescription>
            </DialogHeader>

            {!selectedPlatform && (
              <div
                style={{
                  padding: "10px 12px",
                  borderRadius: 8,
                  background: "color-mix(in srgb, var(--surface2) 85%, white)",
                  border: "1px solid var(--dborder)",
                  fontSize: 12,
                  lineHeight: 1.55,
                  color: "var(--dmuted)",
                }}
              >
                Dashboard connections use UniPost-managed OAuth unless your workspace has platform credentials for that platform.
                For customer-owned account onboarding, use Developer → Hosted Connect.
              </div>
            )}

            {/* Profile selector */}
            {profiles.length > 1 && !selectedPlatform && (
              <div style={{ padding: "0 0 4px", borderBottom: "1px solid var(--dborder)", marginBottom: 4 }}>
                <label className="dt-label" style={{ color: "var(--dmuted2)", display: "block", marginBottom: 6 }}>
                  Add to profile
                </label>
                <select
                  value={connectProfileId}
                  onChange={(e) => setConnectProfileId(e.target.value)}
                  style={{
                    width: "100%", padding: "7px 10px", fontSize: 13,
                    background: "var(--surface1)", border: "1px solid var(--dborder)",
                    borderRadius: 6, color: "var(--dtext)", outline: "none",
                    fontFamily: "inherit", cursor: "pointer",
                  }}
                >
                  {profiles.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}{p.id === profileId ? " (current)" : ""}
                    </option>
                  ))}
                </select>
              </div>
            )}

            {!selectedPlatform ? (
              <div style={{ padding: "8px 0" }}>
                {PLATFORMS
                  .filter((p) => !firstTimeMode || FIRST_TIME_PLATFORM_IDS.has(p.id))
                  .map((p) => {
                  const connectedCount = accounts.filter((a) => a.platform === p.id).length;
                  return (
                    <div
                      key={p.id}
                      onClick={() => { if (p.type === "oauth") handleOAuthConnect(p.id); else setSelectedPlatform(p.id); }}
                      style={{ display: "flex", alignItems: "center", justifyContent: "space-between", padding: "8px 10px", borderRadius: 6, cursor: connecting ? "not-allowed" : "pointer", opacity: connecting ? 0.5 : 1, marginBottom: 2, transition: "background 0.1s" }}
                      onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; }}
                      onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                    >
                      <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                        <div className="platform-icon-wrap"><PlatformIcon platform={p.id} /></div>
                        <span className="dt-body-sm" style={{ fontWeight: 500, color: "var(--dtext)" }}>{p.name}</span>
                      </div>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        {connectedCount > 0 && (
                          <span className="dt-micro" style={{ fontFamily: "var(--font-geist-mono), monospace" }}>
                            {connectedCount} connected
                          </span>
                        )}
                        <ExternalLink style={{ width: 12, height: 12, color: "var(--dmuted2)" }} />
                      </div>
                    </div>
                  );
                })}
                {firstTimeMode && (
                  <button
                    type="button"
                    onClick={() => setFirstTimeMode(false)}
                    className="dt-body-sm"
                    style={{
                      display: "block", width: "100%",
                      marginTop: 8, padding: "8px 10px", borderRadius: 6,
                      border: "none", background: "transparent",
                      color: "var(--dmuted)", cursor: "pointer",
                      fontFamily: "inherit", textAlign: "center",
                    }}
                    onMouseEnter={(e) => { e.currentTarget.style.color = "var(--dtext)"; }}
                    onMouseLeave={(e) => { e.currentTarget.style.color = "var(--dmuted)"; }}
                  >
                    Show all platforms (including video)
                  </button>
                )}
              </div>
            ) : selectedPlatform === "bluesky" ? (
              <div style={{ padding: "8px 0" }}>
                <div style={{ marginBottom: 14 }}>
                  <Label className="dform-label">Handle</Label>
                  <Input placeholder="alice.bsky.social" value={handle} onChange={(e) => setHandle(e.target.value)} className="dform-input" />
                </div>
                <div style={{ marginBottom: 14 }}>
                  <Label className="dform-label">App Password</Label>
                  <Input type="password" placeholder="xxxx-xxxx-xxxx-xxxx" value={appPassword} onChange={(e) => setAppPassword(e.target.value)} className="dform-input" />
                </div>
                <div className="dt-micro" style={{ color: "var(--dmuted2)" }}>Generate at bsky.app → Settings → App Passwords</div>
                {connectError && <div style={{ fontSize: 12, color: "var(--danger)", marginTop: 8 }}>{connectError}</div>}
              </div>
            ) : null}
            {connectError && !selectedPlatform && <div style={{ fontSize: 12, color: "var(--danger)", padding: "0 4px" }}>{connectError}</div>}
            {selectedPlatform === "bluesky" && (
              <DialogFooter>
                <button className="dbtn dbtn-ghost" onClick={() => setSelectedPlatform(null)}>Back</button>
                <button className="dbtn dbtn-primary" onClick={handleBlueskyConnect} disabled={connecting || !handle.trim() || !appPassword.trim()}>
                  {connecting ? "Connecting..." : "Connect"}
                </button>
              </DialogFooter>
            )}
          </DialogContent>
        </Dialog>
      </div>

      {!loading && visibleAccounts.length > 0 && (
        <ConnectionStats
          accounts={visibleAccounts}
          profiles={profileFilter === "all" ? profiles : profiles.filter((p) => p.id === profileFilter)}
        />
      )}

      {loading ? (
        <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Account</th><th>UniPost Profile</th><th>Source platform</th><th>Connected</th><th>UniPost status</th><th></th></tr></thead>
            <tbody>
              {visibleAccounts.map((a) => (
                <tr key={a.id}>
                  <td>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <div className="platform-icon-wrap"><PlatformIcon platform={a.platform} /></div>
                      <div>
                        <div style={{ fontWeight: 500 }}>{a.account_name || a.id}</div>
                        <div className="dt-micro" style={{ color: "var(--dmuted2)", marginTop: 2, textTransform: "none", letterSpacing: 0 }}>
                          {accountSourceLabel(a.platform)}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td style={{ color: "var(--dmuted)", fontSize: 13, fontWeight: 500 }}>
                    {profiles.find((p) => p.id === a.profile_id)?.name || "—"}
                  </td>
                  <td style={{ color: "var(--dmuted)", fontWeight: 500 }}>{accountSourceLabel(a.platform)}</td>
                  <td style={{ color: "var(--dmuted)", fontWeight: 500 }}>
                    {new Date(a.connected_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}
                  </td>
                  <td>
                    <span className={`dbadge ${
                      a.status === "active"
                        ? "dbadge-green"
                        : a.status === "reconnect_required"
                          ? "dbadge-amber"
                          : "dbadge-red"
                    }`}>
                      <span className="dbadge-dot" />
                      {a.status === "active" ? "Active" : a.status === "reconnect_required" ? "Reconnect" : "Disconnected"}
                    </span>
                  </td>
                  <td style={{ textAlign: "right" }}>
                    <button className="dbtn dbtn-danger" style={{ padding: "4px 10px", fontSize: 12 }} onClick={() => setDisconnectTarget(a.id)}>
                      Disconnect
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <ConfirmModal
        open={!!disconnectTarget}
        title="Disconnect Account"
        message="Are you sure you want to disconnect this social account? You can reconnect it later."
        confirmLabel="Disconnect"
        variant="danger"
        onConfirm={() => disconnectTarget && handleDisconnect(disconnectTarget)}
        onCancel={() => setDisconnectTarget(null)}
      />

      {pendingFacebookId && workspaceId && (
        <FacebookPagePicker
          open
          pendingId={pendingFacebookId}
          workspaceId={workspaceId}
          getToken={getToken}
          onClose={closePicker}
          onFinalized={(count) => {
            // Atomic URL transition: strip `pending` AND add the
            // success banner params in one router.replace so there's
            // no intermediate render where the picker is still
            // mounted but its backing row has been deleted (that
            // race produced a "Pending connection not found" flash
            // on the modal before unmount).
            const url = new URL(window.location.href);
            url.searchParams.delete("pending");
            if (count > 0) {
              url.searchParams.set("status", "success");
              url.searchParams.set("account_name", `${count} Facebook Page${count === 1 ? "" : "s"}`);
            }
            router.replace(url.pathname + url.search);
            loadAccounts();
            setAccountsError(null);
          }}
        />
      )}
    </>
  );
}
