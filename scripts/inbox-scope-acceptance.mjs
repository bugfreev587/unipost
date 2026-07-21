import assert from "node:assert/strict";
import { createHash, randomBytes } from "node:crypto";
import { spawn } from "node:child_process";
import { request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";

const required = [
  "INBOX_ACCEPT_API_URL",
  "INBOX_ACCEPT_API_KEY",
  "INBOX_ACCEPT_EXTERNAL_USER_A",
  "INBOX_ACCEPT_EXTERNAL_USER_B",
  "INBOX_ACCEPT_ITEM_A",
  "INBOX_ACCEPT_ITEM_B",
  "INBOX_ACCEPT_EVENT_DATABASE_URL",
  "INBOX_ACCEPT_ALLOW_PG_NOTIFY",
];
for (const name of required) {
  if (!process.env[name]) {
    throw new Error("missing required acceptance input: " + name);
  }
}
if (process.env.INBOX_ACCEPT_ALLOW_PG_NOTIFY !== "1") {
  throw new Error("INBOX_ACCEPT_ALLOW_PG_NOTIFY must equal 1 for the ephemeral WebSocket probe");
}

const apiURL = new URL(process.env.INBOX_ACCEPT_API_URL);
if (apiURL.protocol !== "https:" || apiURL.username || apiURL.password || apiURL.search || apiURL.hash) {
  throw new Error("INBOX_ACCEPT_API_URL must be an explicit credential-free HTTPS origin");
}
apiURL.pathname = apiURL.pathname.replace(/\/$/, "");

const apiKey = process.env.INBOX_ACCEPT_API_KEY;
if (apiKey.trim() !== apiKey || /[\x00-\x20\x7f]/.test(apiKey)) {
  throw new Error("INBOX_ACCEPT_API_KEY must be a canonical nonempty token");
}
const externalUserA = fixtureComponent("INBOX_ACCEPT_EXTERNAL_USER_A");
const externalUserB = fixtureComponent("INBOX_ACCEPT_EXTERNAL_USER_B");
const itemA = fixtureComponent("INBOX_ACCEPT_ITEM_A");
const itemB = fixtureComponent("INBOX_ACCEPT_ITEM_B");
assert.notEqual(externalUserA, externalUserB, "fixture managed users must be distinct");
assert.notEqual(itemA, itemB, "fixture Inbox items must be distinct");

const eventDatabaseURL = new URL(process.env.INBOX_ACCEPT_EVENT_DATABASE_URL);
if (!['postgres:', 'postgresql:'].includes(eventDatabaseURL.protocol)) {
  throw new Error("INBOX_ACCEPT_EVENT_DATABASE_URL must be an explicit PostgreSQL URL");
}
const inboxEventChannel = "inbox_events";
assert.match(inboxEventChannel, /^[a-z][a-z0-9_]{0,62}$/, "Inbox event channel must be canonical");

function fixtureComponent(name) {
  const value = process.env[name];
  if (value.trim() !== value || !/^[A-Za-z0-9._:@+-]{1,200}$/.test(value)) {
    throw new Error(`${name} must be a nonempty, canonical fixture identifier`);
  }
  return value;
}

function scopedQuery(mode, externalUserID) {
  const query = new URLSearchParams({ inbox_scope: mode });
  if (mode === "managed_user") query.set("external_user_id", externalUserID);
  return query;
}

function endpoint(pathname, query) {
  const target = new URL(apiURL);
  target.pathname = `${apiURL.pathname}${pathname}`;
  target.search = query?.toString() ?? "";
  return target;
}

async function api(method, pathname, query, body) {
  const response = await fetch(endpoint(pathname, query), {
    method,
    headers: {
      Authorization: `Bearer ${apiKey}`,
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    redirect: "error",
  });
  const raw = await response.text();
  let payload = null;
  if (raw !== "") {
    try {
      payload = JSON.parse(raw);
    } catch {
      throw new Error(`${method} ${pathname} returned non-JSON status ${response.status}`);
    }
  }
  return { status: response.status, payload };
}

function responseData(result, label) {
  assert.equal(result.status, 200, `${label} status`);
  assert.ok(result.payload && Object.hasOwn(result.payload, "data"), `${label} data envelope`);
  return result.payload.data;
}

function itemIDs(items) {
  assert.ok(Array.isArray(items), "Inbox list data must be an array");
  return new Set(items.map((item) => item?.id).filter((id) => typeof id === "string"));
}

async function assertNotFound(label, method, path, body) {
  const result = await api(method, path, scopedQuery("managed_user", externalUserA), body);
  assert.equal(result.status, 404, `${label} must hide cross-scope fixture existence`);
  assert.equal(result.payload?.error?.code, "NOT_FOUND", `${label} error code`);
}

function workspaceFromFixture(item, label) {
  const workspaceID = item?.workspace_id;
  if (typeof workspaceID !== "string" || workspaceID.trim() !== workspaceID || !/^[A-Za-z0-9._:@+-]{1,200}$/.test(workspaceID)) {
    throw new Error(`${label} did not return a canonical fixture workspace_id`);
  }
  return workspaceID;
}

function webSocketURL(mode, externalUserID) {
  const target = endpoint("/v1/inbox/ws", scopedQuery(mode, externalUserID));
  target.protocol = "wss:";
  return target;
}

const webSocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11";

class AcceptanceWebSocket {
  constructor(label, target) {
    this.label = label;
    this.target = target;
    this.buffer = Buffer.alloc(0);
    this.messages = [];
    this.waiters = new Set();
    this.socket = null;
  }

  async connect() {
    const key = randomBytes(16).toString("base64");
    const requester = this.target.protocol === "wss:" ? httpsRequest : httpRequest;
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error(`${this.label} WebSocket upgrade timed out`)), 10_000);
      const request = requester(this.target, {
        headers: {
          Authorization: `Bearer ${apiKey}`,
          Connection: "Upgrade",
          Upgrade: "websocket",
          "Sec-WebSocket-Key": key,
          "Sec-WebSocket-Version": "13",
        },
      });
      request.once("upgrade", (response, socket, head) => {
        clearTimeout(timer);
        const expected = createHash("sha1").update(key + webSocketGUID).digest("base64");
        if (response.statusCode !== 101 || response.headers["sec-websocket-accept"] !== expected) {
          socket.destroy();
          reject(new Error(`${this.label} WebSocket returned an invalid upgrade response`));
          return;
        }
        this.socket = socket;
        socket.on("data", (chunk) => this.consume(chunk));
        socket.on("error", (error) => this.rejectWaiters(error));
        socket.on("close", () => this.rejectWaiters(new Error(`${this.label} WebSocket closed`)));
        if (head.length > 0) this.consume(head);
        resolve();
      });
      request.once("response", (response) => {
        clearTimeout(timer);
        response.resume();
        reject(new Error(`${this.label} WebSocket upgrade failed with status ${response.statusCode}`));
      });
      request.once("error", (error) => {
        clearTimeout(timer);
        reject(error);
      });
      request.end();
    });
  }

  consume(chunk) {
    this.buffer = Buffer.concat([this.buffer, chunk]);
    while (this.buffer.length >= 2) {
      const first = this.buffer[0];
      const second = this.buffer[1];
      let length = second & 0x7f;
      let offset = 2;
      if (length === 126) {
        if (this.buffer.length < 4) return;
        length = this.buffer.readUInt16BE(2);
        offset = 4;
      } else if (length === 127) {
        if (this.buffer.length < 10) return;
        const longLength = this.buffer.readBigUInt64BE(2);
        if (longLength > BigInt(Number.MAX_SAFE_INTEGER)) throw new Error(`${this.label} WebSocket frame is too large`);
        length = Number(longLength);
        offset = 10;
      }
      const masked = (second & 0x80) !== 0;
      const maskLength = masked ? 4 : 0;
      if (this.buffer.length < offset + maskLength + length) return;
      const mask = masked ? this.buffer.subarray(offset, offset + 4) : null;
      offset += maskLength;
      const payload = Buffer.from(this.buffer.subarray(offset, offset + length));
      this.buffer = this.buffer.subarray(offset + length);
      if (mask) {
        for (let index = 0; index < payload.length; index += 1) payload[index] ^= mask[index % 4];
      }
      const opcode = first & 0x0f;
      if (opcode === 0x1) {
        const message = JSON.parse(payload.toString("utf8"));
        this.messages.push(message);
        this.resolveWaiters();
      } else if (opcode === 0x8) {
        this.socket?.destroy();
      } else if (opcode === 0x9) {
        this.socket?.write(maskedClientFrame(0xA, payload));
      }
    }
  }

  waitFor(predicate, timeoutMs = 10_000) {
    const existing = this.messages.find(predicate);
    if (existing) return Promise.resolve(existing);
    return new Promise((resolve, reject) => {
      const waiter = { predicate, resolve, reject, timer: null };
      waiter.timer = setTimeout(() => {
        this.waiters.delete(waiter);
        reject(new Error(`${this.label} WebSocket did not receive the expected fixture event`));
      }, timeoutMs);
      this.waiters.add(waiter);
    });
  }

  resolveWaiters() {
    for (const waiter of this.waiters) {
      const match = this.messages.find(waiter.predicate);
      if (!match) continue;
      clearTimeout(waiter.timer);
      this.waiters.delete(waiter);
      waiter.resolve(match);
    }
  }

  rejectWaiters(error) {
    for (const waiter of this.waiters) {
      clearTimeout(waiter.timer);
      waiter.reject(error);
    }
    this.waiters.clear();
  }

  close() {
    if (!this.socket || this.socket.destroyed) return;
    this.socket.write(maskedClientFrame(0x8, Buffer.alloc(0)));
    this.socket.destroy();
  }
}

