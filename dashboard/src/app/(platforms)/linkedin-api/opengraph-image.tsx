import { createOgImage } from "../_components/og-image";
import { linkedin } from "../_config/platforms";

export const alt = "LinkedIn API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(linkedin.name, linkedin.brandColor);
