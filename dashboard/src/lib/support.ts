export const SUPPORT_EMAIL = process.env.NEXT_PUBLIC_SUPPORT_EMAIL || "support@unipost.dev";
export const SUPPORT_SLACK_URL = process.env.NEXT_PUBLIC_SUPPORT_SLACK_URL || "";

type SupportEmailOptions = {
  subject: string;
  intro?: string;
  details?: Array<string | false | null | undefined>;
};

export function buildSupportMailto({
  subject,
  intro = "I need help with UniPost.",
  details = [],
}: SupportEmailOptions): string {
  const contextLines = details.filter(Boolean) as string[];
  const body = [
    "Hello UniPost team,",
    "",
    intro,
    "",
    ...(contextLines.length > 0 ? ["Context:", ...contextLines.map((line) => `- ${line}`), ""] : []),
    "What I expected:",
    "",
    "What happened instead:",
    "",
  ].join("\n");

  return `mailto:${SUPPORT_EMAIL}?subject=${encodeURIComponent(`[UniPost] ${subject}`)}&body=${encodeURIComponent(body)}`;
}

export function buildContactPageHref(params: Record<string, string | undefined>): string {
  const search = new URLSearchParams();
  Object.entries(params).forEach(([key, value]) => {
    if (value) search.set(key, value);
  });
  const query = search.toString();
  return query ? `/contact?${query}` : "/contact";
}
