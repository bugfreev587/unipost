"use client";

import { useAuth } from "@clerk/nextjs";
import { ChevronLeft, ChevronRight, List, Plus, X } from "lucide-react";
import Link from "next/link";
import { usePathname, useParams, useRouter, useSearchParams } from "next/navigation";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
  type MouseEvent,
  type TouchEvent,
  type WheelEvent,
} from "react";
import { PlatformIcon } from "@/components/platform-icons";
import { CreatePostDrawer } from "@/components/posts/create-post/create-post-drawer";
import {
  listProfiles,
  listSocialAccounts,
  listSocialPosts,
  type Profile,
  type SocialAccount,
  type SocialPost,
} from "@/lib/api";
import { useWorkspaceId } from "@/lib/use-workspace-id";
import {
  buildMonthGrid,
  buildWeekDays,
  bucketPostByLocalDay,
  formatLocalDateKey,
  getAccumulatedWheelNavigationIntent,
  getAnchoredPopoverPlacement,
  getCalendarPostDate,
  getCalendarPostMinuteOfDay,
  getPostStatusGroup,
  getProfileCalendarColor,
  getSwipeNavigationIntent,
  getTimedEventTop,
  getTimedTimelineContentHeight,
  parseCalendarViewMode,
  type CalendarDayCell,
  type CalendarPopoverRect,
  type CalendarPopoverSize,
  type CalendarWheelNavigationAccumulator,
  shouldShowPostForStatusFilter,
  shiftCalendarDateBySwipe,
  type CalendarStatusFilter,
  type CalendarStatusGroup,
  type CalendarViewMode,
} from "./calendar-model";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
const HOUR_HEIGHT = 64;
const TIMED_EVENT_MIN_HEIGHT = 38;
const TIMELINE_END_PADDING = 8;
const TIMELINE_CONTENT_HEIGHT = getTimedTimelineContentHeight(
  HOUR_HEIGHT,
  TIMED_EVENT_MIN_HEIGHT,
  TIMELINE_END_PADDING,
);
const POPOVER_FALLBACK_SIZE: CalendarPopoverSize = { width: 420, height: 320 };
const SWIPE_TRANSITION_MS = 260;

type SelectedPostTarget = {
  postId: string;
  anchorRect: CalendarPopoverRect;
};

type SwipeTransitionState = {
  id: number;
  mode: Extract<CalendarViewMode, "month" | "week">;
  direction: -1 | 1;
  fromDate: Date;
  fromMonth: Date;
  toDate: Date;
  toMonth: Date;
};

const STATUS_FILTERS: Array<{ value: CalendarStatusFilter; label: string }> = [
  { value: "all", label: "All Status" },
  { value: "published", label: "Published" },
  { value: "scheduled", label: "Scheduled" },
  { value: "in_progress", label: "In Progress" },
  { value: "failed", label: "Failed" },
  { value: "draft", label: "Drafts" },
  { value: "cancelled", label: "Cancelled" },
  { value: "archived", label: "Archived" },
];

