"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
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
  listApiKeys,
  createApiKey,
  revokeApiKey,
  type ApiKey,
} from "@/lib/api";
import { Plus, Key, Copy, Check, Trash2 } from "lucide-react";

export default function ApiKeysPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [keys, setKeys] = useState<ApiKey[]>([]);
  const [loading, setLoading] = useState(true);

  const [createOpen, setCreateOpen] = useState(false);
  const [keyName, setKeyName] = useState("");
  const [keyEnv, setKeyEnv] = useState<"production" | "test">("production");
  const [creating, setCreating] = useState(false);

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
    if (
      !confirm(
        "Are you sure you want to revoke this API key? This cannot be undone."
      )
    )
      return;

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
      <div className="flex items-center justify-between mb-6 animate-fade-up">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">API Keys</h1>
          <p className="text-[13px] text-muted-foreground mt-1">
            Manage API keys for this project.
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<Button size="sm" className="gap-1.5" />}>
            <Plus className="w-3.5 h-3.5" />
            New Key
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>New API Key</DialogTitle>
              <DialogDescription>
                Create a new API key for this project.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <Label htmlFor="key-name" className="text-[13px]">
                  Name
                </Label>
                <Input
                  id="key-name"
                  placeholder="Production Key"
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-[13px]">Environment</Label>
                <div className="flex gap-1.5">
                  <button
                    type="button"
                    onClick={() => setKeyEnv("production")}
                    className={`px-3 py-1.5 rounded-md border text-[12px] font-medium transition-all cursor-pointer ${
                      keyEnv === "production"
                        ? "border-foreground/20 bg-foreground/[0.04] text-foreground"
                        : "border-border text-muted-foreground hover:border-foreground/10"
                    }`}
                  >
                    Production
                  </button>
                  <button
                    type="button"
                    onClick={() => setKeyEnv("test")}
                    className={`px-3 py-1.5 rounded-md border text-[12px] font-medium transition-all cursor-pointer ${
                      keyEnv === "test"
                        ? "border-foreground/20 bg-foreground/[0.04] text-foreground"
                        : "border-border text-muted-foreground hover:border-foreground/10"
                    }`}
                  >
                    Test
                  </button>
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setCreateOpen(false)}
              >
                Cancel
              </Button>
              <Button
                size="sm"
                onClick={handleCreate}
                disabled={creating || !keyName.trim()}
              >
                {creating ? "Creating..." : "Create Key"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* New key reveal */}
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
            <DialogTitle>Copy your API key</DialogTitle>
            <DialogDescription>
              This key won&apos;t be shown again. Store it securely.
            </DialogDescription>
          </DialogHeader>
          <div className="my-3">
            <div className="flex items-center gap-2 p-3 rounded-md bg-muted border border-border">
              <code className="flex-1 mono-data text-[12px] break-all select-all">
                {newKey}
              </code>
              <Button
                size="sm"
                variant="ghost"
                className="shrink-0 h-7 w-7 p-0"
                onClick={handleCopy}
              >
                {copied ? (
                  <Check className="w-3.5 h-3.5 text-foreground/60" />
                ) : (
                  <Copy className="w-3.5 h-3.5" />
                )}
              </Button>
            </div>
          </div>
          <DialogFooter>
            <Button
              size="sm"
              onClick={() => {
                setNewKey(null);
                setCopied(false);
              }}
            >
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Keys list */}
      {loading ? (
        <div className="space-y-2">
          {[1, 2].map((i) => (
            <div
              key={i}
              className="h-[68px] rounded-lg bg-muted/50 animate-pulse"
            />
          ))}
        </div>
      ) : keys.length === 0 ? (
        <div
          className="border border-dashed border-border rounded-lg py-16 flex flex-col items-center animate-fade-up"
          style={{ animationDelay: "60ms" }}
        >
          <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-4">
            <Key className="w-5 h-5 text-muted-foreground" />
          </div>
          <p className="text-[15px] font-medium mb-1">No API keys yet</p>
          <p className="text-[13px] text-muted-foreground mb-5">
            Create your first API key to start making requests.
          </p>
          <Button size="sm" onClick={() => setCreateOpen(true)} className="gap-1.5">
            <Plus className="w-3.5 h-3.5" />
            Create Key
          </Button>
        </div>
      ) : (
        <div className="space-y-1.5">
          {keys.map((key, i) => (
            <div
              key={key.id}
              className="flex items-center justify-between px-4 py-3 rounded-lg border border-border bg-card animate-fade-up"
              style={{ animationDelay: `${(i + 1) * 60}ms` }}
            >
              <div className="min-w-0">
                <div className="flex items-center gap-2.5">
                  <p className="text-[13px] font-medium">{key.name}</p>
                  <Badge
                    variant={
                      key.environment === "production" ? "default" : "secondary"
                    }
                    className="text-[10px]"
                  >
                    {key.environment}
                  </Badge>
                </div>
                <div className="flex items-center gap-4 mt-1">
                  <span className="mono-data text-[11px] text-muted-foreground">
                    {key.prefix}{"••••••••"}
                  </span>
                  <span className="text-[11px] text-muted-foreground">
                    Created{" "}
                    {new Date(key.created_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <span className="text-[11px] text-muted-foreground">
                    Last used:{" "}
                    {key.last_used_at
                      ? new Date(key.last_used_at).toLocaleDateString("en-US", {
                          month: "short",
                          day: "numeric",
                        })
                      : "Never"}
                  </span>
                  {key.expires_at && (
                    <span className="text-[11px] text-muted-foreground">
                      Expires{" "}
                      {new Date(key.expires_at).toLocaleDateString("en-US", {
                        month: "short",
                        day: "numeric",
                      })}
                    </span>
                  )}
                </div>
              </div>
              <Button
                size="sm"
                variant="ghost"
                className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0 shrink-0"
                onClick={() => handleRevoke(key.id)}
              >
                <Trash2 className="w-3.5 h-3.5" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
