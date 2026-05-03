"use client";

import { useEffect, useState } from "react";

import type { PinterestBoard, SocialPostValidationIssue } from "@/lib/api";
import { createPinterestBoard, listPinterestBoards } from "@/lib/api";
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
  const [newBoardName, setNewBoardName] = useState("");
  const [creatingBoard, setCreatingBoard] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  async function loadBoards() {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) {
        setError("Sign in again to load Pinterest boards.");
        return;
      }
      const res = await listPinterestBoards(token, profileId, accountId);
      setBoards(res.data.boards || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load Pinterest boards.");
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        setLoading(true);
        setError(null);
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

  async function handleCreateBoard() {
    const name = newBoardName.trim();
    if (!name) {
      setCreateError("Board name is required.");
      return;
    }

    setCreatingBoard(true);
    setCreateError(null);
    try {
      const token = await getToken();
      if (!token) {
        setCreateError("Sign in again to create a Pinterest board.");
        return;
      }
      const res = await createPinterestBoard(token, profileId, accountId, name);
      const board = res.data.board;
      setBoards((prev) => {
        const withoutDup = prev.filter((item) => item.id !== board.id);
        return [...withoutDup, board];
      });
      onChange({ boardId: board.id });
      setNewBoardName("");
      setError(null);
    } catch (err) {
      setCreateError(err instanceof Error ? err.message : "Failed to create Pinterest board.");
    } finally {
      setCreatingBoard(false);
    }
  }

  const boardIssue = issues.find((issue) => issue.field === "platform_options.board_id");
  const linkIssue = issues.find((issue) => issue.field === "platform_options.link");

  return (
    <div className="space-y-4">
      <div>
        <div className="mb-1.5 flex items-center justify-between gap-3">
          <label className="block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
            Board
          </label>
          <button
            type="button"
            onClick={() => { void loadBoards(); }}
            className="text-[11px] underline disabled:opacity-50"
            style={{ color: "var(--dmuted)" }}
            disabled={loading || creatingBoard}
          >
            Refresh
          </button>
        </div>
        <select
          value={fields.boardId || ""}
          onChange={(e) => onChange({ boardId: e.target.value })}
          className="w-full rounded-md border px-3 py-2 text-sm outline-none"
          style={{
            background: "var(--surface1)",
            borderColor: boardIssue ? "var(--danger)" : "var(--dborder)",
            color: "var(--dtext)",
          }}
          disabled={loading || !!error || (!loading && !error && boards.length === 0)}
        >
          <option value="">
            {loading
              ? "Loading boards..."
              : error
                ? "Couldn't load boards"
                : boards.length === 0
                  ? "No boards on this Pinterest account"
                  : "Select a Pinterest board"}
          </option>
          {boards.map((board) => (
            <option key={board.id} value={board.id}>
              {board.name}
            </option>
          ))}
        </select>
        <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: boardIssue ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted)" }}>
          {boardIssue?.message
            || error
            || (!loading && boards.length === 0
              ? <>This account has no Pinterest boards yet. <a href="https://www.pinterest.com/board/create/" target="_blank" rel="noreferrer" className="underline">Create one on Pinterest</a>, then reopen this drawer.</>
              : "Every Pinterest Pin must be saved to a board.")}
        </p>
      </div>

      <div>
        <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
          Create board
        </label>
        <div className="flex gap-2">
          <input
            type="text"
            value={newBoardName}
            onChange={(e) => setNewBoardName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                void handleCreateBoard();
              }
            }}
            placeholder="Sandbox test board"
            className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
            style={{
              background: "var(--surface1)",
              borderColor: createError ? "var(--danger)" : "var(--dborder)",
              color: "var(--dtext)",
            }}
            disabled={creatingBoard}
          />
          <button
            type="button"
            onClick={() => { void handleCreateBoard(); }}
            className="rounded-md border px-3 py-2 text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-50"
            style={{
              background: "var(--surface1)",
              borderColor: "var(--dborder)",
              color: "var(--dtext)",
            }}
            disabled={creatingBoard}
          >
            {creatingBoard ? "Creating..." : "Create"}
          </button>
        </div>
        <p className="mt-1.5 text-[11px] leading-relaxed" style={{ color: createError ? "color-mix(in srgb, var(--danger) 45%, white)" : "var(--dmuted)" }}>
          {createError || "Creates a board using the currently connected Pinterest account and token."}
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
