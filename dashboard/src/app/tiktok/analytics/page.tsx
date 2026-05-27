import { cookies } from "next/headers";
import { TikTokReviewAnalyticsClient, type ReviewAnalyticsSession } from "./tiktok-review-analytics-client";

const API_URL = process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev";
const REVIEW_COOKIE = "__unipost_review_session";

type ApiEnvelope<T> = { data?: T; error?: { code: string; message: string } };

type PageProps = {
  searchParams: Promise<{ connect_status?: string }>;
};

export default async function TikTokReviewAnalyticsPage({ searchParams }: PageProps) {
  const { connect_status = "" } = await searchParams;
  const cookieStore = await cookies();
  const sessionToken = cookieStore.get(REVIEW_COOKIE)?.value || "";
  const { session, error } = await loadReviewSession(sessionToken);

  return (
    <TikTokReviewAnalyticsClient
      session={session}
      error={error}
      initiallyConnected={connect_status === "success"}
    />
  );
}

async function loadReviewSession(token: string): Promise<{ session: ReviewAnalyticsSession | null; error: string }> {
  if (!token) {
    return { session: null, error: "No active review session." };
  }
  try {
    const res = await fetch(`${API_URL}/v1/review/session`, {
      cache: "no-store",
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${token}`,
      },
    });
    const body: ApiEnvelope<ReviewAnalyticsSession> = await res.json();
    if (!res.ok || !body.data) {
      return { session: null, error: body.error?.message || "Review session is unavailable." };
    }
    return { session: body.data, error: "" };
  } catch {
    return { session: null, error: "Couldn't reach the UniPost review service." };
  }
}
