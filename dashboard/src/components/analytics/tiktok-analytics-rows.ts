import type { ApiResponse, PostAnalytics, SocialPost } from "@/lib/api";

export type TikTokPostRow = {
  title: string;
  status: string;
  videoId: string;
  views: number | null;
  likes: number | null;
  comments: number | null;
  shares: number | null;
};

export function buildTikTokPostRows(
  posts: SocialPost[],
  analyticsSettled: PromiseSettledResult<ApiResponse<PostAnalytics[] | null>>[],
  accountId: string
): TikTokPostRow[] {
  return posts.map((post, index) => {
    const result = post.results?.find((item) => item.social_account_id === accountId);
    const analytics = analyticsSettled[index];
    const analyticsRows = analytics?.status === "fulfilled" && Array.isArray(analytics.value.data)
      ? analytics.value.data
      : [];
    const row = analyticsRows.find((item) => item.social_account_id === accountId);
    const matchedVideoID = row?.platform_specific?.tiktok_video_id;
    const matched = typeof matchedVideoID === "string" && matchedVideoID.length > 0;

    return {
      title: post.caption || "Untitled TikTok post",
      status: result?.status || post.status,
      videoId: String(matchedVideoID || result?.external_id || row?.external_id || "-"),
      views: matched ? row?.video_views ?? 0 : null,
      likes: matched ? row?.likes ?? 0 : null,
      comments: matched ? row?.comments ?? 0 : null,
      shares: matched ? row?.shares ?? 0 : null,
    };
  });
}
