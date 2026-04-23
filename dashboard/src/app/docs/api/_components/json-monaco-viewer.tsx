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

function tryFormatJson(value: string) {
  try {
    return JSON.stringify(JSON.parse(value), null, 2);
  } catch {
    return null;
  }
}

function estimateHeight(text: string) {
  const lines = text.split("\n").length;
  return Math.min(Math.max(lines * 22 + 20, 180), 520);
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
