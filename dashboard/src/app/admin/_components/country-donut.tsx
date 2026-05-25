"use client";

import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from "recharts";

import { countryDisplay } from "@/lib/countries";

import { fmtNumber } from "./admin-ui";

export type CountryBreakdownRow = {
  country_code: string;
  count: number;
};

export type SourceBreakdownRow = {
  source_code: string;
  label: string;
  count: number;
};

type BreakdownDonutRow = {
  key: string;
  label: string;
  count: number;
};

const BREAKDOWN_COLORS = ["#059669", "#2563eb", "#d97706", "#dc2626", "#7c3aed", "#64748b"];
const SOURCE_COLORS = ["#0f766e", "#2563eb", "#b45309", "#be123c", "#475569", "#52525b"];

function compactRows(rows: BreakdownDonutRow[]) {
  const top = rows.slice(0, 5).map((row) => ({
    ...row,
  }));
  const rest = rows.slice(5).reduce((sum, row) => sum + row.count, 0);
  if (rest > 0) {
    top.push({ key: "__other", count: rest, label: "Other" });
  }
  return top;
}

function BreakdownDonut({
  title,
  subtitle,
  rows,
  loading,
  valueLabel,
  centerLabel,
  loadingLabel,
  emptyLabel,
  colors = BREAKDOWN_COLORS,
}: {
  title: string;
  subtitle: string;
  rows: BreakdownDonutRow[];
  loading?: boolean;
  valueLabel: string;
  centerLabel: string;
  loadingLabel: string;
  emptyLabel: string;
  colors?: string[];
}) {
  const data = compactRows(rows.filter((row) => row.count > 0));
  const total = rows.reduce((sum, row) => sum + row.count, 0);
  const itemCount = rows.filter((row) => row.count > 0).length;

  return (
    <div className="ad-country-card">
      <style>{countryDonutCss}</style>
      <div className="ad-country-head">
        <div>
          <div className="ad-section-title">{title}</div>
          <div className="ad-section-meta">{subtitle}</div>
        </div>
        <div className="ad-country-total">
          <strong>{fmtNumber(total)}</strong>
          <span>{valueLabel}</span>
        </div>
      </div>

      {data.length > 0 ? (
        <div className="ad-country-body">
          <div className="ad-country-chart">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={data}
                  dataKey="count"
                  nameKey="label"
                  innerRadius="62%"
                  outerRadius="88%"
                  paddingAngle={2}
                  stroke="var(--surface)"
                  strokeWidth={2}
                >
                  {data.map((entry, index) => (
                    <Cell key={entry.key || `other-${index}`} fill={colors[index % colors.length]} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    background: "var(--surface-raised)",
                    border: "1px solid var(--dborder)",
                    borderRadius: 8,
                    fontSize: 12,
                  }}
                  formatter={(value, _name, item) => [fmtNumber(Number(value ?? 0)), item.payload.label]}
                />
              </PieChart>
            </ResponsiveContainer>
            <div className="ad-country-center">
              <strong>{itemCount}</strong>
              <span>{centerLabel}</span>
            </div>
          </div>
          <div className="ad-country-list">
            {data.map((row, index) => {
              const pct = total > 0 ? (row.count / total) * 100 : 0;
              return (
                <div className="ad-country-row" key={row.key || `other-row-${index}`}>
                  <span className="ad-country-dot" style={{ background: colors[index % colors.length] }} />
                  <span className="ad-country-label">{row.label}</span>
                  <span className="ad-country-count">{fmtNumber(row.count)}</span>
                  <span className="ad-country-pct">{pct.toFixed(0)}%</span>
                </div>
              );
            })}
          </div>
        </div>
      ) : (
        <div className="ad-country-empty">{loading ? loadingLabel : emptyLabel}</div>
      )}
    </div>
  );
}

export function CountryDonut({
  title,
  subtitle,
  rows,
  loading,
  valueLabel,
}: {
  title: string;
  subtitle: string;
  rows: CountryBreakdownRow[];
  loading?: boolean;
  valueLabel: string;
}) {
  return (
    <BreakdownDonut
      title={title}
      subtitle={subtitle}
      rows={rows.map((row) => ({
        key: row.country_code || "__unknown_country",
        label: countryDisplay(row.country_code),
        count: row.count,
      }))}
      loading={loading}
      valueLabel={valueLabel}
      centerLabel="countries"
      loadingLabel="Loading country mix..."
      emptyLabel="No country data yet"
    />
  );
}

export function SourceDonut({
  title,
  subtitle,
  rows,
  loading,
  valueLabel,
}: {
  title: string;
  subtitle: string;
  rows: SourceBreakdownRow[];
  loading?: boolean;
  valueLabel: string;
}) {
  return (
    <BreakdownDonut
      title={title}
      subtitle={subtitle}
      rows={rows.map((row) => ({
        key: row.source_code || "__unknown_source",
        label: row.label || row.source_code || "Unknown",
        count: row.count,
      }))}
      loading={loading}
      valueLabel={valueLabel}
      centerLabel="sources"
      loadingLabel="Loading source mix..."
      emptyLabel="No source data yet"
      colors={SOURCE_COLORS}
    />
  );
}

const countryDonutCss = `
.ad-country-card {
  background: var(--surface);
  border: 1px solid var(--dborder);
  border-radius: 8px;
  padding: 14px 16px 16px;
  min-height: 280px;
}
.ad-country-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.ad-country-total {
  text-align: right;
  font-family: var(--font-geist-mono), monospace;
}
.ad-country-total strong {
  display: block;
  color: var(--dtext);
  font-size: 17px;
  line-height: 1.1;
}
.ad-country-total span {
  display: block;
  color: var(--dmuted2);
  font-size: 10px;
  margin-top: 2px;
}
.ad-country-body {
  display: grid;
  grid-template-columns: minmax(118px, 0.9fr) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  min-height: 216px;
}
.ad-country-chart {
  position: relative;
  height: 176px;
  min-width: 0;
}
.ad-country-center {
  position: absolute;
  inset: 50% auto auto 50%;
  transform: translate(-50%, -50%);
  text-align: center;
  pointer-events: none;
}
.ad-country-center strong {
  display: block;
  color: var(--dtext);
  font-family: var(--font-geist-mono), monospace;
  font-size: 20px;
  line-height: 1;
}
.ad-country-center span {
  display: block;
  color: var(--dmuted2);
  font-size: 10px;
  margin-top: 3px;
}
.ad-country-list {
  display: grid;
  gap: 7px;
  min-width: 0;
}
.ad-country-row {
  display: grid;
  grid-template-columns: 9px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 7px;
  min-width: 0;
  font-size: 11.5px;
}
.ad-country-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
}
.ad-country-label {
  color: var(--dtext);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.ad-country-count {
  color: var(--dmuted);
  font-family: var(--font-geist-mono), monospace;
}
.ad-country-pct {
  color: var(--dmuted2);
  font-family: var(--font-geist-mono), monospace;
  width: 34px;
  text-align: right;
}
.ad-country-empty {
  min-height: 216px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--dmuted);
  font-size: 13px;
}
@media (max-width: 900px) {
  .ad-country-body {
    grid-template-columns: 1fr;
  }
}
`;
