export function formatPostLimit(limit: number | null | undefined): string {
  if (typeof limit === "number" && limit < 0) {
    return "Unlimited";
  }
  return (limit ?? 0).toLocaleString();
}

export function formatPlanPostAllowance(limit: number | null | undefined): string {
  return `${formatPostLimit(limit)} posts`;
}

export function formatPostUsage(used: number | null | undefined, limit: number | null | undefined): string {
  return `${(used ?? 0).toLocaleString()} / ${formatPostLimit(limit)} posts`;
}

export function usagePercentage(used: number | null | undefined, limit: number | null | undefined): number {
  if (typeof limit !== "number" || limit <= 0) {
    return 0;
  }
  return Math.min(100, ((used ?? 0) / limit) * 100);
}
