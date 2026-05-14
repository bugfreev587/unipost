"use client";

import { SignInButton, SignUpButton, UserButton, useAuth } from "@clerk/nextjs";
import { ChevronRight, Search } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from "react";
import { createPortal } from "react-dom";
import { UniPostMark } from "@/components/brand/unipost-logo";
import { ThemeToggle } from "@/components/theme-toggle";
import { ApiInlineLink } from "../api/_components/doc-components";
import { CodeBlock, CodeTabs, codeBlockStyles, type CodeSnippet } from "./code-block";

type NavLeaf = {
  label: string;
  href: string;
  badge?: string;
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
};

type NavGroup = {
  label: string;
  children: NavLeaf[];
};

type SidebarItem = NavLeaf | NavGroup;

type DocsPrimaryKey = "overview" | "platforms" | "api-reference" | "resources";

type DocsPrimaryNav = {
  key: DocsPrimaryKey;
  label: string;
  href: string;
};

type DocsSidebarSection = {
  title: string;
  description?: string;
  items: SidebarItem[];
};

type HeadingItem = {
  id: string;
  text: string;
  level: "h2" | "h3";
};

type DocsSearchResult = {
  title: string;
  href: string;
  primary: string;
  section?: string;
  group?: string;
  method?: NavLeaf["method"];
  keywords: string;
};

const API_SIDEBAR_DEFAULT_WIDTH = 336;
const API_SIDEBAR_MIN_WIDTH = 280;
const API_SIDEBAR_MAX_WIDTH = 520;
const API_SIDEBAR_STORAGE_KEY = "unipost-docs-api-sidebar-width";
const DOCS_USER_PATH_KEY = "unipost-docs-user-path";
const DOCS_USER_CHOOSER_HIDE_KEY = "unipost-docs-user-chooser-hide";

function clampApiSidebarWidth(value: number) {
  return Math.min(API_SIDEBAR_MAX_WIDTH, Math.max(API_SIDEBAR_MIN_WIDTH, Math.round(value)));
}

function renderDocsTableCell(cell: React.ReactNode) {
  if (typeof cell !== "string") {
    return cell;
  }

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

  return renderDocsRichContent(cell);
}

function isApiReference(value: string) {
  const trimmed = value.trim();
  return /^(GET|POST|PUT|PATCH|DELETE)\s+\/v1\/[A-Za-z0-9_/:?{}.-]+$/i.test(trimmed)
    || /^\/v1\/[A-Za-z0-9_/:?{}.-]+$/i.test(trimmed);
}

function renderInlineToken(token: string, key: string) {
  if (token.startsWith("`") && token.endsWith("`")) {
    const inner = token.slice(1, -1);
    if (isApiReference(inner)) {
      return <ApiInlineLink key={key} endpoint={inner} />;
    }
    return (
      <code
        key={key}
        style={{
          background: "var(--docs-inline-code-bg)",
          border: "1px solid var(--docs-border)",
          borderRadius: 8,
          padding: "2px 7px",
          fontFamily: "var(--docs-mono)",
          fontSize: "0.92em",
          color: "var(--docs-text-soft)",
        }}
      >
        {inner}
      </code>
    );
  }

  if (isApiReference(token)) {
    return <ApiInlineLink key={key} endpoint={token} />;
  }

  return token;
}

