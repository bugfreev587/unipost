"use client";

import { useAuth } from "@clerk/nextjs";
import { ChevronLeft, ChevronRight, List, Loader2, Plus, Save, SlidersHorizontal, X } from "lucide-react";
import Link from "next/link";
import { usePathname, useParams, useRouter, useSearchParams } from "next/navigation";
import {
  Fragment,
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent,
  type ReactNode,
  type TouchEvent,
  type UIEvent,
  type WheelEvent,
} from "react";
import { AccountDestinationIcon } from "@/components/account-destination-icon";
import { CreatePostDrawer } from "@/components/posts/create-post/create-post-drawer";
import { PostPlatformResults } from "../details/post-platform-results";
import { PlatformEditorBlock } from "@/components/posts/create-post/platform-editor-block";
import {
  getFirstCommentMaxLength,
  getPlatformCaptionLimit,
  supportsFirstComment,
  supportsThreads,
} from "@/components/posts/create-post/ai-assist";
import { measureVideoMetadata, type ExistingMediaItem, type MediaItem, useCreatePostForm } from "@/components/posts/create-post/use-create-post-form";
import {
  createMedia,
  getMedia,
  getPlatformCapabilities,
  listProfiles,
  listSocialAccounts,
  listAllSocialPosts,
  updateSocialPost,
  validateSocialPost,
  type CreateSocialPostPayload,
  type PlatformCapabilitiesEnvelope,
  type Profile,
  type SocialAccount,
  type SocialPost,
  type SocialPostValidationIssue,
  type SocialPostValidationResult,
} from "@/lib/api";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  buildMonthGrid,
  buildRollingWeekDays,
  buildWeekDays,
  bucketPostByLocalDay,
  formatLocalDateKey,
  getBoundedCalendarPopoverPlacement,
  getCalendarSnapOffset,
  getCalendarSnapSteps,
  getCalendarPostDate,
  getCalendarPostMinuteOfDay,
  getCalendarStatusColor,
  getContinuousCalendarSnapOffset,
  getMonthDayPostLayout,
  getPostStatusGroup,
  getProfileCalendarColor,
  getTimedEventLayouts,
  getTimedPostGroups,
  getTimedTimelineContentHeight,
  parseCalendarViewMode,
  type CalendarDayCell,
  type CalendarPopoverRect,
  type CalendarPopoverSize,
  shouldShowPostForStatusFilter,
  shiftCalendarDateBySnapSteps,
  type CalendarStatusFilter,
  type CalendarStatusGroup,
  type CalendarViewMode,
  type TimedCalendarEventLayout,
} from "./calendar-model";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
const HOUR_HEIGHT = 64;
const TIMED_EVENT_MIN_HEIGHT = 38;
const TIMED_GROUP_VISIBLE_POST_LIMIT = 1;
const TIMELINE_END_PADDING = 8;
const TIMELINE_CONTENT_HEIGHT = getTimedTimelineContentHeight(
  HOUR_HEIGHT,
  TIMED_EVENT_MIN_HEIGHT,
  TIMELINE_END_PADDING,
);
const POPOVER_FALLBACK_SIZE: CalendarPopoverSize = { width: 560, height: 560 };
const DAY_OVERFLOW_POPOVER_FALLBACK_SIZE: CalendarPopoverSize = { width: 380, height: 360 };
const SNAP_TRANSITION_MS = 260;
const WHEEL_SNAP_IDLE_MS = 110;
const MONTH_VISIBLE_WEEKS = 6;
const WEEK_VISIBLE_DAYS = 7;
const WEEK_BUFFER_DAYS = 1;

type SelectedPostTarget = {
  postId: string;
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
};

type DayOverflowTarget = {
  dateKey: string;
  dayLabel: string;
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
};

type TimedOverflowTarget = {
  postIds: string[];
  dateLabel: string;
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
};

type CalendarSnapState = {
  mode: Extract<CalendarViewMode, "month" | "week">;
  offsetPx: number;
  isSettling: boolean;
};

type CalendarTouchState = {
  mode: Extract<CalendarViewMode, "month" | "week">;
  startX: number;
  startY: number;
};

const STATUS_FILTERS: Array<{ value: CalendarStatusFilter; label: string }> = [
  { value: "all", label: "All Status" },
  { value: "published", label: "Published" },
  { value: "scheduled", label: "Scheduled" },
  { value: "quota_hold", label: "Quota Hold" },
  { value: "in_progress", label: "In Progress" },
  { value: "failed", label: "Failed" },
  { value: "draft", label: "Drafts" },
  { value: "cancelled", label: "Cancelled" },
  { value: "archived", label: "Archived" },
];

const STATUS_META: Record<CalendarStatusGroup, { label: string; short: string }> = {
  published: { label: "Published", short: "PUB" },
  scheduled: { label: "Scheduled", short: "SCH" },
  quota_hold: { label: "Quota Hold", short: "HOLD" },
  in_progress: { label: "In Progress", short: "RUN" },
  failed: { label: "Failed", short: "FAIL" },
  draft: { label: "Draft", short: "DRFT" },
  cancelled: { label: "Cancelled", short: "CNCL" },
  archived: { label: "Archived", short: "ARCH" },
  unknown: { label: "Unknown", short: "UNK" },
};

