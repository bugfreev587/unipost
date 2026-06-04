import test from "node:test";
import assert from "node:assert/strict";
import {
  buildMonthGrid,
  buildRollingMonthGrid,
  buildRollingWeekDays,
  buildWeekDays,
  bucketPostByLocalDay,
  getCalendarPostMinuteOfDay,
  getAnchoredPopoverPlacement,
  getBoundedCalendarPopoverPlacement,
  getAccumulatedWheelNavigationIntent,
  getCalendarSnapOffset,
  getCalendarSnapSteps,
  getContinuousCalendarSnapOffset,
  getTimedEventLayouts,
  getSwipeNavigationIntent,
  getTimedEventTop,
  getTimedTimelineContentHeight,
  getWheelNavigationIntent,
  getPostStatusGroup,
  getProfileCalendarColor,
  shiftCalendarDateBySwipe,
  shiftCalendarDateBySnapSteps,
  parseCalendarViewMode,
  shouldShowPostForStatusFilter,
} from "../src/components/posts/calendar/calendar-model.ts";

test("buildMonthGrid returns a six week Sunday-first grid with adjacent month days", () => {
  const grid = buildMonthGrid(new Date(2026, 4, 1), new Date(2026, 4, 30));

  assert.equal(grid.length, 42);
  assert.equal(grid[0].dateKey, "2026-04-26");
  assert.equal(grid[5].dateKey, "2026-05-01");
  assert.equal(grid[34].dateKey, "2026-05-30");
  assert.equal(grid[41].dateKey, "2026-06-06");
  assert.equal(grid[34].isToday, true);
  assert.equal(grid[0].isCurrentMonth, false);
  assert.equal(grid[5].isCurrentMonth, true);
});

test("buildMonthGrid can roll one visible week without jumping a full month", () => {
  const grid = buildMonthGrid(new Date(2026, 4, 8), new Date(2026, 4, 30));

  assert.equal(grid.length, 42);
  assert.equal(grid[0].dateKey, "2026-05-03");
  assert.equal(grid[6].dateKey, "2026-05-09");
  assert.equal(grid[41].dateKey, "2026-06-13");
  assert.equal(grid[0].isCurrentMonth, true);
  assert.equal(grid[41].isCurrentMonth, false);
});

test("rolling month grid buffers one week above and below the visible window", () => {
  const grid = buildRollingMonthGrid(new Date(2026, 4, 1), 1, 6, 1, new Date(2026, 4, 31));

  assert.equal(grid.length, 56);
  assert.equal(grid[0].dateKey, "2026-04-19");
  assert.equal(grid[7].dateKey, "2026-04-26");
  assert.equal(grid[49].dateKey, "2026-06-07");
  assert.equal(grid[55].dateKey, "2026-06-13");
});

test("bucketPostByLocalDay uses scheduled, then published, then created timestamps", () => {
  assert.equal(
    bucketPostByLocalDay({
      status: "scheduled",
      scheduled_at: "2026-05-04T18:15:00Z",
      created_at: "2026-05-01T00:00:00Z",
    }),
    "2026-05-04",
  );
  assert.equal(
    bucketPostByLocalDay({
      status: "published",
      published_at: "2026-05-08T19:00:00Z",
      created_at: "2026-05-01T00:00:00Z",
    }),
    "2026-05-08",
  );
  assert.equal(bucketPostByLocalDay({ status: "draft", created_at: "2026-05-10T09:00:00Z" }), "2026-05-10");
  assert.equal(bucketPostByLocalDay({ status: "draft" }), null);
});

test("status groups include in-flight, failed partial, cancelled, archived, and unknown", () => {
  assert.equal(getPostStatusGroup({ status: "queued" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "dispatching" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "retrying" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "processing" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "partial" }), "failed");
  assert.equal(getPostStatusGroup({ status: "failed" }), "failed");
  assert.equal(getPostStatusGroup({ status: "cancelled" }), "cancelled");
  assert.equal(getPostStatusGroup({ status: "published", archived_at: "2026-05-30T12:00:00Z" }), "archived");
  assert.equal(getPostStatusGroup({ status: "future_review" }), "unknown");
});

