import { createOgImage } from "../_components/og-image";
import { threads } from "../_config/platforms";

export const alt = "Threads API for Developers | UniPost";
export { size, contentType } from "../_components/og-image";
export default createOgImage(threads.name, threads.brandColor);
