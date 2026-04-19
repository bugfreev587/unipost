"use client";

import { useState, useMemo, useRef, useEffect } from "react";
import { ChevronLeft, ChevronRight, Calendar } from "lucide-react";
import { type PublishMode } from "./use-create-post-form";

interface PublishModePanelProps {
  mode: PublishMode;
  onModeChange: (mode: PublishMode) => void;
  scheduledAt: string;
  onScheduledAtChange: (value: string) => void;
  queueId: string;
  onQueueIdChange: (value: string) => void;
  queues: Array<{ id: string; name: string }>;
  nextSlot?: string;
}

const MODES: { value: PublishMode; label: string }[] = [
  { value: "now", label: "Now" },
  { value: "schedule", label: "Schedule" },
  { value: "queue", label: "Queue" },
  { value: "draft", label: "Draft" },
];

const MONTH_NAMES = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const DAY_LABELS = ["Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"];

function pad(n: number) { return String(n).padStart(2, "0"); }

// ── Mini Calendar Dropdown ───────────────────────────────────────────

function MiniCalendar({ selected, onSelect, onClose }: {
  selected: Date | null;
  onSelect: (d: Date) => void;
  onClose: () => void;
}) {
  const today = new Date();
  const [viewYear, setViewYear] = useState(selected?.getFullYear() ?? today.getFullYear());
  const [viewMonth, setViewMonth] = useState(selected?.getMonth() ?? today.getMonth());
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener("mousedown", handleClick);
    return () => document.removeEventListener("mousedown", handleClick);
  }, [onClose]);

  const days = useMemo(() => {
    const first = new Date(viewYear, viewMonth, 1);
    const startDay = first.getDay();
    const daysInMonth = new Date(viewYear, viewMonth + 1, 0).getDate();
    const cells: (number | null)[] = [];
    for (let i = 0; i < startDay; i++) cells.push(null);
    for (let d = 1; d <= daysInMonth; d++) cells.push(d);
    return cells;
  }, [viewYear, viewMonth]);

  function prev() {
    if (viewMonth === 0) { setViewMonth(11); setViewYear(viewYear - 1); }
    else setViewMonth(viewMonth - 1);
  }
  function next() {
    if (viewMonth === 11) { setViewMonth(0); setViewYear(viewYear + 1); }
    else setViewMonth(viewMonth + 1);
  }
  function isToday(day: number) {
    return day === today.getDate() && viewMonth === today.getMonth() && viewYear === today.getFullYear();
  }
  function isSelected(day: number) {
    return selected && day === selected.getDate() && viewMonth === selected.getMonth() && viewYear === selected.getFullYear();
  }
  function isPast(day: number) {
    const d = new Date(viewYear, viewMonth, day);
    d.setHours(23, 59, 59);
    return d < today;
  }

  return (
    <div ref={ref} className="absolute top-full left-0 right-0 z-50 mt-1 rounded-lg border p-3 shadow-xl" style={{ borderColor: "var(--dborder)", background: "var(--surface-raised)" }}>
      <div className="flex items-center justify-between mb-2">
        <button type="button" onClick={prev} className="flex h-6 w-6 items-center justify-center rounded transition-colors" style={{ color: "var(--dmuted)" }}>
          <ChevronLeft className="w-3.5 h-3.5" />
        </button>
        <span className="text-xs font-medium" style={{ color: "var(--dtext)" }}>{MONTH_NAMES[viewMonth]} {viewYear}</span>
        <button type="button" onClick={next} className="flex h-6 w-6 items-center justify-center rounded transition-colors" style={{ color: "var(--dmuted)" }}>
          <ChevronRight className="w-3.5 h-3.5" />
        </button>
      </div>
      <div className="grid grid-cols-7 gap-0.5 text-center">
        {DAY_LABELS.map((l) => (
          <div key={l} className="py-1 text-[10px] font-medium" style={{ color: "var(--dmuted2)" }}>{l}</div>
        ))}
        {days.map((day, i) => (
          <div key={i} className="aspect-square flex items-center justify-center">
            {day !== null && (
              <button
                type="button"
                disabled={isPast(day)}
                onClick={() => { onSelect(new Date(viewYear, viewMonth, day)); onClose(); }}
                className="h-7 w-7 rounded-md text-[11px] font-medium transition-all duration-100"
                style={
                  isSelected(day)
                    ? { background: "var(--primary)", color: "var(--primary-foreground)" }
                    : isToday(day)
                    ? { background: "var(--surface3)", color: "var(--dtext)" }
                    : isPast(day)
                    ? { color: "var(--dmuted2)", opacity: 0.5, cursor: "not-allowed" }
                    : { color: "var(--dmuted)" }
                }
              >
                {day}
              </button>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main Panel ───────────────────────────────────────────────────────

export function PublishModePanel({
  mode,
  onModeChange,
  scheduledAt,
  onScheduledAtChange,
  queueId,
  onQueueIdChange,
  queues,
  nextSlot,
}: PublishModePanelProps) {
  const [calendarOpen, setCalendarOpen] = useState(false);

  const parsed = useMemo(() => {
    if (!scheduledAt) return null;
    const d = new Date(scheduledAt);
    return isNaN(d.getTime()) ? null : d;
  }, [scheduledAt]);

  function handleDateSelect(d: Date) {
    // Preserve existing time, or default to current time
    const now = new Date();
    const h = parsed?.getHours() ?? now.getHours();
    const m = parsed?.getMinutes() ?? now.getMinutes();
    onScheduledAtChange(`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(h)}:${pad(m)}`);
  }

  return (
    <div>
      <label className="mb-3 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
        Publish
      </label>

      {/* Segmented control */}
      <div className="grid grid-cols-4 gap-1 rounded-lg border p-1" style={{ background: "var(--surface2)", borderColor: "var(--dborder)" }}>
        {MODES.map((m) => (
          <button
            key={m.value}
            type="button"
            onClick={() => onModeChange(m.value)}
            className="rounded-md py-1.5 text-xs font-medium transition-all duration-[160ms]"
            style={mode === m.value ? { background: "var(--surface-raised)", color: "var(--dtext)" } : { color: "var(--dmuted)" }}
          >
            {m.label}
          </button>
        ))}
      </div>

      {/* Mode-specific panels */}
      <div className="mt-4">
        {mode === "now" && (
          <div className="flex gap-2.5 rounded-lg border p-3" style={{ background: "color-mix(in srgb, var(--surface2) 70%, transparent)", borderColor: "var(--dborder)" }}>
            <div className="w-1 flex-shrink-0 rounded-full" style={{ background: "var(--primary)" }} />
            <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
              Publishes immediately to every account selected above.
            </div>
          </div>
        )}

        {mode === "schedule" && (
          <div className="space-y-3">
            <div>
              <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                Date &amp; time
              </label>
              <div className="relative">
                <input
                  type="datetime-local"
                  value={scheduledAt}
                  onChange={(e) => onScheduledAtChange(e.target.value)}
                  min={new Date().toISOString().slice(0, 16)}
                  className="w-full rounded-md border px-3 py-2 pr-10 text-sm font-mono outline-none transition-[border-color,box-shadow] duration-[140ms]"
                  style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
                />
                <button
                  type="button"
                  onClick={() => setCalendarOpen(!calendarOpen)}
                  className="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded transition-colors"
                  style={{ color: "var(--dmuted)" }}
                >
                  <Calendar className="w-4 h-4" />
                </button>
                {calendarOpen && (
                  <MiniCalendar
                    selected={parsed}
                    onSelect={handleDateSelect}
                    onClose={() => setCalendarOpen(false)}
                  />
                )}
              </div>
            </div>
            <div>
              <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                Timezone
              </label>
              <select className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]" style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}>
                <option>America/Los_Angeles (PDT)</option>
                <option>America/New_York (EDT)</option>
                <option>UTC</option>
              </select>
            </div>
          </div>
        )}

        {mode === "queue" && (
          <div className="space-y-3">
            <div>
              <label className="mb-1.5 block text-[11px] font-semibold uppercase tracking-[0.11em]" style={{ color: "var(--dmuted2)" }}>
                Add to queue
              </label>
              <select
                value={queueId}
                onChange={(e) => onQueueIdChange(e.target.value)}
                className="w-full rounded-md border px-3 py-2 text-sm outline-none transition-[border-color,box-shadow] duration-[140ms]"
                style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
              >
                <option value="">Select a queue...</option>
                {queues.map((q) => (
                  <option key={q.id} value={q.id}>{q.name}</option>
                ))}
              </select>
            </div>
            {nextSlot && (
              <div className="flex gap-2.5 rounded-lg border p-3" style={{ background: "color-mix(in srgb, var(--surface2) 70%, transparent)", borderColor: "var(--dborder)" }}>
                <div className="w-1 flex-shrink-0 rounded-full" style={{ background: "var(--primary)" }} />
                <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
                  Will publish at the next open slot —{" "}
                  <span className="font-mono text-[11.5px]" style={{ color: "var(--dtext)" }}>{nextSlot}</span>
                </div>
              </div>
            )}
          </div>
        )}

        {mode === "draft" && (
          <div className="flex gap-2.5 rounded-lg border p-3" style={{ background: "color-mix(in srgb, var(--surface2) 70%, transparent)", borderColor: "var(--dborder)" }}>
            <div className="w-1 flex-shrink-0 rounded-full" style={{ background: "var(--dmuted2)" }} />
            <div className="text-[12.5px] leading-relaxed" style={{ color: "var(--dmuted)" }}>
              Saves without publishing. You can edit and send it later.
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