export function PostsCalendarView() {
  const params = useParams<{ id: string }>();
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const profileId = params.id;
  const { getToken } = useAuth();
  const workspaceId = useWorkspaceId();
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedProfileIds, setSelectedProfileIds] = useState<Set<string>>(new Set());
  const [selectedPlatforms, setSelectedPlatforms] = useState<Set<string>>(new Set());
  const [statusFilter, setStatusFilter] = useState<CalendarStatusFilter>("all");
  const [visibleDate, setVisibleDate] = useState(() => new Date());
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(new Date()));
  const [filtersInitialized, setFiltersInitialized] = useState(false);
  const [selectedPostTarget, setSelectedPostTarget] = useState<SelectedPostTarget | null>(null);
  const [dayOverflowTarget, setDayOverflowTarget] = useState<DayOverflowTarget | null>(null);
  const [timedOverflowTarget, setTimedOverflowTarget] = useState<TimedOverflowTarget | null>(null);
  const [editingPostTarget, setEditingPostTarget] = useState<SelectedPostTarget | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [mobileFiltersOpen, setMobileFiltersOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [calendarSnap, setCalendarSnap] = useState<CalendarSnapState | null>(null);
  const snapOffsetRef = useRef(0);
  const wheelSnapTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const snapTransitionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const touchStartRef = useRef<CalendarTouchState | null>(null);
  const weekShellRef = useRef<HTMLDivElement | null>(null);
  const weekTimeScrollRef = useRef<HTMLDivElement | null>(null);
  const weekTimeGutterScrollRef = useRef<HTMLDivElement | null>(null);
  const [weekScrollbarWidth, setWeekScrollbarWidth] = useState(0);

  const calendarMode = useMemo(() => parseCalendarViewMode(searchParams.get("view")), [searchParams]);
  const timezone = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || "Local time", []);

  const clearCalendarSnap = useCallback(() => {
    if (wheelSnapTimerRef.current) {
      clearTimeout(wheelSnapTimerRef.current);
      wheelSnapTimerRef.current = null;
    }
    if (snapTransitionTimerRef.current) {
      clearTimeout(snapTransitionTimerRef.current);
      snapTransitionTimerRef.current = null;
    }
    snapOffsetRef.current = 0;
    touchStartRef.current = null;
    setCalendarSnap(null);
  }, []);

  const replaceCalendarMode = useCallback((mode: CalendarViewMode) => {
    const nextParams = new URLSearchParams(searchParams.toString());
    nextParams.set("view", mode);
    const query = nextParams.toString();
    router.replace(`${pathname}${query ? `?${query}` : ""}`, { scroll: false });
  }, [pathname, router, searchParams]);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      if (!token) {
        setLoading(false);
        return;
      }
      const [postRes, profileRes] = await Promise.all([
        listAllSocialPosts(token),
        listProfiles(token),
      ]);
      const loadedProfiles = profileRes.data || [];
      const accountGroups = await Promise.all(
        loadedProfiles.map(async (profile) => {
          try {
            const accountRes = await listSocialAccounts(token, profile.id);
            return accountRes.data || [];
          } catch {
            return [] as SocialAccount[];
          }
        }),
      );
      setPosts(postRes.data || []);
      setProfiles(loadedProfiles);
      setAccounts(accountGroups.flat());
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load posts calendar");
    } finally {
      setLoading(false);
    }
  }, [getToken]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  useEffect(() => {
    clearCalendarSnap();
  }, [calendarMode, clearCalendarSnap]);

  useEffect(() => () => {
    if (wheelSnapTimerRef.current) {
      clearTimeout(wheelSnapTimerRef.current);
    }
    if (snapTransitionTimerRef.current) {
      clearTimeout(snapTransitionTimerRef.current);
    }
  }, []);

  useEffect(() => {
    if (calendarMode !== "week") return;
    const node = weekTimeScrollRef.current;
    if (!node) return;

    const updateScrollbarWidth = () => {
      setWeekScrollbarWidth(Math.max(0, node.offsetWidth - node.clientWidth));
    };

    updateScrollbarWidth();
    const resizeObserver = new ResizeObserver(updateScrollbarWidth);
    resizeObserver.observe(node);
    window.addEventListener("resize", updateScrollbarWidth);

    return () => {
      resizeObserver.disconnect();
      window.removeEventListener("resize", updateScrollbarWidth);
    };
  }, [calendarMode]);

  useEffect(() => {
    if (calendarMode !== "week") return;
    const node = weekShellRef.current;
    if (!node) return;

    const preventWeekBrowserNavigation = (event: Event) => {
      if (!event.cancelable) return;

      if (typeof globalThis.WheelEvent !== "undefined" && event instanceof globalThis.WheelEvent) {
        const horizontalDelta = Math.abs(event.deltaX) >= Math.abs(event.deltaY)
          ? event.deltaX
          : event.shiftKey
            ? event.deltaY
            : 0;
        if (horizontalDelta !== 0) event.preventDefault();
        return;
      }

      if (typeof globalThis.TouchEvent !== "undefined" && event instanceof globalThis.TouchEvent) {
        const start = touchStartRef.current;
        const touch = event.touches[0];
        if (!start || start.mode !== "week" || !touch) return;

        const deltaX = touch.clientX - start.startX;
        const deltaY = touch.clientY - start.startY;
        if (Math.abs(deltaX) > 8 && Math.abs(deltaX) > Math.abs(deltaY)) {
          event.preventDefault();
        }
      }
    };

    node.addEventListener("wheel", preventWeekBrowserNavigation, { passive: false });
    node.addEventListener("touchmove", preventWeekBrowserNavigation, { passive: false });

    return () => {
      node.removeEventListener("wheel", preventWeekBrowserNavigation);
      node.removeEventListener("touchmove", preventWeekBrowserNavigation);
    };
  }, [calendarMode]);

  const profilesById = useMemo(() => new Map(profiles.map((profile) => [profile.id, profile])), [profiles]);
  const profileColors = useMemo(
    () => new Map(profiles.map((profile) => [profile.id, getProfileCalendarColor(profile)])),
    [profiles],
  );

  const platformOptions = useMemo(() => {
    const names = new Set<string>();
    for (const account of accounts) {
      if (account.platform) names.add(account.platform);
    }
    for (const post of posts) {
      for (const platform of getPostPlatforms(post)) names.add(platform);
    }
    return Array.from(names).sort((a, b) => formatPlatformName(a).localeCompare(formatPlatformName(b)));
  }, [accounts, posts]);

  useEffect(() => {
    if (filtersInitialized || loading) return;
    setSelectedProfileIds(new Set(profiles.map((profile) => profile.id)));
    setSelectedPlatforms(new Set(platformOptions));
    setFiltersInitialized(true);
  }, [filtersInitialized, loading, platformOptions, profiles]);

  const currentProfileAccounts = useMemo(
    () => accounts.filter((account) => account.profile_id === profileId),
    [accounts, profileId],
  );

  const filteredPosts = useMemo(() => {
    return posts.filter((post) => {
      if (!shouldShowPostForStatusFilter(post, statusFilter)) return false;
      if (profiles.length > 0 && selectedProfileIds.size === 0) return false;
      if (post.profile_ids?.length > 0) {
        const matchesProfile = post.profile_ids.some((id) => selectedProfileIds.has(id));
        if (!matchesProfile) return false;
      }
      if (platformOptions.length > 0 && selectedPlatforms.size === 0) return false;
      const platforms = getPostPlatforms(post);
      if (platforms.length > 0 && !platforms.some((platform) => selectedPlatforms.has(platform))) return false;
      return true;
    });
  }, [platformOptions.length, posts, profiles.length, selectedPlatforms, selectedProfileIds, statusFilter]);

  const postsByDate = useMemo(() => {
    const byDay = new Map<string, SocialPost[]>();
    for (const post of filteredPosts) {
      const dateKey = bucketPostByLocalDay(post);
      if (!dateKey) continue;
      if (!byDay.has(dateKey)) byDay.set(dateKey, []);
      byDay.get(dateKey)?.push(post);
    }
    for (const dayPosts of byDay.values()) {
      dayPosts.sort((a, b) => getPostTimeValue(a) - getPostTimeValue(b));
    }
    return byDay;
  }, [filteredPosts]);

  const monthCells = useMemo(() => buildMonthGrid(visibleMonth), [visibleMonth]);
  const weekDays = useMemo(() => buildWeekDays(visibleDate), [visibleDate]);
  const rollingWeekDays = useMemo(
    () => buildRollingWeekDays(visibleDate, WEEK_BUFFER_DAYS, WEEK_VISIBLE_DAYS, WEEK_BUFFER_DAYS),
    [visibleDate],
  );
  const dayDateKey = useMemo(() => formatLocalDateKey(visibleDate), [visibleDate]);
  const timelineStyle = useMemo(
    () => ({
      "--hour-height": `${HOUR_HEIGHT}px`,
      "--calendar-timed-event-min-height": `${TIMED_EVENT_MIN_HEIGHT}px`,
      "--calendar-timeline-height": `${TIMELINE_CONTENT_HEIGHT}px`,
    }) as CSSProperties,
    [],
  );

  const selectedPostId = selectedPostTarget?.postId || null;
  const selectedPost = useMemo(
    () => posts.find((post) => post.id === selectedPostId) || null,
    [posts, selectedPostId],
  );
  const dayOverflowPosts = useMemo(() => {
    if (!dayOverflowTarget) return [];
    return postsByDate.get(dayOverflowTarget.dateKey) || [];
  }, [dayOverflowTarget, postsByDate]);
  const timedOverflowPosts = useMemo(() => {
    if (!timedOverflowTarget) return [];
    const order = new Map(timedOverflowTarget.postIds.map((postId, index) => [postId, index]));
    return posts
      .filter((post) => order.has(post.id))
      .sort((a, b) => (order.get(a.id) ?? 0) - (order.get(b.id) ?? 0));
  }, [posts, timedOverflowTarget]);
  const editingPostId = editingPostTarget?.postId || null;
  const editingPost = useMemo(
    () => posts.find((post) => post.id === editingPostId) || null,
    [posts, editingPostId],
  );

  const visibleDateKeys = useMemo(() => {
    if (calendarMode === "month") return new Set(monthCells.map((cell) => cell.dateKey));
    if (calendarMode === "week") return new Set(weekDays.map((cell) => cell.dateKey));
    return new Set([dayDateKey]);
  }, [calendarMode, dayDateKey, monthCells, weekDays]);

  const calendarTitle = useMemo(
    () => getCalendarTitle(calendarMode, visibleMonth, visibleDate, weekDays),
    [calendarMode, visibleDate, visibleMonth, weekDays],
  );

  const visiblePostCount = filteredPosts.filter((post) => {
    const key = bucketPostByLocalDay(post);
    return Boolean(key && visibleDateKeys.has(key));
  }).length;

  const calendarSubtitle = loading
    ? "Loading posts"
    : calendarMode === "day"
      ? `${getWeekdayName(visibleDate)} - ${visiblePostCount} posts in view`
      : `${visiblePostCount} posts in view`;

  const handleCreated = useCallback(async () => {
    await loadData();
  }, [loadData]);

  const handleEdited = useCallback(async () => {
    await loadData();
    setEditingPostTarget(null);
    setSelectedPostTarget(null);
  }, [loadData]);

  const shiftCalendar = useCallback((direction: -1 | 1) => {
    clearCalendarSnap();

    if (calendarMode === "month") {
      setVisibleMonth((date) => addMonths(date, direction));
      setVisibleDate((date) => addMonths(date, direction));
      return;
    }

    const days = calendarMode === "week" ? direction * 7 : direction;
    setVisibleDate((date) => {
      const next = addDays(date, days);
      setVisibleMonth(startOfMonth(next));
      return next;
    });
  }, [calendarMode, clearCalendarSnap]);

  const shiftVisibleCalendarBySnapSteps = useCallback((
    mode: Extract<CalendarViewMode, "month" | "week">,
    steps: number,
  ) => {
    if (steps === 0) return;

    if (mode === "month") {
      setVisibleDate((date) => shiftCalendarDateBySnapSteps(mode, date, steps));
      setVisibleMonth((date) => shiftCalendarDateBySnapSteps(mode, date, steps));
      return;
    }

    setVisibleDate((date) => {
      const next = shiftCalendarDateBySnapSteps(mode, date, steps);
      setVisibleMonth(startOfMonth(next));
      return next;
    });
  }, []);

  const goToToday = useCallback(() => {
    clearCalendarSnap();
    const today = new Date();
    setVisibleDate(today);
    setVisibleMonth(startOfMonth(today));
  }, [clearCalendarSnap]);

  const getCalendarSnapUnitPx = useCallback((
    mode: Extract<CalendarViewMode, "month" | "week">,
    element: HTMLElement,
  ) => {
    if (mode === "month") {
      const viewport = element.querySelector<HTMLElement>(".posts-calendar-month-view");
      return Math.max(1, (viewport?.clientHeight || element.clientHeight) / MONTH_VISIBLE_WEEKS);
    }

    const viewport = element.querySelector<HTMLElement>(".posts-calendar-week-content");
    return Math.max(1, (viewport?.clientWidth || element.clientWidth) / WEEK_VISIBLE_DAYS);
  }, []);

  const setCalendarSnapOffset = useCallback((
    mode: Extract<CalendarViewMode, "month" | "week">,
    offsetPx: number,
    isSettling = false,
  ) => {
    snapOffsetRef.current = offsetPx;
    setCalendarSnap({ mode, offsetPx, isSettling });
  }, []);

  const settleCalendarSnap = useCallback((
    mode: Extract<CalendarViewMode, "month" | "week">,
    unitPx: number,
  ) => {
    if (wheelSnapTimerRef.current) {
      clearTimeout(wheelSnapTimerRef.current);
      wheelSnapTimerRef.current = null;
    }
    if (snapTransitionTimerRef.current) {
      clearTimeout(snapTransitionTimerRef.current);
      snapTransitionTimerRef.current = null;
    }

    const steps = getCalendarSnapSteps(snapOffsetRef.current, unitPx, 1);
    const targetOffset = getCalendarSnapOffset(steps, unitPx);
    if (steps === 0 && Math.abs(snapOffsetRef.current) < 0.5) {
      snapOffsetRef.current = 0;
      setCalendarSnap(null);
      return;
    }

    snapOffsetRef.current = targetOffset;
    setCalendarSnap({ mode, offsetPx: targetOffset, isSettling: true });

    snapTransitionTimerRef.current = setTimeout(() => {
      shiftVisibleCalendarBySnapSteps(mode, steps);
      snapOffsetRef.current = 0;
      setCalendarSnap(null);
      snapTransitionTimerRef.current = null;
    }, SNAP_TRANSITION_MS);
  }, [shiftVisibleCalendarBySnapSteps]);

  const scheduleWheelSnap = useCallback((
    mode: Extract<CalendarViewMode, "month" | "week">,
    unitPx: number,
  ) => {
    if (wheelSnapTimerRef.current) {
      clearTimeout(wheelSnapTimerRef.current);
    }
    wheelSnapTimerRef.current = setTimeout(() => {
      settleCalendarSnap(mode, unitPx);
    }, WHEEL_SNAP_IDLE_MS);
  }, [settleCalendarSnap]);

  const handleCalendarWheel = useCallback((event: WheelEvent<HTMLElement>) => {
    if (calendarMode === "day") return;

    const mode = calendarMode;
    let dragDelta = 0;
    if (mode === "month") {
      if (Math.abs(event.deltaY) < Math.abs(event.deltaX)) return;
      dragDelta = -event.deltaY;
    } else {
      const horizontalDelta = Math.abs(event.deltaX) >= Math.abs(event.deltaY)
        ? event.deltaX
        : event.shiftKey
          ? event.deltaY
          : 0;
      if (horizontalDelta === 0) return;
      dragDelta = -horizontalDelta;
    }

    if (snapTransitionTimerRef.current) {
      clearTimeout(snapTransitionTimerRef.current);
      snapTransitionTimerRef.current = null;
    }

    const unitPx = getCalendarSnapUnitPx(mode, event.currentTarget);
    const continuousSnap = getContinuousCalendarSnapOffset(snapOffsetRef.current + dragDelta, unitPx);
    shiftVisibleCalendarBySnapSteps(mode, continuousSnap.steps);
    const nextOffset = clampCalendarSnapOffset(continuousSnap.offsetPx, unitPx);
    setCalendarSnapOffset(mode, nextOffset);
    scheduleWheelSnap(mode, unitPx);
  }, [
    calendarMode,
    getCalendarSnapUnitPx,
    scheduleWheelSnap,
    setCalendarSnapOffset,
    shiftVisibleCalendarBySnapSteps,
  ]);

  const handleCalendarTouchStart = useCallback((event: TouchEvent<HTMLElement>) => {
    if (calendarMode === "day") return;
    const touch = event.touches[0];
    if (!touch) return;
    if (wheelSnapTimerRef.current) {
      clearTimeout(wheelSnapTimerRef.current);
      wheelSnapTimerRef.current = null;
    }
    if (snapTransitionTimerRef.current) {
      clearTimeout(snapTransitionTimerRef.current);
      snapTransitionTimerRef.current = null;
    }
    snapOffsetRef.current = 0;
    touchStartRef.current = { mode: calendarMode, startX: touch.clientX, startY: touch.clientY };
    setCalendarSnap({ mode: calendarMode, offsetPx: 0, isSettling: false });
  }, [calendarMode]);

  const handleCalendarTouchMove = useCallback((event: TouchEvent<HTMLElement>) => {
    const start = touchStartRef.current;
    const touch = event.touches[0];
    if (!start || !touch || start.mode !== calendarMode) return;

    const unitPx = getCalendarSnapUnitPx(start.mode, event.currentTarget);
    const rawOffset = start.mode === "month" ? touch.clientY - start.startY : touch.clientX - start.startX;
    setCalendarSnapOffset(start.mode, clampCalendarSnapOffset(rawOffset, unitPx));
  }, [calendarMode, getCalendarSnapUnitPx, setCalendarSnapOffset]);

  const handleCalendarTouchEnd = useCallback((event: TouchEvent<HTMLElement>) => {
    const start = touchStartRef.current;
    touchStartRef.current = null;
    if (!start || start.mode !== calendarMode) return;

    const unitPx = getCalendarSnapUnitPx(start.mode, event.currentTarget);
    settleCalendarSnap(start.mode, unitPx);
  }, [calendarMode, getCalendarSnapUnitPx, settleCalendarSnap]);

  const handleCalendarTouchCancel = useCallback((event: TouchEvent<HTMLElement>) => {
    const start = touchStartRef.current;
    touchStartRef.current = null;
    if (!start || start.mode !== calendarMode) return;

    const unitPx = getCalendarSnapUnitPx(start.mode, event.currentTarget);
    snapOffsetRef.current = 0;
    setCalendarSnap({ mode: start.mode, offsetPx: 0, isSettling: true });
    snapTransitionTimerRef.current = setTimeout(() => {
      setCalendarSnap(null);
      snapTransitionTimerRef.current = null;
    }, SNAP_TRANSITION_MS);
  }, [calendarMode, getCalendarSnapUnitPx]);

  const handleWeekTimelineScroll = useCallback((event: UIEvent<HTMLDivElement>) => {
    const timeGutter = weekTimeGutterScrollRef.current;
    if (!timeGutter) return;

    const nextScrollTop = event.currentTarget.scrollTop;
    if (timeGutter.scrollTop !== nextScrollTop) {
      timeGutter.scrollTop = nextScrollTop;
    }
  }, []);

  const handleSelectPost = useCallback((postId: string, target: HTMLElement) => {
    setDayOverflowTarget(null);
    setTimedOverflowTarget(null);
    setSelectedPostTarget({
      postId,
      anchorRect: getElementRect(target),
      boundsRect: getCalendarEditorBoundsRect(target),
    });
  }, []);

  const handleSelectDayOverflow = useCallback((dateKey: string, date: Date, target: HTMLElement) => {
    const hiddenPosts = getMonthDayPostLayout(postsByDate.get(dateKey) || []).hiddenPosts;
    if (hiddenPosts.length === 0) return;

    setTimedOverflowTarget(null);
    setSelectedPostTarget(null);
    setDayOverflowTarget({
      dateKey,
      dayLabel: formatOverflowDayLabel(date),
      anchorRect: getElementRect(target),
      boundsRect: getCalendarEditorBoundsRect(target),
    });
  }, [postsByDate]);

  const handleSelectTimedOverflow = useCallback((groupPosts: SocialPost[], target: HTMLElement) => {
    if (groupPosts.length === 0) return;
    if (groupPosts.length === 1) {
      handleSelectPost(groupPosts[0]!.id, target);
      return;
    }

    const firstPost = groupPosts[0]!;
    const date = getPostDate(firstPost);
    const time = formatPostTime(firstPost);

    setSelectedPostTarget(null);
    setDayOverflowTarget(null);
    setTimedOverflowTarget({
      postIds: groupPosts.map((post) => post.id),
      dateLabel: date ? `${formatOverflowDayLabel(date)}${time ? `, ${time}` : ""}` : "Timed posts",
      anchorRect: getElementRect(target),
      boundsRect: getCalendarEditorBoundsRect(target),
    });
  }, [handleSelectPost]);

  const handleSelectOverflowPost = useCallback((postId: string, target: HTMLElement) => {
    if (!dayOverflowTarget) return;

    setDayOverflowTarget(null);
    setSelectedPostTarget({
      postId,
      anchorRect: getElementRect(target),
      boundsRect: dayOverflowTarget.boundsRect,
    });
  }, [dayOverflowTarget]);

  const handleSelectTimedOverflowPost = useCallback((postId: string, target: HTMLElement) => {
    if (!timedOverflowTarget) return;

    setTimedOverflowTarget(null);
    setSelectedPostTarget({
      postId,
      anchorRect: getElementRect(target),
      boundsRect: timedOverflowTarget.boundsRect,
    });
  }, [timedOverflowTarget]);

  const closeSelectedPost = useCallback(() => {
    setSelectedPostTarget(null);
  }, []);

  const closeDayOverflow = useCallback(() => {
    setDayOverflowTarget(null);
  }, []);

  const closeTimedOverflow = useCallback(() => {
    setTimedOverflowTarget(null);
  }, []);

  const closeEditPost = useCallback(() => {
    setEditingPostTarget(null);
  }, []);

  const openEditPost = useCallback(() => {
    if (!selectedPostTarget) return;
    setEditingPostTarget(selectedPostTarget);
    setSelectedPostTarget(null);
  }, [selectedPostTarget]);

  const getCalendarSnapStyle = (
    mode: Extract<CalendarViewMode, "month" | "week">,
    extra: CSSProperties = {},
  ) => ({
    ...extra,
    "--calendar-snap-offset": `${calendarSnap?.mode === mode ? calendarSnap.offsetPx : 0}px`,
    "--calendar-snap-duration": calendarSnap?.mode === mode && calendarSnap.isSettling ? `${SNAP_TRANSITION_MS}ms` : "0ms",
  }) as CSSProperties;

  const renderMonthWeekdayHeader = () => (
    <div className="posts-calendar-month-weekdays" aria-hidden="true">
      {WEEKDAYS.map((weekday, index) => (
        <div key={weekday} className={`posts-calendar-weekday ${index === 0 || index === 6 ? "weekend" : ""}`}>
          {weekday}
        </div>
      ))}
    </div>
  );

  const renderMonthDayGrid = (
    cells: CalendarDayCell[] = monthCells,
    className = "posts-calendar-month-days",
  ) => (
    <div className={className}>
      {cells.map((cell) => {
        const dayPosts = postsByDate.get(cell.dateKey) || [];
        const dayLayout = getMonthDayPostLayout(dayPosts);
        const overflowStatusColor = getOverflowStatusColor(dayLayout.hiddenPosts);
        return (
          <div
            key={cell.dateKey}
            className={`posts-calendar-day ${cell.isCurrentMonth ? "" : "outside"} ${cell.isToday ? "today" : ""} ${isWeekendDate(cell.date) ? "weekend" : ""}`}
          >
            <div className="posts-calendar-day-number">
              <span>{cell.dayOfMonth}</span>
            </div>
            <div className="posts-calendar-events">
              {dayLayout.visiblePosts.map((post) => (
                <CalendarEventButton
                  key={post.id}
                  post={post}
                  profilesById={profilesById}
                  profileColors={profileColors}
                  timezone={timezone}
                  onClick={(event) => handleSelectPost(post.id, event.currentTarget)}
                />
              ))}
              {dayLayout.hiddenCount > 0 ? (
                <button
                  type="button"
                  className="posts-calendar-more posts-calendar-more-pill"
                  style={{ "--event-status-color": overflowStatusColor } as CSSProperties}
                  aria-haspopup="dialog"
                  aria-expanded={dayOverflowTarget?.dateKey === cell.dateKey}
                  aria-label={`${dayLayout.hiddenCount} more posts on ${formatOverflowDayLabel(cell.date)}`}
                  onClick={(event) => handleSelectDayOverflow(cell.dateKey, cell.date, event.currentTarget)}
                >
                  +{dayLayout.hiddenCount}
                </button>
              ) : null}
            </div>
          </div>
        );
      })}
    </div>
  );

  const renderMonthView = (
    cells: CalendarDayCell[] = monthCells,
    {
      interactive = true,
      ariaLabel = `${calendarTitle} posts`,
    }: { interactive?: boolean; ariaLabel?: string } = {},
  ) => (
    <div
      className="posts-calendar-month-shell"
      aria-label={ariaLabel}
      onWheel={interactive ? handleCalendarWheel : undefined}
      onTouchStart={interactive ? handleCalendarTouchStart : undefined}
      onTouchMove={interactive ? handleCalendarTouchMove : undefined}
      onTouchEnd={interactive ? handleCalendarTouchEnd : undefined}
      onTouchCancel={interactive ? handleCalendarTouchCancel : undefined}
      style={getCalendarSnapStyle("month")}
    >
      {renderMonthWeekdayHeader()}
      <div className="posts-calendar-month-view">
        {renderMonthDayGrid(cells, "posts-calendar-month-days posts-calendar-month-track")}
      </div>
    </div>
  );

  const renderWeekTimeGutter = (attachScrollRef = true) => (
    <div className="posts-calendar-week-time-gutter" aria-hidden="true">
      <div
        ref={attachScrollRef ? weekTimeGutterScrollRef : null}
        className="posts-calendar-week-time-label-scroll"
      >
        <TimeLabels />
      </div>
    </div>
  );

  const renderWeekHeader = (days: CalendarDayCell[] = rollingWeekDays) => (
    <div className="posts-calendar-week-header" aria-hidden="true">
      <div className="posts-calendar-week-header-gutter" />
      <div className="posts-calendar-week-header-row">
        <div className="posts-calendar-week-header-inner posts-calendar-week-track">
          {days.map((day) => (
            <div key={day.dateKey} className={`posts-calendar-week-heading ${day.isToday ? "today" : ""}`}>
              <span>{formatWeekdayShort(day.date)}</span>
              <strong>{day.dayOfMonth}</strong>
            </div>
          ))}
        </div>
        <div className="posts-calendar-week-scrollbar-spacer" aria-hidden="true" />
      </div>
    </div>
  );

  const renderWeekColumns = (
    days: CalendarDayCell[] = rollingWeekDays,
    {
      attachScrollbarRef = true,
      syncTimeGutter = true,
    }: { attachScrollbarRef?: boolean; syncTimeGutter?: boolean } = {},
  ) => (
    <div className="posts-calendar-week-content">
      <div
        ref={attachScrollbarRef ? weekTimeScrollRef : null}
        className="posts-calendar-time-scroll"
        style={timelineStyle}
        onScroll={syncTimeGutter ? handleWeekTimelineScroll : undefined}
      >
        <div className="posts-calendar-week-columns posts-calendar-week-track">
          {days.map((day) => (
            <TimedPostColumn
              key={day.dateKey}
              posts={postsByDate.get(day.dateKey) || []}
              profilesById={profilesById}
              profileColors={profileColors}
              isWeekend={isWeekendDate(day.date)}
              timezone={timezone}
              onSelectPost={handleSelectPost}
              onSelectTimedOverflow={handleSelectTimedOverflow}
            />
          ))}
        </div>
      </div>
    </div>
  );

  const renderWeekView = (
    days: CalendarDayCell[] = weekDays,
    {
      interactive = true,
      attachScrollbarRef = true,
      ariaLabel = `${calendarTitle} week posts`,
    }: { interactive?: boolean; attachScrollbarRef?: boolean; ariaLabel?: string } = {},
  ) => (
    <div
      ref={interactive ? weekShellRef : null}
      className="posts-calendar-week-shell"
      aria-label={ariaLabel}
      onWheel={interactive ? handleCalendarWheel : undefined}
      onTouchStart={interactive ? handleCalendarTouchStart : undefined}
      onTouchMove={interactive ? handleCalendarTouchMove : undefined}
      onTouchEnd={interactive ? handleCalendarTouchEnd : undefined}
      onTouchCancel={interactive ? handleCalendarTouchCancel : undefined}
      style={getCalendarSnapStyle("week", { "--calendar-scrollbar-gutter": `${weekScrollbarWidth}px` } as CSSProperties)}
    >
      {renderWeekHeader(interactive ? rollingWeekDays : days)}
      <div className="posts-calendar-week-grid">
        <div className="posts-calendar-week-body">
          {renderWeekTimeGutter(attachScrollbarRef)}
          {renderWeekColumns(interactive ? rollingWeekDays : days, { attachScrollbarRef, syncTimeGutter: attachScrollbarRef })}
        </div>
      </div>
    </div>
  );

  const renderDayView = () => (
    <div className={`posts-calendar-day-grid ${isWeekendDate(visibleDate) ? "weekend" : ""}`} aria-label={`${calendarTitle} day posts`}>
      <div className="posts-calendar-time-scroll" style={timelineStyle}>
        <TimeLabels />
        <div className="posts-calendar-day-column-wrap">
          <TimedPostColumn
            posts={postsByDate.get(dayDateKey) || []}
            profilesById={profilesById}
            profileColors={profileColors}
            isWeekend={isWeekendDate(visibleDate)}
            timezone={timezone}
            onSelectPost={handleSelectPost}
            onSelectTimedOverflow={handleSelectTimedOverflow}
          />
        </div>
      </div>
    </div>
  );

  return (
    <section className="posts-calendar-fullheight" aria-label="Posts calendar">
      <style>{CALENDAR_CSS}</style>
      <aside
        id="posts-calendar-filters"
        className="posts-calendar-sidebar"
        data-mobile-open={mobileFiltersOpen ? "true" : "false"}
        aria-label="Calendar filters"
      >
        <div className="posts-calendar-sidebar-top">
          <div className="posts-calendar-sidebar-kicker">Posts</div>
          <div className="posts-calendar-sidebar-title">Calendar</div>
        </div>

        <FilterSection title="Profiles">
          {profiles.length === 0 ? (
            <div className="posts-calendar-muted">No profiles found</div>
          ) : (
            profiles.map((profile) => {
              const color = profileColors.get(profile.id) || getProfileCalendarColor(profile);
              return (
                <label
                  key={profile.id}
                  className="posts-calendar-check"
                  style={{ "--profile-color": color } as CSSProperties}
                >
                  <input
                    type="checkbox"
                    checked={selectedProfileIds.has(profile.id)}
                    onChange={() => setSelectedProfileIds((current) => toggleSetValue(current, profile.id))}
                  />
                  <span className="posts-calendar-checkmark" />
                  <span className="posts-calendar-check-label">{profile.name}</span>
                </label>
              );
            })
          )}
        </FilterSection>

        <FilterSection title="Platforms">
          {platformOptions.length === 0 ? (
            <div className="posts-calendar-muted">No connected platforms</div>
          ) : (
            platformOptions.map((platform) => (
              <label key={platform} className="posts-calendar-platform-check">
                <input
                  type="checkbox"
                  checked={selectedPlatforms.has(platform)}
                  onChange={() => setSelectedPlatforms((current) => toggleSetValue(current, platform))}
                />
                <span className="posts-calendar-checkmark neutral" />
                <AccountDestinationIcon platform={platform} size={15} />
                <span>{formatPlatformName(platform)}</span>
              </label>
            ))
          )}
        </FilterSection>

        <FilterSection title="Status">
          <div className="posts-calendar-status-list">
            {STATUS_FILTERS.map((status) => (
              <button
                key={status.value}
                type="button"
                className={`posts-calendar-status-option ${statusFilter === status.value ? "active" : ""}`}
                onClick={() => setStatusFilter(status.value)}
                aria-pressed={statusFilter === status.value}
              >
                <span>{status.label}</span>
              </button>
            ))}
          </div>
        </FilterSection>
      </aside>

      <div className="posts-calendar-main">
        <div className="posts-calendar-topbar">
          <div className="posts-calendar-title-block">
            <h1>{calendarTitle}</h1>
            <span>{calendarSubtitle}</span>
          </div>

          <div className="posts-calendar-toolbar" aria-label="Calendar controls">
            <button
              type="button"
              className="posts-calendar-filter-toggle"
              aria-controls="posts-calendar-filters"
              aria-expanded={mobileFiltersOpen}
              onClick={() => setMobileFiltersOpen((open) => !open)}
            >
              <SlidersHorizontal size={16} />
              Filters
            </button>

            <div className="posts-calendar-segment" aria-label="Calendar view mode">
              <button
                type="button"
                className={calendarMode === "day" ? "active" : ""}
                onClick={() => {
                  replaceCalendarMode("day");
                  setVisibleMonth(startOfMonth(visibleDate));
                }}
              >
                Day
              </button>
              <button
                type="button"
                className={calendarMode === "week" ? "active" : ""}
                onClick={() => {
                  replaceCalendarMode("week");
                  setVisibleMonth(startOfMonth(visibleDate));
                }}
              >
                Week
              </button>
              <button
                type="button"
                className={calendarMode === "month" ? "active" : ""}
                onClick={() => {
                  replaceCalendarMode("month");
                  setVisibleMonth(startOfMonth(visibleDate));
                }}
              >
                Month
              </button>
            </div>

            <div className="posts-calendar-month-nav">
              <button type="button" aria-label={`Previous ${calendarMode}`} onClick={() => shiftCalendar(-1)}>
                <ChevronLeft size={16} />
              </button>
              <button type="button" onClick={goToToday}>Today</button>
              <button type="button" aria-label={`Next ${calendarMode}`} onClick={() => shiftCalendar(1)}>
                <ChevronRight size={16} />
              </button>
            </div>

            <Link className="posts-calendar-list-link" href={`/projects/${profileId}/posts/list`}>
              <List size={16} />
              List View
            </Link>

            <button type="button" className="posts-calendar-create" onClick={() => setDrawerOpen(true)}>
              <Plus size={16} />
              Create +
            </button>
          </div>
        </div>

        {error ? <div className="posts-calendar-error">{error}</div> : null}

        <div className="posts-calendar-view-stage">
          {calendarMode === "month" ? renderMonthView() : null}
          {calendarMode === "week" ? renderWeekView() : null}
          {calendarMode === "day" ? renderDayView() : null}
        </div>
      </div>

      {dayOverflowTarget && dayOverflowPosts.length > 0 ? (
        <DayOverflowPopover
          dateLabel={dayOverflowTarget.dayLabel}
          summaryLabel={`${dayOverflowPosts.length} post${dayOverflowPosts.length === 1 ? "" : "s"} on this day`}
          posts={dayOverflowPosts}
          anchorRect={dayOverflowTarget.anchorRect}
          boundsRect={dayOverflowTarget.boundsRect}
          profilesById={profilesById}
          profileColors={profileColors}
          timezone={timezone}
          onClose={closeDayOverflow}
          onSelectPost={handleSelectOverflowPost}
        />
      ) : null}

      {timedOverflowTarget && timedOverflowPosts.length > 0 ? (
        <DayOverflowPopover
          dateLabel={timedOverflowTarget.dateLabel}
          summaryLabel={`${timedOverflowPosts.length} post${timedOverflowPosts.length === 1 ? "" : "s"} at this time`}
          ariaLabel={`More posts at ${timedOverflowTarget.dateLabel}`}
          posts={timedOverflowPosts}
          anchorRect={timedOverflowTarget.anchorRect}
          boundsRect={timedOverflowTarget.boundsRect}
          profilesById={profilesById}
          profileColors={profileColors}
          timezone={timezone}
          onClose={closeTimedOverflow}
          onSelectPost={handleSelectTimedOverflowPost}
        />
      ) : null}

      {selectedPost && selectedPostTarget ? (
        <EventPopover
          post={selectedPost}
          profileId={profileId}
          anchorRect={selectedPostTarget.anchorRect}
          boundsRect={selectedPostTarget.boundsRect}
          profile={getPrimaryProfile(selectedPost, profilesById)}
          color={getPostColor(selectedPost, profilesById, profileColors)}
          timezone={timezone}
          editable={isEditableCalendarPost(selectedPost)}
          onRetryComplete={loadData}
          onClose={closeSelectedPost}
          onEdit={openEditPost}
        />
      ) : null}

      {editingPost && editingPostTarget ? (
        <CalendarEditInspector
          post={editingPost}
          anchorRect={editingPostTarget.anchorRect}
          boundsRect={editingPostTarget.boundsRect}
          accounts={accounts}
          profile={getPrimaryProfile(editingPost, profilesById)}
          color={getPostColor(editingPost, profilesById, profileColors)}
          getToken={getToken}
          onClose={closeEditPost}
          onSaved={handleEdited}
        />
      ) : null}

      <CreatePostDrawer
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        accounts={currentProfileAccounts}
        workspaceId={workspaceId}
        profileName={profilesById.get(profileId)?.name}
        getToken={getToken}
        onCreated={handleCreated}
      />
    </section>
  );
}

function FilterSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="posts-calendar-filter-section">
      <h2>{title}</h2>
      <div className="posts-calendar-filter-body">{children}</div>
    </section>
  );
}

function getCalendarEventAccessibleLabel(
  post: SocialPost,
  meta: { label: string; short: string },
  profile: Profile | null,
  timezone: string,
): string {
  const caption = post.caption || "No title";
  const platforms = getPostPlatforms(post).map(formatPlatformLabel).join(", ") || "No platforms";
  return [
    caption,
    `Status: ${meta.label}`,
    `Profile: ${profile?.name || "No profile"}`,
    `Platforms: ${platforms}`,
    `Date: ${formatPostDateTime(post)}`,
    `Timezone: ${timezone}`,
  ].join(". ");
}

function CalendarEventButton({
  post,
  profilesById,
  profileColors,
  timezone,
  onClick,
}: {
  post: SocialPost;
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  timezone: string;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const profile = getPrimaryProfile(post, profilesById);
  const color = getPostColor(post, profilesById, profileColors);
  const statusColor = getCalendarStatusColor(status);
  const time = formatPostTime(post);
  const accessibleLabel = getCalendarEventAccessibleLabel(post, meta, profile, timezone);
  return (
    <button
      type="button"
      className="posts-calendar-event"
      style={{
        "--event-profile-color": color,
        "--event-status-color": statusColor,
      } as CSSProperties}
      onClick={onClick}
      aria-label={accessibleLabel}
      title={accessibleLabel}
    >
      <span className="posts-calendar-event-rail" />
      <span className="posts-calendar-event-status">{meta.short}</span>
      <span className="posts-calendar-event-caption">{post.caption || "No title"}</span>
      {time ? <span className="posts-calendar-event-time">{time}</span> : null}
    </button>
  );
}

function TimeLabels() {
  return (
    <div className="posts-calendar-time-labels" aria-hidden="true">
      {HOURS.map((hour) => (
        <div key={hour} className="posts-calendar-time-label">
          {formatHourLabel(hour)}
        </div>
      ))}
    </div>
  );
}

function TimedPostColumn({
  posts,
  profilesById,
  profileColors,
  isWeekend,
  timezone,
  onSelectPost,
  onSelectTimedOverflow,
}: {
  posts: SocialPost[];
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  isWeekend: boolean;
  timezone: string;
  onSelectPost: (postId: string, target: HTMLElement) => void;
  onSelectTimedOverflow: (posts: SocialPost[], target: HTMLElement) => void;
}) {
  const postsById = new Map(posts.map((post) => [post.id, post]));
  const postTimesById = new Map<string, number>();
  const timedGroups = getTimedPostGroups(
    posts.flatMap((post) => {
      const minute = getCalendarPostMinuteOfDay(post);
      if (minute !== null) postTimesById.set(post.id, minute);
      return minute === null ? [] : [{ id: post.id, minuteOfDay: minute }];
    }),
  );
  const timedLayoutInputs = timedGroups.flatMap((group) => {
    const visiblePostIds = group.postIds.slice(0, TIMED_GROUP_VISIBLE_POST_LIMIT);
    return visiblePostIds.flatMap((postId) => {
      const minuteOfDay = postTimesById.get(postId);
      return minuteOfDay === undefined ? [] : [{ id: postId, minuteOfDay }];
    });
  });
  const eventLayouts = getTimedEventLayouts(
    timedLayoutInputs,
    HOUR_HEIGHT,
    TIMED_EVENT_MIN_HEIGHT,
  );

  return (
    <div className={`posts-calendar-time-column ${isWeekend ? "weekend" : ""}`}>
      {timedGroups.map((group) => {
        const groupPosts = group.postIds.flatMap((postId) => {
          const groupedPost = postsById.get(postId);
          return groupedPost ? [groupedPost] : [];
        });
        if (groupPosts.length === 0) return null;
        const visibleGroupPosts = groupPosts.slice(0, TIMED_GROUP_VISIBLE_POST_LIMIT);
        const overflowCount = Math.max(0, groupPosts.length - TIMED_GROUP_VISIBLE_POST_LIMIT);
        const overflowLayout = visibleGroupPosts.length > 0
          ? eventLayouts.get(visibleGroupPosts[0].id)
          : undefined;
        return (
          <Fragment key={group.id}>
            {visibleGroupPosts.map((post) => {
              const layout = eventLayouts.get(post.id);
              if (!layout) return null;
              return (
                <TimedPostButton
                  key={post.id}
                  post={post}
                  profilesById={profilesById}
                  profileColors={profileColors}
                  layout={layout}
                  timezone={timezone}
                  hasOverflow={overflowCount > 0}
                  onClick={(event) => onSelectPost(post.id, event.currentTarget)}
                />
              );
            })}
            {overflowCount > 0 && overflowLayout ? (
              <TimedOverflowButton
                groupPosts={groupPosts}
                layout={overflowLayout}
                overflowCount={overflowCount}
                onClick={(event) => onSelectTimedOverflow(groupPosts, event.currentTarget)}
              />
            ) : null}
          </Fragment>
        );
      })}
    </div>
  );
}

function TimedPostButton({
  post,
  profilesById,
  profileColors,
  layout,
  timezone,
  hasOverflow,
  onClick,
}: {
  post: SocialPost;
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  layout: TimedCalendarEventLayout;
  timezone: string;
  hasOverflow: boolean;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const profile = getPrimaryProfile(post, profilesById);
  const color = getPostColor(post, profilesById, profileColors);
  const statusColor = getCalendarStatusColor(status);
  const time = formatPostTime(post);
  const accessibleLabel = getCalendarEventAccessibleLabel(post, meta, profile, timezone);
  return (
    <button
      type="button"
      className={`posts-calendar-timed-event ${hasOverflow ? "has-overflow-pill" : ""}`}
      style={{
        "--event-profile-color": color,
        "--event-status-color": statusColor,
        top: `${Math.max(4, layout.top)}px`,
        left: `calc(${layout.leftPercent}% + 6px)`,
        width: `calc(${layout.widthPercent}% - 12px)`,
        zIndex: layout.lane + 1,
      } as CSSProperties}
      onClick={onClick}
      aria-label={accessibleLabel}
      title={accessibleLabel}
    >
      <span className="posts-calendar-event-rail" />
      <span className="posts-calendar-timed-content">
        <span className="posts-calendar-timed-title">{post.caption || "No title"}</span>
        <span className="posts-calendar-timed-meta">{meta.short}{time ? ` - ${time}` : ""}</span>
      </span>
    </button>
  );
}

function TimedOverflowButton({
  groupPosts,
  layout,
  overflowCount,
  onClick,
}: {
  groupPosts: SocialPost[];
  layout: TimedCalendarEventLayout;
  overflowCount: number;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const statusColor = getOverflowStatusColor(groupPosts.slice(TIMED_GROUP_VISIBLE_POST_LIMIT));
  const time = formatPostTime(groupPosts[TIMED_GROUP_VISIBLE_POST_LIMIT] || groupPosts[0]);
  const label = `${overflowCount} more posts at ${time || "this hour"}`;
  return (
    <button
      type="button"
      className="posts-calendar-timed-overflow posts-calendar-more-pill"
      style={{
        "--event-status-color": statusColor,
        top: `${Math.max(4, layout.top + 8)}px`,
        right: `calc(${100 - layout.leftPercent - layout.widthPercent}% + 8px)`,
        zIndex: layout.lane + 12,
      } as CSSProperties}
      onClick={onClick}
      aria-label={label}
      title={label}
    >
      +{overflowCount}
    </button>
  );
}

function DayOverflowPopover({
  dateLabel,
  posts,
  anchorRect,
  boundsRect,
  profilesById,
  profileColors,
  timezone,
  summaryLabel,
  ariaLabel,
  onClose,
  onSelectPost,
}: {
  dateLabel: string;
  posts: SocialPost[];
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  timezone: string;
  summaryLabel?: string;
  ariaLabel?: string;
  onClose: () => void;
  onSelectPost: (postId: string, target: HTMLElement) => void;
}) {
  const popoverRef = useRef<HTMLElement | null>(null);
  const [viewportSize, setViewportSize] = useState<CalendarPopoverSize>(() => getViewportSize());
  const [popoverSize, setPopoverSize] = useState<CalendarPopoverSize>(DAY_OVERFLOW_POPOVER_FALLBACK_SIZE);

  useLayoutEffect(() => {
    const updateGeometry = () => {
      setViewportSize(getViewportSize());
      const rect = popoverRef.current?.getBoundingClientRect();
      if (rect && rect.width > 0 && rect.height > 0) {
        setPopoverSize({ width: rect.width, height: rect.height });
      }
    };

    updateGeometry();
    const resizeObserver = typeof ResizeObserver === "undefined" || !popoverRef.current
      ? null
      : new ResizeObserver(updateGeometry);
    if (popoverRef.current) resizeObserver?.observe(popoverRef.current);
    window.addEventListener("resize", updateGeometry);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateGeometry);
    };
  }, [posts.length]);

  const placement = useMemo(
    () => getBoundedCalendarPopoverPlacement({
      anchor: anchorRect,
      viewport: viewportSize,
      popover: popoverSize,
      bounds: boundsRect,
      verticalStrategy: "anchor",
    }),
    [anchorRect, boundsRect, popoverSize, viewportSize],
  );
  const popoverStyle = {
    "--event-profile-color": "#8b8b93",
    "--event-status-color": "#475569",
    "--popover-left": `${placement.left}px`,
    "--popover-top": `${placement.top}px`,
    "--popover-available-height": `${placement.availableHeight}px`,
    "--popover-arrow-x": `${placement.arrowX}px`,
    "--popover-arrow-y": `${placement.arrowY}px`,
    "--popover-transform-origin": placement.transformOrigin,
  } as CSSProperties;

  return (
    <div className="posts-calendar-popover-layer" role="presentation" onMouseDown={onClose}>
      <article
        ref={popoverRef}
        className="posts-calendar-popover posts-calendar-more-popover"
        data-side={placement.side}
        role="dialog"
        aria-label={ariaLabel || `More posts on ${dateLabel}`}
        onMouseDown={(event) => event.stopPropagation()}
        style={popoverStyle}
      >
        <div className="posts-calendar-popover-content">
          <div className="posts-calendar-popover-head">
            <div>
              <div className="posts-calendar-popover-profile">
                <span />
                {dateLabel}
              </div>
              <h2>More posts</h2>
              <p className="posts-calendar-more-summary">
                {summaryLabel || `${posts.length} hidden post${posts.length === 1 ? "" : "s"}`}
              </p>
            </div>
            <button type="button" aria-label="Close more posts" onClick={onClose}>
              <X size={16} />
            </button>
          </div>

          <div className="posts-calendar-more-list">
            {posts.map((post) => (
              <CalendarEventButton
                key={post.id}
                post={post}
                profilesById={profilesById}
                profileColors={profileColors}
                timezone={timezone}
                onClick={(event) => onSelectPost(post.id, event.currentTarget)}
              />
            ))}
          </div>
        </div>
      </article>
    </div>
  );
}

function EventPopover({
  post,
  profileId,
  anchorRect,
  boundsRect,
  profile,
  color,
  timezone,
  editable,
  onRetryComplete,
  onClose,
  onEdit,
}: {
  post: SocialPost;
  profileId: string;
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
  profile: Profile | null;
  color: string;
  timezone: string;
  editable: boolean;
  onRetryComplete: () => void | Promise<void>;
  onClose: () => void;
  onEdit: () => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const statusColor = getCalendarStatusColor(status);
  const platforms = getPostPlatforms(post);
  const popoverRef = useRef<HTMLElement | null>(null);
  const [viewportSize, setViewportSize] = useState<CalendarPopoverSize>(() => getViewportSize());
  const [popoverSize, setPopoverSize] = useState<CalendarPopoverSize>(POPOVER_FALLBACK_SIZE);

  useLayoutEffect(() => {
    const updateGeometry = () => {
      setViewportSize(getViewportSize());
      const rect = popoverRef.current?.getBoundingClientRect();
      if (rect && rect.width > 0 && rect.height > 0) {
        setPopoverSize({ width: rect.width, height: rect.height });
      }
    };

    updateGeometry();
    const resizeObserver = typeof ResizeObserver === "undefined" || !popoverRef.current
      ? null
      : new ResizeObserver(updateGeometry);
    if (popoverRef.current) resizeObserver?.observe(popoverRef.current);
    window.addEventListener("resize", updateGeometry);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateGeometry);
    };
  }, [post.id]);

  const placement = useMemo(
    () => getBoundedCalendarPopoverPlacement({
      anchor: anchorRect,
      viewport: viewportSize,
      popover: popoverSize,
      bounds: boundsRect,
      verticalStrategy: "anchor",
    }),
    [anchorRect, boundsRect, popoverSize, viewportSize],
  );
  const popoverStyle = {
    "--event-profile-color": color,
    "--event-status-color": statusColor,
    "--popover-left": `${placement.left}px`,
    "--popover-top": `${placement.top}px`,
    "--popover-available-height": `${placement.availableHeight}px`,
    "--popover-arrow-x": `${placement.arrowX}px`,
    "--popover-arrow-y": `${placement.arrowY}px`,
    "--popover-transform-origin": placement.transformOrigin,
  } as CSSProperties;

  return (
    <div className="posts-calendar-popover-layer" role="presentation" onMouseDown={onClose}>
      <article
        ref={popoverRef}
        className="posts-calendar-popover"
        data-side={placement.side}
        role="dialog"
        aria-label="Post details"
        onMouseDown={(event) => event.stopPropagation()}
        style={popoverStyle}
      >
        <div className="posts-calendar-popover-content">
          <div className="posts-calendar-popover-head">
            <div>
              <div className="posts-calendar-popover-profile">
                <span />
                {profile?.name || "Unassigned profile"}
              </div>
              <h2>{post.caption || "No title"}</h2>
            </div>
            <button type="button" aria-label="Close post details" onClick={onClose}>
              <X size={16} />
            </button>
          </div>

          <dl className="posts-calendar-popover-meta">
            <div>
              <dt>Status</dt>
              <dd><span className="posts-calendar-popover-status">{meta.short}</span>{meta.label}</dd>
            </div>
            <div>
              <dt>Time</dt>
              <dd>{formatPostDateTime(post)} {timezone}</dd>
            </div>
            <div>
              <dt>Platforms</dt>
              <dd>
                {platforms.length > 0 ? (
                  <span className="posts-calendar-popover-platforms">
                    {platforms.map((platform) => (
                      <span key={platform} className="posts-calendar-popover-platform-chip">
                        <AccountDestinationIcon platform={platform} size={14} />
                        {formatPlatformName(platform)}
                      </span>
                    ))}
                  </span>
                ) : (
                  "No platforms"
                )}
              </dd>
            </div>
          </dl>

          {post.status === "quota_hold" ? (
            <div className="posts-calendar-hold-notice">
              This post is preserved but will not publish while it is on quota hold. Move or cancel scheduled posts,
              wait for the monthly reset, or upgrade the workspace plan.
            </div>
          ) : null}

          <CalendarPostDetailGrid post={post} meta={meta} />
          <section className="posts-calendar-results">
            <div className="posts-calendar-results-label">Platform results</div>
            <PostPlatformResults
              post={post}
              workspaceId={profileId}
              layout="stack"
              onRetryComplete={onRetryComplete}
            />
          </section>

          <div className="posts-calendar-popover-actions">
            <Link
              className="posts-calendar-open-list"
              href={`/projects/${profileId}/posts/list?post=${encodeURIComponent(post.id)}`}
              onClick={(event) => event.stopPropagation()}
            >
              <List size={14} />
              Open in List
            </Link>
            <button
              type="button"
              className="posts-calendar-open-list"
              onClick={onEdit}
              disabled={!editable}
            >
              {editable ? "Edit" : "View only"}
            </button>
          </div>
        </div>
      </article>
    </div>
  );
}