const STATUS_META: Record<CalendarStatusGroup, { label: string; short: string }> = {
  published: { label: "Published", short: "PUB" },
  scheduled: { label: "Scheduled", short: "SCH" },
  in_progress: { label: "In Progress", short: "RUN" },
  failed: { label: "Failed", short: "FAIL" },
  draft: { label: "Draft", short: "DRFT" },
  cancelled: { label: "Cancelled", short: "CNCL" },
  archived: { label: "Archived", short: "ARCH" },
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
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [swipeTransition, setSwipeTransition] = useState<SwipeTransitionState | null>(null);
  const wheelDeltaRef = useRef<CalendarWheelNavigationAccumulator>({ deltaX: 0, deltaY: 0 });
  const wheelLockRef = useRef(0);
  const wheelResetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const swipeTransitionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const touchStartRef = useRef<{ x: number; y: number } | null>(null);
  const weekTimeScrollRef = useRef<HTMLDivElement | null>(null);
  const [weekScrollbarWidth, setWeekScrollbarWidth] = useState(0);

  const calendarMode = useMemo(() => parseCalendarViewMode(searchParams.get("view")), [searchParams]);
  const timezone = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || "Local time", []);

  const clearSwipeTransition = useCallback(() => {
    if (swipeTransitionTimerRef.current) {
      clearTimeout(swipeTransitionTimerRef.current);
      swipeTransitionTimerRef.current = null;
    }
    setSwipeTransition(null);
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
        listSocialPosts(token),
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
    wheelDeltaRef.current = { deltaX: 0, deltaY: 0 };
    wheelLockRef.current = 0;
    if (wheelResetTimerRef.current) {
      clearTimeout(wheelResetTimerRef.current);
      wheelResetTimerRef.current = null;
    }
    clearSwipeTransition();
  }, [calendarMode, clearSwipeTransition]);

  useEffect(() => () => {
    if (wheelResetTimerRef.current) {
      clearTimeout(wheelResetTimerRef.current);
    }
    if (swipeTransitionTimerRef.current) {
      clearTimeout(swipeTransitionTimerRef.current);
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

  const shiftCalendar = useCallback((direction: -1 | 1) => {
    clearSwipeTransition();

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
  }, [calendarMode, clearSwipeTransition]);

  const startCalendarSwipeTransition = useCallback((direction: -1 | 1) => {
    if (calendarMode === "day") return;

    const fromDate = visibleDate;
    const fromMonth = visibleMonth;
    const toDate = shiftCalendarDateBySwipe(calendarMode, fromDate, direction);
    const toMonth = calendarMode === "month"
      ? shiftCalendarDateBySwipe(calendarMode, fromMonth, direction)
      : startOfMonth(toDate);

    if (swipeTransitionTimerRef.current) {
      clearTimeout(swipeTransitionTimerRef.current);
      swipeTransitionTimerRef.current = null;
    }

    setSwipeTransition({
      id: Date.now(),
      mode: calendarMode,
      direction,
      fromDate,
      fromMonth,
      toDate,
      toMonth,
    });
    setVisibleDate(toDate);
    setVisibleMonth(toMonth);

    swipeTransitionTimerRef.current = setTimeout(() => {
      setSwipeTransition(null);
      swipeTransitionTimerRef.current = null;
    }, SWIPE_TRANSITION_MS);
  }, [calendarMode, visibleDate, visibleMonth]);

  const shiftCalendarBySwipe = useCallback((direction: -1 | 1) => {
    startCalendarSwipeTransition(direction);
  }, [startCalendarSwipeTransition]);

  const goToToday = useCallback(() => {
    clearSwipeTransition();
    const today = new Date();
    setVisibleDate(today);
    setVisibleMonth(startOfMonth(today));
  }, [clearSwipeTransition]);

  const handleCalendarWheel = useCallback((event: WheelEvent<HTMLElement>) => {
    if (calendarMode === "day") return;

    event.preventDefault();

    const now = Date.now();
    if (now < wheelLockRef.current) {
      return;
    }

    const result = getAccumulatedWheelNavigationIntent(
      calendarMode,
      wheelDeltaRef.current,
      event.deltaX,
      event.deltaY,
      event.shiftKey,
    );
    wheelDeltaRef.current = result.accumulator;

    if (wheelResetTimerRef.current) {
      clearTimeout(wheelResetTimerRef.current);
    }
    wheelResetTimerRef.current = setTimeout(() => {
      wheelDeltaRef.current = { deltaX: 0, deltaY: 0 };
      wheelResetTimerRef.current = null;
    }, 180);

    if (result.direction === 0) return;

    if (wheelResetTimerRef.current) {
      clearTimeout(wheelResetTimerRef.current);
      wheelResetTimerRef.current = null;
    }
    wheelDeltaRef.current = { deltaX: 0, deltaY: 0 };
    wheelLockRef.current = now + SWIPE_TRANSITION_MS;
    shiftCalendarBySwipe(result.direction);
  }, [calendarMode, shiftCalendarBySwipe]);

  const handleCalendarTouchStart = useCallback((event: TouchEvent<HTMLElement>) => {
    const touch = event.touches[0];
    if (!touch) return;
    touchStartRef.current = { x: touch.clientX, y: touch.clientY };
  }, []);

  const handleCalendarTouchEnd = useCallback((event: TouchEvent<HTMLElement>) => {
    const start = touchStartRef.current;
    const touch = event.changedTouches[0];
    touchStartRef.current = null;
    if (!start || !touch) return;

    const direction = getSwipeNavigationIntent(calendarMode, start.x, start.y, touch.clientX, touch.clientY);
    if (direction === 0) return;

    event.preventDefault();
    shiftCalendarBySwipe(direction);
  }, [calendarMode, shiftCalendarBySwipe]);

  const handleSelectPost = useCallback((postId: string, target: HTMLElement) => {
    setSelectedPostTarget({ postId, anchorRect: getElementRect(target) });
  }, []);

  const closeSelectedPost = useCallback(() => {
    setSelectedPostTarget(null);
  }, []);

  const renderMonthView = (
    cells: CalendarDayCell[] = monthCells,
    {
      interactive = true,
      ariaLabel = `${calendarTitle} posts`,
    }: { interactive?: boolean; ariaLabel?: string } = {},
  ) => (
    <div
      className="posts-calendar-grid"
      aria-label={ariaLabel}
      onWheel={interactive ? handleCalendarWheel : undefined}
      onTouchStart={interactive ? handleCalendarTouchStart : undefined}
      onTouchEnd={interactive ? handleCalendarTouchEnd : undefined}
    >
      {WEEKDAYS.map((weekday) => (
        <div key={weekday} className="posts-calendar-weekday">{weekday}</div>
      ))}
      {cells.map((cell) => {
        const dayPosts = postsByDate.get(cell.dateKey) || [];
        const visibleDayPosts = dayPosts.slice(0, 4);
        return (
          <div
            key={cell.dateKey}
            className={`posts-calendar-day ${cell.isCurrentMonth ? "" : "outside"} ${cell.isToday ? "today" : ""}`}
          >
            <div className="posts-calendar-day-number">
              <span>{cell.dayOfMonth}</span>
            </div>
            <div className="posts-calendar-events">
              {visibleDayPosts.map((post) => (
                <CalendarEventButton
                  key={post.id}
                  post={post}
                  profilesById={profilesById}
                  profileColors={profileColors}
                  onClick={(event) => handleSelectPost(post.id, event.currentTarget)}
                />
              ))}
              {dayPosts.length > visibleDayPosts.length ? (
                <button
                  type="button"
                  className="posts-calendar-more"
                  onClick={(event) => handleSelectPost(dayPosts[4].id, event.currentTarget)}
                >
                  + {dayPosts.length - visibleDayPosts.length} more
                </button>
              ) : null}
            </div>
          </div>
        );
      })}
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
      className="posts-calendar-week-grid"
      aria-label={ariaLabel}
      onWheel={interactive ? handleCalendarWheel : undefined}
      onTouchStart={interactive ? handleCalendarTouchStart : undefined}
      onTouchEnd={interactive ? handleCalendarTouchEnd : undefined}
      style={{ "--calendar-scrollbar-gutter": `${weekScrollbarWidth}px` } as CSSProperties}
    >
      <div className="posts-calendar-week-header">
        <div className="posts-calendar-week-header-inner">
          <div className="posts-calendar-all-day-label">all-day</div>
          {days.map((day) => (
            <div key={day.dateKey} className={`posts-calendar-week-heading ${day.isToday ? "today" : ""}`}>
              <span>{formatWeekdayShort(day.date)}</span>
              <strong>{day.dayOfMonth}</strong>
            </div>
          ))}
        </div>
        <div className="posts-calendar-week-scrollbar-spacer" aria-hidden="true" />
      </div>
      <div
        ref={attachScrollbarRef ? weekTimeScrollRef : null}
        className="posts-calendar-time-scroll"
        style={timelineStyle}
      >
        <TimeLabels />
        <div className="posts-calendar-week-columns">
          {days.map((day) => (
            <TimedPostColumn
              key={day.dateKey}
              posts={postsByDate.get(day.dateKey) || []}
              profilesById={profilesById}
              profileColors={profileColors}
              onSelectPost={handleSelectPost}
            />
          ))}
        </div>
      </div>
    </div>
  );

  const renderDayView = () => (
    <div className="posts-calendar-day-grid" aria-label={`${calendarTitle} day posts`}>
      <div className="posts-calendar-day-all-day">
        <span>all-day</span>
      </div>
      <div className="posts-calendar-time-scroll" style={timelineStyle}>
        <TimeLabels />
        <div className="posts-calendar-day-column-wrap">
          <TimedPostColumn
            posts={postsByDate.get(dayDateKey) || []}
            profilesById={profilesById}
            profileColors={profileColors}
            onSelectPost={handleSelectPost}
          />
        </div>
      </div>
    </div>
  );

  const renderSwipeTransitionView = () => {
    if (!swipeTransition) return null;

    const axis = swipeTransition.mode === "month" ? "y" : "x";
    const fromLabel = swipeTransition.mode === "month" ? "Previous visible month" : "Previous visible week";
    const toLabel = swipeTransition.mode === "month" ? `${calendarTitle} posts` : `${calendarTitle} week posts`;

    return (
      <div
        key={swipeTransition.id}
        className="posts-calendar-swipe-viewport"
        data-mode={swipeTransition.mode}
        data-axis={axis}
        data-direction={swipeTransition.direction}
        onWheel={handleCalendarWheel}
        onTouchStart={handleCalendarTouchStart}
        onTouchEnd={handleCalendarTouchEnd}
      >
        <div className="posts-calendar-swipe-layer posts-calendar-swipe-layer-from" aria-hidden="true">
          {swipeTransition.mode === "month"
            ? renderMonthView(buildMonthGrid(swipeTransition.fromMonth), { interactive: false, ariaLabel: fromLabel })
            : renderWeekView(buildWeekDays(swipeTransition.fromDate), {
              interactive: false,
              attachScrollbarRef: false,
              ariaLabel: fromLabel,
            })}
        </div>
        <div className="posts-calendar-swipe-layer posts-calendar-swipe-layer-to">
          {swipeTransition.mode === "month"
            ? renderMonthView(buildMonthGrid(swipeTransition.toMonth), { interactive: false, ariaLabel: toLabel })
            : renderWeekView(buildWeekDays(swipeTransition.toDate), {
              interactive: false,
              attachScrollbarRef: false,
              ariaLabel: toLabel,
            })}
        </div>
      </div>
    );
  };

  return (
    <section className="posts-calendar-fullheight" aria-label="Posts calendar">
      <style>{CALENDAR_CSS}</style>
      <aside className="posts-calendar-sidebar" aria-label="Calendar filters">
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
                <PlatformIcon platform={platform} size={15} />
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
            <div className="posts-calendar-segment" aria-label="Calendar view mode">
              <button
                type="button"
                className={calendarMode === "day" ? "active" : ""}
                onClick={() => replaceCalendarMode("day")}
              >
                Day
              </button>
              <button
                type="button"
                className={calendarMode === "week" ? "active" : ""}
                onClick={() => replaceCalendarMode("week")}
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
          {calendarMode === "month"
            ? (swipeTransition?.mode === "month" ? renderSwipeTransitionView() : renderMonthView())
            : null}
          {calendarMode === "week"
            ? (swipeTransition?.mode === "week" ? renderSwipeTransitionView() : renderWeekView())
            : null}
          {calendarMode === "day" ? renderDayView() : null}
        </div>
      </div>

      {selectedPost && selectedPostTarget ? (
        <EventPopover
          post={selectedPost}
          anchorRect={selectedPostTarget.anchorRect}
          profile={getPrimaryProfile(selectedPost, profilesById)}
          color={getPostColor(selectedPost, profilesById, profileColors)}
          timezone={timezone}
          profileId={profileId}
          onClose={closeSelectedPost}
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

function CalendarEventButton({
  post,
  profilesById,
  profileColors,
  onClick,
}: {
  post: SocialPost;
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const profile = getPrimaryProfile(post, profilesById);
  const color = getPostColor(post, profilesById, profileColors);
  const time = formatPostTime(post);
  return (
    <button
      type="button"
      className="posts-calendar-event"
      style={{ "--event-color": color } as CSSProperties}
      onClick={onClick}
      title={`${post.caption || "No title"} - ${meta.label}${profile ? ` - ${profile.name}` : ""}`}
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
  onSelectPost,
}: {
  posts: SocialPost[];
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  onSelectPost: (postId: string, target: HTMLElement) => void;
}) {
  const laneCounts = new Map<number, number>();
  return (
    <div className="posts-calendar-time-column">
      {posts.map((post) => {
        const minute = getCalendarPostMinuteOfDay(post);
        if (minute === null) return null;
        const bucket = Math.floor(minute / 30);
        const lane = laneCounts.get(bucket) || 0;
        laneCounts.set(bucket, lane + 1);
        return (
          <TimedPostButton
            key={post.id}
            post={post}
            profilesById={profilesById}
            profileColors={profileColors}
            top={getTimedEventTop(minute, HOUR_HEIGHT)}
            lane={lane}
            onClick={(event) => onSelectPost(post.id, event.currentTarget)}
          />
        );
      })}
    </div>
  );
}

function TimedPostButton({
  post,
  profilesById,
  profileColors,
  top,
  lane,
  onClick,
}: {
  post: SocialPost;
  profilesById: Map<string, Profile>;
  profileColors: Map<string, string>;
  top: number;
  lane: number;
  onClick: (event: MouseEvent<HTMLButtonElement>) => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const profile = getPrimaryProfile(post, profilesById);
  const color = getPostColor(post, profilesById, profileColors);
  const time = formatPostTime(post);
  return (
    <button
      type="button"
      className="posts-calendar-timed-event"
      style={{
        "--event-color": color,
        top: `${Math.max(4, top + lane * 6)}px`,
        left: `${7 + lane * 5}px`,
        right: `${7 + lane * 5}px`,
      } as CSSProperties}
      onClick={onClick}
      title={`${post.caption || "No title"} - ${meta.label}${profile ? ` - ${profile.name}` : ""}`}
    >
      <span className="posts-calendar-event-rail" />
      <span className="posts-calendar-timed-content">
        <span className="posts-calendar-timed-title">{post.caption || "No title"}</span>
        <span className="posts-calendar-timed-meta">{meta.short}{time ? ` - ${time}` : ""}</span>
      </span>
    </button>
  );
}

function EventPopover({
  post,
  anchorRect,
  profile,
  color,
  timezone,
  profileId,
  onClose,
}: {
  post: SocialPost;
  anchorRect: CalendarPopoverRect;
  profile: Profile | null;
  color: string;
  timezone: string;
  profileId: string;
  onClose: () => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
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
    () => getAnchoredPopoverPlacement({ anchor: anchorRect, viewport: viewportSize, popover: popoverSize }),
    [anchorRect, popoverSize, viewportSize],
  );
  const popoverStyle = {
    "--event-color": color,
    "--popover-left": `${placement.left}px`,
    "--popover-top": `${placement.top}px`,
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
                    <span key={platform}>
                      <PlatformIcon platform={platform} size={14} />
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

        <Link className="posts-calendar-open-list" href={`/projects/${profileId}/posts/list?post=${post.id}`}>
          Open in List
        </Link>
      </article>
    </div>
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

function startOfMonth(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), 1);
}

function addMonths(date: Date, amount: number): Date {
  return new Date(date.getFullYear(), date.getMonth() + amount, 1);
}

function addDays(date: Date, days: number): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate() + days);
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
.posts-calendar-fullheight{min-height:calc(100dvh - 86px);display:grid;grid-template-columns:248px minmax(0,1fr);background:var(--surface);border:1px solid var(--dborder);border-radius:18px;overflow:hidden;box-shadow:0 18px 46px color-mix(in srgb,var(--shadow-color) 90%,transparent)}
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
.posts-calendar-topbar{min-height:76px;padding:15px 18px;border-bottom:1px solid var(--dborder);display:flex;align-items:center;justify-content:space-between;gap:18px}
.posts-calendar-title-block{min-width:0}
.posts-calendar-title-block h1{font-size:32px;line-height:1.05;font-weight:760;color:var(--dtext);margin:0;letter-spacing:0}
.posts-calendar-title-block span{display:block;margin-top:5px;font-size:13px;color:var(--dmuted2)}
.posts-calendar-toolbar{display:flex;align-items:center;justify-content:flex-end;gap:10px;flex-wrap:wrap}
.posts-calendar-segment{display:flex;align-items:center;border:1px solid var(--dborder);background:var(--surface2);border-radius:999px;padding:3px}
.posts-calendar-segment button{height:30px;min-width:62px;border:0;border-radius:999px;background:transparent;color:var(--dmuted);font:inherit;font-size:14px;font-weight:650}
.posts-calendar-segment button.active{background:var(--surface-raised);color:var(--dtext);box-shadow:0 1px 0 color-mix(in srgb,var(--shadow-color) 70%,transparent)}
.posts-calendar-segment button:disabled{cursor:not-allowed}
.posts-calendar-month-nav{display:flex;align-items:center;gap:4px}
.posts-calendar-month-nav button,.posts-calendar-list-link,.posts-calendar-create{height:34px;display:inline-flex;align-items:center;justify-content:center;gap:7px;border:1px solid var(--dborder);border-radius:999px;background:var(--surface2);color:var(--dtext);font:inherit;font-size:14px;font-weight:650;text-decoration:none;padding:0 12px;cursor:pointer;transition:background .12s,border-color .12s,transform .12s}
.posts-calendar-month-nav button:first-child,.posts-calendar-month-nav button:last-child{width:34px;padding:0}
.posts-calendar-month-nav button:hover,.posts-calendar-list-link:hover,.posts-calendar-create:hover{background:var(--surface3);border-color:var(--dborder2)}
.posts-calendar-month-nav button:active,.posts-calendar-list-link:active,.posts-calendar-create:active{transform:translateY(1px)}
.posts-calendar-create{background:var(--daccent);border-color:var(--daccent);color:var(--primary-foreground)}
.posts-calendar-error{margin:12px 18px 0;border:1px solid color-mix(in srgb,var(--danger) 24%,transparent);background:var(--danger-soft);color:var(--danger);border-radius:10px;padding:10px 12px;font-size:13px;line-height:1.45}
.posts-calendar-view-stage{flex:1;min-height:0;display:flex;overflow:hidden;background:var(--surface)}
.posts-calendar-swipe-viewport{--calendar-swipe-duration:260ms;--calendar-swipe-distance:100%;--calendar-swipe-from-x:0px;--calendar-swipe-from-y:0px;--calendar-swipe-to-x:0px;--calendar-swipe-to-y:0px;position:relative;flex:1;min-width:0;min-height:0;overflow:hidden;background:var(--surface);contain:layout paint}
.posts-calendar-swipe-viewport[data-mode="month"]{--calendar-swipe-distance:calc((100% - 34px) / 6)}
.posts-calendar-swipe-viewport[data-mode="week"]{--calendar-swipe-distance:calc((100% - var(--calendar-time-gutter,76px)) / 7)}
.posts-calendar-swipe-viewport[data-axis="x"][data-direction="1"]{--calendar-swipe-from-x:calc(0px - var(--calendar-swipe-distance));--calendar-swipe-to-x:var(--calendar-swipe-distance)}
.posts-calendar-swipe-viewport[data-axis="x"][data-direction="-1"]{--calendar-swipe-from-x:var(--calendar-swipe-distance);--calendar-swipe-to-x:calc(0px - var(--calendar-swipe-distance))}
.posts-calendar-swipe-viewport[data-axis="y"][data-direction="1"]{--calendar-swipe-from-y:calc(0px - var(--calendar-swipe-distance));--calendar-swipe-to-y:var(--calendar-swipe-distance)}
.posts-calendar-swipe-viewport[data-axis="y"][data-direction="-1"]{--calendar-swipe-from-y:var(--calendar-swipe-distance);--calendar-swipe-to-y:calc(0px - var(--calendar-swipe-distance))}
.posts-calendar-swipe-layer{position:absolute;inset:0;display:flex;min-width:0;min-height:0;will-change:transform;contain:layout paint;backface-visibility:hidden;transform:translate3d(0,0,0);pointer-events:none}
.posts-calendar-swipe-layer>.posts-calendar-grid,.posts-calendar-swipe-layer>.posts-calendar-week-grid{flex:1;height:100%;min-height:0}
.posts-calendar-swipe-layer>.posts-calendar-week-grid{overflow:hidden}
.posts-calendar-swipe-layer-from{z-index:1;animation:posts-calendar-swipe-from var(--calendar-swipe-duration) cubic-bezier(.22,1,.36,1) both}
.posts-calendar-swipe-layer-to{z-index:2;animation:posts-calendar-swipe-to var(--calendar-swipe-duration) cubic-bezier(.22,1,.36,1) both}
@keyframes posts-calendar-swipe-from{from{transform:translate3d(0,0,0)}to{transform:translate3d(var(--calendar-swipe-from-x),var(--calendar-swipe-from-y),0)}}
@keyframes posts-calendar-swipe-to{from{transform:translate3d(var(--calendar-swipe-to-x),var(--calendar-swipe-to-y),0)}to{transform:translate3d(0,0,0)}}
.posts-calendar-grid{flex:1;min-height:640px;display:grid;grid-template-columns:repeat(7,minmax(0,1fr));grid-template-rows:34px repeat(6,minmax(104px,1fr));background:var(--dborder);gap:1px;overscroll-behavior:contain;touch-action:none}
.posts-calendar-weekday{background:var(--surface);display:flex;align-items:center;justify-content:flex-end;padding:0 12px;color:var(--dmuted);font-size:13px;font-weight:650}
.posts-calendar-day{background:var(--surface);min-width:0;min-height:104px;padding:8px 6px 7px;display:flex;flex-direction:column;gap:5px}
.posts-calendar-day.outside{background:color-mix(in srgb,var(--surface2) 42%,var(--surface));color:var(--dmuted2)}
.posts-calendar-day-number{display:flex;justify-content:flex-end;height:22px;font-size:16px;color:var(--dmuted);font-weight:600}
.posts-calendar-day.today .posts-calendar-day-number span{display:inline-flex;align-items:center;justify-content:center;width:24px;height:24px;border-radius:999px;background:var(--danger);color:white;margin-top:-2px}
.posts-calendar-events{display:flex;flex-direction:column;gap:4px;min-width:0}
.posts-calendar-event{--event-color:#8b8b93;position:relative;display:grid;grid-template-columns:3px auto minmax(0,1fr) auto;align-items:center;gap:5px;width:100%;min-height:22px;border:1px solid color-mix(in srgb,var(--event-color) 22%,transparent);border-radius:6px;background:color-mix(in srgb,var(--event-color) 17%,var(--surface));color:var(--dtext);font:inherit;text-align:left;padding:2px 6px 2px 4px;cursor:pointer;overflow:hidden}
.posts-calendar-event:hover{border-color:color-mix(in srgb,var(--event-color) 48%,var(--dborder));background:color-mix(in srgb,var(--event-color) 25%,var(--surface))}
.posts-calendar-event-rail{width:3px;align-self:stretch;border-radius:99px;background:var(--event-color)}
.posts-calendar-event-status{font-size:9px;font-weight:800;letter-spacing:.04em;color:color-mix(in srgb,var(--event-color) 84%,var(--dtext));line-height:1}
.posts-calendar-event-caption{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:650}
.posts-calendar-event-time{font-size:11px;color:var(--dmuted2);white-space:nowrap}
.posts-calendar-more{height:22px;border:0;border-radius:6px;background:transparent;color:var(--dmuted);font:inherit;font-size:12px;font-weight:650;text-align:left;padding:0 7px;cursor:pointer}
.posts-calendar-more:hover{background:var(--surface2);color:var(--dtext)}
.posts-calendar-week-grid,.posts-calendar-day-grid{--calendar-time-gutter:76px;--calendar-week-day-min:132px;--calendar-scrollbar-gutter:0px;--calendar-week-min-width:calc(1007px + var(--calendar-scrollbar-gutter));--calendar-week-template:var(--calendar-time-gutter) repeat(7,minmax(var(--calendar-week-day-min),1fr));flex:1;min-width:0;min-height:0;display:flex;flex-direction:column;background:var(--surface)}
.posts-calendar-week-grid{overscroll-behavior-x:contain;overflow-x:auto;touch-action:pan-y}
.posts-calendar-week-header{display:grid;grid-template-columns:minmax(0,1fr) var(--calendar-scrollbar-gutter);border-bottom:1px solid var(--dborder);background:var(--dborder);min-width:var(--calendar-week-min-width)}
.posts-calendar-week-header-inner{display:grid;grid-template-columns:var(--calendar-week-template);background:var(--dborder);gap:1px;min-width:0}
.posts-calendar-week-scrollbar-spacer{background:var(--surface)}
.posts-calendar-all-day-label,.posts-calendar-day-all-day{height:44px;display:flex;align-items:center;color:var(--dmuted);font-size:13px;font-weight:650;background:var(--surface);padding:0 12px;white-space:nowrap;word-break:keep-all;hyphens:none}
.posts-calendar-week-heading{height:44px;background:var(--surface);display:flex;align-items:center;justify-content:center;gap:7px;color:var(--dmuted);font-size:13px;font-weight:650}
.posts-calendar-week-heading strong{font-size:17px;color:var(--dtext);font-weight:720}
.posts-calendar-week-heading.today strong{display:inline-flex;align-items:center;justify-content:center;width:26px;height:26px;border-radius:999px;background:var(--danger);color:white}
.posts-calendar-day-all-day{height:34px;border-bottom:1px solid var(--dborder)}
.posts-calendar-time-scroll{flex:1;min-height:0;display:grid;grid-template-columns:var(--calendar-time-gutter) minmax(0,1fr);overflow-y:auto;overflow-x:hidden;overscroll-behavior:contain;background:var(--surface)}
.posts-calendar-week-grid .posts-calendar-time-scroll{grid-template-columns:var(--calendar-week-template);gap:1px;min-width:var(--calendar-week-min-width);background:var(--dborder)}
.posts-calendar-time-labels{height:var(--calendar-timeline-height,calc(24 * var(--hour-height,64px)));background:var(--surface);border-right:1px solid var(--dborder)}
.posts-calendar-time-label{height:var(--hour-height,64px);display:flex;align-items:flex-start;justify-content:flex-end;padding:5px 8px 0 4px;color:var(--dmuted);font-size:12px;font-weight:650;border-top:1px solid var(--dborder);white-space:nowrap}
.posts-calendar-week-columns{grid-column:2/-1;min-width:0;display:grid;grid-template-columns:repeat(7,minmax(var(--calendar-week-day-min),1fr));background:var(--dborder);gap:1px}
.posts-calendar-day-column-wrap{min-width:0;background:var(--dborder);padding-left:1px}
.posts-calendar-time-column{position:relative;height:var(--calendar-timeline-height,calc(24 * var(--hour-height,64px)));background-color:var(--surface);background-image:repeating-linear-gradient(to bottom,transparent 0 calc(var(--hour-height,64px) - 1px),var(--dborder) calc(var(--hour-height,64px) - 1px) var(--hour-height,64px));overflow:hidden}
.posts-calendar-timed-event{--event-color:#8b8b93;position:absolute;min-height:var(--calendar-timed-event-min-height,38px);border:1px solid color-mix(in srgb,var(--event-color) 30%,transparent);border-radius:7px;background:color-mix(in srgb,var(--event-color) 21%,var(--surface));color:var(--dtext);font:inherit;text-align:left;padding:5px 7px 5px 5px;display:grid;grid-template-columns:3px minmax(0,1fr);gap:7px;cursor:pointer;box-shadow:0 1px 0 color-mix(in srgb,var(--shadow-color) 52%,transparent);overflow:hidden}
.posts-calendar-timed-event:hover{border-color:color-mix(in srgb,var(--event-color) 54%,var(--dborder));background:color-mix(in srgb,var(--event-color) 28%,var(--surface))}
.posts-calendar-timed-content{min-width:0;display:flex;flex-direction:column;gap:2px}
.posts-calendar-timed-title{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:12px;font-weight:760;line-height:1.15}
.posts-calendar-timed-meta{font-size:11px;color:color-mix(in srgb,var(--event-color) 78%,var(--dmuted));font-weight:700;line-height:1.15;white-space:nowrap}
.posts-calendar-popover-layer{position:fixed;inset:0;background:transparent;z-index:90}
.posts-calendar-popover{position:fixed;left:var(--popover-left);top:var(--popover-top);width:min(420px,calc(100vw - 24px));max-height:calc(100dvh - 24px);background:var(--surface-raised);border:1px solid var(--dborder);border-radius:16px;box-shadow:0 24px 70px color-mix(in srgb,var(--shadow-color) 160%,transparent);padding:16px;transform-origin:var(--popover-transform-origin);animation:posts-calendar-popover-open .18s cubic-bezier(.16,1,.3,1)}
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
.posts-calendar-popover-profile span{width:9px;height:9px;border-radius:999px;background:var(--event-color)}
.posts-calendar-popover-meta{display:grid;gap:12px;margin:0}
.posts-calendar-popover-meta div{display:grid;grid-template-columns:82px minmax(0,1fr);gap:12px;align-items:flex-start}
.posts-calendar-popover-meta dt{font-size:12px;font-weight:700;letter-spacing:.04em;text-transform:uppercase;color:var(--dmuted2)}
.posts-calendar-popover-meta dd{margin:0;color:var(--dtext);font-size:14px;line-height:1.45}
.posts-calendar-popover-status{display:inline-flex;align-items:center;height:19px;border-radius:5px;padding:0 5px;margin-right:7px;background:color-mix(in srgb,var(--event-color) 20%,transparent);color:color-mix(in srgb,var(--event-color) 80%,var(--dtext));font-size:10px;font-weight:800;letter-spacing:.04em}
.posts-calendar-popover-platforms{display:flex;flex-wrap:wrap;gap:6px}
.posts-calendar-popover-platforms span{display:inline-flex;align-items:center;gap:5px;border:1px solid var(--dborder);background:var(--surface2);border-radius:999px;padding:3px 8px;font-size:12px;font-weight:650}
.posts-calendar-open-list{display:inline-flex;align-items:center;justify-content:center;margin-top:17px;width:100%;height:36px;border-radius:10px;background:var(--surface2);border:1px solid var(--dborder);color:var(--dtext);text-decoration:none;font-size:14px;font-weight:700}
.posts-calendar-open-list:hover{background:var(--surface3)}
@media (max-width: 980px){.posts-calendar-fullheight{grid-template-columns:1fr}.posts-calendar-sidebar{border-right:0;border-bottom:1px solid var(--dborder);display:grid;grid-template-columns:repeat(3,minmax(0,1fr));align-items:start}.posts-calendar-sidebar-top{grid-column:1/-1}.posts-calendar-topbar{align-items:flex-start;flex-direction:column}.posts-calendar-toolbar{justify-content:flex-start}.posts-calendar-grid{min-height:720px;grid-template-rows:34px repeat(6,minmax(114px,1fr))}}
@media (max-width: 680px){.posts-calendar-fullheight{border-radius:12px}.posts-calendar-sidebar{grid-template-columns:1fr}.posts-calendar-title-block h1{font-size:26px}.posts-calendar-segment button{min-width:54px}.posts-calendar-grid{overflow-x:auto;grid-template-columns:repeat(7,minmax(132px,1fr))}.posts-calendar-popover{width:min(360px,calc(100vw - 24px))}}
@media (prefers-reduced-motion:reduce){.posts-calendar-popover,.posts-calendar-swipe-layer-from,.posts-calendar-swipe-layer-to{animation:none}.posts-calendar-swipe-layer-from{display:none}}
`;
