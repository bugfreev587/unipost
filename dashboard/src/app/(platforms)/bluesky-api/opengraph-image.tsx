import { createOgImage } from "../_components/og-image";
import { bluesky } from "../_config/platforms";

export const alt = "Bluesky API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(bluesky.name, bluesky.brandColor);
