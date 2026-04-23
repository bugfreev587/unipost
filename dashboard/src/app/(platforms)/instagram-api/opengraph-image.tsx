import { createOgImage } from "../_components/og-image";
import { instagram } from "../_config/platforms";

export const alt = "Instagram API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(instagram.name, instagram.brandColor);
