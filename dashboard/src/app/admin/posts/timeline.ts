import type { AdminPostRow } from "@/lib/api";

export type AdminPostPublishTimeline = {
  label: "scheduled" | "published";
  at: string;
};

export function getAdminPostPublishTimeline(
  post: Pick<AdminPostRow, "status" | "scheduled_at" | "published_at">,
): AdminPostPublishTimeline | null {
  if (post.status === "scheduled" && post.scheduled_at) {
    return { label: "scheduled", at: post.scheduled_at };
  }
  if (post.status === "published" && post.published_at) {
    return { label: "published", at: post.published_at };
  }
  return null;
}

export function fmtAdminPostTimelineDate(iso: string) {
  return new Date(iso).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}
