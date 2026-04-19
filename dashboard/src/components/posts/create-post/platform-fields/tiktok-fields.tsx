"use client";

import { useEffect, useState } from "react";

import type { SocialAccount, SocialPostValidationIssue, TikTokCreatorInfo } from "@/lib/api";
import { getTikTokCreatorInfo } from "@/lib/api";
import type { PlatformOverride } from "../use-create-post-form";

interface TikTokFieldsProps {
  account: SocialAccount;
  fields: NonNullable<PlatformOverride["tiktok"]>;
  mediaKind: "video" | "photo" | "none";
  profileId: string;
  getToken: () => Promise<string | null>;
  issues?: SocialPostValidationIssue[];
  onChange: (fields: Partial<NonNullable<PlatformOverride["tiktok"]>>) => void;
}

// Human-facing labels for TikTok's privacy enum. We render only the options
// creator_info actually returns for this creator, so accounts in sandbox/
// unaudited mode (TikTok forces SELF_ONLY on them) naturally see just the
// "Only me" option and nothing misleading.
const PRIVACY_LABELS: Record<string, string> = {
  PUBLIC_TO_EVERYONE: "Public",
  MUTUAL_FOLLOW_FRIENDS: "Friends",
  FOLLOWER_OF_CREATOR: "Followers",
  SELF_ONLY: "Only me",
};

type LoadState =
  | { status: "idle" }
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; info: TikTokCreatorInfo };

export function TikTokFields({
  account,
  fields,
  mediaKind,
  profileId,
  getToken,
  onChange,
}: TikTokFieldsProps) {
  const [state, setState] = useState<LoadState>({ status: "idle" });

  // Fetch creator_info once per mount per account. If the token/account
  // changes we re-fetch — creator_info caps, toggles, and privacy options
  // can all change on TikTok's side, so we don't want to cache across
  // remounts either. The result drives every downstream toggle, so we
  // render "loading" placeholders rather than showing stale defaults.
  useEffect(() => {
    let cancelled = false;
    (async () => {
      setState({ status: "loading" });
      try {
        const token = await getToken();
        if (!token) {
          if (!cancelled) setState({ status: "error", message: "Not signed in" });
          return;
        }
        const res = await getTikTokCreatorInfo(token, profileId, account.id);
        if (cancelled) return;
        setState({ status: "ready", info: res.data });
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : "Failed to load TikTok creator info";
        setState({ status: "error", message });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [account.id, profileId, getToken]);

  // TikTok disallows Branded Content posts set to SELF_ONLY. Show the
  // conflict inline and also require the publish-button guard in the
  // drawer to block submission — see create-post-drawer.tsx disabledReason.
  const brandedPrivateConflict =
    fields.disclosureEnabled && fields.brandedContent && fields.privacy === "SELF_ONLY";
  const disclosureIncomplete =
    fields.disclosureEnabled && !fields.yourBrand && !fields.brandedContent;
  const showDuetStitch = mediaKind !== "photo";

  return (
    <div className="space-y-4">
      {/* Creator identity — required for the audit so users know which
          account they're posting from, especially when they've connected
          multiple TikTok accounts. */}
      <CreatorHeader state={state} fallbackName={account.account_name} />

      {/* Privacy — no default; options come from creator_info. */}
      <Field label="Who can view this video">
        <select
          value={fields.privacy}
          onChange={(e) => onChange({ privacy: e.target.value as typeof fields.privacy })}
          disabled={state.status !== "ready"}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms] disabled:opacity-60"
          style={{
            background: "var(--surface1)",
            borderColor: !fields.privacy ? "color-mix(in srgb, var(--warning) 60%, transparent)" : "var(--dborder)",
            color: "var(--dtext)",
          }}
        >
          <option value="" disabled>
            {state.status === "loading"
              ? "Loading options…"
              : state.status === "error"
                ? "Unavailable — see error above"
                : "Select who can view"}
          </option>
          {state.status === "ready" &&
            state.info.privacy_level_options.map((opt) => (
              <option key={opt} value={opt}>
                {PRIVACY_LABELS[opt] || opt}
              </option>
            ))}
        </select>
        {!fields.privacy && state.status === "ready" && (
          <Hint tone="warning">Pick a visibility — TikTok requires an explicit choice.</Hint>
        )}
        {brandedPrivateConflict && (
          <Hint tone="error">
            Branded Content cannot be posted as &ldquo;Only me&rdquo; — change the visibility or turn off Branded Content.
          </Hint>
        )}
      </Field>

      {/* Interactions — all OFF by default. If creator_info says the
          feature is disabled at the account level, the toggle is locked
          OFF and greyed out. */}
      <Field label="Allow users to">
        <div className="flex flex-wrap gap-2">
          <InteractionToggle
            label="Comment"
            // UI "allow X" = TikTok API "disable_X" inverted.
            checked={!fields.disableComment}
            creatorDisabled={state.status === "ready" && state.info.comment_disabled}
            onChange={(allow) => onChange({ disableComment: !allow })}
          />
          {showDuetStitch && (
            <InteractionToggle
              label="Duet"
              checked={!fields.disableDuet}
              creatorDisabled={state.status === "ready" && state.info.duet_disabled}
              onChange={(allow) => onChange({ disableDuet: !allow })}
            />
          )}
          {showDuetStitch && (
            <InteractionToggle
              label="Stitch"
              checked={!fields.disableStitch}
              creatorDisabled={state.status === "ready" && state.info.stitch_disabled}
              onChange={(allow) => onChange({ disableStitch: !allow })}
            />
          )}
        </div>
      </Field>

      {/* Commercial Content Disclosure — required by TikTok audit. */}
      <DisclosureSection
        fields={fields}
        onChange={onChange}
        disclosureIncomplete={disclosureIncomplete}
      />

      {/* Max video duration — surfaced from creator_info so users know
          the upper bound before uploading. TikTok caps vary per account
          (60s for some, 10min for others). Showing it satisfies audit
          requirement #3 even when we don't validate duration client-side. */}
      {state.status === "ready" && mediaKind === "video" && state.info.max_video_post_duration_sec > 0 && (
        <Hint tone="info">
          Max video length for this account: {formatDuration(state.info.max_video_post_duration_sec)}.
        </Hint>
      )}

      {/* Consent footer — shown above Publish. The Music Usage
          Confirmation link is required on every post; the Branded
          Content Policy link appears only when brandedContent is on. */}
      <Consent showBrandedContent={fields.disclosureEnabled && fields.brandedContent} />
    </div>
  );
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return rem === 0 ? `${minutes}m` : `${minutes}m ${rem}s`;
}

// ── Sub-components ──────────────────────────────────────────────────────

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label
        className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]"
        style={{ color: "var(--dmuted2)" }}
      >
        {label}
      </label>
      {children}
    </div>
  );
}

