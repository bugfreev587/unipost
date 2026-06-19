import { NextResponse } from "next/server";

export const runtime = "nodejs";

const RATINGS = new Set(["helpful", "not_helpful", "missing_docs"]);
const CONFIDENCE = new Set(["high", "medium", "low", "none"]);
const GENERATED_BY = new Set(["ai", "extractive", "fallback"]);

type DocsAiFeedbackRequest = {
  query?: unknown;
  rating?: unknown;
  path?: unknown;
  sources?: unknown;
  related?: unknown;
  confidence?: unknown;
  generated_by?: unknown;
  coverage_reason?: unknown;
};

function badRequest(message: string) {
  return NextResponse.json({ error: { code: "bad_request", message } }, { status: 400 });
}

export async function POST(request: Request) {
  let body: DocsAiFeedbackRequest;

  try {
    body = await request.json();
  } catch {
    return badRequest("Request body must be valid JSON.");
  }

  const query = typeof body.query === "string" ? body.query.trim() : "";
  const rating = typeof body.rating === "string" ? body.rating : "";
  const path = typeof body.path === "string" ? body.path : undefined;
  const sources = Array.isArray(body.sources)
    ? body.sources.filter((source): source is string => typeof source === "string").slice(0, 5)
    : [];
  const related = Array.isArray(body.related)
    ? body.related.filter((source): source is string => typeof source === "string").slice(0, 5)
    : [];
  const confidence = typeof body.confidence === "string" && CONFIDENCE.has(body.confidence)
    ? body.confidence
    : undefined;
  const generated_by = typeof body.generated_by === "string" && GENERATED_BY.has(body.generated_by)
    ? body.generated_by
    : undefined;
  const coverage_reason = typeof body.coverage_reason === "string" ? body.coverage_reason.slice(0, 160) : undefined;

  if (!query) {
    return badRequest("query is required.");
  }

  if (!RATINGS.has(rating)) {
    return badRequest("rating must be helpful, not_helpful, or missing_docs.");
  }

  console.info("docs_ai_feedback", {
    query,
    rating,
    path,
    sources,
    related,
    confidence,
    generated_by,
    coverage_reason,
    recorded_at: new Date().toISOString(),
  });

  return NextResponse.json({ data: { ok: true } });
}
