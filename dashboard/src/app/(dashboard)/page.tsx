"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { listProjects, type Project } from "@/lib/api";
import { Plus, FolderOpen, ArrowRight } from "lucide-react";

export default function DashboardPage() {
  const { getToken } = useAuth();
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listProjects(token);
        setProjects(res.data);
      } catch (err) {
        console.error("Failed to load projects:", err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [getToken]);

  return (
    <div>
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Projects</h1>
          <p className="text-[13px] text-muted-foreground mt-1">
            Each project has its own API keys, accounts, and billing.
          </p>
        </div>
        <Link href="/projects/new">
          <Button size="sm" className="gap-1.5">
            <Plus className="w-3.5 h-3.5" />
            New Project
          </Button>
        </Link>
      </div>

      {loading ? (
        <div className="space-y-3">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-[72px] rounded-lg border border-border bg-muted/30 animate-pulse"
              style={{ animationDelay: `${i * 100}ms` }}
            />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="border border-dashed border-border rounded-lg py-16 flex flex-col items-center animate-fade-up">
          <div className="w-12 h-12 rounded-xl bg-muted flex items-center justify-center mb-4">
            <FolderOpen className="w-5 h-5 text-muted-foreground" />
          </div>
          <p className="text-[15px] font-medium mb-1">No projects yet</p>
          <p className="text-[13px] text-muted-foreground mb-5">
            Create your first project to get started with UniPost.
          </p>
          <Link href="/projects/new">
            <Button size="sm" className="gap-1.5">
              <Plus className="w-3.5 h-3.5" />
              Create Project
            </Button>
          </Link>
        </div>
      ) : (
        <div className="space-y-2">
          {projects.map((project, i) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className="group animate-fade-up"
              style={{ animationDelay: `${i * 60}ms` }}
            >
              <div className="card-hover flex items-center justify-between px-4 py-3.5 rounded-lg border border-border bg-card hover:border-foreground/15">
                <div className="flex items-center gap-3.5 min-w-0">
                  <div className="w-8 h-8 rounded-md bg-foreground/[0.04] border border-border flex items-center justify-center shrink-0">
                    <span className="text-xs font-semibold text-foreground/60">
                      {project.name.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div className="min-w-0">
                    <p className="text-[14px] font-medium truncate">
                      {project.name}
                    </p>
                    <p className="mono-data text-muted-foreground text-[11px] mt-0.5">
                      {project.id}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3">
                  <Badge variant="secondary" className="text-[11px] font-normal">
                    {project.mode}
                  </Badge>
                  <span className="mono-data text-[11px] text-muted-foreground">
                    {new Date(project.created_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <ArrowRight className="w-3.5 h-3.5 text-muted-foreground/40 group-hover:text-foreground/50 transition-colors" />
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
