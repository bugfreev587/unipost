import type { ApiResponse, PostAnalytics, SocialPost } from "@/lib/api";

export type TikTokPostRow = {
  title: string;
  status: string;
  videoId: string;
  views: number;
  likes: number;
  comments: number;
  shares: number;
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

    return {
      title: post.caption || "Untitled TikTok post",
      status: result?.status || post.status,
      videoId: String(row?.platform_specific?.tiktok_video_id || result?.external_id || row?.external_id || "-"),
      views: row?.video_views || 0,
      likes: row?.likes || 0,
      comments: row?.comments || 0,
      shares: row?.shares || 0,
    };
  });
}
