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
        gap: 10,
        flexWrap: "wrap",
        marginBottom: 18,
        color: "var(--docs-text-faint)",
        fontSize: 13,
        fontWeight: 560,
      }}
    >
      <Link
        href="/docs"
        className="docs-guide-breadcrumb-home"
        style={{
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
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
          style={{ display: "inline-flex", alignItems: "center", gap: 10 }}
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
                fontSize: 13,
                fontWeight: 700,
                letterSpacing: "0",
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
                fontSize: 13,
                fontWeight: 700,
                letterSpacing: "0",
              }}
            >
              {item.label}
            </span>
          )}
        </div>
      ))}

      <ChevronRight
        className="docs-guide-breadcrumb-chevron"
        size={14}
        strokeWidth={2.4}
        style={{ color: "var(--docs-text-faint)" }}
      />

      <span
        className="docs-guide-breadcrumb-current"
        style={{
          display: "inline-flex",
          alignItems: "center",
          padding: "5px 10px",
          borderRadius: 5,
          background: "color-mix(in srgb, #8a2d8d 12%, transparent)",
          color: "#8a2d8d",
          fontSize: 12,
          fontWeight: 760,
          letterSpacing: ".08em",
          lineHeight: 1,
          textTransform: "uppercase",
          whiteSpace: "nowrap",
        }}
      >
        {current.label}
      </span>
    </nav>
  );
}