function CalendarPostDetailGrid({
  post,
  meta,
}: {
  post: SocialPost;
  meta: { label: string; short: string };
}) {
  const mode = post.scheduled_at ? "Scheduled" : post.status === "draft" ? "Draft" : "Immediate";
  return (
    <div className="posts-calendar-detail-grid">
      <CalendarPostMetaCard label="Caption" value={post.caption || "(no caption)"} />
      <CalendarPostMetaCard label="Mode" value={mode} />
      <CalendarPostMetaCard label="Status" value={meta.label} />
      <CalendarPostMetaCard label="Created" value={formatCalendarDetailDate(post.created_at)} />
      <CalendarPostMetaCard label="Scheduled" value={post.scheduled_at ? formatCalendarDetailDate(post.scheduled_at) : "-"} />
      <CalendarPostMetaCard label="Published" value={post.published_at ? formatCalendarDetailDate(post.published_at) : "-"} />
    </div>
  );
}

function CalendarPostMetaCard({
  label,
  value,
}: {
  label: string;
  value: ReactNode;
}) {
  return (
    <div className="posts-calendar-detail-card">
      <div>{label}</div>
      <span>{value}</span>
    </div>
  );
}

function CalendarEditInspector({
  post,
  anchorRect,
  boundsRect,
  accounts,
  profile,
  color,
  getToken,
  onClose,
  onSaved,
}: {
  post: SocialPost;
  anchorRect: CalendarPopoverRect;
  boundsRect: CalendarPopoverRect;
  accounts: SocialAccount[];
  profile: Profile | null;
  color: string;
  getToken: () => Promise<string | null>;
  onClose: () => void;
  onSaved: (postId?: string) => void | Promise<void>;
}) {
  const form = useCreatePostForm(accounts);
  const inspectorRef = useRef<HTMLElement | null>(null);
  const hydratedRef = useRef<string | null>(null);
  const [viewportSize, setViewportSize] = useState<CalendarPopoverSize>(() => getViewportSize());
  const [inspectorSize, setInspectorSize] = useState<CalendarPopoverSize>({ width: 760, height: 680 });
  const [platformCapabilities, setPlatformCapabilities] = useState<PlatformCapabilitiesEnvelope["platforms"] | null>(null);
  const [validationResult, setValidationResult] = useState<SocialPostValidationResult | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [tiktokBlockers, setTiktokBlockers] = useState<Record<string, string>>({});
  const [tiktokMaxByAccount, setTiktokMaxByAccount] = useState<Record<string, number>>({});

  useEffect(() => {
    const key = `${post.id}:${accounts.map((account) => account.id).join(",")}`;
    if (hydratedRef.current === key) return;
    form.hydrateFromPost(post, accounts);
    hydratedRef.current = key;
  }, [accounts, form, post]);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await getPlatformCapabilities();
        if (!cancelled) setPlatformCapabilities(res.data.platforms);
      } catch {
        if (!cancelled) setPlatformCapabilities(null);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  useLayoutEffect(() => {
    const updateGeometry = () => {
      setViewportSize(getViewportSize());
      const rect = inspectorRef.current?.getBoundingClientRect();
      if (rect && rect.width > 0 && rect.height > 0) {
        setInspectorSize({ width: rect.width, height: Math.min(rect.height, window.innerHeight - 24) });
      }
    };

    updateGeometry();
    const resizeObserver = typeof ResizeObserver === "undefined" || !inspectorRef.current
      ? null
      : new ResizeObserver(updateGeometry);
    if (inspectorRef.current) resizeObserver?.observe(inspectorRef.current);
    window.addEventListener("resize", updateGeometry);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateGeometry);
    };
  }, [post.id]);

  useEffect(() => {
    setValidationResult(null);
    setError(null);
  }, [form.mainContent, form.selectedAccountIds, form.overrides, form.mediaItems, form.existingMediaItems, form.scheduledAt]);

  const placement = useMemo(
    () => getBoundedCalendarPopoverPlacement({
      anchor: anchorRect,
      viewport: viewportSize,
      popover: inspectorSize,
      bounds: boundsRect,
    }),
    [anchorRect, boundsRect, inspectorSize, viewportSize],
  );

  const strictestTiktokMaxSec = useMemo(() => {
    const caps: number[] = [];
    for (const id of form.selectedAccountIds) {
      const cap = tiktokMaxByAccount[id];
      if (typeof cap === "number" && cap > 0) caps.push(cap);
    }
    return caps.length ? Math.min(...caps) : null;
  }, [form.selectedAccountIds, tiktokMaxByAccount]);

  const oversizeVideos = useMemo(() => {
    if (!strictestTiktokMaxSec) return [];
    return form.mediaItems.filter((item) => typeof item.durationSec === "number" && item.durationSec > strictestTiktokMaxSec);
  }, [form.mediaItems, strictestTiktokMaxSec]);

  const mediaKind: "video" | "photo" | "none" = useMemo(() => {
    if (form.mediaItems.length === 0) return "none";
    return form.mediaItems.some((item) => item.file.type.startsWith("video/")) ? "video" : "photo";
  }, [form.mediaItems]);

  const primaryVideoFile = useMemo<File | null>(() => {
    const item = form.mediaItems.find((candidate) => candidate.file.type.startsWith("video/"));
    return item?.file || null;
  }, [form.mediaItems]);

  const primaryVideoMeta = useMemo(() => {
    const item = form.mediaItems.find((candidate) => candidate.file.type.startsWith("video/"));
    if (!item) return null;
    return {
      width: item.videoWidth ?? null,
      height: item.videoHeight ?? null,
      durationSec: item.durationSec ?? null,
    };
  }, [form.mediaItems]);

  const setTiktokBlocker = useCallback((accountId: string, reason: string | null) => {
    setTiktokBlockers((prev) => {
      if (!reason) {
        if (!(accountId in prev)) return prev;
        const next = { ...prev };
        delete next[accountId];
        return next;
      }
      if (prev[accountId] === reason) return prev;
      return { ...prev, [accountId]: reason };
    });
  }, []);

  const setTiktokMaxDuration = useCallback((accountId: string, sec: number | null) => {
    setTiktokMaxByAccount((prev) => {
      if (sec == null || !Number.isFinite(sec) || sec <= 0) {
        if (!(accountId in prev)) return prev;
        const next = { ...prev };
        delete next[accountId];
        return next;
      }
      if (prev[accountId] === sec) return prev;
      return { ...prev, [accountId]: sec };
    });
  }, []);

  async function hashFile(file: File): Promise<string> {
    const buffer = await file.arrayBuffer();
    const hashBuffer = await crypto.subtle.digest("SHA-256", buffer);
    return Array.from(new Uint8Array(hashBuffer)).map((b) => b.toString(16).padStart(2, "0")).join("");
  }

  async function handleFileUpload(file: File) {
    const { cached, fingerprint } = form.addMediaItem(file);
    if (cached) return;
    try {
      if (file.type.startsWith("video/")) {
        form.updateMediaItem(fingerprint, { progress: 5 });
        const meta = await measureVideoMetadata(file);
        form.updateMediaItem(fingerprint, {
          durationSec: meta.durationSec,
          videoWidth: meta.width,
          videoHeight: meta.height,
        });
        if (strictestTiktokMaxSec && typeof meta.durationSec === "number" && meta.durationSec > strictestTiktokMaxSec) {
          form.updateMediaItem(fingerprint, { error: "TIKTOK_VIDEO_TOO_LONG", progress: 0 });
          return;
        }
      } else {
        form.updateMediaItem(fingerprint, { durationSec: null, videoWidth: null, videoHeight: null });
      }

      const token = await getToken();
      if (!token) return;
      form.updateMediaItem(fingerprint, { progress: 5 });
      const contentHash = await hashFile(file);
      form.updateMediaItem(fingerprint, { progress: 10 });
      const res = await createMedia(token, {
        filename: file.name,
        content_type: file.type || "application/octet-stream",
        size_bytes: file.size,
        content_hash: contentHash,
      });
      if (res.data.status === "uploaded") {
        form.updateMediaItem(fingerprint, { progress: 100, mediaId: res.data.id });
        return;
      }
      form.updateMediaItem(fingerprint, { progress: 30 });
      await new Promise<void>((resolve, reject) => {
        const xhr = new XMLHttpRequest();
        xhr.open("PUT", res.data.upload_url);
        xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
        xhr.upload.onprogress = (event) => {
          if (event.lengthComputable) {
            form.updateMediaItem(fingerprint, { progress: Math.round(30 + (event.loaded / event.total) * 65) });
          }
        };
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) resolve();
          else reject(new Error(`Upload failed: ${xhr.status}`));
        };
        xhr.onerror = () => reject(new Error("Upload failed"));
        xhr.send(file);
      });
      await getMedia(token, res.data.id);
      form.updateMediaItem(fingerprint, { progress: 100, mediaId: res.data.id });
    } catch (err) {
      form.updateMediaItem(fingerprint, { error: err instanceof Error ? err.message : "Upload failed", progress: 0 });
    }
  }

  async function runValidation(payload: CreateSocialPostPayload) {
    const token = await getToken();
    if (!token) {
      setError("You need to be signed in to save this post.");
      return { ok: false as const, token: null };
    }
    const res = await validateSocialPost(token, payload);
    setValidationResult(res.data);
    if (res.data.errors.length > 0) {
      setError(res.data.errors[0]?.message || "Fix validation errors before saving.");
      return { ok: false as const, token };
    }
    return { ok: true as const, token };
  }

  async function handleSave() {
    if (!form.canSubmit || saving || Object.keys(tiktokBlockers).length > 0 || oversizeVideos.length > 0) return;
    setSaving(true);
    setError(null);
    try {
      const payload = form.buildPayload();
      const validation = await runValidation(payload);
      if (!validation.ok || !validation.token) return;
      const response = await updateSocialPost(validation.token, post.id, payload);
      await onSaved(response.data.id);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save post");
    } finally {
      setSaving(false);
    }
  }

  const runtimeBlocker = Object.values(tiktokBlockers).find(Boolean);
  const disabledReason = runtimeBlocker
    || (oversizeVideos.length > 0 ? "Remove or replace videos that are too long for TikTok." : null)
    || (!form.canSubmit ? "Complete the required post fields before saving." : null);

  const inspectorStyle = {
    "--event-profile-color": color,
    "--popover-left": `${placement.left}px`,
    "--popover-top": `${placement.top}px`,
    "--popover-available-height": `${placement.availableHeight}px`,
    "--popover-arrow-x": `${placement.arrowX}px`,
    "--popover-arrow-y": `${placement.arrowY}px`,
    "--popover-transform-origin": placement.transformOrigin,
  } as CSSProperties;

  return (
    <div className="posts-calendar-popover-layer edit-layer" role="presentation" onMouseDown={onClose}>
      <article
        ref={inspectorRef}
        className="posts-calendar-edit-inspector"
        data-side={placement.side}
        role="dialog"
        aria-label="Edit post"
        onMouseDown={(event) => event.stopPropagation()}
        style={inspectorStyle}
      >
        <header className="posts-calendar-edit-header">
          <div className="posts-calendar-edit-title-row">
            <h2>Edit post</h2>
            <div className="posts-calendar-popover-profile">
              <span />
              {profile?.name || "Unassigned profile"}
            </div>
          </div>
          <button type="button" aria-label="Close editor" onClick={onClose}>
            <X size={16} />
          </button>
        </header>

        <div className="posts-calendar-edit-body">
          <section className="posts-calendar-edit-section">
            <label>Content</label>
            <textarea
              rows={5}
              value={form.mainContent}
              onChange={(event) => form.setMainContent(event.target.value)}
              placeholder="What's on your mind?"
            />
          </section>

          <section className="posts-calendar-edit-section">
            <div className="posts-calendar-edit-section-head">
              <label>Media</label>
              <span>{form.totalMediaCount} attached</span>
            </div>
            <CalendarEditMediaStrip
              existingItems={form.existingMediaItems}
              mediaItems={form.mediaItems}
              onRemoveExisting={form.removeExistingMediaItem}
              onRemoveMedia={form.removeMediaItem}
              onRetryMedia={(index) => {
                const item = form.mediaItems[index];
                if (!item) return;
                form.removeMediaItem(index);
                void handleFileUpload(item.file);
              }}
              onAdd={(files) => files.forEach((file) => void handleFileUpload(file))}
            />
          </section>

          <section className="posts-calendar-edit-section">
            <div className="posts-calendar-edit-section-head">
              <label>Platforms</label>
              <span>{form.selectedAccountIds.size} selected</span>
            </div>
            <div className="posts-calendar-edit-account-grid">
              {form.activeAccounts.map((account) => (
                <label key={account.id} className="posts-calendar-edit-account">
                  <input
                    type="checkbox"
                    checked={form.selectedAccountIds.has(account.id)}
                    onChange={() => form.toggleAccount(account.id)}
                  />
                  <AccountDestinationIcon platform={account.platform} size={15} />
                  <span>{account.account_name || formatPlatformName(account.platform)}</span>
                </label>
              ))}
            </div>
          </section>

          <section className="posts-calendar-edit-section">
            <label>Schedule</label>
            {post.status === "scheduled" || post.status === "quota_hold" ? (
              <input
                type="datetime-local"
                value={form.scheduledAt}
                onChange={(event) => form.setScheduledAt(event.target.value)}
              />
            ) : (
              <div className="posts-calendar-edit-muted">Draft posts save without a scheduled publish time.</div>
            )}
          </section>

          <section className="posts-calendar-edit-section">
            <div className="posts-calendar-edit-section-head">
              <label>Per-platform customization</label>
              <span>{form.uniqueSelectedAccounts.length} destinations</span>
            </div>
            {form.uniqueSelectedAccounts.length === 0 ? (
              <div className="posts-calendar-edit-muted">Select at least one account.</div>
            ) : (
              <div className="posts-calendar-edit-platforms">
                {form.uniqueSelectedAccounts.map((account, index) => {
                  const override = form.overrides[account.id] || { caption: "" };
                  const text = override.caption || form.mainContent;
                  const charCount = form.getCharCount(text, account.platform);
                  const accountIssues = [
                    ...(validationResult?.errors || []),
                    ...(validationResult?.warnings || []),
                  ].filter((issue) => issue.account_id === account.id);
                  return (
                    <PlatformEditorBlock
                      key={account.id}
                      account={account}
                      index={index}
                      override={override}
                      collapsed={form.collapsedBlocks.has(account.id)}
                      charCount={charCount}
                      captionLimit={getPlatformCaptionLimit(account.platform, charCount.limit, platformCapabilities)}
                      issues={accountIssues}
                      mediaKind={mediaKind}
                      mediaFile={primaryVideoFile}
                      videoMetadata={primaryVideoMeta}
                      getToken={getToken}
                      profileId={account.profile_id || profile?.id || ""}
                      onTiktokBlockerChange={(reason) => setTiktokBlocker(account.id, reason)}
                      onTiktokMaxDurationChange={(sec) => setTiktokMaxDuration(account.id, sec)}
                      onCaptionChange={(caption) => form.updateOverrideCaption(account.id, caption)}
                      onFirstCommentChange={(firstComment) => form.updateOverrideFirstComment(account.id, firstComment)}
                      firstCommentSupported={supportsFirstComment(account.platform, platformCapabilities)}
                      firstCommentMaxLength={getFirstCommentMaxLength(account.platform, platformCapabilities)}
                      threadSupported={supportsThreads(account.platform, platformCapabilities)}
                      onThreadFieldsChange={(fields) => form.updateOverrideThreadFields(account.id, fields)}
                      onAddThreadReply={() => form.addOverrideThreadReply(account.id)}
                      onUpdateThreadReply={(replyIndex, value) => form.updateOverrideThreadReply(account.id, replyIndex, value)}
                      onRemoveThreadReply={(replyIndex) => form.removeOverrideThreadReply(account.id, replyIndex)}
                      onPlatformFieldChange={(platform, fields) => form.updateOverridePlatformField(account.id, platform, fields)}
                      onToggleCollapse={() => form.toggleBlockCollapse(account.id)}
                    />
                  );
                })}
              </div>
            )}
          </section>

          <CalendarEditValidation issues={[...(validationResult?.errors || []), ...(validationResult?.warnings || [])]} />
        </div>

        <footer className="posts-calendar-edit-footer">
          <div className="posts-calendar-edit-status">{error || disabledReason || "Ready to save changes."}</div>
          <div className="posts-calendar-edit-actions">
            <button type="button" className="secondary" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="button" className="primary" onClick={handleSave} disabled={!form.canSubmit || saving || !!runtimeBlocker || oversizeVideos.length > 0}>
              {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
              {saving ? "Saving" : "Save changes"}
            </button>
          </div>
        </footer>
      </article>
    </div>
  );
}

