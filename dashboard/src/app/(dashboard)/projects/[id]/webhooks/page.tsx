"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  createWebhook,
  deleteWebhook,
  listWebhooks,
  rotateWebhookSecret,
  updateWebhook,
  type WebhookSubscription,
} from "@/lib/api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ConfirmModal } from "@/components/confirm-modal";
import {
  AlertTriangle,
  CheckCircle2,
  Copy,
  KeyRound,
  Pencil,
  Plus,
  RotateCw,
  ShieldCheck,
  Trash2,
  Webhook,
} from "lucide-react";

type WebhookEventOption = {
  id: string;
  title: string;
  description: string;
  group: "Publishing" | "Accounts";
};

const EVENT_OPTIONS: WebhookEventOption[] = [
  {
    id: "post.published",
    title: "post.published",
    description: "Every platform result finished successfully.",
    group: "Publishing",
  },
  {
    id: "post.partial",
    title: "post.partial",
    description: "At least one platform succeeded and at least one failed.",
    group: "Publishing",
  },
  {
    id: "post.failed",
    title: "post.failed",
    description: "All platform results ended in failure.",
    group: "Publishing",
  },
  {
    id: "account.connected",
    title: "account.connected",
    description: "A user connected a new account from Connect or the dashboard.",
    group: "Accounts",
  },
  {
    id: "account.disconnected",
    title: "account.disconnected",
    description: "An account was disconnected or permanently expired.",
    group: "Accounts",
  },
];

type SecretRevealState = {
  secret: string;
  webhookId: string;
  preview: string;
} | null;

const GROUPS: Array<WebhookEventOption["group"]> = ["Publishing", "Accounts"];

