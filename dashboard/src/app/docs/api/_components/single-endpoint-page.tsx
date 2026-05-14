"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  ApiRequestConfigCard,
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

function normalizeSectionTitle(title: string) {
  return title.trim().toLowerCase();
}

function queryTemplateFromFields(fields: ApiFieldItem[]) {
  if (fields.length === 0) {
    return "";
  }

  return `?${fields
    .map((field) => field.name.replace(/\?$/, ""))
    .map((name) => `{${name}}`)
    .join("&")}`;
}

function isErrorResponse(fields: ApiFieldItem[]) {
  return fields.some((field) => field.name.startsWith("error."));
}

function defaultErrorSnippet(code: string) {
  switch (code) {
    case "400":
      return {
        code: "VALIDATION_ERROR",
        normalized_code: "validation_error",
        message: "Request failed validation.",
      };
    case "401":
      return {
        code: "UNAUTHORIZED",
        normalized_code: "unauthorized",
        message: "Missing or invalid credentials.",
      };
    case "404":
      return {
        code: "NOT_FOUND",
        normalized_code: "not_found",
        message: "Resource not found.",
      };
    case "409":
      return {
        code: "CONFLICT",
        normalized_code: "conflict",
        message: "Request conflicts with the current resource state.",
      };
    case "422":
      return {
        code: "VALIDATION_ERROR",
        normalized_code: "validation_error",
        message: "Request body failed validation.",
      };
    case "500":
      return {
        code: "INTERNAL_ERROR",
        normalized_code: "internal_error",
        message: "Internal server error.",
      };
    case "502":
      return {
        code: "UPSTREAM_ERROR",
        normalized_code: "upstream_error",
        message: "Upstream platform request failed.",
      };
    case "503":
      return {
        code: "SERVICE_UNAVAILABLE",
        normalized_code: "service_unavailable",
        message: "Service unavailable.",
      };
    default:
      return {
        code: "ERROR",
        normalized_code: "error",
        message: `Request failed with status ${code}.`,
      };
  }
}

function buildDefaultResponseSnippet(response: ResponseSection): Snippet {
  if (response.code === "204") {
    return {
      lang: "text",
      label: response.code,
      code: "No response body",
    };
  }

  if (isErrorResponse(response.fields)) {
    return {
      lang: "json",
      label: response.code,
      code: JSON.stringify({
        error: defaultErrorSnippet(response.code),
        request_id: "req_123",
      }, null, 2),
    };
  }

  return {
    lang: "json",
    label: response.code,
    code: JSON.stringify({
      data: {},
      request_id: "req_123",
    }, null, 2),
  };
}

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
  const authSection = requestSections.find((section) => normalizeSectionTitle(section.title) === "authorization");
  const pathSection = requestSections.find((section) => normalizeSectionTitle(section.title).startsWith("path"));
  const querySection = requestSections.find((section) => normalizeSectionTitle(section.title).startsWith("query"));
  const bodySection = requestSections.find((section) => normalizeSectionTitle(section.title) === "request body");
  const shouldRenderPlayground = Boolean(authSection || pathSection || querySection || bodySection);
  const providedResponseSnippets = new Map(responseSnippets.map((snippet) => [snippet.label, snippet]));
  const resolvedResponseSnippets = responses.map((response) => (
    providedResponseSnippets.get(response.code) ?? buildDefaultResponseSnippet(response)
  ));

  return (
    <ApiReferencePage section={section} title={title} description={description}>
      <ApiReferenceGrid
        left={
          <div className="api-reference-left-flow" style={{ display: "grid", gap: 16 }}>
            {shouldRenderPlayground ? (
              <ApiRequestConfigCard
                method={method}
                path={path}
                requestPathTemplate={`${path}${queryTemplateFromFields(querySection?.items || [])}`}
                baseUrl="https://api.unipost.dev"
                authFields={authSection?.items || []}
                pathFields={pathSection?.items || []}
                queryFields={querySection?.items || []}
                bodyFields={bodySection?.items || []}
                useMonacoForJsonResponse
              />
            ) : null}

            {!shouldRenderPlayground ? (
              <section className="api-endpoint-summary">
                <div>
                  <span style={{ fontFamily: "var(--docs-mono)", fontSize: 15, fontWeight: 700, color: METHOD_COLORS[method], marginRight: 12 }}>{method}</span>
                  <code style={{ fontFamily: "var(--docs-mono)", fontSize: 15, color: "var(--docs-text)" }}>{path}</code>
                </div>
              </section>
            ) : null}

            <div className="api-field-sections">
              {requestSections.map((section, index) => (
                <section
                  key={section.title}
                  className="api-field-section"
                  style={{
                    paddingTop: index === 0 ? 0 : undefined,
                  }}
                >
                  <h2 className="api-field-section-title">{section.title}</h2>
                  <ApiFieldList items={section.items} />
                </section>
              ))}
            </div>

            <section className="api-field-section api-response-field-section">
              <h2 className="api-field-section-title">Response Body</h2>
              {responses.map((response) => (
                <ApiAccordion key={response.code} title={response.code}>
                  <ApiFieldList items={response.fields} />
                </ApiAccordion>
              ))}
            </section>

            {children ? <div className="api-reference-left-extra">{children}</div> : null}
          </div>
        }
        right={
          <div style={{ display: "grid", gap: 14, alignContent: "start" }}>
            <CodeTabs snippets={snippets} />
            <CodeTabs snippets={resolvedResponseSnippets} />
          </div>
        }
      />
    </ApiReferencePage>
  );
}
