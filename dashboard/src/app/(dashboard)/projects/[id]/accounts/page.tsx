"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
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
  type SocialAccount,
} from "@/lib/api";

export default function AccountsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [loading, setLoading] = useState(true);

  // Connect dialog state
  const [connectOpen, setConnectOpen] = useState(false);
  const [handle, setHandle] = useState("");
  const [appPassword, setAppPassword] = useState("");
  const [connecting, setConnecting] = useState(false);
  const [connectError, setConnectError] = useState("");

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

  async function handleConnect() {
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

  async function handleDisconnect(accountId: string) {
    if (
      !confirm(
        "Are you sure you want to disconnect this account? You will need to reconnect it to post again."
      )
    ) {
      return;
    }

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
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">Connected Accounts</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Connect social media accounts to start posting
          </p>
        </div>
        <Dialog open={connectOpen} onOpenChange={setConnectOpen}>
          <DialogTrigger render={<Button />}>+ Connect</DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Connect Bluesky Account</DialogTitle>
              <DialogDescription>
                Enter your Bluesky handle and an App Password. You can generate
                an App Password at bsky.app &rarr; Settings &rarr; App
                Passwords.
              </DialogDescription>
            </DialogHeader>
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
              {connectError && (
                <p className="text-sm text-destructive">{connectError}</p>
              )}
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setConnectOpen(false)}
              >
                Cancel
              </Button>
              <Button
                onClick={handleConnect}
                disabled={connecting || !handle.trim() || !appPassword.trim()}
              >
                {connecting ? "Connecting..." : "Connect Account"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

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
                    {account.status === "active" ? "Active" : "Reconnect Required"}
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
