"use client";

import { useAuth } from "@clerk/nextjs";
import {
  CheckCircle2,
  FlaskConical,
  KeyRound,
  Power,
  RotateCw,
  Route,
  Save,
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

const PROVIDERS: Array<{ id: AdminAIProvider; label: string; defaultBaseURL: string }> = [
  { id: "tokengate", label: "TokenGate", defaultBaseURL: "https://gateway.mytokengate.com/v1" },
  { id: "openai", label: "OpenAI", defaultBaseURL: "https://api.openai.com/v1" },
  { id: "anthropic", label: "Anthropic", defaultBaseURL: "https://api.anthropic.com/v1" },
];

const SURFACES: Array<{ id: AdminAISurface; label: string; clientKind: AdminAIClientKind }> = [
  { id: "post_assist", label: "Post Assist", clientKind: "chat_completions" },
  { id: "error_triage", label: "Error Triage", clientKind: "chat_completions" },
  { id: "app_review_ai", label: "App Review", clientKind: "messages" },
];

const providerLabels: Record<AdminAIProvider, string> = {
  tokengate: "TokenGate",
  openai: "OpenAI",
  anthropic: "Anthropic",
};

const surfaceLabels: Record<AdminAISurface, string> = {
  post_assist: "Post Assist",
  error_triage: "Error Triage",
  app_review_ai: "App Review",
};

function defaultForm(provider: AdminAIProvider, status?: AdminAIProviderStatus) {
  const meta = PROVIDERS.find((item) => item.id === provider) || PROVIDERS[0];
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

  const selectedStatus = providersById.get(selectedProvider);
  const selectedSurface = SURFACES.find((item) => item.id === routeSurface) || SURFACES[0];

  return (
    <AdminShell title="AI Keys" loading={loading} onRefresh={load} requireSuperAdmin>
      <style>{aiKeysCss}</style>

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

      <div className="ad-section-header">
        <div>
          <div className="ad-section-title">Provider status</div>
          <div className="ad-section-meta">Admin-managed credentials and environment fallbacks</div>
        </div>
      </div>

      <div className="ai-grid">
        <div className="ai-main-col">
          <div className="ad-tbl-wrap ad-tbl-static">
            <table>
              <thead>
                <tr>
                  <th>Provider</th>
                  <th>Status</th>
                  <th>Source</th>
                  <th>Key tail</th>
                  <th>Base URL</th>
                  <th>Models</th>
                  <th>Validated</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {(data?.providers || PROVIDERS.map((item) => ({ provider: item.id }) as AdminAIProviderStatus)).map((provider) => {
                  const label = statusLabel(provider);
                  return (
                    <tr key={provider.provider} data-selected={provider.provider === selectedProvider} onClick={() => selectProvider(provider.provider)}>
                      <td>
                        <div className="ai-provider-cell">
                          <KeyRound strokeWidth={1.75} />
                          <span>{providerLabels[provider.provider]}</span>
                        </div>
                      </td>
                      <td><span className={badgeClass(label)}>{label}</span></td>
                      <td className="ad-mono">{provider.source || "none"}</td>
                      <td className="ad-mono">{provider.key_tail ? `••••${provider.key_tail}` : "—"}</td>
                      <td className="ai-url">{provider.base_url || "—"}</td>
                      <td className="ai-models">
                        <span>{provider.chat_model || "—"}</span>
                        <span>{provider.messages_model || "—"}</span>
                      </td>
                      <td>{provider.last_validated_at ? fmtRelative(provider.last_validated_at) : "Never"}</td>
                      <td>
                        <div className="ai-actions">
                          <button type="button" className="ad-btn ad-btn-ghost" onClick={(event) => { event.stopPropagation(); selectProvider(provider.provider); }}>
                            <KeyRound strokeWidth={1.75} />
                            Configure
                          </button>
                          <button type="button" className="ad-btn ad-btn-ghost" disabled={!!actionKey} onClick={(event) => { event.stopPropagation(); handleTest(provider.provider); }}>
                            <FlaskConical strokeWidth={1.75} />
                            Test
                          </button>
                          {provider.source === "admin" ? (
                            <button type="button" className="ad-btn ad-btn-ghost" disabled={!!actionKey} onClick={(event) => { event.stopPropagation(); handleDisable(provider.provider); }}>
                              <Power strokeWidth={1.75} />
                              Disable
                            </button>
                          ) : null}
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <div className="ai-panel">
            <div className="ai-panel-head">
              <div>
                <div className="ad-panel-section-title">Active routing policy</div>
                <div className="ai-panel-title">Effective AI surfaces</div>
              </div>
            </div>
            <div className="ai-route-grid">
              {SURFACES.map((surface) => {
                const effective = data?.effective?.[surface.id];
                const route = data?.routes?.[surface.id];
                return (
                  <div className="ai-route-row" key={surface.id}>
                    <div>
                      <div className="ai-route-name">{surface.label}</div>
                      <div className="ai-route-meta">{surface.clientKind.replace("_", " ")}</div>
                    </div>
                    <div>
                      <span className={badgeClass(effective?.source === "admin" ? "Active" : effective?.source === "env" ? "Env fallback" : "Not configured")}>
                        {effective?.source || "none"}
                      </span>
                    </div>
                    <div className="ai-route-detail">
                      <span>{effective?.provider ? providerLabels[effective.provider] : "—"}</span>
                      <span className="ad-mono">{effective?.model || "—"}</span>
                    </div>
                    <button
                      type="button"
                      className="ad-btn ad-btn-ghost"
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
          </div>
        </div>

        <div className="ai-side-col">
          <div className="ai-panel">
            <div className="ai-panel-head">
              <div>
                <div className="ad-panel-section-title">Provider editor</div>
                <div className="ai-panel-title">{providerLabels[selectedProvider]}</div>
              </div>
              <span className={badgeClass(selectedStatus ? statusLabel(selectedStatus) : "Not configured")}>
                {selectedStatus ? statusLabel(selectedStatus) : "Not configured"}
              </span>
            </div>

            <div className="ai-provider-tabs">
              {PROVIDERS.map((provider) => (
                <button
                  key={provider.id}
                  type="button"
                  className="ai-tab"
                  data-active={provider.id === selectedProvider}
                  onClick={() => selectProvider(provider.id)}
                >
                  {provider.label}
                </button>
              ))}
            </div>

            <label className="ai-field">
              <span>API key</span>
              <input
                type="password"
                value={form.apiKey}
                placeholder={selectedStatus?.configured ? "Leave blank to keep stored key" : "Required on first save"}
                onChange={(event) => setForm((current) => ({ ...current, apiKey: event.target.value }))}
              />
            </label>
            <label className="ai-field">
              <span>Base URL</span>
              <input value={form.baseURL} onChange={(event) => setForm((current) => ({ ...current, baseURL: event.target.value }))} />
            </label>
            <label className="ai-field">
              <span>Chat model</span>
              <input value={form.chatModel} onChange={(event) => setForm((current) => ({ ...current, chatModel: event.target.value }))} />
            </label>
            <label className="ai-field">
              <span>Messages model</span>
              <input value={form.messagesModel} onChange={(event) => setForm((current) => ({ ...current, messagesModel: event.target.value }))} />
            </label>
            <label className="ai-check">
              <input type="checkbox" checked={form.enabled} onChange={(event) => setForm((current) => ({ ...current, enabled: event.target.checked }))} />
              <span>Enabled</span>
            </label>

            <div className="ai-form-actions">
              <button type="button" className="ad-btn ai-primary-btn" disabled={!!actionKey} onClick={handleSave}>
                <Save strokeWidth={1.75} />
                Save
              </button>
              <button type="button" className="ad-btn ad-btn-ghost" disabled={!!actionKey} onClick={() => handleTest()}>
                <FlaskConical strokeWidth={1.75} />
                Test connection
              </button>
            </div>
          </div>

          <div className="ai-panel">
            <div className="ad-panel-section-title">Route surface</div>
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
                <input value={routeModel} onChange={(event) => setRouteModel(event.target.value)} />
              </label>
              {routeProvider === "tokengate" ? (
                <div className="ai-notice">TokenGate can receive prompt payloads for the selected surface.</div>
              ) : null}
              <button type="button" className="ad-btn ai-primary-btn" disabled={!!actionKey} onClick={handleRoute}>
                <Route strokeWidth={1.75} />
                Route surface
              </button>
            </div>
          </div>

          <div className="ai-panel">
            <div className="ai-panel-head">
              <div>
                <div className="ad-panel-section-title">Recent events</div>
                <div className="ai-panel-title">Credential activity</div>
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
                      {[event.provider ? providerLabels[event.provider] : null, event.surface ? surfaceLabels[event.surface] : null].filter(Boolean).join(" · ") || "Global"}
                    </div>
                  </div>
                  <span>{event.created_at ? fmtRelative(event.created_at) : "—"}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </AdminShell>
  );
}

const aiKeysCss = `
.ai-grid { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(340px, 0.55fr); gap: 16px; align-items: start; }
.ai-main-col, .ai-side-col { display: grid; gap: 16px; min-width: 0; }
.ai-panel { background: var(--surface); border: 1px solid var(--dborder); border-radius: 8px; padding: 16px; min-width: 0; }
.ai-panel-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 12px; margin-bottom: 14px; }
.ai-panel-title { font-size: 14px; font-weight: 650; color: var(--dtext); }
.ai-provider-cell { display: inline-flex; align-items: center; gap: 7px; font-weight: 600; }
.ai-provider-cell svg { width: 15px; height: 15px; color: var(--dmuted); }
.ai-url { max-width: 240px; word-break: break-all; color: var(--dmuted); font-family: var(--font-geist-mono), monospace; font-size: 11px; }
.ai-models { display: grid; gap: 2px; color: var(--dmuted); font-family: var(--font-geist-mono), monospace; font-size: 11px; }
.ai-actions { display: flex; flex-wrap: wrap; gap: 6px; }
.ad-tbl-wrap tr[data-selected="true"] { background: color-mix(in srgb, var(--accent-dim) 42%, transparent); }
.ad-b-green { background: var(--success-soft); color: var(--success); border: 1px solid color-mix(in srgb, var(--success) 24%, transparent); }
.ad-b-red { background: var(--danger-soft); color: var(--danger); border: 1px solid color-mix(in srgb, var(--danger) 24%, transparent); }
.ai-alert { display: flex; align-items: center; gap: 8px; border-radius: 8px; padding: 10px 12px; font-size: 12px; margin-bottom: 12px; }
.ai-alert svg { width: 16px; height: 16px; flex-shrink: 0; }
.ai-alert-error { color: var(--danger); background: var(--danger-soft); border: 1px solid color-mix(in srgb, var(--danger) 20%, transparent); }
.ai-alert-ok { color: var(--success); background: var(--success-soft); border: 1px solid color-mix(in srgb, var(--success) 20%, transparent); }
.ai-provider-tabs { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 6px; margin-bottom: 12px; }
.ai-tab { border: 1px solid var(--dborder2); border-radius: 6px; background: var(--surface2); color: var(--dmuted); font: inherit; font-size: 12px; padding: 7px 8px; cursor: pointer; }
.ai-tab[data-active="true"] { color: var(--daccent); border-color: color-mix(in srgb, var(--daccent) 30%, var(--dborder)); background: var(--accent-dim); }
.ai-field { display: grid; gap: 5px; margin-bottom: 10px; }
.ai-field span { font-size: 11px; color: var(--dmuted); font-weight: 600; }
.ai-field input, .ai-field select { width: 100%; min-width: 0; background: var(--surface2); border: 1px solid var(--dborder2); border-radius: 6px; color: var(--dtext); font: inherit; font-size: 12px; padding: 7px 9px; outline: none; }
.ai-field input:focus, .ai-field select:focus { border-color: color-mix(in srgb, var(--primary) 32%, transparent); box-shadow: 0 0 0 3px var(--focus-ring); }
.ai-field input[readonly] { color: var(--dmuted); cursor: default; }
.ai-check { display: inline-flex; align-items: center; gap: 8px; color: var(--dtext); font-size: 12px; margin: 2px 0 12px; }
.ai-check input { width: 14px; height: 14px; accent-color: var(--primary); }
.ai-form-actions { display: flex; flex-wrap: wrap; gap: 8px; }
.ai-primary-btn { background: var(--primary); color: var(--primary-foreground); border-color: color-mix(in srgb, var(--primary) 75%, black); }
.ai-primary-btn:hover:not(:disabled) { filter: brightness(0.98); }
.ai-route-grid { display: grid; gap: 8px; }
.ai-route-row { display: grid; grid-template-columns: minmax(120px, 1fr) auto minmax(150px, 1fr) auto; gap: 10px; align-items: center; border: 1px solid var(--dborder); border-radius: 8px; padding: 10px; background: var(--surface2); }
.ai-route-name { font-weight: 650; color: var(--dtext); font-size: 12.5px; }
.ai-route-meta { color: var(--dmuted); font-size: 11px; text-transform: capitalize; }
.ai-route-detail { display: grid; gap: 1px; min-width: 0; }
.ai-route-detail span { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.ai-route-form { display: grid; gap: 0; }
.ai-notice { color: var(--warning); background: var(--warning-soft); border: 1px solid color-mix(in srgb, var(--warning) 22%, transparent); border-radius: 8px; padding: 9px 10px; font-size: 12px; margin-bottom: 10px; }
.ai-event-list { display: grid; gap: 8px; }
.ai-event { display: flex; justify-content: space-between; gap: 10px; border: 1px solid var(--dborder); border-radius: 8px; padding: 9px 10px; background: var(--surface2); }
.ai-event-action { color: var(--dtext); font-family: var(--font-geist-mono), monospace; font-size: 11px; }
.ai-event-meta, .ai-event span { color: var(--dmuted); font-size: 11px; }
.ai-muted-icon { width: 15px; height: 15px; color: var(--dmuted); }
.ai-empty { color: var(--dmuted); font-size: 12px; border: 1px dashed var(--dborder); border-radius: 8px; padding: 12px; text-align: center; }
@media (max-width: 1120px) {
  .ai-grid { grid-template-columns: 1fr; }
}
@media (max-width: 720px) {
  .ai-route-row { grid-template-columns: 1fr; align-items: stretch; }
  .ai-provider-tabs { grid-template-columns: 1fr; }
}
`;