function Hint({ tone, children }: { tone: "warning" | "error" | "info"; children: React.ReactNode }) {
  const color =
    tone === "error"
      ? "color-mix(in srgb, var(--danger) 45%, white)"
      : tone === "warning"
        ? "color-mix(in srgb, var(--warning) 70%, white)"
        : "var(--dmuted)";
  return (
    <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color }}>
      {children}
    </p>
  );
}

function CreatorHeader({
  state,
  fallbackName,
}: {
  state: LoadState;
  fallbackName: string | null;
}) {
  if (state.status === "ready") {
    const { creator_nickname, creator_username, creator_avatar_url } = state.info;
    return (
      <div
        className="flex items-center gap-2.5 rounded-md border px-3 py-2"
        style={{ background: "var(--surface1)", borderColor: "var(--dborder)" }}
      >
        {creator_avatar_url ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img
            src={creator_avatar_url}
            alt=""
            className="h-7 w-7 rounded-full object-cover"
            style={{ background: "var(--surface2)" }}
          />
        ) : (
          <div
            className="h-7 w-7 rounded-full"
            style={{ background: "var(--surface2)" }}
          />
        )}
        <div className="min-w-0">
          <div className="truncate text-[12px] font-medium" style={{ color: "var(--dtext)" }}>
            Posting as {creator_nickname || creator_username || fallbackName || "your TikTok account"}
          </div>
          {creator_username && (
            <div className="truncate font-mono text-[10.5px]" style={{ color: "var(--dmuted2)" }}>
              @{creator_username}
            </div>
          )}
        </div>
      </div>
    );
  }

  if (state.status === "error") {
    return (
      <div
        className="rounded-md border px-3 py-2 text-[12px]"
        style={{
          background: "color-mix(in srgb, var(--danger) 10%, var(--surface1))",
          borderColor: "color-mix(in srgb, var(--danger) 40%, transparent)",
          color: "color-mix(in srgb, var(--danger) 26%, white)",
        }}
      >
        Couldn&rsquo;t load TikTok creator info: {state.message}. Reconnect the account or try again.
      </div>
    );
  }

  return (
    <div
      className="rounded-md border px-3 py-2 text-[12px]"
      style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dmuted)" }}
    >
      Loading TikTok account details…
    </div>
  );
}

