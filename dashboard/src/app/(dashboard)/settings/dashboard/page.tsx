"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@clerk/nextjs";
import { listWorkspaces, updateWorkspace, type Workspace } from "@/lib/api";

const ALL_FEATURES_ID = "all";

const MODE_OPTIONS = [
  {
    id: "personal",
    title: "Post to my own accounts",
    description:
      "Connect your social accounts and publish to all of them in one click. Shows Quickstart under Connections.",
    features: ["Connections > Quickstart"],
  },
  {
    id: "whitelabel",
    title: "Post with my own app credentials",
    description:
      "Use your own OAuth apps for each platform. Your brand shows up during authorization. Shows White-label under Connections and API Keys.",
    features: ["Connections > Quickstart", "Connections > White-label", "API Keys"],
  },
  {
    id: "api",
    title: "Build an app on UniPost API",
    description:
      "Your customers connect their accounts through a hosted OAuth flow, and you post on their behalf. Shows Developer App Users and API Keys.",
    features: ["Connections > Developer App Users", "API Keys"],
  },
  {
    id: ALL_FEATURES_ID,
    title: "All features enabled",
    description:
      "Show every feature in the dashboard. Choose this if you use multiple workflows or just want full access to everything.",
    features: [],
  },
];

export default function DashboardSettingsTab() {
  const { getToken } = useAuth();
  const [workspace, setWorkspace] = useState<Workspace | null>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    async function load() {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await listWorkspaces(token);
        if (res.data.length > 0) {
          const ws = res.data[0];
          setWorkspace(ws);
          if (ws.usage_modes.length === 0) {
            setSelected(new Set([ALL_FEATURES_ID]));
          } else {
            setSelected(new Set(ws.usage_modes));
          }
        }
      } catch (err) {
        console.error("Failed to load workspace:", err);
      }
    }
    load();
  }, [getToken]);

  function toggle(id: string) {
    setSelected((prev) => {
      if (id === ALL_FEATURES_ID) {
        return prev.has(ALL_FEATURES_ID) ? new Set() : new Set([ALL_FEATURES_ID]);
      }
      const next = new Set(prev);
      next.delete(ALL_FEATURES_ID);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleSave() {
    if (!workspace) return;
    setSaving(true);
    setSaved(false);
    try {
      const token = await getToken();
      if (!token) return;
      const modes = selected.has(ALL_FEATURES_ID)
        ? []
        : [...selected];
      const res = await updateWorkspace(token, workspace.id, {
        name: workspace.name,
        usage_modes: modes,
      });
      setWorkspace(res.data);
      setSaved(true);
      setTimeout(() => window.location.reload(), 600);
    } catch (err) {
      console.error("Failed to save:", err);
    } finally {
      setSaving(false);
    }
  }

  const hasChanges = (() => {
    if (!workspace) return false;
    const currentModes = workspace.usage_modes;
    if (selected.has(ALL_FEATURES_ID)) return currentModes.length !== 0;
    if (currentModes.length !== selected.size) return true;
    return currentModes.some((m) => !selected.has(m));
  })();

  if (!workspace) return <div style={{ color: "var(--dmuted)" }}>Loading...</div>;

  return (
    <>
      <div className="settings-section">
        <div className="settings-section-header">Dashboard Features</div>
        <div className="settings-section-body">
          <div className="dt-body-sm" style={{ marginBottom: 16, lineHeight: 1.6 }}>
            Choose which workflow modes are active. The dashboard sidebar will
            show only the features relevant to your selected modes. Changes take
            effect after you click Save and refresh the page.
          </div>

          <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
            {MODE_OPTIONS.map((mode) => {
              const isAll = mode.id === ALL_FEATURES_ID;
              const active = selected.has(mode.id);
              return (
                <button
                  key={mode.id}
                  onClick={() => toggle(mode.id)}
                  style={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 12,
                    padding: "14px 16px",
                    border: `1px solid ${active ? "var(--daccent)" : "var(--dborder)"}`,
                    borderRadius: 8,
                    background: active ? "rgba(16,185,129,0.05)" : "transparent",
                    cursor: "pointer",
                    textAlign: "left",
                    fontFamily: "inherit",
                    transition: "all 0.1s",
                    width: "100%",
                  }}
                  onMouseEnter={(e) => {
                    if (!active) e.currentTarget.style.borderColor = "#333";
                  }}
                  onMouseLeave={(e) => {
                    if (!active)
                      e.currentTarget.style.borderColor = "var(--dborder)";
                  }}
                >
                  <div
                    style={{
                      width: 18,
                      height: 18,
                      borderRadius: isAll ? 9 : 4,
                      border: `2px solid ${active ? "var(--daccent)" : "var(--dmuted2)"}`,
                      background: active ? "var(--daccent)" : "transparent",
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                      flexShrink: 0,
                      marginTop: 1,
                      transition: "all 0.1s",
                    }}
                  >
                    {active && (
                      <svg
                        width="10"
                        height="10"
                        viewBox="0 0 10 10"
                        fill="none"
                        stroke="#000"
                        strokeWidth="2"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      >
                        <polyline points="1.5 5 4 7.5 8.5 2.5" />
                      </svg>
                    )}
                  </div>
                  <div style={{ flex: 1 }}>
                    <div className="dt-body" style={{ fontWeight: 600, marginBottom: 3 }}>

                      {mode.title}
                    </div>
                    <div className="dt-body-sm" style={{ lineHeight: 1.5 }}>
                      {mode.description}
                    </div>
                    {mode.features.length > 0 && (
                      <div
                        style={{
                          display: "flex",
                          gap: 6,
                          flexWrap: "wrap",
                          marginTop: 8,
                        }}
                      >
                        {mode.features.map((f) => (
                          <span
                            key={f}
                            style={{
                              fontSize: 11,
                              padding: "2px 8px",
                              borderRadius: 4,
                              background: active
                                ? "rgba(16,185,129,0.12)"
                                : "var(--surface2)",
                              color: active ? "var(--daccent)" : "var(--dmuted)",
                              fontWeight: 500,
                            }}
                          >
                            {f}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                </button>
              );
            })}
          </div>

          {selected.size === 0 && (
            <div
              style={{
                marginTop: 12,
                padding: "8px 12px",
                borderRadius: 6,
                background: "#f59e0b10",
                border: "1px solid #f59e0b25",
                fontSize: 13,
                color: "var(--warning)",
              }}
            >
              No mode selected. Please select at least one option.
            </div>
          )}

          <div
            style={{
              display: "flex",
              justifyContent: "flex-end",
              marginTop: 20,
              paddingTop: 16,
              borderTop: "1px solid var(--dborder)",
            }}
          >
            <button
              className="dbtn dbtn-primary"
              onClick={handleSave}
              disabled={saving || !hasChanges || selected.size === 0}
            >
              {saving ? "Saving..." : saved ? "✓ Saved" : "Save"}
            </button>
          </div>
        </div>
      </div>
    </>
  );
}
