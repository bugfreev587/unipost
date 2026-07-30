import type { IntegrationLog } from "@/lib/api";

export function logDetailUnavailableMessage(
  log: Pick<IntegrationLog, "id" | "category" | "status" | "detail_status">,
): string | null {
  if (log.detail_status === "expired") {
    return "Detail payload expired. The structured log metadata remains available.";
  }
  if (typeof log.id === "string" && log.category === "api_request" && log.status === "success") {
    return "Detailed payloads are retained only for failed API requests.";
  }
  return null;
}

export function integrationLogMatchesSearch(
  log: Pick<
    IntegrationLog,
    "message" | "action" | "request_id" | "post_id" | "error_code" | "metadata"
  >,
  rawQuery: string,
): boolean {
  const exactQuery = rawQuery.trim();
  if (!exactQuery) return true;

  const metadata = log.metadata || {};
  const exactIDs = [metadata.connect_session_id, metadata.external_user_id]
    .filter((value): value is string => typeof value === "string");
  if (exactIDs.includes(exactQuery)) return true;

  const textQuery = exactQuery.toLowerCase();
  return [log.message, log.action, log.request_id, log.post_id, log.error_code]
    .filter((value): value is string => Boolean(value))
    .some((value) => value.toLowerCase().includes(textQuery));
}
