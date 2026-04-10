"use client";

import { useState, useMemo } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
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

// ── Mini Calendar ────────────────────────────────────────────────────

function MiniCalendar({ selected, onSelect }: { selected: Date | null; onSelect: (d: Date) => void }) {
  const today = new Date();
  const [viewYear, setViewYear] = useState(selected?.getFullYear() ?? today.getFullYear());
  const [viewMonth, setViewMonth] = useState(selected?.getMonth() ?? today.getMonth());

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
    <div className="select-none">
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
                onClick={() => onSelect(new Date(viewYear, viewMonth, day))}
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

// ── Time Picker (scrollable hour / minute / second) ──────────────────

function TimePicker({ hour, minute, second, onChange }: {
  hour: number; minute: number; second: number;
  onChange: (h: number, m: number, s: number) => void;
}) {
  const hours = Array.from({ length: 24 }, (_, i) => i);
  const minutes = Array.from({ length: 60 }, (_, i) => i);
  const seconds = Array.from({ length: 60 }, (_, i) => i);

  const col = (items: number[], value: number, onPick: (v: number) => void) => (
    <div className="flex-1 h-[140px] overflow-y-auto [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-track]:bg-transparent [&::-webkit-scrollbar-thumb]:bg-[#2e2e38] [&::-webkit-scrollbar-thumb]:rounded-full">
      {items.map((v) => (
        <button
          key={v} type="button" onClick={() => onPick(v)}
          className={`w-full py-1 text-center text-xs font-mono transition-colors rounded ${
            v === value ? "bg-[#10b981] text-[#0a0a0b] font-semibold" : "text-[#8a8a93] hover:bg-[#22222a] hover:text-[#f4f4f5]"
          }`}
        >
          {pad(v)}
        </button>
      ))}
    </div>
  );

  return (
    <div>
      <div className="flex items-center gap-1 mb-1.5">
        <span className="flex-1 text-[10px] text-[#55555c] text-center font-medium">HR</span>
        <span className="flex-1 text-[10px] text-[#55555c] text-center font-medium">MIN</span>
        <span className="flex-1 text-[10px] text-[#55555c] text-center font-medium">SEC</span>
      </div>
      <div className="flex gap-1 rounded-md border border-[#22222a] bg-[#0a0a0b] p-1">
        {col(hours, hour, (h) => onChange(h, minute, second))}
        <div className="w-px bg-[#22222a]" />
        {col(minutes, minute, (m) => onChange(hour, m, second))}
        <div className="w-px bg-[#22222a]" />
        {col(seconds, second, (s) => onChange(hour, minute, s))}
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
  // Parse scheduledAt ("YYYY-MM-DDTHH:MM") into date + time parts.
  const parsed = useMemo(() => {
    if (!scheduledAt) return null;
    const d = new Date(scheduledAt);
    return isNaN(d.getTime()) ? null : d;
  }, [scheduledAt]);

  const now = new Date();
  const selDate = parsed ?? null;
  const selHour = parsed?.getHours() ?? now.getHours();
  const selMinute = parsed?.getMinutes() ?? now.getMinutes();
  const selSecond = parsed ? 0 : 0;

  function handleDateSelect(d: Date) {
    const h = parsed?.getHours() ?? now.getHours();
    const m = parsed?.getMinutes() ?? now.getMinutes();
    onScheduledAtChange(`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(h)}:${pad(m)}`);
  }

  function handleTimeChange(h: number, m: number, _s: number) {
    const d = parsed ?? now;
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
          <div className="space-y-4">
            {/* Calendar */}
            <div>
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-2">
                Date
              </label>
              <div className="rounded-lg border border-[#22222a] bg-[#0a0a0b] p-3">
                <MiniCalendar selected={selDate} onSelect={handleDateSelect} />
              </div>
            </div>

            {/* Time */}
            <div>
              <label className="text-[11px] uppercase tracking-wider text-[#55555c] font-medium block mb-2">
                Time
              </label>
              <TimePicker
                hour={selHour}
                minute={selMinute}
                second={selSecond}
                onChange={handleTimeChange}
              />
            </div>

            {/* Timezone */}
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
