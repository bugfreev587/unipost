"use client";

import { useCallback, useEffect, useState } from "react";
import { useAuth, useUser } from "@clerk/nextjs";
import { Mail, Plus, Trash2, AlertCircle, CheckCircle2 } from "lucide-react";
import {
  listNotificationEvents,
  listNotificationChannels,
  listNotificationSubscriptions,
  createNotificationChannel,
  deleteNotificationChannel,
  upsertNotificationSubscription,
  type NotificationEvent,
  type NotificationChannel,
  type NotificationSubscription,
} from "@/lib/api";

// Severity → badge color. Mirrors backend severity field so adding a
// new level in Go shows up here with the right visual weight.
const SEVERITY_STYLES: Record<string, { bg: string; fg: string; label: string }> = {
  critical: { bg: "rgba(239,68,68,.12)", fg: "var(--danger)", label: "Critical" },
  high: { bg: "rgba(245,158,11,.14)", fg: "var(--warning)", label: "Important" },
  medium: { bg: "rgba(59,130,246,.14)", fg: "#60a5fa", label: "Informational" },
  low: { bg: "rgba(156,163,175,.14)", fg: "var(--dmuted)", label: "Low" },
};

// The matrix lets the user toggle each event × channel pair. For the
// MVP we only render one column (the first email channel) — the UI
// is designed to grow to a real matrix when Slack/SMS land, but the
// data model is already many-to-many so nothing here has to change.
export default function NotificationsSettingsPage() {
  const { getToken } = useAuth();
  const { user } = useUser();
  const [events, setEvents] = useState<NotificationEvent[]>([]);
  const [channels, setChannels] = useState<NotificationChannel[]>([]);
  const [subs, setSubs] = useState<NotificationSubscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState<Record<string, boolean>>({}); // keyed by `${eventType}:${channelId}`
  const [showAddEmail, setShowAddEmail] = useState(false);
  const [newEmail, setNewEmail] = useState("");
  const [addingEmail, setAddingEmail] = useState(false);

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

  useEffect(() => {
    load();
  }, [load]);

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

  async function handleAddEmail(e: React.FormEvent) {
    e.preventDefault();
    if (!newEmail.trim()) return;
    setAddingEmail(true);
    const token = await getToken();
    if (!token) return;
    try {
      await createNotificationChannel(token, { kind: "email", address: newEmail.trim() });
      setNewEmail("");
      setShowAddEmail(false);
      await load();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setAddingEmail(false);
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

  // Subscription lookup by (event_type, channel_id).
  function findSub(eventType: string, channelId: string): NotificationSubscription | null {
    return subs.find((s) => s.event_type === eventType && s.channel_id === channelId) ?? null;
  }

  if (loading) {
    return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;
  }

  const emailChannels = channels.filter((c) => c.kind === "email");
  const signupEmail = user?.primaryEmailAddress?.emailAddress;

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
              Where UniPost sends you notifications.
            </div>
          </div>
          {!showAddEmail && (
            <button
              type="button"
              onClick={() => setShowAddEmail(true)}
              style={btnSecondary}
            >
              <Plus style={{ width: 13, height: 13 }} /> Add email
            </button>
          )}
        </div>

        {showAddEmail && (
          <form
            onSubmit={handleAddEmail}
            style={{ display: "flex", gap: 8, marginBottom: 12, padding: 12, border: "1px solid var(--dborder)", borderRadius: 8, background: "var(--surface)" }}
          >
            <input
              type="email"
              required
              placeholder="you@example.com"
              value={newEmail}
              onChange={(e) => setNewEmail(e.target.value)}
              autoFocus
              style={{
                flex: 1, padding: "8px 12px", borderRadius: 6,
                border: "1px solid var(--dborder)", background: "var(--surface2)",
                color: "var(--dtext)", fontSize: 13, fontFamily: "inherit",
              }}
            />
            <button type="submit" disabled={addingEmail} style={btnPrimary}>
              {addingEmail ? "Adding..." : "Add"}
            </button>
            <button
              type="button"
              onClick={() => { setShowAddEmail(false); setNewEmail(""); }}
              style={btnGhost}
            >
              Cancel
            </button>
          </form>
        )}

        {channels.length === 0 ? (
          <div style={emptyBox}>
            No channels yet — add an email above to start receiving notifications.
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
                <Mail style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: 13, color: "var(--dtext)", fontWeight: 500 }}>
                    {c.config.address || "(unknown address)"}
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 8, marginTop: 2 }}>
                    {c.verified ? (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--daccent)" }}>
                        <CheckCircle2 style={{ width: 11, height: 11 }} /> Verified
                      </span>
                    ) : (
                      <span style={{ display: "inline-flex", alignItems: "center", gap: 4, fontSize: 11, color: "var(--warning)" }}>
                        <AlertCircle style={{ width: 11, height: 11 }} /> Unverified — notifications won&apos;t send
                      </span>
                    )}
                    {signupEmail && c.config.address === signupEmail && (
                      <span style={{ fontSize: 11, color: "var(--dmuted2)" }}>
                        · Signup email
                      </span>
                    )}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => handleDeleteChannel(c.id)}
                  title="Delete channel"
                  style={iconBtn}
                >
                  <Trash2 style={{ width: 14, height: 14 }} />
                </button>
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

        {emailChannels.length === 0 ? (
          <div style={emptyBox}>
            Add a channel above before subscribing to events.
          </div>
        ) : (
          <div style={{ border: "1px solid var(--dborder)", borderRadius: 10, overflow: "hidden" }}>
            {/* Header row */}
            <div
              style={{
                display: "grid",
                gridTemplateColumns: `1fr repeat(${emailChannels.length}, minmax(140px, auto))`,
                alignItems: "center",
                padding: "10px 14px",
                background: "var(--surface2)",
                borderBottom: "1px solid var(--dborder)",
                fontSize: 11, fontWeight: 700, letterSpacing: ".06em",
                textTransform: "uppercase", color: "var(--dmuted2)",
              }}
            >
              <div>Event</div>
              {emailChannels.map((c) => (
                <div key={c.id} style={{ textAlign: "center" }}>
                  {c.config.address || c.label || "Email"}
                </div>
              ))}
            </div>

            {/* Event rows */}
            {events.map((ev, idx) => (
              <div
                key={ev.event_type}
                style={{
                  display: "grid",
                  gridTemplateColumns: `1fr repeat(${emailChannels.length}, minmax(140px, auto))`,
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
                {emailChannels.map((c) => {
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
                          disabled={isBusy || !c.verified}
                          onChange={() => handleToggle(ev.event_type, c.id, enabled)}
                          style={{
                            width: 18, height: 18,
                            accentColor: "var(--daccent)",
                            cursor: c.verified ? "pointer" : "not-allowed",
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

// ── Styles (inline to match the rest of settings) ───────────────────

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
