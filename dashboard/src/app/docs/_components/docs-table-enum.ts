export type DocsEnumTone =
  | "success"
  | "warning"
  | "danger"
  | "info"
  | "neutral"
  | "caution";

const ENUM_TONES: Readonly<Record<string, Readonly<Record<string, DocsEnumTone>>>> = {
  support: {
    yes: "success",
    no: "danger",
    partial: "warning",
    limited: "warning",
  },
  available: {
    yes: "success",
    no: "danger",
  },
  required: {
    yes: "success",
    no: "danger",
    required: "info",
    optional: "neutral",
    rejected: "danger",
  },
  severity: {
    critical: "danger",
    high: "caution",
    medium: "warning",
  },
  "default on": {
    yes: "success",
    no: "danger",
  },
  "use this page?": {
    yes: "success",
    no: "danger",
    partially: "warning",
  },
  "unipost status": {
    supported: "success",
    "coming soon": "warning",
  },
};

function normalize(value: string) {
  return value.trim().toLowerCase();
}

export function resolveDocsTableEnumTone(column: string, value: string): DocsEnumTone | null {
  return ENUM_TONES[normalize(column)]?.[normalize(value)] ?? null;
}
