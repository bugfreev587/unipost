import { NextRequest, NextResponse } from "next/server";

const REVIEW_COOKIE = "__unipost_review_session";

export function GET(request: NextRequest) {
  const url = new URL(request.url);
  const token = url.searchParams.get("token")?.trim() || "";
  const redirectURL = new URL("/tiktok/posting", url.origin);

  const response = NextResponse.redirect(redirectURL);
  if (token) {
    response.cookies.set(REVIEW_COOKIE, token, {
      httpOnly: true,
      maxAge: 30 * 60,
      path: "/",
      sameSite: "lax",
      secure: true,
    });
  }
  return response;
}
