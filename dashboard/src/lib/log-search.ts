import type { IntegrationLog } from "@/lib/api";

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