function CalendarEditMediaStrip({
  existingItems,
  mediaItems,
  onRemoveExisting,
  onRemoveMedia,
  onRetryMedia,
  onAdd,
}: {
  existingItems: ExistingMediaItem[];
  mediaItems: MediaItem[];
  onRemoveExisting: (index: number) => void;
  onRemoveMedia: (index: number) => void;
  onRetryMedia: (index: number) => void;
  onAdd: (files: File[]) => void;
}) {
  return (
    <div className="posts-calendar-edit-media">
      {existingItems.map((item, index) => (
        <div key={item.id ? `id:${item.id}` : `url:${item.url}`} className="posts-calendar-edit-media-item">
          {item.url ? <img src={item.url} alt={item.label} /> : <span>Media</span>}
          <button type="button" onClick={() => onRemoveExisting(index)} aria-label="Remove existing media">
            <X size={12} />
          </button>
          <small>{item.label}</small>
        </div>
      ))}
      {mediaItems.map((item, index) => (
        <div key={item.fingerprint} className="posts-calendar-edit-media-item">
          <span>{item.progress < 100 && !item.error ? `${item.progress}%` : item.file.type.startsWith("video/") ? "Video" : "Image"}</span>
          <button type="button" onClick={() => onRemoveMedia(index)} aria-label="Remove media">
            <X size={12} />
          </button>
          <small>{item.error ? item.error : item.file.name}</small>
          {item.error ? <button type="button" className="retry" onClick={() => onRetryMedia(index)}>Retry</button> : null}
        </div>
      ))}
      <label className="posts-calendar-edit-media-add">
        <Plus size={16} />
        <span>Add media</span>
        <input
          type="file"
          multiple
          accept="image/jpeg,image/png,image/webp,image/gif,image/heic,video/mp4,video/quicktime,video/webm,video/x-m4v"
          onChange={(event) => {
            if (event.target.files) {
              onAdd(Array.from(event.target.files));
              event.target.value = "";
            }
          }}
        />
      </label>
    </div>
  );
}

