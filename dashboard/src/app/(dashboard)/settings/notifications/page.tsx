"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth, useUser } from "@clerk/nextjs";
import { Mail, Hash, Plus, Trash2, AlertCircle, CheckCircle2, ChevronDown } from "lucide-react";
import {
  listNotificationEvents,
  listNotificationChannels,
  listNotificationSubscriptions,
  createNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
  upsertNotificationSubscription,
  type NotificationEvent,
  type NotificationChannel,
  type NotificationSubscription,
} from "@/lib/api";

const SEVERITY_STYLES: Record<string, { bg: string; fg: string; label: string }> = {
  critical: { bg: "rgba(239,68,68,.12)", fg: "var(--danger)", label: "Critical" },
  high: { bg: "rgba(245,158,11,.14)", fg: "var(--warning)", label: "Important" },
  medium: { bg: "rgba(59,130,246,.14)", fg: "#60a5fa", label: "Informational" },
  low: { bg: "rgba(156,163,175,.14)", fg: "var(--dmuted)", label: "Low" },
};

type AddChannelKind = "slack_webhook" | "discord_webhook" | null;

const CHANNEL_ICONS: Record<string, React.ReactNode> = {
  email: <Mail style={{ width: 15, height: 15, color: "var(--dmuted2)" }} />,
  slack_webhook: <SlackIcon />,
  discord_webhook: <DiscordIcon />,
};

const CHANNEL_LABELS: Record<string, string> = {
  email: "Email",
  slack_webhook: "Slack",
  discord_webhook: "Discord",
};

function channelDisplayName(c: NotificationChannel): string {
  return c.config.address || c.config.url || c.label || CHANNEL_LABELS[c.kind] || c.kind;
}

function channelColumnLabel(c: NotificationChannel): string {
  if (c.kind === "email") return c.config.address || "Email";
  return c.label || CHANNEL_LABELS[c.kind] || c.kind;
}

