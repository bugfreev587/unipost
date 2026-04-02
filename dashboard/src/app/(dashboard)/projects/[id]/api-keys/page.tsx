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
  listApiKeys,
  createApiKey,
  revokeApiKey,
  type ApiKey,
} from "@/lib/api";

export default function ApiKeysPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);

  // Create dialog state
  const [createOpen, setCreateOpen] = useState(false);
  const [keyName, setKeyName] = useState("");
  const [keyEnv, setKeyEnv] = useState<"production" | "test">("production");
  const [creating, setCreating] = useState(false);

  // One-time key display state
  const [newKey, setNewKey] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const loadKeys = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await listApiKeys(token, projectId);
      setKeys(res.data);
    } catch (err) {
      console.error("Failed to load API keys:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => {
    loadKeys();
  }, [loadKeys]);

  async function handleCreate() {
    if (!keyName.trim()) return;
    setCreating(true);

    try {
      const token = await getToken();
      if (!token) return;
      const res = await createApiKey(token, projectId, {
        name: keyName.trim(),
        environment: keyEnv,
      });
      setNewKey(res.data.key);
      setCreateOpen(false);
      setKeyName("");
      setKeyEnv("production");
      loadKeys();
    } catch (err) {
      console.error("Failed to create API key:", err);
    } finally {
      setCreating(false);
    }
  }

  async function handleRevoke(keyId: string) {
    if (!confirm("Are you sure you want to revoke this API key? This cannot be undone.")) {
      return;
    }

    try {
      const token = await getToken();
      if (!token) return;
      await revokeApiKey(token, projectId, keyId);
      loadKeys();
    } catch (err) {
      console.error("Failed to revoke API key:", err);
    }
  }

  function handleCopy() {
    if (newKey) {
      navigator.clipboard.writeText(newKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold">API Keys</h1>
          <p className="text-muted-foreground text-sm mt-1">
            Manage API keys for this project
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<Button />}>
            + New Key
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>New API Key</DialogTitle>
              <DialogDescription>
                Create a new API key for this project.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="key-name">Name</Label>
                <Input
                  id="key-name"
                  placeholder="Production Key"
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                />
              </div>
              <div className="space-y-2">
                <Label>Environment</Label>
                <div className="flex gap-2">
                  <Button
                    type="button"
                    size="sm"
                    variant={keyEnv === "production" ? "default" : "outline"}
                    onClick={() => setKeyEnv("production")}
                  >
                    Production
                  </Button>
                  <Button
                    type="button"
                    size="sm"
                    variant={keyEnv === "test" ? "default" : "outline"}
                    onClick={() => setKeyEnv("test")}
                  >
                    Test
                  </Button>
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                onClick={() => setCreateOpen(false)}
              >
                Cancel
              </Button>
              <Button
                onClick={handleCreate}
                disabled={creating || !keyName.trim()}
              >
                {creating ? "Creating..." : "Create Key"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* One-time key display dialog */}
      <Dialog
        open={!!newKey}
        onOpenChange={(open) => {
          if (!open) {
            setNewKey(null);
            setCopied(false);
          }
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Copy your API key now</DialogTitle>
            <DialogDescription>
              It won&apos;t be shown again. Store it somewhere safe.
            </DialogDescription>
          </DialogHeader>
          <div className="my-4">
            <div className="flex items-center gap-2">
              <code className="flex-1 bg-muted px-3 py-2 rounded-md text-sm font-mono break-all">
                {newKey}
              </code>
              <Button size="sm" variant="outline" onClick={handleCopy}>
                {copied ? "Copied!" : "Copy"}
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => { setNewKey(null); setCopied(false); }}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Keys list */}
      {loading ? (
        <div className="text-muted-foreground">Loading...</div>
      ) : keys.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <p className="text-muted-foreground mb-4">No API keys yet</p>
            <Button onClick={() => setCreateOpen(true)}>
              Create your first API key
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-3">
          {keys.map((key) => (
            <Card key={key.id}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <div>
                  <CardTitle className="text-base font-medium">
                    {key.name}
                  </CardTitle>
                  <CardDescription className="mt-1 font-mono text-xs">
                    {key.prefix}{"••••••••••••••"}
                  </CardDescription>
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant={key.environment === "production" ? "default" : "secondary"}>
                    {key.environment}
                  </Badge>
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={() => handleRevoke(key.id)}
                  >
                    Revoke
                  </Button>
                </div>
              </CardHeader>
              <CardContent>
                <div className="flex gap-6 text-xs text-muted-foreground">
                  <span>
                    Created {new Date(key.created_at).toLocaleDateString()}
                  </span>
                  <span>
                    Last used:{" "}
                    {key.last_used_at
                      ? new Date(key.last_used_at).toLocaleDateString()
                      : "Never"}
                  </span>
                  {key.expires_at && (
                    <span>
                      Expires {new Date(key.expires_at).toLocaleDateString()}
                    </span>
                  )}
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
