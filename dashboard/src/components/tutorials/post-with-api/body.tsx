"use client";

// post_with_api tutorial body. Two steps:
//
//   1. Create an API key. If the user already has keys (api_keys count
//      >= 1), we offer either "paste your existing key" or "create a
//      new Tutorial key anyway" (Option A in the design doc — we never
//      re-show a plaintext key, so resumes require the user to supply
//      it or create a fresh one).
//
//   2. Show a language-tabbed code block using the key + one of the
//      user's connected account IDs, with a live "Send post" button.
//      After a successful send, reveal DONE. Clicking DONE calls
//      onRequestComplete, which marks the tutorial complete on the
//      server and shows the celebration screen.

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { Check, Lock, Loader2, ExternalLink, Copy } from "lucide-react";
import {
  createApiKey,
  listSocialAccounts,
  type ApiKeyCreateResponse,
  type SocialAccount,
} from "@/lib/api";
import { useCurrentWorkspace } from "@/lib/use-current-workspace";
import type { TutorialBodyProps } from "../registry";
import { CodeBlock } from "./code-block";

const API_BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
const APP_BASE = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";
const DEFAULT_CAPTION = "Hello from UniPost API!";
const VERIFIED_TUTORIAL_MEDIA_URL = "https://media.unipost.dev/media/e7920ed2-3ffc-4493-a467-3c3fa3457e08.jpg";
const TUTORIAL_MEDIA_URLS: Partial<Record<SocialAccount["platform"], string>> = {
  instagram: VERIFIED_TUTORIAL_MEDIA_URL,
  threads: VERIFIED_TUTORIAL_MEDIA_URL,
  linkedin: VERIFIED_TUTORIAL_MEDIA_URL,
  twitter: `${APP_BASE}/brand/unipost-icon-light.png`,
  bluesky: VERIFIED_TUTORIAL_MEDIA_URL,
};