test("status filters default to all posts and exclude archived from non-archived filters", () => {
  assert.equal(
    shouldShowPostForStatusFilter({ status: "published", archived_at: "2026-05-30T12:00:00Z" }, "all"),
    true,
  );
  assert.equal(
    shouldShowPostForStatusFilter({ status: "published", archived_at: "2026-05-30T12:00:00Z" }, "published"),
    false,
  );
  assert.equal(
    shouldShowPostForStatusFilter({ status: "published", archived_at: "2026-05-30T12:00:00Z" }, "archived"),
    true,
  );
  assert.equal(shouldShowPostForStatusFilter({ status: "processing" }, "in_progress"), true);
  assert.equal(shouldShowPostForStatusFilter({ status: "future_review" }, "all"), true);
  assert.equal(shouldShowPostForStatusFilter({ status: "future_review" }, "draft"), false);
});

test("profile colors prefer valid branding colors and otherwise use a stable palette", () => {
  assert.equal(getProfileCalendarColor({ id: "profile-a", name: "A", branding_primary_color: "#1d4ed8" }), "#1d4ed8");
  assert.equal(getProfileCalendarColor({ id: "profile-a", name: "A" }), getProfileCalendarColor({ id: "profile-a", name: "A" }));
  assert.notEqual(getProfileCalendarColor({ id: "profile-a", name: "A" }), getProfileCalendarColor({ id: "profile-b", name: "B" }));
});

test("buildWeekDays returns a rolling seven-day window anchored to the selected day", () => {
  const week = buildWeekDays(new Date(2026, 3, 8), new Date(2026, 3, 10));

  assert.equal(week.length, 7);
  assert.equal(week[0].dateKey, "2026-04-08");
  assert.equal(week[2].dateKey, "2026-04-10");
  assert.equal(week[6].dateKey, "2026-04-14");
  assert.equal(week[2].isToday, true);
});

test("rolling week days buffer the previous and next day for snap scrolling", () => {
  const week = buildRollingWeekDays(new Date(2026, 4, 31), 1, 7, 1, new Date(2026, 4, 31));

  assert.equal(week.length, 9);
  assert.equal(week[0].dateKey, "2026-05-30");
  assert.equal(week[1].dateKey, "2026-05-31");
  assert.equal(week[7].dateKey, "2026-06-06");
  assert.equal(week[8].dateKey, "2026-06-07");
  assert.equal(week[1].isToday, true);
});

test("getCalendarPostMinuteOfDay maps post timestamps onto a day timeline", () => {
  assert.equal(
    getCalendarPostMinuteOfDay({
      status: "scheduled",
      scheduled_at: "2026-04-08T14:30:00",
      created_at: "2026-04-01T00:00:00",
    }),
    870,
  );
  assert.equal(getCalendarPostMinuteOfDay({ status: "draft" }), null);
});

test("timed timeline height preserves late-night event visibility", () => {
  const hourHeight = 64;
  const eventMinHeight = 38;
  const minuteOfDay = 23 * 60 + 46;
  const top = getTimedEventTop(minuteOfDay, hourHeight);
  const contentHeight = getTimedTimelineContentHeight(hourHeight, eventMinHeight, 8);

  assert.equal(Math.round(top), 1521);
  assert.ok(
    top + eventMinHeight <= contentHeight,
    "11:46 PM events should fit inside the scrollable day timeline",
  );
});

test("timed event layouts place same and nearby posts in horizontal lanes", () => {
  const layouts = getTimedEventLayouts(
    [
      { id: "published", minuteOfDay: 11 * 60 + 10 },
      { id: "scheduled", minuteOfDay: 11 * 60 + 11 },
      { id: "later", minuteOfDay: 12 * 60 + 15 },
    ],
    64,
    38,
  );

  assert.deepEqual(layouts.get("published"), {
    id: "published",
    top: 714.6666666666666,
    lane: 0,
    laneCount: 2,
    leftPercent: 0,
    widthPercent: 50,
  });
  assert.deepEqual(layouts.get("scheduled"), {
    id: "scheduled",
    top: 715.7333333333333,
    lane: 1,
    laneCount: 2,
    leftPercent: 50,
    widthPercent: 50,
  });
  assert.deepEqual(layouts.get("later"), {
    id: "later",
    top: 784,
    lane: 0,
    laneCount: 1,
    leftPercent: 0,
    widthPercent: 100,
  });
});

test("wheel navigation follows Apple Calendar style directions per view", () => {
  assert.equal(getWheelNavigationIntent("month", 0, 160), 1);
  assert.equal(getWheelNavigationIntent("month", 0, -160), -1);
  assert.equal(getWheelNavigationIntent("week", 140, 0), 1);
  assert.equal(getWheelNavigationIntent("week", -140, 0), -1);
  assert.equal(getWheelNavigationIntent("day", 0, 160), 0);
});

