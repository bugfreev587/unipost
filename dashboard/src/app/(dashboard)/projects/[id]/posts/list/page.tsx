"use client";

import { useEffect } from "react";
import { useParams, useRouter } from "next/navigation";
import { PostsLegacyListView } from "@/components/posts/list/posts-legacy-list-view";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";
import { useFeatureFlags } from "@/lib/use-feature-flags";

export default function PostsListPage() {
  const params = useParams<{ id: string }>();
  const router = useRouter();
  const { flags, loading } = useFeatureFlags();
  const enabled = Boolean(flags[FEATURE_FLAG_KEYS.postsCalendarViewV1]);

  useEffect(() => {
    if (!loading && !enabled) router.replace(`/projects/${params.id}/posts`);
  }, [enabled, loading, params.id, router]);

  if (loading || !enabled) return null;
  return <PostsLegacyListView showCalendarLink />;
}
