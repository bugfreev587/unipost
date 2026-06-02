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
export type CalendarPopoverSide = "right" | "left" | "bottom" | "top";

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

export type CalendarPopoverRect = {
  left: number;
  top: number;
  right: number;
  bottom: number;
  width: number;
  height: number;
};

export type CalendarPopoverSize = {
  width: number;
  height: number;
};

export type CalendarWheelNavigationAccumulator = {
  deltaX: number;
  deltaY: number;
};

export type CalendarPopoverPlacement = {
  side: CalendarPopoverSide;
  left: number;
  top: number;
  arrowX: number;
  arrowY: number;
  transformOrigin: string;
};

export type BoundedCalendarPopoverPlacement = CalendarPopoverPlacement & {
  availableHeight: number;
};

export type TimedCalendarEventInput = {
  id: string;
  minuteOfDay: number;
};

export type TimedCalendarEventLayout = {
  id: string;
  top: number;
  lane: number;
  laneCount: number;
  leftPercent: number;
  widthPercent: number;
};

const IN_PROGRESS_STATUSES = new Set(["queued", "dispatching", "retrying", "processing"]);
const FAILED_STATUSES = new Set(["failed", "partial"]);
const WHEEL_NAVIGATION_THRESHOLD = 80;
const SIDE_POPOVER_ARROW_EDGE_INSET = 2;

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
  return buildRollingMonthGrid(monthDate, 0, 6, 0, today);
}