function maskedClientFrame(opcode, payload) {
  assert.ok(payload.length < 126, "acceptance control frame must stay small");
  const mask = randomBytes(4);
  const frame = Buffer.alloc(2 + mask.length + payload.length);
  frame[0] = 0x80 | opcode;
  frame[1] = 0x80 | payload.length;
  mask.copy(frame, 2);
  for (let index = 0; index < payload.length; index += 1) {
    frame[6 + index] = payload[index] ^ mask[index % 4];
  }
  return frame;
}

async function emitFixtureEvent(workspaceID, externalUserID, probeID) {
  const payload = JSON.stringify({
    type: "inbox.acceptance_probe",
    workspace_id: workspaceID,
    external_user_id: externalUserID,
    probe_id: probeID,
  });
  assert.doesNotMatch(payload, /\$inbox_acceptance_payload\$/);
  const sql = `BEGIN READ ONLY;\nSELECT pg_notify('${inboxEventChannel}', $inbox_acceptance_payload$${payload}$inbox_acceptance_payload$);\nCOMMIT;\n`;

  await new Promise((resolve, reject) => {
    const child = spawn("psql", ["-X", "--no-psqlrc", "--set", "ON_ERROR_STOP=1"], {
      env: {
        ...process.env,
        PGDATABASE: process.env.INBOX_ACCEPT_EVENT_DATABASE_URL,
        INBOX_ACCEPT_EVENT_DATABASE_URL: "",
        INBOX_ACCEPT_API_KEY: "",
      },
      stdio: ["pipe", "pipe", "pipe"],
      shell: false,
    });
    child.stderr.resume();
    child.stdout.resume();
    child.once("error", (error) => {
      if (error.code === "ENOENT") reject(new Error("psql is required for the ephemeral WebSocket fixture event"));
      else reject(error);
    });
    child.once("close", (code) => {
      if (code === 0) resolve();
      else reject(new Error(`psql fixture event failed with exit ${code}; connection details were suppressed`));
    });
    child.stdin.end(sql);
  });
}