export default function NotificationsSettingsPage() {
  const { getToken } = useAuth();
  const { user } = useUser();
  const [events, setEvents] = useState<NotificationEvent[]>([]);
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [subs, setSubs] = useState<NotificationSubscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState<Record<string, boolean>>({});
  const [addKind, setAddKind] = useState<AddChannelKind>(null);
  const [addInput, setAddInput] = useState("");
  const [addLabel, setAddLabel] = useState("");
  const [adding, setAdding] = useState(false);
  const [showAddMenu, setShowAddMenu] = useState(false);
  const [testBusy, setTestBusy] = useState<Record<string, boolean>>({});
  const [testState, setTestState] = useState<Record<string, { kind: "success" | "error"; message: string }>>({});

  const load = useCallback(async () => {
    const token = await getToken();
    if (!token) return;
    try {
      const [ev, ch, su] = await Promise.all([
        listNotificationEvents(token),
        listNotificationChannels(token),
        listNotificationSubscriptions(token),
      ]);
      setEvents(ev.data || []);
      setChannels(ch.data || []);
      setSubs(su.data || []);
      setErr(null);
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => { load(); }, [load]);

  async function handleToggle(eventType: string, channelId: string, currentEnabled: boolean) {
    const key = `${eventType}:${channelId}`;
    setBusy((p) => ({ ...p, [key]: true }));
    const token = await getToken();
    if (!token) return;
    try {
      await upsertNotificationSubscription(token, {
        event_type: eventType,
        channel_id: channelId,
        enabled: !currentEnabled,
      });
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy((p) => ({ ...p, [key]: false }));
    }
  }

  async function handleAddChannel(e: React.FormEvent) {
    e.preventDefault();
    if (!addKind || !addInput.trim()) return;
    setAdding(true);
    const token = await getToken();
    if (!token) return;
    try {
      const data: Record<string, string> = { kind: addKind };
      data.url = addInput.trim();
      if (addLabel.trim()) data.label = addLabel.trim();
      await createNotificationChannel(token, data as Parameters<typeof createNotificationChannel>[1]);
      setAddInput("");
      setAddLabel("");
      setAddKind(null);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setAdding(false);
    }
  }

  async function handleDeleteChannel(id: string) {
    if (!confirm("Delete this channel? Related subscriptions will also be removed.")) return;
    const token = await getToken();
    if (!token) return;
    try {
      await deleteNotificationChannel(token, id);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    }
  }

  async function handleTestChannel(channelId: string) {
    setTestBusy((p) => ({ ...p, [channelId]: true }));
    setTestState((p) => {
      const next = { ...p };
      delete next[channelId];
      return next;
    });
    const token = await getToken();
    if (!token) return;
    try {
      const res = await testNotificationChannel(token, channelId);
      setTestState((p) => ({
        ...p,
        [channelId]: { kind: "success", message: res.data.message || "Test sent." },
      }));
    } catch (e) {
      setTestState((p) => ({
        ...p,
        [channelId]: { kind: "error", message: e instanceof Error ? e.message : String(e) },
      }));
    } finally {
      setTestBusy((p) => ({ ...p, [channelId]: false }));
    }
  }

  function findSub(eventType: string, channelId: string): NotificationSubscription | null {
    return subs.find((s) => s.event_type === eventType && s.channel_id === channelId) ?? null;
  }

  if (loading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  const verifiedChannels = channels.filter((c) => c.verified);
  const signupEmail = user?.primaryEmailAddress?.emailAddress;

  const addPlaceholder: Record<Exclude<AddChannelKind, null>, string> = {
    slack_webhook: "https://hooks.slack.com/services/...",
    discord_webhook: "https://discord.com/api/webhooks/...",
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 28 }}>
      {err && (
        <div style={{
          padding: "10px 14px", borderRadius: 8,
          background: "var(--danger-soft)", color: "var(--danger)",
          border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)",
          fontSize: 13, display: "flex", alignItems: "center", gap: 8,
        }}>
          <AlertCircle style={{ width: 14, height: 14 }} />
          {err}
          <button type="button" onClick={() => setErr(null)} style={{ marginLeft: "auto", background: "none", border: "none", color: "inherit", cursor: "pointer", fontSize: 16 }}>×</button>
        </div>
      )}

      {/* ── Channels ── */}
      <section>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
          <div>
            <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 4 }}>
              Channels
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
              Your signup email is already connected. Add Slack or Discord for shared alerts.
            </div>
          </div>
          {!addKind && (
            <div style={{ position: "relative" }}>
              <button
                type="button"
                onClick={() => setShowAddMenu(!showAddMenu)}
                style={btnSecondary}
              >
                <Plus style={{ width: 13, height: 13 }} /> Add channel <ChevronDown style={{ width: 12, height: 12 }} />
              </button>
              {showAddMenu && (
                <>
                  <div style={{ position: "fixed", inset: 0, zIndex: 50 }} onClick={() => setShowAddMenu(false)} />
                  <div style={{
                    position: "absolute", right: 0, top: "calc(100% + 4px)", zIndex: 51,
                    minWidth: 180, padding: 4, borderRadius: 8,
                    background: "var(--surface-raised)", border: "1px solid var(--dborder)",
                    boxShadow: "0 12px 28px rgba(0,0,0,.3)",
                  }}>
                    {([
                      { kind: "slack_webhook" as const, label: "Slack Webhook", icon: <SlackIcon /> },
                      { kind: "discord_webhook" as const, label: "Discord Webhook", icon: <DiscordIcon /> },
                    ]).map((opt) => (
                      <button
                        key={opt.kind}
                        type="button"
                        onClick={() => { setAddKind(opt.kind); setShowAddMenu(false); }}
                        style={{
                          display: "flex", alignItems: "center", gap: 10, width: "100%",
                          padding: "9px 12px", borderRadius: 6, border: "none",
                          background: "transparent", color: "var(--dtext)",
                          fontSize: 13, cursor: "pointer", fontFamily: "inherit",
                          textAlign: "left",
                        }}
                        onMouseEnter={(e) => { e.currentTarget.style.background = "var(--surface2)"; }}
                        onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                      >
                        {opt.icon} {opt.label}
                      </button>
                    ))}
                  </div>
                </>
              )}
            </div>
          )}
        </div>

        {addKind && (
          <form
            onSubmit={handleAddChannel}
            style={{ display: "flex", flexDirection: "column", gap: 8, marginBottom: 12, padding: 14, border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface)" }}
          >
            <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)", display: "flex", alignItems: "center", gap: 8 }}>
              {CHANNEL_ICONS[addKind]} Add {CHANNEL_LABELS[addKind]}
            </div>
            <input
              type="url"
              required
              placeholder={addPlaceholder[addKind]}
              value={addInput}
              onChange={(e) => setAddInput(e.target.value)}
              autoFocus
              style={{
                padding: "8px 12px", borderRadius: 6,
                border: "1px solid var(--dborder)", background: "var(--surface2)",
                color: "var(--dtext)", fontSize: 13, fontFamily: "var(--font-geist-mono), monospace",
              }}
            />
            <input
              type="text"
              placeholder="Label (optional, e.g. #ops-alerts)"
              value={addLabel}
              onChange={(e) => setAddLabel(e.target.value)}
              style={{
                padding: "8px 12px", borderRadius: 6,
                border: "1px solid var(--dborder)", background: "var(--surface2)",
                color: "var(--dtext)", fontSize: 13, fontFamily: "inherit",
              }}
            />
            <div style={{ display: "flex", gap: 8 }}>
              <button type="submit" disabled={adding} style={btnPrimary}>
                {adding ? "Adding..." : "Add"}
              </button>
              <button
                type="button"
                onClick={() => { setAddKind(null); setAddInput(""); setAddLabel(""); }}
                style={btnGhost}
              >
                Cancel
              </button>
            </div>
          </form>
        )}

        {channels.length === 0 ? (
          <div style={emptyBox}>
            No channels yet — add one above to start receiving notifications.
          </div>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {channels.map((c) => (
              <div
                key={c.id}
                style={{
                  display: "flex", alignItems: "center", gap: 12,
                  padding: "12px 14px", borderRadius: 8,
                  border: "1px solid var(--dborder)", background: "var(--surface)",
                }}
              >
                {CHANNEL_ICONS[c.kind] || <Hash style={{ width: 15, height: 15, color: "var(--dmuted2)" }} />}
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 500, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {channelDisplayName(c)}
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 2, flexWrap: "wrap" }}>
                    <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>
                      {CHANNEL_LABELS[c.kind] || c.kind}
                    </span>
                    {c.verified ? (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--daccent)" }}>
                        <CheckCircle2 style={{ width: 11, height: 11 }} /> Verified
                      </span>
                    ) : (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--warning)" }}>
                        <AlertCircle style={{ width: 11, height: 11 }} /> Unverified
                      </span>
                    )}
                    {signupEmail && c.config.address === signupEmail && (
                      <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>· Built-in signup email</span>
                    )}
                    {c.label && c.kind !== "email" && (
                      <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>· {c.label}</span>
                    )}
                  </div>
                  {testState[c.id] && (
                    <div
                      style={{
                        marginTop: 6,
                        fontSize: 11,
                        color: testState[c.id].kind === "success" ? "var(--daccent)" : "var(--danger)",
                      }}
                    >
                      {testState[c.id].message}
                    </div>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => handleTestChannel(c.id)}
                  disabled={!!testBusy[c.id]}
                  style={{
                    ...btnGhost,
                    padding: "7px 10px",
                    fontSize: 12,
                    color: testBusy[c.id] ? "var(--dmuted2)" : "var(--dtext)",
                    opacity: testBusy[c.id] ? 0.7 : 1,
                  }}
                >
                  {testBusy[c.id] ? "Testing..." : "Test"}
                </button>
                {c.kind !== "email" && (
                  <button
                    type="button"
                    onClick={() => handleDeleteChannel(c.id)}
                    title="Delete channel"
                    style={iconBtn}
                  >
                    <Trash2 style={{ width: 14, height: 14 }} />
                  </button>
                )}
              </div>
            ))}
          </div>
        )}
      </section>

      {/* ── Subscriptions matrix ── */}
      <section>
        <div style={{ marginBottom: 10 }}>
          <div style={{ fontSize: 16, fontWeight: 600, color: "var(--dtext)", marginBottom: 4 }}>
            Subscriptions
          </div>
          <div style={{ fontSize: 13, color: "var(--dmuted)" }}>
            Pick which events get delivered to which channels.
          </div>
        </div>

        {verifiedChannels.length === 0 ? (
          <div style={emptyBox}>
            Add and verify a channel above before subscribing to events.
          </div>
        ) : (
          <div style={{ border: "1px solid var(--dborder)", borderRadius: 10, overflow: "hidden" }}>
            <div
              style={{
                display: "grid",
                gridTemplateColumns: `1fr repeat(${verifiedChannels.length}, minmax(100px, auto))`,
                alignItems: "center",
                padding: "10px 14px",
                background: "var(--surface2)",
                borderBottom: "1px solid var(--dborder)",
                fontSize: 11, fontWeight: 700, letterSpacing: ".06em",
                textTransform: "uppercase", color: "var(--dmuted2)",
              }}
            >
              <div>Event</div>
              {verifiedChannels.map((c) => (
                <div key={c.id} style={{ textAlign: "center", display: "flex", flexDirection: "column", alignItems: "center", gap: 2 }}>
                  {CHANNEL_ICONS[c.kind]}
                  <span style={{ fontSize: 10, maxWidth: 120, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {channelColumnLabel(c)}
                  </span>
                </div>
              ))}
            </div>

            {events.map((ev, idx) => (
              <div
                key={ev.event_type}
                style={{
                  display: "grid",
                  gridTemplateColumns: `1fr repeat(${verifiedChannels.length}, minmax(100px, auto))`,
                  alignItems: "center",
                  padding: "14px",
                  borderBottom: idx === events.length - 1 ? "none" : "1px solid var(--dborder)",
                  background: "var(--surface)",
                }}
              >
                <div style={{ paddingRight: 16 }}>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 4 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)" }}>
                      {ev.label}
                    </div>
                    <span style={{
                      fontSize: 10, fontWeight: 600, padding: "2px 6px", borderRadius: 4,
                      background: SEVERITY_STYLES[ev.severity]?.bg || "var(--surface2)",
                      color: SEVERITY_STYLES[ev.severity]?.fg || "var(--dmuted)",
                    }}>
                      {SEVERITY_STYLES[ev.severity]?.label || ev.severity}
                    </span>
                  </div>
                  <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
                    {ev.description}
                  </div>
                </div>
                {verifiedChannels.map((c) => {
                  const sub = findSub(ev.event_type, c.id);
                  const enabled = !!sub?.enabled;
                  const key = `${ev.event_type}:${c.id}`;
                  const isBusy = !!busy[key];
                  return (
                    <div key={c.id} style={{ textAlign: "center" }}>
                      <label style={{ display: "inline-flex", cursor: "pointer", alignItems: "center" }}>
                        <input
                          type="checkbox"
                          checked={enabled}
                          disabled={isBusy}
                          onChange={() => handleToggle(ev.event_type, c.id, enabled)}
                          style={{
                            width: 18, height: 18,
                            accentColor: "var(--daccent)",
                            cursor: "pointer",
                            opacity: isBusy ? 0.5 : 1,
                          }}
                        />
                      </label>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

// ── Inline SVG icons for Slack and Discord (no external deps) ────────

function SlackIcon() {
  return (
    <svg viewBox="0 0 24 24" width={15} height={15} fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={{ color: "var(--dmuted2)" }}>
      <rect x="13" y="2" width="3" height="8" rx="1.5" /><rect x="8" y="14" width="3" height="8" rx="1.5" />
      <rect x="2" y="8" width="8" height="3" rx="1.5" /><rect x="14" y="13" width="8" height="3" rx="1.5" />
    </svg>
  );
}

function DiscordIcon() {
  return (
    <svg viewBox="0 0 24 24" width={15} height={15} fill="currentColor" style={{ color: "var(--dmuted2)" }}>
      <path d="M19.27 5.33C17.94 4.71 16.5 4.26 15 4a.09.09 0 0 0-.07.03c-.18.33-.39.76-.53 1.09a16.09 16.09 0 0 0-4.8 0c-.14-.34-.35-.76-.54-1.09-.01-.02-.04-.03-.07-.03-1.5.26-2.93.71-4.27 1.33-.01 0-.02.01-.03.02-2.72 4.07-3.47 8.03-3.1 11.95 0 .02.01.04.03.05 1.8 1.32 3.53 2.12 5.24 2.65.03.01.06 0 .07-.02.4-.55.76-1.13 1.07-1.74.02-.04 0-.08-.04-.09-.57-.22-1.11-.48-1.64-.78-.04-.02-.04-.08-.01-.11.11-.08.22-.17.33-.25.02-.02.05-.02.07-.01 3.44 1.57 7.15 1.57 10.55 0 .02-.01.05-.01.07.01.11.09.22.17.33.26.04.03.04.09-.01.11-.52.31-1.07.56-1.64.78-.04.01-.05.06-.04.09.32.61.68 1.19 1.07 1.74.02.03.05.03.07.02 1.72-.53 3.45-1.33 5.25-2.65.02-.01.03-.03.03-.05.44-4.53-.73-8.46-3.1-11.95-.01-.01-.02-.02-.04-.02zM8.52 14.91c-1.03 0-1.89-.95-1.89-2.12s.84-2.12 1.89-2.12c1.06 0 1.9.96 1.89 2.12 0 1.17-.84 2.12-1.89 2.12zm6.97 0c-1.03 0-1.89-.95-1.89-2.12s.84-2.12 1.89-2.12c1.06 0 1.9.96 1.89 2.12 0 1.17-.83 2.12-1.89 2.12z" />
    </svg>
  );
}

// ── Styles ───────────────────────────────────────────────────────────

const btnPrimary: React.CSSProperties = {
  display: "inline-flex", alignItems: "center", gap: 6,
  padding: "8px 14px", borderRadius: 6, fontSize: 13, fontWeight: 600,
  background: "var(--daccent)", color: "var(--primary-foreground)",
  border: "none", cursor: "pointer", fontFamily: "inherit",
};

const btnSecondary: React.CSSProperties = {
  display: "inline-flex", alignItems: "center", gap: 6,
  padding: "7px 12px", borderRadius: 6, fontSize: 13, fontWeight: 500,
  background: "var(--surface2)", color: "var(--dtext)",
  border: "1px solid var(--dborder)", cursor: "pointer", fontFamily: "inherit",
};

const btnGhost: React.CSSProperties = {
  padding: "8px 12px", borderRadius: 6, fontSize: 13, fontWeight: 500,
  background: "transparent", color: "var(--dmuted)",
  border: "1px solid var(--dborder)", cursor: "pointer", fontFamily: "inherit",
};

const iconBtn: React.CSSProperties = {
  width: 32, height: 32, borderRadius: 6,
  display: "inline-flex", alignItems: "center", justifyContent: "center",
  background: "transparent", color: "var(--dmuted)",
  border: "1px solid transparent", cursor: "pointer",
};

const emptyBox: React.CSSProperties = {
  padding: "16px 14px", borderRadius: 8,
  border: "1px dashed var(--dborder)", background: "var(--surface)",
  color: "var(--dmuted)", fontSize: 13, textAlign: "center",
};