test("wheel navigation accumulates small trackpad deltas into one swipe", () => {
  let monthAccumulator = { deltaX: 0, deltaY: 0 };
  let monthDirection: -1 | 0 | 1 = 0;

  for (const deltaY of [22, 20, 21, 18]) {
    const result = getAccumulatedWheelNavigationIntent("month", monthAccumulator, 0, deltaY);
    monthAccumulator = result.accumulator;
    monthDirection = result.direction;
  }

  assert.equal(monthDirection, 1);
  assert.deepEqual(monthAccumulator, { deltaX: 0, deltaY: 0 });

  let weekAccumulator = { deltaX: 0, deltaY: 0 };
  let weekDirection: -1 | 0 | 1 = 0;

  for (const deltaX of [-18, -20, -22, -23]) {
    const result = getAccumulatedWheelNavigationIntent("week", weekAccumulator, deltaX, 0);
    weekAccumulator = result.accumulator;
    weekDirection = result.direction;
  }

  assert.equal(weekDirection, -1);
  assert.deepEqual(weekAccumulator, { deltaX: 0, deltaY: 0 });
});

test("touch swipe navigation follows the same calendar directions", () => {
  assert.equal(getSwipeNavigationIntent("month", 120, 240, 120, 80), 1);
  assert.equal(getSwipeNavigationIntent("month", 120, 80, 120, 240), -1);
  assert.equal(getSwipeNavigationIntent("week", 240, 120, 80, 120), 1);
  assert.equal(getSwipeNavigationIntent("week", 80, 120, 240, 120), -1);
  assert.equal(getSwipeNavigationIntent("day", 120, 240, 120, 80), 0);
});

test("swipe date shifts use week granularity in month view and day granularity in week view", () => {
  assert.equal(formatDate(shiftCalendarDateBySwipe("month", new Date(2026, 4, 1), 1)), "2026-05-08");
  assert.equal(formatDate(shiftCalendarDateBySwipe("month", new Date(2026, 4, 1), -1)), "2026-04-24");
  assert.equal(formatDate(shiftCalendarDateBySwipe("week", new Date(2026, 4, 1), 1)), "2026-05-02");
  assert.equal(formatDate(shiftCalendarDateBySwipe("week", new Date(2026, 4, 1), -1)), "2026-04-30");
  assert.equal(formatDate(shiftCalendarDateBySwipe("day", new Date(2026, 4, 1), 1)), "2026-05-01");
});

test("calendar snap steps choose the nearest date line and clamp to buffered cells", () => {
  assert.equal(getCalendarSnapSteps(-61, 100), 1);
  assert.equal(getCalendarSnapSteps(-50, 100), 1);
  assert.equal(getCalendarSnapSteps(-49, 100), 0);
  assert.equal(getCalendarSnapSteps(61, 100), -1);
  assert.equal(getCalendarSnapSteps(50, 100), -1);
  assert.equal(getCalendarSnapSteps(-260, 100), 1);
  assert.equal(getCalendarSnapOffset(1, 100), -100);
  assert.equal(getCalendarSnapOffset(-1, 100), 100);
  assert.equal(formatDate(shiftCalendarDateBySnapSteps("month", new Date(2026, 4, 1), 1)), "2026-05-08");
  assert.equal(formatDate(shiftCalendarDateBySnapSteps("week", new Date(2026, 4, 1), -1)), "2026-04-30");
});

test("continuous calendar snap offsets recycle whole date lines without waiting for idle snap", () => {
  assert.deepEqual(getContinuousCalendarSnapOffset(-260, 100), { steps: 2, offsetPx: -60 });
  assert.deepEqual(getContinuousCalendarSnapOffset(260, 100), { steps: -2, offsetPx: 60 });
  assert.deepEqual(getContinuousCalendarSnapOffset(-99, 100), { steps: 0, offsetPx: -99 });
  assert.deepEqual(getContinuousCalendarSnapOffset(100, 100), { steps: -1, offsetPx: 0 });
  assert.deepEqual(getContinuousCalendarSnapOffset(0, 100), { steps: 0, offsetPx: 0 });
});

test("parseCalendarViewMode keeps v1 scoped to month", () => {
  assert.equal(parseCalendarViewMode("day"), "month");
  assert.equal(parseCalendarViewMode("week"), "month");
  assert.equal(parseCalendarViewMode("month"), "month");
  assert.equal(parseCalendarViewMode("agenda"), "month");
  assert.equal(parseCalendarViewMode(null), "month");
});