export function renderDocsRichContent(text: string) {
  const pattern = /`[^`]+`|(?:GET|POST|PUT|PATCH|DELETE)\s+\/v1\/[A-Za-z0-9_/:?{}.-]+|\/v1\/[A-Za-z0-9_/:?{}.-]+/g;
  const parts: Array<string | React.ReactNode> = [];
  let lastIndex = 0;
  let matchIndex = 0;

  for (const match of text.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > lastIndex) {
      parts.push(text.slice(lastIndex, index));
    }
    parts.push(renderInlineToken(match[0], `token-${matchIndex}`) as any);
    lastIndex = index + match[0].length;
    matchIndex += 1;
  }

  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  if (parts.length === 0) return text;
  if (parts.length === 1 && typeof parts[0] === "string") return parts[0];
  return <>{parts.map((part, index) => typeof part === "string" ? <span key={`text-${index}`}>{part}</span> : part)}</>;
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

const DOCS_PRIMARY_NAV: DocsPrimaryNav[] = [
  { key: "overview", label: "Overview", href: "/docs" },
  { key: "platforms", label: "Platforms", href: "/docs/platforms" },
  { key: "api-reference", label: "API Reference", href: "/docs/api" },
  { key: "resources", label: "Resources", href: "/docs/resources" },
];

const DOCS_SIDEBAR_NAV: Record<DocsPrimaryKey, DocsSidebarSection[]> = {
  overview: [
    {
      title: "Using the Dashboard",
      items: [
        { label: "Dashboard Quickstart", href: "/docs/dashboard-quickstart" },
      ],
    },
    {
      title: "Using the API",
      items: [
        { label: "API Quickstart", href: "/docs/quickstart" },
        { label: "SDKs", href: "/docs/sdk" },
        { label: "CLI", href: "/docs/cli" },
        { label: "MCP", href: "/docs/mcp" },
      ],
    },
    {
      title: "Advanced",
      items: [
        {
          label: "White-label",
          children: [
            { label: "Overview", href: "/docs/white-label" },
            { label: "Meta", href: "/docs/white-label/meta" },
            { label: "LinkedIn", href: "/docs/white-label/linkedin" },
            { label: "TikTok", href: "/docs/white-label/tiktok" },
            { label: "YouTube", href: "/docs/white-label/youtube" },
            { label: "X / Twitter", href: "/docs/white-label/twitter" },
          ],
        },
      ],
    },
  ],
  platforms: [
    {
      title: "Platforms",
      items: [
        { label: "Twitter/X", href: "/docs/platforms/twitter" },
        { label: "LinkedIn", href: "/docs/platforms/linkedin" },
        { label: "Instagram", href: "/docs/platforms/instagram" },
        { label: "Threads", href: "/docs/platforms/threads" },
        { label: "TikTok", href: "/docs/platforms/tiktok" },
        { label: "YouTube", href: "/docs/platforms/youtube" },
        { label: "Bluesky", href: "/docs/platforms/bluesky" },
        { label: "Facebook", href: "/docs/platforms/facebook", badge: "Beta" },
      ],
    },
  ],
  resources: [
    {
      title: "Resources",
      items: [
        {
          label: "Notifications",
          children: [
            { label: "Overview", href: "/docs/resources/notifications" },
            { label: "Slack Webhook URL", href: "/docs/resources/slack-webhook" },
            { label: "Discord Webhook URL", href: "/docs/resources/discord-webhook" },
          ],
        },
      ],
    },
  ],
  "api-reference": [
    {
      title: "Core",
      items: [
        {
          label: "profiles",
          children: [
            { label: "List profiles", href: "/docs/api/profiles/list", method: "GET" },
            { label: "Create profile", href: "/docs/api/profiles/create", method: "POST" },
            { label: "Get profile", href: "/docs/api/profiles/get", method: "GET" },
            { label: "Update profile", href: "/docs/api/profiles/update", method: "PATCH" },
            { label: "Delete profile", href: "/docs/api/profiles/delete", method: "DELETE" },
          ],
        },
        {
          label: "accounts",
          children: [
            { label: "List accounts", href: "/docs/api/accounts/list", method: "GET" },
            { label: "Connect account (credentials)", href: "/docs/api/accounts/connect", method: "POST" },
            { label: "Connect account (OAuth)", href: "/docs/api/accounts/oauth-connect", method: "POST" },
            { label: "Disconnect account", href: "/docs/api/accounts/disconnect", method: "DELETE" },
            { label: "Get account capabilities", href: "/docs/api/accounts/capabilities", method: "GET" },
            { label: "Check account health", href: "/docs/api/accounts/health", method: "GET" },
            { label: "Get account metrics", href: "/docs/api/accounts/metrics", method: "GET" },
            { label: "Get TikTok creator info", href: "/docs/api/accounts/tiktok-creator-info", method: "GET" },
          ],
        },
        {
          label: "connect",
          children: [
            { label: "Create session", href: "/docs/api/connect/sessions/create", method: "POST" },
            { label: "Get session", href: "/docs/api/connect/sessions/get", method: "GET" },
          ],
        },
        {
          label: "users",
          children: [
            { label: "List users", href: "/docs/api/users/list", method: "GET" },
            { label: "Get user", href: "/docs/api/users/get", method: "GET" },
          ],
        },
        {
          label: "api keys",
          children: [
            { label: "List API keys", href: "/docs/api/api-keys/list", method: "GET" },
            { label: "Create API key", href: "/docs/api/api-keys/create", method: "POST" },
            { label: "Revoke API key", href: "/docs/api/api-keys/delete", method: "DELETE" },
          ],
        },
      ],
    },
    {
      title: "Publishing",
      items: [
        {
          label: "posts",
          children: [
            { label: "Create post", href: "/docs/api/posts/create", method: "POST" },
            { label: "List posts", href: "/docs/api/posts/list", method: "GET" },
            { label: "Get post", href: "/docs/api/posts/get", method: "GET" },
            { label: "Update post", href: "/docs/api/posts/update", method: "PATCH" },
            { label: "Validate post", href: "/docs/api/posts/validate", method: "POST" },
          ],
        },
        {
          label: "drafts",
          children: [
            { label: "Create draft", href: "/docs/api/posts/drafts/create", method: "POST" },
            { label: "Publish draft", href: "/docs/api/posts/drafts/publish", method: "POST" },
          ],
        },
        {
          label: "media",
          children: [
            { label: "Reserve upload", href: "/docs/api/media/reserve", method: "POST" },
            { label: "Get media", href: "/docs/api/media/get", method: "GET" },
          ],
        },
      ],
    },
    {
      title: "Inbox",
      items: [
        { label: "Overview", href: "/docs/api/inbox" },
      ],
    },
    {
      title: "Analytics",
      items: [
        {
          label: "analytics",
          children: [
            { label: "Workspace summary", href: "/docs/api/analytics/summary", method: "GET" },
            { label: "Post analytics", href: "/docs/api/analytics/posts", method: "GET" },
          ],
        },
      ],
    },
    {
      title: "Developer Webhooks",
      items: [
        {
          label: "webhooks",
          children: [
            { label: "Overview", href: "/docs/api/webhooks" },
            { label: "Create webhook", href: "/docs/api/webhooks/create", method: "POST" },
            { label: "List webhooks", href: "/docs/api/webhooks/list", method: "GET" },
            { label: "Get webhook", href: "/docs/api/webhooks/get", method: "GET" },
            { label: "Update webhook", href: "/docs/api/webhooks/update", method: "PATCH" },
            { label: "Rotate secret", href: "/docs/api/webhooks/rotate", method: "POST" },
          ],
        },
      ],
    },
  ],
};

const DOCS_METHOD_COLORS: Record<NonNullable<NavLeaf["method"]>, string> = {
  GET: "#16a34a",
  POST: "#2563eb",
  PUT: "#d97706",
  PATCH: "#a855f7",
  DELETE: "#dc2626",
};

function buildDocsSearchIndex(): DocsSearchResult[] {
  const results = new Map<string, DocsSearchResult>();

  for (const primary of DOCS_PRIMARY_NAV) {
    results.set(primary.href, {
      title: primary.label,
      href: primary.href,
      primary: primary.label,
      keywords: `${primary.label} ${primary.href}`,
    });

    for (const section of DOCS_SIDEBAR_NAV[primary.key] || []) {
      for (const item of section.items) {
        if ("children" in item) {
          for (const child of item.children) {
            results.set(child.href, {
              title: child.label,
              href: child.href,
              primary: primary.label,
              section: section.title,
              group: item.label,
              method: child.method,
              keywords: [
                child.label,
                child.href,
                child.method,
                item.label,
                section.title,
                primary.label,
              ].filter(Boolean).join(" "),
            });
          }
        } else {
          results.set(item.href, {
            title: item.label,
            href: item.href,
            primary: primary.label,
            section: section.title,
            method: item.method,
            keywords: [
              item.label,
              item.href,
              item.method,
              section.title,
              primary.label,
            ].filter(Boolean).join(" "),
          });
        }
      }
    }
  }

  return Array.from(results.values());
}

const DOCS_SEARCH_INDEX = buildDocsSearchIndex();

function scoreDocsSearchResult(result: DocsSearchResult, query: string) {
  const q = query.trim().toLowerCase();
  if (!q) return 1;

  const title = result.title.toLowerCase();
  const href = result.href.toLowerCase();
  const keywords = result.keywords.toLowerCase();
  const parts = q.split(/\s+/).filter(Boolean);
  if (parts.length === 0) return 1;
  if (!parts.every((part) => keywords.includes(part))) return 0;

  let score = 10;
  if (title === q) score += 80;
  if (title.startsWith(q)) score += 52;
  if (title.includes(q)) score += 32;
  if (href.includes(q)) score += 22;
  if (result.method && result.method.toLowerCase() === q) score += 12;
  score += Math.max(0, 24 - title.length / 2);
  return score;
}

function DocsSearch() {
  const router = useRouter();
  const pathname = usePathname();
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const [mounted, setMounted] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const results = useMemo(() => {
    const trimmed = query.trim();
    if (!trimmed) {
      const current = DOCS_SEARCH_INDEX.find((result) => result.href === pathname);
      const suggested = DOCS_SEARCH_INDEX
        .map((result) => {
          let score = 0;
          if (result.href === pathname) score += 120;
          if (current?.group && result.group === current.group) score += 80;
          if (current?.section && result.section === current.section) score += 42;
          if (current?.primary && result.primary === current.primary) score += 18;
          if (result.href.includes("/webhooks")) score += pathname.includes("/posts/create") ? 8 : 0;
          return { result, score };
        })
        .filter((item) => item.score > 0)
        .sort((a, b) => b.score - a.score || a.result.title.localeCompare(b.result.title));

      return suggested.slice(0, 7).map((item) => item.result);
    }

    const scored = DOCS_SEARCH_INDEX
      .map((result) => ({ result, score: scoreDocsSearchResult(result, trimmed) }))
      .filter((item) => item.score > 0)
      .sort((a, b) => b.score - a.score || a.result.title.localeCompare(b.result.title));

    return scored.slice(0, 8).map((item) => item.result);
  }, [pathname, query]);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      const isSearchShortcut = (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k";
      if (!isSearchShortcut) return;
      event.preventDefault();
      setOpen(true);
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

  useEffect(() => {
    if (!open) return;
    setActiveIndex(0);
    window.setTimeout(() => inputRef.current?.focus(), 0);
  }, [open]);

  useEffect(() => {
    setActiveIndex(0);
  }, [query]);

  function closeSearch() {
    setOpen(false);
    setQuery("");
    setActiveIndex(0);
  }

  function selectResult(result: DocsSearchResult) {
    closeSearch();
    router.push(result.href);
  }

  function handleDialogKeyDown(event: ReactKeyboardEvent<HTMLDivElement>) {
    if (event.key === "Escape") {
      event.preventDefault();
      closeSearch();
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((value) => Math.min(results.length - 1, value + 1));
      return;
    }

    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((value) => Math.max(0, value - 1));
      return;
    }

    if (event.key === "Enter" && results[activeIndex]) {
      event.preventDefault();
      selectResult(results[activeIndex]);
    }
  }

  return (
    <>
      <button type="button" className="docs-search-trigger" onClick={() => setOpen(true)} aria-label="Search docs">
        <Search size={15} />
        <span>Search docs</span>
        <kbd>⌘K</kbd>
      </button>
      {open && mounted ? createPortal((
        <div className="docs-search-overlay" role="presentation" onMouseDown={closeSearch}>
          <div
            className="docs-search-dialog"
            role="dialog"
            aria-modal="true"
            aria-label="Search UniPost docs"
            onMouseDown={(event) => event.stopPropagation()}
            onKeyDown={handleDialogKeyDown}
          >
            <div className="docs-search-input-row">
              <Search size={18} />
              <input
                ref={inputRef}
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search docs..."
                aria-label="Search docs"
              />
              <kbd>Esc</kbd>
            </div>
            <div className="docs-search-section-label">{query.trim() ? "Results" : "Suggested"}</div>
            <div className="docs-search-results" role="listbox" aria-label="Search results">
              {results.length > 0 ? (
                results.map((result, index) => (
                  <button
                    key={result.href}
                    type="button"
                    className={`docs-search-result${index === activeIndex ? " active" : ""}`}
                    onMouseEnter={() => setActiveIndex(index)}
                    onClick={() => selectResult(result)}
                    role="option"
                    aria-selected={index === activeIndex}
                  >
                    <span className="docs-search-result-main">
                      <span className="docs-search-result-title">
                        {result.method ? (
                          <span
                            className="docs-search-method"
                            style={{ color: DOCS_METHOD_COLORS[result.method] }}
                          >
                            {result.method}
                          </span>
                        ) : null}
                        {result.title}
                      </span>
                      <span className="docs-search-result-meta">
                        {[result.primary, result.section, result.group].filter(Boolean).join(" / ")}
                      </span>
                    </span>
                    <span className="docs-search-result-path">{result.href}</span>
                  </button>
                ))
              ) : (
                <div className="docs-search-empty">No docs found for “{query.trim()}”.</div>
              )}
            </div>
          </div>
        </div>
      ), document.body) : null}
    </>
  );
}

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
  --docs-tech-bg: #eef2f7;
  --docs-tech-bg-2: #e4e9f1;
  --docs-tech-border: #d9dfea;
  --docs-tech-text: #1a2031;
  --docs-tech-text-soft: #2f3a52;
  --docs-tech-muted: #6b7588;
  --docs-tech-chip: rgba(17, 24, 39, 0.05);
  --docs-code-plain: #24292f;
  --docs-code-comment: #6e7781;
  --docs-code-string: #0a3069;
  --docs-code-keyword: #cf222e;
  --docs-code-number: #0550ae;
  --docs-code-function: #8250df;
  --docs-code-type: #116329;
  --docs-code-constant: #953800;
  --docs-tab-active-bg: #e8f1ff;
  --docs-tab-active-border: #9bbcf1;
  --docs-tab-active-text: #0f56b8;
  --docs-tab-active-shadow: inset 0 0 0 1px rgba(15, 86, 184, 0.12);
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
  --docs-tech-bg: #1f2736;
  --docs-tech-bg-2: #1a2231;
  --docs-tech-border: #2c3647;
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
  --docs-tab-active-bg: #223752;
  --docs-tab-active-border: #4e77b9;
  --docs-tab-active-text: #dce9ff;
  --docs-tab-active-shadow: inset 0 0 0 1px rgba(124, 178, 255, 0.18), 0 0 0 1px rgba(124, 178, 255, 0.08);
}
*{box-sizing:border-box}
body{background:var(--docs-bg);color:var(--docs-text);font-family:var(--docs-ui);-webkit-font-smoothing:antialiased;text-rendering:optimizeLegibility}
.docs-shell{min-height:100vh;background:var(--docs-shell-gradient)}
.docs-topbar{position:sticky;top:0;z-index:50;border-bottom:1px solid color-mix(in srgb, var(--docs-border) 82%, transparent);background:var(--docs-topbar-bg);backdrop-filter:blur(16px)}
.docs-topbar-inner{max-width:1540px;margin:0 auto;padding:0 28px;min-height:72px;display:flex;align-items:center;justify-content:space-between;gap:20px}
.docs-topbar-left{display:flex;align-items:center;gap:28px;min-width:0;flex-wrap:wrap}
.docs-brand{display:flex;align-items:center;gap:12px;text-decoration:none;color:inherit;min-width:0}
.docs-brand-mark{width:30px;height:30px;display:flex;align-items:center;justify-content:center;flex-shrink:0}
.docs-brand-name{display:block;font-size:15px;font-weight:760;letter-spacing:-.02em;line-height:1.1}
.docs-primary-nav{display:flex;align-items:center;gap:18px;min-width:0;flex-wrap:wrap}
.docs-primary-link{display:inline-flex;align-items:center;padding:12px 2px 14px;border-bottom:3px solid transparent;font-size:15px;font-weight:650;line-height:1;color:var(--docs-text-muted);text-decoration:none;transition:color .12s,border-color .12s}
.docs-primary-link:hover{color:var(--docs-text)}
.docs-primary-link.active{color:var(--docs-text);border-bottom-color:var(--docs-link)}
.docs-topbar-right{display:flex;align-items:center;gap:14px;justify-content:flex-end;flex-wrap:wrap}
.docs-dashboard-toplink{display:inline-flex;align-items:center;justify-content:center;height:36px;padding:0 12px;border-radius:10px;color:var(--docs-text);font-size:14px;font-weight:650;line-height:1;text-decoration:none;transition:background .14s ease,color .14s ease}
.docs-dashboard-toplink:hover{background:var(--docs-bg-muted);color:var(--docs-text)}
.docs-search-trigger{height:38px;display:inline-flex;align-items:center;gap:9px;padding:0 10px 0 12px;border:1px solid var(--docs-border-strong);border-radius:8px;background:color-mix(in srgb, var(--docs-bg-elevated) 92%, transparent);color:var(--docs-text-muted);font-family:var(--docs-ui);font-size:14px;font-weight:500;line-height:1;cursor:pointer;box-shadow:0 1px 0 rgba(15,23,42,.03);transition:border-color .14s ease,background .14s ease,color .14s ease,box-shadow .14s ease}
.docs-search-trigger:hover{background:var(--docs-bg-elevated);border-color:color-mix(in srgb, var(--docs-text-muted) 38%, var(--docs-border));color:var(--docs-text);box-shadow:0 8px 18px rgba(15,23,42,.06)}
.docs-search-trigger svg{color:var(--docs-text-faint)}
.docs-search-trigger kbd,.docs-search-input-row kbd{display:inline-flex;align-items:center;justify-content:center;min-width:32px;height:24px;padding:0 6px;border:1px solid var(--docs-border-strong);border-radius:6px;background:var(--docs-bg-muted);color:var(--docs-text-muted);font-family:var(--docs-ui);font-size:12px;font-weight:650;line-height:1;box-shadow:inset 0 -1px 0 rgba(15,23,42,.04)}
.docs-search-overlay{position:fixed;inset:0;z-index:90;display:flex;align-items:flex-start;justify-content:center;padding:92px 20px 24px;background:rgba(15,18,26,.16);backdrop-filter:blur(5px)}
.docs-search-dialog{width:min(610px,100%);overflow:hidden;border:1px solid color-mix(in srgb, var(--docs-border) 88%, transparent);border-radius:14px;background:var(--docs-bg-elevated);box-shadow:0 22px 62px rgba(15,23,42,.18)}
.docs-search-input-row{display:flex;align-items:center;gap:11px;padding:12px 13px;border-bottom:1px solid var(--docs-border);color:var(--docs-text-faint)}
.docs-search-input-row:focus-within{box-shadow:inset 0 0 0 1px color-mix(in srgb, #7aa7e8 42%, transparent)}
.docs-search-input-row input{width:100%;border:0!important;outline:0!important;box-shadow:none!important;background:transparent;color:var(--docs-text);font-family:var(--docs-ui);font-size:16px;font-weight:520;line-height:1.35;appearance:none}
.docs-search-input-row input::placeholder{color:var(--docs-text-faint)}
.docs-search-section-label{padding:9px 13px 2px;color:var(--docs-text-faint);font-size:11px;font-weight:760;letter-spacing:.08em;text-transform:uppercase}
.docs-search-results{max-height:min(392px,calc(100vh - 210px));overflow:auto;padding:5px 7px 8px;scrollbar-width:none}
.docs-search-results::-webkit-scrollbar{display:none}
.docs-search-result{width:100%;display:grid;grid-template-columns:minmax(0,1fr) auto;align-items:center;gap:14px;padding:10px 10px;border:0;border-radius:10px;background:transparent;color:inherit;text-align:left;cursor:pointer}
.docs-search-result:hover,.docs-search-result.active{background:var(--docs-bg-muted)}
.docs-search-result-main{display:grid;gap:5px;min-width:0}
.docs-search-result-title{display:flex;align-items:center;gap:9px;min-width:0;color:var(--docs-text);font-size:14.5px;font-weight:680;line-height:1.25}
.docs-search-method{font-family:var(--docs-mono);font-size:11px;font-weight:760;letter-spacing:.02em}
.docs-search-result-meta{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--docs-text-muted);font-size:12.5px;font-weight:500}
.docs-search-result-path{max-width:210px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--docs-text-faint);font-family:var(--docs-mono);font-size:12px}
.docs-search-empty{padding:34px 18px 38px;color:var(--docs-text-muted);font-size:14px;text-align:center}
.docs-auth-actions{display:flex;align-items:center;gap:8px}
.docs-auth-btn{display:inline-flex;align-items:center;justify-content:center;padding:8px 13px;border-radius:10px;border:1px solid transparent;font-family:var(--docs-ui);font-size:13px;font-weight:600;line-height:1;text-decoration:none;cursor:pointer;transition:all .14s}
.docs-auth-btn.ghost{background:transparent;color:var(--docs-text-muted);border-color:var(--docs-border)}
.docs-auth-btn.ghost:hover{background:var(--docs-bg-muted);color:var(--docs-text);border-color:var(--docs-border-strong)}
.docs-auth-btn.primary{background:var(--docs-accent);color:#07140d;box-shadow:0 10px 22px rgba(16,185,129,.18)}
.docs-auth-btn.primary:hover{filter:brightness(1.04)}
.docs-layout{max-width:1540px;margin:0 auto;padding:28px 28px 88px;display:grid;grid-template-columns:264px minmax(0,1fr) 224px;gap:30px}
.docs-layout-api{grid-template-columns:var(--docs-api-sidebar-width, 336px) 14px minmax(0,1fr);column-gap:0}
.docs-layout-platforms{grid-template-columns:228px minmax(0,1fr) 224px}
.docs-sidebar,.docs-toc{position:sticky;top:96px;align-self:start;max-height:calc(100vh - 118px);overflow:auto;padding-bottom:16px}
.docs-sidebar-card,.docs-toc-card{background:var(--docs-nav-surface);border:1px solid var(--docs-border);border-radius:18px;padding:15px 14px;box-shadow:var(--docs-card-shadow)}
.docs-sidebar-resizer{position:sticky;top:96px;align-self:start;height:calc(100vh - 118px);display:flex;align-items:stretch;justify-content:center}
.docs-sidebar-resizer::before{content:"";width:1px;background:color-mix(in srgb, var(--docs-border) 84%, transparent);border-radius:999px}
.docs-sidebar-resizer-handle{position:absolute;top:0;left:50%;transform:translateX(-50%);width:14px;height:100%;border:none;background:transparent;cursor:col-resize}
.docs-sidebar-resizer-handle::before{content:"";position:absolute;top:50%;left:50%;transform:translate(-50%,-50%);width:5px;height:54px;border-radius:999px;background:color-mix(in srgb, var(--docs-border-strong) 82%, transparent);transition:background .14s ease, box-shadow .14s ease, width .14s ease}
.docs-sidebar-resizer-handle:hover::before{background:color-mix(in srgb, var(--docs-link) 42%, var(--docs-border-strong));box-shadow:0 0 0 4px color-mix(in srgb, var(--docs-link) 10%, transparent)}
.docs-sidebar-resizer.dragging .docs-sidebar-resizer-handle::before{width:6px;background:color-mix(in srgb, var(--docs-link) 58%, var(--docs-border-strong));box-shadow:0 0 0 6px color-mix(in srgb, var(--docs-link) 12%, transparent)}
.docs-sidebar-section{padding:10px 0 2px;margin-bottom:14px}
.docs-sidebar-section:last-child{margin-bottom:0}
.docs-sidebar-section-header{padding:0 8px 10px;margin-bottom:4px;border-bottom:1px solid color-mix(in srgb, var(--docs-border) 86%, transparent)}
.docs-section-label{padding:0;font-size:11px;font-weight:750;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-nav-text-faint)}
.docs-section-desc{margin-top:7px;font-size:13px;line-height:1.58;color:var(--docs-nav-text-faint)}
.docs-nav-group-title{padding:12px 8px 6px;font-size:11px;font-weight:750;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-nav-text-faint)}
.docs-nav-subgroup{margin:4px 0 8px}
.docs-nav-subgroup>summary{list-style:none}
.docs-nav-subgroup>summary::-webkit-details-marker{display:none}
.docs-nav-subgroup-toggle{width:100%;display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 8px 6px;border:none;background:transparent;color:var(--docs-nav-text);font-size:15px;font-weight:560;line-height:1.35;text-align:left;cursor:pointer}
.docs-nav-subgroup-toggle:hover{color:var(--docs-nav-text-strong)}
.docs-nav-subgroup-chevron{width:18px;height:18px;color:var(--docs-nav-text-faint);flex-shrink:0;transition:transform .18s ease,color .18s ease;transform:rotate(0deg)}
.docs-nav-subgroup[open] .docs-nav-subgroup-chevron{transform:rotate(90deg);color:var(--docs-nav-text)}
.docs-nav-subgroup-toggle:hover .docs-nav-subgroup-chevron{color:var(--docs-nav-text-strong)}
.docs-nav-subgroup-items{margin-left:12px;padding-left:12px;border-left:1px solid color-mix(in srgb, var(--docs-border) 88%, transparent);display:grid;gap:2px}
.docs-nav-link{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:8px 8px;border-radius:10px;font-size:14.5px;font-weight:560;line-height:1.38;color:var(--docs-nav-text);text-decoration:none;transition:all .12s}
.docs-nav-link:hover{color:var(--docs-nav-text-strong);background:var(--docs-nav-hover)}
.docs-nav-link.active{color:var(--docs-nav-text-strong);font-weight:600;background:var(--docs-nav-active-bg);box-shadow:inset 0 0 0 1px var(--docs-nav-active-border)}
.docs-api-inline{position:relative;display:inline-flex;align-items:center;padding:2px 8px 3px;border-radius:10px;background:color-mix(in srgb, #2f7d4e 18%, var(--docs-inline-code-bg));border:1px solid color-mix(in srgb, #5ca772 24%, var(--docs-border));color:var(--docs-text);font-family:var(--docs-mono);font-size:.84em;font-weight:560;line-height:1.15;letter-spacing:.005em;text-decoration:none;vertical-align:baseline;overflow:hidden;transition:all .14s}
.docs-api-inline:hover{background:color-mix(in srgb, #2f7d4e 24%, var(--docs-inline-code-bg));border-color:color-mix(in srgb, #5ca772 40%, var(--docs-border));color:var(--docs-text);transform:translateY(-1px)}
.docs-api-inline.docs-api-inline-post{background:color-mix(in srgb, #2563eb 16%, var(--docs-inline-code-bg));border-color:color-mix(in srgb, #60a5fa 28%, var(--docs-border))}
.docs-api-inline.docs-api-inline-post:hover{background:color-mix(in srgb, #2563eb 22%, var(--docs-inline-code-bg));border-color:color-mix(in srgb, #60a5fa 42%, var(--docs-border))}
.docs-api-inline-static{cursor:default}
.docs-api-inline-glow{position:absolute;inset:0;background:linear-gradient(90deg,rgba(104,211,145,.2),transparent 62%);opacity:.34;pointer-events:none}
.docs-api-inline.docs-api-inline-post .docs-api-inline-glow{background:linear-gradient(90deg,rgba(96,165,250,.24),transparent 62%)}
.docs-api-inline-label{position:relative;z-index:1;display:inline-flex;align-items:center;gap:8px}
.docs-api-inline-method{color:#83d39e;font-weight:700;letter-spacing:.02em}
.docs-api-inline.docs-api-inline-post .docs-api-inline-method{color:#60a5fa}
.docs-api-inline-path{color:var(--docs-link)}
.docs-nav-badge{font-size:10px;font-family:var(--docs-mono);padding:2px 6px;border-radius:999px;background:color-mix(in srgb, var(--docs-bg-elevated) 78%, var(--docs-nav-surface));color:var(--docs-nav-text-faint)}
.docs-main{min-width:0}
.docs-main-api{max-width:none}
.docs-page{background:color-mix(in srgb, var(--docs-bg-elevated) 98%, transparent);border:1px solid var(--docs-border);border-radius:24px;padding:48px 52px 56px;box-shadow:var(--docs-card-shadow)}
.docs-page-api{padding:42px 46px 52px}
.docs-page.docs-page-wide h1,.docs-page.docs-page-wide .docs-lead,.docs-page.docs-page-wide p,.docs-page.docs-page-wide .docs-list,.docs-page.docs-page-wide .docs-step-list,.docs-page.docs-page-wide .docs-callout{max-width:none}
.docs-eyebrow{display:inline-flex;align-items:center;gap:8px;padding:6px 10px;border-radius:999px;background:var(--docs-bg-muted);border:1px solid var(--docs-border);font-size:10.5px;font-weight:700;letter-spacing:.12em;text-transform:uppercase;color:var(--docs-text-faint);margin-bottom:18px}
.docs-page h1{font-size:42px;line-height:1.04;letter-spacing:-.045em;font-weight:730;margin:0 0 14px;color:var(--docs-text);max-width:12ch}
.docs-page-api h1{max-width:none}
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
.docs-toc-link.level-h3{position:relative;margin-left:12px;padding-left:22px;color:var(--docs-nav-text-faint)}
.docs-toc-link.level-h3::before{content:"";position:absolute;left:8px;top:6px;bottom:6px;width:1px;background:var(--docs-border-strong);border-radius:999px}
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
.docs-checklist{list-style:none;padding:0;margin:10px 0 14px;display:grid;grid-template-columns:1fr;gap:4px}
.docs-checklist li{display:flex;align-items:baseline;gap:12px;font-size:14px;line-height:1.7;color:var(--docs-text-soft)}
.docs-checklist li::before{content:"";flex:none;width:6px;height:6px;border-radius:999px;background:color-mix(in srgb, var(--docs-link) 70%, var(--docs-border-strong));transform:translateY(-2px)}
.docs-checklist li code{font-family:var(--docs-mono);font-size:12.5px}
.docs-checklist.docs-checklist-2col{grid-template-columns:repeat(2,minmax(0,1fr));gap:6px 22px}
@media (max-width:960px){.docs-checklist.docs-checklist-2col{grid-template-columns:1fr}}
.docs-topbar .theme-picker{margin-right:2px}
.docs-topbar .theme-picker-trigger{height:35px;border-radius:10px}
.docs-shell-api-create-post{
  --docs-api-create-gutter:48px;
  --docs-api-sidebar-inner-left:var(--docs-api-create-gutter);
  --docs-api-nav-warm-bg:#ffffff;
  --docs-api-nav-warm-line:transparent;
  background:var(--docs-api-nav-warm-bg);
}
.docs-shell-api-create-post .docs-topbar{
  position:fixed;
  top:0;
  left:0;
  right:0;
  background:var(--docs-api-nav-warm-bg);
  border-bottom-color:transparent;
  backdrop-filter:blur(18px);
}
.docs-shell-api-create-post .docs-topbar-inner{
  max-width:none;
  min-height:78px;
  padding-left:var(--docs-api-sidebar-inner-left);
  padding-right:var(--docs-api-create-gutter);
  display:grid;
  grid-template-columns:minmax(190px, calc(var(--docs-api-sidebar-width, 336px) - 70px)) minmax(360px, 1fr) minmax(140px, auto);
  align-items:center;
  column-gap:36px;
}
.docs-shell-api-create-post .docs-topbar-left{
  display:contents;
}
.docs-shell-api-create-post .docs-brand{
  grid-column:1;
  justify-self:start;
}
.docs-shell-api-create-post .docs-primary-nav{
  grid-column:2;
  justify-self:start;
  gap:30px;
  flex-wrap:nowrap;
}
.docs-shell-api-create-post .docs-topbar-right{
  grid-column:3;
  justify-self:end;
  gap:12px;
  flex-wrap:nowrap;
}
.docs-shell-api-create-post .docs-brand-mark{
  width:28px;
  height:28px;
}
.docs-shell-api-create-post .docs-brand-name{
  font-size:17px;
  font-weight:720;
  letter-spacing:0;
}
.docs-shell-api-create-post .docs-primary-link{
  padding:18px 2px 18px;
  font-size:14px;
  font-weight:620;
  letter-spacing:0;
}
.docs-shell-api-create-post .docs-primary-link.active{
  border-bottom-color:#f04d23;
}
.docs-shell-api-create-post .docs-layout-api{
  max-width:none;
  margin:0;
  padding:78px var(--docs-api-create-gutter) 44px calc(var(--docs-api-sidebar-width, 336px) + var(--docs-api-create-gutter));
  background:transparent;
  display:block;
}
.docs-shell-api-create-post .docs-sidebar{
  position:fixed;
  top:78px;
  left:0;
  width:calc(var(--docs-api-sidebar-width, 336px) + var(--docs-api-create-gutter));
  height:calc(100vh - 78px);
  max-height:none;
  padding-top:22px;
  overflow:auto;
  z-index:20;
  scrollbar-gutter:stable;
  scrollbar-width:thin;
  scrollbar-color:transparent transparent;
  transition:scrollbar-color .16s ease;
}
.docs-shell-api-create-post .docs-sidebar:hover,
.docs-shell-api-create-post .docs-sidebar:focus-within{
  scrollbar-color:color-mix(in srgb, #b7c0ce 64%, transparent) transparent;
}
.docs-shell-api-create-post .docs-sidebar::-webkit-scrollbar{
  width:10px;
}
.docs-shell-api-create-post .docs-sidebar::-webkit-scrollbar-track{
  background:transparent;
}
.docs-shell-api-create-post .docs-sidebar::-webkit-scrollbar-thumb{
  background:transparent;
  border-radius:999px;
  border:3px solid transparent;
  background-clip:content-box;
}
.docs-shell-api-create-post .docs-sidebar:hover::-webkit-scrollbar-thumb,
.docs-shell-api-create-post .docs-sidebar:focus-within::-webkit-scrollbar-thumb{
  background:color-mix(in srgb, #b7c0ce 64%, transparent);
  background-clip:content-box;
}
.docs-shell-api-create-post .docs-sidebar-card{
  background:transparent;
  border:none;
  border-radius:0;
  box-shadow:none;
  padding:0 34px 0 var(--docs-api-sidebar-inner-left);
}
.docs-shell-api-create-post .docs-sidebar-section{
  padding:12px 0 2px;
  margin-bottom:14px;
}
.docs-shell-api-create-post .docs-sidebar-section-header{
  padding:0 8px 12px;
  margin-bottom:6px;
  border-bottom:none;
}
.docs-shell-api-create-post .docs-section-label{
  font-size:12px;
  font-weight:650;
  letter-spacing:0;
  text-transform:none;
}
.docs-shell-api-create-post .docs-nav-subgroup-toggle{
  padding:8px 8px 7px;
  font-size:15px;
  font-weight:500;
  letter-spacing:0;
}
.docs-shell-api-create-post .docs-nav-subgroup-items{
  margin-left:12px;
  padding-left:14px;
  border-left:1px solid color-mix(in srgb, var(--docs-api-nav-warm-line) 82%, #cdd6e2);
  gap:4px;
}
.docs-shell-api-create-post .docs-nav-link{
  border-radius:10px;
  padding:7px 10px;
  font-size:14px;
  font-weight:500;
  letter-spacing:0;
}
.docs-shell-api-create-post .docs-nav-link.active{
  background:color-mix(in srgb, #f04d23 8%, #f7f4f1);
  box-shadow:none;
}
.docs-shell-api-create-post .docs-sidebar-resizer{
  display:none;
}
.docs-shell-api-create-post .docs-main-api{
  border-left:none;
  padding-top:14px;
  padding-left:36px;
  padding-right:32px;
}
.docs-shell-api-create-post .docs-page.docs-page-api{
  background:transparent;
  border:none;
  border-radius:0;
  box-shadow:none;
  padding:20px 0 8px;
}
.docs-shell-api-create-post .docs-page-api .api-reference-page-header{
  width:100%;
  margin-bottom:46px;
  padding:4px 0 0;
  border-bottom-color:transparent!important;
}
.docs-shell-api-create-post .docs-page-api .api-reference-page-header h1{
  font-size:44px!important;
  line-height:1.04!important;
  letter-spacing:0!important;
  max-width:1120px;
}
.docs-shell-api-create-post .docs-page-api .api-reference-page-header > div:first-child{
  font-size:13px!important;
  margin-bottom:18px!important;
}
.docs-shell-api-create-post .docs-page-api .api-reference-page-header > div:last-child{
  max-width:min(1120px, 100%)!important;
  font-size:18px!important;
  line-height:1.62!important;
  text-wrap:pretty;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid{
  gap:32px;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right{
  box-sizing:border-box;
  top:88px;
  max-height:calc(100vh - 104px);
  overflow-y:auto;
  overscroll-behavior-y:auto;
  padding-right:6px;
  padding-bottom:6px;
  scrollbar-gutter:stable;
  scrollbar-width:thin;
  scrollbar-color:transparent transparent;
  transition:scrollbar-color .16s ease;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right:hover,
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right:focus-within{
  scrollbar-color:color-mix(in srgb, #b7c0ce 62%, transparent) transparent;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right::-webkit-scrollbar{
  width:10px;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right::-webkit-scrollbar-track{
  background:transparent;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right::-webkit-scrollbar-thumb{
  background:transparent;
  border-radius:999px;
  border:3px solid transparent;
  background-clip:content-box;
}
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right:hover::-webkit-scrollbar-thumb,
.docs-shell-api-create-post .docs-page-api .api-reference-grid-right:focus-within::-webkit-scrollbar-thumb{
  background:color-mix(in srgb, #b7c0ce 62%, transparent);
  background-clip:content-box;
}
.docs-shell-api-create-post .docs-page-api .api-reference-left-flow{
  gap:26px!important;
}
.docs-shell-api-create-post .docs-page-api .api-endpoint-summary{
  padding:4px 0 6px;
}
.docs-shell-api-create-post .docs-page-api .api-endpoint-card{
  border-radius:16px;
  box-shadow:none;
  border-color:color-mix(in srgb, var(--docs-border) 92%, transparent);
  background:color-mix(in srgb, var(--docs-bg-elevated) 96%, transparent);
}
.docs-shell-api-create-post .docs-page-api .api-field-sections{
  display:grid;
  gap:38px;
}
.docs-shell-api-create-post .docs-page-api .api-field-section{
  padding:0;
}
.docs-shell-api-create-post .docs-page-api .api-response-field-section{
  margin-top:2px;
}
.docs-shell-api-create-post .docs-page-api .api-reference-left-extra{
  margin-top:-4px;
}
.docs-shell-api-create-post .docs-page-api .api-field-section-title{
  font-size:25px;
  line-height:1.22;
  letter-spacing:0;
  font-weight:720;
  color:var(--docs-text);
  margin:0 0 20px;
}
.docs-shell-api-create-post .docs-page-api .api-field-list-items{
  gap:0!important;
}
.docs-shell-api-create-post .docs-page-api .api-field-row{
  padding:19px 0 20px;
  border-top:1px solid color-mix(in srgb, var(--docs-border) 84%, transparent);
}
.docs-shell-api-create-post .docs-page-api .api-field-row:first-child{
  border-top:none;
  padding-top:0;
}
.docs-shell-api-create-post .docs-page-api .api-field-row-heading{
  margin-bottom:9px!important;
}
.docs-shell-api-create-post .docs-page-api .api-field-name{
  color:#e6401a!important;
}
.docs-shell-api-create-post .docs-page-api .api-field-description{
  max-width:78ch;
  font-size:15.5px!important;
  line-height:1.68!important;
}
.docs-shell-api-create-post .docs-page-api .api-accordion{
  border-top:1px solid color-mix(in srgb, var(--docs-border) 84%, transparent);
}
.docs-shell-api-create-post .docs-page-api .api-accordion-summary{
  padding:16px 0!important;
  font-size:14px!important;
}
.docs-shell-api-create-post .docs-page-api .api-accordion-panel{
  padding:0 0 22px 28px!important;
}
.docs-shell-api-create-post .docs-page-api .api-endpoint-card code,
.docs-shell-api-create-post .docs-page-api .api-endpoint-card span{
  letter-spacing:0!important;
}
.docs-shell-api-create-post .docs-page-api .api-endpoint-card span[style*="#f04d23"],
.docs-shell-api-create-post .docs-page-api .api-endpoint-card span[style*="#ff3b1f"]{
  color:#d83a18!important;
}
.docs-shell-api-create-post .docs-page-api .docs-api-inline{
  padding:1px 7px 2px;
  border-radius:8px;
  background:color-mix(in srgb, #2563eb 10%, var(--docs-inline-code-bg));
  border-color:color-mix(in srgb, #60a5fa 20%, var(--docs-border));
}
.docs-shell-api-create-post .docs-page-api .docs-api-inline.docs-api-inline-post{
  background:color-mix(in srgb, #2563eb 11%, var(--docs-inline-code-bg));
  border-color:color-mix(in srgb, #60a5fa 22%, var(--docs-border));
}
.docs-shell-api-create-post .docs-page-api .docs-api-inline-method{
  color:#2563eb;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs{
  border-radius:8px;
  box-shadow:none;
  border-color:transparent;
  background:#272936!important;
  position:relative;
  overflow:hidden;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs-header{
  padding:14px 16px 0;
  background:#272936!important;
  align-items:flex-start;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-code-tab-list{
  padding-right:86px;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tab{
  border-radius:6px;
  padding:6px 10px;
  font-size:12px;
  border-color:rgba(255,255,255,.08);
  background:rgba(255,255,255,.05);
  color:#d4d4d8;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tab:hover{
  color:#ffffff;
  background:rgba(255,255,255,.08);
}
.docs-shell-api-create-post .docs-page-api .docs-code-tab.active{
  color:#ffffff;
  border-color:rgba(255,255,255,.16);
  background:rgba(255,255,255,.11);
  box-shadow:none;
}
.docs-shell-api-create-post .docs-page-api .docs-api-code-tabs .docs-code-tabs{
  background:#272936!important;
}
.docs-shell-api-create-post .docs-page-api .docs-api-code-tabs .docs-code-tabs > div:last-child{
  border:none!important;
  border-radius:0!important;
  background:#272936!important;
}
.docs-shell-api-create-post .docs-page-api .docs-monaco-frame{
  position:relative;
  border:none!important;
  border-radius:0!important;
  background:#272936!important;
}
.docs-shell-api-create-post .docs-page-api .docs-monaco-frame::after{
  content:"";
  position:absolute;
  top:0;
  right:0;
  bottom:0;
  width:28px;
  pointer-events:none;
  background:linear-gradient(90deg, transparent, #272936);
  opacity:.86;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-copy-button,
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-expand-button{
  position:absolute;
  top:12px;
  width:34px;
  height:34px;
  border-radius:6px;
  border-color:rgba(255,255,255,.12);
  background:rgba(255,255,255,.06);
  color:#f4f4f5;
  opacity:0;
  transform:translateY(-3px);
  transition:opacity .14s ease, transform .14s ease, background .14s ease;
  z-index:2;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-copy-button{
  right:54px;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-expand-button{
  right:12px;
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs:hover > .docs-code-tabs-header .docs-copy-button,
.docs-shell-api-create-post .docs-page-api .docs-code-tabs:focus-within > .docs-code-tabs-header .docs-copy-button,
.docs-shell-api-create-post .docs-page-api .docs-code-tabs:hover > .docs-code-tabs-header .docs-expand-button,
.docs-shell-api-create-post .docs-page-api .docs-code-tabs:focus-within > .docs-code-tabs-header .docs-expand-button{
  opacity:1;
  transform:translateY(0);
}
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-copy-button:hover,
.docs-shell-api-create-post .docs-page-api .docs-code-tabs > .docs-code-tabs-header .docs-expand-button:hover{
  background:rgba(255,255,255,.12);
  border-color:rgba(255,255,255,.2);
}
html.dark .docs-shell-api-create-post{
  --docs-api-nav-warm-bg:#18181b;
  background:#18181b;
}
html.dark .docs-shell-api-create-post .docs-topbar{
  background:#18181b;
  border-bottom-color:transparent;
}
html.dark .docs-shell-api-create-post .docs-main-api{
  border-left-color:transparent;
}
html.dark .docs-shell-api-create-post .docs-nav-link.active{
  background:rgba(240,77,35,.11);
}
html.dark .docs-shell-api-create-post .docs-page-api .docs-api-inline-method{
  color:#7cb2ff;
}
html.dark .docs-shell-api-create-post .docs-page-api .docs-api-code-tabs .docs-code-tabs > div:last-child{
  background:#272936!important;
}
@media (max-width:960px){
  .docs-shell-api-create-post{
    background:var(--docs-bg-elevated);
  }
  .docs-shell-api-create-post .docs-layout-api{
    padding:22px 16px 60px;
    display:grid;
  }
  .docs-shell-api-create-post .docs-topbar{
    position:sticky;
  }
  .docs-shell-api-create-post .docs-sidebar{
    position:sticky;
    width:auto;
    height:auto;
    overflow:auto;
  }
  .docs-shell-api-create-post .docs-main-api{
    padding-top:0;
    padding-left:0;
  }
  .docs-shell-api-create-post .docs-page.docs-page-api{
    padding:28px 0 34px;
  }
}
.docs-chooser-overlay{position:fixed;inset:0;z-index:70;display:flex;align-items:center;justify-content:center;padding:24px;background:rgba(10,14,20,.42);backdrop-filter:blur(12px)}
.docs-chooser-card{width:min(640px,100%);background:var(--docs-bg-elevated);border:1px solid var(--docs-border);border-radius:24px;box-shadow:var(--docs-shadow);padding:28px}
.docs-chooser-title{font-size:28px;line-height:1.1;letter-spacing:-.04em;font-weight:760;color:var(--docs-text);margin:0 0 10px}
.docs-chooser-sub{font-size:15px;line-height:1.72;color:var(--docs-text-soft);margin:0 0 20px}
.docs-chooser-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:14px;margin-bottom:18px}
.docs-chooser-option{display:flex;flex-direction:column;gap:8px;text-align:left;padding:18px 18px 16px;border-radius:18px;border:1px solid var(--docs-border);background:var(--docs-bg-muted);color:inherit;cursor:pointer;transition:border-color .14s ease,transform .14s ease,box-shadow .14s ease,background .14s ease}
.docs-chooser-option:hover{border-color:color-mix(in srgb, var(--docs-link) 34%, var(--docs-border));transform:translateY(-1px);box-shadow:var(--docs-card-shadow);background:color-mix(in srgb, var(--docs-bg-elevated) 84%, var(--docs-bg-muted))}
.docs-chooser-option-title{font-size:17px;font-weight:720;letter-spacing:-.02em;color:var(--docs-text)}
.docs-chooser-option-body{font-size:14px;line-height:1.68;color:var(--docs-text-soft)}
.docs-chooser-footer{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}
.docs-chooser-checkbox{display:inline-flex;align-items:center;gap:10px;font-size:13px;line-height:1.5;color:var(--docs-text-soft)}
.docs-chooser-checkbox input{width:15px;height:15px;accent-color:var(--docs-link)}
.docs-chooser-skip{border:none;background:transparent;color:var(--docs-link);font-size:13px;font-weight:700;cursor:pointer;padding:0}
.docs-chooser-skip:hover{text-decoration:underline}
@media (max-width:1240px){.docs-layout{grid-template-columns:252px minmax(0,1fr);gap:26px}.docs-toc{display:none}.docs-layout-api{grid-template-columns:var(--docs-api-sidebar-width, 312px) 14px minmax(0,1fr)}.docs-layout-platforms{grid-template-columns:220px minmax(0,1fr)}}
@media (max-width:960px){.docs-topbar-inner{padding:12px 18px;align-items:flex-start;flex-direction:column}.docs-topbar-left,.docs-topbar-right{width:100%}.docs-topbar-left{gap:14px}.docs-primary-nav{gap:14px;overflow:auto;flex-wrap:nowrap;padding-bottom:2px}.docs-topbar-right{align-items:flex-start;justify-content:flex-start;flex-direction:row}.docs-layout{grid-template-columns:1fr;padding:22px 16px 60px}.docs-sidebar,.docs-sidebar-resizer{display:none}.docs-page{padding:32px 24px 38px;border-radius:20px}.docs-page-api{padding:32px 24px 38px}.docs-page h1{font-size:34px;max-width:none}.docs-lead{font-size:17px}.docs-grid,.docs-mini-grid,.docs-chooser-grid{grid-template-columns:1fr}.docs-task-item{grid-template-columns:1fr}.docs-chooser-card{padding:22px 18px}}
`;

