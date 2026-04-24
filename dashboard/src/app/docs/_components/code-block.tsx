"use client";

import { useCallback, useMemo, useState } from "react";
import { Check, Copy } from "lucide-react";
import { JsonMonacoTabs, JsonMonacoViewer, MonacoCodeViewer, MonacoTabs } from "../api/_components/json-monaco-viewer";

export type CodeLanguage =
  | "javascript"
  | "python"
  | "go"
  | "json"
  | "bash"
  | "text";

export type CodeSnippet = {
  label: string;
  code: string;
  lang?: string;
};

function normalizeLanguage(value?: string, code?: string): CodeLanguage {
  const lower = (value || "").toLowerCase();

  if (lower.includes("javascript") || lower === "js" || lower === "node.js") return "javascript";
  if (lower.includes("python") || lower === "py") return "python";
  if (lower === "go" || lower.includes("golang")) return "go";
  if (lower === "json") return "json";
  if (lower === "bash" || lower === "shell" || lower === "sh" || lower === "curl" || lower === "curl") return "bash";

  const sample = code || "";
  if (/^\s*[{[]/.test(sample)) return "json";
  if (/^\s*(curl\b|npm\b|pnpm\b|yarn\b|pip\b|go get\b)/m.test(sample)) return "bash";
  if (/\bpackage main\b|\bfunc\s+\w+\s*\(/.test(sample)) return "go";
  if (/\bfrom\s+\w+\s+import\b|\bdef\s+\w+\s*\(|\bprint\s*\(/.test(sample)) return "python";
  if (/\bconst\b|\blet\b|\bawait\b|\bimport\s+[{*]/.test(sample)) return "javascript";
  return "text";
}

function formatJsonForCopy(code: string) {
  try {
    return JSON.stringify(JSON.parse(code), null, 2);
  } catch {
    return code;
  }
}

function CopyButton({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  const handleCopy = useCallback(async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }, [code]);

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="docs-copy-button"
      aria-label="Copy code to clipboard"
    >
      {copied ? <Check size={16} /> : <Copy size={16} />}
    </button>
  );
}

export function CodeBlock({
  code,
  language,
  title,
  compact = false,
  bare = false,
}: {
  code: string;
  language?: string;
  title?: string;
  compact?: boolean;
  bare?: boolean;
}) {
  const normalized = useMemo(() => normalizeLanguage(language, code), [code, language]);
  if (normalized === "json") {
    if (bare) {
      return <JsonMonacoViewer code={code} height={compact ? 220 : undefined} />;
    }

    return (
      <div className={`docs-code-block${compact ? " compact" : ""}`}>
        <div className="docs-code-toolbar">
          <div className="docs-code-meta">
            <span className="docs-code-lang">{title || normalized}</span>
          </div>
          <CopyButton code={formatJsonForCopy(code)} />
        </div>
        <JsonMonacoViewer code={code} />
      </div>
    );
  }

  if (bare) {
    return <MonacoCodeViewer code={code} language={normalized} height={compact ? 220 : undefined} />;
  }

  return (
    <div className={`docs-code-block${compact ? " compact" : ""}`}>
      <div className="docs-code-toolbar">
        <div className="docs-code-meta">
          <span className="docs-code-lang">{title || normalized}</span>
        </div>
        <CopyButton code={code} />
      </div>
      <MonacoCodeViewer code={code} language={normalized} height={compact ? 220 : undefined} />
    </div>
  );
}

export function CodeTabs({ snippets }: { snippets: CodeSnippet[] }) {
  const allJson = snippets.length > 0 && snippets.every((snippet) => normalizeLanguage(snippet.lang || snippet.label, snippet.code) === "json");

  if (allJson) {
    return <JsonMonacoTabs snippets={snippets} />;
  }
  return <MonacoTabs snippets={snippets} />;
}

export function codeBlockStyles() {
  return `
.docs-code-block,.docs-code-tabs{margin:20px 0;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);overflow:hidden;box-shadow:var(--docs-card-shadow);width:100%;min-width:0}
.docs-code-block.compact{margin:0}
.docs-code-toolbar,.docs-code-tabs-header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:12px 14px;background:var(--docs-bg-elevated);min-width:0}
.docs-code-meta{display:flex;align-items:center;gap:8px;min-width:0}
.docs-code-lang{font-size:11px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint);font-family:var(--docs-mono, var(--mono), monospace)}
.docs-copy-button{width:34px;height:34px;display:inline-flex;align-items:center;justify-content:center;border-radius:10px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated);color:var(--docs-text-muted);cursor:pointer;transition:all .12s;flex-shrink:0}
.docs-copy-button:hover{color:var(--docs-text);border-color:var(--docs-border-strong);background:var(--docs-bg-muted)}
.docs-copy-button svg{width:16px;height:16px}
.docs-code-surface{margin:0 14px 14px;padding:18px 20px;background:var(--docs-tech-bg);overflow:auto;border-radius:16px}
.docs-code-surface.tabs{border-radius:16px}
.docs-code-surface.bare{margin:0;padding:18px 20px;border-radius:16px}
.docs-code-content{display:block;white-space:pre;font-family:var(--docs-mono, var(--mono), monospace);font-size:13px;line-height:1.75;color:var(--docs-tech-text-soft)}
.docs-code-tab-list{display:flex;gap:6px;flex-wrap:wrap;min-width:0}
.docs-code-tab{padding:8px 12px;border-radius:10px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated);color:var(--docs-text-muted);font-size:12.5px;font-family:var(--docs-mono, var(--mono), monospace);cursor:pointer;transition:all .12s}
.docs-code-tab:hover{color:var(--docs-text);background:var(--docs-bg-muted)}
.docs-code-tab.active{color:var(--docs-tab-active-text);border-color:var(--docs-tab-active-border);background:var(--docs-tab-active-bg);box-shadow:var(--docs-tab-active-shadow)}
`;
}