const managedAQuery = scopedQuery("managed_user", externalUserA);
const managedBQuery = scopedQuery("managed_user", externalUserB);
const workspaceQuery = scopedQuery("workspace");

const [listAResult, listBResult, workspaceListResult, fixtureAResult, fixtureBResult] = await Promise.all([
  api("GET", "/v1/inbox", managedAQuery),
  api("GET", "/v1/inbox", managedBQuery),
  api("GET", "/v1/inbox", workspaceQuery),
  api("GET", `/v1/inbox/${encodeURIComponent(itemA)}`, workspaceQuery),
  api("GET", `/v1/inbox/${encodeURIComponent(itemB)}`, workspaceQuery),
]);

const listA = itemIDs(responseData(listAResult, "managed user A list"));
const listB = itemIDs(responseData(listBResult, "managed user B list"));
const workspaceList = itemIDs(responseData(workspaceListResult, "owner/admin workspace list"));
assert.ok(listA.has(itemA), "managed user A list must contain fixture A");
assert.ok(!listA.has(itemB), "managed user A list must exclude fixture B");
assert.ok(listB.has(itemB), "managed user B list must contain fixture B");
assert.ok(!listB.has(itemA), "managed user B list must exclude fixture A");
assert.ok(workspaceList.has(itemA) && workspaceList.has(itemB), "owner/admin workspace list must contain both fixtures");

