"use client";

import { memo, useMemo, useState } from "react";

import type { AdminObjectStorageDailyActivity } from "@/lib/api";

import { fmtBytes } from "../_components/admin-ui";

type DailyGroupProps = {
  row: AdminObjectStorageDailyActivity;
  maxBytes: number;
  showLabel: boolean;
  active: boolean;
  onActivate: (date: string | null) => void;
};

const CHART_TICKS = [1, 0.75, 0.5, 0.25, 0];

export function ObjectStorageDailyChart({ rows }: { rows: AdminObjectStorageDailyActivity[] }) {
  const [activeDate, setActiveDate] = useState<string | null>(null);
  const maxBytes = useMemo(
    () => Math.max(1, ...rows.flatMap((row) => [row.confirmed_bytes, row.deleted_bytes])),
    [rows],
  );
  const active = rows.find((row) => row.date === activeDate) ?? null;

  if (rows.every((row) => row.confirmed_bytes === 0 && row.deleted_bytes === 0)) {
    return <div className="aos-chart-empty">No Confirm or DELETE activity in this period.</div>;
  }

  const labelEvery = Math.max(1, Math.ceil(rows.length / 7));

  return (
    <div className="aos-chart" role="group" aria-label="Daily Confirm and DELETE size">
      <div className="aos-chart-legend" aria-hidden="true">
        <span><i className="aos-chart-confirm" />Confirm size</span>
        <span><i className="aos-chart-delete" />DELETE size</span>
      </div>
      <div className="aos-chart-scroll">
        <div className="aos-chart-axis" aria-hidden="true">
          {CHART_TICKS.map((ratio) => <span key={ratio}>{fmtBytes(maxBytes * ratio)}</span>)}
        </div>
        <div className="aos-chart-plot">
          <div className="aos-chart-gridlines" aria-hidden="true">
            <i /><i /><i /><i />
          </div>
          <div className="aos-chart-groups" style={{ gridTemplateColumns: `repeat(${Math.max(rows.length, 7)}, minmax(38px, 1fr))` }}>
            {rows.map((row, index) => (
              <DailyGroup
                key={row.date}
                row={row}
                maxBytes={maxBytes}
                showLabel={index === 0 || index === rows.length - 1 || index % labelEvery === 0}
                active={row.date === activeDate}
                onActivate={setActiveDate}
              />
            ))}
          </div>
        </div>
      </div>
      {active && (
        <div className="aos-chart-tooltip" role="status">
          <strong>{formatUTCDate(active.date)}</strong>
          <span>Confirm {fmtBytes(active.confirmed_bytes)}</span>
          <span>DELETE {fmtBytes(active.deleted_bytes)}</span>
        </div>
      )}
    </div>
  );
}

const DailyGroup = memo(function DailyGroup({ row, maxBytes, showLabel, active, onActivate }: DailyGroupProps) {
  const dateLabel = formatUTCDate(row.date);
  const confirmLabel = `${dateLabel}: Confirm size ${fmtBytes(row.confirmed_bytes)}`;
  const deleteLabel = `${dateLabel}: DELETE size ${fmtBytes(row.deleted_bytes)}`;

  return (
    <div className="aos-chart-group">
      <div className="aos-chart-bars">
        <button
          type="button"
          className="aos-chart-bar aos-chart-confirm"
          style={{ height: `${barHeight(row.confirmed_bytes, maxBytes)}%` }}
          data-empty={row.confirmed_bytes === 0}
          aria-label={confirmLabel}
          aria-pressed={active}
          onMouseEnter={() => onActivate(row.date)}
          onMouseLeave={() => onActivate(null)}
          onFocus={() => onActivate(row.date)}
          onBlur={() => onActivate(null)}
          onClick={() => onActivate(active ? null : row.date)}
        />
        <button
          type="button"
          className="aos-chart-bar aos-chart-delete"
          style={{ height: `${barHeight(row.deleted_bytes, maxBytes)}%` }}
          data-empty={row.deleted_bytes === 0}
          aria-label={deleteLabel}
          aria-pressed={active}
          onMouseEnter={() => onActivate(row.date)}
          onMouseLeave={() => onActivate(null)}
          onFocus={() => onActivate(row.date)}
          onBlur={() => onActivate(null)}
          onClick={() => onActivate(active ? null : row.date)}
        />
      </div>
      <span className="aos-chart-date" aria-hidden={!showLabel}>{showLabel ? dateLabel : ""}</span>
    </div>
  );
});

function barHeight(value: number, maxBytes: number) {
  if (value <= 0) return 0;
  return Math.max(2, (value / maxBytes) * 100);
}

function formatUTCDate(date: string) {
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", timeZone: "UTC" }).format(new Date(`${date}T00:00:00Z`));
}
