"use client";

import { useEffect, useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Check, Copy, Maximize2, X } from "lucide-react";
import { createPortal } from "react-dom";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

export type JsonViewerSnippet = {
  label: string;
  code: string;
  lang?: string;
};

type MonacoLanguage = "javascript" | "python" | "go" | "json" | "shell" | "plaintext";
type ViewerThemeVariant = "default" | "api";

function tryFormatJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return null;
  }
}

function estimateHeight(text: string, opts?: { min?: number; max?: number; lineHeight?: number; extraLines?: number; padding?: number }) {
  const lines = text.split("\n").length;
  const max = opts?.max ?? Number.POSITIVE_INFINITY;
  const min = opts?.min ?? 0;
  const lineHeight = opts?.lineHeight ?? 20;
  const extraLines = opts?.extraLines ?? 1;
  const padding = opts?.padding ?? 18;
  return Math.max(min, Math.min((lines + extraLines) * lineHeight + padding, max));
}

function estimateViewerHeight(text: string, themeVariant: ViewerThemeVariant, maxHeight: number) {
  if (themeVariant === "api") {
    return estimateHeight(text, { max: maxHeight, lineHeight: 22, extraLines: 0, padding: 48 });
  }

  return estimateHeight(text, { max: maxHeight, lineHeight: 20, extraLines: 1, padding: 28 });
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

function ExpandButton({ code, language, label, themeVariant }: {
  code: string;
  language?: string;
  label: string;
  themeVariant: ViewerThemeVariant;
}) {
  const [open, setOpen] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!open) return;

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setOpen(false);
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [open]);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="docs-expand-button"
        aria-label="Expand code example"
      >
        <Maximize2 size={16} />
      </button>
      {open && mounted ? createPortal((
        <div
          className="docs-code-expand-overlay"
          role="dialog"
          aria-modal="true"
          aria-label={`${label} code example`}
          style={{
            position: "fixed",
            inset: 0,
            zIndex: 90,
            background: "rgba(7,10,16,.62)",
            backdropFilter: "blur(12px)",
          }}
        >
          <div
            className="docs-code-expand-panel"
            style={{
              position: "fixed",
              top: "50%",
              left: "50%",
              width: "min(1180px, 72vw)",
              height: "min(760px, 68vh)",
              transform: "translate(-50%, -50%)",
              display: "grid",
              gridTemplateRows: "auto minmax(0, 1fr)",
              overflow: "hidden",
              border: "1px solid color-mix(in srgb, var(--docs-border) 70%, transparent)",
              borderRadius: 10,
              background: "#272936",
              boxShadow: "0 28px 90px rgba(0,0,0,.42)",
            }}
          >
            <div className="docs-code-expand-header">
              <div className="docs-code-expand-title">{label}</div>
              <div className="docs-code-actions">
                <CopyButton code={code} />
                <button
                  type="button"
                  onClick={() => setOpen(false)}
                  className="docs-expand-button"
                  aria-label="Close expanded code example"
                >
                  <X size={16} />
                </button>
              </div>
            </div>
            <div className="docs-code-expand-body">
              {normalizeMonacoLanguage(language || label, code) === "json" ? (
                <JsonMonacoViewer code={code} height="72vh" themeVariant={themeVariant} expanded />
              ) : (
                <MonacoCodeViewer code={code} language={language || label} height="72vh" themeVariant={themeVariant} expanded />
              )}
            </div>
          </div>
        </div>
      ), document.body) : null}
    </>
  );
}

