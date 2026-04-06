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
  listSocialAccounts, connectSocialAccount, disconnectSocialAccount, getOAuthConnectURL, type SocialAccount,
} from "@/lib/api";
import { Plus, Unplug, ExternalLink, CheckCircle2, XCircle } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { ConfirmModal } from "@/components/confirm-modal";

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
  const { id: projectId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [connectOpen, setConnectOpen] = useState(false);
  const [disconnectTarget, setDisconnectTarget] = useState<string | null>(null);
  const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
  const [handle, setHandle] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState("");

  const callbackStatus = searchParams.get("status");
  const callbackAccount = searchParams.get("account_name");

  const loadAccounts = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listSocialAccounts(token, projectId);
      setAccounts(res.data);
    } catch (err) {
      console.error("Failed to load accounts:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => { loadAccounts(); }, [loadAccounts]);

  async function handleBlueskyConnect() {
    if (!handle.trim() || !appPassword.trim()) return;
    setConnecting(true); setConnectError("");
    try {
      const token = await getToken();
      if (!token) return;
      await connectSocialAccount(token, projectId, { platform: "bluesky", credentials: { handle: handle.trim(), app_password: appPassword.trim() } });
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
      const res = await getOAuthConnectURL(token, projectId, platform, redirectUrl);
      window.location.href = res.data.auth_url;
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : "Failed to start OAuth");
      setConnecting(false);
    }
  }

  async function handleDisconnect(accountId: string) {
    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, projectId, accountId);
      loadAccounts();
    } catch (err) { console.error("Failed to disconnect:", err); }
    finally { setDisconnectTarget(null); }
  }


  return (
    <>
      {callbackStatus === "success" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 14px", borderRadius: 6, background: "#10b98110", border: "1px solid #10b98125", fontSize: 12.5, color: "var(--daccent)", marginBottom: 20 }}>
          <CheckCircle2 style={{ width: 14, height: 14 }} /> Connected {callbackAccount || "account"} successfully.
        </div>
      )}
      {callbackStatus === "error" && (
        <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "10px 14px", borderRadius: 6, background: "#ef444410", border: "1px solid #ef444425", fontSize: 12.5, color: "var(--danger)", marginBottom: 20 }}>
          <XCircle style={{ width: 14, height: 14 }} /> Failed to connect. Please try again.
        </div>
      )}

      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Quickstart Mode</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6, maxWidth: 520, lineHeight: 1.6 }}>Connect social accounts instantly — no developer approvals or platform credentials needed. UniPost handles OAuth so you can start posting in minutes.</div>
        </div>
        <Dialog open={connectOpen} onOpenChange={(open) => { setConnectOpen(open); if (!open) { setSelectedPlatform(null); setConnectError(""); } }}>
          <DialogTrigger render={<button className="dbtn dbtn-primary" />}>
            <Plus style={{ width: 13, height: 13 }} /> Connect Account
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{selectedPlatform ? `Connect ${PLATFORMS.find((p) => p.id === selectedPlatform)?.name}` : "Connect Account"}</DialogTitle>
              <DialogDescription>{selectedPlatform === "bluesky" ? "Enter your handle and App Password." : selectedPlatform ? "Redirecting to authorize." : "Choose a platform."}</DialogDescription>
            </DialogHeader>
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
                        <span style={{ fontSize: 13, fontWeight: 500, color: "var(--dtext)" }}>{p.name}</span>
                      </div>
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        {connectedCount > 0 && (
                          <span style={{ fontSize: 11, color: "var(--dmuted)", fontFamily: "var(--font-geist-mono), monospace" }}>
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
                <div style={{ fontSize: 11, color: "var(--dmuted2)" }}>Generate at bsky.app → Settings → App Passwords</div>
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

      {loading ? (
        <div style={{ color: "var(--dmuted)" }}>Loading...</div>
      ) : accounts.length === 0 ? (
        <div className="empty-state">
          <Unplug style={{ width: 32, height: 32, opacity: 0.4, marginBottom: 12 }} />
          <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 6 }}>No accounts connected</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", maxWidth: 280, lineHeight: 1.6 }}>Connect a social account to start posting.</div>
          <button className="dbtn dbtn-primary" onClick={() => setConnectOpen(true)} style={{ marginTop: 16 }}>
            <Plus style={{ width: 13, height: 13 }} /> Connect Account
          </button>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead><tr><th>Account</th><th>Platform</th><th>Connected</th><th>Status</th><th></th></tr></thead>
            <tbody>
              {accounts.map((a) => (
                <tr key={a.id}>
                  <td>
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <div className="platform-icon-wrap"><PlatformIcon platform={a.platform} /></div>
                      <span style={{ fontWeight: 500 }}>{a.account_name || a.id}</span>
                    </div>
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
                    <button className="dbtn dbtn-danger" style={{ padding: "4px 10px", fontSize: 11.5 }} onClick={() => setDisconnectTarget(a.id)}>
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
