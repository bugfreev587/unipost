"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { createProject } from "@/lib/api";
import { ArrowLeft } from "lucide-react";
import Link from "next/link";

export default function NewProjectPage() {
  const { getToken } = useAuth();
  const router = useRouter();
  const [name, setName] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;

    setLoading(true);
    setError("");

    try {
      const token = await getToken();
      if (!token) return;
      const res = await createProject(token, { name: name.trim() });
      router.push(`/projects/${res.data.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create project");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="max-w-md mx-auto pt-8">
      <Link
        href="/"
        className="inline-flex items-center gap-1.5 text-[13px] text-muted-foreground hover:text-foreground transition-colors mb-6"
      >
        <ArrowLeft className="w-3.5 h-3.5" />
        Back to Projects
      </Link>

      <div className="rounded-lg border border-border bg-card p-6 animate-fade-up">
        <div className="mb-5">
          <h1 className="text-lg font-semibold tracking-tight">
            New Project
          </h1>
          <p className="text-[13px] text-muted-foreground mt-1">
            Each project has its own API keys, accounts, and billing.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label htmlFor="name" className="text-[13px]">
              Project Name
            </Label>
            <Input
              id="name"
              placeholder="My App"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
              required
            />
          </div>
          {error && (
            <p className="text-[12px] text-destructive">{error}</p>
          )}
          <div className="flex gap-2 pt-1">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => router.back()}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              size="sm"
              disabled={loading || !name.trim()}
            >
              {loading ? "Creating..." : "Create Project"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