function CalendarEditValidation({ issues }: { issues: SocialPostValidationIssue[] }) {
  if (issues.length === 0) return null;
  return (
    <section className="posts-calendar-edit-validation">
      {issues.slice(0, 4).map((issue, index) => (
        <div key={`${issue.field}-${issue.code}-${index}`} className={issue.severity}>
          <strong>{issue.severity}</strong>
          <span>{issue.message}</span>
        </div>
      ))}
    </section>
  );
}

function getElementRect(element: HTMLElement): CalendarPopoverRect {
  const rect = element.getBoundingClientRect();
  return {
    left: rect.left,
    top: rect.top,
    right: rect.right,
    bottom: rect.bottom,
    width: rect.width,
    height: rect.height,
  };
}

function getCalendarEditorBoundsRect(element: HTMLElement): CalendarPopoverRect {
  const boundsElement = element.closest(".posts-calendar-month-view, .posts-calendar-week-grid, .posts-calendar-day-grid");
  return getElementRect(boundsElement instanceof HTMLElement ? boundsElement : element);
}

function getViewportSize(): CalendarPopoverSize {
  if (typeof window === "undefined") return { width: 1280, height: 720 };
  return { width: window.innerWidth, height: window.innerHeight };
}

function getPrimaryProfile(post: SocialPost, profilesById: Map<string, Profile>): Profile | null {
  const primaryId = post.profile_ids?.[0];
  return primaryId ? profilesById.get(primaryId) || null : null;
}

function getPostColor(post: SocialPost, profilesById: Map<string, Profile>, profileColors: Map<string, string>): string {
  const primaryId = post.profile_ids?.[0];
  if (primaryId && profileColors.has(primaryId)) return profileColors.get(primaryId)!;
  const profile = primaryId ? profilesById.get(primaryId) : null;
  return profile ? getProfileCalendarColor(profile) : "#8b8b93";
}

function getOverflowStatusColor(posts: SocialPost[]): string {
  const post = posts[0];
  return post ? getCalendarStatusColor(getPostStatusGroup(post)) : getCalendarStatusColor("unknown");
}

function getPostPlatforms(post: SocialPost): string[] {
  const platforms = new Set<string>();
  for (const platform of post.target_platforms || []) {
    if (platform) platforms.add(platform);
  }
  for (const result of post.results || []) {
    if (result.platform) platforms.add(result.platform);
  }
  return Array.from(platforms);
}

function isEditableCalendarPost(post: SocialPost): boolean {
  return post.status === "scheduled" || post.status === "quota_hold" || post.status === "draft";
}

function getPostDate(post: SocialPost): Date | null {
  return getCalendarPostDate(post);
}

function getPostTimeValue(post: SocialPost): number {
  return getPostDate(post)?.getTime() || 0;
}

function formatPostTime(post: SocialPost): string {
  const date = getPostDate(post);
  if (!date) return "";
  return date.toLocaleTimeString("en-US", { hour: "numeric", minute: "2-digit" });
}

