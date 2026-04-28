"use client";

import { PublicSiteHeader } from "@/components/marketing/nav";

// Shared CSS for all /tools/* pages. Each page adds its own
// tool-specific styles on top. The tl- prefix scopes these to the
// tools section to avoid collisions with the landing/pricing/solutions
// pages that use similar design tokens but different prefix namespaces.
const TOOLS_CSS = `
:root{--bg:var(--app-bg);--s1:var(--marketing-surface);--s2:var(--marketing-surface-alt);--s3:var(--marketing-surface-elevated);--border:var(--marketing-border);--b2:var(--marketing-border-strong);--b3:var(--marketing-border-strong);--text:var(--marketing-text);--muted:var(--marketing-muted);--muted2:var(--marketing-subtle);--accent:var(--primary);--blue:var(--marketing-link);--glow:var(--marketing-glow);--card-shadow:var(--marketing-shadow-soft);--r:8px;--mono:var(--font-fira-code),monospace;--ui:var(--font-dm-sans),system-ui,sans-serif;--content-max:1100px;--px:32px;--section-py:96px}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

/* Buttons */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}
.lp-btn-primary{background:var(--blue);color:#fff}
.lp-btn-primary:hover{background:var(--marketing-link-hover);box-shadow:var(--card-shadow)}
.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}
.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}
.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}
.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}
.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}
.lp-btn-accent{background:var(--accent);color:var(--marketing-auth-primary-text)}
.lp-btn-accent:hover{background:var(--marketing-auth-primary-hover);box-shadow:var(--card-shadow)}

/* Page container */
.tl-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}

/* Hero */
.tl-hero{padding:var(--section-py) 0 56px;max-width:880px}
.tl-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}
.tl-hero-title{font-size:52px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}
.tl-hero-title em{color:var(--accent);font-style:normal}
.tl-hero-sub{font-size:17px;color:#bbb;line-height:1.7;max-width:680px}

/* Tool card grid */
.tl-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;padding:0 0 var(--section-py)}
.tl-card{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:28px 26px;display:flex;flex-direction:column;gap:12px;transition:all .2s;text-decoration:none;color:inherit;min-height:220px;box-shadow:var(--card-shadow)}
.tl-card:hover{border-color:var(--accent);background:var(--s3);transform:translateY(-2px)}
.tl-card-icon{font-size:32px;line-height:1}
.tl-card-name{font-size:18px;font-weight:700;letter-spacing:-.3px;color:var(--text)}
.tl-card-badge{display:inline-flex;padding:3px 8px;border-radius:5px;font-size:10.5px;font-weight:700;text-transform:uppercase;letter-spacing:.06em;font-family:var(--mono);background:var(--success-soft);color:var(--accent);border:1px solid color-mix(in srgb,var(--accent) 20%,transparent)}
.tl-card-badge-muted{background:var(--s2);color:var(--muted2);border:1px solid var(--b2)}
.tl-card-desc{font-size:13.5px;color:var(--muted);line-height:1.6;flex:1}
.tl-card-cta{font-size:13px;font-weight:600;color:var(--blue);margin-top:auto}
.tl-card-soon{font-size:11px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.08em;font-family:var(--mono);margin-top:auto}

/* CTA section */
.tl-cta{padding:0 0 var(--section-py)}
.tl-cta-inner{background:var(--s3);border:1px solid var(--border);border-radius:16px;padding:56px 48px;text-align:center;position:relative;overflow:hidden;box-shadow:var(--card-shadow)}
.tl-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:var(--glow);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}
.tl-cta-title{font-size:32px;font-weight:800;letter-spacing:-.6px;margin-bottom:12px;position:relative}
.tl-cta-sub{font-size:14.5px;color:var(--muted);margin-bottom:28px;position:relative;max-width:520px;margin-left:auto;margin-right:auto}
.tl-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative;flex-wrap:wrap}

/* Footer */
.tl-footer{width:100%;border-top:1px solid var(--border);padding:32px 0;margin-top:32px}
.tl-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between;font-size:13px;color:var(--muted2)}
.tl-footer-inner a{color:var(--blue);text-decoration:none}
.tl-footer-inner a:hover{color:var(--marketing-link-hover);text-decoration:underline}

/* Responsive */
@media(max-width:1024px){.tl-grid{grid-template-columns:1fr 1fr}.tl-hero-title{font-size:40px}}
@media(max-width:680px){.tl-grid{grid-template-columns:1fr}.tl-hero-title{font-size:32px}.tl-cta-inner{padding:40px 24px}.tl-cta-title{font-size:26px}.tl-footer-inner{flex-direction:column;gap:8px;text-align:center}}`;

export default function ToolsLayout({ children }: { children: React.ReactNode }) {
  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: TOOLS_CSS }} />
      <PublicSiteHeader active="tools" />

      {children}
    </>
  );
}
