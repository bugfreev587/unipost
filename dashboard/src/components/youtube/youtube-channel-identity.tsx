"use client";

import { useEffect, useState } from "react";

import { AccountDestinationIcon } from "@/components/account-destination-icon";
import type { SocialAccount } from "@/lib/api";
import {
  buildYouTubeChannelUrl,
  getYouTubeIdentityInitials,
  isDisconnectedYouTubeAccount,
  YOUTUBE_HOME_URL,
} from "@/lib/youtube-source";
import { YouTubeSourceLink } from "./youtube-source-link";
import styles from "./youtube-source.module.css";

type YouTubeChannelIdentityProps = {
  account: Pick<
    SocialAccount,
    "id" | "account_name" | "account_avatar_url" | "external_account_id" | "status"
  >;
  compact?: boolean;
  disclosure?: "Source: YouTube" | "Data from YouTube";
};

const warnedHomeFallbackAccountIds = new Set<string>();

export function YouTubeChannelIdentity({
  account,
  compact = false,
  disclosure = "Source: YouTube",
}: YouTubeChannelIdentityProps) {
  const [failedAvatarUrl, setFailedAvatarUrl] = useState("");
  const disconnected = isDisconnectedYouTubeAccount(account.status, account.external_account_id);
  const channelUrl = buildYouTubeChannelUrl(account.external_account_id);
  const displayName = disconnected
    ? "Disconnected YouTube channel"
    : account.account_name?.trim() || "YouTube channel";
  const sourceHref = disconnected ? null : channelUrl || YOUTUBE_HOME_URL;
  const initials = getYouTubeIdentityInitials(account.account_name, disconnected);
  const showAvatar = !disconnected
    && Boolean(account.account_avatar_url)
    && failedAvatarUrl !== account.account_avatar_url;

  useEffect(() => {
    if (disconnected || channelUrl || warnedHomeFallbackAccountIds.has(account.id)) return;
    warnedHomeFallbackAccountIds.add(account.id);
    console.warn("YouTube channel identity is using the YouTube home fallback", {
      accountId: account.id,
    });
  }, [account.id, channelUrl, disconnected]);

  return (
    <div
      className={`${styles.identity}${compact ? ` ${styles.identityCompact}` : ""}`}
      data-youtube-account-id={account.id}
      data-youtube-channel-identity
    >
      {showAvatar ? (
        // The API supplies the connected channel image URL; a native image preserves its remote host.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={account.account_avatar_url || undefined}
          alt=""
          className={styles.avatar}
          data-youtube-channel-avatar
          onError={() => setFailedAvatarUrl(account.account_avatar_url || "")}
        />
      ) : (
        <span className={styles.avatarFallback} aria-hidden="true" data-youtube-channel-avatar-fallback>
          {initials || <AccountDestinationIcon platform="youtube" size={compact ? 16 : 18} />}
        </span>
      )}

      <span className={styles.copy}>
        <span className={styles.name} title={displayName}>{displayName}</span>
        {sourceHref ? (
          <YouTubeSourceLink
            href={sourceHref}
            label={channelUrl ? displayName : "YouTube"}
            disclosure={disclosure}
          />
        ) : null}
      </span>
    </div>
  );
}
