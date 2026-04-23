"use client";

import { useMemo, useState } from "react";
import dynamic from "next/dynamic";
import { Check, Copy } from "lucide-react";
import { codeBlockStyles, type CodeSnippet } from "../../_components/code-block";

const MonacoEditor = dynamic(() => import("@monaco-editor/react"), { ssr: false });

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

  if (!formatted) {
    return null;
  }

  return (
    <>
      <style dangerouslySetInnerHTML={{ __html: codeBlockStyles() }} />
      <div
        style={{
          border: "1px solid var(--docs-border)",
          borderRadius: 16,
          overflow: "hidden",
          background: "#1e1e1e",
        }}
      >
        <MonacoEditor
          height={height ?? estimateHeight(formatted)}
          defaultLanguage="json"
          theme="vs-dark"
          value={formatted}
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
    </>
  );
}

export function JsonMonacoTabs({ snippets }: { snippets: CodeSnippet[] }) {
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
    <>
      <style dangerouslySetInnerHTML={{ __html: codeBlockStyles() }} />
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
    </>
  );
}
