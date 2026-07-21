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

const httpTimeoutMs = timeoutSetting("INBOX_ACCEPT_HTTP_TIMEOUT_MS", 10_000, 1_000, 60_000);
const wsUpgradeTimeoutMs = timeoutSetting("INBOX_ACCEPT_WS_UPGRADE_TIMEOUT_MS", 10_000, 1_000, 60_000);
const wsEventTimeoutMs = timeoutSetting("INBOX_ACCEPT_WS_EVENT_TIMEOUT_MS", 5_000, 500, 60_000);
const wsReadyTimeoutMs = timeoutSetting("INBOX_ACCEPT_WS_READY_TIMEOUT_MS", 20_000, 2_000, 120_000);
const psqlTimeoutMs = timeoutSetting("INBOX_ACCEPT_PSQL_TIMEOUT_MS", 10_000, 1_000, 60_000);
const psqlKillGraceMs = timeoutSetting("INBOX_ACCEPT_PSQL_KILL_GRACE_MS", 1_000, 100, 5_000);

function timeoutSetting(name, defaultValue, minimum, maximum) {
  const raw = process.env[name];
  if (raw === undefined || raw === "") return defaultValue;
  if (!/^\d+$/.test(raw)) {
    throw new Error(`${name} must be an integer from ${minimum} through ${maximum}`);
  }
  const value = Number(raw);
  if (!Number.isSafeInteger(value) || value < minimum || value > maximum) {
    throw new Error(`${name} must be an integer from ${minimum} through ${maximum}`);
  }
  return value;
}

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
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), httpTimeoutMs);
  try {
    const response = await fetch(endpoint(pathname, query), {
      method,
      headers: {
        Authorization: `Bearer ${apiKey}`,
        ...(body === undefined ? {} : { "Content-Type": "application/json" }),
      },
      body: body === undefined ? undefined : JSON.stringify(body),
      redirect: "error",
      signal: controller.signal,
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
  } catch (error) {
    if (controller.signal.aborted) {
      throw new Error(`${method} ${pathname} timed out after ${httpTimeoutMs}ms`, { cause: error });
    }
    throw error;
  } finally {
    clearTimeout(timer);
  }
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
    this.upgradeTimeoutMs = wsUpgradeTimeoutMs;
    this.eventTimeoutMs = wsEventTimeoutMs;
    this.buffer = Buffer.alloc(0);
    this.messages = [];
    this.waiters = new Set();
    this.socket = null;
    this.socketHandlers = null;
  }

  async connect() {
    const key = randomBytes(16).toString("base64");
    const requester = this.target.protocol === "wss:" ? httpsRequest : httpRequest;
    await new Promise((resolve, reject) => {
      const request = requester(this.target, {
        headers: {
          Authorization: `Bearer ${apiKey}`,
          Connection: "Upgrade",
          Upgrade: "websocket",
          "Sec-WebSocket-Key": key,
          "Sec-WebSocket-Version": "13",
        },
      });
      let timer = null;
      let settled = false;
      let upgradedSocket = null;
      const cleanup = () => {
        if (timer !== null) clearTimeout(timer);
        request.off("upgrade", onUpgrade);
        request.off("response", onResponse);
        request.off("error", onError);
      };
      const finish = (callback, value) => {
        if (settled) return;
        settled = true;
        cleanup();
        callback(value);
      };
      const onUpgrade = (response, socket, head) => {
        upgradedSocket = socket;
        const expected = createHash("sha1").update(key + webSocketGUID).digest("base64");
        if (response.statusCode !== 101 || response.headers["sec-websocket-accept"] !== expected) {
          socket.destroy();
          finish(reject, new Error(`${this.label} WebSocket returned an invalid upgrade response`));
          return;
        }
        this.socket = socket;
        const onData = (chunk) => {
          try {
            this.consume(chunk);
          } catch (error) {
            this.rejectWaiters(error);
            socket.destroy();
          }
        };
        const onSocketError = (error) => this.rejectWaiters(error);
        const onSocketClose = () => {
          this.rejectWaiters(new Error(`${this.label} WebSocket closed`));
          this.detachSocket(socket);
        };
        this.socketHandlers = { onData, onSocketError, onSocketClose };
        socket.on("data", onData);
        socket.on("error", onSocketError);
        socket.on("close", onSocketClose);
        try {
          if (head.length > 0) this.consume(head);
        } catch (error) {
          socket.destroy();
          finish(reject, error);
          return;
        }
        finish(resolve);
      };
      const onResponse = (response) => {
        response.resume();
        request.destroy();
        finish(reject, new Error(`${this.label} WebSocket upgrade failed with status ${response.statusCode}`));
      };
      const onError = (error) => {
        upgradedSocket?.destroy();
        finish(reject, error);
      };
      request.once("upgrade", onUpgrade);
      request.once("response", onResponse);
      request.once("error", onError);
      timer = setTimeout(() => {
        const error = new Error(`${this.label} WebSocket upgrade timed out after ${this.upgradeTimeoutMs}ms`);
        request.destroy();
        upgradedSocket?.destroy();
        finish(reject, error);
      }, this.upgradeTimeoutMs);
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

  waitFor(predicate, timeoutMs = this.eventTimeoutMs) {
    assert.ok(Number.isFinite(timeoutMs) && timeoutMs > 0, "WebSocket waiter timeout must be finite and positive");
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

  detachSocket(socket) {
    if (this.socket !== socket || !this.socketHandlers) return;
    socket.off("data", this.socketHandlers.onData);
    socket.off("error", this.socketHandlers.onSocketError);
    socket.off("close", this.socketHandlers.onSocketClose);
    this.socketHandlers = null;
    this.socket = null;
  }

  close() {
    const socket = this.socket;
    this.rejectWaiters(new Error(`${this.label} WebSocket closed by acceptance cleanup`));
    if (!socket) return;
    this.detachSocket(socket);
    if (socket.destroyed) return;
    try {
      socket.write(maskedClientFrame(0x8, Buffer.alloc(0)));
    } catch {
    } finally {
      socket.destroy();
    }
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

const fixtureEventType = "inbox.acceptance_probe";

function fixtureEvent(workspaceID, externalUserID, probeID) {
  for (const [label, value] of [
    ["workspace_id", workspaceID],
    ["external_user_id", externalUserID],
    ["probe_id", probeID],
  ]) {
    if (typeof value !== "string" || !/^[A-Za-z0-9._:@+-]{1,200}$/.test(value)) {
      throw new Error(`${label} must be a canonical fixture event component`);
    }
  }
  return {
    type: fixtureEventType,
    workspace_id: workspaceID,
    external_user_id: externalUserID,
    probe_id: probeID,
  };
}

function fixtureEventMatcher(expected) {
  return (message) => message?.type === expected.type &&
    message?.probe_id === expected.probe_id &&
    message?.workspace_id === expected.workspace_id &&
    message?.external_user_id === expected.external_user_id;
}

async function emitFixtureEvents(events) {
  assert.ok(Array.isArray(events) && events.length > 0 && events.length <= 10, "fixture event batch must contain 1 through 10 events");
  const statements = events.map((event, index) => {
    const payload = JSON.stringify(event);
    assert.ok(Buffer.byteLength(payload, "utf8") < 7_900, "fixture event must fit PostgreSQL NOTIFY limits");
    const delimiter = `$inbox_acceptance_payload_${index}$`;
    assert.doesNotMatch(payload, new RegExp(delimiter.replaceAll("$", "\\$")));
    return `SELECT pg_notify('${inboxEventChannel}', ${delimiter}${payload}${delimiter});`;
  });
  const sql = `BEGIN READ ONLY;\n${statements.join("\n")}\nCOMMIT;\n`;
  const childEnvironment = {
    PGAPPNAME: "unipost-inbox-scope-acceptance",
    PGCONNECT_TIMEOUT: String(Math.max(1, Math.ceil(psqlTimeoutMs / 1_000))),
    PGDATABASE: process.env.INBOX_ACCEPT_EVENT_DATABASE_URL,
  };
  if (process.env.PATH) childEnvironment.PATH = process.env.PATH;

  await new Promise((resolve, reject) => {
    const child = spawn("psql", ["-X", "--no-psqlrc", "--set", "ON_ERROR_STOP=1"], {
      env: childEnvironment,
      stdio: ["pipe", "pipe", "pipe"],
      shell: false,
    });
    let processError = null;
    let inputError = null;
    let timedOut = false;
    let settled = false;
    let killTimer = null;
    const runTimer = setTimeout(() => {
      timedOut = true;
      child.kill("SIGTERM");
      killTimer = setTimeout(() => {
        if (child.exitCode === null && child.signalCode === null) child.kill("SIGKILL");
        child.stdin.destroy();
        child.stdout.destroy();
        child.stderr.destroy();
        child.unref();
        finish(new Error(`psql fixture event timed out after ${psqlTimeoutMs}ms`));
      }, psqlKillGraceMs);
    }, psqlTimeoutMs);
    const cleanup = () => {
      clearTimeout(runTimer);
      if (killTimer !== null) clearTimeout(killTimer);
      child.off("error", onError);
      child.off("close", onClose);
      child.stdin.off("error", onInputError);
    };
    const finish = (error) => {
      if (settled) return;
      settled = true;
      cleanup();
      if (error) reject(error);
      else resolve();
    };
    const onError = (error) => {
      processError = error;
    };
    const onInputError = (error) => {
      inputError = error;
    };
    const onClose = (code, signal) => {
      if (timedOut) {
        finish(new Error(`psql fixture event timed out after ${psqlTimeoutMs}ms`));
      } else if (processError?.code === "ENOENT") {
        finish(new Error("psql is required for the ephemeral WebSocket fixture event"));
      } else if (processError || inputError) {
        finish(new Error("psql fixture event process failed; connection details were suppressed"));
      } else if (code === 0) {
        finish();
      } else {
        finish(new Error(`psql fixture event failed with exit ${code ?? "none"} and signal ${signal ?? "none"}; connection details were suppressed`));
      }
    };
    child.once("error", onError);
    child.once("close", onClose);
    child.stdin.once("error", onInputError);
    child.stderr.resume();
    child.stdout.resume();
    child.stdin.end(sql);
  });
}

function requireFulfilled(label, results) {
  const rejected = results.find((result) => result.status === "rejected");
  if (!rejected) return;
  const detail = rejected.reason instanceof Error ? rejected.reason.message : "unknown event waiter failure";
  throw new Error(`${label}: ${detail}`);
}

async function establishWebSocketReadiness(managedSocket, aggregateSocket, workspaceID) {
  const deadline = Date.now() + wsReadyTimeoutMs;
  let attempts = 0;
  while (Date.now() < deadline) {
    attempts += 1;
    const readinessEvent = fixtureEvent(
      workspaceID,
      externalUserA,
      `ready-a-${attempts}-${randomBytes(8).toString("hex")}`,
    );
    await emitFixtureEvents([readinessEvent]);
    const remainingMs = deadline - Date.now();
    if (remainingMs <= 0) break;
    const attemptTimeoutMs = Math.max(1, Math.min(wsEventTimeoutMs, remainingMs));
    const matchesReadiness = fixtureEventMatcher(readinessEvent);
    const results = await Promise.allSettled([
      managedSocket.waitFor(matchesReadiness, attemptTimeoutMs),
      aggregateSocket.waitFor(matchesReadiness, attemptTimeoutMs),
    ]);
    if (results.every((result) => result.status === "fulfilled")) return;
  }
  throw new Error(`WebSocket registration readiness was not observed within ${wsReadyTimeoutMs}ms`);
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
await assertNotFound("cross-scope reply", "POST", `/v1/inbox/${encodeURIComponent(itemB)}/reply`, {});
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
  await establishWebSocketReadiness(managedSocket, aggregateSocket, workspaceID);
  const probeB = `accept-b-${randomBytes(8).toString("hex")}`;
  const barrierA = `barrier-a-${randomBytes(8).toString("hex")}`;
  const orderedIsolationEvents = [
    fixtureEvent(workspaceID, externalUserB, probeB),
    fixtureEvent(workspaceID, externalUserA, barrierA),
  ];
  const [eventB, barrierEventA] = orderedIsolationEvents;
  const matchesB = fixtureEventMatcher(eventB);
  const matchesBarrierA = fixtureEventMatcher(barrierEventA);
  await emitFixtureEvents(orderedIsolationEvents);
  const isolationResults = await Promise.allSettled([
    aggregateSocket.waitFor(matchesB),
    aggregateSocket.waitFor(matchesBarrierA),
    managedSocket.waitFor(matchesBarrierA),
  ]);
  requireFulfilled("WebSocket causal isolation barrier failed", isolationResults);

  const aggregateBIndex = aggregateSocket.messages.findIndex(matchesB);
  const aggregateBarrierIndex = aggregateSocket.messages.findIndex(matchesBarrierA);
  assert.ok(aggregateBIndex >= 0 && aggregateBarrierIndex >= 0, "owner/admin aggregate must receive both ordered isolation events");
  assert.ok(aggregateBIndex < aggregateBarrierIndex, "owner/admin aggregate must observe B before the A barrier");

  const managedBarrierIndex = managedSocket.messages.findIndex(matchesBarrierA);
  assert.ok(managedBarrierIndex >= 0, "managed user A must observe its causal barrier");
  assert.equal(
    managedSocket.messages.slice(0, managedBarrierIndex + 1).some(matchesB),
    false,
    "managed user A must not receive the exact B probe before its causal barrier",
  );
} finally {
  managedSocket.close();
  aggregateSocket.close();
}

console.log("Inbox scope acceptance passed for UniPost-owned synthetic fixtures.");
