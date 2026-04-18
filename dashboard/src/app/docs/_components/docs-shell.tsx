"use client";

import { SignInButton, SignUpButton, UserButton, useAuth } from "@clerk/nextjs";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { UniPostMark } from "@/components/brand/unipost-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { CodeBlock, CodeTabs, codeBlockStyles, type CodeSnippet } from "./code-block";

type NavLeaf = {
  label: string;
  href: string;
  badge?: string;
};

type NavGroup = {
  title: string;
  items: NavLeaf[];
};

type NavSection = {
  title: string;
  description?: string;
  groups?: NavGroup[];
  items?: NavLeaf[];
};

type HeadingItem = {
  id: string;
  text: string;
  level: "h2" | "h3";
};

function renderDocsTableCell(cell: string) {
  const normalized = cell.trim().toLowerCase();

  if (normalized === "yes") {
    return (
      <span style={{ display: "inline-flex", alignItems: "center", color: "#22c55e", fontWeight: 700 }}>
        ✓
      </span>
    );
  }

  if (normalized === "no") {
    return (
      <span style={{ display: "inline-flex", alignItems: "center", color: "#ef4444", fontWeight: 700 }}>
        X
      </span>
    );
  }

  return cell;
}

const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";
const SIGN_UP_REDIRECT_URL = `${APP_URL}/`;

const userButtonAppearance = {
  elements: {
    avatarBox: "w-8 h-8",
    userButtonPopoverCard: {
      color: "#1f2937",
    },
  },
};

export const DOCS_NAV: NavSection[] = [
  {
    title: "Get Started",
    description: "Fastest path from API key to first publish.",
    items: [
      { label: "Quickstart", href: "/docs/quickstart" },
      { label: "SDKs", href: "/docs/sdk" },
      { label: "MCP", href: "/docs/mcp" },
      { label: "Pricing", href: "/docs/pricing" },
    ],
  },
  {
    title: "Platforms",
    description: "Per-platform support, constraints, and examples.",
    items: [
      { label: "Overview", href: "/docs/platforms" },
      { label: "Twitter/X", href: "/docs/platforms/twitter" },
      { label: "LinkedIn", href: "/docs/platforms/linkedin" },
      { label: "Instagram", href: "/docs/platforms/instagram" },
      { label: "Threads", href: "/docs/platforms/threads" },
      { label: "TikTok", href: "/docs/platforms/tiktok" },
      { label: "YouTube", href: "/docs/platforms/youtube" },
      { label: "Bluesky", href: "/docs/platforms/bluesky" },
    ],
  },
  {
    title: "API References",
    description: "Endpoint-level docs for auth, publishing, and analytics.",
    groups: [
      {
        title: "Core",
        items: [
          { label: "Overview", href: "/docs/api" },
          { label: "Authentication", href: "/docs/api/authentication" },
          { label: "Errors", href: "/docs/api/errors" },
        ],
      },
      {
        title: "Accounts",
        items: [
          { label: "Social Accounts", href: "/docs/api/accounts/list" },
          { label: "Connect Sessions", href: "/docs/api/connect/sessions" },
          { label: "Managed Users", href: "/docs/api/users" },
          { label: "Account Health", href: "/docs/api/accounts/health" },
        ],
      },
      {
        title: "Publishing",
        items: [
          { label: "Create Post", href: "/docs/api/posts/create" },
          { label: "Validate", href: "/docs/api/posts/validate" },
          { label: "Drafts", href: "/docs/api/posts/drafts" },
          { label: "Media", href: "/docs/api/media" },
        ],
      },
      {
        title: "Insights",
        items: [
          { label: "Analytics", href: "/docs/api/analytics" },
          { label: "Notifications", href: "/docs/api/notifications" },
          { label: "Slack Webhook URL", href: "/docs/api/slack-webhook" },
          { label: "Discord Webhook URL", href: "/docs/api/discord-webhook" },
          { label: "Webhooks", href: "/docs/api/webhooks" },
          { label: "Billing", href: "/docs/api/billing" },
        ],
      },
    ],
  },
];

