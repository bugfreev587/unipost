"use client";

import { useState, useEffect, useCallback } from "react";
import { PLATFORM_LIMITS, countCharacters, getCountStatus, STATUS_COLORS } from "@/components/tools/platform-limits";
import { PlatformCard } from "@/components/tools/PlatformCard";
import { MarketingCTALight } from "@/components/marketing/nav";

// ── Page-specific styles ──
const CSS = `.ap-hero{padding:64px 0 40px;max-width:880px;text-align:center;margin:0 auto}
.ap-hero-title{font-size:48px;font-weight:900;letter-spacing:-1.5px;line-height:1.1;color:var(--text);margin-bottom:14px}
.ap-hero-title em{color:var(--accent);font-style:normal}
.ap-hero-sub{font-size:16px;color:#bbb;line-height:1.6;margin-bottom:8px}
.ap-tabs{display:flex;justify-content:center;gap:4px;margin-top:24px}
.ap-tab{padding:8px 20px;border-radius:8px;font-size:13.5px;font-weight:600;cursor:pointer;border:1px solid var(--b2);background:transparent;color:var(--muted);transition:all .15s;font-family:var(--ui)}
.ap-tab:hover{background:var(--s2);color:var(--text)}
.ap-tab.active{background:var(--accent);color:#000;border-color:var(--accent)}
.ap-section{max-width:720px;margin:0 auto;padding:48px 0}
.ap-section-title{font-size:14px;font-weight:700;color:var(--accent);text-transform:uppercase;letter-spacing:.1em;font-family:var(--mono);margin-bottom:16px}
.ap-input{width:100%;background:#0f0f0f;border:1px solid #1a1a1a;border-radius:10px;padding:14px 16px;font-size:14px;color:#f0f0f0;font-family:var(--ui);outline:none;transition:border-color .15s}
.ap-input:focus{border-color:#333}
.ap-input::placeholder{color:#444}
.ap-textarea{width:100%;min-height:120px;background:#0f0f0f;border:1px solid #1a1a1a;border-radius:12px;padding:18px;font-size:15px;line-height:1.6;color:#f0f0f0;font-family:var(--ui);resize:vertical;outline:none;transition:border-color .15s}
.ap-textarea:focus{border-color:#333}
.ap-textarea::placeholder{color:#444}
.ap-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px;margin-top:16px}
.ap-draft-card{background:#0f0f0f;border:1px solid #1a1a1a;border-radius:12px;padding:18px}
.ap-draft-header{display:flex;justify-content:space-between;align-items:center;margin-bottom:10px}
.ap-draft-platform{font-size:14px;font-weight:700;color:var(--text)}
.ap-draft-count{font-size:11px;font-family:var(--mono)}
.ap-draft-text{font-size:13.5px;color:#ccc;line-height:1.6;white-space:pre-wrap;word-break:break-word}
.ap-draft-edit{width:100%;background:#080808;border:1px solid #222;border-radius:8px;padding:12px;font-size:13.5px;color:#ccc;line-height:1.6;font-family:var(--ui);resize:vertical;outline:none;min-height:80px}
.ap-draft-edit:focus{border-color:#444}
.ap-btn{display:inline-flex;align-items:center;gap:6px;padding:10px 24px;border-radius:10px;font-size:14px;font-weight:600;cursor:pointer;transition:all .15s;border:none;font-family:var(--ui)}
.ap-btn-primary{background:var(--blue);color:#000}
.ap-btn-primary:hover:not(:disabled){background:#38bdf8;box-shadow:0 0 24px #0ea5e930}
.ap-btn-primary:disabled{opacity:.5;cursor:not-allowed}
.ap-btn-accent{background:var(--accent);color:#000}
.ap-btn-accent:hover:not(:disabled){background:#34d399}
.ap-btn-accent:disabled{opacity:.5;cursor:not-allowed}
.ap-btn-ghost{background:transparent;color:var(--muted);border:1px solid var(--b2);padding:8px 16px;font-size:13px}
.ap-btn-ghost:hover{background:var(--s2);color:var(--text)}
.ap-btn-sm{padding:6px 12px;font-size:12px;border-radius:6px}
.ap-result{display:flex;align-items:center;gap:8px;padding:8px 0;font-size:14px}
.ap-result-ok{color:#10b981}
.ap-result-fail{color:#ef4444}
.ap-privacy{font-size:11px;color:var(--muted2);text-align:center;margin-top:16px;line-height:1.5;max-width:520px;margin-left:auto;margin-right:auto}
.ap-install-box{background:var(--s1);border:1px solid var(--b2);border-radius:10px;padding:20px 28px;font-family:var(--mono);font-size:14.5px;color:var(--accent);text-align:center;max-width:560px;margin:24px auto 0;user-select:all}
.ap-install-box span{color:var(--muted)}
.ap-status{font-size:13px;color:var(--muted);margin-top:12px;min-height:20px}
@media(max-width:680px){.ap-grid{grid-template-columns:1fr}.ap-hero-title{font-size:34px}}`;

