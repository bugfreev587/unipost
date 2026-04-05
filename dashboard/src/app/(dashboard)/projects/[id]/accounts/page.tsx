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
import { Plus, ExternalLink, Unplug, CheckCircle2, XCircle } from "lucide-react";

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
        credentials: {
          handle: handle.trim(),
          app_password: appPassword.trim(),
        },
      });
      setConnectOpen(false);
      setSelectedPlatform(null);
      setHandle("");
      setAppPassword("");
      loadAccounts();
    } catch (err) {
      setConnectError(
        err instanceof Error ? err.message : "Failed to connect account"
      );
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
      const res = await getOAuthConnectURL(
        token,
        projectId,
        platform,
        redirectUrl
      );
      window.location.href = res.data.auth_url;
    } catch (err) {
      setConnectError(
        err instanceof Error ? err.message : "Failed to start OAuth flow"
      );
      setConnecting(false);
    }
  }

  async function handleDisconnect(accountId: string) {
    if (!confirm("Are you sure you want to disconnect this account?")) return;

    try {
      const token = await getToken();
      if (!token) return;
      await disconnectSocialAccount(token, projectId, accountId);
      loadAccounts();
    } catch (err) {
      console.error("Failed to disconnect account:", err);
    }
  }

  return (
    <div>
      {/* Callback notifications */}
      {callbackStatus === "success" && (
        <div className="mb-6 flex items-center gap-2 px-4 py-3 rounded-lg border border-foreground/10 bg-foreground/[0.02] text-[13px] animate-fade-up">
          <CheckCircle2 className="w-4 h-4 text-foreground/60 shrink-0" />
          Successfully connected {callbackAccount || "account"}.
        </div>
      )}
      {callbackStatus === "error" && (
        <div className="mb-6 flex items-center gap-2 px-4 py-3 rounded-lg border border-destructive/20 bg-destructive/5 text-[13px] text-destructive animate-fade-up">
          <XCircle className="w-4 h-4 shrink-0" />
          Failed to connect account. Please try again.
        </div>
      )}

      <div className="flex items-center justify-between mb-6 animate-fade-up">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Accounts</h1>
          <p className="text-[13px] text-muted-foreground mt-1">
            Connect social media accounts to start posting.
          </p>
        </div>
        <Dialog
          open={connectOpen}
          onOpenChange={(open) => {
            setConnectOpen(open);
            if (!open) {
              setSelectedPlatform(null);
              setConnectError("");
            }
          }}
        >
          <DialogTrigger render={<Button size="sm" className="gap-1.5" />}>
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
                  ? "Enter your Bluesky handle and App Password."
                  : selectedPlatform
                    ? "You will be redirected to authorize your account."
                    : "Choose a platform to connect."}
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
                        if (p.type === "oauth") {
                          handleOAuthConnect(p.id);
                        } else {
                          setSelectedPlatform(p.id);
                        }
                      }}
                      disabled={connecting}
                      className="w-full flex items-center justify-between px-3.5 py-2.5 rounded-md border border-border hover:border-foreground/15 hover:bg-muted/50 transition-all text-left cursor-pointer disabled:opacity-50"
                    >
                      <div className="flex items-center gap-2.5">
                        <div
                          className={`w-2 h-2 rounded-full ${
                            connected ? "bg-foreground/60" : "bg-muted-foreground/20"
                          }`}
                        />
                        <span className="text-[13px] font-medium">{p.name}</span>
                      </div>
                      {connected ? (
                        <Badge variant="secondary" className="text-[10px]">
                          Connected
                        </Badge>
                      ) : (
                        <ExternalLink className="w-3.5 h-3.5 text-muted-foreground/40" />
                      )}
                    </button>
                  );
                })}
              </div>
            ) : selectedPlatform === "bluesky" ? (
              <>
                <div className="space-y-4 py-2">
                  <div className="space-y-1.5">
                    <Label htmlFor="handle" className="text-[13px]">
                      Handle
                    </Label>
                    <Input
                      id="handle"
                      placeholder="alice.bsky.social"
                      value={handle}
                      onChange={(e) => setHandle(e.target.value)}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="app-password" className="text-[13px]">
                      App Password
                    </Label>
                    <Input
                      id="app-password"
                      type="password"
                      placeholder="xxxx-xxxx-xxxx-xxxx"
                      value={appPassword}
                      onChange={(e) => setAppPassword(e.target.value)}
                    />
                  </div>
                  <p className="text-[11px] text-muted-foreground">
                    Generate at bsky.app &rarr; Settings &rarr; App Passwords
                  </p>
                  {connectError && (
                    <p className="text-[12px] text-destructive">{connectError}</p>
                  )}
                </div>
                <DialogFooter>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => setSelectedPlatform(null)}
                  >
                    Back
                  </Button>
                  <Button
                    size="sm"
                    onClick={handleBlueskyConnect}
                    disabled={
                      connecting || !handle.trim() || !appPassword.trim()
                    }
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

      {/* Accounts list */}
      {loading ? (
        <div className="space-y-2">
          {[1, 2].map((i) => (
            <div
              key={i}
              className="h-16 rounded-lg bg-muted/50 animate-pulse"
            />
          ))}
        </div>
      ) : accounts.length === 0 ? (
        <div
          className="border border-dashed border-border rounded-lg py-16 flex flex-col items-center animate-fade-up"
          style={{ animationDelay: "60ms" }}
        >
          <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-4">
            <Unplug className="w-5 h-5 text-muted-foreground" />
          </div>
          <p className="text-[15px] font-medium mb-1">No accounts connected</p>
          <p className="text-[13px] text-muted-foreground mb-5">
            Connect your first social account to start posting.
          </p>
          <Button size="sm" onClick={() => setConnectOpen(true)} className="gap-1.5">
            <Plus className="w-3.5 h-3.5" />
            Connect Account
          </Button>
        </div>
      ) : (
        <div className="space-y-1.5">
          {accounts.map((account, i) => (
            <div
              key={account.id}
              className="flex items-center justify-between px-4 py-3 rounded-lg border border-border bg-card animate-fade-up"
              style={{ animationDelay: `${(i + 1) * 60}ms` }}
            >
              <div className="flex items-center gap-3 min-w-0">
                <div
                  className={`w-2 h-2 rounded-full shrink-0 ${
                    account.status === "active"
                      ? "bg-foreground/60"
                      : "bg-destructive/60"
                  }`}
                />
                <div className="min-w-0">
                  <p className="text-[13px] font-medium truncate">
                    {account.account_name || account.id}
                  </p>
                  <p className="mono-data text-[11px] text-muted-foreground">
                    {account.platform} &middot;{" "}
                    {new Date(account.connected_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                      year: "numeric",
                    })}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-2.5 shrink-0">
                <Badge
                  variant={
                    account.status === "active" ? "secondary" : "destructive"
                  }
                  className="text-[10px]"
                >
                  {account.status === "active" ? "Active" : "Reconnect"}
                </Badge>
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive hover:text-destructive hover:bg-destructive/10 text-[12px] h-7 px-2"
                  onClick={() => handleDisconnect(account.id)}
                >
                  Disconnect
                </Button>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
