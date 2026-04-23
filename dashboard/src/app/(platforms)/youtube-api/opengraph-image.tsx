import { createOgImage } from "../_components/og-image";
import { youtube } from "../_config/platforms";

export const alt = "YouTube API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(youtube.name, youtube.brandColor);
