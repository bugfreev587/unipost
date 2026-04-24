import type { CodeSnippet } from "../../_components/code-block";

type JsonValue =
  | string
  | number
  | boolean
  | null
  | JsonValue[]
  | { [key: string]: JsonValue };

type JsonObject = { [key: string]: JsonValue };

const TOP_LEVEL_JS_KEYS: Record<string, string> = {
  caption: "caption",
  account_ids: "accountIds",
  media_urls: "mediaUrls",
  media_ids: "mediaIds",
  scheduled_at: "scheduledAt",
  platform_options: "platformOptions",
  platform_posts: "platformPosts",
  status: "status",
  archived: "archived",
  idempotency_key: "idempotencyKey",
};

const PLATFORM_POST_JS_KEYS: Record<string, string> = {
  account_id: "accountId",
  caption: "caption",
  media_urls: "mediaUrls",
  media_ids: "mediaIds",
  thread_position: "threadPosition",
  first_comment: "firstComment",
  in_reply_to: "inReplyTo",
  platform_options: "platformOptions",
};

type GoFieldType = "string" | "stringSlice" | "int" | "bool" | "map" | "platformPosts";

const TOP_LEVEL_GO_FIELDS: Record<string, { name: string; type: GoFieldType }> = {
  caption: { name: "Caption", type: "string" },
  account_ids: { name: "AccountIDs", type: "stringSlice" },
  media_urls: { name: "MediaURLs", type: "stringSlice" },
  media_ids: { name: "MediaIDs", type: "stringSlice" },
  scheduled_at: { name: "ScheduledAt", type: "string" },
  platform_options: { name: "PlatformOptions", type: "map" },
  platform_posts: { name: "PlatformPosts", type: "platformPosts" },
  status: { name: "Status", type: "string" },
  archived: { name: "Archived", type: "bool" },
  idempotency_key: { name: "IdempotencyKey", type: "string" },
};

const PLATFORM_POST_GO_FIELDS: Record<string, { name: string; type: GoFieldType }> = {
  account_id: { name: "AccountID", type: "string" },
  caption: { name: "Caption", type: "string" },
  media_urls: { name: "MediaURLs", type: "stringSlice" },
  media_ids: { name: "MediaIDs", type: "stringSlice" },
  thread_position: { name: "ThreadPosition", type: "int" },
  first_comment: { name: "FirstComment", type: "string" },
  in_reply_to: { name: "InReplyTo", type: "string" },
  platform_options: { name: "PlatformOptions", type: "map" },
};

function pad(level: number): string {
  return "  ".repeat(level);
}

function isIdent(s: string): boolean {
  return /^[A-Za-z_$][A-Za-z0-9_$]*$/.test(s);
}

