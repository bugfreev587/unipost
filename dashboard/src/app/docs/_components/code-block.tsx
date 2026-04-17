"use client";

import { useCallback, useMemo, useState } from "react";

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

type TokenKind =
  | "plain"
  | "comment"
  | "string"
  | "keyword"
  | "number"
  | "function"
  | "type"
  | "constant";

type Token = {
  kind: TokenKind;
  value: string;
};

const KEYWORDS: Record<CodeLanguage, string[]> = {
  javascript: [
    "async",
    "await",
    "break",
    "case",
    "catch",
    "class",
    "const",
    "continue",
    "default",
    "else",
    "export",
    "extends",
    "finally",
    "for",
    "from",
    "function",
    "if",
    "import",
    "in",
    "instanceof",
    "let",
    "new",
    "of",
    "return",
    "switch",
    "throw",
    "try",
    "while",
    "yield",
  ],
  python: [
    "and",
    "as",
    "class",
    "def",
    "elif",
    "else",
    "except",
    "False",
    "finally",
    "for",
    "from",
    "if",
    "import",
    "in",
    "is",
    "None",
    "or",
    "pass",
    "raise",
    "return",
    "True",
    "try",
    "while",
    "with",
    "yield",
  ],
  go: [
    "break",
    "case",
    "const",
    "continue",
    "default",
    "defer",
    "else",
    "fallthrough",
    "for",
    "func",
    "go",
    "if",
    "import",
    "interface",
    "map",
    "package",
    "range",
    "return",
    "select",
    "struct",
    "switch",
    "type",
    "var",
  ],
  json: ["false", "null", "true"],
  bash: [
    "case",
    "do",
    "done",
    "elif",
    "else",
    "esac",
    "fi",
    "for",
    "function",
    "if",
    "in",
    "then",
    "while",
  ],
  text: [],
};

const TYPE_WORDS = new Set([
  "boolean",
  "dict",
  "float",
  "int",
  "interface",
  "list",
  "map",
  "number",
  "object",
  "string",
  "struct",
]);

const TOKEN_COLORS: Record<TokenKind, string> = {
  plain: "var(--docs-code-plain)",
  comment: "var(--docs-code-comment)",
  string: "var(--docs-code-string)",
  keyword: "var(--docs-code-keyword)",
  number: "var(--docs-code-number)",
  function: "var(--docs-code-function)",
  type: "var(--docs-code-type)",
  constant: "var(--docs-code-constant)",
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

function tokenize(code: string, language: CodeLanguage): Token[] {
  if (language === "text") {
    return [{ kind: "plain", value: code }];
  }

  const keywordPattern = KEYWORDS[language].length
    ? `\\b(?:${KEYWORDS[language].join("|")})\\b`
    : "(?!)";

  const commentPattern =
    language === "python" || language === "bash"
      ? "#.*$"
      : language === "json"
        ? "(?!)"
        : "//.*$";

  const stringPattern =
    language === "bash"
      ? `"(?:\\\\.|[^"\\\\])*"|'(?:\\\\.|[^'\\\\])*'`
      : "\"(?:\\\\.|[^\"\\\\])*\"|'(?:\\\\.|[^'\\\\])*'|`(?:\\\\.|[^`\\\\])*`";

  const regex = new RegExp(
    [
      `(?<comment>${commentPattern})`,
      `(?<string>${stringPattern})`,
      `(?<keyword>${keywordPattern})`,
      `(?<number>\\b\\d+(?:\\.\\d+)?\\b)`,
      `(?<function>\\b[A-Za-z_][\\w]*\\s*(?=\\())`,
      `(?<constant>\\b[A-Z][A-Z0-9_]{2,}\\b)`,
      `(?<type>\\b(?:${Array.from(TYPE_WORDS).join("|")})\\b)`,
    ].join("|"),
    "gm"
  );

  const tokens: Token[] = [];
  let lastIndex = 0;

  for (const match of code.matchAll(regex)) {
    const index = match.index ?? 0;
    if (index > lastIndex) {
      tokens.push({ kind: "plain", value: code.slice(lastIndex, index) });
    }

    const groups = match.groups || {};
    const kind = (Object.keys(groups).find((key) => groups[key]) as TokenKind | undefined) || "plain";
    tokens.push({ kind, value: match[0] });
    lastIndex = index + match[0].length;
  }

  if (lastIndex < code.length) {
    tokens.push({ kind: "plain", value: code.slice(lastIndex) });
  }

  return tokens;
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
      {copied ? "Copied" : "Copy"}
    </button>
  );
}

