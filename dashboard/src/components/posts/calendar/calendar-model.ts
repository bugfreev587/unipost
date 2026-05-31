export type CalendarStatusFilter =
  | "all"
  | "published"
  | "scheduled"
  | "in_progress"
  | "failed"
  | "draft"
  | "cancelled"
  | "archived";

export type CalendarStatusGroup = Exclude<CalendarStatusFilter, "all">;
export type CalendarViewMode = "day" | "week" | "month";

export type CalendarModelPost = {
  status: string;
  scheduled_at?: string | null;
  published_at?: string | null;
  created_at?: string | null;
  archived_at?: string | null;
};

const CALENDAR_VIEW_MODES = new Set<CalendarViewMode>(["day", "week", "month"]);

export type CalendarModelProfile = {
  id: string;
  name: string;
  branding_primary_color?: string | null;
};

export type CalendarDayCell = {
  date: Date;
  dateKey: string;
  dayOfMonth: number;
  isCurrentMonth: boolean;
  isToday: boolean;
};

const IN_PROGRESS_STATUSES = new Set(["queued", "dispatching", "retrying", "processing"]);
const FAILED_STATUSES = new Set(["failed", "partial"]);

const PROFILE_COLOR_PALETTE = [
  "#ff453a",
  "#ff9f0a",
  "#ffd60a",
  "#32d74b",
  "#64d2ff",
  "#0a84ff",
  "#5e5ce6",
  "#bf5af2",
  "#ff375f",
  "#ac8e68",
];

export function buildMonthGrid(monthDate: Date, today = new Date()): CalendarDayCell[] {
  const monthStart = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
  const monthEnd = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 0);
  const gridStart = addDays(monthStart, -monthStart.getDay());
  const todayKey = formatLocalDateKey(today);
  const cells: CalendarDayCell[] = [];

  for (let offset = 0; offset < 42; offset += 1) {
    const date = addDays(gridStart, offset);
    cells.push({
      date,
      dateKey: formatLocalDateKey(date),
      dayOfMonth: date.getDate(),
      isCurrentMonth: date >= monthStart && date <= monthEnd,
      isToday: formatLocalDateKey(date) === todayKey,
    });
  }

  return cells;
}

export function buildWeekDays(anchorDate: Date, today = new Date()): CalendarDayCell[] {
  const weekStart = startOfMondayWeek(anchorDate);
  const todayKey = formatLocalDateKey(today);
  const cells: CalendarDayCell[] = [];

  for (let offset = 0; offset < 7; offset += 1) {
    const date = addDays(weekStart, offset);
    cells.push({
      date,
      dateKey: formatLocalDateKey(date),
      dayOfMonth: date.getDate(),
      isCurrentMonth: date.getMonth() === anchorDate.getMonth(),
      isToday: formatLocalDateKey(date) === todayKey,
    });
  }

  return cells;
}

export function bucketPostByLocalDay(post: CalendarModelPost): string | null {
  const date = getCalendarPostDate(post);
  if (!date) return null;
  return formatLocalDateKey(date);
}

export function getCalendarPostDate(post: CalendarModelPost): Date | null {
  const source =
    post.status === "scheduled" && post.scheduled_at
      ? post.scheduled_at
      : post.published_at || post.created_at;

  if (!source) return null;
  const date = new Date(source);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function getCalendarPostMinuteOfDay(post: CalendarModelPost): number | null {
  const date = getCalendarPostDate(post);
  if (!date) return null;
  return date.getHours() * 60 + date.getMinutes();
}

export function getWheelNavigationIntent(
  mode: CalendarViewMode,
  deltaX: number,
  deltaY: number,
  shiftKey = false,
): -1 | 0 | 1 {
  const threshold = 80;
  if (mode === "day") return 0;

  if (mode === "month") {
    if (Math.abs(deltaY) < threshold || Math.abs(deltaY) < Math.abs(deltaX)) return 0;
    return deltaY > 0 ? 1 : -1;
  }

  const horizontalDelta = Math.abs(deltaX) >= Math.abs(deltaY) ? deltaX : shiftKey ? deltaY : 0;
  if (Math.abs(horizontalDelta) < threshold) return 0;
  return horizontalDelta > 0 ? 1 : -1;
}

export function parseCalendarViewMode(value: string | null | undefined): CalendarViewMode {
  return value && CALENDAR_VIEW_MODES.has(value as CalendarViewMode) ? (value as CalendarViewMode) : "month";
}

export function getPostStatusGroup(post: CalendarModelPost): CalendarStatusGroup {
  if (post.archived_at) return "archived";
  if (post.status === "published") return "published";
  if (post.status === "scheduled") return "scheduled";
  if (IN_PROGRESS_STATUSES.has(post.status)) return "in_progress";
  if (FAILED_STATUSES.has(post.status)) return "failed";
  if (post.status === "cancelled") return "cancelled";
  return "draft";
}

export function shouldShowPostForStatusFilter(post: CalendarModelPost, filter: CalendarStatusFilter): boolean {
  if (filter === "all") return true;
  return getPostStatusGroup(post) === filter;
}

export function getProfileCalendarColor(profile: CalendarModelProfile): string {
  if (profile.branding_primary_color && isValidHexColor(profile.branding_primary_color)) {
    return profile.branding_primary_color;
  }

  const source = profile.id || profile.name;
  const index = Math.abs(hashString(source)) % PROFILE_COLOR_PALETTE.length;
  return PROFILE_COLOR_PALETTE[index];
}

export function formatLocalDateKey(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
}

function startOfMondayWeek(date: Date): Date {
  const day = date.getDay();
  const offset = day === 0 ? -6 : 1 - day;
  return addDays(date, offset);
}

function isValidHexColor(value: string): boolean {
  return /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(value.trim());
}

function hashString(value: string): number {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) | 0;
  }
  return hash;
}