function isLeafActive(current: string, href: string) {
  return current === href.split("#")[0];
}

function isNavGroup(item: SidebarItem): item is NavGroup {
  return "children" in item;
}

function isNavGroupActive(current: string, item: NavGroup) {
  return item.children.some((child) => isLeafActive(current, child.href));
}

function getActivePrimaryNav(current: string): DocsPrimaryKey {
  if (current.startsWith("/docs/platforms")) return "platforms";
  if (current.startsWith("/docs/resources")) return "resources";
  if (current.startsWith("/docs/api")) return "api-reference";
  return "overview";
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
  const router = useRouter();
  const { isLoaded, isSignedIn } = useAuth();
  const [headings, setHeadings] = useState<HeadingItem[]>([]);
  const [activeHeading, setActiveHeading] = useState("");
  const [apiSidebarWidth, setApiSidebarWidth] = useState(API_SIDEBAR_DEFAULT_WIDTH);
  const [isDraggingSidebar, setIsDraggingSidebar] = useState(false);
  const [showUserChooser, setShowUserChooser] = useState(false);
  const [dontShowChooserAgain, setDontShowChooserAgain] = useState(false);
  const dragStartXRef = useRef(0);
  const dragStartWidthRef = useRef(API_SIDEBAR_DEFAULT_WIDTH);
  const activePrimaryNav = getActivePrimaryNav(pathname);
  const sidebarSections = DOCS_SIDEBAR_NAV[activePrimaryNav];
  const isApiPage = pathname.startsWith("/docs/api");
  const useApiReferenceRedesign = isApiPage;

  useEffect(() => {
    if (typeof window === "undefined") return;
    const storedWidth = window.localStorage.getItem(API_SIDEBAR_STORAGE_KEY);
    if (!storedWidth) return;
    const parsed = Number.parseInt(storedWidth, 10);
    if (Number.isFinite(parsed)) {
      setApiSidebarWidth(clampApiSidebarWidth(parsed));
    }
  }, []);

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
    if (headingNodes.length === 0) return;

    let frame = 0;

    const syncActiveHeading = () => {
      const activationOffset = 132;
      let nextActive = headingNodes[0]?.id || "";

      for (const node of headingNodes) {
        const top = node.getBoundingClientRect().top;
        if (top <= activationOffset) {
          nextActive = node.id;
        } else {
          break;
        }
      }

      setActiveHeading((prev) => (prev === nextActive ? prev : nextActive));
    };

    const scheduleSync = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(syncActiveHeading);
    };

    scheduleSync();
    window.addEventListener("scroll", scheduleSync, { passive: true });
    window.addEventListener("resize", scheduleSync);

    return () => {
      window.cancelAnimationFrame(frame);
      window.removeEventListener("scroll", scheduleSync);
      window.removeEventListener("resize", scheduleSync);
    };
  }, [pathname, headings]);

  useEffect(() => {
    if (!isDraggingSidebar) return;

    const handlePointerMove = (event: PointerEvent) => {
      const delta = event.clientX - dragStartXRef.current;
      const nextWidth = clampApiSidebarWidth(dragStartWidthRef.current + delta);
      setApiSidebarWidth(nextWidth);
    };

    const stopDragging = () => {
      setIsDraggingSidebar(false);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
    };

    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", stopDragging);

    return () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", stopDragging);
    };
  }, [isDraggingSidebar]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    window.localStorage.setItem(API_SIDEBAR_STORAGE_KEY, String(apiSidebarWidth));
  }, [apiSidebarWidth]);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (pathname !== "/docs") {
      setShowUserChooser(false);
      return;
    }

    const hideChooser = window.localStorage.getItem(DOCS_USER_CHOOSER_HIDE_KEY) === "true";
    const preferredPath = window.localStorage.getItem(DOCS_USER_PATH_KEY);

    if (hideChooser && preferredPath) {
      router.replace(preferredPath);
      return;
    }

    setDontShowChooserAgain(hideChooser);
    setShowUserChooser(true);
  }, [pathname, router]);

  const topLinks = useMemo(() => DOCS_PRIMARY_NAV, []);

  const handleChooseDocsPath = (href: string) => {
    if (typeof window !== "undefined") {
      window.localStorage.setItem(DOCS_USER_PATH_KEY, href);
      if (dontShowChooserAgain) {
        window.localStorage.setItem(DOCS_USER_CHOOSER_HIDE_KEY, "true");
      } else {
        window.localStorage.removeItem(DOCS_USER_CHOOSER_HIDE_KEY);
      }
    }
    setShowUserChooser(false);
    router.push(href);
  };

  const handleSkipChooser = () => {
    if (typeof window !== "undefined") {
      if (dontShowChooserAgain) {
        window.localStorage.setItem(DOCS_USER_CHOOSER_HIDE_KEY, "true");
      } else {
        window.localStorage.removeItem(DOCS_USER_CHOOSER_HIDE_KEY);
      }
    }
    setShowUserChooser(false);
  };

  return (
    <div className={`docs-shell${isApiPage ? " docs-shell-api" : ""}${useApiReferenceRedesign ? " docs-shell-api-create-post" : ""}`}>
      <style dangerouslySetInnerHTML={{ __html: `${CSS}\n${codeBlockStyles()}` }} />
      <header className="docs-topbar">
        <div className="docs-topbar-inner">
          <div className="docs-topbar-left">
            <Link href="/docs" className="docs-brand">
              <span className="docs-brand-mark"><UniPostMark size={30} /></span>
              <span className="docs-brand-name">UniPost</span>
            </Link>
            <nav className="docs-primary-nav">
              {topLinks.map((link) => (
                <Link
                  key={link.href}
                  href={link.href}
                  className={`docs-primary-link${activePrimaryNav === link.key ? " active" : ""}`}
                >
                  {link.label}
                </Link>
              ))}
            </nav>
          </div>
          <div className="docs-topbar-right">
            <a href={APP_URL} className="docs-dashboard-toplink">
              Dashboard
            </a>
            <ThemeToggle />
            <DocsSearch />
            {isLoaded ? (
              isSignedIn ? (
                <div className="docs-auth-actions">
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

      <div
        className={`docs-layout${isApiPage ? " docs-layout-api" : ""}${activePrimaryNav === "platforms" ? " docs-layout-platforms" : ""}`}
        style={isApiPage ? { ["--docs-api-sidebar-width" as any]: `${apiSidebarWidth}px` } : undefined}
      >
        <aside className="docs-sidebar">
          <div className="docs-sidebar-card">
            {sidebarSections.map((section) => (
              <section key={section.title} className="docs-sidebar-section">
                <div className="docs-sidebar-section-header">
                  <div className="docs-section-label">{section.title}</div>
                  {section.description ? <div className="docs-section-desc">{section.description}</div> : null}
                </div>
                {section.items.map((item) =>
                  isNavGroup(item) ? (
                    <details
                      key={item.label}
                      className="docs-nav-subgroup"
                      open={isNavGroupActive(pathname, item)}
                    >
                      <summary className="docs-nav-subgroup-toggle">
                        <span>{item.label}</span>
                        <ChevronRight className="docs-nav-subgroup-chevron" strokeWidth={2.2} />
                      </summary>
                      <div className="docs-nav-subgroup-items">
                        {item.children.map((child) => (
                          <Link
                            key={child.href}
                            href={child.href}
                            className={`docs-nav-link${isLeafActive(pathname, child.href) ? " active" : ""}`}
                          >
                            <span>{child.label}</span>
                            {child.method ? (
                              <span
                                style={{
                                  fontFamily: "var(--docs-mono)",
                                  fontSize: 12,
                                  fontWeight: 700,
                                  letterSpacing: ".04em",
                                  color: DOCS_METHOD_COLORS[child.method],
                                  flexShrink: 0,
                                }}
                              >
                                {child.method}
                              </span>
                            ) : child.badge ? <span className="docs-nav-badge">{child.badge}</span> : null}
                          </Link>
                        ))}
                      </div>
                    </details>
                  ) : (
                    <Link
                      key={item.href}
                      href={item.href}
                      className={`docs-nav-link${isLeafActive(pathname, item.href) ? " active" : ""}`}
                    >
                      <span>{item.label}</span>
                      {item.method ? (
                        <span
                          style={{
                            fontFamily: "var(--docs-mono)",
                            fontSize: 12,
                            fontWeight: 700,
                            letterSpacing: ".04em",
                            color: DOCS_METHOD_COLORS[item.method],
                            flexShrink: 0,
                          }}
                        >
                          {item.method}
                        </span>
                      ) : item.badge ? <span className="docs-nav-badge">{item.badge}</span> : null}
                    </Link>
                  )
                )}
              </section>
            ))}
          </div>
        </aside>

        {isApiPage ? (
          <div className={`docs-sidebar-resizer${isDraggingSidebar ? " dragging" : ""}`} aria-hidden="true">
            <button
              type="button"
              className="docs-sidebar-resizer-handle"
              aria-label="Resize API reference sidebar"
              onPointerDown={(event) => {
                if (event.pointerType === "mouse" || event.pointerType === "pen") {
                  event.preventDefault();
                }
                dragStartXRef.current = event.clientX;
                dragStartWidthRef.current = apiSidebarWidth;
                setIsDraggingSidebar(true);
              }}
            />
          </div>
        ) : null}

        <main className={`docs-main${isApiPage ? " docs-main-api" : ""}`}>{children}</main>

        {!isApiPage ? (
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
        ) : null}
      </div>
      {showUserChooser ? (
        <div className="docs-chooser-overlay" role="dialog" aria-modal="true" aria-labelledby="docs-user-chooser-title">
          <div className="docs-chooser-card">
            <h2 id="docs-user-chooser-title" className="docs-chooser-title">How do you want to use UniPost?</h2>
            <p className="docs-chooser-sub">
              Choose the onboarding path that matches how you want to publish first. You can still browse the full docs either way.
            </p>
            <div className="docs-chooser-grid">
              <button type="button" className="docs-chooser-option" onClick={() => handleChooseDocsPath("/docs/dashboard-quickstart")}>
                <span className="docs-chooser-option-title">Use the Dashboard</span>
                <span className="docs-chooser-option-body">Connect accounts and publish from the UniPost UI.</span>
              </button>
              <button type="button" className="docs-chooser-option" onClick={() => handleChooseDocsPath("/docs/quickstart")}>
                <span className="docs-chooser-option-title">Use the API</span>
                <span className="docs-chooser-option-body">Publish programmatically with API keys, SDKs, CLI, or MCP.</span>
              </button>
            </div>
            <div className="docs-chooser-footer">
              <label className="docs-chooser-checkbox">
                <input
                  type="checkbox"
                  checked={dontShowChooserAgain}
                  onChange={(event) => setDontShowChooserAgain(event.target.checked)}
                />
                <span>Don&apos;t show this again</span>
              </label>
              <button type="button" className="docs-chooser-skip" onClick={handleSkipChooser}>
                Skip for now
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

export function DocsPage({
  eyebrow,
  title,
  lead,
  children,
  className,
}: {
  eyebrow?: string;
  title: string;
  lead?: React.ReactNode;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <article className={`docs-page${className ? ` ${className}` : ""}`}>
      {eyebrow ? <div className="docs-eyebrow">{eyebrow}</div> : null}
      <h1>{title}</h1>
      {lead ? <p className="docs-lead">{lead}</p> : null}
      {children}
    </article>
  );
}

export function DocsTable({
  columns,
  rows,
}: {
  columns: readonly string[];
  rows: readonly (readonly React.ReactNode[])[];
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

export function DocsRichText({ text }: { text: string }) {
  return <>{renderDocsRichContent(text)}</>;
}

export function DocsCodeTabs({
  snippets,
}: {
  snippets: CodeSnippet[];
}) {
  return <CodeTabs snippets={snippets} />;
}
