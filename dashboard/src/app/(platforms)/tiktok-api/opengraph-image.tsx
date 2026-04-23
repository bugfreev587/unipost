import { createOgImage } from "../_components/og-image";
import { tiktok } from "../_config/platforms";

export const alt = "TikTok API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(tiktok.name, tiktok.brandColor);
