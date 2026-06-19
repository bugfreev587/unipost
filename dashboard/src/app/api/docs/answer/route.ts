import { generateText, gateway, type GatewayModelId } from "ai";
import { NextResponse } from "next/server";
import {
  buildGroundedDocsAnswer,
  searchDocsIndex,
  type GroundedDocsAnswer,
} from "@/lib/docs-ai-search-index";

export const runtime = "nodejs";

const MAX_QUERY_LENGTH = 500;
const RATE_LIMIT_WINDOW_MS = 60_000;
const RATE_LIMIT_MAX_REQUESTS = 30;
const DOCS_AI_MODEL = (process.env.DOCS_AI_MODEL || "anthropic/claude-sonnet-4.5") as GatewayModelId;
const SOURCE_COVERAGE_FALLBACK = "not enough source coverage";
const rateLimitBuckets = new Map<string, { count: number; resetAt: number }>();

type DocsAnswerRequest = {
  query?: unknown;
};

function badRequest(message: string) {
  return NextResponse.json({ error: { code: "bad_request", message } }, { status: 400 });
}

function rateLimited() {
  return NextResponse.json(
    { error: { code: "rate_limited", message: "Too many docs AI search requests. Try again shortly." } },
    { status: 429 },
  );
}

function logAnswer(query: string, answer: GroundedDocsAnswer) {
  console.info("docs_ai_answer", {
    query,
    query_length: query.length,
    confidence: answer.confidence,
    generated_by: answer.generated_by,
    coverage_reason: answer.coverage_reason,
    source_ids: answer.sources.map((source) => source.id),
    source_paths: answer.sources.map((source) => source.path),
    related_ids: answer.related.map((source) => source.id),
    related_paths: answer.related.map((source) => source.path),
    recorded_at: new Date().toISOString(),
  });
}

function answerResponse(query: string, answer: GroundedDocsAnswer) {
  logAnswer(query, answer);
  return NextResponse.json({
    data: answer,
  });
}

function clientKey(request: Request) {
  return request.headers.get("x-forwarded-for")?.split(",")[0]?.trim()
    || request.headers.get("x-real-ip")?.trim()
    || "anonymous";
}

function isRateLimited(request: Request) {
  const key = clientKey(request);
  const now = Date.now();
  const bucket = rateLimitBuckets.get(key);

  if (!bucket || bucket.resetAt <= now) {
    rateLimitBuckets.set(key, { count: 1, resetAt: now + RATE_LIMIT_WINDOW_MS });
    return false;
  }

  bucket.count += 1;
  return bucket.count > RATE_LIMIT_MAX_REQUESTS;
}

function sourcePrompt(answer: GroundedDocsAnswer) {
  return answer.sources
    .map((source, index) => {
      return [
        `Source ${index + 1}: ${source.title}`,
        `Path: ${source.path}`,
        `Section: ${source.section_title}`,
        `Excerpt: ${source.excerpt}`,
      ].join("\n");
    })
    .join("\n\n");
}

async function refineWithAi(query: string, groundedAnswer: GroundedDocsAnswer) {
  if (groundedAnswer.sources.length === 0 || groundedAnswer.confidence === "none") {
    return groundedAnswer;
  }

  try {
    const result = await generateText({
      model: gateway(DOCS_AI_MODEL),
      system:
        `You answer UniPost documentation questions. Use only the provided sources and deterministic draft. Do not add facts, endpoints, scopes, fields, or platform support claims that are not present in the sources. If the sources do not answer the question, say the docs have ${SOURCE_COVERAGE_FALLBACK}.`,
      prompt: [
        `Question: ${query}`,
        "",
        "Deterministic draft:",
        groundedAnswer.answer,
        "",
        "Steps:",
        groundedAnswer.steps.map((step, index) => `${index + 1}. ${step}`).join("\n") || "None",
        "",
        "Sources:",
        sourcePrompt(groundedAnswer),
        "",
        "Rewrite the answer in 2-5 concise sentences. Preserve exact endpoint paths, scope names, and response fields.",
      ].join("\n"),
      temperature: 0,
      maxOutputTokens: 420,
    });

    const text = result.text.trim();
    if (!text) return groundedAnswer;

    return {
      ...groundedAnswer,
      answer: text,
      generated_by: "ai" as const,
    };
  } catch (error) {
    console.info("docs_ai_answer_fallback", {
      message: error instanceof Error ? error.message : String(error),
      model: DOCS_AI_MODEL,
    });
    return groundedAnswer;
  }
}

export async function POST(request: Request) {
  if (isRateLimited(request)) {
    return rateLimited();
  }

  let body: DocsAnswerRequest;

  try {
    body = await request.json();
  } catch {
    return badRequest("Request body must be valid JSON.");
  }

  const query = typeof body.query === "string" ? body.query.trim() : "";
  if (!query) {
    return badRequest("query is required.");
  }

  if (query.length > MAX_QUERY_LENGTH) {
    return badRequest(`query must be ${MAX_QUERY_LENGTH} characters or fewer.`);
  }

  const search = searchDocsIndex(query, { limit: 5 });
  const groundedAnswer = buildGroundedDocsAnswer(query, search);

  if (groundedAnswer.confidence === "none") {
    return answerResponse(query, groundedAnswer);
  }

  return answerResponse(query, await refineWithAi(query, groundedAnswer));
}