export function CodeBlock({
  code,
  language,
  title,
  compact = false,
}: {
  code: string;
  language?: string;
  title?: string;
  compact?: boolean;
}) {
  const normalized = useMemo(() => normalizeLanguage(language, code), [code, language]);
  const tokens = useMemo(() => tokenize(code, normalized), [code, normalized]);

  return (
    <div className={`docs-code-block${compact ? " compact" : ""}`}>
      <div className="docs-code-toolbar">
        <div className="docs-code-meta">
          <span className="docs-code-lang">{title || normalized}</span>
        </div>
        <CopyButton code={code} />
      </div>
      <pre className="docs-code-surface">
        <code className={`docs-code-content language-${normalized}`}>
          {tokens.map((token, index) => (
            <span
              key={`${token.kind}-${index}`}
              style={{ color: TOKEN_COLORS[token.kind] }}
            >
              {token.value}
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

export function CodeTabs({ snippets }: { snippets: CodeSnippet[] }) {
  const [active, setActive] = useState(0);
  const current = snippets[active];

  return (
    <div className="docs-code-tabs">
      <div className="docs-code-tabs-header">
        <div className="docs-code-tab-list">
          {snippets.map((snippet, index) => (
            <button
              key={`${snippet.label}-${index}`}
              type="button"
              onClick={() => setActive(index)}
              className={`docs-code-tab${index === active ? " active" : ""}`}
            >
              {snippet.label}
            </button>
          ))}
        </div>
        <CopyButton code={current.code} />
      </div>
      <pre className="docs-code-surface tabs">
        <code className={`docs-code-content language-${normalizeLanguage(current.lang || current.label, current.code)}`}>
          {tokenize(current.code, normalizeLanguage(current.lang || current.label, current.code)).map((token, index) => (
            <span
              key={`${token.kind}-${index}`}
              style={{ color: TOKEN_COLORS[token.kind] }}
            >
              {token.value}
            </span>
          ))}
        </code>
      </pre>
    </div>
  );
}

export function codeBlockStyles() {
  return `
.docs-code-block,.docs-code-tabs{margin:20px 0;border:1px solid var(--docs-border);border-radius:18px;background:var(--docs-bg-elevated);overflow:hidden;box-shadow:0 1px 0 rgba(255,255,255,.02)}
.docs-code-block.compact{margin:0}
.docs-code-toolbar,.docs-code-tabs-header{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:10px 12px;border-bottom:1px solid var(--docs-border);background:var(--docs-bg-muted)}
.docs-code-meta{display:flex;align-items:center;gap:8px;min-width:0}
.docs-code-lang{font-size:11px;font-weight:700;letter-spacing:.08em;text-transform:uppercase;color:var(--docs-text-faint);font-family:var(--docs-mono, var(--mono), monospace)}
.docs-copy-button{padding:6px 10px;border-radius:9px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated);color:var(--docs-text-muted);font-size:11px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;font-family:var(--docs-mono, var(--mono), monospace);cursor:pointer;transition:all .12s}
.docs-copy-button:hover{color:var(--docs-text);background:var(--docs-bg-muted)}
.docs-code-surface{margin:0;padding:18px 20px;background:color-mix(in srgb, var(--docs-bg-muted) 66%, var(--docs-bg-elevated));overflow:auto}
.docs-code-surface.tabs{border-radius:0}
.docs-code-content{display:block;white-space:pre;font-family:var(--docs-mono, var(--mono), monospace);font-size:13px;line-height:1.75;color:var(--docs-text-soft)}
.docs-code-tab-list{display:flex;gap:6px;flex-wrap:wrap}
.docs-code-tab{padding:7px 12px;border-radius:10px;border:1px solid var(--docs-border);background:var(--docs-bg-elevated);color:var(--docs-text-muted);font-size:12.5px;font-family:var(--docs-mono, var(--mono), monospace);cursor:pointer;transition:all .12s}
.docs-code-tab.active{color:var(--docs-text);border-color:color-mix(in srgb, var(--docs-accent) 28%, var(--docs-border));background:var(--docs-accent-soft)}
`;
}
