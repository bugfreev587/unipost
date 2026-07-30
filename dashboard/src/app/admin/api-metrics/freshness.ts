import type { MetricsFreshness } from "@/lib/api";

export function combineMetricsFreshness(
  values: Array<MetricsFreshness | undefined>,
): MetricsFreshness | null {
  const reported = values.filter((value): value is MetricsFreshness => Boolean(value));
  if (reported.length === 0) return null;
  const delayed = reported.some((value) => value.data_state === "delayed");
  const approximate = reported.some(
    (value) => value.data_state === "approximate" || value.percentiles_approximate,
  );
  return {
    data_state: delayed ? "delayed" : approximate ? "approximate" : "exact",
    percentiles_approximate: reported.some((value) => value.percentiles_approximate),
    missing_rollup_hours: Math.max(
      0,
      ...reported.map((value) => value.missing_rollup_hours ?? 0),
    ),
  };
}
