"use client";

import { useMemo, useState } from "react";
import type { Profile, SocialAccount } from "@/lib/api";

interface ProfileBindingsProps {
  account: SocialAccount;
  profiles: Profile[];
  onBind: (profileId: string) => Promise<void>;
  onRequestUnbind: () => void;
}

export function ProfileBindings({
  account,
  profiles,
  onBind,
  onRequestUnbind,
}: ProfileBindingsProps) {
  const [bindingProfileId, setBindingProfileId] = useState<string | null>(null);
  const [error, setError] = useState("");
  const boundProfileIds = useMemo(
    () => new Set(account.bound_profile_ids?.length ? account.bound_profile_ids : [account.profile_id]),
    [account.bound_profile_ids, account.profile_id],
  );
  const boundCount = boundProfileIds.size;

  async function bind(profileId: string) {
    setBindingProfileId(profileId);
    setError("");
    try {
      await onBind(profileId);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Failed to add this account to the Profile.");
    } finally {
      setBindingProfileId(null);
    }
  }

  return (
    <div className="min-w-[220px]">
      <div className="flex items-center justify-end gap-2">
        {account.shared_connection || boundCount > 1 ? (
          <span className="dbadge dbadge-green">Shared across {boundCount} Profiles</span>
        ) : (
          <span className="text-[11px]" style={{ color: "var(--dmuted2)" }}>1 Profile</span>
        )}
        <details className="text-left">
          <summary className="dbtn dbtn-ghost cursor-pointer list-none px-2.5 py-1 text-[12px] active:scale-[0.98]">
            Manage Profiles
          </summary>
          <div
            className="ml-auto mt-2 w-[280px] rounded-lg border p-3 shadow-[0_16px_32px_-18px_rgba(15,23,42,0.35)]"
            style={{ background: "var(--surface1)", borderColor: "var(--dborder)" }}
          >
            <div className="mb-2 text-[11px] font-semibold uppercase tracking-[0.1em]" style={{ color: "var(--dmuted2)" }}>
              Profile bindings
            </div>
            <div className="divide-y" style={{ borderColor: "var(--dborder)" }}>
              {profiles.map((profile) => {
                const alreadyBound = boundProfileIds.has(profile.id);
                const binding = bindingProfileId === profile.id;
                return (
                  <div key={profile.id} className="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0">
                    <span className="truncate text-[12px] font-medium" style={{ color: "var(--dtext)" }}>
                      {profile.name}
                    </span>
                    {alreadyBound ? (
                      <span className="shrink-0 text-[11px]" style={{ color: "var(--dmuted2)" }}>Already added</span>
                    ) : (
                      <button
                        type="button"
                        className="dbtn dbtn-ghost shrink-0 px-2 py-1 text-[11px] active:scale-[0.98]"
                        disabled={bindingProfileId !== null}
                        onClick={() => bind(profile.id)}
                      >
                        {binding ? "Adding..." : "Add to profile"}
                      </button>
                    )}
                  </div>
                );
              })}
            </div>
            <div className="mt-3 border-t pt-3" style={{ borderColor: "var(--dborder)" }}>
              <button
                type="button"
                className="dbtn dbtn-ghost w-full justify-center text-[11px] active:scale-[0.98]"
                disabled={bindingProfileId !== null}
                onClick={onRequestUnbind}
              >
                Remove from this Profile
              </button>
              <p className="mt-2 text-[10.5px] leading-[1.45]" style={{ color: "var(--dmuted2)" }}>
                This keeps the physical social connection active for any other Profile bindings.
              </p>
            </div>
            {error ? (
              <p role="alert" className="mt-2 text-[11px] leading-[1.45]" style={{ color: "var(--danger)" }}>
                {error}
              </p>
            ) : null}
          </div>
        </details>
      </div>
    </div>
  );
}
