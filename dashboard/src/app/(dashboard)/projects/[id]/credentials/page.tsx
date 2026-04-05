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
      loadCreds();
    } catch (err) {
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

  if (loading) return <div className="text-muted-foreground">Loading...</div>;

  return (
    <div>
      <h1 className="text-2xl font-bold mb-2">Native Mode Credentials</h1>
      <p className="text-muted-foreground text-sm mb-6">
        Use your own platform app credentials for full brand ownership. OAuth pages will show your app name instead of UniPost&apos;s.
        Requires a paid plan.
      </p>

      {error && (
        <div className="mb-4 p-3 rounded-md bg-red-50 border border-red-200 text-red-800 text-sm">
          {error}
        </div>
      )}

      <div className="space-y-4">
        {PLATFORMS.map((p) => {
          const configured = configuredPlatforms.has(p.id);
          const cred = creds.find((c) => c.platform === p.id);
          const form = forms[p.id] || { clientId: "", clientSecret: "" };

          return (
            <Card key={p.id}>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                <div>
                  <CardTitle className="text-base">{p.name}</CardTitle>
                  <CardDescription>
                    {configured
                      ? `Configured (${cred?.client_id})`
                      : "Using Quickstart mode"}
                  </CardDescription>
                </div>
                <div className="flex items-center gap-2">
                  {configured ? (
                    <>
                      <Badge variant="default">Native</Badge>
                      <Button size="sm" variant="destructive" onClick={() => handleDelete(p.id)}>
                        Remove
                      </Button>
                    </>
                  ) : (
                    <Badge variant="secondary">Quickstart</Badge>
                  )}
                </div>
              </CardHeader>
              {!configured && (
                <CardContent className="space-y-3">
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1">
                      <Label className="text-xs">{p.idLabel}</Label>
                      <Input
                        placeholder={p.idLabel}
                        value={form.clientId}
                        onChange={(e) => updateForm(p.id, "clientId", e.target.value)}
                      />
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs">{p.secretLabel}</Label>
                      <Input
                        type="password"
                        placeholder={p.secretLabel}
                        value={form.clientSecret}
                        onChange={(e) => updateForm(p.id, "clientSecret", e.target.value)}
                      />
                    </div>
                  </div>
                  <div className="flex items-center justify-between">
                    <a
                      href={p.docs}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-xs text-blue-600 hover:underline"
                    >
                      Get credentials from {p.name} Developer Portal
                    </a>
                    <Button
                      size="sm"
                      onClick={() => handleSave(p.id)}
                      disabled={saving === p.id || !form.clientId || !form.clientSecret}
                    >
                      {saving === p.id ? "Saving..." : "Save"}
                    </Button>
                  </div>
                </CardContent>
              )}
            </Card>
          );
        })}
      </div>
    </div>
  );
}
