"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { getProject, updateProject, deleteProject, type Project } from "@/lib/api";
import { AlertTriangle } from "lucide-react";

export default function SettingsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [deleting, setDeleting] = useState(false);

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
      console.error("Failed to update project:", err);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (
      !confirm(
        "Are you sure you want to delete this project? All API keys and data will be permanently deleted."
      )
    )
      return;
    setDeleting(true);

    try {
      const token = await getToken();
      if (!token) return;
      await deleteProject(token, projectId);
      router.push("/");
    } catch (err) {
      console.error("Failed to delete project:", err);
    } finally {
      setDeleting(false);
    }
  }

  if (!project) {
    return (
      <div className="space-y-4">
        <div className="h-6 w-32 bg-muted rounded animate-pulse" />
        <div className="h-32 rounded-lg bg-muted/50 animate-pulse" />
      </div>
    );
  }

  return (
    <div className="max-w-lg">
      <div className="mb-6 animate-fade-up">
        <h1 className="text-xl font-semibold tracking-tight">Settings</h1>
        <p className="text-[13px] text-muted-foreground mt-1">
          Manage project configuration.
        </p>
      </div>

      {/* General */}
      <div
        className="rounded-lg border border-border bg-card p-5 mb-6 animate-fade-up"
        style={{ animationDelay: "60ms" }}
      >
        <p className="text-[13px] font-medium mb-4">General</p>
        <form onSubmit={handleSave} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="project-name" className="text-[13px]">
              Project Name
            </Label>
            <Input
              id="project-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label className="text-[13px] text-muted-foreground">
              Project ID
            </Label>
            <p className="mono-data text-[12px] text-muted-foreground select-all">
              {project.id}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Button
              type="submit"
              size="sm"
              disabled={saving || !name.trim()}
            >
              {saving ? "Saving..." : saved ? "Saved" : "Save Changes"}
            </Button>
            {saved && (
              <span className="text-[12px] text-muted-foreground animate-fade-up">
                Changes saved
              </span>
            )}
          </div>
        </form>
      </div>

      <Separator className="my-6" />

      {/* Danger zone */}
      <div
        className="rounded-lg border border-destructive/20 p-5 animate-fade-up"
        style={{ animationDelay: "120ms" }}
      >
        <div className="flex items-center gap-2 mb-2">
          <AlertTriangle className="w-4 h-4 text-destructive" />
          <p className="text-[13px] font-medium text-destructive">
            Danger Zone
          </p>
        </div>
        <p className="text-[12px] text-muted-foreground mb-4">
          Deleting this project will permanently remove all API keys, connected
          accounts, and associated data. This action cannot be undone.
        </p>
        <Button
          variant="destructive"
          size="sm"
          onClick={handleDelete}
          disabled={deleting}
        >
          {deleting ? "Deleting..." : "Delete Project"}
        </Button>
      </div>
    </div>
  );
}