const fixtureA = responseData(fixtureAResult, "workspace fixture A get");
const fixtureB = responseData(fixtureBResult, "workspace fixture B get");
assert.equal(fixtureA.id, itemA, "fixture A response must match the requested item");
assert.equal(fixtureB.id, itemB, "fixture B response must match the requested item");
const workspaceID = workspaceFromFixture(fixtureA, "fixture A");
assert.equal(workspaceFromFixture(fixtureB, "fixture B"), workspaceID, "fixtures must belong to one workspace");
assert.equal(fixtureB.is_read, true, "fixture B must already be read before the cross-scope read probe");
assert.match(fixtureB.thread_status, /^(open|assigned|resolved)$/, "fixture B must expose its current thread state");

await assertNotFound("cross-scope get", "GET", `/v1/inbox/${encodeURIComponent(itemB)}`);
await assertNotFound("cross-scope read", "POST", `/v1/inbox/${encodeURIComponent(itemB)}/read`);
await assertNotFound("cross-scope reply", "POST", `/v1/inbox/${encodeURIComponent(itemB)}/reply`, {
  text: "UniPost synthetic isolation probe. The fixture adapter must reject delivery.",
});
await assertNotFound("cross-scope thread-state", "POST", `/v1/inbox/${encodeURIComponent(itemB)}/thread-state`, {
  thread_status: fixtureB.thread_status,
  assigned_to: fixtureB.assigned_to ?? "",
});

const missingScope = await api("GET", "/v1/inbox");
assert.equal(missingScope.status, 400, "missing Inbox scope status");
assert.equal(missingScope.payload?.error?.code, "INBOX_SCOPE_REQUIRED", "missing Inbox scope error code");

const managedSocket = new AcceptanceWebSocket("managed user A", webSocketURL("managed_user", externalUserA));
const aggregateSocket = new AcceptanceWebSocket("owner/admin aggregate", webSocketURL("workspace"));
try {
  await Promise.all([managedSocket.connect(), aggregateSocket.connect()]);
  await new Promise((resolve) => setTimeout(resolve, 150));
  const probeA = `accept-a-${randomBytes(8).toString("hex")}`;
  const probeB = `accept-b-${randomBytes(8).toString("hex")}`;
  await emitFixtureEvent(workspaceID, externalUserA, probeA);
  await emitFixtureEvent(workspaceID, externalUserB, probeB);
  const isProbe = (probeID) => (message) => message?.type === "inbox.acceptance_probe" && message?.probe_id === probeID;
  await Promise.all([
    managedSocket.waitFor(isProbe(probeA)),
    aggregateSocket.waitFor(isProbe(probeA)),
    aggregateSocket.waitFor(isProbe(probeB)),
  ]);
  await new Promise((resolve) => setTimeout(resolve, 500));
  assert.ok(!managedSocket.messages.some(isProbe(probeB)), "managed user A WebSocket must exclude fixture B events");
} finally {
  managedSocket.close();
  aggregateSocket.close();
}

console.log("Inbox scope acceptance passed for UniPost-owned synthetic fixtures.");
