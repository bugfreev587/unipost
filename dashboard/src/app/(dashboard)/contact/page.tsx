"use client";

import { Bug, Lightbulb, HelpCircle } from "lucide-react";

const CONTACT_OPTIONS = [
  {
    icon: Bug,
    emoji: "🐛",
    title: "I want to report a bug",
    desc: "Something isn't working as expected.",
    subject: "Bug Report",
  },
  {
    icon: Lightbulb,
    emoji: "💡",
    title: "I want to request a feature",
    desc: "I have an idea for a new feature.",
    subject: "Feature Request",
  },
  {
    icon: HelpCircle,
    emoji: "🙋",
    title: "I need help",
    desc: "I have a question or need assistance.",
    subject: "Help Request",
  },
];

export default function ContactPage() {
  function openMail(subject: string) {
    window.location.href = `mailto:admin@unipost.dev?subject=${encodeURIComponent(`[UniPost] ${subject}`)}`;
  }

  return (
    <div style={{ maxWidth: 640, margin: "0 auto", paddingTop: 32 }}>
      <div style={{ textAlign: "center", marginBottom: 48 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: "var(--daccent)", fontFamily: "var(--font-geist-mono), monospace", marginBottom: 8 }}>
          Contact UniPost
        </div>
        <h1 style={{ fontSize: 28, fontWeight: 800, letterSpacing: -0.5, color: "var(--dtext)", marginBottom: 10 }}>
          Get help or ask a question
        </h1>
        <p style={{ fontSize: 14, color: "var(--dmuted)", lineHeight: 1.7 }}>
          Report a bug, request a feature, or ask a question about UniPost.
        </p>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
        {CONTACT_OPTIONS.slice(0, 2).map((opt) => (
          <button
            key={opt.title}
            onClick={() => openMail(opt.subject)}
            style={{
              display: "flex", alignItems: "center", gap: 12,
              padding: "18px 20px", borderRadius: 10,
              border: "1px solid var(--dborder2)", background: "var(--surface)",
              cursor: "pointer", transition: "all 0.15s", textAlign: "left",
              width: "100%", fontFamily: "inherit",
            }}
            onMouseEnter={(e) => { e.currentTarget.style.borderColor = "#333"; e.currentTarget.style.background = "var(--surface2)"; }}
            onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = "var(--surface)"; }}
          >
            <span style={{ fontSize: 20 }}>{opt.emoji}</span>
            <span style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)" }}>{opt.title}</span>
          </button>
        ))}
      </div>
      <div style={{ marginTop: 12 }}>
        <button
          onClick={() => openMail(CONTACT_OPTIONS[2].subject)}
          style={{
            display: "flex", alignItems: "center", gap: 12,
            padding: "18px 20px", borderRadius: 10,
            border: "1px solid var(--dborder2)", background: "var(--surface)",
            cursor: "pointer", transition: "all 0.15s", textAlign: "left",
            width: "calc(50% - 6px)", fontFamily: "inherit",
          }}
          onMouseEnter={(e) => { e.currentTarget.style.borderColor = "#333"; e.currentTarget.style.background = "var(--surface2)"; }}
          onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = "var(--surface)"; }}
        >
          <span style={{ fontSize: 20 }}>{CONTACT_OPTIONS[2].emoji}</span>
          <span style={{ fontSize: 14, fontWeight: 500, color: "var(--dtext)" }}>{CONTACT_OPTIONS[2].title}</span>
        </button>
      </div>
    </div>
  );
}
