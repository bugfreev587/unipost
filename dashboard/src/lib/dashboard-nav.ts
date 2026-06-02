export type ProjectNavMatchOptions = {
  exactMatch?: boolean;
};

export function buildProjectNavHref(profileId: string, itemHref: string) {
  return `/projects/${profileId}${itemHref}`;
}

export function projectNavRouteIsActive(
  pathname: string,
  profileId: string | undefined,
  itemHref: string,
  options: ProjectNavMatchOptions = {},
) {
  if (!profileId) return false;
  const fullHref = buildProjectNavHref(profileId, itemHref);
  if (options.exactMatch) return pathname === fullHref;
  return pathname === fullHref || pathname.startsWith(`${fullHref}/`);
}

export function projectNavItemIsActive(
  pathname: string,
  profileId: string | undefined,
  itemHref: string,
  options: ProjectNavMatchOptions = {},
) {
  return projectNavRouteIsActive(pathname, profileId, itemHref, options);
}

export function projectNavSubItemIsActive(
  pathname: string,
  profileId: string | undefined,
  itemHref: string,
  options: ProjectNavMatchOptions = {},
) {
  return projectNavRouteIsActive(pathname, profileId, itemHref, options);
}
