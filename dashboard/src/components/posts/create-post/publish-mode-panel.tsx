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
              <input
                type="datetime-local"
                value={scheduledAt}
                onChange={(e) => onScheduledAtChange(e.target.value)}
                min={new Date().toISOString().slice(0, 16)}
                className="w-full rounded-md border px-3 py-2 text-sm font-mono outline-none transition-[border-color,box-shadow] duration-[140ms]"
                style={{ background: "var(--surface1)", borderColor: "var(--dborder)", color: "var(--dtext)" }}
              />
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