function formatPostDateTime(post: SocialPost): string {
  const date = getPostDate(post);
  if (!date) return "No date";
  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatOverflowDayLabel(date: Date): string {
  return date.toLocaleDateString("en-US", {
    weekday: "long",
    month: "long",
    day: "numeric",
  });
}

function formatCalendarDetailDate(iso: string): string {
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

function getCalendarTitle(mode: CalendarViewMode, visibleMonth: Date, visibleDate: Date, weekDays: Array<{ date: Date }>): string {
  if (mode === "month") {
    return visibleMonth.toLocaleDateString("en-US", { month: "long", year: "numeric" });
  }
  if (mode === "week") {
    return formatWeekTitle(weekDays[0]?.date || visibleDate, weekDays[6]?.date || visibleDate);
  }
  return visibleDate.toLocaleDateString("en-US", { month: "long", day: "numeric", year: "numeric" });
}

function formatWeekTitle(start: Date, end: Date): string {
  if (start.getFullYear() === end.getFullYear() && start.getMonth() === end.getMonth()) {
    return start.toLocaleDateString("en-US", { month: "long", year: "numeric" });
  }
  if (start.getFullYear() === end.getFullYear()) {
    return `${start.toLocaleDateString("en-US", { month: "short" })} - ${end.toLocaleDateString("en-US", {
      month: "short",
      year: "numeric",
    })}`;
  }
  return `${start.toLocaleDateString("en-US", {
    month: "short",
    year: "numeric",
  })} - ${end.toLocaleDateString("en-US", { month: "short", year: "numeric" })}`;
}

function getWeekdayName(date: Date): string {
  return date.toLocaleDateString("en-US", { weekday: "long" });
}

function formatWeekdayShort(date: Date): string {
  return date.toLocaleDateString("en-US", { weekday: "short" });
}

function formatHourLabel(hour: number): string {
  if (hour === 0) return "12 AM";
  if (hour === 12) return "Noon";
  return hour > 12 ? `${hour - 12} PM` : `${hour} AM`;
}

function formatPlatformName(platform: string): string {
  return platform
    .split(/[_-]/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function formatPlatformLabel(platform: string): string {
  return formatPlatformName(platform);
}

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
}

function clampCalendarSnapOffset(offsetPx: number, unitPx: number): number {
  if (!Number.isFinite(offsetPx) || !Number.isFinite(unitPx) || unitPx <= 0) return 0;
  return Math.max(-unitPx, Math.min(unitPx, offsetPx));
}

function isWeekendDate(date: Date): boolean {
  const day = date.getDay();
  return day === 0 || day === 6;
}

function toggleSetValue(current: Set<string>, value: string): Set<string> {
  const next = new Set(current);
  if (next.has(value)) {
    next.delete(value);
  } else {
    next.add(value);
  }
  return next;
}

const CALENDAR_CSS = `
.posts-calendar-fullheight{--calendar-weekend-surface:color-mix(in srgb,var(--surface2) 58%,var(--surface));min-height:calc(100dvh - 86px);display:grid;grid-template-columns:248px minmax(0,1fr);background:var(--surface);border:1px solid var(--dborder);border-radius:18px;overflow:hidden;box-shadow:0 18px 46px color-mix(in srgb,var(--shadow-color) 90%,transparent)}
.dark .posts-calendar-fullheight{--calendar-weekend-surface:color-mix(in srgb,var(--surface3) 72%,var(--surface))}
.posts-calendar-sidebar{background:color-mix(in srgb,var(--surface2) 74%,var(--surface));border-right:1px solid var(--dborder);padding:18px 14px 16px;display:flex;flex-direction:column;gap:18px;min-width:0}
.posts-calendar-sidebar-top{padding:2px 2px 6px}
.posts-calendar-sidebar-kicker{font-size:12px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--dmuted2);margin-bottom:4px}
.posts-calendar-sidebar-title{font-size:20px;font-weight:720;color:var(--dtext);letter-spacing:0}
.posts-calendar-filter-section{display:flex;flex-direction:column;gap:8px}
.posts-calendar-filter-section h2{font-size:12px;font-weight:700;letter-spacing:.04em;color:var(--dmuted2);margin:0;padding:0 2px;text-transform:uppercase}
.posts-calendar-filter-body{display:flex;flex-direction:column;gap:4px}
.posts-calendar-check,.posts-calendar-platform-check{display:flex;align-items:center;gap:9px;min-height:30px;border:0;background:transparent;border-radius:8px;padding:5px 7px;color:var(--dtext);font-size:14px;line-height:1.25;cursor:pointer}
.posts-calendar-check:hover,.posts-calendar-platform-check:hover{background:color-mix(in srgb,var(--surface3) 66%,transparent)}
.posts-calendar-check input,.posts-calendar-platform-check input{position:absolute;opacity:0;pointer-events:none}
.posts-calendar-checkmark{width:15px;height:15px;border-radius:5px;border:1px solid color-mix(in srgb,var(--profile-color) 70%,var(--dborder2));background:color-mix(in srgb,var(--profile-color) 18%,transparent);position:relative;flex:0 0 auto}
.posts-calendar-platform-check .posts-calendar-checkmark,.posts-calendar-checkmark.neutral{--profile-color:var(--daccent)}
.posts-calendar-check input:checked+.posts-calendar-checkmark,.posts-calendar-platform-check input:checked+.posts-calendar-checkmark{background:var(--profile-color);border-color:var(--profile-color)}
.posts-calendar-check input:checked+.posts-calendar-checkmark::after,.posts-calendar-platform-check input:checked+.posts-calendar-checkmark::after{content:"";position:absolute;left:3px;top:3px;width:7px;height:4px;border-left:2px solid #050505;border-bottom:2px solid #050505;transform:rotate(-45deg)}
.posts-calendar-check-label{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.posts-calendar-muted{font-size:13px;line-height:1.45;color:var(--dmuted2);padding:4px 7px}
.posts-calendar-status-list{display:flex;flex-direction:column;gap:3px}
.posts-calendar-status-option{height:30px;border:0;border-radius:8px;background:transparent;color:var(--dmuted);font:inherit;font-size:14px;text-align:left;padding:0 8px;cursor:pointer}
.posts-calendar-status-option:hover{background:color-mix(in srgb,var(--surface3) 66%,transparent);color:var(--dtext)}
.posts-calendar-status-option.active{background:color-mix(in srgb,var(--daccent) 13%,transparent);color:var(--dtext);font-weight:650}
.posts-calendar-main{min-width:0;min-height:0;display:flex;flex-direction:column;background:var(--surface)}
.posts-calendar-topbar{min-height:76px;padding:15px 18px;display:flex;align-items:center;justify-content:space-between;gap:18px}
.posts-calendar-title-block{min-width:0}
.posts-calendar-title-block h1{font-size:32px;line-height:1.05;font-weight:760;color:var(--dtext);margin:0;letter-spacing:0}
.posts-calendar-title-block span{display:block;margin-top:5px;font-size:13px;color:var(--dmuted2)}
.posts-calendar-toolbar{display:flex;align-items:center;justify-content:flex-end;gap:10px;flex-wrap:wrap}
.posts-calendar-segment{display:flex;align-items:center;border:1px solid var(--dborder);background:var(--surface2);border-radius:999px;padding:3px}
.posts-calendar-segment button{height:30px;min-width:62px;border:0;border-radius:999px;background:transparent;color:var(--dmuted);font:inherit;font-size:14px;font-weight:650}
.posts-calendar-segment button.active{background:var(--surface-raised);color:var(--dtext);box-shadow:0 1px 0 color-mix(in srgb,var(--shadow-color) 70%,transparent)}
.posts-calendar-segment button:disabled{cursor:not-allowed}
.posts-calendar-month-nav{display:flex;align-items:center;gap:4px}
.posts-calendar-month-nav button,.posts-calendar-list-link,.posts-calendar-create,.posts-calendar-filter-toggle{height:34px;display:inline-flex;align-items:center;justify-content:center;gap:7px;border:1px solid var(--dborder);border-radius:999px;background:var(--surface2);color:var(--dtext);font:inherit;font-size:14px;font-weight:650;text-decoration:none;padding:0 12px;cursor:pointer;transition:background .12s,border-color .12s,transform .12s}
.posts-calendar-filter-toggle{display:none}
.posts-calendar-month-nav button:first-child,.posts-calendar-month-nav button:last-child{width:34px;padding:0}
.posts-calendar-month-nav button:hover,.posts-calendar-list-link:hover,.posts-calendar-create:hover,.posts-calendar-filter-toggle:hover{background:var(--surface3);border-color:var(--dborder2)}
.posts-calendar-month-nav button:active,.posts-calendar-list-link:active,.posts-calendar-create:active,.posts-calendar-filter-toggle:active{transform:translateY(1px)}
.posts-calendar-create{background:var(--daccent);border-color:var(--daccent);color:var(--primary-foreground)}
.posts-calendar-error{margin:12px 18px 0;border:1px solid color-mix(in srgb,var(--danger) 24%,transparent);background:var(--danger-soft);color:var(--danger);border-radius:10px;padding:10px 12px;font-size:13px;line-height:1.45}
.posts-calendar-view-stage{flex:1;min-height:0;display:flex;overflow:hidden;background:var(--surface)}
.posts-calendar-month-shell{--calendar-snap-offset:0px;--calendar-snap-duration:0ms;flex:1;min-width:0;min-height:640px;display:flex;flex-direction:column;overflow:hidden;background:var(--surface);overscroll-behavior:contain;touch-action:none}
.posts-calendar-month-view{flex:1;min-width:0;min-height:0;display:flex;overflow:hidden;background:var(--surface)}
.posts-calendar-month-weekdays{flex:0 0 38px;height:38px;display:grid;grid-template-columns:repeat(7,minmax(0,1fr));background:var(--surface);border-bottom:1px solid var(--dborder)}
.posts-calendar-month-days{flex:1;min-width:0;min-height:0;display:grid;grid-template-columns:repeat(7,minmax(0,1fr));grid-template-rows:repeat(6,minmax(104px,1fr));background:var(--dborder);gap:1px}
.posts-calendar-month-track{width:100%;height:100%;flex:0 0 auto;grid-template-rows:repeat(6,minmax(104px,1fr));will-change:transform;contain:layout paint;backface-visibility:hidden;transform:translate3d(0,var(--calendar-snap-offset),0);transition:transform var(--calendar-snap-duration) cubic-bezier(.16,1,.3,1)}
.posts-calendar-weekday{background:transparent;display:flex;align-items:center;justify-content:flex-end;padding:0 12px;color:var(--dmuted);font-size:13px;font-weight:650}
.posts-calendar-weekday.weekend{background:var(--calendar-weekend-surface)}
.posts-calendar-day{background:var(--surface);min-width:0;min-height:104px;padding:8px 6px 7px;display:flex;flex-direction:column;gap:5px;overflow:hidden}
.posts-calendar-day.outside{background:color-mix(in srgb,var(--surface2) 42%,var(--surface));color:var(--dmuted2)}
.posts-calendar-day.weekend{--calendar-event-surface:var(--calendar-weekend-surface);background:var(--calendar-weekend-surface)}
.posts-calendar-day.outside.weekend{background:color-mix(in srgb,var(--calendar-weekend-surface) 72%,var(--surface2))}
.posts-calendar-day-number{display:flex;justify-content:flex-end;height:22px;font-size:16px;color:var(--dmuted);font-weight:600}
.posts-calendar-day.today .posts-calendar-day-number span{display:inline-flex;align-items:center;justify-content:center;width:24px;height:24px;border-radius:999px;background:var(--danger);color:white;margin-top:-2px}
.posts-calendar-events{flex:1;display:flex;flex-direction:column;gap:4px;min-width:0;overflow:hidden}
.posts-calendar-event{--event-profile-color:#8b8b93;--event-status-color:#475569;position:relative;display:grid;grid-template-columns:3px auto minmax(0,1fr) auto;align-items:center;gap:5px;width:100%;min-height:22px;border:1px solid color-mix(in srgb,var(--event-profile-color) 20%,transparent);border-radius:6px;background:color-mix(in srgb,var(--event-profile-color) 15%,var(--calendar-event-surface,var(--surface)));color:var(--dtext);font:inherit;text-align:left;padding:2px 6px 2px 4px;cursor:pointer;overflow:hidden}
.posts-calendar-event:hover{border-color:color-mix(in srgb,var(--event-profile-color) 42%,var(--dborder));background:color-mix(in srgb,var(--event-profile-color) 22%,var(--calendar-event-surface,var(--surface)))}
.posts-calendar-event-rail{width:3px;align-self:stretch;border-radius:99px;background:var(--event-status-color)}
.posts-calendar-event-status{font-size:9px;font-weight:800;letter-spacing:.04em;color:var(--event-status-color);line-height:1}
.posts-calendar-event-caption{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:650}
.posts-calendar-event-time{font-size:11px;color:var(--dmuted2);white-space:nowrap}
.posts-calendar-more{height:22px;border:0;border-radius:6px;background:transparent;color:var(--dmuted);font:inherit;font-size:12px;font-weight:650;text-align:left;padding:0 7px;cursor:pointer}
.posts-calendar-more:hover{background:var(--surface2);color:var(--dtext)}
.posts-calendar-more-pill{--event-status-color:#475569;align-self:flex-start;min-width:28px;height:20px;display:inline-flex;align-items:center;justify-content:center;border-radius:999px;background:var(--event-status-color);color:white;text-align:center;line-height:1;padding:0 8px;box-shadow:0 1px 0 color-mix(in srgb,var(--shadow-color) 80%,transparent)}
.posts-calendar-more-pill:hover{background:color-mix(in srgb,var(--event-status-color) 88%,var(--dtext));color:white}
.posts-calendar-week-shell,.posts-calendar-day-grid{--calendar-time-gutter:76px;--calendar-week-day-min:132px;--calendar-scrollbar-gutter:0px;--calendar-snap-offset:0px;--calendar-snap-duration:0ms;--calendar-week-template:repeat(9,minmax(var(--calendar-week-day-min),1fr));--calendar-week-min-width:calc(var(--calendar-week-day-min) * 7 + var(--calendar-scrollbar-gutter));flex:1;min-width:0;min-height:0;display:flex;flex-direction:column;background:var(--surface)}
.posts-calendar-week-shell{overscroll-behavior:contain;overscroll-behavior-x:contain;overflow:hidden;touch-action:pan-y}
.posts-calendar-week-grid{flex:1;min-width:0;min-height:0;display:flex;flex-direction:column;background:var(--surface);overflow:hidden}
.posts-calendar-week-header{flex:0 0 44px;height:44px;display:grid;grid-template-columns:var(--calendar-time-gutter) minmax(0,1fr);background:var(--surface);min-width:0}
.posts-calendar-week-header-gutter{background:var(--surface)}
.posts-calendar-week-header-row{display:grid;grid-template-columns:minmax(0,1fr) var(--calendar-scrollbar-gutter);min-width:0;background:var(--surface);overflow:hidden}
.posts-calendar-week-header-inner{display:grid;grid-template-columns:var(--calendar-week-template);width:calc(100% * 9 / 7);min-width:calc(var(--calendar-week-day-min) * 9)}
.posts-calendar-week-track{will-change:transform;contain:layout paint;backface-visibility:hidden;transform:translate3d(calc(-11.111111% + var(--calendar-snap-offset)),0,0);transition:transform var(--calendar-snap-duration) cubic-bezier(.16,1,.3,1)}
.posts-calendar-week-body{position:relative;flex:1;min-width:0;min-height:0;display:grid;grid-template-columns:var(--calendar-time-gutter) minmax(0,1fr);grid-template-rows:minmax(0,1fr);background:var(--dborder);gap:1px;overflow:hidden}
.posts-calendar-week-body::before{content:"";position:absolute;left:0;right:0;top:0;border-top:1px solid var(--dborder);z-index:3;pointer-events:none}
.posts-calendar-week-time-gutter{grid-column:1;grid-row:1;min-width:0;min-height:0;display:block;background:var(--surface);overflow:hidden}
.posts-calendar-week-time-label-scroll{height:100%;min-height:0;overflow:hidden;background:var(--surface)}
.posts-calendar-week-time-label-scroll .posts-calendar-time-labels{border-right:0}
.posts-calendar-week-content{grid-column:2;grid-row:1;min-width:0;min-height:0;display:block;background:var(--surface);overflow:hidden}
.posts-calendar-week-scrollbar-spacer{background:var(--surface)}
.posts-calendar-week-heading{height:44px;background:transparent;display:flex;align-items:center;justify-content:center;gap:7px;color:var(--dmuted);font-size:13px;font-weight:650}
.posts-calendar-week-heading strong{font-size:17px;color:var(--dtext);font-weight:720}
.posts-calendar-week-heading.today strong{display:inline-flex;align-items:center;justify-content:center;width:26px;height:26px;border-radius:999px;background:var(--danger);color:white}
.posts-calendar-time-scroll{flex:1;min-height:0;display:grid;grid-template-columns:var(--calendar-time-gutter) minmax(0,1fr);overflow-y:auto;overflow-x:hidden;overscroll-behavior:contain;background:var(--surface)}
.posts-calendar-week-content>.posts-calendar-time-scroll{height:100%;grid-template-columns:minmax(0,1fr);gap:1px;min-width:0;background:var(--dborder)}
.posts-calendar-time-labels{height:var(--calendar-timeline-height,calc(24 * var(--hour-height,64px)));background:var(--surface);border-right:1px solid var(--dborder)}
.posts-calendar-time-label{height:var(--hour-height,64px);display:flex;align-items:flex-start;justify-content:flex-end;padding:5px 8px 0 4px;color:var(--dmuted);font-size:12px;font-weight:650;border-top:1px solid var(--dborder);white-space:nowrap}
.posts-calendar-week-columns{width:calc(100% * 9 / 7);min-width:calc(var(--calendar-week-day-min) * 9);display:grid;grid-template-columns:var(--calendar-week-template);background:var(--dborder);gap:1px}
.posts-calendar-week-grid .posts-calendar-time-label:first-child,.posts-calendar-day-grid .posts-calendar-time-label:first-child{border-top:0}
.posts-calendar-day-column-wrap{min-width:0;background:var(--dborder);padding-left:1px}
.posts-calendar-day-grid.weekend .posts-calendar-day-column-wrap{background:var(--calendar-weekend-surface)}
.posts-calendar-time-column{--calendar-event-surface:var(--surface);position:relative;height:var(--calendar-timeline-height,calc(24 * var(--hour-height,64px)));background-color:var(--surface);background-image:repeating-linear-gradient(to bottom,transparent 0 calc(var(--hour-height,64px) - 1px),var(--dborder) calc(var(--hour-height,64px) - 1px) var(--hour-height,64px));overflow:hidden}
.posts-calendar-time-column.weekend{--calendar-event-surface:var(--calendar-weekend-surface);background-color:var(--calendar-weekend-surface)}
.posts-calendar-timed-event{--event-profile-color:#8b8b93;--event-status-color:#475569;position:absolute;min-height:var(--calendar-timed-event-min-height,38px);border:1px solid color-mix(in srgb,var(--event-profile-color) 24%,transparent);border-radius:7px;background:color-mix(in srgb,var(--event-profile-color) 17%,var(--calendar-event-surface,var(--surface)));color:var(--dtext);font:inherit;text-align:left;padding:5px 7px 5px 5px;display:grid;grid-template-columns:3px minmax(0,1fr);gap:7px;cursor:pointer;box-shadow:0 1px 0 color-mix(in srgb,var(--shadow-color) 52%,transparent);overflow:hidden}
.posts-calendar-timed-event.has-overflow-pill{padding-right:48px}
.posts-calendar-timed-event:hover{border-color:color-mix(in srgb,var(--event-profile-color) 48%,var(--dborder));background:color-mix(in srgb,var(--event-profile-color) 24%,var(--calendar-event-surface,var(--surface)))}
.posts-calendar-timed-content{min-width:0;display:flex;flex-direction:column;gap:2px}
.posts-calendar-timed-title{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:760;line-height:1.15}
.posts-calendar-timed-meta{font-size:11px;color:var(--event-status-color);font-weight:700;line-height:1.15;white-space:nowrap}
.posts-calendar-timed-overflow{position:absolute;border:0;font:inherit;font-size:11px;font-weight:820;cursor:pointer}
.posts-calendar-timed-overflow.posts-calendar-more-pill{align-self:auto;min-width:34px;height:22px}
.posts-calendar-popover-layer{position:fixed;inset:0;background:transparent;z-index:90}
.posts-calendar-popover{position:fixed;left:var(--popover-left);top:var(--popover-top);box-sizing:border-box;width:min(560px,calc(100vw - 24px));max-height:min(calc(100dvh - 24px),var(--popover-available-height,calc(100dvh - 24px)));background:var(--surface-raised);border:1px solid var(--dborder);border-radius:16px;box-shadow:0 24px 70px color-mix(in srgb,var(--shadow-color) 160%,transparent);transform-origin:var(--popover-transform-origin);animation:posts-calendar-popover-open .18s cubic-bezier(.16,1,.3,1);overflow:visible}
.posts-calendar-popover-content{box-sizing:border-box;max-height:min(calc(100dvh - 26px),calc(var(--popover-available-height,calc(100dvh - 24px)) - 2px));overflow:auto;padding:16px;border-radius:inherit}
.posts-calendar-popover::before{content:"";position:absolute;width:16px;height:16px;background:var(--surface-raised);border:1px solid var(--dborder);transform:rotate(45deg);pointer-events:none}
.posts-calendar-popover[data-side="right"]::before{left:-9px;top:calc(var(--popover-arrow-y) - 8px);border-top:0;border-right:0}
.posts-calendar-popover[data-side="left"]::before{right:-9px;top:calc(var(--popover-arrow-y) - 8px);border-bottom:0;border-left:0}
.posts-calendar-popover[data-side="bottom"]::before{left:calc(var(--popover-arrow-x) - 8px);top:-9px;border-right:0;border-bottom:0}
.posts-calendar-popover[data-side="top"]::before{left:calc(var(--popover-arrow-x) - 8px);bottom:-9px;border-top:0;border-left:0}
@keyframes posts-calendar-popover-open{from{opacity:0;transform:scale(.94) translateY(3px)}to{opacity:1;transform:scale(1) translateY(0)}}
.posts-calendar-popover-head{display:flex;align-items:flex-start;justify-content:space-between;gap:14px;margin-bottom:16px}
.posts-calendar-popover-head h2{margin:5px 0 0;color:var(--dtext);font-size:18px;line-height:1.35;font-weight:720;letter-spacing:0}
.posts-calendar-popover-head button{width:30px;height:30px;display:inline-flex;align-items:center;justify-content:center;border:1px solid var(--dborder);border-radius:999px;background:var(--surface2);color:var(--dmuted);cursor:pointer}
.posts-calendar-popover-profile{display:inline-flex;align-items:center;gap:7px;color:var(--dmuted);font-size:13px;font-weight:650}
.posts-calendar-popover-profile span{width:9px;height:9px;border-radius:999px;background:var(--event-profile-color)}
.posts-calendar-more-popover{width:min(380px,calc(100vw - 24px))}
.posts-calendar-more-summary{margin:5px 0 0;color:var(--dmuted2);font-size:13px;line-height:1.35}
.posts-calendar-more-list{display:flex;flex-direction:column;gap:7px;max-height:min(380px,calc(var(--popover-available-height,calc(100dvh - 24px)) - 96px));overflow:auto;padding-right:2px}
.posts-calendar-more-list .posts-calendar-event{min-height:30px;border-radius:8px;padding:4px 7px 4px 5px}
.posts-calendar-popover-meta{display:grid;gap:12px;margin:0}
.posts-calendar-popover-meta div{display:grid;grid-template-columns:82px minmax(0,1fr);gap:12px;align-items:flex-start}
.posts-calendar-popover-meta dt{font-size:12px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;color:var(--dmuted2)}
.posts-calendar-popover-meta dd{margin:0;color:var(--dtext);font-size:14px;line-height:1.45}
.posts-calendar-hold-notice{padding:12px 14px;border:1px solid color-mix(in srgb,var(--warning) 28%,transparent);border-radius:10px;background:color-mix(in srgb,var(--warning) 9%,transparent);color:var(--warning);font-size:12px;line-height:1.5}
.posts-calendar-popover-status{display:inline-flex;align-items:center;height:19px;border-radius:5px;padding:0 5px;margin-right:7px;background:color-mix(in srgb,var(--event-status-color) 18%,transparent);color:var(--event-status-color);font-size:10px;font-weight:800;letter-spacing:.04em}
.posts-calendar-popover-platforms{display:flex;flex-wrap:wrap;gap:6px}
.posts-calendar-popover-platform-chip{display:inline-flex;align-items:center;gap:5px;border:1px solid var(--dborder);background:var(--surface2);border-radius:999px;padding:3px 8px;font-size:12px;font-weight:650}
.posts-calendar-detail-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:7px;margin-top:12px}
.posts-calendar-detail-card{min-width:0;min-height:68px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);padding:8px 9px}
.posts-calendar-detail-card div{margin-bottom:5px;color:var(--dmuted2);font-size:10px;font-weight:780;letter-spacing:.08em;text-transform:uppercase}
.posts-calendar-detail-card span{display:-webkit-box;min-width:0;overflow:hidden;-webkit-line-clamp:2;-webkit-box-orient:vertical;color:var(--dtext);font-size:12px;line-height:1.3}
.posts-calendar-results{margin-top:16px}
.posts-calendar-results-label{margin-bottom:9px;color:var(--dmuted2);font-size:11px;font-weight:780;letter-spacing:.08em;text-transform:uppercase}
.posts-calendar-popover-actions{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:8px;margin-top:17px}
.posts-calendar-open-list{display:inline-flex;align-items:center;justify-content:center;gap:6px;width:100%;height:36px;border-radius:10px;background:var(--surface2);border:1px solid var(--dborder);color:var(--dtext);text-decoration:none;font-size:14px;font-weight:700}
.posts-calendar-open-list:hover{background:var(--surface3)}
.posts-calendar-open-list:disabled{opacity:.55;cursor:not-allowed}
.posts-calendar-popover-layer.edit-layer{z-index:92;background:color-mix(in srgb,var(--surface) 8%,transparent)}
.posts-calendar-edit-inspector{position:fixed;left:var(--popover-left);top:var(--popover-top);width:min(760px,calc(100vw - 24px));max-height:min(calc(100dvh - 24px),var(--popover-available-height,calc(100dvh - 24px)));display:flex;flex-direction:column;background:color-mix(in srgb,var(--surface-raised) 96%,black);border:1px solid var(--dborder);border-radius:18px;box-shadow:0 26px 78px color-mix(in srgb,var(--shadow-color) 170%,transparent);transform-origin:var(--popover-transform-origin);animation:posts-calendar-popover-open .18s cubic-bezier(.16,1,.3,1);overflow:hidden}
.posts-calendar-edit-inspector::before{content:"";position:absolute;width:16px;height:16px;background:color-mix(in srgb,var(--surface-raised) 96%,black);border:1px solid var(--dborder);transform:rotate(45deg);pointer-events:none}
.posts-calendar-edit-inspector[data-side="right"]::before{left:-9px;top:calc(var(--popover-arrow-y) - 8px);border-top:0;border-right:0}
.posts-calendar-edit-inspector[data-side="left"]::before{right:-9px;top:calc(var(--popover-arrow-y) - 8px);border-bottom:0;border-left:0}
.posts-calendar-edit-inspector[data-side="bottom"]::before{left:calc(var(--popover-arrow-x) - 8px);top:-9px;border-right:0;border-bottom:0}
.posts-calendar-edit-inspector[data-side="top"]::before{left:calc(var(--popover-arrow-x) - 8px);bottom:-9px;border-top:0;border-left:0}
.posts-calendar-edit-header,.posts-calendar-edit-footer{flex:0 0 auto}
.posts-calendar-edit-header{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:15px 18px;border-bottom:1px solid var(--dborder)}
.posts-calendar-edit-title-row{min-width:0;display:flex;align-items:center;gap:10px;flex-wrap:wrap}
.posts-calendar-edit-header h2{margin:0;color:var(--dtext);font-size:22px;line-height:1.15;font-weight:760;letter-spacing:0}
.posts-calendar-edit-header .posts-calendar-popover-profile{min-width:0;max-width:320px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.posts-calendar-edit-header button{width:32px;height:32px;display:inline-flex;align-items:center;justify-content:center;border:1px solid var(--dborder);border-radius:999px;background:var(--surface2);color:var(--dmuted);cursor:pointer}
.posts-calendar-edit-body{flex:1 1 auto;min-height:0;overflow:auto;padding:16px 18px 18px;display:flex;flex-direction:column;gap:16px}
.posts-calendar-edit-section{display:flex;flex-direction:column;gap:8px}
.posts-calendar-edit-section label,.posts-calendar-edit-section-head label{font-size:11px;font-weight:780;text-transform:uppercase;letter-spacing:.08em;color:var(--dmuted2)}
.posts-calendar-edit-section textarea,.posts-calendar-edit-section input[type="datetime-local"]{width:100%;border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);color:var(--dtext);font:inherit;font-size:14px;line-height:1.5;outline:0;padding:10px 11px}
.posts-calendar-edit-section textarea:focus,.posts-calendar-edit-section input[type="datetime-local"]:focus{border-color:color-mix(in srgb,var(--daccent) 60%,var(--dborder));box-shadow:0 0 0 3px color-mix(in srgb,var(--daccent) 14%,transparent)}
.posts-calendar-edit-section-head{display:flex;align-items:center;justify-content:space-between;gap:10px}
.posts-calendar-edit-section-head span,.posts-calendar-edit-muted{font-size:12px;color:var(--dmuted2)}
.posts-calendar-edit-account-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:7px}
.posts-calendar-edit-account{min-width:0;min-height:34px;display:flex;align-items:center;gap:8px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);padding:7px 9px;color:var(--dtext);font-size:13px;font-weight:650;cursor:pointer}
.posts-calendar-edit-account:hover{background:var(--surface2)}
.posts-calendar-edit-account input{accent-color:var(--daccent)}
.posts-calendar-edit-account span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.posts-calendar-edit-platforms{display:flex;flex-direction:column;gap:10px}
.posts-calendar-edit-media{display:flex;flex-wrap:wrap;gap:8px}
.posts-calendar-edit-media-item,.posts-calendar-edit-media-add{position:relative;width:88px;height:88px;display:flex;align-items:center;justify-content:center;border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);overflow:hidden}
.posts-calendar-edit-media-item img{width:100%;height:100%;object-fit:cover}
.posts-calendar-edit-media-item>span{font-size:11px;font-weight:700;color:var(--dmuted)}
.posts-calendar-edit-media-item small{position:absolute;left:0;right:0;bottom:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:rgba(0,0,0,.58);color:#f4f4f5;font-size:9px;padding:2px 4px}
.posts-calendar-edit-media-item button:not(.retry){position:absolute;right:5px;top:5px;width:20px;height:20px;border:0;border-radius:999px;background:rgba(0,0,0,.7);color:white;display:inline-flex;align-items:center;justify-content:center}
.posts-calendar-edit-media-item .retry{position:absolute;left:5px;top:5px;border:0;border-radius:999px;background:var(--danger);color:white;font-size:10px;padding:2px 6px}
.posts-calendar-edit-media-add{flex-direction:column;gap:5px;border-style:dashed;color:var(--dmuted);font-size:11px;font-weight:650;cursor:pointer}
.posts-calendar-edit-media-add input{display:none}
.posts-calendar-edit-validation{display:flex;flex-direction:column;gap:7px}
.posts-calendar-edit-validation div{display:flex;gap:7px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface1);padding:8px 10px;font-size:12px;line-height:1.4;color:var(--dtext)}
.posts-calendar-edit-validation div.error{border-color:color-mix(in srgb,var(--danger) 45%,transparent);background:var(--danger-soft);color:var(--danger)}
.posts-calendar-edit-validation strong{text-transform:uppercase;font-size:10px;letter-spacing:.06em}
.posts-calendar-edit-footer{display:flex;align-items:center;justify-content:space-between;gap:14px;border-top:1px solid var(--dborder);padding:12px 14px;background:var(--surface-raised)}
.posts-calendar-edit-status{min-width:0;color:var(--dmuted);font-size:12px;line-height:1.35}
.posts-calendar-edit-actions{display:flex;align-items:center;justify-content:flex-end;gap:8px;flex:0 0 auto}
.posts-calendar-edit-footer button{height:36px;display:inline-flex;align-items:center;justify-content:center;gap:7px;border:1px solid var(--dborder);border-radius:10px;background:var(--surface2);color:var(--dtext);font:inherit;font-size:13px;font-weight:760;padding:0 13px;white-space:nowrap}
.posts-calendar-edit-footer button.primary{border-color:var(--daccent);background:var(--daccent);color:var(--primary-foreground)}
.posts-calendar-edit-footer button:disabled{opacity:.55;cursor:not-allowed}
@media (max-width: 980px){.posts-calendar-fullheight{grid-template-columns:1fr}.posts-calendar-sidebar{border-right:0;border-bottom:1px solid var(--dborder);display:grid;grid-template-columns:repeat(3,minmax(0,1fr));align-items:start}.posts-calendar-sidebar-top{grid-column:1/-1}.posts-calendar-topbar{align-items:flex-start;flex-direction:column}.posts-calendar-toolbar{justify-content:flex-start}.posts-calendar-month-shell{min-height:720px}.posts-calendar-month-days{grid-template-rows:repeat(6,minmax(114px,1fr))}}
@media (max-width: 680px){.posts-calendar-fullheight{position:relative;border-radius:12px;min-height:calc(100dvh - 80px)}.posts-calendar-main{min-height:0}.posts-calendar-filter-toggle{display:inline-flex}.posts-calendar-sidebar{position:absolute;left:10px;right:10px;top:76px;z-index:8;max-height:min(64dvh,520px);overflow:auto;border:1px solid var(--dborder);border-radius:14px;background:color-mix(in srgb,var(--surface-raised) 96%,var(--surface));box-shadow:0 18px 48px color-mix(in srgb,var(--shadow-color) 120%,transparent);grid-template-columns:1fr;padding:14px}.posts-calendar-sidebar:not([data-mobile-open="true"]){display:none}.posts-calendar-sidebar-top{padding:0}.posts-calendar-topbar{min-height:auto;padding:12px;gap:12px}.posts-calendar-title-block h1{font-size:26px}.posts-calendar-toolbar{gap:8px}.posts-calendar-segment button{min-width:54px}.posts-calendar-month-shell{min-height:520px;overflow:hidden;touch-action:pan-y}.posts-calendar-month-weekdays,.posts-calendar-month-days{min-width:0;width:100%}.posts-calendar-month-days,.posts-calendar-month-track{grid-template-rows:repeat(6,minmax(70px,1fr))}.posts-calendar-weekday{justify-content:center;padding:0;font-size:11px}.posts-calendar-day{min-height:70px;padding:5px 2px;gap:3px}.posts-calendar-day-number{height:18px;justify-content:center;font-size:12px}.posts-calendar-day.today .posts-calendar-day-number span{width:20px;height:20px;margin-top:-1px}.posts-calendar-events{align-items:center;gap:2px}.posts-calendar-event{width:16px;height:7px;min-height:7px;display:block;border-radius:999px;padding:0}.posts-calendar-event-rail,.posts-calendar-event-status,.posts-calendar-event-caption,.posts-calendar-event-time{display:none}.posts-calendar-more{height:15px;padding:0;text-align:center;font-size:10px}.posts-calendar-popover{width:min(360px,calc(100vw - 24px))}.posts-calendar-detail-grid{grid-template-columns:1fr}.posts-calendar-edit-inspector{width:min(420px,calc(100vw - 24px))}.posts-calendar-edit-account-grid{grid-template-columns:1fr}}
@media (prefers-reduced-motion:reduce){.posts-calendar-popover,.posts-calendar-edit-inspector{animation:none}.posts-calendar-month-track,.posts-calendar-week-track{transition-duration:0ms}}
`;
