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
    if (!confirm("Revoke this API key? This cannot be undone.")) return;
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
      <div className="flex items-center justify-between mb-6 animate-enter">
        <div>
          <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
            API Keys
          </h1>
          <p className="text-[13px] text-[#525252] mt-0.5">
            Create and manage API keys for this project.
          </p>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<Button size="sm" className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90" />}>
            <Plus className="w-3.5 h-3.5" />
            Create Key
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create API Key</DialogTitle>
              <DialogDescription>
                Generate a new key for authenticating API requests.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-2">
              <div className="space-y-1.5">
                <Label className="text-[12px] text-[#a3a3a3]">Name</Label>
                <Input
                  placeholder="e.g. Production"
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                  className="bg-[#111111] border-[#1e1e1e]"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-[12px] text-[#a3a3a3]">Environment</Label>
                <div className="flex gap-1.5">
                  {(["production", "test"] as const).map((env) => (
                    <button
                      key={env}
                      type="button"
                      onClick={() => setKeyEnv(env)}
                      className={`px-3 py-1.5 rounded-md border text-[12px] font-medium transition-all cursor-pointer ${
                        keyEnv === env
                          ? "border-emerald/30 bg-emerald/5 text-emerald"
                          : "border-[#1e1e1e] text-[#525252] hover:border-[#2a2a2a] hover:text-[#737373]"
                      }`}
                    >
                      {env.charAt(0).toUpperCase() + env.slice(1)}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" size="sm" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button
                size="sm"
                onClick={handleCreate}
                disabled={creating || !keyName.trim()}
                className="bg-emerald text-emerald-foreground hover:bg-emerald/90"
              >
                {creating ? "Creating..." : "Create"}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* New key reveal modal */}
      <Dialog
        open={!!newKey}
        onOpenChange={(open) => { if (!open) { setNewKey(null); setCopied(false); } }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Your new API key</DialogTitle>
            <DialogDescription>
              Copy it now — it won&apos;t be shown again.
            </DialogDescription>
          </DialogHeader>
          <div className="my-2 flex items-center gap-2 p-3 rounded-md bg-[#0a0a0a] border border-[#1e1e1e]">
            <code className="flex-1 mono text-[12px] text-emerald break-all select-all">
              {newKey}
            </code>
            <button
              onClick={handleCopy}
              className="shrink-0 p-1.5 rounded hover:bg-[#1a1a1a] transition-colors cursor-pointer"
            >
              {copied ? (
                <Check className="w-3.5 h-3.5 text-emerald" />
              ) : (
                <Copy className="w-3.5 h-3.5 text-[#525252]" />
              )}
            </button>
          </div>
          <DialogFooter>
            <Button size="sm" onClick={() => { setNewKey(null); setCopied(false); }}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Keys table */}
      {loading ? (
        <div className="space-y-1">
          {[1, 2].map((i) => (
            <div key={i} className="h-[52px] rounded-md bg-[#111111] animate-pulse" />
          ))}
        </div>
      ) : keys.length === 0 ? (
        <div className="border border-dashed border-[#1e1e1e] rounded-lg py-20 flex flex-col items-center animate-enter" style={{ animationDelay: "50ms" }}>
          <div className="w-12 h-12 rounded-xl bg-[#111111] border border-[#1e1e1e] flex items-center justify-center mb-4">
            <Key className="w-5 h-5 text-[#525252]" />
          </div>
          <p className="text-[14px] font-medium text-[#d4d4d4] mb-1">No API keys</p>
          <p className="text-[13px] text-[#525252] mb-6">
            Create a key to start making API requests.
          </p>
          <Button size="sm" onClick={() => setCreateOpen(true)} className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90">
            <Plus className="w-3.5 h-3.5" />
            Create Key
          </Button>
        </div>
      ) : (
        <div className="rounded-lg border border-[#1e1e1e] overflow-hidden animate-enter" style={{ animationDelay: "50ms" }}>
          {/* Table header */}
          <div className="grid grid-cols-[1fr_140px_100px_100px_48px] gap-4 px-4 py-2.5 bg-[#0d0d0d] border-b border-[#1e1e1e]">
            <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Name</span>
            <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Prefix</span>
            <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Created</span>
            <span className="text-[10px] font-medium uppercase tracking-[0.1em] text-[#3a3a3a]">Last Used</span>
            <span />
          </div>
          {/* Table rows */}
          {keys.map((key) => (
            <div
              key={key.id}
              className="table-row grid grid-cols-[1fr_140px_100px_100px_48px] gap-4 items-center px-4 py-2.5 border-b border-[#1e1e1e] last:border-b-0"
            >
              <div className="flex items-center gap-2 min-w-0">
                <span className="text-[13px] font-medium text-[#d4d4d4] truncate">
                  {key.name}
                </span>
                <Badge
                  variant="secondary"
                  className={`text-[9px] border-0 shrink-0 ${
                    key.environment === "production"
                      ? "bg-emerald/10 text-emerald"
                      : "bg-[#1a1a1a] text-[#525252]"
                  }`}
                >
                  {key.environment}
                </Badge>
              </div>
              <span className="mono text-[12px] text-[#525252] truncate">
                {key.prefix}{"••••••••"}
              </span>
              <span className="mono text-[11px] text-[#3a3a3a]">
                {new Date(key.created_at).toLocaleDateString("en-US", {
                  month: "short",
                  day: "numeric",
                })}
              </span>
              <span className="mono text-[11px] text-[#3a3a3a]">
                {key.last_used_at
                  ? new Date(key.last_used_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })
                  : "Never"}
              </span>
              <button
                onClick={() => handleRevoke(key.id)}
                className="p-1.5 rounded hover:bg-destructive/10 transition-colors cursor-pointer"
                title="Revoke"
              >
                <Trash2 className="w-3.5 h-3.5 text-[#3a3a3a] hover:text-destructive transition-colors" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
