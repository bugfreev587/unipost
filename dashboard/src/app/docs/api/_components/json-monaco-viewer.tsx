"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Check, Copy } from "lucide-react";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

export type JsonViewerSnippet = {
  label: string;
  code: string;
  lang?: string;
};

type MonacoLanguage = "javascript" | "python" | "go" | "json" | "shell" | "plaintext";

function tryFormatJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return null;
  }
}

function estimateHeight(text: string) {
  const lines = text.split("\n").length;
  return Math.min(Math.max(lines * 20 + 18, 160), 460);
}

function normalizeMonacoLanguage(value?: string, code?: string): MonacoLanguage {
  const lower = (value || "").toLowerCase();

  if (lower.includes("javascript") || lower === "js" || lower === "node.js") return "javascript";
  if (lower.includes("python") || lower === "py") return "python";
  if (lower === "go" || lower.includes("golang")) return "go";
  if (lower === "json") return "json";
  if (lower === "bash" || lower === "shell" || lower === "sh" || lower === "curl") return "shell";

  const sample = code || "";
  if (/^\s*[{[]/.test(sample)) return "json";
  if (/^\s*(curl\b|npm\b|pnpm\b|yarn\b|pip\b|go get\b|export\b)/m.test(sample)) return "shell";
  if (/\bpackage main\b|\bfunc\s+\w+\s*\(/.test(sample)) return "go";
  if (/\bfrom\s+\w+\s+import\b|\bdef\s+\w+\s*\(|\bprint\s*\(/.test(sample)) return "python";
  if (/\bconst\b|\blet\b|\bawait\b|\bimport\s+[{*]/.test(sample)) return "javascript";
  return "plaintext";
}

function getViewerValue(code: string, language: MonacoLanguage) {
  if (language === "json") {
    return tryFormatJson(code) || code;
  }
  return code;
}

function CopyButton({ code }: { code: string }) {
  const [copied, setCopied] = useState(false);

  async function handleCopy() {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }

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

export function JsonMonacoViewer({
  code,
  height,
}: {
  code: string;
  height?: number;
}) {
  const formatted = useMemo(() => tryFormatJson(code), [code]);
  const [themeName, setThemeName] = useState("unipost-json-dark");

  if (!formatted) {
    return null;
  }

  function applyTheme(monaco: typeof import("monaco-editor")) {
    const styles = getComputedStyle(document.documentElement);
    const editorBackground = styles.getPropertyValue("--docs-tech-bg").trim() || "#2c2d39";
    const editorForeground = styles.getPropertyValue("--docs-tech-text-soft").trim() || "#d6d9e5";
    const lineNumber = styles.getPropertyValue("--docs-tech-muted").trim() || "#9aa0b5";
    const borderColor = styles.getPropertyValue("--docs-tech-border").trim() || "#3a3d4f";
    const stringColor = styles.getPropertyValue("--docs-code-string").trim() || "#7dc7ff";
    const numberColor = styles.getPropertyValue("--docs-code-number").trim() || "#f9b44d";
    const keywordColor = styles.getPropertyValue("--docs-code-keyword").trim() || "#d1a8ff";
    const constantColor = styles.getPropertyValue("--docs-code-constant").trim() || "#f08ab1";

    monaco.editor.defineTheme("unipost-json-dark", {
      base: "vs-dark",
      inherit: true,
      rules: [
        { token: "string.key.json", foreground: stringColor.replace("#", "") },
        { token: "string.value.json", foreground: constantColor.replace("#", "") },
        { token: "number", foreground: numberColor.replace("#", "") },
        { token: "keyword.json", foreground: keywordColor.replace("#", "") },
      ],
      colors: {
        "editor.background": editorBackground,
        "editor.foreground": editorForeground,
        "editorLineNumber.foreground": lineNumber,
        "editorLineNumber.activeForeground": editorForeground,
        "editorGutter.background": editorBackground,
        "editorIndentGuide.background1": borderColor,
        "editorIndentGuide.activeBackground1": lineNumber,
        "editor.selectionBackground": "rgba(124,178,255,0.16)",
        "editor.inactiveSelectionBackground": "rgba(124,178,255,0.10)",
      },
    });
    setThemeName("unipost-json-dark");
  }

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const monaco = (window as typeof window & { monaco?: typeof import("monaco-editor") }).monaco;
      if (monaco) {
        applyTheme(monaco);
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return (
    <div
      style={{
        border: "1px solid var(--docs-border)",
        borderRadius: 16,
        overflow: "hidden",
        background: "var(--docs-tech-bg)",
      }}
    >
      <MonacoEditor
        height={height ?? estimateHeight(formatted)}
        defaultLanguage="json"
        theme={themeName}
        value={formatted}
        beforeMount={(monaco) => {
          applyTheme(monaco);
        }}
        onMount={(_, monaco) => {
          (window as typeof window & { monaco?: typeof import("monaco-editor") }).monaco = monaco;
          applyTheme(monaco);
        }}
        options={{
          readOnly: true,
          domReadOnly: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: "on",
          folding: true,
          lineNumbers: "on",
          renderLineHighlight: "none",
          glyphMargin: false,
          overviewRulerBorder: false,
          overviewRulerLanes: 0,
          padding: { top: 12, bottom: 12 },
          fontSize: 13,
          lineHeight: 21,
          fontFamily: "var(--docs-mono, var(--mono), monospace)",
          scrollbar: {
            verticalScrollbarSize: 10,
            horizontalScrollbarSize: 10,
          },
        }}
      />
    </div>
  );
}

export function MonacoCodeViewer({
  code,
  language,
  height,
}: {
  code: string;
  language?: string;
  height?: number;
}) {
  const normalizedLanguage = useMemo(() => normalizeMonacoLanguage(language, code), [code, language]);
  const value = useMemo(() => getViewerValue(code, normalizedLanguage), [code, normalizedLanguage]);
  const [themeName, setThemeName] = useState("unipost-snippet-dark");

  function applyTheme(monaco: typeof import("monaco-editor")) {
    const styles = getComputedStyle(document.documentElement);
    const editorBackground = styles.getPropertyValue("--docs-tech-bg").trim() || "#2c2d39";
    const editorForeground = styles.getPropertyValue("--docs-tech-text-soft").trim() || "#d6d9e5";
    const borderColor = styles.getPropertyValue("--docs-tech-border").trim() || "#3a3d4f";
    const stringColor = styles.getPropertyValue("--docs-code-string").trim() || "#7dc7ff";
    const numberColor = styles.getPropertyValue("--docs-code-number").trim() || "#f9b44d";
    const keywordColor = styles.getPropertyValue("--docs-code-keyword").trim() || "#d1a8ff";
    const constantColor = styles.getPropertyValue("--docs-code-constant").trim() || "#f08ab1";
    const functionColor = styles.getPropertyValue("--docs-code-function").trim() || "#ff9857";
    const typeColor = styles.getPropertyValue("--docs-code-type").trim() || "#6dd39a";
    const commentColor = styles.getPropertyValue("--docs-code-comment").trim() || "#7c8aa0";

    monaco.editor.defineTheme("unipost-snippet-dark", {
      base: "vs-dark",
      inherit: true,
      rules: [
        { token: "comment", foreground: commentColor.replace("#", "") },
        { token: "string", foreground: stringColor.replace("#", "") },
        { token: "string.key.json", foreground: stringColor.replace("#", "") },
        { token: "string.value.json", foreground: constantColor.replace("#", "") },
        { token: "number", foreground: numberColor.replace("#", "") },
        { token: "keyword", foreground: keywordColor.replace("#", "") },
        { token: "keyword.json", foreground: keywordColor.replace("#", "") },
        { token: "identifier.function", foreground: functionColor.replace("#", "") },
        { token: "function", foreground: functionColor.replace("#", "") },
        { token: "type.identifier", foreground: typeColor.replace("#", "") },
      ],
      colors: {
        "editor.background": editorBackground,
        "editor.foreground": editorForeground,
        "editorGutter.background": editorBackground,
        "editorIndentGuide.background1": borderColor,
        "editorIndentGuide.activeBackground1": borderColor,
        "editor.selectionBackground": "rgba(124,178,255,0.14)",
        "editor.inactiveSelectionBackground": "rgba(124,178,255,0.08)",
        "editor.lineHighlightBackground": "transparent",
      },
    });
    setThemeName("unipost-snippet-dark");
  }

  useEffect(() => {
    const observer = new MutationObserver(() => {
      const monaco = (window as typeof window & { monaco?: typeof import("monaco-editor") }).monaco;
      if (monaco) {
        applyTheme(monaco);
      }
    });
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    return () => observer.disconnect();
  }, []);

  return (
    <div
      style={{
        border: "1px solid var(--docs-border)",
        borderRadius: 16,
        overflow: "hidden",
        background: "var(--docs-tech-bg)",
      }}
    >
      <MonacoEditor
        height={height ?? estimateHeight(value)}
        defaultLanguage={normalizedLanguage}
        theme={themeName}
        value={value}
        beforeMount={(monaco) => {
          applyTheme(monaco);
        }}
        onMount={(_, monaco) => {
          (window as typeof window & { monaco?: typeof import("monaco-editor") }).monaco = monaco;
          applyTheme(monaco);
        }}
        options={{
          readOnly: true,
          domReadOnly: true,
          minimap: { enabled: false },
          scrollBeyondLastLine: false,
          wordWrap: "on",
          folding: false,
          lineNumbers: "off",
          lineDecorationsWidth: 0,
          lineNumbersMinChars: 0,
          glyphMargin: false,
          renderLineHighlight: "none",
          overviewRulerBorder: false,
          overviewRulerLanes: 0,
          hideCursorInOverviewRuler: true,
          guides: { indentation: false },
          padding: { top: 10, bottom: 10 },
          fontSize: 12.5,
          lineHeight: 19,
          fontFamily: "var(--docs-mono, var(--mono), monospace)",
          scrollbar: {
            verticalScrollbarSize: 8,
            horizontalScrollbarSize: 8,
          },
        }}
      />
    </div>
  );
}

export function JsonMonacoTabs({ snippets }: { snippets: JsonViewerSnippet[] }) {
  const validSnippets = useMemo(
    () => snippets.filter((snippet) => tryFormatJson(snippet.code)),
    [snippets]
  );
  const [active, setActive] = useState(0);

  if (validSnippets.length === 0) {
    return null;
  }

  const current = validSnippets[Math.min(active, validSnippets.length - 1)];
  const formatted = tryFormatJson(current.code);
  if (!formatted) {
    return null;
  }

  return (
    <div className="docs-code-tabs" style={{ margin: 0 }}>
      <div className="docs-code-tabs-header">
        <div className="docs-code-tab-list">
          {validSnippets.map((snippet, index) => (
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
        <CopyButton code={formatted} />
      </div>
      <JsonMonacoViewer code={formatted} />
    </div>
  );
}

export function MonacoTabs({ snippets }: { snippets: JsonViewerSnippet[] }) {
  const [active, setActive] = useState(0);
  const current = snippets[Math.min(active, snippets.length - 1)];
  const copyValue = getViewerValue(current.code, normalizeMonacoLanguage(current.lang || current.label, current.code));

  return (
    <div className="docs-code-tabs" style={{ margin: 0 }}>
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
        <CopyButton code={copyValue} />
      </div>
      <MonacoCodeViewer code={current.code} language={current.lang || current.label} />
    </div>
  );
}