const CSS = `
:root{
  --docs-ui: var(--font-inter), var(--font-geist-sans), system-ui, sans-serif;
  --docs-mono: var(--font-fira-code), var(--font-geist-mono), monospace;
  --docs-radius: 18px;
  --docs-radius-sm: 12px;
  --docs-shadow: 0 20px 56px rgba(15, 23, 42, 0.08);
  --docs-reading-width: 780px;
}
html.light{
  --docs-bg: #f7f8fb;
  --docs-bg-elevated: #ffffff;
  --docs-bg-muted: #f1f4f8;
  --docs-border: #e3e7ee;
  --docs-border-strong: #d5dbe5;
  --docs-text: #141b2d;
  --docs-text-soft: #465069;
  --docs-text-muted: #697489;
  --docs-text-faint: #8a94a8;
  --docs-nav-surface: #f3f5f9;
  --docs-nav-text: #4d5870;
  --docs-nav-text-strong: #243047;
  --docs-nav-text-faint: #778199;
  --docs-nav-hover: #e9edf4;
  --docs-nav-active-bg: #e2e9f3;
  --docs-nav-active-border: #cad5e4;
  --docs-link: #1264d6;
  --docs-link-hover: #0f56b8;
  --docs-accent: #1f7a4f;
  --docs-accent-soft: rgba(31, 122, 79, 0.08);
  --docs-shell-gradient: linear-gradient(180deg, #fcfdff 0%, #f7f8fb 34%, #f4f6fa 100%);
  --docs-topbar-bg: rgba(247, 248, 251, 0.88);
  --docs-card-shadow: 0 18px 46px rgba(15, 23, 42, 0.06);
  --docs-inline-code-bg: #f3f6fa;
  --docs-tech-bg: #2c2d39;
  --docs-tech-bg-2: #262833;
  --docs-tech-border: #313445;
  --docs-tech-text: #f8f8fb;
  --docs-tech-text-soft: #d6d9e5;
  --docs-tech-muted: #9aa0b5;
  --docs-tech-chip: rgba(255,255,255,.06);
  --docs-code-plain: #1e293b;
  --docs-code-comment: #8f96ad;
  --docs-code-string: #8fd4ff;
  --docs-code-keyword: #c9b0ff;
  --docs-code-number: #f2c170;
  --docs-code-function: #f6a76c;
  --docs-code-type: #7ae0b2;
  --docs-code-constant: #f39abb;
}
html.dark{
  --docs-bg: #0c1017;
  --docs-bg-elevated: #11161f;
  --docs-bg-muted: #161c27;
  --docs-border: #242c38;
  --docs-border-strong: #313b49;
  --docs-text: #edf2fb;
  --docs-text-soft: #c7d2e3;
  --docs-text-muted: #9aa7bc;
  --docs-text-faint: #738199;
  --docs-nav-surface: #121924;
  --docs-nav-text: #a7b3c7;
  --docs-nav-text-strong: #eef2fb;
  --docs-nav-text-faint: #7f8ca4;
  --docs-nav-hover: #1a2330;
  --docs-nav-active-bg: #212b38;
  --docs-nav-active-border: #344154;
  --docs-link: #7cb2ff;
  --docs-link-hover: #a8cbff;
  --docs-accent: #6dd39a;
  --docs-accent-soft: rgba(109, 211, 154, 0.11);
  --docs-shell-gradient: linear-gradient(180deg, #10141b 0%, #0c1017 36%, #0c1017 100%);
  --docs-topbar-bg: rgba(12, 16, 23, 0.82);
  --docs-card-shadow: 0 20px 60px rgba(0, 0, 0, 0.22);
  --docs-inline-code-bg: #161d28;
  --docs-tech-bg: #2c2d39;
  --docs-tech-bg-2: #262833;
  --docs-tech-border: #3a3d4f;
  --docs-tech-text: #f8f8fb;
  --docs-tech-text-soft: #d6d9e5;
  --docs-tech-muted: #9aa0b5;
  --docs-tech-chip: rgba(255,255,255,.06);
  --docs-code-plain: #d7dfec;
  --docs-code-comment: #7c8aa0;
  --docs-code-string: #7dc7ff;
  --docs-code-keyword: #d1a8ff;
  --docs-code-number: #f9b44d;
  --docs-code-function: #ff9857;
  --docs-code-type: #6dd39a;
  --docs-code-constant: #f08ab1;
}
*{box-sizing:border-box}
body{background:var(--docs-bg);color:var(--docs-text);font-family:var(--docs-ui);-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility}
.docs-shell{min-height:100vh;background:var(--docs-shell-gradient)}
.docs-topbar{position:sticky;top:0;z-index:50;border-bottom:1px solid color-mix(in srgb, var(--docs-border) 82%, transparent);background:var(--docs-topbar-bg);backdrop-filter:blur(16px)}
.docs-topbar-inner{max-width:1540px;margin:0 auto;padding:0 28px;min-height:68px;display:flex;align-items:center;justify-content:space-between;gap:20px}
.docs-brand{display:flex;align-items:center;gap:12px;text-decoration:none;color:inherit;min-width:0}
.docs-brand-mark{width:30px;height:30px;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.docs-brand-copy{display:flex;flex-direction:column;gap:3px;min-width:0}
.docs-brand-name{display:block;font-size:14px;font-weight:700;letter-spacing:-.015em;line-height:1.15}
.docs-brand-context{display:block;font-size:12px;line-height:1.45;color:var(--docs-text-muted)}
.docs-topbar-right{display:flex;align-items:center;gap:14px;justify-content:flex-end;flex-wrap:wrap}
.docs-topbar-links{display:flex;align-items:center;gap:6px;flex-wrap:wrap}
.docs-topbar-link{padding:7px 10px;border-radius:10px;font-size:13px;font-weight:500;color:var(--docs-text-muted);text-decoration:none;transition:all .12s}
.docs-topbar-link:hover{color:var(--docs-text);background:color-mix(in srgb, var(--docs-bg-muted) 86%, transparent)}
.docs-topbar-link.active{color:var(--docs-text);background:color-mix(in srgb, var(--docs-bg-muted) 100%, transparent)}
.docs-auth-actions{display:flex;align-items:center;gap:8px}
.docs-auth-btn{display:inline-flex;align-items:center;justify-content:center;padding:8px 13px;border-radius:10px;border:1px solid transparent;font-family:var(--docs-ui);font-size:13px;font-weight:600;line-height:1;text-decoration:none;cursor:pointer;transition:all .14s}
.docs-auth-btn.ghost{background:transparent;color:var(--docs-text-muted);border-color:var(--docs-border)}
.docs-auth-btn.ghost:hover{background:var(--docs-bg-muted);color:var(--docs-text);border-color:var(--docs-border-strong)}
.docs-auth-btn.primary{background:var(--docs-accent);color:#07140d;box-shadow:0 10px 22px rgba(16,185,129,.18)}
.docs-auth-btn.primary:hover{filter:brightness(1.04)}
.docs-layout{max-width:1540px;margin:0 auto;padding:32px 28px 88px;display:grid;grid-template-columns:252px minmax(0,var(--docs-reading-width)) 224px;justify-content:center;gap:34px}
.docs-sidebar,.docs-toc{position:sticky;top:96px;align-self:start;max-height:calc(100vh - 118px);overflow:auto;padding-bottom:16px}
.docs-sidebar-card,.docs-toc-card{background:var(--docs-nav-surface);border:1px solid var(--docs-border);border-radius:18px;padding:15px 14px;box-shadow:var(--docs-card-shadow)}
.docs-sidebar-section{padding:10px 0 2px;margin-bottom:14px}
.docs-sidebar-section:last-child{margin-bottom:0}
.docs-sidebar-section-header{padding:0 8px 10px;margin-bottom:4px;border-bottom:1px solid color-mix(in srgb, var(--docs-border) 86%, transparent)}
.docs-section-label{padding:0;font-size:11px;font-weight:750;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-nav-text-faint)}
.docs-section-desc{margin-top:7px;font-size:13px;line-height:1.58;color:var(--docs-nav-text-faint)}
.docs-nav-group-title{padding:12px 8px 6px;font-size:11px;font-weight:750;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-nav-text-faint)}
.docs-nav-link{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 8px;border-radius:10px;font-size:14.5px;font-weight:560;line-height:1.38;color:var(--docs-nav-text);text-decoration:none;transition:all .12s}
.docs-nav-link:hover{color:var(--docs-nav-text-strong);background:var(--docs-nav-hover)}
.docs-nav-link.active{color:var(--docs-nav-text-strong);font-weight:600;background:var(--docs-nav-active-bg);box-shadow:inset 0 0 0 1px var(--docs-nav-active-border)}
.docs-nav-badge{font-size:10px;font-family:var(--docs-mono);padding:2px 6px;border-radius:999px;background:color-mix(in srgb, var(--docs-bg-elevated) 78%, var(--docs-nav-surface));color:var(--docs-nav-text-faint)}
.docs-main{min-width:0}
.docs-page{background:color-mix(in srgb, var(--docs-bg-elevated) 98%, transparent);border:1px solid var(--docs-border);border-radius:24px;padding:48px 52px 56px;box-shadow:var(--docs-card-shadow)}
.docs-eyebrow{display:inline-flex;align-items:center;gap:8px;padding:6px 10px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);font-size:10.5px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:18px}
.docs-page h1{font-size:42px;line-height:1.04;letter-spacing:-.045em;font-weight:730;margin:0 0 14px;color:var(--docs-text);max-width:12ch}
.docs-lead{font-size:18px;line-height:1.72;color:var(--docs-text-soft);margin:0 0 34px;max-width:68ch}
.docs-page h2,.docs-page h3{scroll-margin-top:96px}
.docs-page h2{font-size:27px;line-height:1.18;letter-spacing:-.03em;font-weight:710;margin:42px 0 14px;color:var(--docs-text)}
.docs-page h3{font-size:20px;line-height:1.28;letter-spacing:-.02em;font-weight:680;margin:28px 0 12px;color:var(--docs-text)}
.docs-page p{font-size:16px;line-height:1.76;color:var(--docs-text-soft);margin:0 0 16px;max-width:66ch}
.docs-page a{color:var(--docs-link);text-decoration:none}
.docs-page a:hover{color:var(--docs-link-hover);text-decoration:underline;text-decoration-thickness:1.2px}
.docs-page strong{color:var(--docs-text)}
.docs-page :where(p,li,td,th,h1,h2,h3,h4,h5,h6,blockquote,strong,em,a) > code{font-family:var(--docs-mono);font-size:12.5px;background:var(--docs-inline-code-bg);border:1px solid var(--docs-border);padding:1px 6px;border-radius:8px;color:var(--docs-text)}
.docs-page hr{border:none;border-top:1px solid var(--docs-border);margin:30px 0}
.docs-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:16px;margin:22px 0}
.docs-card{background:var(--docs-bg-elevated);border:1px solid var(--docs-border);border-radius:16px;padding:20px;box-shadow:0 1px 0 rgba(255,255,255,.02)}
.docs-card h3{margin-top:0}
.docs-card-title{font-size:16px;font-weight:700;color:var(--docs-text);margin-bottom:8px;letter-spacing:-.015em}
.docs-card p{margin-bottom:0;font-size:14.5px;line-height:1.7;color:var(--docs-text-soft)}
.docs-list{margin:0 0 18px;padding-left:20px;color:var(--docs-text-soft);max-width:66ch}
.docs-list li{margin-bottom:10px;line-height:1.72}
.docs-table-wrap{overflow:auto;margin:20px 0}
.docs-table{width:100%;border-collapse:separate;border-spacing:0;min-width:620px;border:1px solid var(--docs-border);border-radius:16px;overflow:hidden;background:var(--docs-bg-elevated)}
.docs-table th{font-size:11px;font-weight:700;letter-spacing:.11em;text-transform:uppercase;color:var(--docs-text-faint);text-align:left;padding:13px 15px;background:var(--docs-bg-muted);border-bottom:1px solid var(--docs-border)}
.docs-table td{padding:15px;color:var(--docs-text-soft);font-size:14.5px;line-height:1.62;border-bottom:1px solid var(--docs-border);vertical-align:top}
.docs-table tr:last-child td{border-bottom:none}
.docs-callout{margin:22px 0;padding:16px 18px;border-radius:16px;background:var(--docs-accent-soft);border:1px solid color-mix(in srgb, var(--docs-accent) 18%, transparent);color:var(--docs-text-soft);max-width:66ch}
.docs-callout strong{color:var(--docs-text)}
.docs-toc-title{padding:6px 8px 10px;font-size:10.5px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-nav-text-faint)}
.docs-toc-link{display:block;padding:7px 8px;border-radius:10px;font-size:12.5px;line-height:1.45;color:var(--docs-nav-text);text-decoration:none;transition:all .12s}
.docs-toc-link:hover{color:var(--docs-nav-text-strong);background:var(--docs-nav-hover)}
.docs-toc-link.active{color:var(--docs-nav-text-strong);background:var(--docs-nav-active-bg);box-shadow:inset 0 0 0 1px var(--docs-nav-active-border)}
.docs-toc-link.level-h3{padding-left:18px;color:var(--docs-nav-text-faint)}
.docs-empty-toc{padding:8px;color:var(--docs-nav-text-faint);font-size:12.5px;line-height:1.6}
.docs-home-section{margin-top:46px}
.docs-home-section:first-of-type{margin-top:10px}
.docs-kicker{font-size:11px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:10px}
.docs-task-list{display:grid;grid-template-columns:1fr;gap:14px;margin:20px 0 6px}
.docs-task-item{display:grid;grid-template-columns:minmax(0,1fr) auto;gap:20px;align-items:start;padding:18px 20px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated);text-decoration:none;transition:border-color .12s,transform .12s,box-shadow .12s}
.docs-task-item:hover{border-color:color-mix(in srgb, var(--docs-link) 34%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow)}
.docs-task-copy{min-width:0}
.docs-task-title{font-size:17px;font-weight:700;letter-spacing:-.02em;color:var(--docs-text);margin-bottom:6px}
.docs-task-body{font-size:15px;line-height:1.72;color:var(--docs-text-soft);max-width:62ch}
.docs-task-links{display:flex;align-items:center;gap:10px;justify-content:flex-end;flex-wrap:wrap}
.docs-task-link{font-size:12px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-link)}
.docs-mini-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin:18px 0}
.docs-mini-card{padding:18px;border:1px solid var(--docs-border);border-radius:16px;background:var(--docs-bg-elevated)}
.docs-mini-title{font-size:15px;font-weight:700;letter-spacing:-.015em;color:var(--docs-text);margin-bottom:8px}
.docs-mini-card p{font-size:14.5px;line-height:1.68;color:var(--docs-text-soft);margin-bottom:8px}
.docs-step-list{margin:18px 0 20px;padding-left:20px;max-width:66ch}
.docs-step-list li{padding-left:4px;margin-bottom:10px;font-size:16px;line-height:1.72;color:var(--docs-text-soft)}
.docs-topbar .theme-picker{margin-right:2px}
.docs-topbar .theme-picker-trigger{height:35px;border-radius:10px}
@media (max-width:1240px){.docs-layout{grid-template-columns:252px minmax(0,var(--docs-reading-width));gap:26px}.docs-toc{display:none}}
@media (max-width:960px){.docs-topbar-inner{padding:10px 18px;align-items:flex-start;flex-direction:column}.docs-topbar-right{width:100%;align-items:flex-start;justify-content:flex-start;flex-direction:column}.docs-topbar-links{width:100%}.docs-layout{grid-template-columns:1fr;padding:22px 16px 60px}.docs-sidebar{display:none}.docs-page{padding:32px 24px 38px;border-radius:20px}.docs-page h1{font-size:34px;max-width:none}.docs-lead{font-size:17px}.docs-grid,.docs-mini-grid{grid-template-columns:1fr}.docs-task-item{grid-template-columns:1fr}}
`;

