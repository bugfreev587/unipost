"use client";

import { ChevronRight, Home } from "lucide-react";
import Link from "next/link";

type BreadcrumbItem = {
  label: string;
  href?: string;
};

export function DocsContentBreadcrumb({
  items,
}: {
  items: BreadcrumbItem[];
}) {
  if (!items.length) {
    return null;
  }

  const current = items[items.length - 1];
  const trail = items.slice(0, -1);

  return (
    <nav
      aria-label="Breadcrumb"
      className="docs-guide-breadcrumb"
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        flexWrap: "wrap",
        marginBottom: 20,
      }}
    >
      <Link
        href="/docs"
        className="docs-guide-breadcrumb-home"
        style={{
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          width: 30,
          height: 30,
          borderRadius: 999,
          color: "var(--docs-text-muted)",
          textDecoration: "none",
          transition: "color .18s ease, background-color .18s ease",
        }}
      >
        <Home size={15} strokeWidth={2.7} />
      </Link>

      {trail.map((item) => (
        <div
          key={`${item.label}-${item.href || "current"}`}
          style={{ display: "inline-flex", alignItems: "center", gap: 12 }}
        >
          <ChevronRight
            className="docs-guide-breadcrumb-chevron"
            size={14}
            strokeWidth={2.4}
            style={{ color: "var(--docs-text-faint)" }}
          />
          {item.href ? (
            <Link
              href={item.href}
              className="docs-guide-breadcrumb-link"
              style={{
                color: "var(--docs-text-muted)",
                fontSize: 15,
                fontWeight: 700,
                letterSpacing: "-0.02em",
                textDecoration: "none",
                transition: "color .18s ease",
              }}
            >
              {item.label}
            </Link>
          ) : (
            <span
              className="docs-guide-breadcrumb-link"
              style={{
                color: "var(--docs-text-muted)",
                fontSize: 15,
                fontWeight: 700,
                letterSpacing: "-0.02em",
              }}
            >
              {item.label}
            </span>
          )}
        </div>
      ))}

      {trail.length > 0 ? (
        <ChevronRight
          className="docs-guide-breadcrumb-chevron"
          size={14}
          strokeWidth={2.4}
          style={{ color: "var(--docs-text-faint)" }}
        />
      ) : null}

      <span
        className="docs-guide-breadcrumb-current"
        style={{
          display: "inline-flex",
          alignItems: "center",
          minHeight: 44,
          padding: "0 20px",
          borderRadius: 12,
          background: "color-mix(in srgb, var(--docs-link) 12%, transparent)",
          color: "color-mix(in srgb, var(--docs-link) 88%, #5b2aa6)",
          fontSize: 14,
          fontWeight: 900,
          letterSpacing: ".12em",
          textTransform: "uppercase",
          whiteSpace: "nowrap",
        }}
      >
        {current.label}
      </span>
    </nav>
  );
}
