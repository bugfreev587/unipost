type ProfileRouteEntry = {
  id: string;
};

const PROJECT_PROFILE_ROUTE_RE = /^\/projects\/([^/]+)(.*)$/;

export function getCanonicalProjectPath({
  pathname,
  profiles,
}: {
  pathname: string;
  profiles: ProfileRouteEntry[];
}): string | null {
  const match = pathname.match(PROJECT_PROFILE_ROUTE_RE);
  if (!match) return null;

  const [, profileId, suffix = ""] = match;
  if (!profileId || profileId === "new") return null;
  if (profiles.length === 0) return null;
  if (profiles.some((profile) => profile.id === profileId)) return null;

  return `/projects/${profiles[0].id}${suffix}`;
}
