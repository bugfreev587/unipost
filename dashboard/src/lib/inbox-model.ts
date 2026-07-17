export type InboxSource =
  | "ig_comment"
  | "ig_dm"
  | "threads_reply"
  | "youtube_comment"
  | "fb_comment"
  | "fb_dm"
  | "x_reply"
  | "x_dm";

export type InboxSourceKind = "public_comment" | "private_message";
export type InboxSourceTab = "comments" | "dms" | "threads";

export type InboxSourceDefinition = {
  source: InboxSource;
  platform: string;
  kind: InboxSourceKind;
  tab: InboxSourceTab;
  label: string;
  shortLabel: string;
  private: boolean;
};

const DEFINITIONS: Record<InboxSource, InboxSourceDefinition> = {
  ig_comment: {
    source: "ig_comment",
    platform: "instagram",
    kind: "public_comment",
    tab: "comments",
    label: "Instagram comment",
    shortLabel: "Comment",
    private: false,
  },
  ig_dm: {
    source: "ig_dm",
    platform: "instagram",
    kind: "private_message",
    tab: "dms",
    label: "Instagram DM",
    shortLabel: "DM",
    private: true,
  },
  threads_reply: {
    source: "threads_reply",
    platform: "threads",
    kind: "public_comment",
    tab: "threads",
    label: "Threads reply",
    shortLabel: "Reply",
    private: false,
  },
  youtube_comment: {
    source: "youtube_comment",
    platform: "youtube",
    kind: "public_comment",
    tab: "comments",
    label: "YouTube comment",
    shortLabel: "Comment",
    private: false,
  },
  fb_comment: {
    source: "fb_comment",
    platform: "facebook",
    kind: "public_comment",
    tab: "comments",
    label: "Facebook comment",
    shortLabel: "Comment",
    private: false,
  },
  fb_dm: {
    source: "fb_dm",
    platform: "facebook",
    kind: "private_message",
    tab: "dms",
    label: "Messenger DM",
    shortLabel: "DM",
    private: true,
  },
  x_reply: {
    source: "x_reply",
    platform: "twitter",
    kind: "public_comment",
    tab: "comments",
    label: "X comment",
    shortLabel: "Comment",
    private: false,
  },
  x_dm: {
    source: "x_dm",
    platform: "twitter",
    kind: "private_message",
    tab: "dms",
    label: "X DM",
    shortLabel: "DM",
    private: true,
  },
};

export function getInboxSourceDefinition(source: string): InboxSourceDefinition {
  return DEFINITIONS[source as InboxSource] || {
    source: source as InboxSource,
    platform: "instagram",
    kind: "public_comment",
    tab: "comments",
    label: source,
    shortLabel: source,
    private: false,
  };
}

export function isKnownInboxSource(source: unknown): source is InboxSource {
  return typeof source === "string" && Object.prototype.hasOwnProperty.call(DEFINITIONS, source);
}

export function isInboxDMSource(source?: string | null): boolean {
  return source ? getInboxSourceDefinition(source).kind === "private_message" : false;
}

export type InboxConversationItem = {
  id: string;
  social_account_id: string;
  source: string;
  external_id: string;
  thread_key?: string;
  parent_external_id?: string;
  author_id?: string;
  linked_post_id?: string;
  received_at: string;
};

function commentPostRootKey<T extends InboxConversationItem>(
  item: T,
  items: T[],
): string {
  const byExternalID = new Map(items.map((candidate) => [candidate.external_id, candidate]));
  let current = item;
  const seen = new Set<string>();
  while (true) {
    if (seen.has(current.external_id)) return current.external_id;
    seen.add(current.external_id);
    const parentID = current.parent_external_id;
    if (!parentID) return current.external_id;
    const parent = byExternalID.get(parentID);
    if (!parent) return parentID;
    current = parent;
  }
}

export function canonicalInboxConversationKey<T extends InboxConversationItem>(
  item: T,
  sourceItems: T[],
): string {
  const definition = getInboxSourceDefinition(item.source);
  let root: string;
  if (definition.private || item.source === "x_reply") {
    root =
      item.thread_key ||
      item.parent_external_id ||
      item.author_id ||
      item.external_id;
  } else if (item.linked_post_id) {
    root = `post:${item.linked_post_id}`;
  } else {
    root = `post-ext:${commentPostRootKey(item, sourceItems)}`;
  }
  return `${item.social_account_id}:${item.source}:${root}`;
}

export function groupInboxItemsByConversation<T extends InboxConversationItem>(
  items: T[],
  source: string,
): Array<{ id: string; threadKey: string; items: T[] }> {
  const sourceItems = items.filter((item) => item.source === source);
  const grouped = new Map<string, T[]>();
  for (const item of sourceItems) {
    const key = canonicalInboxConversationKey(item, sourceItems);
    const existing = grouped.get(key) || [];
    existing.push(item);
    grouped.set(key, existing);
  }
  return Array.from(grouped.entries()).map(([id, groupedItems]) => {
    const sorted = [...groupedItems].sort(
      (a, b) => Date.parse(a.received_at) - Date.parse(b.received_at),
    );
    return {
      id,
      threadKey:
        sorted[0]?.thread_key ||
        sorted[0]?.parent_external_id ||
        sorted[0]?.external_id ||
        id,
      items: sorted,
    };
  });
}
