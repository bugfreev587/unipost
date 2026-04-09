"use client";

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
              <input
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => onScheduledAtChange(e.target.value)}
                min={new Date().toISOString().slice(0, 16)}
                className="w-full rounded-md px-3 py-2 text-sm font-mono bg-[#0a0a0b] border border-[#22222a] text-[#f4f4f5] outline-none transition-[border-color] duration-[140ms] focus:border-[#10b981] focus:shadow-[0_0_0_3px_rgba(16,185,129,0.15)]"
              />
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
                <option value="">Select a queue…</option>
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
