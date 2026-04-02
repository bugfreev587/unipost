"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { getProject, updateProject, deleteProject, type Project } from "@/lib/api";

export default function SettingsPage() {
  const { id: projectId } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const router = useRouter();
  const [project, setProject] = useState<Project | null>(null);
  const [name, setName] = useState("");
  const [saving, setSaving] = useState(false);
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

    try {
      const token = await getToken();
      if (!token) return;
      const res = await updateProject(token, projectId, { name: name.trim() });
      setProject(res.data);
    } catch (err) {
      console.error("Failed to update project:", err);
    } finally {
      setSaving(false);
    }
  }

  async function handleDelete() {
    if (!confirm("Are you sure you want to delete this project? All API keys and data will be permanently deleted.")) {
      return;
    }
    setDeleting(true);

    try {
      const token = await getToken();
      if (!token) return;
      await deleteProject(token, projectId);
      router.push("/dashboard");
    } catch (err) {
      console.error("Failed to delete project:", err);
    } finally {
      setDeleting(false);
    }
  }

  if (!project) {
    return <div className="text-muted-foreground">Loading...</div>;
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold mb-6">Settings</h1>

      <Card className="mb-6">
        <CardHeader>
          <CardTitle className="text-base">General</CardTitle>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSave} className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="project-name">Project Name</Label>
              <Input
                id="project-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </div>
            <Button type="submit" disabled={saving || !name.trim()}>
              {saving ? "Saving..." : "Save"}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Separator className="my-6" />

      <Card className="border-destructive/50">
        <CardHeader>
          <CardTitle className="text-base text-destructive">Danger Zone</CardTitle>
          <CardDescription>
            Deleting this project will permanently remove all API keys and associated data.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Button
            variant="destructive"
            onClick={handleDelete}
            disabled={deleting}
          >
            {deleting ? "Deleting..." : "Delete Project"}
          </Button>
        </CardContent>
      </Card>
    </div>
  );
}
