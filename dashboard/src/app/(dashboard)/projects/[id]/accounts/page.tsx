"use client";

import { useCallback, useEffect, useState } from "react";
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
import { QuickstartStats } from "@/components/dashboard/connection-stats";
import { buildContactPageHref, buildSupportMailto } from "@/lib/support";

// BASE_PLATFORMS is the always-available set. Feature-flagged platforms
// (currently just Facebook during audit) are appended at render time
// via the isFacebookEnabled() check so a flag-off deploy doesn't show a
// broken entry.
const BASE_PLATFORMS = [
  { id: "bluesky", name: "Bluesky", type: "credentials" as const },
  { id: "linkedin", name: "LinkedIn", type: "oauth" as const },
  { id: "instagram", name: "Instagram", type: "oauth" as const },
  { id: "threads", name: "Threads", type: "oauth" as const },
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
const FIRST_TIME_PLATFORM_IDS = new Set(["bluesky", "linkedin", "instagram", "threads", "twitter"]);

export default function AccountsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  // isSuperAdmin comes from /v1/me and authoritatively gates the
  // Facebook Pages entry during in-development lockdown. Starts
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

  // Activation flow: if this success is the user's FIRST-ever connection
  // (activation modal was the origin), bounce them back to the dashboard
  // so the Welcome modal re-pops with step 1 checked and step 2 ready.
  // The check queries the activation API — if completed or dismissed we
  // skip the redirect and let the user stay on the accounts page.
  useEffect(() => {
    if (callbackStatus !== "success") return;
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token || cancelled) return;
        const res = await getActivation(token);
        if (cancelled) return;
        // Only redirect when the activation guide is still active
        // (not completed and not dismissed). This covers the first-time
        // connect case and avoids yanking power users off the accounts
        // page after reconnecting a second/third account.
        if (!res.data.completed && !res.data.dismissed) {
          router.replace(`/projects/${profileId}`);
        }
      } catch { /* silent — stay on accounts page */ }
    })();
    return () => { cancelled = true; };
  }, [callbackStatus, getToken, profileId, router]);

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
          <XCircle style={{ width: 14, height: 14 }} /> Failed to connect. Please try again.
        </div>
      )}

      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div className="dt-page-title">Quickstart Mode</div>
          <div className="dt-subtitle" style={{ maxWidth: 520, lineHeight: 1.6 }}>Connect social accounts instantly — no developer approvals or platform credentials needed. UniPost handles OAuth so you can start posting in minutes.</div>
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
        <Dialog open={connectOpen} onOpenChange={(open) => { setConnectOpen(open); if (!open) { setSelectedPlatform(null); setConnectError(""); } else { setConnectProfileId(profileId); } }}>
          <DialogTrigger render={<button className="dbtn dbtn-primary" />}>
            <Plus style={{ width: 13, height: 13 }} /> Connect Account
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{selectedPlatform ? `Connect ${PLATFORMS.find((p) => p.id === selectedPlatform)?.name}` : "Connect Account"}</DialogTitle>
              <DialogDescription>{selectedPlatform === "bluesky" ? "Enter your handle and App Password." : selectedPlatform ? "Redirecting to authorize." : "Choose a platform."}</DialogDescription>
            </DialogHeader>

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

      {!loading && accounts.length > 0 && (
        <QuickstartStats accounts={accounts.filter((a) => a.connection_type === "byo")} profiles={profiles} />
      )}

      {loading ? (
        <div style={{ color: "var(--dmuted)", fontSize: 14, lineHeight: "20px" }}>Loading...</div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Account</th><th>Profile</th><th>Platform</th><th>Connected</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {(profileFilter === "all" ? accounts : accounts.filter((a) => a.profile_id === profileFilter)).filter((a) => a.connection_type === "byo").map((a) => (
                <tr key={a.id}>
                  <td>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <div className="platform-icon-wrap"><PlatformIcon platform={a.platform} /></div>
                      <span style={{ fontWeight: 500 }}>{a.account_name || a.id}</span>
                    </div>
                  </td>
                  <td style={{ color: "var(--dmuted)", fontSize: 13, fontWeight: 500 }}>
                    {profiles.find((p) => p.id === a.profile_id)?.name || "—"}
                  </td>
                  <td style={{ color: "var(--dmuted)", textTransform: "capitalize", fontWeight: 500 }}>{a.platform}</td>
                  <td style={{ color: "var(--dmuted)", fontWeight: 500 }}>
                    {new Date(a.connected_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}
                  </td>
                  <td>
                    <span className={`dbadge ${a.status === "active" ? "dbadge-green" : "dbadge-amber"}`}>
                      <span className="dbadge-dot" />
                      {a.status === "active" ? "Active" : "Reconnect"}
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
          getToken={async () => (await getToken()) ?? null}
          onClose={closePicker}
          onFinalized={(count) => {
            closePicker();
            loadAccounts();
            setAccountsError(null);
            if (count > 0) {
              // Reuse the success banner that the non-FB OAuth flow
              // already uses; replaceState avoids an extra fetch.
              const url = new URL(window.location.href);
              url.searchParams.set("status", "success");
              url.searchParams.set("account_name", `${count} Facebook Page${count === 1 ? "" : "s"}`);
              router.replace(url.pathname + url.search);
            }
          }}
        />
      )}
    </>
  );
}
