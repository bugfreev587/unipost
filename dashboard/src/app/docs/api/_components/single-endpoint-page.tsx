"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  CodeTabs,
  type ApiFieldItem,
} from "./doc-components";

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

type RequestSection = {
  title: string;
  items: ApiFieldItem[];
};

type ResponseSection = {
  code: string;
  fields: ApiFieldItem[];
};

type Snippet = {
  lang: string;
  label: string;
  code: string;
};

const METHOD_COLORS: Record<Method, string> = {
  GET: "#10b981",
  POST: "#3b82f6",
  PUT: "#f59e0b",
  PATCH: "#a855f7",
  DELETE: "#ef4444",
};

export function SingleEndpointReferencePage({
  section,
  title,
  description,
  method,
  path,
  requestSections,
  responses,
  snippets,
  responseSnippets,
  children,
}: {
  section: string;
  title: string;
  description: React.ReactNode;
  method: Method;
  path: string;
  requestSections: RequestSection[];
  responses: ResponseSection[];
  snippets: Snippet[];
  responseSnippets: Snippet[];
  children?: React.ReactNode;
}) {
  return (
    <ApiReferencePage section={section} title={title} description={description}>
      <ApiReferenceGrid
        left={
          <div style={{ display: "grid", gap: 16 }}>
            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "16px 18px" }}>
                <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: METHOD_COLORS[method], marginRight: 12 }}>{method}</span>
                <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{path}</code>
              </div>
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              {requestSections.map((section, index) => (
                <div
                  key={section.title}
                  style={{
                    padding: "18px",
                    borderBottom: index < requestSections.length - 1 ? "1px solid var(--docs-border)" : undefined,
                  }}
                >
                  <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>{section.title}</div>
                  <ApiFieldList items={section.items} />
                </div>
              ))}
            </ApiEndpointCard>

            <ApiEndpointCard method={method} path={path}>
              <div style={{ padding: "18px 18px 4px" }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: "var(--docs-text)", marginBottom: 14 }}>Response Body</div>
              </div>
              {responses.map((response) => (
                <ApiAccordion key={response.code} title={response.code}>
                  <ApiFieldList items={response.fields} />
                </ApiAccordion>
              ))}
            </ApiEndpointCard>
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={snippets} />
            <CodeTabs snippets={responseSnippets} />
          </div>
        }
      />
      {children ? <div style={{ marginTop: 20 }}>{children}</div> : null}
    </ApiReferencePage>
  );
}