export function JsonMonacoViewer({
  code,
  height,
  maxHeight = 468,
  themeVariant = "default",
  expanded = false,
}: {
  code: string;
  height?: number | string;
  maxHeight?: number;
  themeVariant?: ViewerThemeVariant;
  expanded?: boolean;
}) {
  const formatted = useMemo(() => tryFormatJson(code), [code]);
  const [themeName, setThemeName] = useState("unipost-json-dark");

  function applyTheme(monaco: typeof import("monaco-editor")) {
    const isDark = document.documentElement.classList.contains("dark");
    const isApi = themeVariant === "api";
    const name = isDark ? "unipost-json-dark" : "unipost-json-light";
    const styles = getComputedStyle(document.documentElement);
    const editorBackground = isApi
      ? "#272936"
      : (styles.getPropertyValue("--docs-code-surface-bg").trim() || styles.getPropertyValue("--docs-tech-bg").trim() || "#2c2d39");
    const editorForeground = isApi ? "#f4f4f5" : (styles.getPropertyValue("--docs-tech-text-soft").trim() || "#d6d9e5");
    const lineNumber = isApi ? "#a1a1aa" : (styles.getPropertyValue("--docs-tech-muted").trim() || "#9aa0b5");
    const borderColor = isApi ? "#3f4150" : (styles.getPropertyValue("--docs-tech-border").trim() || "#3a3d4f");
    const keyColor = isApi ? "#4aa3ff" : (styles.getPropertyValue("--docs-code-string").trim() || "#7dc7ff");
    const stringColor = isApi ? "#d65f13" : (styles.getPropertyValue("--docs-code-constant").trim() || "#f08ab1");
    const numberColor = isApi ? "#0b7fd3" : (styles.getPropertyValue("--docs-code-number").trim() || "#f9b44d");
    const keywordColor = isApi ? "#ffdd00" : (styles.getPropertyValue("--docs-code-keyword").trim() || "#d1a8ff");
    const nullColor = isApi ? "#ff3d5a" : (styles.getPropertyValue("--docs-code-constant").trim() || "#f08ab1");

    monaco.editor.defineTheme(name, {
      base: isApi || isDark ? "vs-dark" : "vs",
      inherit: true,
      rules: [
        { token: "delimiter.bracket.json", foreground: keywordColor.replace("#", "") },
        { token: "string.key.json", foreground: keyColor.replace("#", "") },
        { token: "string.value.json", foreground: stringColor.replace("#", "") },
        { token: "number", foreground: numberColor.replace("#", "") },
        { token: "keyword.json", foreground: nullColor.replace("#", "") },
      ],
      colors: {
        "editor.background": editorBackground,
        "editor.foreground": editorForeground,
        "editorLineNumber.foreground": lineNumber,
        "editorLineNumber.activeForeground": editorForeground,
        "editorGutter.background": editorBackground,
        "editorIndentGuide.background1": borderColor,
        "editorIndentGuide.activeBackground1": lineNumber,
        "editor.selectionBackground": isDark ? "rgba(124,178,255,0.28)" : "rgba(96,165,250,0.24)",
        "editor.inactiveSelectionBackground": isDark ? "rgba(124,178,255,0.16)" : "rgba(96,165,250,0.14)",
      },
    });
    setThemeName(name);
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

  if (!formatted) {
    return null;
  }

  return (
    <div
      className={`docs-monaco-frame${expanded ? " expanded" : ""}`}
      style={{
        border: "1px solid var(--docs-border)",
        borderRadius: themeVariant === "api" ? 8 : 16,
        overflow: "hidden",
        background: themeVariant === "api" ? "#272936" : "var(--docs-code-surface-bg, var(--docs-tech-bg))",
      }}
    >
      <MonacoEditor
        height={height ?? estimateViewerHeight(formatted, themeVariant, maxHeight)}
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
          wordWrap: "off",
          folding: true,
          lineNumbers: themeVariant === "api" ? "off" : "on",
          renderLineHighlight: "none",
          glyphMargin: false,
          lineDecorationsWidth: themeVariant === "api" ? 0 : undefined,
          lineNumbersMinChars: themeVariant === "api" ? 0 : undefined,
          overviewRulerBorder: false,
          overviewRulerLanes: 0,
          padding: themeVariant === "api" ? { top: 24, bottom: 24 } : { top: 14, bottom: 14 },
          fontSize: themeVariant === "api" ? 13.5 : 13,
          lineHeight: themeVariant === "api" ? 22 : 21,
          fontFamily: "var(--docs-mono, var(--mono), monospace)",
          scrollbar: {
            alwaysConsumeMouseWheel: false,
            handleMouseWheel: expanded,
            verticalScrollbarSize: 10,
            horizontalScrollbarSize: 10,
            vertical: expanded ? "auto" : "hidden",
            horizontal: "auto",
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
  maxHeight = 468,
  themeVariant = "default",
  expanded = false,
}: {
  code: string;
  language?: string;
  height?: number | string;
  maxHeight?: number;
  themeVariant?: ViewerThemeVariant;
  expanded?: boolean;
}) {
  const normalizedLanguage = useMemo(() => normalizeMonacoLanguage(language, code), [code, language]);
  const value = useMemo(() => getViewerValue(code, normalizedLanguage), [code, normalizedLanguage]);
  const [themeName, setThemeName] = useState("unipost-snippet-dark");

  function applyTheme(monaco: typeof import("monaco-editor")) {
    const isDark = document.documentElement.classList.contains("dark");
    const isApi = themeVariant === "api";
    const name = isDark ? "unipost-snippet-dark" : "unipost-snippet-light";
    const styles = getComputedStyle(document.documentElement);
    const editorBackground = isApi
      ? "#272936"
      : (styles.getPropertyValue("--docs-code-surface-bg").trim() || styles.getPropertyValue("--docs-tech-bg").trim() || "#2c2d39");
    const editorForeground = isApi ? "#f4f4f5" : (styles.getPropertyValue("--docs-tech-text-soft").trim() || "#d6d9e5");
    const lineNumber = isApi ? "#a1a1aa" : (styles.getPropertyValue("--docs-tech-muted").trim() || "#9aa0b5");
    const borderColor = isApi ? "#3f4150" : (styles.getPropertyValue("--docs-tech-border").trim() || "#3a3d4f");
    const stringColor = isApi ? "#d65f13" : (styles.getPropertyValue("--docs-code-string").trim() || "#7dc7ff");
    const numberColor = isApi ? "#0b7fd3" : (styles.getPropertyValue("--docs-code-number").trim() || "#f9b44d");
    const keywordColor = isApi ? "#ff3d5a" : (styles.getPropertyValue("--docs-code-keyword").trim() || "#d1a8ff");
    const constantColor = isApi ? "#ff3d5a" : (styles.getPropertyValue("--docs-code-constant").trim() || "#f08ab1");
    const functionColor = styles.getPropertyValue("--docs-code-function").trim() || "#ff9857";
    const typeColor = styles.getPropertyValue("--docs-code-type").trim() || "#6dd39a";
    const commentColor = styles.getPropertyValue("--docs-code-comment").trim() || "#7c8aa0";
    const keyColor = isApi ? "#4aa3ff" : stringColor;
    const bracketColor = isApi ? "#ffdd00" : keywordColor;

    monaco.editor.defineTheme(name, {
      base: isApi || isDark ? "vs-dark" : "vs",
      inherit: true,
      rules: [
        { token: "comment", foreground: commentColor.replace("#", "") },
        { token: "string", foreground: stringColor.replace("#", "") },
        { token: "delimiter.bracket.json", foreground: bracketColor.replace("#", "") },
        { token: "string.key.json", foreground: keyColor.replace("#", "") },
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
        "editorLineNumber.foreground": lineNumber,
        "editorLineNumber.activeForeground": editorForeground,
        "editorGutter.background": editorBackground,
        "editorIndentGuide.background1": borderColor,
        "editorIndentGuide.activeBackground1": lineNumber,
        "editor.selectionBackground": isDark ? "rgba(124,178,255,0.28)" : "rgba(96,165,250,0.24)",
        "editor.inactiveSelectionBackground": isDark ? "rgba(124,178,255,0.16)" : "rgba(96,165,250,0.14)",
        "editor.lineHighlightBackground": "transparent",
      },
    });
    setThemeName(name);
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
      className={`docs-monaco-frame${expanded ? " expanded" : ""}`}
      style={{
        border: "1px solid var(--docs-border)",
        borderRadius: themeVariant === "api" ? 8 : 16,
        overflow: "hidden",
        background: themeVariant === "api" ? "#272936" : "var(--docs-code-surface-bg, var(--docs-tech-bg))",
      }}
    >
      <MonacoEditor
        height={height ?? estimateViewerHeight(value, themeVariant, maxHeight)}
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
          wordWrap: "off",
          folding: true,
          lineNumbers: themeVariant === "api" ? "off" : "on",
          glyphMargin: false,
          lineDecorationsWidth: themeVariant === "api" ? 0 : undefined,
          lineNumbersMinChars: themeVariant === "api" ? 0 : undefined,
          renderLineHighlight: "none",
          overviewRulerBorder: false,
          overviewRulerLanes: 0,
          padding: themeVariant === "api" ? { top: 24, bottom: 24 } : { top: 14, bottom: 14 },
          fontSize: themeVariant === "api" ? 13.5 : 13,
          lineHeight: themeVariant === "api" ? 22 : 21,
          fontFamily: "var(--docs-mono, var(--mono), monospace)",
          scrollbar: {
            alwaysConsumeMouseWheel: false,
            handleMouseWheel: expanded,
            verticalScrollbarSize: 10,
            horizontalScrollbarSize: 10,
            vertical: expanded ? "auto" : "hidden",
            horizontal: "auto",
          },
        }}
      />
    </div>
  );
}

export function JsonMonacoTabs({
  snippets,
  maxHeight = 468,
  themeVariant = "default",
}: {
  snippets: JsonViewerSnippet[];
  maxHeight?: number;
  themeVariant?: ViewerThemeVariant;
}) {
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
        <div className="docs-code-actions">
          <CopyButton code={formatted} />
          <ExpandButton code={formatted} language="json" label={current.label} themeVariant={themeVariant} />
        </div>
      </div>
      <JsonMonacoViewer code={formatted} maxHeight={maxHeight} themeVariant={themeVariant} />
    </div>
  );
}

export function MonacoTabs({
  snippets,
  maxHeight = 468,
  themeVariant = "default",
}: {
  snippets: JsonViewerSnippet[];
  maxHeight?: number;
  themeVariant?: ViewerThemeVariant;
}) {
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
        <div className="docs-code-actions">
          <CopyButton code={copyValue} />
          <ExpandButton code={copyValue} language={current.lang || current.label} label={current.label} themeVariant={themeVariant} />
        </div>
      </div>
      <MonacoCodeViewer code={current.code} language={current.lang || current.label} maxHeight={maxHeight} themeVariant={themeVariant} />
    </div>
  );
}
