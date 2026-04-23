import { createOgImage } from "../_components/og-image";
import { twitter } from "../_config/platforms";

export const alt = "Twitter API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(twitter.name, twitter.brandColor);
