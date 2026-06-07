"use client";

import { useAuth } from "@clerk/nextjs";
import {
  CheckCircle2,
  Circle,
  FlaskConical,
  Globe2,
  KeyRound,
  LockKeyhole,
  Power,
  RotateCw,
  Route,
  Save,
  Server,
  ShieldAlert,
  Trash2,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import {
  deleteAdminAIProviderSurfaceRoute,
  disableAdminAIProvider,
  listAdminAIProviderEvents,
  listAdminAIProviders,
  routeAdminAIProviderSurface,
  testAdminAIProvider,
  updateAdminAIProvider,
  type AdminAIClientKind,
  type AdminAIProvider,
  type AdminAIProviderEvent,
  type AdminAIProviderStatus,
  type AdminAIProvidersResponse,
  type AdminAISurface,
} from "@/lib/api";

import { AdminShell, fmtRelative } from "../_components/admin-ui";

const PROVIDERS: Array<{
  id: AdminAIProvider;
  label: string;
  subtitle: string;
  defaultBaseURL: string;
  keyPlaceholder: string;
}> = [
  {
    id: "tokengate",
    label: "TokenGate",
    subtitle: "OpenAI-compatible gateway",
    defaultBaseURL: "https://gateway.mytokengate.com/v1",
    keyPlaceholder: "tg_...:...",
  },
  {
    id: "openai",
    label: "OpenAI",
    subtitle: "Chat Completions",
    defaultBaseURL: "https://api.openai.com/v1",
    keyPlaceholder: "sk-...",
  },
  {
    id: "anthropic",
    label: "Claude Code",
    subtitle: "Anthropic API",
    defaultBaseURL: "https://api.anthropic.com/v1",
    keyPlaceholder: "sk-ant-...",
  },
];

const SURFACES: Array<{ id: AdminAISurface; label: string; clientKind: AdminAIClientKind }> = [
  { id: "post_assist", label: "Post Assist", clientKind: "chat_completions" },
  { id: "error_triage", label: "Error Triage", clientKind: "chat_completions" },
  { id: "app_review_ai", label: "App Review", clientKind: "messages" },
];

const providerLabels: Record<AdminAIProvider, string> = {
  tokengate: "TokenGate",
  openai: "OpenAI",
  anthropic: "Claude Code",
};

const surfaceLabels: Record<AdminAISurface, string> = {
  post_assist: "Post Assist",
  error_triage: "Error Triage",
  app_review_ai: "App Review",
};

function providerMeta(provider: AdminAIProvider) {
  return PROVIDERS.find((item) => item.id === provider) || PROVIDERS[0];
}

function defaultForm(provider: AdminAIProvider, status?: AdminAIProviderStatus) {
  const meta = providerMeta(provider);
  return {
    apiKey: "",
    baseURL: status?.base_url || meta.defaultBaseURL,
    chatModel: status?.chat_model || "",
    messagesModel: status?.messages_model || "",
    enabled: status?.enabled ?? true,
  };
}

function statusLabel(provider: AdminAIProviderStatus) {
  if (provider.source === "admin" && provider.enabled && provider.last_validation_status === "ok") return "Active";
  if (provider.source === "admin" && provider.last_validation_status && provider.last_validation_status !== "ok") return "Validation failed";
  if (provider.source === "admin" && provider.enabled) return "Configured";
  if (provider.source === "admin") return "Disabled";
  if (provider.source === "env") return "Env fallback";
  return "Not configured";
}

function badgeClass(value: string) {
  if (value === "Active" || value === "ok") return "ad-badge ad-b-green";
  if (value === "Validation failed" || value.endsWith("failed")) return "ad-badge ad-b-red";
  if (value === "Env fallback") return "ad-badge ad-b-blue";
  return "ad-badge ad-b-gray";
}

function keyTailLabel(tail?: string) {
  return tail ? `...${tail}` : "No key";
}

function providerSummary(status?: AdminAIProviderStatus) {
  if (!status) return "Not configured";
  const label = statusLabel(status).toLowerCase();
  return `${providerLabels[status.provider]} ${label} ${keyTailLabel(status.key_tail)}`;
}

function modelSummary(status?: AdminAIProviderStatus) {
  if (!status) return "No model set";
  return status.chat_model || status.messages_model || "Default model";
}

export default function AdminAIKeysPage() {
  const { getToken } = useAuth();
  const [data, setData] = useState<AdminAIProvidersResponse | null>(null);
  const [events, setEvents] = useState<AdminAIProviderEvent[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<AdminAIProvider>("tokengate");
  const [form, setForm] = useState(defaultForm("tokengate"));
  const [routeSurface, setRouteSurface] = useState<AdminAISurface>("post_assist");
  const [routeProvider, setRouteProvider] = useState<AdminAIProvider>("tokengate");
  const [routeModel, setRouteModel] = useState("");
  const [loading, setLoading] = useState(true);
  const [actionKey, setActionKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const providersById = useMemo(() => {
    const map = new Map<AdminAIProvider, AdminAIProviderStatus>();
    for (const provider of data?.providers || []) map.set(provider.provider, provider);
    return map;
  }, [data]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      const [statusRes, eventsRes] = await Promise.all([
        listAdminAIProviders(token),
        listAdminAIProviderEvents(token, { limit: 8 }),
      ]);
      setData(statusRes.data);
      setEvents(eventsRes.data.events);
      const selected = statusRes.data.providers.find((item) => item.provider === selectedProvider);
      setForm((current) => ({ ...defaultForm(selectedProvider, selected), apiKey: current.apiKey }));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load AI providers");
    } finally {
      setLoading(false);
    }
  }, [getToken, selectedProvider]);

  useEffect(() => {
    load();
  }, [load]);

  function selectProvider(provider: AdminAIProvider) {
    setSelectedProvider(provider);
    setRouteProvider(provider);
    setNotice(null);
    setError(null);
    setForm(defaultForm(provider, providersById.get(provider)));
  }

  async function withAction(key: string, fn: (token: string) => Promise<void>) {
    setActionKey(key);
    setNotice(null);
    setError(null);
    try {
      const token = await getToken();
      if (!token) throw new Error("Not authenticated");
      await fn(token);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Action failed");
    } finally {
      setActionKey("");
    }
  }

  async function handleSave() {
    await withAction(`save:${selectedProvider}`, async (token) => {
      await updateAdminAIProvider(token, selectedProvider, {
        api_key: form.apiKey.trim() || undefined,
        base_url: form.baseURL.trim(),
        chat_model: form.chatModel.trim(),
        messages_model: form.messagesModel.trim(),
        enabled: form.enabled,
      });
      setForm((current) => ({ ...current, apiKey: "" }));
      setNotice(`${providerLabels[selectedProvider]} saved.`);
    });
  }

  async function handleTest(provider = selectedProvider) {
    await withAction(`test:${provider}`, async (token) => {
      const payload = provider === selectedProvider
        ? {
            api_key: form.apiKey.trim() || undefined,
            base_url: form.baseURL.trim(),
            chat_model: form.chatModel.trim(),
            messages_model: form.messagesModel.trim(),
          }
        : {};
      const res = await testAdminAIProvider(token, provider, payload);
      setNotice(`${providerLabels[provider]} validation: ${res.data.status}`);
    });
  }

  async function handleDisable(provider: AdminAIProvider) {
    if (!window.confirm(`Disable ${providerLabels[provider]} and remove its routed surfaces?`)) return;
    await withAction(`disable:${provider}`, async (token) => {
      await disableAdminAIProvider(token, provider);
      setNotice(`${providerLabels[provider]} disabled.`);
    });
  }

  async function handleRoute() {
    const surface = SURFACES.find((item) => item.id === routeSurface);
    if (!surface) return;
    if (routeProvider === "tokengate") {
      const ok = window.confirm(`Route ${surface.label} to TokenGate? Prompt payloads for this surface will be processed by TokenGate.`);
      if (!ok) return;
    }
    await withAction(`route:${routeSurface}`, async (token) => {
      await routeAdminAIProviderSurface(token, routeSurface, {
        provider: routeProvider,
        client_kind: surface.clientKind,
        model_override: routeModel.trim() || undefined,
      });
      setNotice(`${surface.label} routed to ${providerLabels[routeProvider]}.`);
    });
  }

  async function handleUnroute(surface: AdminAISurface) {
    await withAction(`unroute:${surface}`, async (token) => {
      await deleteAdminAIProviderSurfaceRoute(token, surface);
      setNotice(`${surfaceLabels[surface]} returned to fallback.`);
    });
  }

  const selectedMeta = providerMeta(selectedProvider);
  const selectedStatus = providersById.get(selectedProvider);
  const selectedStatusLabel = selectedStatus ? statusLabel(selectedStatus) : "Not configured";
  const selectedSurface = SURFACES.find((item) => item.id === routeSurface) || SURFACES[0];

  return (
    <AdminShell title="AI Keys" loading={loading} onRefresh={load} requireSuperAdmin>
      <style>{aiKeysCss}</style>

      <section className="ai-page-frame">
        <div className="ai-hero">
          <div>
            <div className="ai-hero-kicker">LLM provider</div>
            <h1>Admin</h1>
          </div>
          <div className={badgeClass(selectedStatusLabel)}>
            <CheckCircle2 strokeWidth={1.75} />
            <span>{providerSummary(selectedStatus)}</span>
          </div>
        </div>

        {error ? (
          <div className="ai-alert ai-alert-error">
            <ShieldAlert strokeWidth={1.75} />
            <span>{error}</span>
          </div>
        ) : null}
        {notice ? (
          <div className="ai-alert ai-alert-ok">
            <CheckCircle2 strokeWidth={1.75} />
            <span>{notice}</span>
          </div>
        ) : null}

        <section className="ai-credential-card">
          <div className="ai-card-title-row">
            <div className="ai-card-title">
              <KeyRound strokeWidth={1.75} />
              <span>API key</span>
            </div>
            <div className="ai-card-meta">
              <span className={badgeClass(selectedStatusLabel)}>{selectedStatusLabel}</span>
              <span className="ai-key-tail">{keyTailLabel(selectedStatus?.key_tail)}</span>
            </div>
          </div>

          <div className="ai-provider-card-grid">
            {PROVIDERS.map((provider) => {
              const status = providersById.get(provider.id);
              const label = status ? statusLabel(status) : "Not configured";
              const active = provider.id === selectedProvider;
              return (
                <button
                  key={provider.id}
                  type="button"
                  className="ai-provider-card"
                  data-active={active}
                  onClick={() => selectProvider(provider.id)}
                >
                  <span>
                    <strong>{provider.label}</strong>
                    <small>{provider.subtitle}</small>
                  </span>
                  <span className="ai-provider-right">
                    <span className={badgeClass(label)}>{status?.source || "none"}</span>
                    {active ? <CheckCircle2 strokeWidth={2} /> : <Circle strokeWidth={1.75} />}
                  </span>
                </button>
              );
            })}
          </div>

          <div className="ai-form-stack">
            <label className="ai-field ai-field-full">
              <span>{selectedMeta.label} API key</span>
              <div className="ai-input-shell">
                <KeyRound strokeWidth={1.75} />
                <input
                  type="password"
                  value={form.apiKey}
                  placeholder={selectedStatus?.configured ? "Leave blank to keep the existing key" : selectedMeta.keyPlaceholder}
                  onChange={(event) => setForm((current) => ({ ...current, apiKey: event.target.value }))}
                />
              </div>
              <small>Leave blank to keep the existing key.</small>
            </label>

            <label className="ai-field ai-field-full">
              <span>Base URL</span>
              <div className="ai-input-shell">
                <Globe2 strokeWidth={1.75} />
                <input value={form.baseURL} onChange={(event) => setForm((current) => ({ ...current, baseURL: event.target.value }))} />
              </div>
              <small>Use the API backend URL with /v1, not the dashboard URL.</small>
            </label>

            <div className="ai-model-grid">
              <label className="ai-field">
                <span>Chat model</span>
                <div className="ai-input-shell">
                  <Server strokeWidth={1.75} />
                  <input
                    value={form.chatModel}
                    placeholder="Optional"
                    onChange={(event) => setForm((current) => ({ ...current, chatModel: event.target.value }))}
                  />
                </div>
                <small>{selectedProvider === "anthropic" ? "Used only when routed through chat completions." : "Overrides the default chat model."}</small>
              </label>
              <label className="ai-field">
                <span>Messages model</span>
                <div className="ai-input-shell">
                  <Server strokeWidth={1.75} />
                  <input
                    value={form.messagesModel}
                    placeholder="Optional"
                    onChange={(event) => setForm((current) => ({ ...current, messagesModel: event.target.value }))}
                  />
                </div>
                <small>{modelSummary(selectedStatus)}</small>
              </label>
            </div>
          </div>

          <div className="ai-card-actions">
            <label className="ai-toggle">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(event) => setForm((current) => ({ ...current, enabled: event.target.checked }))}
              />
              <span>Enabled</span>
            </label>
            <button type="button" className="ai-save-button" disabled={!!actionKey} onClick={handleSave}>
              <Save strokeWidth={1.85} />
              Save credentials
            </button>
            <button type="button" className="ai-secondary-button" disabled={!!actionKey} onClick={() => handleTest()}>
              <FlaskConical strokeWidth={1.75} />
              Test connection
            </button>
            <button type="button" className="ai-secondary-button" disabled={loading} onClick={load}>
              <RotateCw strokeWidth={1.75} />
              Refresh
            </button>
            {selectedStatus?.source === "admin" ? (
              <button type="button" className="ai-danger-button" disabled={!!actionKey} onClick={() => handleDisable(selectedProvider)}>
                <Power strokeWidth={1.75} />
                Disable
              </button>
            ) : null}
          </div>
        </section>

        <section className="ai-secret-card">
          <LockKeyhole strokeWidth={1.75} />
          <div>
            <h2>Secrets stay server-side</h2>
            <p>Only the provider, base URL, model names, and key tail are returned to the browser after saving.</p>
          </div>
        </section>

        <div className="ai-ops-grid">
          <section className="ai-ops-panel">
            <div className="ai-ops-head">
              <div>
                <div className="ai-ops-kicker">Active routing policy</div>
                <h2>Effective AI surfaces</h2>
              </div>
            </div>
            <div className="ai-route-grid">
              {SURFACES.map((surface) => {
                const effective = data?.effective?.[surface.id];
                const route = data?.routes?.[surface.id];
                const sourceLabel = effective?.source === "admin" ? "Active" : effective?.source === "env" ? "Env fallback" : "Not configured";
                return (
                  <div className="ai-route-row" key={surface.id}>
                    <div>
                      <div className="ai-route-name">{surface.label}</div>
                      <div className="ai-route-meta">{surface.clientKind.replace("_", " ")}</div>
                    </div>
                    <span className={badgeClass(sourceLabel)}>{effective?.source || "none"}</span>
                    <div className="ai-route-detail">
                      <span>{effective?.provider ? providerLabels[effective.provider] : "-"}</span>
                      <span>{effective?.model || "-"}</span>
                    </div>
                    <button
                      type="button"
                      className="ai-mini-button"
                      disabled={!route || !!actionKey}
                      onClick={() => handleUnroute(surface.id)}
                    >
                      <Trash2 strokeWidth={1.75} />
                      Unroute
                    </button>
                  </div>
                );
              })}
            </div>
          </section>

          <section className="ai-ops-panel">
            <div className="ai-ops-head">
              <div>
                <div className="ai-ops-kicker">Route surface</div>
                <h2>Change provider</h2>
              </div>
            </div>
            <div className="ai-route-form">
              <label className="ai-field">
                <span>Surface</span>
                <select value={routeSurface} onChange={(event) => setRouteSurface(event.target.value as AdminAISurface)}>
                  {SURFACES.map((surface) => (
                    <option key={surface.id} value={surface.id}>{surface.label}</option>
                  ))}
                </select>
              </label>
              <label className="ai-field">
                <span>Provider</span>
                <select value={routeProvider} onChange={(event) => setRouteProvider(event.target.value as AdminAIProvider)}>
                  {PROVIDERS.map((provider) => (
                    <option key={provider.id} value={provider.id}>{provider.label}</option>
                  ))}
                </select>
              </label>
              <label className="ai-field">
                <span>Client kind</span>
                <input value={selectedSurface.clientKind} readOnly />
              </label>
              <label className="ai-field">
                <span>Model override</span>
                <input value={routeModel} placeholder="Optional" onChange={(event) => setRouteModel(event.target.value)} />
              </label>
              {routeProvider === "tokengate" ? (
                <div className="ai-notice">TokenGate can receive prompt payloads for the selected surface.</div>
              ) : null}
              <button type="button" className="ai-save-button ai-route-button" disabled={!!actionKey} onClick={handleRoute}>
                <Route strokeWidth={1.85} />
                Route surface
              </button>
            </div>
          </section>
        </div>

        <section className="ai-events-panel">
          <div className="ai-ops-head">
            <div>
              <div className="ai-ops-kicker">Recent events</div>
              <h2>Credential activity</h2>
            </div>
            <RotateCw className="ai-muted-icon" strokeWidth={1.75} />
          </div>
          <div className="ai-event-list">
            {events.length === 0 ? (
              <div className="ai-empty">No events recorded.</div>
            ) : events.map((event) => (
              <div className="ai-event" key={event.id}>
                <div>
                  <div className="ai-event-action">{event.action}</div>
                  <div className="ai-event-meta">
                    {[event.provider ? providerLabels[event.provider] : null, event.surface ? surfaceLabels[event.surface] : null].filter(Boolean).join(" / ") || "Global"}
                  </div>
                </div>
                <span>{event.created_at ? fmtRelative(event.created_at) : "-"}</span>
              </div>
            ))}
          </div>
        </section>
      </section>
    </AdminShell>
  );
}

