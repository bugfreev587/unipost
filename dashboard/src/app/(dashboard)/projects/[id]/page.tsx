"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { useAuth } from "@clerk/nextjs";
import Link from "next/link";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { getProject, type Project } from "@/lib/api";

export default function ProjectOverviewPage() {
  const { id } = useParams<{ id: string }>();
  const { getToken } = useAuth();
  const [project, setProject] = useState<Project | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getProject(token, id);
        setProject(res.data);
      } catch (err) {
        console.error("Failed to load project:", err);
      } finally {
        setLoading(false);
      }
    }
    load();
  }, [getToken, id]);

  if (loading) {
    return <div className="text-muted-foreground">Loading...</div>;
  }

  if (!project) {
    return <div className="text-destructive">Project not found</div>;
  }

  return (
    <div>
      <h1 className="text-2xl font-bold mb-2">{project.name}</h1>
      <p className="text-muted-foreground mb-8">
        {project.mode} &middot; Created{" "}
        {new Date(project.created_at).toLocaleDateString()}
      </p>

      <div className="grid grid-cols-2 gap-4">
        <Link href={`/projects/${id}/api-keys`}>
          <Card className="hover:border-primary/50 transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle className="text-base">API Keys</CardTitle>
              <CardDescription>Manage your API keys</CardDescription>
            </CardHeader>
          </Card>
        </Link>
        <Link href={`/projects/${id}/accounts`}>
          <Card className="hover:border-primary/50 transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle className="text-base">Accounts</CardTitle>
              <CardDescription>Connected social accounts</CardDescription>
            </CardHeader>
          </Card>
        </Link>
        <Link href={`/projects/${id}/posts`}>
          <Card className="hover:border-primary/50 transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle className="text-base">Posts</CardTitle>
              <CardDescription>Send and manage posts</CardDescription>
            </CardHeader>
          </Card>
        </Link>
        <Link href={`/projects/${id}/billing`}>
          <Card className="hover:border-primary/50 transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle className="text-base">Billing</CardTitle>
              <CardDescription>Plan and usage</CardDescription>
            </CardHeader>
          </Card>
        </Link>
        <Link href={`/projects/${id}/settings`}>
          <Card className="hover:border-primary/50 transition-colors cursor-pointer">
            <CardHeader>
              <CardTitle className="text-base">Settings</CardTitle>
              <CardDescription>Project settings</CardDescription>
            </CardHeader>
          </Card>
        </Link>
      </div>
    </div>
  );
}
