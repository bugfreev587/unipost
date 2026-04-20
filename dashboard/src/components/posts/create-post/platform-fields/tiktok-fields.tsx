"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import type { SocialAccount, SocialPostValidationIssue, TikTokCreatorInfo } from "@/lib/api";
import { getTikTokCreatorInfo } from "@/lib/api";
import type { PlatformOverride } from "../use-create-post-form";

interface TikTokFieldsProps {
  account: SocialAccount;
  fields: NonNullable<PlatformOverride["tiktok"]>;
  mediaKind: "video" | "photo" | "none";
  // The first video file in the upload queue — we measure its duration
  // client-side (HTML5 Video API) and compare against the creator's
  // max_video_post_duration_sec. Null when no video is present.
  mediaFile: File | null;
  profileId: string;
  getToken: () => Promise<string | null>;
  issues?: SocialPostValidationIssue[];
  onChange: (fields: Partial<NonNullable<PlatformOverride["tiktok"]>>) => void;
  // Called whenever this component has a reason Publish should be blocked
  // that can't be derived from form state alone — e.g., creator_info
  // returned an error, or the uploaded video exceeds max length. Pass null
  // when the blocker clears.
  onBlockerChange: (reason: string | null) => void;
}

const PRIVACY_LABELS: Record<string, string> = {
  PUBLIC_TO_EVERYONE: "Everyone",
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
  mediaFile,
  profileId,
  getToken,
  onChange,
  onBlockerChange,
}: TikTokFieldsProps) {
  const [state, setState] = useState<LoadState>({ status: "idle" });
  const [autoSwitchNotice, setAutoSwitchNotice] = useState<string | null>(null);
  const [videoDurationSec, setVideoDurationSec] = useState<number | null>(null);
  const [videoMeasureError, setVideoMeasureError] = useState<string | null>(null);
  // Track the previous mediaFile identity so we don't re-measure on
  // unrelated re-renders (File identity changes per render if the parent
  // rebuilds the array).
  const measuredFileRef = useRef<File | null>(null);

  // Fetch creator_info once per mount per account. Any error (including
  // "daily cap reached" / "account restricted") triggers the blocker
  // path below — the PRD treats creator_info errors as unconditional
  // publish blockers so audit reviewers always see a clear message.
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

  // Measure the selected video's duration with a throwaway <video>
  // element. loadedmetadata fires before any frame decode, so this is
  // cheap even for long videos. We only measure when we have a video
  // AND we know the creator's cap (otherwise comparison is pointless).
  useEffect(() => {
    if (mediaKind !== "video" || !mediaFile) {
      setVideoDurationSec(null);
      setVideoMeasureError(null);
      measuredFileRef.current = null;
      return;
    }
    if (measuredFileRef.current === mediaFile) return;
    measuredFileRef.current = mediaFile;

    const url = URL.createObjectURL(mediaFile);
    const video = document.createElement("video");
    video.preload = "metadata";
    let settled = false;
    const cleanup = () => {
      URL.revokeObjectURL(url);
      video.src = "";
    };
    video.onloadedmetadata = () => {
      if (settled) return;
      settled = true;
      setVideoDurationSec(video.duration);
      setVideoMeasureError(null);
      cleanup();
    };
    video.onerror = () => {
      if (settled) return;
      settled = true;
      setVideoDurationSec(null);
      setVideoMeasureError("Couldn't read video metadata");
      cleanup();
    };
    video.src = url;
    return () => {
      if (!settled) cleanup();
    };
  }, [mediaFile, mediaKind]);

  const brandedPrivateConflict =
    fields.disclosureEnabled && fields.brandedContent && fields.privacy === "SELF_ONLY";
  const disclosureIncomplete =
    fields.disclosureEnabled && !fields.yourBrand && !fields.brandedContent;
  const showDuetStitch = mediaKind !== "photo";
  const brandedLocksPrivate = fields.disclosureEnabled && fields.brandedContent;

  // Compute per-creator duration cap (falls back to undefined while
  // creator_info loads; we skip the check until we have a real value).
  const maxDurationSec = state.status === "ready" ? state.info.max_video_post_duration_sec : undefined;
  const durationOverLimit =
    typeof maxDurationSec === "number" &&
    maxDurationSec > 0 &&
    typeof videoDurationSec === "number" &&
    videoDurationSec > maxDurationSec;

  // Roll up the runtime reasons Publish should be blocked. Creator
  // errors outrank duration errors — if creator_info failed we can't
  // trust any of the secondary data anyway.
  const effectiveBlocker = useMemo<string | null>(() => {
    if (state.status === "error") {
      // PRD treats any creator_info failure as a posting block. Show
      // the specific message when TikTok gave us one (covers "daily
      // cap reached", "account restricted", etc.) otherwise the
      // generic cap message.
      return state.message
        ? `TikTok: ${state.message}`
        : "This TikTok account has reached its daily posting limit. Please try again later.";
    }
    if (durationOverLimit && videoDurationSec != null && maxDurationSec) {
      return `TikTok video is ${formatDuration(Math.round(videoDurationSec))} long; max for this account is ${formatDuration(maxDurationSec)}.`;
    }
    return null;
  }, [state, durationOverLimit, videoDurationSec, maxDurationSec]);

  useEffect(() => {
    onBlockerChange(effectiveBlocker);
    // We only want this to fire when the blocker itself changes — the
    // parent's callback identity is stable via the account-scoped
    // wrapper so we don't need to subscribe to it.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [effectiveBlocker]);

  // Clear the blocker on unmount so the account switching doesn't
  // leave stale "Publish disabled" state in the drawer.
  useEffect(() => {
    return () => onBlockerChange(null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Handler that toggles Branded Content AND enforces the
  // "no private branded content" rule. When the user *enables*
  // Branded Content while privacy is currently SELF_ONLY, we auto-
  // switch to the first non-private option available for this
  // creator (falling back to PUBLIC_TO_EVERYONE if the creator's
  // list is unexpectedly empty) and surface an inline notice so
  // the switch isn't silent.
  const handleBrandedContentChange = (checked: boolean) => {
    const updates: Partial<NonNullable<PlatformOverride["tiktok"]>> = { brandedContent: checked };
    if (checked && fields.privacy === "SELF_ONLY") {
      const options = state.status === "ready" ? state.info.privacy_level_options : [];
      const firstPublic = options.find((opt) => opt !== "SELF_ONLY") || "PUBLIC_TO_EVERYONE";
      updates.privacy = firstPublic as typeof fields.privacy;
      const label = PRIVACY_LABELS[firstPublic] || firstPublic;
      setAutoSwitchNotice(`Branded content cannot be set to private. Visibility changed to ${label}.`);
    } else if (!checked) {
      setAutoSwitchNotice(null);
    }
    onChange(updates);
  };

  return (
    <div className="space-y-4">
      <CreatorHeader state={state} fallbackName={account.account_name} />

      {/* Creator cannot post — PRD Fix 2 §3.3. Rendered as a prominent
          red banner so the audit reviewer can clearly see the gate. */}
      {state.status === "error" && (
        <div
          className="rounded-md border px-3 py-2 text-[12px]"
          style={{
            background: "color-mix(in srgb, var(--danger) 12%, var(--surface-raised))",
            borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)",
            color: "color-mix(in srgb, var(--danger) 26%, white)",
          }}
        >
          <div className="mb-0.5 text-[12.5px] font-semibold">Cannot publish to this TikTok account</div>
          <div className="leading-relaxed">
            {state.message || "This TikTok account has reached its daily posting limit. Please try again later."}
            {" "}You can still save this post as a draft.
          </div>
        </div>
      )}

      {autoSwitchNotice && (
        <Hint tone="warning">{autoSwitchNotice}</Hint>
      )}

      {/* Privacy — no default; options come from creator_info. When
          Branded Content is enabled we disable the SELF_ONLY option so
          the rule is visually enforceable, not just a validation error. */}
      <Field label="Who can view this video">
        <select
          value={fields.privacy}
          onChange={(e) => {
            // User picked something manually — clear any "we auto-switched"
            // notice, their choice is now the source of truth.
            setAutoSwitchNotice(null);
            onChange({ privacy: e.target.value as typeof fields.privacy });
          }}
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
            state.info.privacy_level_options.map((opt) => {
              const isPrivate = opt === "SELF_ONLY";
              const lockedByBrand = isPrivate && brandedLocksPrivate;
              return (
                <option
                  key={opt}
                  value={opt}
                  disabled={lockedByBrand}
                  title={lockedByBrand ? "Branded content visibility cannot be set to private." : undefined}
                >
                  {PRIVACY_LABELS[opt] || opt}
                  {lockedByBrand ? " (not allowed for branded content)" : ""}
                </option>
              );
            })}
        </select>
        {!fields.privacy && state.status === "ready" && (
          <Hint tone="warning">Pick a visibility — TikTok requires an explicit choice.</Hint>
        )}
        {brandedPrivateConflict && (
          <Hint tone="error">Branded content cannot be posted as private.</Hint>
        )}
      </Field>

      {/* Interactions — all OFF by default. If creator_info says the
          feature is disabled at the account level, the toggle is locked
          OFF and greyed out (applies to Comment too — PRD Fix 2 §3.2). */}
      <Field label="Allow users to">
        <div className="flex flex-wrap gap-2">
          <InteractionToggle
            label="Comment"
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

      {/* Video duration — under cap we just show the reading; over cap
          we surface an explicit error and the publish blocker above
          handles the actual gating. */}
      {mediaKind === "video" && typeof videoDurationSec === "number" && typeof maxDurationSec === "number" && maxDurationSec > 0 && (
        durationOverLimit ? (
          <div
            className="rounded-md border px-3 py-2 text-[12px]"
            style={{
              background: "color-mix(in srgb, var(--danger) 12%, var(--surface-raised))",
              borderColor: "color-mix(in srgb, var(--danger) 45%, transparent)",
              color: "color-mix(in srgb, var(--danger) 26%, white)",
            }}
          >
            <div className="mb-0.5 font-semibold">Video is too long for this TikTok account</div>
            <div className="leading-relaxed">
              This video is {formatDuration(Math.round(videoDurationSec))} long. Maximum allowed for this account is {formatDuration(maxDurationSec)}. Please upload a shorter video.
            </div>
          </div>
        ) : (
          <Hint tone="info">
            Video duration: {formatDuration(Math.round(videoDurationSec))} (max: {formatDuration(maxDurationSec)}).
          </Hint>
        )
      )}
      {mediaKind === "video" && videoMeasureError && (
        <Hint tone="warning">{videoMeasureError}. TikTok will still enforce its own duration limit.</Hint>
      )}
      {/* If we have a cap but haven't received a video yet, show the cap
          as a heads-up so the user knows before they upload. */}
      {mediaKind === "video" && typeof maxDurationSec === "number" && maxDurationSec > 0 && videoDurationSec == null && !videoMeasureError && (
        <Hint tone="info">Max video length for this account: {formatDuration(maxDurationSec)}.</Hint>
      )}

      <DisclosureSection
        fields={fields}
        onChange={onChange}
        onBrandedContentChange={handleBrandedContentChange}
        disclosureIncomplete={disclosureIncomplete}
      />

      <Consent showBrandedContent={fields.disclosureEnabled && fields.brandedContent} />
    </div>
  );
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
    return null; // the dedicated error banner below handles this case
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
  onBrandedContentChange,
  disclosureIncomplete,
}: {
  fields: NonNullable<PlatformOverride["tiktok"]>;
  onChange: (fields: Partial<NonNullable<PlatformOverride["tiktok"]>>) => void;
  onBrandedContentChange: (checked: boolean) => void;
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
            onChange={onBrandedContentChange}
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

// Consent footer — wording and link order follow the PRD exactly so the
// audit reviewer sees a one-to-one match:
//   - Your Brand only → "Music Usage Confirmation"
//   - Branded Content (with or without Your Brand) → "Branded Content
//     Policy and Music Usage Confirmation"
function Consent({ showBrandedContent }: { showBrandedContent: boolean }) {
  const music = (
    <a
      href="https://www.tiktok.com/legal/page/global/music-usage-confirmation/en"
      target="_blank"
      rel="noreferrer"
      className="underline"
      style={{ color: "var(--primary)" }}
    >
      Music Usage Confirmation
    </a>
  );
  const branded = (
    <a
      href="https://www.tiktok.com/legal/page/global/bc-policy/en"
      target="_blank"
      rel="noreferrer"
      className="underline"
      style={{ color: "var(--primary)" }}
    >
      Branded Content Policy
    </a>
  );
  return (
    <div className="text-[11px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
      By posting, you agree to TikTok&rsquo;s{" "}
      {showBrandedContent ? (
        <>
          {branded} and {music}
        </>
      ) : (
        music
      )}
      .
    </div>
  );
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  const rem = seconds % 60;
  return rem === 0 ? `${minutes}m` : `${minutes}m ${rem}s`;
}
