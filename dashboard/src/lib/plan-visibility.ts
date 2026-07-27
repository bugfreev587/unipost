export function isPlanVisibleToNewUsers(planId: string): boolean {
  return planId !== "api";
}

export function isPlanVisibleInBilling(planId: string, currentPlanId?: string | null): boolean {
  return isPlanVisibleToNewUsers(planId) || planId === currentPlanId;
}
