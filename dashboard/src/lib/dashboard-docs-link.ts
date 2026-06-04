const PRODUCTION_LANDING_ORIGIN = "https://unipost.dev";
const DEFAULT_DASHBOARD_DOCS_PATH = "/docs/quickstart";

const APP_ORIGIN_TO_LANDING_ORIGIN: Record<string, string> = {
  "https://app.unipost.dev": PRODUCTION_LANDING_ORIGIN,
  "https://dev-app.unipost.dev": "https://dev.unipost.dev",
  "https://staging-app.unipost.dev": "https://staging.unipost.dev",
};

type DashboardDocsHrefOptions = {
  landingUrl?: string;
  baseUrl?: string;
  appUrl?: string;
  currentOrigin?: string;
  path?: string;
};

function normalizeOrigin(value: string | undefined): string | undefined {
  if (!value?.trim()) return undefined;

  try {
    return new URL(value).origin;
  } catch {
    return undefined;
  }
}

function normalizePath(path: string | undefined): string {
  if (!path?.trim()) return DEFAULT_DASHBOARD_DOCS_PATH;
  return path.startsWith("/") ? path : `/${path}`;
}

function toLandingOrigin(value: string | undefined): string | undefined {
  const origin = normalizeOrigin(value);
  if (!origin) return undefined;
  return APP_ORIGIN_TO_LANDING_ORIGIN[origin] ?? origin;
}

export function getDashboardDocsHref(options: DashboardDocsHrefOptions = {}) {
  const path = normalizePath(options.path);
  const landingOrigin =
    toLandingOrigin(options.landingUrl ?? process.env.NEXT_PUBLIC_LANDING_URL) ??
    toLandingOrigin(options.appUrl ?? process.env.NEXT_PUBLIC_APP_URL) ??
    toLandingOrigin(options.currentOrigin) ??
    toLandingOrigin(options.baseUrl ?? process.env.NEXT_PUBLIC_BASE_URL) ??
    PRODUCTION_LANDING_ORIGIN;

  return `${landingOrigin}${path}`;
}
