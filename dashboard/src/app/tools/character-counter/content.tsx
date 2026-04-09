"use client";

import { useState } from "react";
import { PLATFORM_LIMITS } from "@/components/tools/platform-limits";
import { PlatformCard } from "@/components/tools/PlatformCard";

const CSS = `.cc-hero{padding:var(--section-py) 0 48px;max-width:880px}
.cc-hero-title{font-size:48px;font-weight:900;letter-spacing:-1.5px;line-height:1.1;color:var(--text);margin-bottom:16px}
.cc-hero-title em{color:var(--accent);font-style:normal}
.cc-hero-sub{font-size:16px;color:#bbb;line-height:1.6;margin-bottom:6px}
.cc-hero-free{font-size:12px;color:var(--muted2);font-family:var(--mono)}
.cc-tool{padding:0 0 var(--section-py)}
.cc-textarea{width:100%;min-height:160px;background:#0f0f0f;border:1px solid #1a1a1a;border-radius:12px;padding:20px;font-size:15px;line-height:1.6;color:#f0f0f0;font-family:var(--ui);resize:vertical;outline:none;transition:border-color .15s}
.cc-textarea:focus{border-color:#333}
.cc-textarea::placeholder{color:#444}
.cc-count-label{text-align:right;font-size:12px;color:var(--muted2);font-family:var(--mono);margin-top:8px;margin-bottom:24px}
.cc-grid{display:grid;grid-template-columns:repeat(4,1fr);gap:14px}
.cc-ref{padding:var(--section-py) 0}
.cc-ref-title{font-size:22px;font-weight:800;letter-spacing:-.3px;margin-bottom:20px;color:var(--text)}
.cc-ref-table{width:100%;border-collapse:collapse}
.cc-ref-table th{text-align:left;font-size:12px;color:var(--muted);font-weight:600;text-transform:uppercase;letter-spacing:.06em;padding:10px 12px;border-bottom:1px solid var(--b2)}
.cc-ref-table td{font-size:14px;color:#ccc;padding:12px;border-bottom:1px solid #111}
.cc-ref-table td:first-child{color:var(--text);font-weight:600}
.cc-ref-table td:nth-child(2){font-family:var(--mono);color:var(--accent)}
.cc-ref-table td:nth-child(3){color:var(--muted);font-size:13px}
.cc-tips{padding:0 0 var(--section-py)}
.cc-tips-title{font-size:22px;font-weight:800;letter-spacing:-.3px;margin-bottom:20px;color:var(--text)}
.cc-tip{background:#0f0f0f;border:1px solid #1a1a1a;border-radius:10px;padding:16px 20px;margin-bottom:10px}
.cc-tip-platform{font-size:13px;font-weight:700;color:var(--accent);margin-bottom:4px}
.cc-tip-text{font-size:13.5px;color:#b0b0b0;line-height:1.55}
@media(max-width:1024px){.cc-grid{grid-template-columns:repeat(2,1fr)}.cc-hero-title{font-size:38px}}
@media(max-width:680px){.cc-grid{grid-template-columns:1fr}.cc-hero-title{font-size:30px}}`;

const SCHEMA_ORG = {
  "@context": "https://schema.org",
  "@type": "WebApplication",
  name: "Social Media Character Counter",
  description: "Check post length for Twitter, LinkedIn, Instagram, Threads, TikTok, YouTube, and Bluesky",
  url: "https://unipost.dev/tools/character-counter",
  applicationCategory: "UtilitiesApplication",
  offers: { "@type": "Offer", price: "0", priceCurrency: "USD" },
  featureList: [
    "Twitter character counting with weighted method",
    "LinkedIn post length checker",
    "Instagram caption length checker",
    "Bluesky grapheme counting",
    "Real-time updates",
  ],
};

const TIPS = [
  { platform: "Twitter / X", text: "Keep it under 240 to leave room for reply threads. URLs count as 23 chars regardless of length. CJK characters count as 2." },
  { platform: "LinkedIn", text: "Posts with 1,200\u20131,500 characters tend to get the best engagement. Use line breaks generously \u2014 LinkedIn rewards whitespace." },
  { platform: "Instagram", text: "Only the first ~125 characters show in the feed before \u201Cmore\u201D. Front-load the hook." },
  { platform: "Bluesky", text: "300 graphemes, not bytes. Compound emoji count as 1. Similar vibe to Twitter but slightly more casual." },
  { platform: "Threads", text: "500 chars, similar to Twitter but with room to breathe. Conversational tone wins." },
  { platform: "TikTok", text: "2,200 chars for descriptions but most viewers read < 100. Short and punchy with hashtags." },
];

export function CharacterCounterContent() {
  const [text, setText] = useState("");

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: CSS }} />
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(SCHEMA_ORG) }}
      />

      <div className="tl-page">
        {/* Hero */}
        <div className="cc-hero">
          <h1 className="cc-hero-title">
            Social Media <em>Character Counter</em>
          </h1>
          <p className="cc-hero-sub">
            Check your post length for every platform at once.
          </p>
          <p className="cc-hero-free">No sign-up required &middot; Free forever</p>
        </div>

        {/* Tool */}
        <div className="cc-tool">
          <textarea
            className="cc-textarea"
            placeholder="Type or paste your post here..."
            value={text}
            onChange={(e) => setText(e.target.value)}
            autoFocus
          />
          <div className="cc-count-label">{text.length} characters</div>

          <div className="cc-grid">
            {PLATFORM_LIMITS.map((p) => (
              <PlatformCard key={p.platform} platform={p} text={text} />
            ))}
          </div>
        </div>

        {/* Reference table */}
        <div className="cc-ref">
          <h2 className="cc-ref-title">Platform Limits Reference</h2>
          <table className="cc-ref-table">
            <thead>
              <tr>
                <th>Platform</th>
                <th>Max Length</th>
                <th>Counting Method</th>
              </tr>
            </thead>
            <tbody>
              {PLATFORM_LIMITS.map((p) => (
                <tr key={p.platform}>
                  <td>{p.icon} {p.name}</td>
                  <td>{p.maxLength.toLocaleString()}</td>
                  <td>
                    {p.countingMethod === "twitter"
                      ? "Weighted (URLs = 23, CJK = 2)"
                      : p.countingMethod === "grapheme"
                        ? "Grapheme clusters (Intl.Segmenter)"
                        : "Standard (string.length)"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Tips */}
        <div className="cc-tips">
          <h2 className="cc-tips-title">Pro tips for platform-optimized content</h2>
          {TIPS.map((t) => (
            <div key={t.platform} className="cc-tip">
              <div className="cc-tip-platform">{t.platform}</div>
              <div className="cc-tip-text">{t.text}</div>
            </div>
          ))}
        </div>

        {/* CTA */}
        <div className="tl-cta">
          <div className="tl-cta-inner">
            <div className="tl-cta-glow" />
            <h2 className="tl-cta-title">Want to post to all these platforms with one API call?</h2>
            <p className="tl-cta-sub">
              UniPost handles the character limits, OAuth flows, and platform quirks for you.
            </p>
            <div className="tl-cta-actions">
              <a href="https://app.unipost.dev" className="lp-btn lp-btn-primary lp-btn-lg">
                Get UniPost API Key
              </a>
              <a href="/pricing" className="lp-btn lp-btn-outline lp-btn-lg">
                View Pricing
              </a>
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
