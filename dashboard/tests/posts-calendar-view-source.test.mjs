import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const calendarViewPath = path.join(root, "src/components/posts/calendar/posts-calendar-view.tsx");

test("Posts calendar view keeps the requested calendar controls and drawer integration", async () => {
  const source = await readFile(calendarViewPath, "utf8");

  assert.match(source, /export function PostsCalendarView/);
  assert.match(source, /CreatePostDrawer/);
  assert.match(source, /List View/);
  assert.match(source, /Create \+/);
  assert.match(source, /Day/);
  assert.match(source, /Week/);
  assert.match(source, /Month/);
  assert.match(source, /setCalendarMode\("day"\)/);
  assert.match(source, /setCalendarMode\("week"\)/);
  assert.match(source, /renderWeekView/);
  assert.match(source, /renderDayView/);
  assert.match(source, /handleCalendarWheel/);
  assert.match(source, /posts-calendar-week-grid/);
  assert.match(source, /posts-calendar-day-grid/);
  assert.match(source, /posts-calendar-time-scroll/);
  assert.match(source, /--calendar-time-gutter:76px/);
  assert.match(source, /\.posts-calendar-week-header\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) repeat\(7,minmax\(132px,1fr\)\)/);
  assert.match(source, /\.posts-calendar-time-scroll\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) minmax\(0,1fr\)/);
  assert.match(source, /\.posts-calendar-all-day-label,[^}]*white-space:nowrap/);
  assert.match(source, /All Status/);
  assert.match(source, /In Progress/);
  assert.match(source, /Cancelled/);
  assert.match(source, /Archived/);
  assert.match(source, /bucketPostByLocalDay/);
  assert.match(source, /buildWeekDays/);
  assert.match(source, /getCalendarPostMinuteOfDay/);
  assert.match(source, /getWheelNavigationIntent/);
  assert.match(source, /getProfileCalendarColor/);
  assert.match(source, /shouldShowPostForStatusFilter/);
  assert.match(source, /posts-calendar-fullheight/);
  assert.match(source, /resolvedOptions\(\)\.timeZone/);
});
