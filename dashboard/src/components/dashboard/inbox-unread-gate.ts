type InboxUnreadGateInput = {
  profileId?: string | null;
  planAllowsInbox: boolean | null;
};

export function shouldLoadGlobalInboxUnreadCount({
  profileId,
  planAllowsInbox,
}: InboxUnreadGateInput): boolean {
  return Boolean(profileId) && planAllowsInbox === true;
}
