type InboxUnreadGateInput = {
  profileId?: string | null;
  inboxFeatureEnabled: boolean;
  planAllowsInbox: boolean | null;
};

export function shouldLoadGlobalInboxUnreadCount({
  profileId,
  inboxFeatureEnabled,
  planAllowsInbox,
}: InboxUnreadGateInput): boolean {
  return Boolean(profileId) && inboxFeatureEnabled && planAllowsInbox === true;
}
