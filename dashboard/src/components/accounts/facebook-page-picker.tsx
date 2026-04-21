"use client";

// FacebookPagePicker renders the modal the user lands on after the
// Facebook OAuth callback. The callback stashes a pending_connection
// row server-side and redirects here with ?pending=<id>; we fetch
// the stored Page list and let the user check which Pages to connect.
//
// Server-side rules enforced here in the UI:
//   - Rows whose tasks don't include a publishing permission render
//     as read-only (can_publish = false) with the PRD §15 copy
//   - 0-Page responses render the "create a Page" CTA
//   - Submitting with none selected stays disabled
//
// After finalize, the modal calls `onFinalized` so the parent page
// can refetch the account list.

import { useCallback, useEffect, useRef, useState } from "react";
import Image from "next/image";
import { Loader2, CheckCircle2, AlertTriangle } from "lucide-react";

import {
  Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle,
} from "@/components/ui/dialog";
import type { PendingConnection, PendingFacebookPage } from "@/lib/api";
import { getPendingConnection, finalizePendingConnection } from "@/lib/api";

interface Props {
  open: boolean;
  pendingId: string;
  workspaceId: string;
  // Loose return type to match Clerk's useAuth().getToken signature
  // directly without wrapping it in an extra arrow function whose
  // identity changes every render.
  getToken: () => Promise<string | null | undefined>;
  onClose: () => void;
  onFinalized: (connectedCount: number) => void;
}

type LoadState =
  | { kind: "loading" }
  | { kind: "error"; message: string }
  | { kind: "ready"; data: PendingConnection };

