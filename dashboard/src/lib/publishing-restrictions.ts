export const PLAN_PLATFORM_PUBLISHING_RESTRICTED_CODE =
  "plan_platform_publishing_restricted";

export const PLAN_PLATFORM_PUBLISHING_RESTRICTED_MESSAGE =
  "TikTok publishing is temporarily unavailable on the Free plan due to platform capacity limits. We’re working with TikTok to increase capacity. Upgrade your plan or try again after the restriction is lifted.";

export interface PublishingRestrictionProjection {
  restricted: boolean;
  platform: string;
  plan_id: string;
  reason_code?: string;
  code?: string;
  message?: string;
  next_action?: string;
}

type AccountPlatform = { id: string; platform: string };

type PublishingRestrictionError = {
  code?: string;
  rawCode?: string;
};

type PublishingRestrictionResult = {
	platform?: string;
	status?: string;
	error_code?: string;
};

type PublishingRestrictionRetryInput = {
  retry_policy?: {
    manual_retry_allowed: boolean;
    reason?: string;
  };
  media_retained_until?: string;
};

export type PublishingRestrictionRetryDescription =
  | { state: "blocked"; label: "Publishing restriction active" }
  | { state: "ready"; label: "Retry" }
  | { state: "expired"; label: "Upload media again to retry" };

function normalizedCode(value: string | undefined): string {
  return (value || "").trim().toLowerCase();
}

export function isPlanPlatformPublishingRestrictedError(
  error: PublishingRestrictionError | null | undefined,
): boolean {
  return [error?.code, error?.rawCode]
    .map(normalizedCode)
    .includes(PLAN_PLATFORM_PUBLISHING_RESTRICTED_CODE);
}

export function findPublishingRestrictionResult<T extends PublishingRestrictionResult>(
	results: T[] | undefined,
): T | undefined {
	return results?.find(
		(result) =>
			result.status === "failed" &&
			normalizedCode(result.error_code) === PLAN_PLATFORM_PUBLISHING_RESTRICTED_CODE,
	);
}

export function isPlatformRestricted(
  restrictions: PublishingRestrictionProjection[],
  platform: string,
  planId?: string,
): boolean {
  const normalizedPlatform = platform.trim().toLowerCase();
  const normalizedPlan = planId?.trim().toLowerCase();
  return restrictions.some((restriction) => {
    if (!restriction.restricted || restriction.platform.trim().toLowerCase() !== normalizedPlatform) {
      return false;
    }
    return !normalizedPlan || restriction.plan_id.trim().toLowerCase() === normalizedPlan;
  });
}

export function filterAllowedAccountIds(
  accounts: AccountPlatform[],
  accountIds: Iterable<string>,
  restrictions: PublishingRestrictionProjection[],
): string[] {
  const byId = new Map(accounts.map((account) => [account.id, account]));
  return Array.from(accountIds).filter((id) => {
    const account = byId.get(id);
    return account && !isPlatformRestricted(restrictions, account.platform);
  });
}

export function describePublishingRestrictionRetry(
  result: PublishingRestrictionRetryInput,
  now = new Date(),
): PublishingRestrictionRetryDescription {
  const deadline = result.media_retained_until
    ? new Date(result.media_retained_until)
    : null;
  const deadlineExpired = Boolean(
    deadline && !Number.isNaN(deadline.getTime()) && deadline.getTime() <= now.getTime(),
  );

  if (result.retry_policy?.reason === "media_reupload_required" || deadlineExpired) {
    return { state: "expired", label: "Upload media again to retry" };
  }
  if (result.retry_policy?.manual_retry_allowed) {
    return { state: "ready", label: "Retry" };
  }
  return { state: "blocked", label: "Publishing restriction active" };
}
