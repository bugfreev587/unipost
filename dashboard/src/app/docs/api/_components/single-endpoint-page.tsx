"use client";

import {
  ApiReferencePage,
  ApiReferenceGrid,
  ApiEndpointCard,
  ApiAccordion,
  ApiFieldList,
  ApiRequestConfigCard,
  CodeTabs,
  type ApiGuideLink,
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

const ENDPOINT_GUIDE_LINKS: Array<{
  method: Method;
  path: RegExp;
  guides: ApiGuideLink[];
}> = [
  {
    method: "POST",
    path: /^\/v1\/media\/audio-overlays$/,
    guides: [{ label: "Video + audio overlay", href: "/docs/guides/video-audio-overlay" }],
  },
  {
    method: "POST",
    path: /^\/v1\/media$/,
    guides: [{ label: "Video + audio overlay", href: "/docs/guides/video-audio-overlay" }],
  },
  {
    method: "GET",
    path: /^\/v1\/media\/:[^/]+$/,
    guides: [{ label: "Video + audio overlay", href: "/docs/guides/video-audio-overlay" }],
  },
  {
    method: "POST",
    path: /^\/v1\/posts$/,
    guides: [
      { label: "Publishing guide", href: "/docs/publishing" },
      { label: "Video + audio overlay", href: "/docs/guides/video-audio-overlay" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/posts\/:[^/]+$/,
    guides: [{ label: "Publishing guide", href: "/docs/publishing" }],
  },
  {
    method: "GET",
    path: /^\/v1\/accounts$/,
    guides: [
      { label: "Get account metrics", href: "/docs/guides/analytics/account-metrics" },
      { label: "Get TikTok followers", href: "/docs/guides/analytics/tiktok-followers" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/accounts\/:[^/]+\/metrics$/,
    guides: [
      { label: "Get account metrics", href: "/docs/guides/analytics/account-metrics" },
      { label: "Get TikTok followers", href: "/docs/guides/analytics/tiktok-followers" },
      { label: "Reconnect analytics scopes", href: "/docs/guides/analytics/reconnect-analytics-scopes" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/posts\/:[^/]+\/analytics$/,
    guides: [{ label: "Get post analytics", href: "/docs/guides/analytics/post-analytics" }],
  },
  {
    method: "GET",
    path: /^\/v1\/analytics\/posts\/export$/,
    guides: [{ label: "Export post analytics rows", href: "/docs/guides/analytics/export-post-analytics" }],
  },
  {
    method: "GET",
    path: /^\/v1\/accounts\/:[^/]+\/health$/,
    guides: [{ label: "Reconnect analytics scopes", href: "/docs/guides/analytics/reconnect-analytics-scopes" }],
  },
  {
    method: "POST",
    path: /^\/v1\/oauth\/connect$/,
    guides: [
      { label: "Publishing guide", href: "/docs/publishing" },
      { label: "Reconnect analytics scopes", href: "/docs/guides/analytics/reconnect-analytics-scopes" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/analytics\/platforms$/,
    guides: [{ label: "Analytics guides", href: "/docs/guides/analytics" }],
  },
  {
    method: "POST",
    path: /^\/v1\/connect\/sessions$/,
    guides: [
      { label: "Connect Sessions guide", href: "/docs/connect-sessions" },
      { label: "Local Connect testing", href: "/docs/local-connect-test" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/connect\/sessions\/:[^/]+$/,
    guides: [
      { label: "Connect Sessions guide", href: "/docs/connect-sessions" },
      { label: "Local Connect testing", href: "/docs/local-connect-test" },
    ],
  },
  {
    method: "GET",
    path: /^\/v1\/accounts\/:[^/]+\/youtube\/analytics\/(?:summary|trend|videos)$/,
    guides: [{ label: "Reconnect analytics scopes", href: "/docs/guides/analytics/reconnect-analytics-scopes" }],
  },
];

function normalizeEndpointPathForGuideLookup(path: string) {
  return path.replace(/\{[^}]+\}/g, ":id");
}

function resolveEndpointGuideLinks(method: Method, path: string) {
  const normalizedPath = normalizeEndpointPathForGuideLookup(path);
  return ENDPOINT_GUIDE_LINKS.find((item) => item.method === method && item.path.test(normalizedPath))?.guides || [];
}

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
  breadcrumbItems,
  section,
  title,
  description,
  method,
  path,
  requestSections,
  responses,
  snippets,
  responseSnippets,
  guideLinks,
  children,
}: {
  breadcrumbItems?: { label: string; href?: string }[];
  section: string;
  title: string;
  description: React.ReactNode;
  guideLinks?: ApiGuideLink[];
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
  const resolvedGuideLinks = guideLinks ?? resolveEndpointGuideLinks(method, path);

  return (
    <ApiReferencePage breadcrumbItems={breadcrumbItems} section={section} title={title} description={description} guideLinks={resolvedGuideLinks}>
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