function isObject(v: JsonValue): v is JsonObject {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// -------- cURL --------

function renderCurl(parsed: JsonObject): string {
  const pretty = JSON.stringify(parsed, null, 2);
  const indented = pretty
    .split("\n")
    .map((line, i) => (i === 0 ? line : "  " + line))
    .join("\n");
  return `curl -X POST "https://api.unipost.dev/v1/posts" \\
  -H "Authorization: Bearer $UNIPOST_API_KEY" \\
  -H "Content-Type: application/json" \\
  -d '${indented}'`;
}

// -------- JavaScript --------

function jsObj(obj: JsonObject, depth: number, keyMap?: Record<string, string>): string {
  const entries = Object.entries(obj);
  if (entries.length === 0) return "{}";
  const lines = entries.map(([k, v]) => {
    const outKey = keyMap?.[k] ?? k;
    const keyStr = isIdent(outKey) ? outKey : JSON.stringify(outKey);
    let valStr: string;
    if (k === "platform_posts" && Array.isArray(v)) {
      valStr = jsPlatformPostsArr(v, depth + 1);
    } else if (k === "platform_options") {
      valStr = jsValueRaw(v, depth + 1);
    } else {
      valStr = jsValueRaw(v, depth + 1);
    }
    return `${pad(depth + 1)}${keyStr}: ${valStr}`;
  });
  return `{\n${lines.join(",\n")},\n${pad(depth)}}`;
}

function jsPlatformPostsArr(arr: JsonValue[], depth: number): string {
  if (arr.length === 0) return "[]";
  const items = arr.map((item) => {
    if (isObject(item)) {
      return `${pad(depth + 1)}${jsObj(item, depth + 1, PLATFORM_POST_JS_KEYS)}`;
    }
    return `${pad(depth + 1)}${jsValueRaw(item, depth + 1)}`;
  });
  return `[\n${items.join(",\n")},\n${pad(depth)}]`;
}

function jsValueRaw(v: JsonValue, depth: number): string {
  if (v === null) return "null";
  if (typeof v === "string") return JSON.stringify(v);
  if (typeof v === "number" || typeof v === "boolean") return String(v);
  if (Array.isArray(v)) {
    if (v.length === 0) return "[]";
    if (v.every((x) => typeof x === "string")) {
      const inline = "[" + v.map((x) => JSON.stringify(x)).join(", ") + "]";
      if (inline.length <= 70) return inline;
      const items = v.map((x) => `${pad(depth + 1)}${JSON.stringify(x)}`).join(",\n");
      return `[\n${items},\n${pad(depth)}]`;
    }
    const items = v.map((x) => `${pad(depth + 1)}${jsValueRaw(x, depth + 1)}`).join(",\n");
    return `[\n${items},\n${pad(depth)}]`;
  }
  if (isObject(v)) return jsObj(v, depth);
  return JSON.stringify(v);
}

function renderJs(parsed: JsonObject): string {
  const literal = jsObj(parsed, 0, TOP_LEVEL_JS_KEYS);
  return `import { UniPost } from "@unipost/sdk";

const client = new UniPost({
  apiKey: process.env.UNIPOST_API_KEY,
});

const post = await client.posts.create(${literal});

console.log(post.id);`;
}

// -------- Python --------

function pyDict(obj: JsonObject, depth: number): string {
  const entries = Object.entries(obj);
  if (entries.length === 0) return "{}";
  const lines = entries.map(
    ([k, v]) => `${pad(depth + 1)}${JSON.stringify(k)}: ${pyValueRaw(v, depth + 1)}`,
  );
  return `{\n${lines.join(",\n")},\n${pad(depth)}}`;
}

function pyValueRaw(v: JsonValue, depth: number): string {
  if (v === null) return "None";
  if (typeof v === "string") return JSON.stringify(v);
  if (typeof v === "number") return String(v);
  if (typeof v === "boolean") return v ? "True" : "False";
  if (Array.isArray(v)) {
    if (v.length === 0) return "[]";
    if (v.every((x) => typeof x === "string")) {
      const inline = "[" + v.map((x) => JSON.stringify(x)).join(", ") + "]";
      if (inline.length <= 70) return inline;
      const items = v.map((x) => `${pad(depth + 1)}${JSON.stringify(x)}`).join(",\n");
      return `[\n${items},\n${pad(depth)}]`;
    }
    const items = v.map((x) => `${pad(depth + 1)}${pyValueRaw(x, depth + 1)}`).join(",\n");
    return `[\n${items},\n${pad(depth)}]`;
  }
  if (isObject(v)) return pyDict(v, depth);
  return JSON.stringify(v);
}

function renderPython(parsed: JsonObject): string {
  const entries = Object.entries(parsed);
  const kwargs = entries
    .map(([k, v]) => `  ${k}=${pyValueRaw(v, 1)}`)
    .join(",\n");
  return `from unipost import UniPost
import os

client = UniPost(api_key=os.environ["UNIPOST_API_KEY"])

post = client.posts.create(
${kwargs},
)

print(post.id)`;
}

// -------- Go --------

function goParams(parsed: JsonObject): string {
  const entries = Object.entries(parsed);
  const lines = entries.map(([k, v]) => {
    const info = TOP_LEVEL_GO_FIELDS[k];
    if (!info) return `    // unknown field: ${k}`;
    return `    ${info.name}: ${goRenderValue(v, info.type, 2)},`;
  });
  return lines.join("\n");
}

function goPlatformPostLiteral(item: JsonObject, depth: number): string {
  const entries = Object.entries(item);
  const lines = entries.map(([k, v]) => {
    const info = PLATFORM_POST_GO_FIELDS[k];
    if (!info) return `${pad(depth + 1)}// unknown field: ${k}`;
    return `${pad(depth + 1)}${info.name}: ${goRenderValue(v, info.type, depth + 1)},`;
  });
  return `{\n${lines.join("\n")}\n${pad(depth)}}`;
}

function goRenderValue(v: JsonValue, type: GoFieldType, depth: number): string {
  switch (type) {
    case "string":
      return JSON.stringify(v);
    case "int":
      return String(v);
    case "bool":
      return String(v);
    case "stringSlice": {
      if (!Array.isArray(v) || v.length === 0) return "nil";
      const inline = `[]string{${v.map((x) => JSON.stringify(x)).join(", ")}}`;
      if (inline.length <= 80) return inline;
      const items = v.map((x) => `${pad(depth + 1)}${JSON.stringify(x)},`).join("\n");
      return `[]string{\n${items}\n${pad(depth)}}`;
    }
    case "map": {
      if (!isObject(v)) return "nil";
      return goMap(v, depth);
    }
    case "platformPosts": {
      if (!Array.isArray(v) || v.length === 0) return "nil";
      const items = v
        .map((item) =>
          isObject(item)
            ? `${pad(depth + 1)}${goPlatformPostLiteral(item, depth + 1)},`
            : `${pad(depth + 1)}// invalid element`,
        )
        .join("\n");
      return `[]unipost.PlatformPost{\n${items}\n${pad(depth)}}`;
    }
  }
}

function goMap(obj: JsonObject, depth: number): string {
  const entries = Object.entries(obj);
  if (entries.length === 0) return "map[string]any{}";
  const lines = entries.map(
    ([k, v]) => `${pad(depth + 1)}${JSON.stringify(k)}: ${goAnyValue(v, depth + 1)},`,
  );
  return `map[string]any{\n${lines.join("\n")}\n${pad(depth)}}`;
}

function goAnyValue(v: JsonValue, depth: number): string {
  if (v === null) return "nil";
  if (typeof v === "string") return JSON.stringify(v);
  if (typeof v === "number") return String(v);
  if (typeof v === "boolean") return String(v);
  if (Array.isArray(v)) {
    if (v.length === 0) return "[]any{}";
    if (v.every((x) => typeof x === "string")) {
      return `[]string{${v.map((x) => JSON.stringify(x)).join(", ")}}`;
    }
    const items = v.map((x) => `${pad(depth + 1)}${goAnyValue(x, depth + 1)},`).join("\n");
    return `[]any{\n${items}\n${pad(depth)}}`;
  }
  if (isObject(v)) return goMap(v, depth);
  return "nil";
}

function renderGo(parsed: JsonObject): string {
  const params = goParams(parsed);
  return `package main

import (
  "context"
  "log"
  "os"

  "github.com/unipost-dev/sdk-go/unipost"
)

func main() {
  client := unipost.NewClient(
    unipost.WithAPIKey(os.Getenv("UNIPOST_API_KEY")),
  )

  post, err := client.Posts.Create(context.Background(), &unipost.CreatePostParams{
${params}
  })
  if err != nil {
    log.Fatal(err)
  }

  _ = post
}`;
}

// -------- Public --------

export function toExampleSnippets(body: string): CodeSnippet[] {
  const parsed = JSON.parse(body) as JsonObject;
  return [
    { label: "cURL", lang: "bash", code: renderCurl(parsed) },
    { label: "Node.js", lang: "javascript", code: renderJs(parsed) },
    { label: "Python", lang: "python", code: renderPython(parsed) },
    { label: "Go", lang: "go", code: renderGo(parsed) },
  ];
}
