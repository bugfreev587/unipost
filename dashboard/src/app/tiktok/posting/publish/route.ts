import { cookies } from "next/headers";
import { NextResponse } from "next/server";

const API_URL = (process.env.NEXT_PUBLIC_API_URL || "https://api.unipost.dev").replace(/\/+$/, "");
const REVIEW_COOKIE = "__unipost_review_session";

export async function POST(request: Request) {
  const cookieStore = await cookies();
  const sessionToken = cookieStore.get(REVIEW_COOKIE)?.value || "";

  if (!sessionToken) {
    return NextResponse.json(
      { error: { code: "UNAUTHORIZED", message: "No active review session." } },
      { status: 401 },
    );
  }

  let payload: unknown = {};
  try {
    payload = await request.json();
  } catch {
    payload = {};
  }

  const upstream = await fetch(`${API_URL}/v1/review/session/tiktok/publish`, {
    method: "POST",
    cache: "no-store",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      Authorization: `Bearer ${sessionToken}`,
    },
    body: JSON.stringify(payload),
  });

  const text = await upstream.text();
  return new NextResponse(text, {
    status: upstream.status,
    headers: {
      "Content-Type": upstream.headers.get("Content-Type") || "application/json",
    },
  });
}
