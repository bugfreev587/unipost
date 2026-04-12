"use client";

export function EmptyPlatformState() {
  return (
    <div
      className="rounded-lg border border-dashed py-10 px-6 text-center"
      style={{ borderColor: "var(--dborder2)", background: "color-mix(in srgb, var(--surface2) 72%, transparent)" }}
    >
      <div className="mb-1 font-serif text-xl italic" style={{ color: "var(--dmuted)" }}>
        Nothing selected yet
      </div>
      <p className="text-[13px]" style={{ color: "var(--dmuted2)" }}>
        Pick one or more accounts on the right to unlock platform-specific fields.
      </p>
    </div>
  );
}