export function FacebookPagePicker({ open, pendingId, workspaceId, getToken, onClose, onFinalized }: Props) {
  const [load, setLoad] = useState<LoadState>({ kind: "loading" });
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  // finalizedRef short-circuits the fetch effect once finalize
  // succeeds. The server deletes the pending row inside finalize,
  // so any refetch between that moment and the parent unmounting
  // this component (URL transition) would 404 and briefly render
  // "Pending connection not found or expired" on a modal that's
  // about to disappear.
  const finalizedRef = useRef(false);

  // Reset the guard when the parent hands us a NEW pendingId, so a
  // second Connect attempt within the same session works.
  useEffect(() => {
    finalizedRef.current = false;
  }, [pendingId]);

  useEffect(() => {
    if (!open || finalizedRef.current) return;
    let cancelled = false;
    (async () => {
      setLoad({ kind: "loading" });
      setSelected(new Set());
      setSubmitError(null);
      try {
        const token = await getToken();
        if (!token) {
          if (!cancelled) setLoad({ kind: "error", message: "Not signed in" });
          return;
        }
        const res = await getPendingConnection(token, workspaceId, pendingId);
        if (cancelled) return;
        setLoad({ kind: "ready", data: res.data });
        // Pre-select the sole publishable Page when there's exactly
        // one — matches the "if single Page, skip picker" idea from
        // the PRD while still keeping the picker visible so the user
        // sees what they're connecting.
        const publishable = res.data.pages.filter((p) => p.can_publish);
        if (publishable.length === 1) {
          setSelected(new Set([publishable[0].id]));
        }
      } catch (err) {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : "Failed to load Pages";
        setLoad({ kind: "error", message });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, pendingId, workspaceId, getToken]);

  const toggleOne = useCallback((pageId: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(pageId)) next.delete(pageId);
      else next.add(pageId);
      return next;
    });
  }, []);

  const handleFinalize = useCallback(async () => {
    if (load.kind !== "ready" || selected.size === 0 || submitting) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const token = await getToken();
      if (!token) {
        setSubmitError("Not signed in");
        return;
      }
      const res = await finalizePendingConnection(token, workspaceId, pendingId, Array.from(selected));
      // Block any further refetches: the pending row is gone on the
      // server, so the next GET would 404 and flash an error banner
      // on the modal right before the parent unmounts us.
      finalizedRef.current = true;
      onFinalized(res.data.connected_count);
    } catch (err) {
      setSubmitError(err instanceof Error ? err.message : "Failed to connect selected Pages");
    } finally {
      setSubmitting(false);
    }
  }, [load, selected, submitting, getToken, workspaceId, pendingId, onFinalized]);

  return (
    <Dialog open={open} onOpenChange={(o) => { if (!o) onClose(); }}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Select Facebook Pages to connect</DialogTitle>
          <DialogDescription>
            Each Page becomes its own UniPost account. You can come back later to add more.
          </DialogDescription>
        </DialogHeader>

        {load.kind === "loading" && (
          <div style={{ padding: 24, display: "flex", justifyContent: "center", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <Loader2 style={{ width: 14, height: 14 }} className="animate-spin" />
            Loading your Pages…
          </div>
        )}

        {load.kind === "error" && (
          <div style={{ padding: 18, borderRadius: 8, background: "color-mix(in srgb, var(--danger-soft) 82%, white)", border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)", color: "var(--danger)", fontSize: 13 }}>
            <AlertTriangle style={{ width: 14, height: 14, marginRight: 6, display: "inline" }} />
            {load.message}
          </div>
        )}

        {load.kind === "ready" && <PagesList data={load.data} selected={selected} onToggle={toggleOne} />}

        {submitError && (
          <div style={{ padding: "8px 12px", borderRadius: 6, background: "color-mix(in srgb, var(--danger-soft) 82%, white)", color: "var(--danger)", fontSize: 12 }}>
            {submitError}
          </div>
        )}

        <DialogFooter>
          <button type="button" className="dbtn dbtn-ghost" onClick={onClose} disabled={submitting}>
            Cancel
          </button>
          <button
            type="button"
            className="dbtn dbtn-primary"
            disabled={load.kind !== "ready" || selected.size === 0 || submitting}
            onClick={handleFinalize}
          >
            {submitting ? (
              <>
                <Loader2 style={{ width: 12, height: 12 }} className="animate-spin" />
                Connecting…
              </>
            ) : (
              <>
                <CheckCircle2 style={{ width: 12, height: 12 }} />
                Connect selected ({selected.size})
              </>
            )}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function PagesList({
  data,
  selected,
  onToggle,
}: {
  data: PendingConnection;
  selected: Set<string>;
  onToggle: (pageId: string) => void;
}) {
  if (data.pages.length === 0) {
    // PRD §15 "0 Pages" copy. Distinguish this from the "has Pages
    // but lacks publishing permission" case a level down, where
    // individual rows render disabled.
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <div style={{ fontSize: 14, fontWeight: 600, marginBottom: 8 }}>No Facebook Pages found</div>
        <div style={{ fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 14 }}>
          You need to be an admin of a Facebook Page to connect it to UniPost.
        </div>
        <a
          href="https://facebook.com/pages/create"
          target="_blank"
          rel="noreferrer"
          className="dbtn dbtn-ghost"
          style={{ fontSize: 12.5 }}
        >
          Create a Page →
        </a>
      </div>
    );
  }

  const anyPublishable = data.pages.some((p) => p.can_publish);
  if (!anyPublishable) {
    // PRD §15 "insufficient permissions" copy.
    return (
      <div style={{ padding: 18, borderRadius: 8, background: "color-mix(in srgb, var(--warning-soft, var(--surface1)) 82%, white)", border: "1px solid color-mix(in srgb, var(--warning, var(--dborder)) 40%, transparent)", fontSize: 12.5, lineHeight: 1.6 }}>
        <div style={{ fontWeight: 600, marginBottom: 6 }}>You don&rsquo;t have publishing permissions for any Pages</div>
        Ask the Page admin to grant you a Page role with content publishing access, then reconnect.
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6, maxHeight: 320, overflowY: "auto", padding: "4px 0" }}>
      {data.pages.map((page) => (
        <PageRow key={page.id} page={page} checked={selected.has(page.id)} onToggle={() => onToggle(page.id)} />
      ))}
    </div>
  );
}

function PageRow({ page, checked, onToggle }: { page: PendingFacebookPage; checked: boolean; onToggle: () => void }) {
  const disabled = !page.can_publish;
  return (
    <label
      style={{
        display: "flex",
        alignItems: "center",
        gap: 10,
        padding: "10px 12px",
        borderRadius: 8,
        border: checked ? "1px solid var(--daccent)" : "1px solid var(--dborder)",
        background: checked ? "color-mix(in srgb, var(--daccent) 12%, var(--surface1))" : "var(--surface1)",
        opacity: disabled ? 0.55 : 1,
        cursor: disabled ? "not-allowed" : "pointer",
      }}
    >
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        onChange={onToggle}
        style={{ cursor: disabled ? "not-allowed" : "pointer" }}
      />
      {page.picture_url ? (
        // eslint-disable-next-line @next/next/no-img-element
        <img src={page.picture_url} alt="" style={{ width: 32, height: 32, borderRadius: 16, objectFit: "cover", flexShrink: 0 }} />
      ) : (
        <div style={{ width: 32, height: 32, borderRadius: 16, background: "var(--surface2)", flexShrink: 0 }} />
      )}
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 13, fontWeight: 500, color: "var(--dtext)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
          {page.name}
        </div>
        <div style={{ fontSize: 11, color: "var(--dmuted)" }}>
          {page.category}
          {disabled ? " · no publish permission" : ""}
        </div>
      </div>
    </label>
  );
}

// Avoid a lint warning for Image being imported but only used by the
// picture fallback on some revs; keep it referenced until the
// next-image migration is done across the file.
void Image;
