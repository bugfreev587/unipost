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
    <div ref={ref} className="absolute top-full left-0 right-0 mt-1 z-50 rounded-lg border border-[#22222a] bg-[#111113] p-3 shadow-xl">
      <div className="flex items-center justify-between mb-2">
        <button type="button" onClick={prev} className="w-6 h-6 flex items-center justify-center rounded hover:bg-[#22222a] text-[#8a8a93] hover:text-[#f4f4f5] transition-colors">
          <ChevronLeft className="w-3.5 h-3.5" />
        </button>
        <span className="text-xs font-medium text-[#f4f4f5]">{MONTH_NAMES[viewMonth]} {viewYear}</span>
        <button type="button" onClick={next} className="w-6 h-6 flex items-center justify-center rounded hover:bg-[#22222a] text-[#8a8a93] hover:text-[#f4f4f5] transition-colors">
          <ChevronRight className="w-3.5 h-3.5" />
        </button>
      </div>
      <div className="grid grid-cols-7 gap-0.5 text-center">
        {DAY_LABELS.map((l) => (
          <div key={l} className="text-[10px] text-[#55555c] py-1 font-medium">{l}</div>
        ))}
        {days.map((day, i) => (
          <div key={i} className="aspect-square flex items-center justify-center">
            {day !== null && (
              <button
                type="button"
                disabled={isPast(day)}
                onClick={() => { onSelect(new Date(viewYear, viewMonth, day)); onClose(); }}
                className={`w-7 h-7 rounded-md text-[11px] font-medium transition-all duration-100 ${
                  isSelected(day)
                    ? "bg-[#10b981] text-[#0a0a0b]"
                    : isToday(day)
                    ? "bg-[#22222a] text-[#f4f4f5]"
                    : isPast(day)
                    ? "text-[#33333a] cursor-not-allowed"
                    : "text-[#8a8a93] hover:bg-[#22222a] hover:text-[#f4f4f5]"
                }`}
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
      <label className="text-xs uppercase tracking-wider text-[#55555c] font-medium block mb-3">
        Publish
      </label>

      {/* Segmented control */}
      <div className="grid grid-cols-4 gap-1 p-1 bg-[#17171a] rounded-lg border border-[#22222a]">
        {MODES.map((m) => (
          <button
            key={m.value}
            type="button"
            onClick={() => onModeChange(m.value)}
            className={`rounded-md py-1.5 text-xs font-medium transition-all duration-[160ms] ${
              mode === m.value
                ? "bg-[#f4f4f5] text-[#0a0a0b]"
                : "text-[#8a8a93] hover:text-[#f4f4f5]"
            }`}
          >
            {m.label}
          </button>
        ))}
      </div>

      {/* Mode-specific panels */}
      <div className="mt-4">
        {mode === "now" && (
          <div className="flex gap-2.5 p-3 rounded-lg bg-[#17171a]/60 border border-[#22222a]">
            <div className="w-1 rounded-full bg-[#10b981] flex-shrink-0" />
            <div className="text-[12.5px] leading-relaxed text-[#8a8a93]">
              Publishes immediately to every account selected above.
            </div>
          </div>
        )}

        {mode === "schedule" && (
          <div className="space-y-3">
            <div>
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
                Date &amp; time
              </label>
              <div className="relative">
                <input
                  type="datetime-local"
                  value={scheduledAt}
                  onChange={(e) => onScheduledAtChange(e.target.value)}
                  min={new Date().toISOString().slice(0, 16)}
                  className="w-full rounded-md px-3 py-2 text-sm font-mono bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)] pr-10"
                />
                <button
                  type="button"
                  onClick={() => setCalendarOpen(!calendarOpen)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 w-7 h-7 flex items-center justify-center rounded hover:bg-[#22222a] text-[#8a8a93] hover:text-[#f4f4f5] transition-colors"
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
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
                Timezone
              </label>
              <select className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]">
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
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-1.5">
                Add to queue
              </label>
              <select
                value={queueId}
                onChange={(e) => onQueueIdChange(e.target.value)}
                className="w-full rounded-md px-3 py-2 text-sm bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
              >
                <option value="">Select a queue...</option>
                {queues.map((q) => (
                  <option key={q.id} value={q.id}>{q.name}</option>
                ))}
              </select>
            </div>
            {nextSlot && (
              <div className="flex gap-2.5 p-3 rounded-lg bg-[#17171a]/60 border border-[#22222a]">
                <div className="w-1 rounded-full bg-teal-500 flex-shrink-0" />
                <div className="text-[12.5px] leading-relaxed text-[#8a8a93]">
                  Will publish at the next open slot —{" "}
                  <span className="text-[#f4f4f5] font-mono text-[11.5px]">{nextSlot}</span>
                </div>
              </div>
            )}
          </div>
        )}

        {mode === "draft" && (
          <div className="flex gap-2.5 p-3 rounded-lg bg-[#17171a]/60 border border-[#22222a]">
            <div className="w-1 rounded-full bg-[#55555c] flex-shrink-0" />
            <div className="text-[12.5px] leading-relaxed text-[#8a8a93]">
              Saves without publishing. You can edit and send it later.
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