test("getAnchoredPopoverPlacement keeps details beside the selected post when space allows", () => {
  const placement = getAnchoredPopoverPlacement({
    anchor: { left: 120, top: 520, right: 260, bottom: 544, width: 140, height: 24 },
    viewport: { width: 1200, height: 780 },
    popover: { width: 420, height: 320 },
  });

  assert.equal(placement.side, "right");
  assert.equal(placement.left, 272);
  assert.equal(placement.top, 372);
  assert.equal(placement.arrowY, 160);
  assert.equal(placement.transformOrigin, "left 160px");
});

test("getAnchoredPopoverPlacement flips and clamps around viewport edges", () => {
  assert.deepEqual(
    getAnchoredPopoverPlacement({
      anchor: { left: 1080, top: 220, right: 1160, bottom: 244, width: 80, height: 24 },
      viewport: { width: 1200, height: 780 },
      popover: { width: 420, height: 320 },
    }),
    {
      side: "left",
      left: 648,
      top: 72,
      arrowX: 420,
      arrowY: 160,
      transformOrigin: "right 160px",
    },
  );

  assert.equal(
    getAnchoredPopoverPlacement({
      anchor: { left: 250, top: 80, right: 340, bottom: 104, width: 90, height: 24 },
      viewport: { width: 620, height: 780 },
      popover: { width: 420, height: 320 },
    }).side,
    "bottom",
  );

  const topPlacement = getAnchoredPopoverPlacement({
    anchor: { left: 250, top: 720, right: 340, bottom: 744, width: 90, height: 24 },
    viewport: { width: 620, height: 780 },
    popover: { width: 420, height: 320 },
  });

  assert.equal(topPlacement.side, "top");
  assert.equal(topPlacement.top, 388);
  assert.equal(topPlacement.left, 85);
  assert.equal(topPlacement.arrowX, 210);
  assert.equal(topPlacement.transformOrigin, "210px bottom");
});

test("getBoundedCalendarPopoverPlacement can grow edit panels to the calendar grid body", () => {
  const placement = getBoundedCalendarPopoverPlacement({
    anchor: { left: 42, top: 712, right: 224, bottom: 736, width: 182, height: 24 },
    viewport: { width: 1440, height: 980 },
    popover: { width: 760, height: 680 },
    bounds: { left: 0, top: 190, right: 1440, bottom: 930, width: 1440, height: 740 },
  });

  assert.equal(placement.side, "right");
  assert.equal(placement.top, 190);
  assert.equal(placement.availableHeight, 740);
  assert.equal(placement.arrowY, 534);
  assert.equal(placement.transformOrigin, "left 534px");
});

test("getBoundedCalendarPopoverPlacement keeps side arrows inside anchor-aligned detail popovers", () => {
  const placement = getBoundedCalendarPopoverPlacement({
    anchor: { left: 495, top: 858, right: 656, bottom: 883, width: 161, height: 25 },
    viewport: { width: 1728, height: 1280 },
    popover: { width: 560, height: 620 },
    bounds: { left: 489, top: 144, right: 1707, bottom: 888, width: 1218, height: 744 },
    verticalStrategy: "anchor",
  });

  assert.equal(placement.side, "right");
  assert.equal(placement.top, 268);
  assert.equal(placement.availableHeight, 620);
  assert.equal(placement.arrowY, 603);
  assert.equal(placement.transformOrigin, "left 603px");
});

test("getBoundedCalendarPopoverPlacement keeps side arrows attached to lower timed posts", () => {
  const placement = getBoundedCalendarPopoverPlacement({
    anchor: {
      left: 652.732666015625,
      top: 866.1171875,
      right: 721.357666015625,
      bottom: 906.5625,
      width: 68.625,
      height: 40.4453125,
    },
    viewport: { width: 1728, height: 907 },
    popover: { width: 560, height: 420 },
    bounds: { left: 489, top: 144, right: 1707, bottom: 888, width: 1218, height: 744 },
    verticalStrategy: "anchor",
  });

  assert.equal(placement.side, "right");
  assert.equal(placement.top, 468);
  assert.equal(placement.availableHeight, 420);
  assert.equal(placement.arrowY, 418);
  assert.equal(placement.transformOrigin, "left 418px");
});

function formatDate(date: Date): string {
  return date.toISOString().slice(0, 10);
}