export function PostWithApiBody({ ctx, steps, onRequestComplete }: TutorialBodyProps) {
  const { getToken } = useAuth();
  const { workspace } = useCurrentWorkspace();

  // Key state: plaintext lives in component memory. If the user closes
  // the modal and reopens, they'll be prompted to paste or create
  // another (matches Option A).
  const [keyState, setKeyState] = useState<
    | { kind: "none" }
    | { kind: "creating" }
    | { kind: "ready"; key: string; prefix?: string }
    | { kind: "error"; message: string }
  >({ kind: "none" });
  const [pasteInput, setPasteInput] = useState("");
  const [keyCopied, setKeyCopied] = useState(false);

  // Load the user's connected accounts so we can template a real
  // account_id into the snippet. Quickstart is a prerequisite, so we
  // know at least one exists.
  const [account, setAccount] = useState<SocialAccount | null>(null);
  const [accountError, setAccountError] = useState("");
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listSocialAccounts(token, ctx.profileId);
        if (cancelled) return;
        const active = (res.data || []).find((a) => a.status === "active");
        if (active) {
          setAccount(active);
        } else {
          setAccountError(
            "No active connected accounts found. Connect one in the Accounts page, then come back.",
          );
        }
      } catch (err) {
        if (!cancelled) setAccountError((err as Error).message || "Failed to load accounts");
      }
    })();
    return () => { cancelled = true; };
  }, [getToken, ctx.profileId]);

  // Send state.
  const [sendState, setSendState] = useState<
    | { kind: "idle" }
    | { kind: "sending" }
    | { kind: "sent"; postId: string; permalink?: string }
    | { kind: "error"; message: string }
  >({ kind: "idle" });

  const apiKeysAlreadyExist = ctx.counts.api_keys >= 1;
  const step1Completed = keyState.kind === "ready";
  const step2Completed = sendState.kind === "sent";
  const tutorialMediaUrl = account ? TUTORIAL_MEDIA_URLS[account.platform] : undefined;
  const shouldAttachTutorialImage = !!tutorialMediaUrl;

  async function handleCreateKey() {
    if (!workspace) {
      setKeyState({ kind: "error", message: "Workspace still loading. Try again in a moment." });
      return;
    }
    setKeyState({ kind: "creating" });
    try {
      const token = await getToken();
      if (!token) {
        setKeyState({ kind: "error", message: "Session expired. Please sign in again." });
        return;
      }
      const res = await createApiKey(token, workspace.id, {
        name: "Tutorial key",
        environment: "production",
      });
      const created: ApiKeyCreateResponse = res.data;
      setKeyState({ kind: "ready", key: created.key, prefix: created.prefix });
    } catch (err) {
      setKeyState({ kind: "error", message: (err as Error).message || "Failed to create API key" });
    }
  }

  function handleUsePastedKey() {
    const trimmed = pasteInput.trim();
    if (!trimmed) return;
    setKeyState({ kind: "ready", key: trimmed });
  }

  async function handleCopyKey() {
    if (keyState.kind !== "ready") return;
    try {
      await navigator.clipboard.writeText(keyState.key);
      setKeyCopied(true);
      setTimeout(() => setKeyCopied(false), 1500);
    } catch { /* clipboard unavailable — silent */ }
  }

  async function handleSend() {
    if (keyState.kind !== "ready" || !account) return;
    setSendState({ kind: "sending" });
    try {
      const res = await fetch(`${API_BASE}/v1/social-posts`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${keyState.key}`,
        },
        body: JSON.stringify({
          caption: DEFAULT_CAPTION,
          account_ids: [account.id],
          ...(tutorialMediaUrl ? { media_urls: [tutorialMediaUrl] } : {}),
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        const msg = (body as { error?: { message?: string } })?.error?.message
          || `Request failed: ${res.status}`;
        setSendState({ kind: "error", message: msg });
        return;
      }
      const body = await res.json();
      const data = (body as { data?: { id?: string; results?: Array<{ permalink?: string; url?: string }> } }).data || {};
      const permalink = data.results?.[0]?.permalink || data.results?.[0]?.url;
      setSendState({ kind: "sent", postId: data.id || "", permalink });
    } catch (err) {
      setSendState({ kind: "error", message: (err as Error).message || "Failed to send post" });
    }
  }

  return (
    <div style={{ display: "grid", gap: 16, width: "100%", minWidth: 0 }}>
      {/* Step 1: Create API key */}
      <StepCard
        number={1}
        title={steps[0]?.title || "Create an API key"}
        completed={step1Completed}
        active={!step1Completed}
      >
        {keyState.kind === "ready" ? (
          <div style={{ width: "100%", minWidth: 0 }}>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 8 }}>
              Use this key to authenticate your request. We&apos;ll only show it once —
              store it somewhere safe.
            </div>
            <div style={{
              width: "100%",
              minWidth: 0,
              boxSizing: "border-box",
              padding: "10px 12px", borderRadius: 8,
              background: "rgba(16,185,129,.06)",
              border: "1px solid rgba(16,185,129,.20)",
              display: "flex",
              alignItems: "center",
              gap: 10,
            }}>
              <div
                style={{
                  flex: 1,
                  minWidth: 0,
                  fontFamily: "var(--font-geist-mono), monospace",
                  fontSize: 12,
                  color: "var(--dtext)",
                  wordBreak: "break-all",
                }}
              >
                {keyState.key}
              </div>
              <button
                type="button"
                onClick={handleCopyKey}
                className="dt-body-sm"
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  gap: 6,
                  flexShrink: 0,
                  padding: "6px 10px",
                  borderRadius: 6,
                  border: "1px solid rgba(16,185,129,.24)",
                  background: "rgba(16,185,129,.10)",
                  color: keyCopied ? "var(--daccent)" : "var(--dmuted)",
                  cursor: "pointer",
                  fontFamily: "inherit",
                  fontSize: 12,
                }}
              >
                {keyCopied ? <Check style={{ width: 12, height: 12 }} /> : <Copy style={{ width: 12, height: 12 }} />}
                {keyCopied ? "Copied" : "Copy"}
              </button>
            </div>
          </div>
        ) : apiKeysAlreadyExist ? (
          <div style={{ width: "100%", minWidth: 0 }}>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 10 }}>
              You already have at least one API key. Paste it below to continue,
              or create a new <strong>Tutorial key</strong>.
            </div>
            <div style={{ display: "flex", gap: 8, marginBottom: 10, width: "100%", minWidth: 0 }}>
              <input
                type="text"
                placeholder="unp_prod_..."
                value={pasteInput}
                onChange={(e) => setPasteInput(e.target.value)}
                style={{
                  flex: 1, padding: "8px 12px", fontSize: 12,
                  background: "#0a0a0c", border: "1px solid var(--dborder)", borderRadius: 8,
                  color: "var(--dtext)", outline: "none",
                  fontFamily: "var(--font-geist-mono), monospace",
                }}
              />
              <button
                type="button"
                onClick={handleUsePastedKey}
                disabled={!pasteInput.trim()}
                className="dt-body-sm"
                style={{
                  padding: "8px 14px", borderRadius: 8,
                  border: "1px solid var(--dborder)",
                  background: "transparent", color: "var(--dtext)",
                  cursor: pasteInput.trim() ? "pointer" : "not-allowed",
                  fontFamily: "inherit", fontWeight: 500,
                  opacity: pasteInput.trim() ? 1 : 0.5,
                }}
              >
                Use this key
              </button>
            </div>
            <button
              type="button"
              onClick={handleCreateKey}
              disabled={keyState.kind === "creating"}
              className="dt-body-sm"
              style={{
                padding: "8px 14px", borderRadius: 8,
                border: "none",
                background: "var(--daccent)", color: "var(--primary-foreground)",
                cursor: "pointer", fontFamily: "inherit", fontWeight: 600,
                display: "inline-flex", alignItems: "center", gap: 6,
              }}
            >
              {keyState.kind === "creating" && <Loader2 style={{ width: 12, height: 12, animation: "spin 1s linear infinite" }} />}
              Create a Tutorial key
            </button>
          </div>
        ) : (
          <div style={{ width: "100%", minWidth: 0 }}>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 10 }}>
              Used to authenticate requests from your app.
            </div>
            <button
              type="button"
              onClick={handleCreateKey}
              disabled={keyState.kind === "creating"}
              className="dt-body-sm"
              style={{
                padding: "8px 14px", borderRadius: 8,
                border: "none",
                background: "var(--daccent)", color: "var(--primary-foreground)",
                cursor: "pointer", fontFamily: "inherit", fontWeight: 600,
                display: "inline-flex", alignItems: "center", gap: 6,
              }}
            >
              {keyState.kind === "creating" && <Loader2 style={{ width: 12, height: 12, animation: "spin 1s linear infinite" }} />}
              Create API key
            </button>
          </div>
        )}
        {keyState.kind === "error" && (
          <div style={{ marginTop: 10, fontSize: 13, color: "var(--danger)" }}>
            {keyState.message}
          </div>
        )}
      </StepCard>

      {/* Step 2: Send post */}
      <StepCard
        number={2}
        title={steps[1]?.title || "Send a post"}
        completed={step2Completed}
        active={step1Completed && !step2Completed}
        locked={!step1Completed}
      >
        {accountError ? (
          <div style={{ fontSize: 13, color: "var(--danger)" }}>{accountError}</div>
        ) : !account ? (
          <div className="dt-body-sm" style={{ color: "var(--dmuted)" }}>Loading your connected account…</div>
        ) : (
          <div style={{ width: "100%", minWidth: 0 }}>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginBottom: 10 }}>
              Run this to {shouldAttachTutorialImage ? "publish a photo post" : "post"}
              {" "}
              <strong style={{ color: "var(--dtext)" }}>&quot;{DEFAULT_CAPTION}&quot;</strong>
              {" "}to <strong style={{ color: "var(--dtext)" }}>@{account.account_name || account.platform}</strong> —
              or click Send below to try it now.
            </div>
            <CodeBlock
              apiBase={API_BASE}
              apiKey={keyState.kind === "ready" ? keyState.key : "your_api_key"}
              accountId={account.id}
              caption={DEFAULT_CAPTION}
              mediaUrl={tutorialMediaUrl}
            />
            <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 12, flexWrap: "wrap" }}>
              <button
                type="button"
                onClick={handleSend}
                disabled={keyState.kind !== "ready" || sendState.kind === "sending" || sendState.kind === "sent"}
                className="dt-body-sm"
                style={{
                  padding: "10px 18px", borderRadius: 8,
                  border: "none",
                  background: sendState.kind === "sent" ? "rgba(16,185,129,.15)" : "var(--daccent)",
                  color: sendState.kind === "sent" ? "var(--daccent)" : "var(--primary-foreground)",
                  cursor: (keyState.kind === "ready" && sendState.kind === "idle") ? "pointer" : "not-allowed",
                  fontFamily: "inherit", fontWeight: 600,
                  display: "inline-flex", alignItems: "center", gap: 8,
                  opacity: keyState.kind !== "ready" ? 0.5 : 1,
                }}
              >
                {sendState.kind === "sending" && <Loader2 style={{ width: 14, height: 14, animation: "spin 1s linear infinite" }} />}
                {sendState.kind === "sent" && <Check style={{ width: 14, height: 14 }} />}
                {sendState.kind === "sending" ? "Sending…" : sendState.kind === "sent" ? "Sent" : "Send post"}
              </button>
              {sendState.kind === "sent" && sendState.permalink && (
                <a
                  href={sendState.permalink}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="dt-body-sm"
                  style={{
                    display: "inline-flex", alignItems: "center", gap: 4,
                    color: "var(--daccent)", textDecoration: "none",
                  }}
                >
                  View post <ExternalLink style={{ width: 12, height: 12 }} />
                </a>
              )}
            </div>
            {sendState.kind === "error" && (
              <div style={{
                marginTop: 10, padding: "10px 12px",
                borderRadius: 8, fontSize: 13, color: "var(--danger)",
                background: "rgba(239,68,68,.06)",
                border: "1px solid rgba(239,68,68,.20)",
              }}>
                {sendState.message}
              </div>
            )}
          </div>
        )}
      </StepCard>

      {/* Done button — only enabled after a successful send */}
      <div style={{ display: "flex", justifyContent: "flex-end" }}>
        <button
          type="button"
          onClick={onRequestComplete}
          disabled={!step2Completed}
          className="dt-body-sm"
          style={{
            padding: "10px 20px", borderRadius: 8,
            border: "none",
            background: step2Completed ? "var(--daccent)" : "var(--sidebar-accent)",
            color: step2Completed ? "var(--primary-foreground)" : "var(--dmuted)",
            cursor: step2Completed ? "pointer" : "not-allowed",
            fontFamily: "inherit", fontWeight: 600,
          }}
        >
          Done
        </button>
      </div>

      <style>{`@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }`}</style>
    </div>
  );
}

function StepCard({
  number,
  title,
  completed,
  active,
  locked,
  children,
}: {
  number: number;
  title: string;
  completed: boolean;
  active?: boolean;
  locked?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div style={{
      width: "100%",
      minWidth: 0,
      boxSizing: "border-box",
      padding: 14,
      borderRadius: 10,
      border: active
        ? "1px solid rgba(16,185,129,.25)"
        : "1px solid var(--dborder)",
      background: active ? "rgba(16,185,129,.04)" : "transparent",
      opacity: locked ? 0.55 : 1,
    }}>
      <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
        <div style={{
          width: 22, height: 22, borderRadius: "50%",
          display: "flex", alignItems: "center", justifyContent: "center",
          flexShrink: 0,
          ...(completed
            ? { background: "var(--daccent)", color: "var(--primary-foreground)" }
            : locked
              ? { border: "1px solid var(--dborder)", color: "var(--dmuted2)" }
              : { border: "2px solid var(--daccent)", color: "var(--daccent)", fontSize: 11, fontWeight: 700 }),
        }}>
          {completed ? (
            <Check style={{ width: 13, height: 13 }} strokeWidth={3} />
          ) : locked ? (
            <Lock style={{ width: 11, height: 11 }} />
          ) : (
            number
          )}
        </div>
        <div className="dt-body-sm" style={{
          fontWeight: 600,
          color: completed ? "var(--dmuted)" : "var(--dtext)",
          textDecoration: completed ? "line-through" : "none",
        }}>
          {title}
        </div>
      </div>
      {!locked && <div style={{ paddingLeft: 32, minWidth: 0, overflow: "hidden" }}>{children}</div>}
    </div>
  );
}