// ── Config storage ──
const STORAGE_KEY = "agentpost_config";
interface Config { unipost_api_key: string; anthropic_api_key: string }

function loadConfig(): Config {
  if (typeof window === "undefined") return { unipost_api_key: "", anthropic_api_key: "" };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { unipost_api_key: "", anthropic_api_key: "" };
    return JSON.parse(raw);
  } catch { return { unipost_api_key: "", anthropic_api_key: "" }; }
}
function saveConfig(c: Config) { localStorage.setItem(STORAGE_KEY, JSON.stringify(c)); }
function clearConfig() { localStorage.removeItem(STORAGE_KEY); }

// ── Types ──
interface Account { id: string; platform: string; account_name: string | null; status: string }
interface Draft { account_id: string; caption: string; platform: string; account_name: string }
interface PublishResult { social_account_id: string; platform: string; account_name?: string; status: string; error_message?: string }

// ── API helpers ──
async function fetchAccounts(apiKey: string): Promise<Account[]> {
  const res = await fetch("https://api.unipost.dev/v1/accounts", {
    headers: { Authorization: `Bearer ${apiKey}` },
  });
  if (!res.ok) throw new Error(`Failed to load accounts (${res.status})`);
  const body = await res.json();
  return body.data || [];
}

async function generateDrafts(message: string, accounts: Account[], anthropicKey: string): Promise<Draft[]> {
  const accountLines = accounts.map(a => `- ${a.platform} (account_id: ${a.id})${a.account_name ? ` — ${a.account_name}` : ""}`).join("\n");
  const system = `You are AgentPost, an AI that translates a developer's one-line update into per-platform social media posts. Respond with ONLY a JSON object: {"drafts":[{"account_id":"<id>","caption":"<text>"},...]}. One entry per account. Twitter ≤280 chars, LinkedIn 100-300 words, Bluesky ≤300 chars casual, Threads ≤500 chars, Instagram ≤2200 chars. NEVER use the same caption across platforms. NEVER use buzzword openers. NEVER invent facts.`;
  const userMsg = `User input: ${JSON.stringify(message)}\n\nConnected accounts:\n${accountLines}\n\nGenerate one post per connected account. Output JSON only.`;

  const res = await fetch("https://api.anthropic.com/v1/messages", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "x-api-key": anthropicKey,
      "anthropic-version": "2023-06-01",
      "anthropic-dangerous-direct-browser-access": "true",
    },
    body: JSON.stringify({
      model: "claude-sonnet-4-5-20250514",
      max_tokens: 2048,
      system,
      messages: [{ role: "user", content: userMsg }],
    }),
  });
  if (!res.ok) {
    const err = await res.text();
    throw new Error(`Claude API error (${res.status}): ${err.slice(0, 200)}`);
  }
  const body = await res.json();
  const text = body.content?.find((b: any) => b.type === "text")?.text || "";
  const cleaned = text.trim().replace(/^```(?:json)?\s*/i, "").replace(/\s*```\s*$/i, "").trim();
  const parsed = JSON.parse(cleaned);
  if (!parsed.drafts) throw new Error('Missing "drafts" in Claude response');

  const idx = new Map(accounts.map(a => [a.id, a]));
  return parsed.drafts.map((d: any) => ({
    account_id: d.account_id,
    caption: d.caption,
    platform: idx.get(d.account_id)?.platform || "unknown",
    account_name: idx.get(d.account_id)?.account_name || d.account_id,
  }));
}

async function publishDrafts(drafts: Draft[], apiKey: string): Promise<{ status: string; results: PublishResult[] }> {
  const res = await fetch("https://api.unipost.dev/v1/posts", {
    method: "POST",
    headers: { Authorization: `Bearer ${apiKey}`, "Content-Type": "application/json" },
    body: JSON.stringify({
      platform_posts: drafts.map(d => ({ account_id: d.account_id, caption: d.caption })),
    }),
  });
  if (!res.ok) throw new Error(`Publish failed (${res.status}): ${(await res.text()).slice(0, 200)}`);
  const body = await res.json();
  return body.data || body;
}

// ── Component ──
type Step = "configure" | "compose" | "preview" | "results";

