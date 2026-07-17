export const X_INBOX_OUTBOUND_STORAGE_KEY = "unipost:x-inbox-outbound:v1";

export type XInboxClientOutboundStatus =
  | "sending"
  | "outcome_unknown"
  | "remote_succeeded"
  | "usage_reversal_pending"
  | "needs_reconciliation";

export type XInboxOutboundLogicalInput = {
  workspaceId: string;
  accountId: string;
  source: "x_reply" | "x_dm";
  targetItemId: string;
  threadKey: string;
  bodyHash: string;
};

export type XInboxClientOutboundOperation = XInboxOutboundLogicalInput & {
  version: 1;
  logicalKey: string;
  idempotencyKey: string;
  operationId?: string;
  status: XInboxClientOutboundStatus;
  createdAt: string;
  updatedAt: string;
};

type StorageLike = Pick<Storage, "getItem" | "setItem" | "removeItem">;

function encoded(value: string): string {
  return encodeURIComponent(value.trim());
}

export function xInboxOutboundLogicalKey(input: XInboxOutboundLogicalInput): string {
  return [
    input.workspaceId,
    input.accountId,
    input.source,
    input.targetItemId,
    input.threadKey,
    input.bodyHash,
  ].map(encoded).join(":");
}

export function beginXInboxOutboundOperation(
  operations: XInboxClientOutboundOperation[],
  input: XInboxOutboundLogicalInput,
  createIdempotencyKey: () => string,
  now = new Date().toISOString(),
): {
  operation: XInboxClientOutboundOperation;
  operations: XInboxClientOutboundOperation[];
  reused: boolean;
} {
  const logicalKey = xInboxOutboundLogicalKey(input);
  const existing = operations.find((operation) => operation.logicalKey === logicalKey);
  if (existing) {
    return { operation: existing, operations, reused: true };
  }
  const operation: XInboxClientOutboundOperation = {
    version: 1,
    ...input,
    logicalKey,
    idempotencyKey: createIdempotencyKey(),
    status: "sending",
    createdAt: now,
    updatedAt: now,
  };
  return { operation, operations: [...operations, operation], reused: false };
}

export function updateXInboxOutboundOperation(
  operations: XInboxClientOutboundOperation[],
  logicalKey: string,
  update: Partial<Pick<XInboxClientOutboundOperation, "status" | "operationId">>,
  now = new Date().toISOString(),
): XInboxClientOutboundOperation[] {
  return operations.map((operation) =>
    operation.logicalKey === logicalKey
      ? { ...operation, ...update, updatedAt: now }
      : operation,
  );
}

export function resolveXInboxOutboundOperation(
  operations: XInboxClientOutboundOperation[],
  logicalKey: string,
): XInboxClientOutboundOperation[] {
  return operations.filter((operation) => operation.logicalKey !== logicalKey);
}

function isStoredOperation(value: unknown): value is XInboxClientOutboundOperation {
  if (!value || typeof value !== "object") return false;
  const operation = value as Partial<XInboxClientOutboundOperation>;
  return operation.version === 1 &&
    typeof operation.workspaceId === "string" &&
    typeof operation.accountId === "string" &&
    (operation.source === "x_reply" || operation.source === "x_dm") &&
    typeof operation.targetItemId === "string" &&
    typeof operation.threadKey === "string" &&
    typeof operation.bodyHash === "string" &&
    typeof operation.logicalKey === "string" &&
    typeof operation.idempotencyKey === "string" &&
    typeof operation.status === "string" &&
    typeof operation.createdAt === "string" &&
    typeof operation.updatedAt === "string";
}

export function loadXInboxOutboundOperations(
  storage: StorageLike,
  workspaceId?: string,
): XInboxClientOutboundOperation[] {
  try {
    const raw = storage.getItem(X_INBOX_OUTBOUND_STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const operations = parsed.filter(isStoredOperation);
    return workspaceId
      ? operations.filter((operation) => operation.workspaceId === workspaceId)
      : operations;
  } catch {
    return [];
  }
}

export function saveXInboxOutboundOperations(
  storage: StorageLike,
  operations: XInboxClientOutboundOperation[],
): void {
  if (operations.length === 0) {
    storage.removeItem(X_INBOX_OUTBOUND_STORAGE_KEY);
    return;
  }
  storage.setItem(X_INBOX_OUTBOUND_STORAGE_KEY, JSON.stringify(operations));
}

export async function hashXInboxReplyBody(body: string): Promise<string> {
  const bytes = new TextEncoder().encode(body);
  const digest = await globalThis.crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((byte) => byte.toString(16).padStart(2, "0"))
    .join("");
}

export function classifyXInboxOutboundStatus(status: string): {
  terminal: boolean;
  manual: boolean;
} {
  if (status === "completed" || status === "succeeded") {
    return { terminal: true, manual: false };
  }
  return {
    terminal: false,
    manual: status === "needs_reconciliation",
  };
}
