"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { listApiKeys, createApiKey, revokeApiKey, type ApiKey } from "@/lib/api";
import { Plus, Key, AlertTriangle } from "lucide-react";

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

  useEffect(() => { loadKeys(); }, [loadKeys]);

  async function handleCreate() {
    if (!keyName.trim()) return;
    setCreating(true);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await createApiKey(token, projectId, { name: keyName.trim(), environment: keyEnv });
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
      console.error("Failed to revoke:", err);
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
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>API Keys</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Manage authentication keys for your project</div>
        </div>
        <Dialog open={createOpen} onOpenChange={setCreateOpen}>
          <DialogTrigger render={<button className="dbtn dbtn-primary" />}>
            <Plus style={{ width: 13, height: 13 }} /> Create Key
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create API Key</DialogTitle>
              <DialogDescription>Generate a new key for authenticating API requests.</DialogDescription>
            </DialogHeader>
            <div style={{ padding: "8px 0" }}>
              <div style={{ marginBottom: 16 }}>
                <label className="dform-label">Key Name</label>
                <input
                  className="dform-input"
                  placeholder="e.g. Production, Staging..."
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleCreate()}
                  autoFocus
                />
              </div>
              <div>
                <label className="dform-label">Environment</label>
                <div style={{ display: "flex", gap: 6 }}>
                  {(["production", "test"] as const).map((env) => (
                    <button
                      key={env}
                      type="button"
                      onClick={() => setKeyEnv(env)}
                      className={keyEnv === env ? "dbtn dbtn-primary" : "dbtn dbtn-ghost"}
                      style={{ padding: "5px 12px", fontSize: 12 }}
                    >
                      {env.charAt(0).toUpperCase() + env.slice(1)}
                    </button>
                  ))}
                </div>
              </div>
            </div>
            <DialogFooter>
              <button className="dbtn dbtn-ghost" onClick={() => setCreateOpen(false)}>Cancel</button>
              <button className="dbtn dbtn-primary" onClick={handleCreate} disabled={creating || !keyName.trim()}>
                {creating ? "Creating..." : "Create Key"}
              </button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* New key reveal */}
      <Dialog open={!!newKey} onOpenChange={(open) => { if (!open) { setNewKey(null); setCopied(false); } }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Key Created</DialogTitle>
            <DialogDescription>Copy this key now — it won&apos;t be shown again.</DialogDescription>
          </DialogHeader>
          <div style={{ padding: "8px 0" }}>
            <div className="key-warning">
              <AlertTriangle style={{ width: 14, height: 14, flexShrink: 0, marginTop: 1 }} />
              <span>Store this key securely. You won&apos;t be able to see it again.</span>
            </div>
            <label className="dform-label">Your API Key</label>
            <div className="key-display">
              <span className="key-value">{newKey}</span>
              <button className={`copy-btn ${copied ? "copied" : ""}`} onClick={handleCopy} style={{ color: copied ? "var(--daccent)" : undefined }}>
                {copied ? "Copied!" : "Copy"}
              </button>
            </div>
          </div>
          <DialogFooter>
            <button className="dbtn dbtn-ghost" onClick={() => { setNewKey(null); setCopied(false); }}>Done</button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Table */}
      {loading ? (
        <div style={{ color: "var(--dmuted)" }}>Loading...</div>
      ) : keys.length === 0 ? (
        <div className="empty-state">
          <Key style={{ width: 32, height: 32, opacity: 0.4, marginBottom: 12 }} />
          <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 6 }}>No API keys</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", maxWidth: 280, lineHeight: 1.6 }}>
            Create a key to start making API requests.
          </div>
          <button className="dbtn dbtn-primary" onClick={() => setCreateOpen(true)} style={{ marginTop: 16 }}>
            <Plus style={{ width: 13, height: 13 }} /> Create Key
          </button>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Key</th>
                <th>Created</th>
                <th>Last Used</th>
                <th>Status</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {keys.map((key) => (
                <tr key={key.id}>
                  <td style={{ fontWeight: 500 }}>{key.name}</td>
                  <td><span className="mono">{key.prefix}{"••••••••"}</span></td>
                  <td style={{ color: "var(--dmuted)" }}>
                    {new Date(key.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}
                  </td>
                  <td style={{ color: "var(--dmuted)" }}>
                    {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString("en-US", { month: "short", day: "numeric" }) : "Never"}
                  </td>
                  <td>
                    <span className={`dbadge ${key.environment === "production" ? "dbadge-green" : "dbadge-blue"}`}>
                      <span className="dbadge-dot" />
                      {key.environment}
                    </span>
                  </td>
                  <td style={{ textAlign: "right" }}>
                    <button
                      className="dbtn dbtn-danger"
                      style={{ padding: "4px 10px", fontSize: 11.5 }}
                      onClick={() => handleRevoke(key.id)}
                    >
                      Revoke
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}
