import { createOgImage } from "../_components/og-image";
import { pinterest } from "../_config/platforms";

export const alt = "Pinterest API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(pinterest.name, pinterest.brandColor);
