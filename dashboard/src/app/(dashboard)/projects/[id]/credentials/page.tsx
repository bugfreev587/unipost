"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { ExternalLink, Trash2, ShieldCheck, ChevronDown, ChevronUp } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const PLATFORMS = [
  { id: "instagram", name: "Instagram / Meta", idLabel: "App ID", secretLabel: "App Secret", docs: "https://developers.facebook.com" },
  { id: "threads", name: "Threads", idLabel: "App ID", secretLabel: "App Secret", docs: "https://developers.facebook.com" },
  { id: "linkedin", name: "LinkedIn", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.linkedin.com" },
  { id: "tiktok", name: "TikTok", idLabel: "Client Key", secretLabel: "Client Secret", docs: "https://developers.tiktok.com" },
  { id: "youtube", name: "YouTube", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://console.cloud.google.com" },
  { id: "twitter", name: "X / Twitter", idLabel: "Client ID", secretLabel: "Client Secret", docs: "https://developer.x.com" },
];

interface PlatformCred {
  platform: string;
  client_id: string;
  created_at: string;
}

export default function CredentialsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [creds, setCreds] = useState<PlatformCred[]>([]);
  const [loading, setLoading] = useState(true);
  const [forms, setForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [saving, setSaving] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [expanded, setExpanded] = useState<string | null>(null);

  const loadCreds = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      setCreds(data.data || []);
    } catch (err) {
      console.error("Failed to load credentials:", err);
    } finally {
      setLoading(false);
    }
  }, [getToken, projectId]);

  useEffect(() => {
    loadCreds();
  }, [loadCreds]);

  function updateForm(platform: string, field: "clientId" | "clientSecret", value: string) {
    setForms((prev) => ({
      ...prev,
      [platform]: { ...prev[platform], [field]: value },
    }));
  }

  async function handleSave(platform: string) {
    const form = forms[platform];
    if (!form?.clientId || !form?.clientSecret) return;
    setSaving(platform);
    setError("");

    try {
      const token = await getToken();
      if (!token) return;
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({
          platform,
          client_id: form.clientId,
          client_secret: form.clientSecret,
        }),
      });
      if (!res.ok) {
        const err = await res.json();
        setError(err.error?.message || "Failed to save");
        return;
      }
      setForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      setExpanded(null);
      loadCreds();
    } catch {
      setError("Failed to save credentials");
    } finally {
      setSaving(null);
    }
  }

  async function handleDelete(platform: string) {
    if (!confirm(`Remove ${platform} credentials? This will revert to Quickstart mode.`)) return;
    try {
      const token = await getToken();
      if (!token) return;
      await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials/${platform}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      loadCreds();
    } catch (err) {
      console.error("Failed to delete:", err);
    }
  }

  const configuredPlatforms = new Set(creds.map((c) => c.platform));

  if (loading) {
    return (
      <div className="space-y-4">
        <div className="h-6 w-48 bg-muted rounded animate-pulse" />
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <div key={i} className="h-14 rounded-lg bg-muted/50 animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6 animate-fade-up">
        <h1 className="text-xl font-semibold tracking-tight">Credentials</h1>
        <p className="text-[13px] text-muted-foreground mt-1">
          Bring your own platform app credentials for native mode. OAuth pages
          will show your app name instead of UniPost&apos;s.
        </p>
      </div>

      {error && (
        <div className="mb-4 px-4 py-3 rounded-lg border border-destructive/20 bg-destructive/5 text-[13px] text-destructive animate-fade-up">
          {error}
        </div>
      )}

      <div className="space-y-1.5">
        {PLATFORMS.map((p, i) => {
          const configured = configuredPlatforms.has(p.id);
          const cred = creds.find((c) => c.platform === p.id);
          const form = forms[p.id] || { clientId: "", clientSecret: "" };
          const isExpanded = expanded === p.id;

          return (
            <div
              key={p.id}
              className="rounded-lg border border-border bg-card overflow-hidden animate-fade-up"
              style={{ animationDelay: `${(i + 1) * 40}ms` }}
            >
              {/* Header row */}
              <div className="flex items-center justify-between px-4 py-3">
                <div className="flex items-center gap-3 min-w-0">
                  <div
                    className={`w-2 h-2 rounded-full shrink-0 ${
                      configured ? "bg-foreground/60" : "bg-muted-foreground/20"
                    }`}
                  />
                  <div>
                    <p className="text-[13px] font-medium">{p.name}</p>
                    <p className="mono-data text-[11px] text-muted-foreground">
                      {configured
                        ? cred?.client_id
                        : "Quickstart mode"}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Badge
                    variant={configured ? "default" : "secondary"}
                    className="text-[10px]"
                  >
                    {configured ? "Native" : "Quickstart"}
                  </Badge>
                  {configured ? (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="text-destructive hover:text-destructive hover:bg-destructive/10 h-7 w-7 p-0"
                      onClick={() => handleDelete(p.id)}
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </Button>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setExpanded(isExpanded ? null : p.id)}
                      className="h-7 w-7 flex items-center justify-center rounded-md hover:bg-muted transition-colors cursor-pointer"
                    >
                      {isExpanded ? (
                        <ChevronUp className="w-3.5 h-3.5 text-muted-foreground" />
                      ) : (
                        <ChevronDown className="w-3.5 h-3.5 text-muted-foreground" />
                      )}
                    </button>
                  )}
                </div>
              </div>

              {/* Expanded form */}
              {!configured && isExpanded && (
                <div className="px-4 pb-4 pt-1 border-t border-border">
                  <div className="grid grid-cols-2 gap-3 mt-3">
                    <div className="space-y-1.5">
                      <Label className="text-[11px] text-muted-foreground">
                        {p.idLabel}
                      </Label>
                      <Input
                        placeholder={p.idLabel}
                        value={form.clientId}
                        onChange={(e) => updateForm(p.id, "clientId", e.target.value)}
                        className="h-8 text-[13px]"
                      />
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-[11px] text-muted-foreground">
                        {p.secretLabel}
                      </Label>
                      <Input
                        type="password"
                        placeholder={p.secretLabel}
                        value={form.clientSecret}
                        onChange={(e) => updateForm(p.id, "clientSecret", e.target.value)}
                        className="h-8 text-[13px]"
                      />
                    </div>
                  </div>
                  <div className="flex items-center justify-between mt-3">
                    <a
                      href={p.docs}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-[11px] text-muted-foreground hover:text-foreground transition-colors"
                    >
                      Developer Portal
                      <ExternalLink className="w-3 h-3" />
                    </a>
                    <Button
                      size="sm"
                      className="h-7 text-[12px]"
                      onClick={() => handleSave(p.id)}
                      disabled={saving === p.id || !form.clientId || !form.clientSecret}
                    >
                      {saving === p.id ? "Saving..." : "Save"}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
