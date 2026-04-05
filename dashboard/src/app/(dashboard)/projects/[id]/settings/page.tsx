"use client";

import { useCallback, useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Separator } from "@/components/ui/separator";
import { getProject, updateProject, deleteProject, type Project } from "@/lib/api";
import { AlertTriangle, Check, ExternalLink, Trash2, ChevronDown, ChevronUp } from "lucide-react";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

const CRED_PLATFORMS = [
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

export default function SettingsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();

  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Credentials state
  const [creds, setCreds] = useState<PlatformCred[]>([]);
  const [credForms, setCredForms] = useState<Record<string, { clientId: string; clientSecret: string }>>({});
  const [credSaving, setCredSaving] = useState<string | null>(null);
  const [credError, setCredError] = useState("");
  const [expandedCred, setExpandedCred] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getProject(token, projectId);
        setProject(res.data);
        setName(res.data.name);
      } catch (err) {
        console.error("Failed to load project:", err);
      }
    }
    load();
  }, [getToken, projectId]);

  const loadCreds = useCallback(async () => {
    try {
      const token = await getToken();
      if (!token) return;
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const data = await res.json();
      setCreds(data.data || []);
    } catch {
      // silent
    }
  }, [getToken, projectId]);

  useEffect(() => {
    loadCreds();
  }, [loadCreds]);

  async function handleSave(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setSaving(true);
    setSaved(false);
    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateProject(token, projectId, { name: name.trim() });
      setProject(res.data);
      setSaved(true);
      setTimeout(() => setSaved(false), 2000);
    } catch (err) {
      console.error("Failed to update:", err);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!confirm("Delete this project? All API keys and data will be permanently removed.")) return;
    setDeleting(true);
    try {
      const token = await getToken();
      if (!token) return;
      await deleteProject(token, projectId);
      router.push("/");
    } catch (err) {
      console.error("Failed to delete:", err);
    } finally {
      setDeleting(false);
    }
  }

  function updateCredForm(platform: string, field: "clientId" | "clientSecret", value: string) {
    setCredForms((prev) => ({
      ...prev,
      [platform]: { ...prev[platform], [field]: value },
    }));
  }

  async function handleCredSave(platform: string) {
    const form = credForms[platform];
    if (!form?.clientId || !form?.clientSecret) return;
    setCredSaving(platform);
    setCredError("");
    try {
      const token = await getToken();
      if (!token) return;
      const res = await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
        body: JSON.stringify({ platform, client_id: form.clientId, client_secret: form.clientSecret }),
      });
      if (!res.ok) {
        const err = await res.json();
        setCredError(err.error?.message || "Failed to save");
        return;
      }
      setCredForms((prev) => ({ ...prev, [platform]: { clientId: "", clientSecret: "" } }));
      setExpandedCred(null);
      loadCreds();
    } catch {
      setCredError("Failed to save credentials");
    } finally {
      setCredSaving(null);
    }
  }

  async function handleCredDelete(platform: string) {
    if (!confirm(`Remove ${platform} credentials?`)) return;
    try {
      const token = await getToken();
      if (!token) return;
      await fetch(`${API_URL}/v1/projects/${projectId}/platform-credentials/${platform}`, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      loadCreds();
    } catch {
      // silent
    }
  }

  const configuredPlatforms = new Set(creds.map((c) => c.platform));

  if (!project) {
    return (
      <div className="space-y-4">
        <div className="h-5 w-32 bg-[#111111] rounded animate-pulse" />
        <div className="h-32 rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse" />
      </div>
    );
  }

  return (
    <div className="max-w-[640px]">
      <div className="mb-6 animate-enter">
        <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
          Settings
        </h1>
        <p className="text-[13px] text-[#525252] mt-0.5">
          Project configuration and credentials.
        </p>
      </div>

      {/* General */}
      <div className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-5 mb-6 animate-enter" style={{ animationDelay: "50ms" }}>
        <p className="text-[12px] font-medium text-[#a3a3a3] mb-4">General</p>
        <form onSubmit={handleSave} className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-[12px] text-[#737373]">Project Name</Label>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="bg-[#0a0a0a] border-[#1e1e1e]"
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[12px] text-[#737373]">Project ID</Label>
            <p className="mono text-[12px] text-[#3a3a3a] select-all">{project.id}</p>
          </div>
          <div className="space-y-1.5">
            <Label className="text-[12px] text-[#737373]">Mode</Label>
            <Badge variant="secondary" className="text-[10px] bg-[#1a1a1a] text-[#737373] border-0">
              {project.mode}
            </Badge>
          </div>
          <div className="flex items-center gap-2">
            <Button type="submit" size="sm" disabled={saving || !name.trim()} className="bg-emerald text-emerald-foreground hover:bg-emerald/90">
              {saving ? "Saving..." : saved ? "Saved" : "Save"}
            </Button>
            {saved && (
              <span className="flex items-center gap-1 text-[11px] text-emerald animate-enter">
                <Check className="w-3 h-3" /> Saved
              </span>
            )}
          </div>
        </form>
      </div>

      {/* Native mode credentials */}
      <div className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-5 mb-6 animate-enter" style={{ animationDelay: "100ms" }}>
        <div className="flex items-center justify-between mb-1">
          <p className="text-[12px] font-medium text-[#a3a3a3]">
            Native Mode Credentials
          </p>
          <Badge variant="secondary" className="text-[9px] bg-[#1a1a1a] text-[#525252] border-0">
            BYOC
          </Badge>
        </div>
        <p className="text-[11px] text-[#3a3a3a] mb-4">
          Use your own app credentials. OAuth pages will show your brand.
        </p>

        {credError && (
          <div className="mb-3 px-3 py-2 rounded-md bg-destructive/5 border border-destructive/10 text-[11px] text-destructive">
            {credError}
          </div>
        )}

        <div className="space-y-1">
          {CRED_PLATFORMS.map((p) => {
            const configured = configuredPlatforms.has(p.id);
            const cred = creds.find((c) => c.platform === p.id);
            const form = credForms[p.id] || { clientId: "", clientSecret: "" };
            const isExpanded = expandedCred === p.id;

            return (
              <div key={p.id} className="rounded-md border border-[#1a1a1a] overflow-hidden">
                <div className="flex items-center justify-between px-3 py-2.5">
                  <div className="flex items-center gap-2.5">
                    <div className={`w-1.5 h-1.5 rounded-full ${configured ? "bg-emerald" : "bg-[#2a2a2a]"}`} />
                    <span className="text-[12px] font-medium text-[#d4d4d4]">{p.name}</span>
                    {configured && (
                      <span className="mono text-[10px] text-[#3a3a3a]">{cred?.client_id}</span>
                    )}
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Badge variant="secondary" className={`text-[9px] border-0 ${configured ? "bg-emerald/10 text-emerald" : "bg-[#1a1a1a] text-[#3a3a3a]"}`}>
                      {configured ? "Native" : "Quickstart"}
                    </Badge>
                    {configured ? (
                      <button
                        onClick={() => handleCredDelete(p.id)}
                        className="p-1 rounded hover:bg-destructive/10 transition-colors cursor-pointer"
                      >
                        <Trash2 className="w-3 h-3 text-[#3a3a3a] hover:text-destructive" />
                      </button>
                    ) : (
                      <button
                        onClick={() => setExpandedCred(isExpanded ? null : p.id)}
                        className="p-1 rounded hover:bg-[#1a1a1a] transition-colors cursor-pointer"
                      >
                        {isExpanded ? (
                          <ChevronUp className="w-3 h-3 text-[#3a3a3a]" />
                        ) : (
                          <ChevronDown className="w-3 h-3 text-[#3a3a3a]" />
                        )}
                      </button>
                    )}
                  </div>
                </div>

                {!configured && isExpanded && (
                  <div className="px-3 pb-3 pt-0 border-t border-[#1a1a1a]">
                    <div className="grid grid-cols-2 gap-2 mt-3">
                      <div className="space-y-1">
                        <Label className="text-[10px] text-[#3a3a3a]">{p.idLabel}</Label>
                        <Input
                          placeholder={p.idLabel}
                          value={form.clientId}
                          onChange={(e) => updateCredForm(p.id, "clientId", e.target.value)}
                          className="h-8 text-[12px] bg-[#0a0a0a] border-[#1e1e1e]"
                        />
                      </div>
                      <div className="space-y-1">
                        <Label className="text-[10px] text-[#3a3a3a]">{p.secretLabel}</Label>
                        <Input
                          type="password"
                          placeholder={p.secretLabel}
                          value={form.clientSecret}
                          onChange={(e) => updateCredForm(p.id, "clientSecret", e.target.value)}
                          className="h-8 text-[12px] bg-[#0a0a0a] border-[#1e1e1e]"
                        />
                      </div>
                    </div>
                    <div className="flex items-center justify-between mt-3">
                      <a
                        href={p.docs}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center gap-1 text-[10px] text-[#3a3a3a] hover:text-[#525252] transition-colors"
                      >
                        Developer Portal <ExternalLink className="w-2.5 h-2.5" />
                      </a>
                      <Button
                        size="sm"
                        className="h-7 text-[11px] bg-emerald text-emerald-foreground hover:bg-emerald/90"
                        onClick={() => handleCredSave(p.id)}
                        disabled={credSaving === p.id || !form.clientId || !form.clientSecret}
                      >
                        {credSaving === p.id ? "Saving..." : "Save"}
                      </Button>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      <Separator className="my-6 bg-[#1e1e1e]" />

      {/* Danger zone */}
      <div className="rounded-lg border border-destructive/20 bg-destructive/[0.02] p-5 animate-enter" style={{ animationDelay: "150ms" }}>
        <div className="flex items-center gap-2 mb-2">
          <AlertTriangle className="w-3.5 h-3.5 text-destructive" />
          <p className="text-[12px] font-medium text-destructive">Danger Zone</p>
        </div>
        <p className="text-[11px] text-[#525252] mb-4">
          Permanently delete this project and all associated data.
        </p>
        <Button
          variant="destructive"
          size="sm"
          onClick={handleDelete}
          disabled={deleting}
          className="bg-destructive/10 text-destructive hover:bg-destructive/20 border border-destructive/20"
        >
          {deleting ? "Deleting..." : "Delete Project"}
        </Button>
      </div>
    </div>
  );
}
