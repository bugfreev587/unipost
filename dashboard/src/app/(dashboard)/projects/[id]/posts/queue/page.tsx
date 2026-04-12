"use client";

import { Calendar } from "lucide-react";

export default function QueuePage() {
  return (
    <div style={{ display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", minHeight: 400, textAlign: "center", padding: 40 }}>
      <div style={{ width: 56, height: 56, borderRadius: 14, background: "var(--surface1)", border: "1px solid var(--dborder)", display: "flex", alignItems: "center", justifyContent: "center", marginBottom: 20 }}>
        <Calendar style={{ width: 24, height: 24, color: "var(--dmuted)" }} />
      </div>
      <div style={{ fontSize: 20, fontWeight: 700, color: "var(--dtext)", marginBottom: 8 }}>
        Queue &mdash; Coming Soon
      </div>
      <p style={{ fontSize: 14, color: "var(--dmuted)", maxWidth: 400, lineHeight: 1.6, marginBottom: 24 }}>
        Scheduled posting queues are under development. Queues let you create a recurring posting schedule and automatically publish posts at the next available time slot.
      </p>
      <a href="https://github.com/unipost-dev" target="_blank" rel="noopener noreferrer" className="dbtn dbtn-ghost" style={{ fontSize: 13 }}>
        View roadmap &rarr;
      </a>
    </div>
  );
}
