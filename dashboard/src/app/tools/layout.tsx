"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { MarketingNav } from "@/components/marketing/nav";

// Shared CSS for all /tools/* pages. Each page adds its own
// tool-specific styles on top. The tl- prefix scopes these to the
// tools section to avoid collisions with the landing/pricing/solutions
// pages that use similar design tokens but different prefix namespaces.
const TOOLS_CSS = `
:root{--bg:#000;--s1:#0a0a0a;--s2:#111;--s3:#1a1a1a;--border:#1a1a1a;--b2:#242424;--b3:#2e2e2e;--text:#f0f0f0;--muted:#999;--muted2:#555;--accent:#10b981;--blue:#0ea5e9;--r:8px;--mono:var(--font-fira-code),monospace;--ui:var(--font-dm-sans),system-ui,sans-serif;--nav-max:1480px;--content-max:1100px;--px:32px;--section-py:96px}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--bg);color:var(--text);font-family:var(--ui);font-size:15px;line-height:1.6;-webkit-font-smoothing:antialiased}

/* Nav */
.tl-nav{position:sticky;top:0;z-index:50;width:100%;border-bottom:1px solid var(--border);background:#00000095;backdrop-filter:blur(12px);-webkit-backdrop-filter:blur(12px)}
.tl-nav-inner{max-width:var(--nav-max);margin:0 auto;padding:0 var(--px);height:56px;display:flex;align-items:center;justify-content:space-between}
.tl-logo{display:flex;align-items:center;gap:10px;text-decoration:none}
.tl-logo-mark{width:28px;height:28px;background:var(--accent);border-radius:7px;display:flex;align-items:center;justify-content:center}
.tl-logo-mark svg{width:14px;height:14px;color:#000}
.tl-logo-name{font-size:16px;font-weight:700;letter-spacing:-.4px;color:var(--text)}
.tl-nav-links{display:flex;align-items:center;gap:4px}
.tl-nav-link{padding:6px 14px;font-size:14px;font-weight:500;color:var(--muted);border-radius:var(--r);transition:color .1s;text-decoration:none}
.tl-nav-link:hover{color:var(--text)}
.tl-nav-link.active{color:var(--text);font-weight:600}

/* Buttons */
.lp-btn{display:inline-flex;align-items:center;gap:6px;padding:8px 18px;border-radius:var(--r);font-size:13.5px;font-weight:600;cursor:pointer;transition:all .15s;border:1px solid transparent;font-family:var(--ui);text-decoration:none;white-space:nowrap}
.lp-btn-primary{background:var(--blue);color:#000}
.lp-btn-primary:hover{background:#38bdf8;box-shadow:0 0 24px #0ea5e930}
.lp-btn-ghost{background:transparent;color:var(--muted);border-color:var(--b2)}
.lp-btn-ghost:hover{background:var(--s2);color:var(--text);border-color:var(--b3)}
.lp-btn-outline{background:transparent;color:var(--text);border-color:var(--b2)}
.lp-btn-outline:hover{background:var(--s2);border-color:var(--b3)}
.lp-btn-lg{padding:12px 28px;font-size:15px;border-radius:10px}
.lp-btn-accent{background:var(--accent);color:#000}
.lp-btn-accent:hover{background:#34d399;box-shadow:0 0 24px #10b98130}

/* Page container */
.tl-page{max-width:var(--content-max);margin:0 auto;padding:0 var(--px)}

/* Hero */
.tl-hero{padding:var(--section-py) 0 56px;max-width:880px}
.tl-eyebrow{font-size:11.5px;color:var(--accent);text-transform:uppercase;letter-spacing:.12em;font-weight:700;margin-bottom:18px;font-family:var(--mono)}
.tl-hero-title{font-size:52px;font-weight:900;letter-spacing:-2px;line-height:1.05;color:var(--text);margin-bottom:24px}
.tl-hero-title em{color:var(--accent);font-style:normal}
.tl-hero-sub{font-size:17px;color:#aaa;line-height:1.7;max-width:680px}

/* Tool card grid */
.tl-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px;padding:0 0 var(--section-py)}
.tl-card{background:var(--s1);border:1px solid var(--b2);border-radius:14px;padding:28px 26px;display:flex;flex-direction:column;gap:12px;transition:all .2s;text-decoration:none;color:inherit;min-height:220px}
.tl-card:hover{border-color:#333;background:#0d0d0d;transform:translateY(-2px);box-shadow:0 8px 32px #00000040}
.tl-card-icon{font-size:32px;line-height:1}
.tl-card-name{font-size:18px;font-weight:700;letter-spacing:-.3px;color:var(--text)}
.tl-card-badge{display:inline-flex;padding:3px 8px;border-radius:5px;font-size:10.5px;font-weight:700;text-transform:uppercase;letter-spacing:.06em;font-family:var(--mono);background:#10b98118;color:var(--accent);border:1px solid #10b98130}
.tl-card-desc{font-size:13.5px;color:#999;line-height:1.6;flex:1}
.tl-card-cta{font-size:13px;font-weight:600;color:var(--blue);margin-top:auto}
.tl-card-soon{font-size:11px;font-weight:600;color:var(--muted2);text-transform:uppercase;letter-spacing:.08em;font-family:var(--mono);margin-top:auto}

/* CTA section */
.tl-cta{padding:0 0 var(--section-py)}
.tl-cta-inner{background:#0d0d0d;border:1px solid var(--border);border-radius:16px;padding:56px 48px;text-align:center;position:relative;overflow:hidden}
.tl-cta-glow{position:absolute;width:400px;height:400px;border-radius:50%;background:radial-gradient(circle,#10b98112,transparent 70%);top:50%;left:50%;transform:translate(-50%,-50%);pointer-events:none}
.tl-cta-title{font-size:32px;font-weight:800;letter-spacing:-.6px;margin-bottom:12px;position:relative}
.tl-cta-sub{font-size:14.5px;color:#aaa;margin-bottom:28px;position:relative;max-width:520px;margin-left:auto;margin-right:auto}
.tl-cta-actions{display:flex;align-items:center;justify-content:center;gap:12px;position:relative;flex-wrap:wrap}

/* Footer */
.tl-footer{width:100%;border-top:1px solid var(--border);padding:32px 0;margin-top:32px}
.tl-footer-inner{max-width:var(--content-max);margin:0 auto;padding:0 var(--px);display:flex;align-items:center;justify-content:space-between;font-size:13px;color:var(--muted2)}
.tl-footer-inner a{color:var(--blue);text-decoration:none}
.tl-footer-inner a:hover{text-decoration:underline}

/* Responsive */
@media(max-width:1024px){.tl-grid{grid-template-columns:1fr 1fr}.tl-hero-title{font-size:40px}}
@media(max-width:680px){.tl-grid{grid-template-columns:1fr}.tl-hero-title{font-size:32px}.tl-cta-inner{padding:40px 24px}.tl-cta-title{font-size:26px}}`;

