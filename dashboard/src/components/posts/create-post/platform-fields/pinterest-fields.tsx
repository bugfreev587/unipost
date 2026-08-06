"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import type { PinterestBoard, SocialPostValidationIssue } from "@/lib/api";
import { createPinterestBoard, listPinterestBoards } from "@/lib/api";
import {
  pinterestBoardEnvironment,
  reconcilePinterestBoardSelection,
  type PinterestBoardSnapshot,
} from "@/lib/pinterest-boards";
import type { PlatformOverride } from "../use-create-post-form";

interface PinterestFieldsProps {
  accountId: string;
  profileId: string;
  getToken: () => Promise<string | null>;
  fields: NonNullable<PlatformOverride["pinterest"]>;
  issues?: SocialPostValidationIssue[];
  onChange: (fields: Partial<NonNullable<PlatformOverride["pinterest"]>>) => void;
  onSnapshotChange: (snapshot: PinterestBoardSnapshot | null) => void;
}

export function PinterestFields({
  accountId,
  profileId,
  getToken,
  fields,
  issues = [],
  onChange,
  onSnapshotChange,
}: PinterestFieldsProps) {
  const [boards, setBoards] = useState<PinterestBoard[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newBoardName, setNewBoardName] = useState("");
  const [creatingBoard, setCreatingBoard] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [sandboxMode, setSandboxMode] = useState(false);
  const previousSnapshotRef = useRef<PinterestBoardSnapshot | null>(null);
  const selectedBoardIdRef = useRef(fields.boardId || "");
  const onChangeRef = useRef(onChange);
  const onSnapshotChangeRef = useRef(onSnapshotChange);
  const requestGenerationRef = useRef(0);

  selectedBoardIdRef.current = fields.boardId || "";
  onChangeRef.current = onChange;
  onSnapshotChangeRef.current = onSnapshotChange;

  const loadBoards = useCallback(async (preferredBoardId?: string): Promise<PinterestBoardSnapshot | null> => {
    const requestGeneration = ++requestGenerationRef.current;
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) {
        if (requestGeneration === requestGenerationRef.current) {
          setError("Sign in again to load Pinterest boards.");
        }
        return null;
      }
      const res = await listPinterestBoards(token, profileId, accountId);
      if (requestGeneration !== requestGenerationRef.current) return null;
      const nextBoards = res.data.boards || [];
      const snapshot: PinterestBoardSnapshot = {
        accountId,
        environment: pinterestBoardEnvironment(!!res.data.sandbox_mode),
        fetchedAt: Date.now(),
        boardIds: nextBoards.map((board) => board.id),
      };
      const candidate = preferredBoardId || selectedBoardIdRef.current;
      const reconciled = reconcilePinterestBoardSelection(candidate, previousSnapshotRef.current, snapshot);
      previousSnapshotRef.current = snapshot;
      setBoards(nextBoards);
      setSandboxMode(snapshot.environment === "sandbox");
      onSnapshotChangeRef.current(snapshot);
      if (reconciled !== selectedBoardIdRef.current) {
        selectedBoardIdRef.current = reconciled;
        onChangeRef.current({ boardId: reconciled });
      }
      return snapshot;
    } catch (err) {
      if (requestGeneration === requestGenerationRef.current) {
        setError(err instanceof Error ? err.message : "Failed to load Pinterest boards.");
      }
      return null;
    } finally {
      if (requestGeneration === requestGenerationRef.current) {
        setLoading(false);
      }
    }
  }, [accountId, getToken, profileId]);

  useEffect(() => {
    previousSnapshotRef.current = null;
    onSnapshotChangeRef.current(null);
    void loadBoards();
    return () => {
      requestGenerationRef.current += 1;
    };
  }, [accountId, loadBoards, profileId]);

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
      const snapshot = await loadBoards(board.id);
      if (!snapshot?.boardIds.includes(board.id)) {
        setCreateError("The board was created, but Pinterest has not returned it yet. Refresh the board list and select it before publishing.");
        return;
      }
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
              ? "This Pinterest account has no available boards. Create a board before publishing a Pin."
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
          {createError || (sandboxMode
            ? "Sandbox mode is enabled. Create boards here so they are available to the same Pinterest API environment UniPost uses."
            : "Creates a board using the currently connected Pinterest account and token.")}
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
