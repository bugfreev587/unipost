// Thin analytics event wrapper. Today it's a no-op / console.info in
// dev — easy to swap in PostHog, Mixpanel, or Segment later without
// touching call sites.
//
// When wiring a real provider, replace the body of `track` and
// `identify` with the provider's SDK calls.

type EventProperties = Record<string, string | number | boolean | null | undefined>;

export function track(event: string, props?: EventProperties) {
  if (typeof window === "undefined") return;
  // Dev visibility — remove or gate behind an env flag when a real
  // analytics provider is wired in.
  if (process.env.NODE_ENV !== "production") {
    // eslint-disable-next-line no-console
    console.info("[analytics]", event, props || {});
  }
  // Future: window.posthog?.capture(event, props)
}

export function identify(userId: string, traits?: EventProperties) {
  if (typeof window === "undefined") return;
  if (process.env.NODE_ENV !== "production") {
    // eslint-disable-next-line no-console
    console.info("[analytics] identify", userId, traits || {});
  }
  // Future: window.posthog?.identify(userId, traits)
}
