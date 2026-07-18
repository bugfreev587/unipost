export type AttemptedDateRange = {
  start_at?: string;
  end_at?: string;
  error?: string;
};

function localMidnight(date: string, dayOffset = 0): Date {
  const [year, month, day] = date.split("-").map(Number);
  return new Date(year, month - 1, day + dayOffset);
}

export function buildAttemptedDateRange(
  startDate: string,
  endDate: string,
): AttemptedDateRange {
  if (startDate && endDate && endDate < startDate) {
    return { error: "End date must be on or after start date." };
  }

  const range: AttemptedDateRange = {};
  if (startDate) {
    range.start_at = localMidnight(startDate).toISOString();
  }
  if (endDate) {
    const endExclusive = localMidnight(endDate, 1);
    range.end_at = endExclusive.toISOString();
  }
  return range;
}
