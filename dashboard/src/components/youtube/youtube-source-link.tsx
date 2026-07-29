import type { MouseEventHandler } from "react";
import { ExternalLink } from "lucide-react";

import { normalizeYouTubeContentUrl } from "@/lib/youtube-source";
import styles from "./youtube-source.module.css";

type YouTubeSourceLinkProps = {
  href?: string | null;
  label: string;
  disclosure: "Source: YouTube" | "Data from YouTube" | "View on YouTube";
  onClick?: MouseEventHandler<HTMLAnchorElement>;
};

export function YouTubeSourceLink({
  href,
  label,
  disclosure,
  onClick,
}: YouTubeSourceLinkProps) {
  const safeHref = normalizeYouTubeContentUrl(href);
  if (!safeHref) return null;

  const accessibleLabel = disclosure === "View on YouTube"
    ? `View ${label} on YouTube`
    : label === "YouTube"
      ? "Open YouTube"
      : `Open ${label} on YouTube`;

  return (
    <a
      href={safeHref}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={accessibleLabel}
      data-youtube-source-link
      className={styles.sourceLink}
      onClick={onClick}
    >
      <span>{disclosure}</span>
      <ExternalLink size={14} strokeWidth={1.8} aria-hidden="true" />
    </a>
  );
}
