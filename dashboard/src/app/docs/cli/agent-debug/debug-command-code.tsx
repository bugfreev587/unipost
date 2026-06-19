"use client";

import { Check, Copy, Maximize2, X } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";

export function DebugCommandCode({ code, language }: { code: string; language: string }) {
  const [copied, setCopied] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    if (!expanded) return;

    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        setExpanded(false);
      }
    }

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [expanded]);

  const copyCode = useCallback(async () => {
    await navigator.clipboard.writeText(code);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  }, [code]);

  return (
    <>
      <div className="docs-api-code-tabs">
        <div className="docs-code-tabs debug-command-code">
          <div className="docs-code-tabs-header">
            <div className="docs-code-tab-list">
              <span className="docs-code-tab active">{language}</span>
            </div>
            <div className="docs-code-actions">
              <button
                type="button"
                onClick={copyCode}
                className="docs-copy-button"
                aria-label="Copy code to clipboard"
              >
                {copied ? <Check size={16} /> : <Copy size={16} />}
              </button>
              <button
                type="button"
                onClick={() => setExpanded(true)}
                className="docs-expand-button"
                aria-label="Expand code example"
              >
                <Maximize2 size={16} />
              </button>
            </div>
          </div>
          <pre className="debug-command-code-body">
            <code>{code}</code>
          </pre>
        </div>
      </div>

      {expanded && mounted
        ? createPortal(
            <div
              className="docs-code-expand-overlay"
              role="dialog"
              aria-modal="true"
              aria-label={`${language} code example`}
            >
              <div className="docs-code-expand-panel">
                <div className="docs-code-expand-header">
                  <div className="docs-code-expand-title">{language}</div>
                  <div className="docs-code-actions">
                    <button
                      type="button"
                      onClick={copyCode}
                      className="docs-copy-button"
                      aria-label="Copy code to clipboard"
                    >
                      {copied ? <Check size={16} /> : <Copy size={16} />}
                    </button>
                    <button
                      type="button"
                      onClick={() => setExpanded(false)}
                      className="docs-expand-button"
                      aria-label="Close expanded code example"
                    >
                      <X size={16} />
                    </button>
                  </div>
                </div>
                <div className="docs-code-expand-body">
                  <pre className="debug-command-expanded-body">
                    <code>{code}</code>
                  </pre>
                </div>
              </div>
            </div>,
            document.body,
          )
        : null}
    </>
  );
}
