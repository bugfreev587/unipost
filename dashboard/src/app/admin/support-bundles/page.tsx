"use client";

import { useAuth } from "@clerk/nextjs";
import {
  AlertTriangle,
  Check,
  Copy,
  FileText,
  LifeBuoy,
  Search,
} from "lucide-react";
import { useEffect, useMemo, useState, type FormEvent } from "react";

import {
  type AdminSupportBundle,
  getAdminSupportBundle,
  listAdminSupportBundles,
} from "@/lib/api";
import { AdminShell, fmtRelative } from "../_components/admin-ui";

const DEFAULT_LIMIT = 50;

export default function AdminSupportBundlesPage() {
  const { getToken } = useAuth();
  const [bundles, setBundles] = useState<AdminSupportBundle[]>([]);
  const [selected, setSelected] = useState<AdminSupportBundle | null>(null);
  const [workspaceID, setWorkspaceID] = useState("");
  const [ownerEmail, setOwnerEmail] = useState("");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [error, setError] = useState("");
  const [detailError, setDetailError] = useState("");
  const [copied, setCopied] = useState(false);

  const loadBundles = async () => {
    setLoading(true);
    setError("");
    try {
      const token = await getToken();
      if (!token) throw new Error("Missing admin session.");
      const res = await listAdminSupportBundles(token, {
        workspace_id: workspaceID.trim(),
        owner_email: ownerEmail.trim(),
        q: query.trim(),
        limit: DEFAULT_LIMIT,
      });
      const rows = res.data || [];
      setBundles(rows);
      if (rows.length === 0) {
        setSelected(null);
      } else if (!selected || !rows.some((row) => row.id === selected.id)) {
        void loadDetail(rows[0].id);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load support bundles.");
    } finally {
      setLoading(false);
    }
  };

  const loadDetail = async (id: string) => {
    setDetailLoading(true);
    setDetailError("");
    setCopied(false);
    try {
      const token = await getToken();
      if (!token) throw new Error("Missing admin session.");
      const res = await getAdminSupportBundle(token, id);
      setSelected(res.data);
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : "Failed to load support bundle.");
    } finally {
      setDetailLoading(false);
    }
  };

  useEffect(() => {
    void loadBundles();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const stats = useMemo(() => {
    const findings = bundles.reduce((sum, bundle) => sum + (bundle.finding_count || 0), 0);
    const errors = bundles.reduce((sum, bundle) => sum + (bundle.recent_error_count || 0), 0);
    return { count: bundles.length, findings, errors, latest: bundles[0]?.created_at || "" };
  }, [bundles]);

  const submitFilters = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    void loadBundles();
  };

  const copyReport = async () => {
    if (!selected?.report_markdown) return;
    await navigator.clipboard.writeText(selected.report_markdown);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  };

  return (
    <AdminShell title="Support Bundles" loading={loading || detailLoading} onRefresh={loadBundles} requireSuperAdmin>
      <style>{supportBundlesCss}</style>
      {error ? <InlineError message={error} /> : null}

      <div className="asb-header">
        <div>
          <div className="ad-section-title">Support bundles</div>
          <div className="ad-section-meta">Super admin support reports uploaded by UniPost CLI doctor</div>
        </div>
        <div className="asb-stat-strip">
          <Metric label="Bundles" value={stats.count} />
          <Metric label="Findings" value={stats.findings} />
          <Metric label="Recent errors" value={stats.errors} />
          <Metric label="Latest" value={stats.latest ? fmtRelative(stats.latest) : "None"} />
        </div>
      </div>

      <form className="ad-filter-bar asb-filter" onSubmit={submitFilters}>
        <div className="asb-search-group">
          <Search strokeWidth={1.75} />
          <input
            className="ad-search asb-search"
            placeholder="Search summary, run id, workspace, owner"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
          />
        </div>
        <input
          className="ad-search"
          placeholder="Workspace id"
          value={workspaceID}
          onChange={(event) => setWorkspaceID(event.target.value)}
        />
        <input
          className="ad-search"
          placeholder="Owner email"
          value={ownerEmail}
          onChange={(event) => setOwnerEmail(event.target.value)}
        />
        <button type="submit" className="ad-btn ad-btn-ghost" disabled={loading}>
          <Search strokeWidth={1.75} />
          Search
        </button>
      </form>

      <div className="asb-layout">
        <section className="asb-list" aria-label="Support bundle list">
          <div className="asb-panel-head">
            <span>Recent uploads</span>
            <span className="ad-mono">{loading ? "Loading" : `${bundles.length} rows`}</span>
          </div>
          {bundles.length === 0 ? (
            <EmptyState loading={loading} />
          ) : (
            <div className="ad-tbl-wrap asb-table-wrap">
              <table>
                <thead>
                  <tr>
                    <th>Bundle</th>
                    <th>Workspace</th>
                    <th>Owner</th>
                    <th>Signals</th>
                    <th>Created</th>
                  </tr>
                </thead>
                <tbody>
                  {bundles.map((bundle) => (
                    <tr
                      key={bundle.id}
                      data-selected={selected?.id === bundle.id}
                      onClick={() => void loadDetail(bundle.id)}
                    >
                      <td>
                        <div className="asb-row-main">
                          <span className="ad-mono">{bundle.id}</span>
                          <span>{bundle.summary || "No summary"}</span>
                        </div>
                      </td>
                      <td>
                        <div className="asb-row-main">
                          <span>{bundle.workspace_name || "Unnamed workspace"}</span>
                          <span className="ad-mono">{bundle.workspace_id}</span>
                        </div>
                      </td>
                      <td>{bundle.owner_email || "Unknown"}</td>
                      <td>
                        <span className="ad-badge ad-b-gray">{bundle.finding_count} findings</span>
                        <span className="ad-badge ad-b-blue asb-badge-gap">{bundle.recent_error_count} errors</span>
                      </td>
                      <td>{fmtRelative(bundle.created_at)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <aside className="asb-detail" aria-label="Support bundle detail">
          {!selected ? (
            <div className="asb-detail-empty">
              <LifeBuoy strokeWidth={1.75} />
              <span>Select a support bundle</span>
            </div>
          ) : (
            <>
              <div className="asb-detail-head">
                <div>
                  <div className="asb-detail-title">{selected.id}</div>
                  <div className="ad-section-meta">{selected.workspace_name || selected.workspace_id}</div>
                </div>
                <button type="button" className="ad-btn ad-btn-ghost" onClick={copyReport} disabled={!selected.report_markdown}>
                  {copied ? <Check strokeWidth={1.75} /> : <Copy strokeWidth={1.75} />}
                  {copied ? "Copied" : "Copy"}
                </button>
              </div>

              {detailError ? <InlineError message={detailError} /> : null}

              <div className="asb-detail-grid">
                <DetailItem label="Owner" value={selected.owner_email || "Unknown"} />
                <DetailItem label="Plan" value={selected.plan_id || "Unknown"} />
                <DetailItem label="Run id" value={selected.run_id} mono />
                <DetailItem label="CLI" value={selected.cli_version || "Unknown"} />
                <DetailItem label="Created" value={fmtRelative(selected.created_at)} />
                <DetailItem label="Schema" value={selected.schema_version} mono />
              </div>

              <div className="asb-report-head">
                <FileText strokeWidth={1.75} />
                <span>Report markdown</span>
              </div>
              {detailLoading ? (
                <div className="asb-skeleton" />
              ) : (
                <pre className="asb-report">{selected.report_markdown || "No report markdown returned."}</pre>
              )}
            </>
          )}
        </aside>
      </div>
    </AdminShell>
  );
}

function Metric({ label, value }: { label: string; value: number | string }) {
  return (
    <div className="asb-metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function DetailItem({ label, value, mono }: { label: string; value: string; mono?: boolean }) {
  return (
    <div className="asb-detail-item">
      <span>{label}</span>
      <strong className={mono ? "ad-mono" : undefined}>{value}</strong>
    </div>
  );
}

function InlineError({ message }: { message: string }) {
  return (
    <div className="asb-error">
      <AlertTriangle strokeWidth={1.75} />
      <span>{message}</span>
    </div>
  );
}

function EmptyState({ loading }: { loading: boolean }) {
  return (
    <div className="asb-empty">
      <LifeBuoy strokeWidth={1.75} />
      <span>{loading ? "Loading support bundles" : "No support bundles found"}</span>
    </div>
  );
}

const supportBundlesCss = `
.asb-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 16px;
}
.asb-stat-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(100px, 1fr));
  gap: 8px;
  min-width: min(560px, 54vw);
}
.asb-metric {
  border: 1px solid var(--dborder);
  background: var(--surface);
  border-radius: 8px;
  padding: 9px 10px;
}
.asb-metric span {
  display: block;
  font-size: 10px;
  color: var(--dmuted);
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-bottom: 4px;
}
.asb-metric strong {
  display: block;
  font-family: var(--font-geist-mono), monospace;
  font-size: 15px;
  color: var(--dtext);
  white-space: nowrap;
}
.asb-filter {
  align-items: stretch;
  flex-wrap: wrap;
}
.asb-search-group {
  position: relative;
}
.asb-search-group svg {
  position: absolute;
  left: 9px;
  top: 50%;
  width: 14px;
  height: 14px;
  transform: translateY(-50%);
  color: var(--dmuted);
  pointer-events: none;
}
.asb-search {
  width: 300px;
  padding-left: 30px;
}
.asb-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.22fr) minmax(360px, 0.78fr);
  gap: 14px;
  min-height: min(760px, calc(100dvh - 184px));
}
.asb-list,
.asb-detail {
  min-width: 0;
  border: 1px solid var(--dborder);
  border-radius: 8px;
  background: var(--surface);
  overflow: hidden;
}
.asb-list {
  display: flex;
  flex-direction: column;
}
.asb-panel-head,
.asb-detail-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--dborder);
  background: var(--surface2);
  font-weight: 600;
  font-size: 13px;
}
.asb-table-wrap {
  border: none;
  border-radius: 0;
  overflow: auto;
}
.asb-table-wrap tbody tr[data-selected="true"] {
  background: color-mix(in srgb, var(--accent-dim) 34%, var(--surface));
}
.asb-row-main {
  display: grid;
  gap: 3px;
  min-width: 0;
}
.asb-row-main span:last-child {
  color: var(--dmuted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}
.asb-badge-gap {
  margin-left: 5px;
}
.asb-detail {
  display: flex;
  flex-direction: column;
}
.asb-detail-title {
  font-family: var(--font-geist-mono), monospace;
  font-size: 12px;
  color: var(--dtext);
}
.asb-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid var(--dborder);
}
.asb-detail-item {
  border: 1px solid var(--dborder2);
  border-radius: 7px;
  padding: 9px;
  background: var(--surface2);
  min-width: 0;
}
.asb-detail-item span {
  display: block;
  color: var(--dmuted);
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  margin-bottom: 4px;
}
.asb-detail-item strong {
  display: block;
  color: var(--dtext);
  font-size: 12px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.asb-report-head {
  display: flex;
  align-items: center;
  gap: 7px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--dborder);
  color: var(--dmuted);
  font-size: 12px;
  font-weight: 600;
}
.asb-report-head svg {
  width: 15px;
  height: 15px;
}
.asb-report {
  flex: 1;
  margin: 0;
  padding: 14px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--dtext);
  background: color-mix(in srgb, var(--surface2) 70%, var(--surface));
  font-family: var(--font-geist-mono), monospace;
  font-size: 11.5px;
  line-height: 1.6;
}
.asb-detail-empty,
.asb-empty {
  min-height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 10px;
  color: var(--dmuted);
  font-size: 13px;
}
.asb-detail-empty svg,
.asb-empty svg {
  width: 24px;
  height: 24px;
}
.asb-error {
  display: flex;
  align-items: center;
  gap: 8px;
  background: var(--danger-soft);
  border: 1px solid color-mix(in srgb, var(--danger) 22%, transparent);
  border-radius: 8px;
  padding: 10px 12px;
  margin-bottom: 12px;
  color: var(--danger);
  font-size: 13px;
}
.asb-error svg {
  width: 16px;
  height: 16px;
  flex-shrink: 0;
}
.asb-skeleton {
  margin: 14px;
  min-height: 320px;
  border-radius: 8px;
  background: linear-gradient(90deg, var(--surface2), var(--surface3), var(--surface2));
  background-size: 180% 100%;
  animation: asb-shimmer 1.2s ease-in-out infinite;
}
@keyframes asb-shimmer {
  0% { background-position: 100% 0; }
  100% { background-position: 0 0; }
}
@media (max-width: 1100px) {
  .asb-header {
    display: grid;
  }
  .asb-stat-strip {
    min-width: 0;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .asb-layout {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 680px) {
  .asb-stat-strip,
  .asb-detail-grid {
    grid-template-columns: 1fr;
  }
  .asb-search,
  .ad-search {
    width: 100%;
  }
}
`;
