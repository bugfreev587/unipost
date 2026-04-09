export { UNIPOST } from "./unipost";
export { AYRSHARE } from "./ayrshare";
export { ZERNIO } from "./zernio";
export { POSTFORME } from "./postforme";

import { AYRSHARE } from "./ayrshare";
import { ZERNIO } from "./zernio";
import { POSTFORME } from "./postforme";

export const ALL_COMPETITORS = [AYRSHARE, ZERNIO, POSTFORME] as const;

export type Competitor = typeof AYRSHARE;

export function getCompetitorBySlug(slug: string): Competitor | undefined {
  return ALL_COMPETITORS.find((c) => c.slug === slug);
}
