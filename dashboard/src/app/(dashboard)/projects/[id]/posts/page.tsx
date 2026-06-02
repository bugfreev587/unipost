"use client";

import { PostsCalendarView } from "@/components/posts/calendar/posts-calendar-view";
import { PostsLegacyListView } from "@/components/posts/list/posts-legacy-list-view";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";
import { useFeatureFlags } from "@/lib/use-feature-flags";

export default function PostsPage() {
  const { flags, loading } = useFeatureFlags();

  if (loading) return <PostsLegacyListView showCalendarLink={false} />;
  if (!flags[FEATURE_FLAG_KEYS.postsCalendarViewV1]) return <PostsLegacyListView showCalendarLink={false} />;

  return <PostsCalendarView />;
}
