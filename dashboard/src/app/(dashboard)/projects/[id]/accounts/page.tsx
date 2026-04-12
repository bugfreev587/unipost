"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger,
} from "@/components/ui/dialog";
import {
  listSocialAccounts, connectSocialAccount, disconnectSocialAccount, getOAuthConnectURL, listProfiles, type SocialAccount, type Profile,
} from "@/lib/api";
import { Plus, ExternalLink, CheckCircle2, XCircle } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { ConfirmModal } from "@/components/confirm-modal";
import { QuickstartStats } from "@/components/dashboard/connection-stats";

const PLATFORMS = [
  { id: "bluesky", name: "Bluesky", type: "credentials" as const },
  { id: "linkedin", name: "LinkedIn", type: "oauth" as const },
  { id: "instagram", name: "Instagram", type: "oauth" as const },
  { id: "threads", name: "Threads", type: "oauth" as const },
  { id: "tiktok", name: "TikTok", type: "oauth" as const },
  { id: "youtube", name: "YouTube", type: "oauth" as const },
  { id: "twitter", name: "X / Twitter", type: "oauth" as const },
];

export default function AccountsPage() {
  const { id: profileId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [profileFilter, setProfileFilter] = useState<string>("all");
  const [loading, setLoading] = useState(true);
  const [connectOpen, setConnectOpen] = useState(false);
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
  const [handle, setHandle] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState("");
  const [connectProfileId, setConnectProfileId] = useState(profileId);

  const callbackStatus = searchParams.get("status");
  const callbackAccount = searchParams.get("account_name");

  const loadAccounts = useCallback(async () => {
    try {
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
      console.error("Failed to load accounts:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => { loadAccounts(); }, [loadAccounts]);

  async function handleBlueskyConnect() {
    if (!handle.trim() || !appPassword.trim()) return;
    setConnecting(true); setConnectError("");
    try {
      const token = await getToken();
      if (!token) return;
      await connectSocialAccount(token, connectProfileId, { platform: "bluesky", credentials: { handle: handle.trim(), app_password: appPassword.trim() } });
      setConnectOpen(false); setSelectedPlatform(null); setHandle(""); setAppPassword(""); loadAccounts();
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : "Failed to connect");
    } finally { setConnecting(false); }
  }

  async function handleOAuthConnect(platform: string) {
    setConnecting(true); setConnectError("");
    try {
      const token = await getToken();
      if (!token) return;
      const redirectUrl = window.location.href.split("?")[0];
      const res = await getOAuthConnectURL(token, connectProfileId, platform, redirectUrl);
      window.location.href = res.data.auth_url;
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : "Failed to start OAuth");
      setConnecting(false);
    }
  }

  async function handleDisconnect(accountId: string) {
    // Find the account's profile_id — accounts may belong to different profiles
    const account = accounts.find((a) => a.id === accountId);
    const ownerProfileId = account?.profile_id || profileId;
    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, ownerProfileId, accountId);
      loadAccounts();
    } catch (err) { console.error("Failed to disconnect:", err); }
    finally { setDisconnectTarget(null); }
  }


  return (
    <>
      {callbackStatus === "success" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 14px", borderRadius: 6, background: "var(--success-soft)", border: "1px solid color-mix(in srgb, var(--success) 24%, transparent)", fontSize: 13, color: "var(--daccent)", marginBottom: 20 }}>
          <CheckCircle2 style={{ width: 14, height: 14 }} /> Connected {callbackAccount || "account"} successfully.
        </div>
      )}
      {callbackStatus === "error" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 14px", borderRadius: 6, background: "var(--danger-soft)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", fontSize: 13, color: "var(--danger)", marginBottom: 20 }}>
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
                {PLATFORMS.map((p) => {
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
                  <td style={{ color: "var(--dmuted)", fontSize: 13 }}>
                    {profiles.find((p) => p.id === a.profile_id)?.name || "—"}
                  </td>
                  <td style={{ color: "var(--dmuted)", textTransform: "capitalize" }}>{a.platform}</td>
                  <td style={{ color: "var(--dmuted)" }}>
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
    </>
  );
}
