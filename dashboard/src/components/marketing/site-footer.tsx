"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { UniPostLogo } from "@/components/brand/unipost-logo";

const FOOTER_CSS = `
.site-footer{width:100%;margin-top:auto;background:#000;border-top:1px solid rgba(255,255,255,.06)}
.site-footer-inner{max-width:1560px;margin:0 auto;padding:56px 32px 64px}
.site-footer-grid{display:grid;grid-template-columns:minmax(0,1.6fr) repeat(4,minmax(0,1fr));gap:48px}
.site-footer-brand{display:flex;flex-direction:column;gap:18px}
.site-footer-brand-copy{max-width:280px;font-size:15px;line-height:1.95;color:#a6a6a0}
.site-footer-title{font-size:12px;font-weight:700;letter-spacing:.1em;text-transform:uppercase;color:#6f6f69;margin-bottom:22px}
.site-footer-links{list-style:none;margin:0;padding:0}
.site-footer-link-item{margin-bottom:14px}
.site-footer-link{font-size:15px;line-height:1.75;color:#b8b8b2;text-decoration:none;transition:color .12s}
.site-footer-link:hover{color:#f7f7f5}
@media (max-width:1100px){.site-footer-grid{grid-template-columns:repeat(2,minmax(0,1fr));gap:40px 28px}}
@media (max-width:720px){.site-footer-inner{padding:44px 20px 52px}.site-footer-grid{grid-template-columns:1fr}.site-footer-brand-copy{max-width:none}}
`;

// Routes where the marketing footer must not render — every page in the
// (dashboard) route group belongs here, plus onboarding/admin flows.
const HIDDEN_PREFIXES = [
  "/projects",
  "/settings",
  "/workspace",
  "/account",
  "/api-keys",
  "/contact",
  "/admin",
  "/setup",
  "/welcome",
  "/connect",
  "/preview",
];

const PLATFORM_LINKS = [
  { label: "X", href: "/twitter-api" },
  { label: "Bluesky", href: "/bluesky-api" },
  { label: "LinkedIn", href: "/linkedin-api" },
  { label: "Instagram", href: "/instagram-api" },
  { label: "Threads", href: "/threads-api" },
  { label: "TikTok", href: "/tiktok-api" },
  { label: "YouTube", href: "/youtube-api" },
];

const COMPARE_LINKS = [
  { label: "vs Ayrshare", href: "/alternatives/ayrshare" },
  { label: "vs Zernio", href: "/alternatives/zernio" },
  { label: "vs PostForMe", href: "/alternatives/postforme" },
  { label: "All Comparisons →", href: "/compare" },
];

function FooterColumn({
  title,
  links,
}: {
  title: string;
  links: Array<{ label: string; href: string }>;
}) {
  return (
    <div>
      <div className="site-footer-title">{title}</div>
      <ul className="site-footer-links">
        {links.map((link) => (
          <li key={link.href} className="site-footer-link-item">
            <Link href={link.href} className="site-footer-link">
              {link.label}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export function SiteFooter() {
  return (
    <footer className="site-footer">
      <style dangerouslySetInnerHTML={{ __html: FOOTER_CSS }} />
      <div className="site-footer-inner">
        <div className="site-footer-grid">
          <div className="site-footer-brand">
            <Link href="/" aria-label="UniPost home" style={{ textDecoration: "none" }}>
              <UniPostLogo markSize={28} wordmarkColor="#f7f7f5" />
            </Link>
            <p className="site-footer-brand-copy">
              Unified social media API for developers.
              <br />
              Post to 7 platforms with one API call.
            </p>
          </div>

          <FooterColumn
            title="Product"
            links={[
              { label: "Overview", href: "/" },
              { label: "Pricing", href: "/pricing" },
              { label: "Docs", href: "/docs" },
            ]}
          />

          <FooterColumn title="Platforms" links={PLATFORM_LINKS} />
          <FooterColumn title="Compare" links={COMPARE_LINKS} />
          <FooterColumn
            title="Legal"
            links={[
              { label: "Privacy", href: "/privacy" },
              { label: "Terms", href: "/terms" },
            ]}
          />
        </div>
      </div>
    </footer>
  );
}

export function SiteFooterGate() {
  const pathname = usePathname();

  const shouldHide = HIDDEN_PREFIXES.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
  if (shouldHide) return null;

  return <SiteFooter />;
}
