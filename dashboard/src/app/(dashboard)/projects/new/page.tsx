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
    <div className="max-w-[420px] mx-auto pt-8">
      <Link
        href="/"
        className="inline-flex items-center gap-1.5 text-[12px] text-[#3a3a3a] hover:text-[#737373] transition-colors mb-6"
      >
        <ArrowLeft className="w-3 h-3" />
        Back to Projects
      </Link>

      <div className="rounded-lg bg-[#111111] border border-[#1e1e1e] p-6 animate-enter">
        <div className="mb-5">
          <h1 className="text-[16px] font-semibold text-[#e5e5e5] tracking-tight">
            New Project
          </h1>
          <p className="text-[13px] text-[#525252] mt-1">
            Each project has isolated API keys, accounts, and billing.
          </p>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="space-y-1.5">
            <Label className="text-[12px] text-[#a3a3a3]">Project Name</Label>
            <Input
              placeholder="My App"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
              required
              className="bg-[#0a0a0a] border-[#1e1e1e]"
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
              className="border-[#1e1e1e] text-[#737373] hover:text-[#e5e5e5] hover:border-[#2a2a2a]"
            >
              Cancel
            </Button>
            <Button
              type="submit"
              size="sm"
              disabled={loading || !name.trim()}
              className="bg-emerald text-emerald-foreground hover:bg-emerald/90"
            >
              {loading ? "Creating..." : "Create Project"}
            </Button>
          </div>
        </form>
      </div>
    </div>
  );
}
