"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
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

const PLATFORMS = [
  { id: "bluesky", name: "Bluesky", type: "credentials" as const },
  { id: "linkedin", name: "LinkedIn", type: "oauth" as const },
  { id: "instagram", name: "Instagram", type: "oauth" as const },
  { id: "threads", name: "Threads", type: "oauth" as const },
];

export default function AccountsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const searchParams = useSearchParams();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [loading, setLoading] = useState(true);

  // Connect dialog state
  const [connectOpen, setConnectOpen] = useState(false);
  const [selectedPlatform, setSelectedPlatform] = useState<string | null>(null);
  const [handle, setHandle] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState("");

  // OAuth callback status
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
      const res = await getOAuthConnectURL(token, projectId, platform, redirectUrl);
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
      {/* OAuth callback notification */}
      {callbackStatus === "success" && (
        <div className="mb-6 p-4 rounded-md bg-green-50 border border-green-200 text-green-800 text-sm">
          Successfully connected {callbackAccount || "account"}!
        </div>
      )}
      {callbackStatus === "error" && (
        <div className="mb-6 p-4 rounded-md bg-red-50 border border-red-200 text-red-800 text-sm">
          Failed to connect account. Please try again.
        </div>
      )}

      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Connected Accounts</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Connect social media accounts to start posting
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
          <DialogTrigger render={<Button />}>+ Connect</DialogTrigger>
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
              <div className="space-y-2 py-4">
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
                      className="w-full flex items-center justify-between px-4 py-3 rounded-md border hover:bg-muted/50 transition-colors text-left cursor-pointer disabled:opacity-50"
                    >
                      <span className="font-medium text-sm">{p.name}</span>
                      {connected && (
                        <Badge variant="secondary">Connected</Badge>
                      )}
                    </button>
                  );
                })}
              </div>
            ) : selectedPlatform === "bluesky" ? (
              <>
                <div className="space-y-4 py-4">
                  <div className="space-y-2">
                    <Label htmlFor="handle">Handle</Label>
                    <Input
                      id="handle"
                      placeholder="alice.bsky.social"
                      value={handle}
                      onChange={(e) => setHandle(e.target.value)}
                    />
                  </div>
                  <div className="space-y-2">
                    <Label htmlFor="app-password">App Password</Label>
                    <Input
                      id="app-password"
                      type="password"
                      placeholder="xxxx-xxxx-xxxx-xxxx"
                      value={appPassword}
                      onChange={(e) => setAppPassword(e.target.value)}
                    />
                  </div>
                  <p className="text-xs text-muted-foreground">
                    Generate at bsky.app &rarr; Settings &rarr; App Passwords
                  </p>
                  {connectError && (
                    <p className="text-sm text-destructive">{connectError}</p>
                  )}
                </div>
                <DialogFooter>
                  <Button
                    variant="outline"
                    onClick={() => setSelectedPlatform(null)}
                  >
                    Back
                  </Button>
                  <Button
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
              <p className="text-sm text-destructive px-4">{connectError}</p>
            )}
          </DialogContent>
        </Dialog>
      </div>

      {/* Accounts list */}
      {loading ? (
        <div className="text-muted-foreground">Loading...</div>
      ) : accounts.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-16">
            <p className="font-medium mb-1">No accounts connected yet</p>
            <p className="text-sm text-muted-foreground mb-6">
              Connect your first social account to start posting.
            </p>
            <Button onClick={() => setConnectOpen(true)}>
              Connect Account
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {accounts.map((account) => (
            <Card key={account.id}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <div>
                  <CardTitle className="text-base font-medium">
                    {account.account_name || account.id}
                  </CardTitle>
                  <CardDescription className="mt-1">
                    {account.platform} &middot; Connected{" "}
                    {new Date(account.connected_at).toLocaleDateString()}
                  </CardDescription>
                </div>
                <div className="flex items-center gap-3">
                  <Badge
                    variant={
                      account.status === "active" ? "default" : "destructive"
                    }
                  >
                    {account.status === "active"
                      ? "Active"
                      : "Reconnect Required"}
                  </Badge>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleDisconnect(account.id)}
                  >
                    Disconnect
                  </Button>
                </div>
              </CardHeader>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
