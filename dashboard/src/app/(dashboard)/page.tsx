"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { listProjects, type Project } from "@/lib/api";
import { Plus, FolderOpen, ArrowUpRight } from "lucide-react";

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
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-[18px] font-semibold text-[#e5e5e5] tracking-tight">
            Projects
          </h1>
          <p className="text-[13px] text-[#525252] mt-0.5">
            Manage your UniPost projects and API integrations.
          </p>
        </div>
        <Link href="/projects/new">
          <Button size="sm" className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90">
            <Plus className="w-3.5 h-3.5" />
            New Project
          </Button>
        </Link>
      </div>

      {loading ? (
        <div className="space-y-2">
          {[1, 2, 3].map((i) => (
            <div
              key={i}
              className="h-[60px] rounded-lg bg-[#111111] border border-[#1e1e1e] animate-pulse"
              style={{ animationDelay: `${i * 80}ms` }}
            />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="border border-dashed border-[#1e1e1e] rounded-lg py-20 flex flex-col items-center animate-enter">
          <div className="w-12 h-12 rounded-xl bg-[#111111] border border-[#1e1e1e] flex items-center justify-center mb-4">
            <FolderOpen className="w-5 h-5 text-[#525252]" />
          </div>
          <p className="text-[14px] font-medium text-[#d4d4d4] mb-1">
            No projects yet
          </p>
          <p className="text-[13px] text-[#525252] mb-6">
            Create your first project to start using the UniPost API.
          </p>
          <Link href="/projects/new">
            <Button size="sm" className="gap-1.5 bg-emerald text-emerald-foreground hover:bg-emerald/90">
              <Plus className="w-3.5 h-3.5" />
              Create Project
            </Button>
          </Link>
        </div>
      ) : (
        <div className="space-y-1">
          {projects.map((project, i) => (
            <Link
              key={project.id}
              href={`/projects/${project.id}`}
              className="group block animate-enter"
              style={{ animationDelay: `${i * 50}ms` }}
            >
              <div className="flex items-center justify-between px-4 py-3 rounded-lg bg-[#111111] border border-[#1e1e1e] hover:border-[#2a2a2a] transition-colors">
                <div className="flex items-center gap-3.5 min-w-0">
                  <div className="w-8 h-8 rounded-md bg-[#0a0a0a] border border-[#1e1e1e] flex items-center justify-center shrink-0">
                    <span className="text-[11px] font-bold text-[#525252]">
                      {project.name.charAt(0).toUpperCase()}
                    </span>
                  </div>
                  <div className="min-w-0">
                    <p className="text-[13px] font-medium text-[#e5e5e5] truncate">
                      {project.name}
                    </p>
                    <p className="mono text-[11px] text-[#3a3a3a] mt-0.5 truncate">
                      {project.id}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-3 shrink-0">
                  <Badge variant="secondary" className="text-[10px] bg-[#1a1a1a] text-[#737373] border-0">
                    {project.mode}
                  </Badge>
                  <span className="mono text-[11px] text-[#3a3a3a]">
                    {new Date(project.created_at).toLocaleDateString("en-US", {
                      month: "short",
                      day: "numeric",
                    })}
                  </span>
                  <ArrowUpRight className="w-3.5 h-3.5 text-[#2a2a2a] group-hover:text-[#525252] transition-colors" />
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
