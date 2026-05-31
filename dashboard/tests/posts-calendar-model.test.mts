import test from "node:test";
import assert from "node:assert/strict";
import {
  buildMonthGrid,
  buildWeekDays,
  bucketPostByLocalDay,
  getCalendarPostMinuteOfDay,
  getWheelNavigationIntent,
  getPostStatusGroup,
  getProfileCalendarColor,
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

test("status groups include in-flight, failed partial, cancelled, and archived", () => {
  assert.equal(getPostStatusGroup({ status: "queued" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "dispatching" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "retrying" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "processing" }), "in_progress");
  assert.equal(getPostStatusGroup({ status: "partial" }), "failed");
  assert.equal(getPostStatusGroup({ status: "failed" }), "failed");
  assert.equal(getPostStatusGroup({ status: "cancelled" }), "cancelled");
  assert.equal(getPostStatusGroup({ status: "published", archived_at: "2026-05-30T12:00:00Z" }), "archived");
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
});

test("profile colors prefer valid branding colors and otherwise use a stable palette", () => {
  assert.equal(getProfileCalendarColor({ id: "profile-a", name: "A", branding_primary_color: "#1d4ed8" }), "#1d4ed8");
  assert.equal(getProfileCalendarColor({ id: "profile-a", name: "A" }), getProfileCalendarColor({ id: "profile-a", name: "A" }));
  assert.notEqual(getProfileCalendarColor({ id: "profile-a", name: "A" }), getProfileCalendarColor({ id: "profile-b", name: "B" }));
});

test("buildWeekDays returns a Monday-first week around the selected day", () => {
  const week = buildWeekDays(new Date(2026, 3, 8), new Date(2026, 3, 10));

  assert.equal(week.length, 7);
  assert.equal(week[0].dateKey, "2026-04-06");
  assert.equal(week[4].dateKey, "2026-04-10");
  assert.equal(week[6].dateKey, "2026-04-12");
  assert.equal(week[4].isToday, true);
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

test("wheel navigation follows Apple Calendar style directions per view", () => {
  assert.equal(getWheelNavigationIntent("month", 0, 160), 1);
  assert.equal(getWheelNavigationIntent("month", 0, -160), -1);
  assert.equal(getWheelNavigationIntent("week", 140, 0), 1);
  assert.equal(getWheelNavigationIntent("week", -140, 0), -1);
  assert.equal(getWheelNavigationIntent("day", 0, 160), 0);
});