const aiKeysCss = `
.ai-page-frame {
  max-width: 1240px;
  margin: 0 auto;
  padding: 10px 0 54px;
}
.ai-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 18px;
  margin: 0 0 34px;
}
.ai-hero-kicker {
  color: #65738a;
  font-size: 17px;
  font-weight: 760;
  letter-spacing: 0;
  line-height: 1.25;
  margin-bottom: 6px;
}
.ai-hero h1 {
  margin: 0;
  color: #111827;
  font-size: 30px;
  line-height: 1.08;
  font-weight: 820;
  letter-spacing: -0.02em;
}
.ai-credential-card,
.ai-secret-card,
.ai-ops-panel,
.ai-events-panel {
  background: #ffffff;
  border: 1px solid #dfe6ef;
  border-radius: 14px;
  box-shadow: 0 22px 52px -38px rgba(15, 23, 42, 0.34);
}
.ai-credential-card {
  padding: 26px;
}
.ai-card-title-row,
.ai-ops-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 18px;
}
.ai-card-title {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  color: #111827;
  font-size: 18px;
  font-weight: 760;
  letter-spacing: -0.01em;
}
.ai-card-title svg {
  width: 22px;
  height: 22px;
  color: #111827;
}
.ai-card-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 8px;
}
.ai-key-tail {
  display: inline-flex;
  align-items: center;
  min-height: 25px;
  border: 1px solid #e5ebf3;
  border-radius: 999px;
  padding: 3px 10px;
  color: #65738a;
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
  font-weight: 650;
}
.ad-badge svg {
  width: 13px;
  height: 13px;
}
.ad-b-green {
  background: #ecfdf3;
  color: #16883d;
  border: 1px solid #bbf7d0;
}
.ad-b-red {
  background: #fff1f0;
  color: #dc2626;
  border: 1px solid #fecaca;
}
.ad-b-blue {
  background: #eff6ff;
  color: #2563eb;
  border: 1px solid #bfdbfe;
}
.ad-b-gray {
  background: #f8fafc;
  color: #64748b;
  border: 1px solid #e2e8f0;
}
.ai-provider-card-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 14px;
  margin: 28px 0 26px;
}
.ai-provider-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-width: 0;
  gap: 14px;
  border: 1px solid #dfe6ef;
  border-radius: 14px;
  background: #ffffff;
  padding: 20px 20px;
  color: #111827;
  text-align: left;
  cursor: pointer;
  font: inherit;
  transition: border-color 160ms ease, background 160ms ease, transform 160ms ease, box-shadow 160ms ease;
}
.ai-provider-card:hover {
  border-color: #cbd5e1;
  transform: translateY(-1px);
  box-shadow: 0 18px 38px -30px rgba(15, 23, 42, 0.38);
}
.ai-provider-card[data-active="true"] {
  background: #fff7f6;
  border-color: #ef3b2d;
  box-shadow: inset 0 0 0 1px rgba(239, 59, 45, 0.1);
}
.ai-provider-card strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 18px;
  font-weight: 780;
  letter-spacing: -0.015em;
  line-height: 1.25;
}
.ai-provider-card small {
  display: block;
  margin-top: 8px;
  color: #65738a;
  font-size: 14px;
  font-weight: 650;
  line-height: 1.3;
}
.ai-provider-right {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}
.ai-provider-right svg {
  width: 26px;
  height: 26px;
  color: #cbd5e1;
}
.ai-provider-card[data-active="true"] .ai-provider-right svg {
  color: #ef3b2d;
  fill: #ef3b2d;
  stroke: #ef3b2d;
}
.ai-form-stack {
  display: grid;
  gap: 18px;
}
.ai-model-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px;
}
.ai-field {
  display: grid;
  gap: 9px;
  min-width: 0;
}
.ai-field span {
  color: #344256;
  font-size: 17px;
  font-weight: 740;
  letter-spacing: -0.01em;
}
.ai-field small {
  color: #65738a;
  font-size: 14px;
  font-weight: 560;
  line-height: 1.45;
}
.ai-input-shell {
  display: flex;
  align-items: center;
  gap: 13px;
  min-width: 0;
  min-height: 48px;
  border: 1px solid #dfe6ef;
  border-radius: 10px;
  background: #ffffff;
  padding: 0 14px;
  transition: border-color 140ms ease, box-shadow 140ms ease;
}
.ai-input-shell:focus-within {
  border-color: #ef3b2d;
  box-shadow: 0 0 0 3px rgba(239, 59, 45, 0.08);
}
.ai-input-shell svg {
  width: 18px;
  height: 18px;
  color: #9aa8ba;
  flex-shrink: 0;
}
.ai-field input,
.ai-field select {
  width: 100%;
  min-width: 0;
  border: 1px solid #dfe6ef;
  border-radius: 10px;
  background: #ffffff;
  color: #111827;
  font: inherit;
  font-size: 14px;
  font-weight: 580;
  outline: none;
  padding: 12px 13px;
}
.ai-input-shell input {
  border: 0;
  border-radius: 0;
  padding: 0;
}
.ai-field input::placeholder {
  color: #9aa8ba;
}
.ai-field input:focus,
.ai-field select:focus {
  border-color: #ef3b2d;
  box-shadow: 0 0 0 3px rgba(239, 59, 45, 0.08);
}
.ai-input-shell input:focus {
  box-shadow: none;
}
.ai-field input[readonly] {
  color: #65738a;
  cursor: default;
}
.ai-card-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 24px;
}
.ai-save-button,
.ai-secondary-button,
.ai-danger-button,
.ai-mini-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  min-height: 42px;
  border-radius: 12px;
  border: 1px solid transparent;
  padding: 0 17px;
  font: inherit;
  font-size: 14px;
  font-weight: 720;
  cursor: pointer;
  white-space: nowrap;
  transition: transform 140ms ease, box-shadow 140ms ease, background 140ms ease, border-color 140ms ease;
}
.ai-save-button {
  background: #ef3b2d;
  color: #ffffff;
  box-shadow: 0 14px 30px -22px rgba(239, 59, 45, 0.72);
}
.ai-save-button:hover:not(:disabled) {
  background: #dc3327;
  transform: translateY(-1px);
}
.ai-save-button:active:not(:disabled),
.ai-secondary-button:active:not(:disabled),
.ai-danger-button:active:not(:disabled),
.ai-mini-button:active:not(:disabled) {
  transform: translateY(1px);
}
.ai-secondary-button,
.ai-mini-button {
  background: #ffffff;
  color: #344256;
  border-color: #dfe6ef;
}
.ai-secondary-button:hover:not(:disabled),
.ai-mini-button:hover:not(:disabled) {
  background: #f8fafc;
  border-color: #cbd5e1;
}
.ai-danger-button {
  background: #fff7f6;
  color: #dc2626;
  border-color: #fecaca;
}
.ai-danger-button:hover:not(:disabled) {
  background: #fff1f0;
}
.ai-save-button:disabled,
.ai-secondary-button:disabled,
.ai-danger-button:disabled,
.ai-mini-button:disabled {
  cursor: not-allowed;
  opacity: 0.48;
  transform: none;
  box-shadow: none;
}
.ai-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 42px;
  padding: 0 8px 0 0;
  color: #344256;
  font-size: 14px;
  font-weight: 680;
}
.ai-toggle input {
  width: 16px;
  height: 16px;
  accent-color: #ef3b2d;
}
.ai-secret-card {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-top: 30px;
  padding: 22px 26px;
}
.ai-secret-card svg {
  width: 22px;
  height: 22px;
  color: #65738a;
  margin-top: 2px;
}
.ai-secret-card h2,
.ai-ops-panel h2,
.ai-events-panel h2 {
  margin: 0;
  color: #344256;
  font-size: 18px;
  line-height: 1.25;
  font-weight: 760;
  letter-spacing: -0.015em;
}
.ai-secret-card p {
  margin: 8px 0 0;
  color: #65738a;
  font-size: 16px;
  line-height: 1.45;
}
.ai-ops-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(320px, 0.75fr);
  gap: 18px;
  margin-top: 30px;
}
.ai-ops-panel,
.ai-events-panel {
  padding: 20px;
}
.ai-ops-kicker {
  color: #65738a;
  font-size: 11px;
  font-weight: 780;
  letter-spacing: 0.09em;
  line-height: 1.2;
  margin-bottom: 7px;
  text-transform: uppercase;
}
.ai-route-grid,
.ai-event-list {
  display: grid;
  gap: 10px;
  margin-top: 18px;
}
.ai-route-row {
  display: grid;
  grid-template-columns: minmax(130px, 1fr) auto minmax(150px, 1fr) auto;
  align-items: center;
  gap: 12px;
  border-top: 1px solid #edf2f7;
  padding: 13px 0 0;
}
.ai-route-row:first-child {
  border-top: 0;
  padding-top: 0;
}
.ai-route-name {
  color: #111827;
  font-size: 14px;
  font-weight: 740;
}
.ai-route-meta {
  color: #65738a;
  font-size: 12px;
  font-weight: 560;
  text-transform: capitalize;
}
.ai-route-detail {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.ai-route-detail span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #344256;
  font-family: var(--font-geist-mono), monospace;
  font-size: 11px;
}
.ai-mini-button {
  min-height: 34px;
  border-radius: 9px;
  padding: 0 11px;
  font-size: 12px;
}
.ai-route-form {
  display: grid;
  gap: 13px;
  margin-top: 18px;
}
.ai-route-form .ai-field {
  gap: 6px;
}
.ai-route-form .ai-field span {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.08em;
}
.ai-notice {
  color: #9a6400;
  background: #fffbeb;
  border: 1px solid #fde68a;
  border-radius: 10px;
  padding: 10px 12px;
  font-size: 13px;
  line-height: 1.45;
}
.ai-route-button {
  justify-self: start;
}
.ai-events-panel {
  margin-top: 18px;
}
.ai-event {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  border-top: 1px solid #edf2f7;
  padding-top: 10px;
}
.ai-event:first-child {
  border-top: 0;
  padding-top: 0;
}
.ai-event-action {
  color: #111827;
  font-family: var(--font-geist-mono), monospace;
  font-size: 12px;
  font-weight: 650;
}
.ai-event-meta,
.ai-event span {
  color: #65738a;
  font-size: 12px;
  line-height: 1.4;
}
.ai-muted-icon {
  width: 18px;
  height: 18px;
  color: #9aa8ba;
}
.ai-empty {
  color: #65738a;
  border: 1px dashed #dfe6ef;
  border-radius: 12px;
  padding: 16px;
  text-align: center;
  font-size: 13px;
}
.ai-alert {
  display: flex;
  align-items: center;
  gap: 9px;
  border-radius: 12px;
  padding: 11px 13px;
  font-size: 13px;
  font-weight: 620;
  margin-bottom: 14px;
}
.ai-alert svg {
  width: 17px;
  height: 17px;
  flex-shrink: 0;
}
.ai-alert-error {
  color: #dc2626;
  background: #fff1f0;
  border: 1px solid #fecaca;
}
.ai-alert-ok {
  color: #16883d;
  background: #ecfdf3;
  border: 1px solid #bbf7d0;
}
@media (max-width: 1120px) {
  .ai-page-frame { max-width: 860px; }
  .ai-provider-card-grid,
  .ai-ops-grid { grid-template-columns: 1fr; }
}
@media (max-width: 760px) {
  .ai-page-frame { padding: 4px 0 32px; }
  .ai-hero { align-items: flex-start; flex-direction: column; margin-bottom: 22px; }
  .ai-hero h1 { font-size: 28px; }
  .ai-credential-card { padding: 18px; border-radius: 12px; }
  .ai-card-title-row { flex-direction: column; }
  .ai-card-meta { justify-content: flex-start; }
  .ai-model-grid { grid-template-columns: 1fr; }
  .ai-provider-card { padding: 16px; }
  .ai-provider-card strong { font-size: 16px; }
  .ai-field span { font-size: 15px; }
  .ai-card-actions { align-items: stretch; flex-direction: column; }
  .ai-toggle,
  .ai-save-button,
  .ai-secondary-button,
  .ai-danger-button { width: 100%; }
  .ai-secret-card { padding: 18px; }
  .ai-secret-card p { font-size: 14px; }
  .ai-route-row { grid-template-columns: 1fr; align-items: stretch; }
}
`;