export function buildRollingMonthGrid(
  anchorDate: Date,
  beforeWeeks = 1,
  visibleWeeks = 6,
  afterWeeks = 1,
  today = new Date(),
): CalendarDayCell[] {
  const monthDate = new Date(anchorDate.getFullYear(), anchorDate.getMonth(), anchorDate.getDate());
  const monthStart = new Date(monthDate.getFullYear(), monthDate.getMonth(), 1);
  const monthEnd = new Date(monthDate.getFullYear(), monthDate.getMonth() + 1, 0);
  const totalWeeks = Math.max(1, beforeWeeks + visibleWeeks + afterWeeks);
  const gridStart = addDays(startOfSundayWeek(monthDate), beforeWeeks * -7);
  const todayKey = formatLocalDateKey(today);
  const cells: CalendarDayCell[] = [];

  for (let offset = 0; offset < totalWeeks * 7; offset += 1) {
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
  return buildRollingWeekDays(anchorDate, 0, 7, 0, today);
}

export function buildRollingWeekDays(
  anchorDate: Date,
  beforeDays = 1,
  visibleDays = 7,
  afterDays = 1,
  today = new Date(),
): CalendarDayCell[] {
  const weekStart = new Date(anchorDate.getFullYear(), anchorDate.getMonth(), anchorDate.getDate());
  const todayKey = formatLocalDateKey(today);
  const cells: CalendarDayCell[] = [];
  const totalDays = Math.max(1, beforeDays + visibleDays + afterDays);
  const windowStart = addDays(weekStart, beforeDays * -1);

  for (let offset = 0; offset < totalDays; offset += 1) {
    const date = addDays(windowStart, offset);
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

export function getTimedEventTop(minuteOfDay: number, hourHeight: number): number {
  return (minuteOfDay / 60) * hourHeight;
}

export function getTimedTimelineContentHeight(
  hourHeight: number,
  eventMinHeight: number,
  bottomPadding: number,
): number {
  return 24 * hourHeight + eventMinHeight + bottomPadding;
}

export function getTimedEventLayouts(
  events: TimedCalendarEventInput[],
  hourHeight: number,
  eventMinHeight: number,
  laneGap = 4,
): Map<string, TimedCalendarEventLayout> {
  const intervals = events
    .filter((event) => Number.isFinite(event.minuteOfDay))
    .map((event) => {
      const top = getTimedEventTop(event.minuteOfDay, hourHeight);
      return {
        ...event,
        top,
        bottom: top + eventMinHeight + laneGap,
      };
    })
    .sort((a, b) => a.top - b.top || a.id.localeCompare(b.id));

  const layouts = new Map<string, TimedCalendarEventLayout>();
  let cluster: typeof intervals = [];
  let clusterBottom = Number.NEGATIVE_INFINITY;

  const flushCluster = () => {
    if (cluster.length === 0) return;

    const laneBottoms: number[] = [];
    const assigned = cluster.map((event) => {
      const openLane = laneBottoms.findIndex((bottom) => bottom <= event.top);
      const lane = openLane === -1 ? laneBottoms.length : openLane;
      laneBottoms[lane] = event.bottom;
      return { event, lane };
    });
    const laneCount = Math.max(1, laneBottoms.length);
    const widthPercent = 100 / laneCount;

    for (const { event, lane } of assigned) {
      layouts.set(event.id, {
        id: event.id,
        top: event.top,
        lane,
        laneCount,
        leftPercent: lane * widthPercent,
        widthPercent,
      });
    }
  };

  for (const interval of intervals) {
    if (cluster.length > 0 && interval.top >= clusterBottom) {
      flushCluster();
      cluster = [];
      clusterBottom = Number.NEGATIVE_INFINITY;
    }

    cluster.push(interval);
    clusterBottom = Math.max(clusterBottom, interval.bottom);
  }

  flushCluster();
  return layouts;
}

export function getWheelNavigationIntent(
  mode: CalendarViewMode,
  deltaX: number,
  deltaY: number,
  shiftKey = false,
): -1 | 0 | 1 {
  if (mode === "day") return 0;

  if (mode === "month") {
    if (Math.abs(deltaY) < WHEEL_NAVIGATION_THRESHOLD || Math.abs(deltaY) < Math.abs(deltaX)) return 0;
    return deltaY > 0 ? 1 : -1;
  }

  const horizontalDelta = Math.abs(deltaX) >= Math.abs(deltaY) ? deltaX : shiftKey ? deltaY : 0;
  if (Math.abs(horizontalDelta) < WHEEL_NAVIGATION_THRESHOLD) return 0;
  return horizontalDelta > 0 ? 1 : -1;
}

export function getAccumulatedWheelNavigationIntent(
  mode: CalendarViewMode,
  accumulator: CalendarWheelNavigationAccumulator,
  deltaX: number,
  deltaY: number,
  shiftKey = false,
): { direction: -1 | 0 | 1; accumulator: CalendarWheelNavigationAccumulator } {
  if (mode === "day") return { direction: 0, accumulator: { deltaX: 0, deltaY: 0 } };

  const nextAccumulator = {
    deltaX: accumulator.deltaX + deltaX,
    deltaY: accumulator.deltaY + deltaY,
  };
  const direction = getWheelNavigationIntent(
    mode,
    nextAccumulator.deltaX,
    nextAccumulator.deltaY,
    shiftKey,
  );

  return {
    direction,
    accumulator: direction === 0 ? nextAccumulator : { deltaX: 0, deltaY: 0 },
  };
}

export function getSwipeNavigationIntent(
  mode: CalendarViewMode,
  startX: number,
  startY: number,
  endX: number,
  endY: number,
): -1 | 0 | 1 {
  return getWheelNavigationIntent(mode, startX - endX, startY - endY);
}

export function shiftCalendarDateBySwipe(mode: CalendarViewMode, date: Date, direction: -1 | 1): Date {
  if (mode === "month") return addDays(date, direction * 7);
  if (mode === "week") return addDays(date, direction);
  return date;
}

export function getCalendarSnapSteps(offsetPx: number, unitPx: number, maxSteps = 1): number {
  if (!Number.isFinite(offsetPx) || !Number.isFinite(unitPx) || unitPx <= 0) return 0;
  const clampedMax = Math.max(1, Math.floor(maxSteps));
  const rawSteps = -offsetPx / unitPx;
  const roundedSteps = rawSteps >= 0 ? Math.floor(rawSteps + 0.5) : Math.ceil(rawSteps - 0.5);
  return clamp(roundedSteps, -clampedMax, clampedMax);
}

export function getCalendarSnapOffset(steps: number, unitPx: number): number {
  if (!Number.isFinite(unitPx) || unitPx <= 0) return 0;
  return -steps * unitPx;
}

export function getContinuousCalendarSnapOffset(
  offsetPx: number,
  unitPx: number,
): { steps: number; offsetPx: number } {
  if (!Number.isFinite(offsetPx) || !Number.isFinite(unitPx) || unitPx <= 0) {
    return { steps: 0, offsetPx: 0 };
  }

  if (offsetPx <= -unitPx) {
    const steps = Math.floor(-offsetPx / unitPx);
    return { steps, offsetPx: offsetPx + steps * unitPx };
  }

  if (offsetPx >= unitPx) {
    const steps = -Math.floor(offsetPx / unitPx);
    return { steps, offsetPx: offsetPx + steps * unitPx };
  }

  return { steps: 0, offsetPx };
}

export function shiftCalendarDateBySnapSteps(mode: CalendarViewMode, date: Date, steps: number): Date {
  if (mode === "month") return addDays(date, steps * 7);
  if (mode === "week") return addDays(date, steps);
  return date;
}

export function parseCalendarViewMode(value: string | null | undefined): CalendarViewMode {
  return value && CALENDAR_VIEW_MODES.has(value as CalendarViewMode) ? (value as CalendarViewMode) : "month";
}

export function getAnchoredPopoverPlacement({
  anchor,
  viewport,
  popover,
  gap = 12,
  margin = 12,
  arrowInset = 18,
}: {
  anchor: CalendarPopoverRect;
  viewport: CalendarPopoverSize;
  popover: CalendarPopoverSize;
  gap?: number;
  margin?: number;
  arrowInset?: number;
}): CalendarPopoverPlacement {
  const availableRight = viewport.width - anchor.right - margin - gap;
  const availableLeft = anchor.left - margin - gap;
  const availableBottom = viewport.height - anchor.bottom - margin - gap;
  const availableTop = anchor.top - margin - gap;
  const side = choosePopoverSide({
    availableRight,
    availableLeft,
    availableBottom,
    availableTop,
    popover,
  });
  const anchorCenterX = anchor.left + anchor.width / 2;
  const anchorCenterY = anchor.top + anchor.height / 2;
  const maxLeft = viewport.width - popover.width - margin;
  const maxTop = viewport.height - popover.height - margin;

  if (side === "right" || side === "left") {
    const left = side === "right"
      ? clamp(anchor.right + gap, margin, maxLeft)
      : clamp(anchor.left - popover.width - gap, margin, maxLeft);
    const top = clamp(anchorCenterY - popover.height / 2, margin, maxTop);
    const arrowY = Math.round(clamp(anchorCenterY - top, arrowInset, popover.height - arrowInset));
    return {
      side,
      left: Math.round(left),
      top: Math.round(top),
      arrowX: side === "right" ? 0 : popover.width,
      arrowY,
      transformOrigin: `${side === "right" ? "left" : "right"} ${arrowY}px`,
    };
  }

  const left = clamp(anchorCenterX - popover.width / 2, margin, maxLeft);
  const top = side === "bottom"
    ? clamp(anchor.bottom + gap, margin, maxTop)
    : clamp(anchor.top - popover.height - gap, margin, maxTop);
  const arrowX = Math.round(clamp(anchorCenterX - left, arrowInset, popover.width - arrowInset));

  return {
    side,
    left: Math.round(left),
    top: Math.round(top),
    arrowX,
    arrowY: side === "bottom" ? 0 : popover.height,
    transformOrigin: `${arrowX}px ${side === "bottom" ? "top" : "bottom"}`,
  };
}

export function getBoundedCalendarPopoverPlacement({
  anchor,
  viewport,
  popover,
  bounds,
  gap = 12,
  margin = 12,
  arrowInset = 18,
  verticalStrategy = "bounds",
}: {
  anchor: CalendarPopoverRect;
  viewport: CalendarPopoverSize;
  popover: CalendarPopoverSize;
  bounds: CalendarPopoverRect;
  gap?: number;
  margin?: number;
  arrowInset?: number;
  verticalStrategy?: "bounds" | "anchor";
}): BoundedCalendarPopoverPlacement {
  const placement = getAnchoredPopoverPlacement({ anchor, viewport, popover, gap, margin, arrowInset });
  const boundedTop = clamp(bounds.top, margin, viewport.height - margin);
  const boundedBottom = clamp(bounds.bottom, boundedTop, viewport.height - margin);
  const boundedHeight = Math.max(0, boundedBottom - boundedTop);
  const desiredHeight = Math.min(popover.height, boundedHeight || popover.height);
  const anchorCenterY = anchor.top + anchor.height / 2;
  const top = placement.side === "right" || placement.side === "left"
    ? clamp(
      verticalStrategy === "anchor" ? anchorCenterY - desiredHeight / 2 : boundedTop,
      boundedTop,
      boundedBottom - desiredHeight,
    )
    : clamp(placement.top, boundedTop, boundedBottom - desiredHeight);
  const availableHeight = Math.max(0, boundedBottom - top);

  if (placement.side === "right" || placement.side === "left") {
    const arrowHeight = Math.min(desiredHeight, availableHeight);
    const sideArrowInset = Math.min(arrowInset, SIDE_POPOVER_ARROW_EDGE_INSET);
    const arrowY = Math.round(
      clamp(anchorCenterY - top, sideArrowInset, Math.max(sideArrowInset, arrowHeight - sideArrowInset)),
    );
    return {
      ...placement,
      top: Math.round(top),
      arrowY,
      transformOrigin: `${placement.side === "right" ? "left" : "right"} ${arrowY}px`,
      availableHeight: Math.round(availableHeight),
    };
  }

  return {
    ...placement,
    top: Math.round(top),
    arrowY: placement.side === "bottom" ? 0 : Math.round(availableHeight),
    availableHeight: Math.round(availableHeight),
  };
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

function startOfSundayWeek(date: Date): Date {
  return addDays(date, -date.getDay());
}

function choosePopoverSide({
  availableRight,
  availableLeft,
  availableBottom,
  availableTop,
  popover,
}: {
  availableRight: number;
  availableLeft: number;
  availableBottom: number;
  availableTop: number;
  popover: CalendarPopoverSize;
}): CalendarPopoverSide {
  if (availableRight >= popover.width) return "right";
  if (availableLeft >= popover.width) return "left";
  if (availableBottom >= popover.height) return "bottom";
  if (availableTop >= popover.height) return "top";

  const spaces: Array<[CalendarPopoverSide, number]> = [
    ["right", availableRight],
    ["left", availableLeft],
    ["bottom", availableBottom],
    ["top", availableTop],
  ];
  spaces.sort((a, b) => b[1] - a[1]);
  return spaces[0]?.[0] || "right";
}

function clamp(value: number, min: number, max: number): number {
  if (max < min) return min;
  return Math.min(Math.max(value, min), max);
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
