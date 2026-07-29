import { PlatformIcon } from "@/components/platform-icons";
import { YouTubeBrandIcon } from "@/components/public/youtube-brand-icon";

type PublicPlatformIconProps = {
  platform: string;
  size?: number;
};

export function PublicPlatformIcon({ platform, size = 14 }: PublicPlatformIconProps) {
  if (platform === "youtube") return <YouTubeBrandIcon size={size} />;
  return <PlatformIcon platform={platform} size={size} />;
}