function isLeafActive(current: string, href: string) {
  return current === href.split("#")[0];
}

function isTopLevelActive(current: string, href: string) {
  if (href.includes("#")) {
    return current === href.split("#")[0];
  }
  if (href === "/docs") {
    return current === "/docs" || current.startsWith("/docs/");
  }
  return current === href || current.startsWith(`${href}/`);
}

function collectHeadingItems() {
  const seen = new Set<string>();
  const items: HeadingItem[] = [];

  const directHeadings = Array.from(
    document.querySelectorAll<HTMLElement>(".docs-main .docs-page h2[id], .docs-main .docs-page h3[id]")
  );

  directHeadings.forEach((node) => {
    const id = node.id;
    const text = node.textContent?.trim() || "";
    if (!id || !text || seen.has(id)) return;
    seen.add(id);
    items.push({
      id,
      text,
      level: node.tagName.toLowerCase() as "h2" | "h3",
    });
  });

  const sectionHeadings = Array.from(
    document.querySelectorAll<HTMLElement>(".docs-main section[id]")
  );

  sectionHeadings.forEach((section) => {
    const id = section.id;
    if (!id || seen.has(id)) return;

    const titleNode = section.querySelector<HTMLElement>("h2, h3");
    const text = titleNode?.textContent?.trim() || "";
    const level = (titleNode?.tagName.toLowerCase() as "h2" | "h3" | undefined) || "h3";
    if (!text) return;

    seen.add(id);
    items.push({ id, text, level });
  });

  return items;
}