function InteractionToggle({
  label,
  checked,
  creatorDisabled,
  onChange,
}: {
  label: string;
  checked: boolean;
  creatorDisabled: boolean;
  onChange: (allow: boolean) => void;
}) {
  const locked = creatorDisabled;
  const effectiveChecked = locked ? false : checked;
  return (
    <label
      className="inline-flex cursor-pointer items-center gap-1.5 rounded-md border px-3 py-1.5 text-[12px]"
      style={{
        background: effectiveChecked ? "color-mix(in srgb, var(--primary) 18%, var(--surface1))" : "var(--surface1)",
        borderColor: effectiveChecked ? "color-mix(in srgb, var(--primary) 55%, transparent)" : "var(--dborder)",
        color: locked ? "var(--dmuted2)" : "var(--dtext)",
        opacity: locked ? 0.5 : 1,
        cursor: locked ? "not-allowed" : "pointer",
      }}
      title={locked ? `${label} is disabled in this creator's TikTok settings` : undefined}
    >
      <input
        type="checkbox"
        checked={effectiveChecked}
        disabled={locked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-3.5 w-3.5 cursor-pointer disabled:cursor-not-allowed"
      />
      <span>{label}</span>
    </label>
  );
}

function DisclosureSection({
  fields,
  onChange,
  disclosureIncomplete,
}: {
  fields: NonNullable<PlatformOverride["tiktok"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["tiktok"]>>) => void;
  disclosureIncomplete: boolean;
}) {
  return (
    <div
      className="rounded-md border px-3 py-3"
      style={{ background: "var(--surface1)", borderColor: "var(--dborder)" }}
    >
      <label className="flex cursor-pointer items-start gap-2">
        <input
          type="checkbox"
          checked={fields.disclosureEnabled}
          onChange={(e) => {
            // Turning disclosure OFF clears the sub-selections so re-opening
            // doesn't surface a stale choice the user didn't re-confirm.
            if (!e.target.checked) {
              onChange({ disclosureEnabled: false, yourBrand: false, brandedContent: false });
            } else {
              onChange({ disclosureEnabled: true });
            }
          }}
          className="mt-0.5 h-3.5 w-3.5 cursor-pointer"
        />
        <div>
          <div className="text-[12.5px] font-medium" style={{ color: "var(--dtext)" }}>
            Disclose video content
          </div>
          <div className="mt-0.5 text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
            Turn on if this post promotes yourself, a third party, or both.
          </div>
        </div>
      </label>

      {fields.disclosureEnabled && (
        <div className="mt-3 space-y-2 border-t pt-3" style={{ borderTopColor: "var(--dborder)" }}>
          <DisclosureOption
            label="Your Brand"
            description="You are promoting yourself or your own business. Your video will be labeled as Promotional content."
            checked={fields.yourBrand}
            onChange={(c) => onChange({ yourBrand: c })}
          />
          <DisclosureOption
            label="Branded Content"
            description="You got paid to promote a third party. Your video will be labeled as Paid partnership."
            checked={fields.brandedContent}
            onChange={(c) => onChange({ brandedContent: c })}
          />
          {disclosureIncomplete && (
            <Hint tone="error">
              Pick at least one option to disclose this as commercial content.
            </Hint>
          )}
        </div>
      )}
    </div>
  );
}

function DisclosureOption({
  label,
  description,
  checked,
  onChange,
}: {
  label: string;
  description: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <label className="flex cursor-pointer items-start gap-2">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="mt-0.5 h-3.5 w-3.5 cursor-pointer"
      />
      <div>
        <div className="text-[12.5px] font-medium" style={{ color: "var(--dtext)" }}>
          {label}
        </div>
        <div className="mt-0.5 text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
          {description}
        </div>
      </div>
    </label>
  );
}

function Consent({ showBrandedContent }: { showBrandedContent: boolean }) {
  return (
    <div className="text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
      By posting, you agree to TikTok&rsquo;s{" "}
      <a
        href="https://www.tiktok.com/legal/page/global/music-usage-confirmation/en"
        target="_blank"
        rel="noreferrer"
        className="underline"
        style={{ color: "var(--primary)" }}
      >
        Music Usage Confirmation
      </a>
      {showBrandedContent && (
        <>
          {" "}and{" "}
          <a
            href="https://www.tiktok.com/legal/bc-policy"
            target="_blank"
            rel="noreferrer"
            className="underline"
            style={{ color: "var(--primary)" }}
          >
            Branded Content Policy
          </a>
        </>
      )}
      .
    </div>
  );
}
