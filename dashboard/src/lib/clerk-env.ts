const FRONTEND_CLERK_SECRET_WARNING =
  "[dashboard] CLERK_SECRET_KEY is set in the frontend runtime. Remove it from the dashboard/Vercel project and keep it only in the API service environment.";

declare global {
  // eslint-disable-next-line no-var
  var __unipostFrontendClerkSecretWarned: boolean | undefined;
}

export function warnIfFrontendHasClerkSecret() {
  if (typeof process === "undefined") {
    return;
  }
  if (!process.env.CLERK_SECRET_KEY || globalThis.__unipostFrontendClerkSecretWarned) {
    return;
  }

  globalThis.__unipostFrontendClerkSecretWarned = true;
  console.warn(FRONTEND_CLERK_SECRET_WARNING);
}
