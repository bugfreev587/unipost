"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import { listProjects, type Project } from "@/lib/api";
import { Plus, FolderOpen, ChevronRight } from "lucide-react";

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
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 32 }}>
        <div>
          <div style={{ fontSize: 24, fontWeight: 700, letterSpacing: -0.5, color: "var(--dtext)" }}>Projects</div>
          <div style={{ fontSize: 14, color: "#aaa", marginTop: 6 }}>Manage your UniPost projects and API integrations.</div>
        </div>
        <Link href="/projects/new" className="dbtn dbtn-primary">
          <Plus style={{ width: 13, height: 13 }} /> New Project
        </Link>
      </div>

      {loading ? (
        <div className="table-wrap">
          {[1, 2, 3].map((i) => (
            <div key={i} style={{ height: 52, background: i % 2 === 0 ? "var(--surface2)" : "transparent" }} />
          ))}
        </div>
      ) : projects.length === 0 ? (
        <div className="empty-state">
          <FolderOpen style={{ width: 32, height: 32, opacity: 0.4, marginBottom: 12 }} />
          <div style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)", marginBottom: 6 }}>No projects yet</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", maxWidth: 280, lineHeight: 1.6 }}>
            Create your first project to start using the UniPost API.
          </div>
          <Link href="/projects/new" className="dbtn dbtn-primary" style={{ marginTop: 16 }}>
            <Plus style={{ width: 13, height: 13 }} /> Create Project
          </Link>
        </div>
      ) : (
        <div className="table-wrap">
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>ID</th>
                <th>Mode</th>
                <th>Created</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {projects.map((project) => (
                <tr
                  key={project.id}
                  onClick={() => (window.location.href = `/projects/${project.id}`)}
                  style={{ cursor: "pointer" }}
                >
                  <td style={{ fontWeight: 500 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
                      <div className="project-initial">
                        {project.name.charAt(0).toUpperCase()}
                      </div>
                      {project.name}
                    </div>
                  </td>
                  <td><span className="mono">{project.id.slice(0, 12)}</span></td>
                  <td>
                    <span className="dbadge dbadge-green">
                      <span className="dbadge-dot" />
                      {project.mode}
                    </span>
                  </td>
                  <td style={{ color: "var(--dmuted)" }}>
                    {new Date(project.created_at).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })}
                  </td>
                  <td style={{ textAlign: "right" }}>
                    <ChevronRight style={{ width: 14, height: 14, color: "var(--dmuted2)" }} />
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