function collectObservedNodes() {
  const nodes = [
    ...Array.from(document.querySelectorAll<HTMLElement>(".docs-main .docs-page h2[id], .docs-main .docs-page h3[id]")),
    ...Array.from(document.querySelectorAll<HTMLElement>(".docs-main section[id]")),
  ];

  const seen = new Set<string>();
  return nodes.filter((node) => {
    const id = node.id;
    if (!id || seen.has(id)) return false;
    seen.add(id);
    return true;
  });
}

export function DocsShell({ children }: { children: React.ReactNode }) {
  const pathname = usePathname();
  const { isLoaded, isSignedIn } = useAuth();
  const [headings, setHeadings] = useState<HeadingItem[]>([]);
  const [activeHeading, setActiveHeading] = useState("");

  useEffect(() => {
    let frame = 0;

    const syncHeadings = () => {
      const nextHeadings = collectHeadingItems();
      setHeadings(nextHeadings);
      setActiveHeading(nextHeadings[0]?.id || "");
    };

    frame = window.requestAnimationFrame(syncHeadings);
    return () => window.cancelAnimationFrame(frame);
  }, [pathname, children]);

  useEffect(() => {
    const headingNodes = collectObservedNodes();

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible[0]?.target instanceof HTMLElement) {
          setActiveHeading(visible[0].target.id);
        }
      },
      { rootMargin: "-100px 0px -60% 0px", threshold: [0.1, 1] }
    );

    headingNodes.forEach((node) => observer.observe(node));
    return () => observer.disconnect();
  }, [pathname, headings]);

  const topLinks = useMemo(
    () => [
      { label: "Docs", href: "/docs" },
      { label: "API", href: "/docs/api" },
      { label: "Pricing", href: "/pricing" },
    ],
    []
  );

  return (
    <div className="docs-shell">
      <style dangerouslySetInnerHTML={{ __html: `${CSS}\n${codeBlockStyles()}` }} />
      <header className="docs-topbar">
        <div className="docs-topbar-inner">
          <Link href="/docs" className="docs-brand">
            <span className="docs-brand-mark"><UniPostMark size={30} /></span>
            <span className="docs-brand-copy">
              <span className="docs-brand-name">UniPost Docs</span>
              <span className="docs-brand-context">Build social publishing, account onboarding, and analytics.</span>
            </span>
          </Link>
          <div className="docs-topbar-right">
            <nav className="docs-topbar-links">
              {topLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`docs-topbar-link${isTopLevelActive(pathname, link.href) ? " active" : ""}`}
                >
                  {link.label}
                </Link>
              ))}
            </nav>
            <ThemeToggle />
            {isLoaded ? (
              isSignedIn ? (
                <div className="docs-auth-actions">
                  <a href={APP_URL} className="docs-auth-btn primary">
                    Go to Dashboard
                  </a>
                  <UserButton appearance={userButtonAppearance} />
                </div>
              ) : (
                <div className="docs-auth-actions">
                  <SignInButton mode="redirect" forceRedirectUrl={APP_URL}>
                    <button type="button" className="docs-auth-btn ghost">
                      Sign in
                    </button>
                  </SignInButton>
                  <SignUpButton mode="redirect" forceRedirectUrl={SIGN_UP_REDIRECT_URL}>
                    <button type="button" className="docs-auth-btn primary">
                      Get Started Free
                    </button>
                  </SignUpButton>
                </div>
              )
            ) : (
              <div className="docs-auth-actions" style={{ minHeight: 36 }} />
            )}
          </div>
        </div>
      </header>

      <div className="docs-layout">
        <aside className="docs-sidebar">
          <div className="docs-sidebar-card">
            {DOCS_NAV.map((section) => (
              <section key={section.title} className="docs-sidebar-section">
                <div className="docs-sidebar-section-header">
                  <div className="docs-section-label">{section.title}</div>
                  {section.description ? <div className="docs-section-desc">{section.description}</div> : null}
                </div>
                {section.items?.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    className={`docs-nav-link${isLeafActive(pathname, item.href) ? " active" : ""}`}
                  >
                    <span>{item.label}</span>
                    {item.badge ? <span className="docs-nav-badge">{item.badge}</span> : null}
                  </Link>
                ))}
                {section.groups?.map((group) => (
                  <div key={group.title}>
                    <div className="docs-nav-group-title">{group.title}</div>
                    {group.items.map((item) => (
                      <Link
                        key={item.href}
                        href={item.href}
                        className={`docs-nav-link${isLeafActive(pathname, item.href) ? " active" : ""}`}
                      >
                        <span>{item.label}</span>
                        {item.badge ? <span className="docs-nav-badge">{item.badge}</span> : null}
                      </Link>
                    ))}
                  </div>
                ))}
              </section>
            ))}
          </div>
        </aside>

        <main className="docs-main">{children}</main>

        <aside className="docs-toc">
          <div className="docs-toc-card">
            <div className="docs-toc-title">On This Page</div>
            {headings.length === 0 ? (
              <div className="docs-empty-toc">This page is a navigation hub. Open a guide or reference page to see section links here.</div>
            ) : (
              headings.map((heading) => (
                <a
                  key={heading.id}
                  href={`#${heading.id}`}
                  className={`docs-toc-link level-${heading.level}${activeHeading === heading.id ? " active" : ""}`}
                >
                  {heading.text}
                </a>
              ))
            )}
          </div>
        </aside>
      </div>
    </div>
  );
}

export function DocsPage({
  eyebrow,
  title,
  lead,
  children,
}: {
  eyebrow?: string;
  title: string;
  lead: string;
  children: React.ReactNode;
}) {
  return (
    <article className="docs-page">
      {eyebrow ? <div className="docs-eyebrow">{eyebrow}</div> : null}
      <h1>{title}</h1>
      <p className="docs-lead">{lead}</p>
      {children}
    </article>
  );
}

export function DocsTable({
  columns,
  rows,
}: {
  columns: readonly string[];
  rows: readonly (readonly string[])[];
}) {
  return (
    <div className="docs-table-wrap">
      <table className="docs-table">
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={index}>
              {row.map((cell, cellIndex) => (
                <td key={cellIndex}>{renderDocsTableCell(cell)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function DocsCode({ code, language }: { code: string; language?: string }) {
  return <CodeBlock code={code} language={language} />;
}

export function DocsCodeTabs({
  snippets,
}: {
  snippets: CodeSnippet[];
}) {
  return <CodeTabs snippets={snippets} />;
}