function ZapIcon() {
  return (
    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" width="14" height="14">
      <path d="M9 2L4 9h4l-1 5 5-7H8l1-5z" />
    </svg>
  );
}

export default function ToolsLayout({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: TOOLS_CSS }} />

      <nav className="tl-nav">
        <div className="tl-nav-inner">
          <div style={{ display: "flex", alignItems: "center", gap: 32 }}>
            <Link href="/" className="tl-logo">
              <span className="tl-logo-mark"><ZapIcon /></span>
              <span className="tl-logo-name">UniPost</span>
            </Link>
            <div className="tl-nav-links">
              <Link href="/solutions" className={`tl-nav-link${pathname === "/solutions" ? " active" : ""}`}>Solutions</Link>
              <Link href="/tools" className={`tl-nav-link${pathname.startsWith("/tools") ? " active" : ""}`}>Tools</Link>
              <Link href="/pricing" className={`tl-nav-link${pathname === "/pricing" ? " active" : ""}`}>Pricing</Link>
              <Link href="/docs" className={`tl-nav-link${pathname === "/docs" ? " active" : ""}`}>Docs</Link>
            </div>
          </div>
          <MarketingNav />
        </div>
      </nav>

      {children}

      <footer className="tl-footer">
        <div className="tl-footer-inner">
          <span>&copy; {new Date().getFullYear()} UniPost</span>
          <div style={{ display: "flex", gap: 16 }}>
            <Link href="/tools">All Tools</Link>
            <a href="https://github.com/unipost-dev" target="_blank" rel="noopener noreferrer">GitHub</a>
          </div>
        </div>
      </footer>
    </>
  );
}