export function AgentPostContent() {
  const [tab, setTab] = useState<"web" | "cli">("web");
  const [step, setStep] = useState<Step>("configure");
  const [config, setConfig] = useState<Config>({ unipost_api_key: "", anthropic_api_key: "" });
  const [message, setMessage] = useState("");
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [drafts, setDrafts] = useState<Draft[]>([]);
  const [editingIdx, setEditingIdx] = useState<number | null>(null);
  const [results, setResults] = useState<PublishResult[]>([]);
  const [status, setStatus] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const saved = loadConfig();
    if (saved.unipost_api_key && saved.anthropic_api_key) {
      setConfig(saved);
      setStep("compose");
    } else if (saved.unipost_api_key || saved.anthropic_api_key) {
      setConfig(saved);
    }
  }, []);

  const handleSaveConfig = useCallback(async () => {
    if (!config.unipost_api_key || !config.anthropic_api_key) return;
    setLoading(true);
    setStatus("Testing connection...");
    try {
      const accts = await fetchAccounts(config.unipost_api_key);
      setAccounts(accts.filter(a => a.status === "active"));
      saveConfig(config);
      setStep("compose");
      setStatus("");
    } catch (e) {
      setStatus(`Error: ${(e as Error).message}`);
    } finally { setLoading(false); }
  }, [config]);

  const handleGenerate = useCallback(async () => {
    if (!message.trim()) return;
    setLoading(true);
    setStatus("Loading accounts...");
    try {
      let accts = accounts;
      if (accts.length === 0) {
        accts = (await fetchAccounts(config.unipost_api_key)).filter(a => a.status === "active");
        setAccounts(accts);
      }
      if (accts.length === 0) { setStatus("No active accounts. Connect at least one in your UniPost dashboard."); setLoading(false); return; }
      setStatus(`Generating drafts for ${accts.length} accounts via Claude...`);
      const d = await generateDrafts(message, accts, config.anthropic_api_key);
      setDrafts(d);
      setStep("preview");
      setStatus("");
    } catch (e) {
      setStatus(`Error: ${(e as Error).message}`);
    } finally { setLoading(false); }
  }, [message, accounts, config]);

  const handlePublish = useCallback(async () => {
    setLoading(true);
    setStatus("Publishing...");
    try {
      const res = await publishDrafts(drafts, config.unipost_api_key);
      setResults(res.results || []);
      setStep("results");
      setStatus("");
    } catch (e) {
      setStatus(`Error: ${(e as Error).message}`);
    } finally { setLoading(false); }
  }, [drafts, config]);

  const handleClearKeys = () => { clearConfig(); setConfig({ unipost_api_key: "", anthropic_api_key: "" }); setStep("configure"); setStatus(""); };

  const handleEditDraft = (idx: number, caption: string) => {
    setDrafts(d => d.map((item, i) => i === idx ? { ...item, caption } : item));
  };

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <div className="tl-page">
        {/* Hero */}
        <div className="ap-hero">
          <h1 className="ap-hero-title"><em>AgentPost</em></h1>
          <p className="ap-hero-sub">Describe what you shipped. AI posts it everywhere.</p>
          <div className="ap-tabs">
            <button className={`ap-tab${tab === "web" ? " active" : ""}`} onClick={() => setTab("web")}>Web App</button>
            <button className={`ap-tab${tab === "cli" ? " active" : ""}`} onClick={() => setTab("cli")}>CLI Install</button>
          </div>
        </div>

        {/* CLI tab */}
        {tab === "cli" && (
          <div className="ap-section" style={{ textAlign: "center" }}>
            <div className="ap-install-box"><span>$</span> npm install -g @unipost/agentpost</div>
            <p style={{ marginTop: 20, fontSize: 14, color: "#999" }}>
              Then run: <code style={{ color: "#10b981", fontFamily: "var(--mono)" }}>agentpost init</code> to set up your API keys.
            </p>
            <div style={{ marginTop: 24 }}>
              <a href="https://github.com/unipost-dev/agentpost" target="_blank" rel="noopener noreferrer" className="lp-btn lp-btn-accent lp-btn-lg">View on GitHub</a>
            </div>
          </div>
        )}

        {/* Web App tab */}
        {tab === "web" && (
          <>
            {/* Step 1: Configure */}
            {step === "configure" && (
              <div className="ap-section">
                <div className="ap-section-title">Step 1 &mdash; Configure</div>
                <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
                  <input className="ap-input" type="password" placeholder="UniPost API Key (up_live_...)" value={config.unipost_api_key} onChange={e => setConfig(c => ({ ...c, unipost_api_key: e.target.value }))} />
                  <input className="ap-input" type="password" placeholder="Anthropic API Key (sk-ant-...)" value={config.anthropic_api_key} onChange={e => setConfig(c => ({ ...c, anthropic_api_key: e.target.value }))} />
                  <button className="ap-btn ap-btn-primary" disabled={loading || !config.unipost_api_key || !config.anthropic_api_key} onClick={handleSaveConfig}>
                    {loading ? "Testing..." : "Save & Continue"}
                  </button>
                </div>
                <div className="ap-privacy">
                  Your API keys are stored locally in your browser. They are never sent to UniPost&#39;s servers. Clear them anytime with the &quot;Clear Keys&quot; button.
                </div>
              </div>
            )}

            {/* Step 2: Compose */}
            {step === "compose" && (
              <div className="ap-section">
                <div className="ap-section-title">Step 2 &mdash; Describe what you want to post</div>
                <textarea className="ap-textarea" placeholder='e.g. "shipped webhooks today — full HMAC signing, retry with exponential backoff, dashboard live"' value={message} onChange={e => setMessage(e.target.value)} autoFocus />
                <div style={{ display: "flex", gap: 8, marginTop: 16 }}>
                  <button className="ap-btn ap-btn-primary" disabled={loading || !message.trim()} onClick={handleGenerate}>
                    {loading ? "Generating..." : "Generate Previews"}
                  </button>
                  <button className="ap-btn ap-btn-ghost ap-btn-sm" onClick={handleClearKeys}>Clear Keys</button>
                </div>
              </div>
            )}

            {/* Step 3: Preview */}
            {step === "preview" && (
              <div className="ap-section" style={{ maxWidth: 880 }}>
                <div className="ap-section-title">Step 3 &mdash; Preview & Edit</div>
                <div className="ap-grid">
                  {drafts.map((d, i) => {
                    const pl = PLATFORM_LIMITS.find(p => p.platform === d.platform);
                    const count = pl ? countCharacters(d.caption, pl.countingMethod) : d.caption.length;
                    const max = pl?.maxLength || 280;
                    const st = getCountStatus(count, max);
                    const color = STATUS_COLORS[st];
                    return (
                      <div key={d.account_id} className="ap-draft-card">
                        <div className="ap-draft-header">
                          <span className="ap-draft-platform">{pl?.icon} {d.platform} <span style={{ fontWeight: 400, color: "#999", fontSize: 12 }}>@{d.account_name}</span></span>
                          <span className="ap-draft-count" style={{ color }}>{count}/{max}</span>
                        </div>
                        {editingIdx === i ? (
                          <textarea className="ap-draft-edit" value={d.caption} onChange={e => handleEditDraft(i, e.target.value)} onBlur={() => setEditingIdx(null)} autoFocus />
                        ) : (
                          <div className="ap-draft-text" onClick={() => setEditingIdx(i)} style={{ cursor: "pointer" }} title="Click to edit">
                            {d.caption}
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
                <div style={{ display: "flex", gap: 8, marginTop: 20 }}>
                  <button className="ap-btn ap-btn-accent" disabled={loading} onClick={handlePublish}>
                    {loading ? "Publishing..." : "Publish All"}
                  </button>
                  <button className="ap-btn ap-btn-ghost ap-btn-sm" onClick={() => { setStep("compose"); setDrafts([]); }}>Back</button>
                  <button className="ap-btn ap-btn-ghost ap-btn-sm" disabled={loading} onClick={handleGenerate}>Regenerate</button>
                </div>
              </div>
            )}

            {/* Step 4: Results */}
            {step === "results" && (
              <div className="ap-section">
                <div className="ap-section-title">Published</div>
                {results.map((r, i) => (
                  <div key={i} className="ap-result">
                    <span className={r.status === "published" ? "ap-result-ok" : "ap-result-fail"}>
                      {r.status === "published" ? "\u2713" : "\u2717"}
                    </span>
                    <span style={{ color: "#f0f0f0", fontWeight: 600 }}>{r.platform}</span>
                    <span style={{ color: "#999" }}>{r.account_name || r.social_account_id}</span>
                    {r.error_message && <span style={{ color: "#ef4444", fontSize: 12 }}>{r.error_message}</span>}
                  </div>
                ))}
                <div style={{ display: "flex", gap: 8, marginTop: 20 }}>
                  <button className="ap-btn ap-btn-primary" onClick={() => { setStep("compose"); setMessage(""); setDrafts([]); setResults([]); }}>New Post</button>
                </div>
              </div>
            )}

            <div className="ap-status">{status}</div>
          </>
        )}

        {/* CTA */}
        <div className="tl-cta" style={{ marginTop: 48 }}>
          <div className="tl-cta-inner">
            <div className="tl-cta-glow" />
            <h2 className="tl-cta-title">Need to post via API?</h2>
            <p className="tl-cta-sub">UniPost handles OAuth, token refresh, and per-platform quirks for you.</p>
            <div className="tl-cta-actions">
              <a href="https://app.unipost.dev" className="lp-btn lp-btn-primary lp-btn-lg">Get UniPost API Key</a>
              <MarketingCTALight />
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
