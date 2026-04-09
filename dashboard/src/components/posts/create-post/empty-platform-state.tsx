"use client";

export function EmptyPlatformState() {
  return (
    <div className="rounded-lg border border-dashed border-[#22222a] bg-[#0a0a0b]/40 py-10 px-6 text-center">
      <div className="font-serif text-xl text-[#8a8a93] italic mb-1">
        Nothing selected yet
      </div>
      <p className="text-[13px] text-[#55555c]">
        Pick one or more accounts on the right to unlock platform-specific fields.
      </p>
    </div>
  );
}
