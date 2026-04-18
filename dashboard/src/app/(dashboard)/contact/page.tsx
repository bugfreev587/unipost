"use client";

import { useMemo } from "react";
import { useSearchParams } from "next/navigation";
import { Bug, Lightbulb, HelpCircle, Mail, MessageSquareText } from "lucide-react";
import { buildSupportMailto, SUPPORT_EMAIL, SUPPORT_SLACK_URL } from "@/lib/support";

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
  const searchParams = useSearchParams();
  const topic = searchParams.get("topic") || undefined;
  const source = searchParams.get("source") || undefined;
  const workspace = searchParams.get("workspace") || undefined;
  const profile = searchParams.get("profile") || undefined;
  const error = searchParams.get("error") || undefined;

  const contextLines = useMemo(
    () => [
      topic ? `Topic: ${topic}` : undefined,
      source ? `Source: ${source}` : undefined,
      workspace ? `Workspace ID: ${workspace}` : undefined,
      profile ? `Profile: ${profile}` : undefined,
      error ? `Error: ${error}` : undefined,
    ],
    [error, profile, source, topic, workspace]
  );

  function openMail(subject: string) {
    window.location.href = buildSupportMailto({
      subject,
      intro: "I need help with the UniPost dashboard.",
      details: contextLines,
    });
  }

  return (
    <div style={{ maxWidth: 880, margin: "0 auto", paddingTop: 32 }}>
      <div style={{ textAlign: "center", marginBottom: 32 }}>
        <div style={{ fontSize: 13, fontWeight: 600, color: "var(--daccent)", fontFamily: "var(--font-geist-mono), monospace", marginBottom: 8 }}>
          Support Center
        </div>
        <h1 style={{ fontSize: 28, fontWeight: 800, letterSpacing: -0.5, color: "var(--dtext)", marginBottom: 10 }}>
          Get help fast
        </h1>
        <p style={{ fontSize: 14, color: "var(--dmuted)", lineHeight: 1.7 }}>
          Report a bug, request a feature, ask a question, or join the Slack community.
        </p>
      </div>

      {(topic || error) && (
        <div
          style={{
            marginBottom: 20,
            padding: "14px 16px",
            borderRadius: 12,
            border: "1px solid rgba(59,130,246,0.24)",
            background: "linear-gradient(180deg, rgba(59,130,246,0.12), rgba(59,130,246,0.06))",
          }}
        >
          <div style={{ fontSize: 13, fontWeight: 700, color: "var(--dtext)", marginBottom: 4 }}>
            We prefilled support context for you
          </div>
          <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>
            Your support email will include the page and error details from the action that brought you here.
          </div>
        </div>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(280px, 1fr))", gap: 16, alignItems: "start" }}>
        <div>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12 }}>
            {CONTACT_OPTIONS.slice(0, 2).map((opt) => {
              const Icon = opt.icon;
              return (
                <button
                  key={opt.title}
                  onClick={() => openMail(opt.subject)}
                  style={{
                    display: "flex", alignItems: "flex-start", gap: 12,
                    padding: "18px 20px", borderRadius: 12,
                    border: "1px solid var(--dborder2)", background: "var(--surface)",
                    cursor: "pointer", transition: "all 0.15s", textAlign: "left",
                    width: "100%", fontFamily: "inherit",
                  }}
                  onMouseEnter={(e) => { e.currentTarget.style.borderColor = "#333"; e.currentTarget.style.background = "var(--surface2)"; }}
                  onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = "var(--surface)"; }}
                >
                  <span style={{ fontSize: 20 }}>{opt.emoji}</span>
                  <span>
                    <span style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>
                      <Icon style={{ width: 15, height: 15 }} />
                      {opt.title}
                    </span>
                    <span style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>{opt.desc}</span>
                  </span>
                </button>
              );
            })}
          </div>
          <div style={{ marginTop: 12 }}>
            <button
              onClick={() => openMail(CONTACT_OPTIONS[2].subject)}
              style={{
                display: "flex", alignItems: "flex-start", gap: 12,
                padding: "18px 20px", borderRadius: 12,
                border: "1px solid var(--dborder2)", background: "var(--surface)",
                cursor: "pointer", transition: "all 0.15s", textAlign: "left",
                width: "100%", fontFamily: "inherit",
              }}
              onMouseEnter={(e) => { e.currentTarget.style.borderColor = "#333"; e.currentTarget.style.background = "var(--surface2)"; }}
              onMouseLeave={(e) => { e.currentTarget.style.borderColor = "var(--dborder2)"; e.currentTarget.style.background = "var(--surface)"; }}
            >
              <span style={{ fontSize: 20 }}>{CONTACT_OPTIONS[2].emoji}</span>
              <span>
                <span style={{ display: "flex", alignItems: "center", gap: 8, fontSize: 14, fontWeight: 600, color: "var(--dtext)", marginBottom: 6 }}>
                  <HelpCircle style={{ width: 15, height: 15 }} />
                  {CONTACT_OPTIONS[2].title}
                </span>
                <span style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6 }}>{CONTACT_OPTIONS[2].desc}</span>
              </span>
            </button>
          </div>
        </div>

        <div style={{ display: "grid", gap: 12 }}>
          <div
            style={{
              padding: "18px 20px", borderRadius: 12,
              border: "1px solid var(--dborder2)", background: "var(--surface)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
              <Mail style={{ width: 16, height: 16, color: "var(--daccent)" }} />
              <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)" }}>Email support</div>
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 12 }}>
              Best for failed actions, billing questions, account-specific issues, or anything that includes sensitive data.
            </div>
            <a
              href={buildSupportMailto({
                subject: topic ? `Help with ${topic}` : "Help Request",
                intro: "I need help with the UniPost dashboard.",
                details: contextLines,
              })}
              style={{
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                padding: "10px 12px",
                borderRadius: 10,
                background: "var(--daccent)",
                color: "var(--primary-foreground)",
                textDecoration: "none",
                fontSize: 13,
                fontWeight: 600,
              }}
            >
              Email {SUPPORT_EMAIL}
            </a>
          </div>

          <div
            style={{
              padding: "18px 20px", borderRadius: 12,
              border: "1px solid var(--dborder2)", background: "var(--surface)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 8 }}>
              <MessageSquareText style={{ width: 16, height: 16, color: "var(--daccent)" }} />
              <div style={{ fontSize: 14, fontWeight: 700, color: "var(--dtext)" }}>Slack community</div>
            </div>
            <div style={{ fontSize: 13, color: "var(--dmuted)", lineHeight: 1.6, marginBottom: 12 }}>
              Best for lightweight discussion, feature ideas, and seeing what other users are building with UniPost.
            </div>
            {SUPPORT_SLACK_URL ? (
              <a
                href={SUPPORT_SLACK_URL}
                target="_blank"
                rel="noopener noreferrer"
                style={{
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                  padding: "10px 12px",
                  borderRadius: 10,
                  border: "1px solid var(--dborder2)",
                  color: "var(--dtext)",
                  textDecoration: "none",
                  fontSize: 13,
                  fontWeight: 600,
                }}
              >
                Join Slack
              </a>
            ) : (
              <div style={{ fontSize: 12, color: "var(--dmuted)" }}>
                Set `NEXT_PUBLIC_SUPPORT_SLACK_URL` to enable the join button.
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
