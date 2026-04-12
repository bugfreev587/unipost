"use client";

import { useEffect, useRef } from "react";

interface ConfirmModalProps {
  open: boolean;
  title: string;
  message: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  variant?: "danger" | "default";
  confirmDisabled?: boolean;
  wide?: boolean;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmModal({
  open,
  title,
  message,
  confirmLabel = "Confirm",
  cancelLabel = "Cancel",
  variant = "default",
  confirmDisabled = false,
  wide = false,
  onConfirm,
  onCancel,
}: ConfirmModalProps) {
  const backdropRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") onCancel();
    }
    document.addEventListener("keydown", handleKey);
    return () => document.removeEventListener("keydown", handleKey);
  }, [open, onCancel]);

  if (!open) return null;

  const confirmBtnStyle = variant === "danger"
    ? { background: "var(--danger)", color: "#fff", borderColor: "transparent" }
    : { background: "var(--daccent)", color: "var(--primary-foreground)", borderColor: "transparent" };

  return (
    <div
      ref={backdropRef}
      onClick={(e) => { if (e.target === backdropRef.current) onCancel(); }}
      style={{
        position: "fixed", inset: 0, zIndex: 200,
        background: "var(--overlay)",
        backdropFilter: "blur(4px)",
        display: "flex", alignItems: "center", justifyContent: "center",
        animation: "fadeIn 0.15s ease",
      }}
    >
      <div
        style={{
          background: "var(--surface)",
          border: "1px solid var(--dborder2)",
          borderRadius: 12,
          width: wide ? 520 : 420, maxWidth: "90vw",
          padding: "24px 28px",
          boxShadow: "0 20px 50px var(--shadow-color)",
          animation: "slideUp 0.2s ease",
        }}
      >
        <div style={{ fontSize: 16, fontWeight: 700, color: "var(--dtext)", marginBottom: 8 }}>
          {title}
        </div>
        <div style={{ fontSize: 14, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 24 }}>
          {message}
        </div>
        <div style={{ display: "flex", justifyContent: "flex-end", gap: 10 }}>
          <button
            onClick={onCancel}
            className="dbtn dbtn-ghost"
            style={{ padding: "8px 20px", fontSize: 13 }}
          >
            {cancelLabel}
          </button>
          <button
            onClick={onConfirm}
            disabled={confirmDisabled}
            className="dbtn"
            style={{
              padding: "8px 20px",
              fontSize: 13,
              fontWeight: 600,
              ...confirmBtnStyle,
              opacity: confirmDisabled ? 0.5 : 1,
              cursor: confirmDisabled ? "not-allowed" : "pointer",
            }}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
