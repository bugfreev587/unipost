"use client";

import { useEffect, useState } from "react";

import type { PinterestBoard, SocialPostValidationIssue } from "@/lib/api";
import { listPinterestBoards } from "@/lib/api";
import type { PlatformOverride } from "../use-create-post-form";

interface PinterestFieldsProps {
  accountId: string;
  profileId: string;
  getToken: () => Promise<string | null>;
  fields: NonNullable<PlatformOverride["pinterest"]>;
  issues?: SocialPostValidationIssue[];
  onChange: (fields: Partial<NonNullable<PlatformOverride["pinterest"]>>) => void;
}

export function PinterestFields({
  accountId,
  profileId,
  getToken,
  fields,
  issues = [],
  onChange,
}: PinterestFieldsProps) {
  const [boards, setBoards] = useState<PinterestBoard[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError(null);
      try {
        const token = await getToken();
        if (!token) {
          if (!cancelled) setError("Sign in again to load Pinterest boards.");
          return;
        }
        const res = await listPinterestBoards(token, profileId, accountId);
        if (cancelled) return;
        setBoards(res.data.boards || []);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Failed to load Pinterest boards.");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [accountId, profileId, getToken]);

  const boardIssue = issues.find((issue) => issue.field === "platform_options.board_id");
  const linkIssue = issues.find((issue) => issue.field === "platform_options.link");

  return (
    <div className="space-y-4">
      <div>
        <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
          Board
        </label>
        <select
          value={fields.boardId || ""}
          onChange={(e) => onChange({ boardId: e.target.value })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none"
          style={{
            background: "var(--surface1)",
            borderColor: boardIssue ? "var(--danger)" : "var(--dborder)",
            color: "var(--dtext)",
          }}
          disabled={loading || !!error}
        >
          <option value="">
            {loading ? "Loading boards..." : error ? "Couldn't load boards" : "Select a Pinterest board"}
          </option>
          {boards.map((board) => (
            <option key={board.id} value={board.id}>
              {board.name}
            </option>
          ))}
        </select>
        <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: boardIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted)" }}>
          {boardIssue?.message || error || "Every Pinterest Pin must be saved to a board."}
        </p>
      </div>

      <div>
        <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
          Title (optional)
        </label>
        <input
          type="text"
          value={fields.title || ""}
          onChange={(e) => onChange({ title: e.target.value })}
          placeholder="Pin title"
          className="w-full rounded-md border px-3 py-2 text-sm outline-none"
          style={{
            background: "var(--surface1)",
            borderColor: "var(--dborder)",
            color: "var(--dtext)",
          }}
        />
      </div>

      <div>
        <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
          Destination link (optional)
        </label>
        <input
          type="url"
          inputMode="url"
          value={fields.link || ""}
          onChange={(e) => onChange({ link: e.target.value })}
          placeholder="https://example.com"
          className="w-full rounded-md border px-3 py-2 text-sm outline-none"
          style={{
            background: "var(--surface1)",
            borderColor: linkIssue ? "var(--danger)" : "var(--dborder)",
            color: "var(--dtext)",
          }}
        />
        <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: linkIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted)" }}>
          {linkIssue?.message || "Pinterest uses this as the click-through URL for the Pin."}
        </p>
      </div>
    </div>
  );
}