function formatDate(value: string) {
  return new Date(value).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function badgeStyle(active: boolean) {
  return {
    display: "inline-flex",
    alignItems: "center",
    gap: 6,
    padding: "4px 10px",
    borderRadius: 999,
    fontSize: 12,
    fontWeight: 600,
    border: "1px solid",
    background: active ? "rgba(16,185,129,.12)" : "rgba(148,163,184,.12)",
    borderColor: active ? "rgba(16,185,129,.18)" : "rgba(148,163,184,.18)",
    color: active ? "#10b981" : "var(--dmuted)",
  } as const;
}

export default function ProjectWebhooksPage() {
  const workspaceId = useWorkspaceId();
  const { getToken } = useAuth();
  const [webhooks, setWebhooks] = useState<WebhookSubscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<WebhookSubscription | null>(null);
  const [saving, setSaving] = useState(false);
  const [name, setName] = useState("");
  const [url, setURL] = useState("");
  const [events, setEvents] = useState<string[]>([
    "post.published",
    "post.partial",
    "post.failed",
  ]);
  const [createActive, setCreateActive] = useState(true);
  const [customSecret, setCustomSecret] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<WebhookSubscription | null>(null);
  const [rotateTarget, setRotateTarget] = useState<WebhookSubscription | null>(null);
  const [actionBusyId, setActionBusyId] = useState<string | null>(null);
  const [secretReveal, setSecretReveal] = useState<SecretRevealState>(null);
  const [copiedSecret, setCopiedSecret] = useState(false);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const loadWebhooks = useCallback(async () => {
    if (!workspaceId) return;
    try {
      setError(null);
      const token = await getToken();
      if (!token) return;
      const res = await listWebhooks(token, workspaceId);
      setWebhooks(res.data);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to load webhooks";
      console.error("Failed to load webhooks:", err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [getToken, workspaceId]);

  useEffect(() => {
    loadWebhooks();
  }, [loadWebhooks]);

  const stats = useMemo(() => {
    const activeCount = webhooks.filter((wh) => wh.active).length;
    const inactiveCount = webhooks.length - activeCount;
    const eventSet = new Set(webhooks.flatMap((wh) => wh.events));
    return {
      total: webhooks.length,
      activeCount,
      inactiveCount,
      eventCount: eventSet.size,
    };
  }, [webhooks]);

  function resetForm() {
    setEditing(null);
    setName("");
    setURL("");
    setEvents(["post.published", "post.partial", "post.failed"]);
    setCreateActive(true);
    setCustomSecret("");
  }

  function openCreate() {
    resetForm();
    setDialogOpen(true);
  }

  function openEdit(webhook: WebhookSubscription) {
    setEditing(webhook);
    setName(webhook.name);
    setURL(webhook.url);
    setEvents(webhook.events);
    setCreateActive(webhook.active);
    setCustomSecret("");
    setDialogOpen(true);
  }

  function toggleEvent(eventId: string) {
    setEvents((current) =>
      current.includes(eventId)
        ? current.filter((value) => value !== eventId)
        : [...current, eventId]
    );
  }

  async function handleSave() {
    const trimmedName = name.trim();
    const trimmedURL = url.trim();
    if (!trimmedName) {
      setError("Webhook name is required.");
      return;
    }
    if (!trimmedURL || !trimmedURL.startsWith("https://")) {
      setError("Webhook URLs must start with https://");
      return;
    }
    if (events.length === 0) {
      setError("Select at least one webhook event.");
      return;
    }

    setSaving(true);
    try {
      setError(null);
      const token = await getToken();
      if (!token) return;

      if (editing) {
        await updateWebhook(token, workspaceId, editing.id, {
          name: trimmedName,
          url: trimmedURL,
          events,
        });
      } else {
        const res = await createWebhook(token, workspaceId, {
          name: trimmedName,
          url: trimmedURL,
          events,
          active: createActive,
          secret: customSecret.trim() || undefined,
        });
        setSecretReveal({
          secret: res.data.secret,
          webhookId: res.data.id,
          preview: res.data.secret_preview,
        });
      }

      setDialogOpen(false);
      resetForm();
      await loadWebhooks();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to save webhook";
      console.error("Failed to save webhook:", err);
      setError(message);
    } finally {
      setSaving(false);
    }
  }

  async function handleToggleActive(webhook: WebhookSubscription) {
    setActionBusyId(webhook.id);
    try {
      setError(null);
      const token = await getToken();
      if (!token) return;
      await updateWebhook(token, workspaceId, webhook.id, {
        active: !webhook.active,
      });
      await loadWebhooks();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to update webhook";
      console.error("Failed to toggle webhook:", err);
      setError(message);
    } finally {
      setActionBusyId(null);
    }
  }

  async function handleRotateSecret() {
    if (!rotateTarget) return;
    setActionBusyId(rotateTarget.id);
    try {
      setError(null);
      const token = await getToken();
      if (!token) return;
      const res = await rotateWebhookSecret(token, workspaceId, rotateTarget.id);
      setRotateTarget(null);
      setSecretReveal({
        secret: res.data.secret,
        webhookId: res.data.id,
        preview: res.data.secret_preview,
      });
      await loadWebhooks();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to rotate secret";
      console.error("Failed to rotate webhook secret:", err);
      setError(message);
    } finally {
      setActionBusyId(null);
    }
  }

  async function handleDeleteWebhook() {
    if (!deleteTarget) return;
    setActionBusyId(deleteTarget.id);
    try {
      setError(null);
      const token = await getToken();
      if (!token) return;
      await deleteWebhook(token, workspaceId, deleteTarget.id);
      setDeleteTarget(null);
      await loadWebhooks();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete webhook";
      console.error("Failed to delete webhook:", err);
      setError(message);
    } finally {
      setActionBusyId(null);
    }
  }

  async function copySecret() {
    if (!secretReveal) return;
    await navigator.clipboard.writeText(secretReveal.secret);
    setCopiedSecret(true);
    window.setTimeout(() => setCopiedSecret(false), 2000);
  }

  async function copyWebhookId(webhookId: string) {
    await navigator.clipboard.writeText(webhookId);
    setCopiedId(webhookId);
    window.setTimeout(() => setCopiedId((current) => (current === webhookId ? null : current)), 1600);
  }

  return (
    <>
      <div style={{ display: "grid", gap: 24 }}>
        <div
          style={{
            display: "grid",
            gridTemplateColumns: "minmax(0, 1.5fr) minmax(280px, .9fr)",
            gap: 18,
            alignItems: "stretch",
          }}
        >
          <section
            style={{
              border: "1px solid var(--dborder)",
              borderRadius: 18,
              background:
                "linear-gradient(180deg, color-mix(in srgb, var(--surface2) 72%, #0b1220), var(--surface))",
              padding: 24,
              boxShadow: "0 20px 50px var(--shadow-color)",
            }}
          >
            <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 16 }}>
              <div style={{ maxWidth: 720 }}>
                <div
                  style={{
                    display: "inline-flex",
                    alignItems: "center",
                    gap: 8,
                    padding: "6px 10px",
                    borderRadius: 999,
                    border: "1px solid rgba(96,165,250,.16)",
                    background: "rgba(59,130,246,.12)",
                    color: "#93c5fd",
                    fontSize: 12,
                    fontWeight: 700,
                    letterSpacing: ".02em",
                    marginBottom: 14,
                  }}
                >
                  <Webhook style={{ width: 13, height: 13 }} />
                  Developer webhooks
                </div>
                <div className="dt-page-title" style={{ marginBottom: 10 }}>
                  Push events into your own backend
                </div>
                <div className="dt-subtitle" style={{ maxWidth: 760, lineHeight: 1.7 }}>
                  Configure machine-facing webhook subscriptions for async post outcomes and account lifecycle events.
                  This is separate from Slack or Discord notification channels.
                </div>
              </div>

              <Dialog
                open={dialogOpen}
                onOpenChange={(open) => {
                  setDialogOpen(open);
                  if (!open) resetForm();
                }}
              >
                <DialogTrigger render={<button className="dbtn dbtn-primary" />}>
                  <Plus style={{ width: 14, height: 14 }} /> New webhook
                </DialogTrigger>
                <DialogContent
                  className="!max-w-[680px] sm:!max-w-[680px] !p-0 !gap-0 flex flex-col max-h-[min(88vh,820px)] overflow-hidden"
                >
                  <DialogHeader className="px-6 pt-6 pb-4 border-b border-[var(--dborder)] gap-1.5">
                    <DialogTitle>{editing ? "Edit webhook" : "Create webhook"}</DialogTitle>
                    <DialogDescription>
                      Choose the destination URL and the event set UniPost should deliver.
                    </DialogDescription>
                  </DialogHeader>

                  <div className="flex-1 min-h-0 overflow-y-auto px-6 py-5" style={{ display: "grid", gap: 18 }}>
                    <div>
                      <label className="dform-label">Name</label>
                      <input
                        className="dform-input"
                        placeholder="Publishing status webhook"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        autoFocus
                      />
                    </div>

                    <div>
                      <label className="dform-label">Destination URL</label>
                      <input
                        className="dform-input"
                        placeholder="https://api.example.com/unipost/webhooks"
                        value={url}
                        onChange={(e) => setURL(e.target.value)}
                      />
                      <div className="dt-micro" style={{ marginTop: 8, color: "var(--dmuted)" }}>
                        Use an HTTPS endpoint you control. UniPost signs every request with{" "}
                        <code style={{ fontFamily: "var(--font-geist-mono, monospace)" }}>
                          X-UniPost-Signature
                        </code>.
                      </div>
                    </div>

                    {!editing && (
                      <div style={{ display: "grid", gap: 14 }}>
                        <div>
                          <label className="dform-label">Secret (optional)</label>
                          <input
                            className="dform-input"
                            placeholder="Leave blank to let UniPost generate one"
                            value={customSecret}
                            onChange={(e) => setCustomSecret(e.target.value)}
                          />
                          <div className="dt-micro" style={{ marginTop: 8, color: "var(--dmuted)" }}>
                            If omitted, UniPost generates a signing secret and returns it once in the create response.
                          </div>
                        </div>

                        <label
                          style={{
                            display: "flex",
                            alignItems: "flex-start",
                            gap: 10,
                            padding: "12px 14px",
                            borderRadius: 12,
                            border: "1px solid var(--dborder)",
                            background: "var(--surface2)",
                            cursor: "pointer",
                          }}
                        >
                          <input
                            type="checkbox"
                            checked={createActive}
                            onChange={(e) => setCreateActive(e.target.checked)}
                            style={{ marginTop: 2 }}
                          />
                          <div>
                            <div style={{ fontSize: 13, fontWeight: 600, color: "var(--dtext)" }}>Activate immediately</div>
                            <div style={{ fontSize: 12.5, color: "var(--dmuted)", lineHeight: 1.55, marginTop: 4 }}>
                              Clear this if you want to save the webhook now but enable deliveries later.
                            </div>
                          </div>
                        </label>
                      </div>
                    )}

                    <div>
                      <label className="dform-label">Events</label>
                      <div
                        style={{
                          display: "grid",
                          gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))",
                          gap: 12,
                        }}
                      >
                        {GROUPS.map((group) => (
                          <div
                            key={group}
                            style={{
                              border: "1px solid var(--dborder)",
                              borderRadius: 12,
                              background: "var(--surface2)",
                              overflow: "hidden",
                              display: "flex",
                              flexDirection: "column",
                            }}
                          >
                            <div
                              style={{
                                padding: "10px 14px",
                                borderBottom: "1px solid var(--dborder)",
                                fontSize: 11,
                                fontWeight: 700,
                                letterSpacing: ".09em",
                                textTransform: "uppercase",
                                color: "var(--dmuted2)",
                                background: "color-mix(in srgb, var(--surface) 64%, var(--surface2))",
                              }}
                            >
                              {group}
                            </div>
                            <div style={{ display: "grid", gap: 8, padding: 10 }}>
                              {EVENT_OPTIONS.filter((option) => option.group === group).map((option) => {
                                const checked = events.includes(option.id);
                                return (
                                  <label
                                    key={option.id}
                                    style={{
                                      display: "flex",
                                      alignItems: "flex-start",
                                      gap: 10,
                                      padding: "10px 12px",
                                      borderRadius: 10,
                                      border: checked ? "1px solid rgba(59,130,246,.36)" : "1px solid var(--dborder)",
                                      background: checked ? "rgba(59,130,246,.08)" : "var(--surface)",
                                      cursor: "pointer",
                                      transition: "border-color .12s ease, background .12s ease",
                                    }}
                                  >
                                    <input
                                      type="checkbox"
                                      checked={checked}
                                      onChange={() => toggleEvent(option.id)}
                                      style={{ marginTop: 2, flex: "none" }}
                                    />
                                    <div style={{ minWidth: 0 }}>
                                      <div
                                        style={{
                                          fontSize: 12.5,
                                          fontWeight: 600,
                                          color: "var(--dtext)",
                                          fontFamily: "var(--font-geist-mono, monospace)",
                                        }}
                                      >
                                        {option.title}
                                      </div>
                                      <div style={{ fontSize: 12, color: "var(--dmuted)", lineHeight: 1.5, marginTop: 4 }}>
                                        {option.description}
                                      </div>
                                    </div>
                                  </label>
                                );
                              })}
                            </div>
                          </div>
                        ))}
                      </div>
                    </div>
                  </div>

                  <DialogFooter className="!m-0 px-6 py-4 border-t border-[var(--dborder)] bg-[var(--surface2)]">
                    <button
                      className="dbtn dbtn-ghost"
                      onClick={() => {
                        setDialogOpen(false);
                        resetForm();
                      }}
                    >
                      Cancel
                    </button>
                    <button
                      className="dbtn dbtn-primary"
                      onClick={handleSave}
                      disabled={saving || !name.trim() || !url.trim() || events.length === 0}
                    >
                      {saving ? (editing ? "Saving..." : "Creating...") : editing ? "Save changes" : "Create webhook"}
                    </button>
                  </DialogFooter>
                </DialogContent>
              </Dialog>
            </div>

            <div
              style={{
                display: "grid",
                gridTemplateColumns: "repeat(3, minmax(0, 1fr))",
                gap: 12,
                marginTop: 20,
              }}
            >
              {[
                { label: "Subscriptions", value: stats.total, hint: "Configured endpoints" },
                { label: "Active", value: stats.activeCount, hint: "Receiving deliveries" },
                { label: "Events in use", value: stats.eventCount, hint: "Across all subscriptions" },
              ].map((item) => (
                <div
                  key={item.label}
                  style={{
                    border: "1px solid rgba(148,163,184,.12)",
                    borderRadius: 14,
                    background: "rgba(15,23,42,.16)",
                    padding: "14px 16px",
                  }}
                >
                  <div style={{ fontSize: 12, color: "var(--dmuted2)", textTransform: "uppercase", letterSpacing: ".08em" }}>
                    {item.label}
                  </div>
                  <div style={{ fontSize: 28, fontWeight: 750, color: "var(--dtext)", marginTop: 8 }}>
                    {item.value}
                  </div>
                  <div style={{ fontSize: 12.5, color: "var(--dmuted)", marginTop: 4 }}>{item.hint}</div>
                </div>
              ))}
            </div>
          </section>

          <section
            style={{
              border: "1px solid var(--dborder)",
              borderRadius: 18,
              background: "var(--surface)",
              padding: 22,
              display: "grid",
              gap: 16,
            }}
          >
            <div>
              <div className="dt-card-title" style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <ShieldCheck style={{ width: 16, height: 16, color: "#60a5fa" }} />
                Receiver checklist
              </div>
              <div className="dt-body-sm" style={{ color: "var(--dmuted)", marginTop: 6, lineHeight: 1.7 }}>
                Match the behavior documented in the API reference so your backend stays reliable as publish traffic grows.
              </div>
            </div>

            <div style={{ display: "grid", gap: 12 }}>
              {[
                "Verify X-UniPost-Signature before processing each payload.",
                "Return a 2xx response within 5 seconds once the event is accepted.",
                "Store the signing secret immediately after create or rotate.",
                "Subscribe to post.* events if you need async publish outcomes.",
              ].map((item) => (
                <div key={item} style={{ display: "flex", alignItems: "flex-start", gap: 10 }}>
                  <CheckCircle2 style={{ width: 15, height: 15, marginTop: 2, color: "#10b981", flexShrink: 0 }} />
                  <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>{item}</div>
                </div>
              ))}
            </div>

            <div
              style={{
                borderRadius: 14,
                border: "1px solid var(--dborder)",
                background: "var(--surface2)",
                padding: 14,
              }}
            >
              <div style={{ fontSize: 12, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: "var(--dmuted2)" }}>
                Need payload details?
              </div>
              <div style={{ fontSize: 13, lineHeight: 1.65, color: "var(--dmuted)", marginTop: 8 }}>
                Review event payloads, retry behavior, and signature verification in the API reference.
              </div>
              <Link href="/docs/api/webhooks" className="dbtn dbtn-ghost" style={{ marginTop: 12, display: "inline-flex" }}>
                Open webhook docs
              </Link>
            </div>
          </section>
        </div>

        {error && (
          <div
            style={{
              display: "flex",
              alignItems: "flex-start",
              gap: 10,
              padding: "12px 14px",
              borderRadius: 10,
              background: "var(--danger-soft)",
              border: "1px solid color-mix(in srgb, var(--danger) 24%, transparent)",
              color: "var(--danger)",
              fontSize: 13,
            }}
          >
            <AlertTriangle style={{ width: 14, height: 14, marginTop: 2, flexShrink: 0 }} />
            <div style={{ lineHeight: 1.6 }}>{error}</div>
          </div>
        )}

        {loading ? (
          <div style={{ color: "var(--dmuted)" }}>Loading webhook subscriptions...</div>
        ) : webhooks.length === 0 ? (
          <section
            style={{
              border: "1px dashed var(--dborder)",
              borderRadius: 20,
              background: "var(--surface)",
              padding: "56px 28px",
              textAlign: "center",
            }}
          >
            <div
              style={{
                width: 56,
                height: 56,
                margin: "0 auto 16px",
                borderRadius: 16,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                background: "rgba(59,130,246,.12)",
                color: "#60a5fa",
              }}
            >
              <Webhook style={{ width: 24, height: 24 }} />
            </div>
            <div className="dt-card-title" style={{ marginBottom: 8 }}>
              No webhook subscriptions yet
            </div>
            <div className="dt-body-sm" style={{ color: "var(--dmuted)", maxWidth: 560, margin: "0 auto", lineHeight: 1.7 }}>
              Create your first endpoint to receive async publish outcomes and account lifecycle events in real time.
            </div>
            <div style={{ display: "flex", justifyContent: "center", gap: 8, flexWrap: "wrap", marginTop: 18 }}>
              {EVENT_OPTIONS.map((option) => (
                <span
                  key={option.id}
                  style={{
                    padding: "6px 10px",
                    borderRadius: 999,
                    border: "1px solid var(--dborder)",
                    background: "var(--surface2)",
                    color: "var(--dmuted)",
                    fontSize: 12,
                    fontFamily: "var(--font-geist-mono, monospace)",
                  }}
                >
                  {option.id}
                </span>
              ))}
            </div>
            <button className="dbtn dbtn-primary" onClick={openCreate} style={{ marginTop: 20 }}>
              <Plus style={{ width: 14, height: 14 }} /> Create webhook
            </button>
          </section>
        ) : (
          <div style={{ display: "grid", gap: 16 }}>
            {webhooks.map((webhook) => (
              <section
                key={webhook.id}
                style={{
                  border: "1px solid var(--dborder)",
                  borderRadius: 18,
                  background: "var(--surface)",
                  overflow: "hidden",
                }}
              >
                <div
                  style={{
                    display: "flex",
                    alignItems: "flex-start",
                    justifyContent: "space-between",
                    gap: 16,
                    padding: 20,
                    borderBottom: "1px solid var(--dborder)",
                  }}
                >
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <div style={{ display: "flex", alignItems: "center", gap: 10, flexWrap: "wrap", marginBottom: 10 }}>
                      <div style={badgeStyle(webhook.active)}>
                        <span
                          style={{
                            width: 7,
                            height: 7,
                            borderRadius: "50%",
                            background: webhook.active ? "#10b981" : "var(--dmuted2)",
                          }}
                        />
                        {webhook.active ? "Active" : "Paused"}
                      </div>
                      <button
                        type="button"
                        onClick={() => copyWebhookId(webhook.id)}
                        className="dbtn dbtn-ghost"
                        style={{ padding: "4px 10px", fontSize: 12, height: "auto" }}
                      >
                        <Copy style={{ width: 12, height: 12 }} />
                        {copiedId === webhook.id ? "Copied id" : webhook.id}
                      </button>
                    </div>

                    <div
                      style={{
                        fontSize: 18,
                        fontWeight: 650,
                        color: "var(--dtext)",
                        lineHeight: 1.5,
                        marginBottom: 6,
                      }}
                    >
                      {webhook.name}
                    </div>

                    <div
                      style={{
                        fontSize: 15,
                        fontWeight: 560,
                        color: "var(--dmuted)",
                        lineHeight: 1.5,
                        wordBreak: "break-word",
                      }}
                    >
                      {webhook.url}
                    </div>

                    <div style={{ display: "flex", gap: 12, flexWrap: "wrap", marginTop: 12, color: "var(--dmuted)", fontSize: 12.5 }}>
                      <span>Created {formatDate(webhook.created_at)}</span>
                      <span>Secret preview {webhook.secret_preview}</span>
                      <span>{webhook.events.length} subscribed events</span>
                    </div>
                  </div>

                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap", justifyContent: "flex-end" }}>
                    <button
                      className={webhook.active ? "dbtn dbtn-ghost" : "dbtn dbtn-primary"}
                      onClick={() => handleToggleActive(webhook)}
                      disabled={actionBusyId === webhook.id}
                    >
                      {actionBusyId === webhook.id ? "Updating..." : webhook.active ? "Pause" : "Activate"}
                    </button>
                    <button className="dbtn dbtn-ghost" onClick={() => openEdit(webhook)}>
                      <Pencil style={{ width: 13, height: 13 }} /> Edit
                    </button>
                    <button className="dbtn dbtn-ghost" onClick={() => setRotateTarget(webhook)}>
                      <RotateCw style={{ width: 13, height: 13 }} /> Rotate secret
                    </button>
                    <button className="dbtn dbtn-danger" onClick={() => setDeleteTarget(webhook)}>
                      <Trash2 style={{ width: 13, height: 13 }} /> Delete
                    </button>
                  </div>
                </div>

                <div style={{ padding: 20 }}>
                  <div style={{ fontSize: 12, fontWeight: 700, letterSpacing: ".08em", textTransform: "uppercase", color: "var(--dmuted2)", marginBottom: 12 }}>
                    Event subscriptions
                  </div>
                  <div style={{ display: "flex", gap: 8, flexWrap: "wrap" }}>
                    {webhook.events.map((event) => (
                      <span
                        key={event}
                        style={{
                          padding: "7px 10px",
                          borderRadius: 999,
                          border: "1px solid var(--dborder)",
                          background: "var(--surface2)",
                          color: "var(--dtext)",
                          fontSize: 12,
                          fontFamily: "var(--font-geist-mono, monospace)",
                        }}
                      >
                        {event}
                      </span>
                    ))}
                  </div>
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      {secretReveal && (
        <div
          onClick={() => {
            setSecretReveal(null);
            setCopiedSecret(false);
          }}
          style={{
            position: "fixed",
            inset: 0,
            zIndex: 200,
            background: "var(--overlay)",
            backdropFilter: "blur(4px)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            style={{
              background: "var(--surface)",
              border: "1px solid var(--dborder2)",
              borderRadius: 16,
              width: 560,
              maxWidth: "92vw",
              padding: "26px 28px",
              boxShadow: "0 20px 50px var(--shadow-color)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", marginBottom: 10 }}>
              <div className="dt-card-title" style={{ display: "flex", alignItems: "center", gap: 10 }}>
                <KeyRound style={{ width: 16, height: 16, color: "#60a5fa" }} />
                {copiedSecret ? "Secret copied" : "Store this signing secret now"}
              </div>
              <button
                onClick={() => {
                  setSecretReveal(null);
                  setCopiedSecret(false);
                }}
                style={{ background: "none", border: "none", color: "var(--dmuted)", cursor: "pointer" }}
              >
                ×
              </button>
            </div>

            <div className="dt-body-sm" style={{ color: "var(--dmuted)", lineHeight: 1.7, marginBottom: 16 }}>
              UniPost only shows the plaintext signing secret once, right after create or rotate. Save it in your backend config before closing this dialog.
            </div>

            <div
              style={{
                display: "flex",
                alignItems: "flex-start",
                gap: 10,
                padding: "12px 14px",
                borderRadius: 10,
                border: "1px solid rgba(245,158,11,.18)",
                background: "rgba(245,158,11,.10)",
                color: "var(--warning)",
                fontSize: 13,
                lineHeight: 1.6,
                marginBottom: 16,
              }}
            >
              <AlertTriangle style={{ width: 15, height: 15, marginTop: 2, flexShrink: 0 }} />
              <div>
                Webhook <span style={{ fontFamily: "var(--font-geist-mono, monospace)" }}>{secretReveal.webhookId}</span> now uses this secret.
                Later reads will only show <strong>{secretReveal.preview}</strong>.
              </div>
            </div>

            <div className="dform-label">Signing secret</div>
            <div
              style={{
                display: "flex",
                alignItems: "stretch",
                gap: 10,
                borderRadius: 12,
                border: "1px solid var(--dborder)",
                background: "var(--surface2)",
                padding: 10,
              }}
            >
              <div
                style={{
                  flex: 1,
                  fontFamily: "var(--font-geist-mono, monospace)",
                  fontSize: 13,
                  lineHeight: 1.7,
                  color: "var(--dtext)",
                  wordBreak: "break-all",
                }}
              >
                {secretReveal.secret}
              </div>
              <button className="dbtn dbtn-primary" onClick={copySecret}>
                <Copy style={{ width: 13, height: 13 }} />
                {copiedSecret ? "Copied" : "Copy"}
              </button>
            </div>

            <div style={{ display: "flex", justifyContent: "flex-end", marginTop: 18 }}>
              <button
                className="dbtn dbtn-ghost"
                onClick={() => {
                  setSecretReveal(null);
                  setCopiedSecret(false);
                }}
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}

      <ConfirmModal
        open={!!rotateTarget}
        title="Rotate signing secret"
        message={
          <>
            Generate a new secret for <strong>{rotateTarget?.url}</strong>. Your receiver must be updated immediately or deliveries will start failing signature checks.
          </>
        }
        confirmLabel={actionBusyId === rotateTarget?.id ? "Rotating..." : "Rotate secret"}
        wide
        onConfirm={handleRotateSecret}
        onCancel={() => setRotateTarget(null)}
      />

      <ConfirmModal
        open={!!deleteTarget}
        title="Delete webhook"
        message={
          <>
            Delete <strong>{deleteTarget?.url}</strong>? UniPost will stop delivering all subscribed events to this endpoint immediately.
          </>
        }
        confirmLabel={actionBusyId === deleteTarget?.id ? "Deleting..." : "Delete webhook"}
        variant="danger"
        onConfirm={handleDeleteWebhook}
        onCancel={() => setDeleteTarget(null)}
      />
    </>
  );
}
