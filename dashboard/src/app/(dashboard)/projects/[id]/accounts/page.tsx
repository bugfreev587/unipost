"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  listSocialAccounts,
  connectSocialAccount,
  disconnectSocialAccount,
  getOAuthConnectURL,
  type SocialAccount,
} from "@/lib/api";
import { Plus, Unplug, ExternalLink, CheckCircle2, XCircle } from "lucide-react";

const PLATFORMS = [
  { id: "bluesky", name: "Bluesky", type: "credentials" as const, color: "#0085ff" },
  { id: "linkedin", name: "LinkedIn", type: "oauth" as const, color: "#0a66c2" },
  { id: "instagram", name: "Instagram", type: "oauth" as const, color: "#e4405f" },
  { id: "threads", name: "Threads", type: "oauth" as const, color: "#ffffff" },
  { id: "tiktok", name: "TikTok", type: "oauth" as const, color: "#fe2c55" },
  { id: "youtube", name: "YouTube", type: "oauth" as const, color: "#ff0000" },
  { id: "twitter", name: "X / Twitter", type: "oauth" as const, color: "#ffffff" },
];

export default function AccountsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [loading, setLoading] = useState(true);

  const [connectOpen, setConnectOpen] = useState(false);
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

  useEffect(() => {
    loadAccounts();
  }, [loadAccounts]);

  async function handleBlueskyConnect() {
    if (!handle.trim() || !appPassword.trim()) return;
    setConnecting(true);
    setConnectError("");
    try {
      const token = await getToken();
      if (!token) return;
      await connectSocialAccount(token, projectId, {
        platform: "bluesky",
        credentials: { handle: handle.trim(), app_password: appPassword.trim() },
      });
      setConnectOpen(false);
      setSelectedPlatform(null);
      setHandle("");
      setAppPassword("");
      loadAccounts();
    } catch (err) {
      setConnectError(err instanceof Error ? err.message : "Failed to connect");
    } finally {
      setConnecting(false);
    }
  }

  async function handleOAuthConnect(platform: string) {
    setConnecting(true);
    setConnectError("");
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
    if (!confirm("Disconnect this account?")) return;
    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, projectId, accountId);
      loadAccounts();
    } catch (err) {
      console.error("Failed to disconnect:", err);
    }
  }

  function getPlatformColor(platformId: string) {
    return PLATFORMS.find((p) => p.id === platformId)?.color || "#525252";
  }

  return (
    <div>
      {/* Callback banners */}
      {callbackStatus === "success" && (
        <div className="mb-5 flex items-center gap-2 px-4 py-3 rounded-lg bg-emerald/5 border border-emerald/10 text-[13px] text-emerald animate-enter">
          <CheckCircle2 className="w-4 h-4 shrink-0" />
          Connected {callbackAccount || "account"} successfully.
        </div>
      )}
      {callbackStatus === "error" && (
        <div className="mb-5 flex items-center gap-2 px-4 py-3 rounded-lg bg-destructive/5 border border-destructive/10 text-[13px] text-destructive animate-enter">
          <XCircle className="w-4 h-4 shrink-0" />
          Failed to connect. Please try again.
        </div>
      )}

      <div className="flex items-center justify-between mb-6 animate-enter">
        <div>
          <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
            Accounts
          </h1>
          <p className="text-[13px] text-[#525252] mt-0.5">
            Connected social media accounts.
          </p>
        </div>
        <Dialog
          open={connectOpen}
          onOpenChange={(open) => {
            setConnectOpen(open);
            if (!open) { setSelectedPlatform(null); setConnectError(""); }
          }}
        >
          <DialogTrigger render={<Button size="sm" className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90" />}>
            <Plus className="w-3.5 h-3.5" />
            Connect
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>
                {selectedPlatform
                  ? `Connect ${PLATFORMS.find((p) => p.id === selectedPlatform)?.name}`
                  : "Connect Account"}
              </DialogTitle>
              <DialogDescription>
                {selectedPlatform === "bluesky"
                  ? "Enter your handle and App Password."
                  : selectedPlatform
                    ? "You'll be redirected to authorize."
                    : "Choose a platform."}
              </DialogDescription>
            </DialogHeader>

            {!selectedPlatform ? (
              <div className="space-y-1 py-2">
                {PLATFORMS.map((p) => {
                  const connected = accounts.some((a) => a.platform === p.id);
                  return (
                    <button
                      key={p.id}
                      onClick={() => {
                        if (p.type === "oauth") handleOAuthConnect(p.id);
                        else setSelectedPlatform(p.id);
                      }}
                      disabled={connecting}
                      className="w-full flex items-center justify-between px-3.5 py-2.5 rounded-md border border-[#1e1e1e] hover:border-[#2a2a2a] hover:bg-[#111111] transition-all text-left cursor-pointer disabled:opacity-50"
                    >
                      <div className="flex items-center gap-3">
                        <div
                          className="w-2 h-2 rounded-full shrink-0"
                          style={{ backgroundColor: p.color }}
                        />
                        <span className="text-[13px] font-medium text-[#d4d4d4]">
                          {p.name}
                        </span>
                      </div>
                      {connected ? (
                        <Badge variant="secondary" className="text-[9px] bg-emerald/10 text-emerald border-0">
                          Connected
                        </Badge>
                      ) : (
                        <ExternalLink className="w-3.5 h-3.5 text-[#2a2a2a]" />
                      )}
                    </button>
                  );
                })}
              </div>
            ) : selectedPlatform === "bluesky" ? (
              <>
                <div className="space-y-4 py-2">
                  <div className="space-y-1.5">
                    <Label className="text-[12px] text-[#a3a3a3]">Handle</Label>
                    <Input
                      placeholder="alice.bsky.social"
                      value={handle}
                      onChange={(e) => setHandle(e.target.value)}
                      className="bg-[#111111] border-[#1e1e1e]"
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-[12px] text-[#a3a3a3]">App Password</Label>
                    <Input
                      type="password"
                      placeholder="xxxx-xxxx-xxxx-xxxx"
                      value={appPassword}
                      onChange={(e) => setAppPassword(e.target.value)}
                      className="bg-[#111111] border-[#1e1e1e]"
                    />
                  </div>
                  <p className="text-[11px] text-[#3a3a3a]">
                    Generate at bsky.app &rarr; Settings &rarr; App Passwords
                  </p>
                  {connectError && (
                    <p className="text-[12px] text-destructive">{connectError}</p>
                  )}
                </div>
                <DialogFooter>
                  <Button variant="outline" size="sm" onClick={() => setSelectedPlatform(null)}>
                    Back
                  </Button>
                  <Button
                    size="sm"
                    onClick={handleBlueskyConnect}
                    disabled={connecting || !handle.trim() || !appPassword.trim()}
                    className="bg-emerald text-emerald-foreground hover:bg-emerald/90"
                  >
                    {connecting ? "Connecting..." : "Connect"}
                  </Button>
                </DialogFooter>
              </>
            ) : null}

            {connectError && !selectedPlatform && (
              <p className="text-[12px] text-destructive px-1">{connectError}</p>
            )}
          </DialogContent>
        </Dialog>
      </div>

      {/* Accounts grid */}
      {loading ? (
        <div className="grid grid-cols-2 gap-2">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="h-[88px] rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse" />
          ))}
        </div>
      ) : accounts.length === 0 ? (
        <div className="border border-dashed border-[#1e1e1e] rounded-lg py-20 flex flex-col items-center animate-enter" style={{ animationDelay: "50ms" }}>
          <div className="w-12 h-12 rounded-xl bg-[#111111] border border-[#1e1e1e] flex items-center justify-center mb-4">
            <Unplug className="w-5 h-5 text-[#525252]" />
          </div>
          <p className="text-[14px] font-medium text-[#d4d4d4] mb-1">No accounts connected</p>
          <p className="text-[13px] text-[#525252] mb-6">Connect a social account to start posting.</p>
          <Button size="sm" onClick={() => setConnectOpen(true)} className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90">
            <Plus className="w-3.5 h-3.5" />
            Connect Account
          </Button>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-2 animate-enter" style={{ animationDelay: "50ms" }}>
          {accounts.map((account) => (
            <div
              key={account.id}
              className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-4 hover:border-[#2a2a2a] transition-colors"
            >
              <div className="flex items-start justify-between mb-3">
                <div className="flex items-center gap-2.5">
                  <div
                    className="w-3 h-3 rounded-full shrink-0"
                    style={{ backgroundColor: getPlatformColor(account.platform) }}
                  />
                  <span className="text-[13px] font-medium text-[#d4d4d4]">
                    {account.account_name || account.id}
                  </span>
                </div>
                <div className="flex items-center gap-1.5">
                  <div
                    className={`w-1.5 h-1.5 rounded-full ${
                      account.status === "active"
                        ? "bg-emerald pulse-dot"
                        : "bg-amber-status"
                    }`}
                  />
                  <span className={`text-[10px] font-medium ${
                    account.status === "active" ? "text-emerald" : "text-amber-status"
                  }`}>
                    {account.status === "active" ? "Active" : "Reconnect"}
                  </span>
                </div>
              </div>
              <div className="flex items-center justify-between">
                <span className="mono text-[11px] text-[#3a3a3a]">
                  {account.platform} &middot;{" "}
                  {new Date(account.connected_at).toLocaleDateString("en-US", {
                    month: "short",
                    day: "numeric",
                    year: "numeric",
                  })}
                </span>
                <button
                  onClick={() => handleDisconnect(account.id)}
                  className="text-[11px] text-[#3a3a3a] hover:text-destructive transition-colors cursor-pointer"
                >
                  Disconnect
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
