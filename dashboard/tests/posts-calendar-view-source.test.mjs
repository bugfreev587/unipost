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
  assert.match(source, /usePathname,\s*useParams,\s*useRouter,\s*useSearchParams/);
  assert.match(source, /parseCalendarViewMode\(searchParams\.get\("view"\)\)/);
  assert.match(source, /replaceCalendarMode\("day"\)/);
  assert.match(source, /replaceCalendarMode\("week"\)/);
  assert.match(source, /replaceCalendarMode\("month"\)/);
  assert.match(source, /nextParams\.set\("view",\s*mode\)/);
  assert.match(source, /router\.replace\(`\$\{pathname\}\$\{query \? `\?\$\{query\}` : ""\}`,\s*\{\s*scroll:\s*false\s*\}\)/);
  assert.match(source, /renderWeekView/);
  assert.match(source, /renderDayView/);
  assert.match(source, /handleCalendarWheel/);
  assert.match(source, /handleCalendarTouchStart/);
  assert.match(source, /handleCalendarTouchEnd/);
  assert.match(source, /shiftCalendarBySwipe/);
  assert.match(source, /SwipeTransitionState/);
  assert.match(source, /startCalendarSwipeTransition/);
  assert.match(source, /SWIPE_TRANSITION_MS/);
  assert.match(source, /posts-calendar-swipe-viewport/);
  assert.match(source, /posts-calendar-swipe-layer/);
  assert.match(source, /posts-calendar-swipe-from/);
  assert.match(source, /posts-calendar-swipe-to/);
  assert.match(source, /renderMonthWeekdayHeader/);
  assert.match(source, /renderMonthDayGrid/);
  assert.match(source, /renderWeekTimeGutter/);
  assert.match(source, /renderWeekColumns/);
  assert.match(source, /posts-calendar-month-shell/);
  assert.match(source, /posts-calendar-month-view/);
  assert.match(source, /posts-calendar-month-weekdays/);
  assert.match(source, /posts-calendar-month-days/);
  assert.match(source, /renderWeekHeader/);
  assert.match(source, /posts-calendar-week-shell/);
  assert.match(source, /posts-calendar-week-body/);
  assert.match(source, /posts-calendar-week-time-gutter/);
  assert.match(source, /posts-calendar-week-content/);
  assert.match(source, /posts-calendar-week-header-row/);
  assert.match(source, /will-change:transform/);
  assert.match(source, /contain:layout paint/);
  assert.match(source, /getAccumulatedWheelNavigationIntent/);
  assert.match(source, /wheelDeltaRef/);
  assert.match(source, /wheelResetTimerRef/);
  assert.match(source, /clearTimeout\(wheelResetTimerRef\.current\)/);
  assert.match(source, /setTimeout\(\(\) => \{/);
  assert.match(source, /shiftCalendarDateBySwipe/);
  assert.match(source, /getSwipeNavigationIntent/);
  assert.match(source, /onTouchStart=\{handleCalendarTouchStart\}/);
  assert.match(source, /onTouchEnd=\{handleCalendarTouchEnd\}/);
  assert.match(source, /formatWeekdayShort\(day\.date\)/);
  assert.match(source, /posts-calendar-week-grid/);
  assert.match(source, /posts-calendar-day-grid/);
  assert.match(source, /posts-calendar-time-scroll/);
  assert.match(source, /timelineStyle/);
  assert.match(source, /getTimedEventLayouts/);
  assert.match(source, /left: `calc\(\$\{layout\.leftPercent\}% \+ 6px\)`/);
  assert.match(source, /width: `calc\(\$\{layout\.widthPercent\}% - 12px\)`/);
  assert.match(source, /--calendar-timeline-height/);
  assert.match(source, /--calendar-timed-event-min-height/);
  assert.match(source, /ref=\{attachScrollbarRef \? weekTimeScrollRef : null\}/);
  assert.match(source, /<div className="posts-calendar-time-scroll" style=\{timelineStyle\}>/);
  assert.match(source, /--calendar-time-gutter:76px/);
  assert.match(source, /--calendar-week-day-min:132px/);
  assert.match(source, /--calendar-week-template:repeat\(7,minmax\(var\(--calendar-week-day-min\),1fr\)\)/);
  assert.match(source, /weekScrollbarWidth/);
  assert.match(source, /offsetWidth - node\.clientWidth/);
  assert.match(source, /posts-calendar-week-header-inner/);
  assert.match(source, /posts-calendar-week-scrollbar-spacer/);
  assert.match(source, /\.posts-calendar-week-header\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) minmax\(0,1fr\)/);
  assert.match(source, /\.posts-calendar-week-header-inner\{[^}]*grid-template-columns:var\(--calendar-week-template\)/);
  assert.match(source, /\.posts-calendar-week-body\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) minmax\(0,1fr\)/);
  assert.match(source, /\.posts-calendar-week-columns\{[^}]*grid-template-columns:repeat\(7,minmax\(var\(--calendar-week-day-min\),1fr\)\)/);
  assert.match(source, /All Status/);
  assert.match(source, /In Progress/);
  assert.match(source, /Cancelled/);
  assert.match(source, /Archived/);
  assert.match(source, /bucketPostByLocalDay/);
  assert.match(source, /buildWeekDays/);
  assert.match(source, /getCalendarPostMinuteOfDay/);
  assert.match(source, /getProfileCalendarColor/);
  assert.match(source, /shouldShowPostForStatusFilter/);
  assert.match(source, /posts-calendar-fullheight/);
  assert.match(source, /resolvedOptions\(\)\.timeZone/);
});

test("Posts calendar swipe transitions keep static headers outside moving layers", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const monthView = source.slice(source.indexOf("const renderMonthView"), source.indexOf("const renderWeekTimeGutter"));
  const weekColumns = source.slice(source.indexOf("const renderWeekColumns"), source.indexOf("const renderWeekView"));
  const weekView = source.slice(source.indexOf("const renderWeekView"));
  const monthTransition = source.slice(source.indexOf("const renderMonthSwipeTransitionView"));
  const weekTransition = source.slice(source.indexOf("const renderWeekSwipeTransitionView"));
  const monthWeekdayIndex = monthTransition.indexOf("{renderMonthWeekdayHeader()}");
  const monthSwipeIndex = monthTransition.indexOf('className="posts-calendar-swipe-viewport"');
  const weekHeaderIndex = weekTransition.indexOf("{renderWeekHeader(");
  const weekSwipeIndex = weekTransition.indexOf('className="posts-calendar-swipe-viewport"');

  assert.match(monthView, /className="posts-calendar-month-shell"/);
  assert.doesNotMatch(monthView, /className="posts-calendar-month-view"[\s\S]*\{renderMonthWeekdayHeader\(\)\}/);
  assert.doesNotMatch(weekColumns, /posts-calendar-week-heading-row/);
  assert.match(weekView, /className="posts-calendar-week-shell"/);
  assert.ok(monthWeekdayIndex >= 0, "month weekday header should render as its own static row");
  assert.ok(monthSwipeIndex >= 0, "month should still render a swipe viewport for date cells");
  assert.ok(monthWeekdayIndex < monthSwipeIndex, "month weekday header should be outside the swipe viewport");
  assert.ok(weekHeaderIndex >= 0, "week date header should render as its own static row");
  assert.ok(weekSwipeIndex >= 0, "week should render moving day columns separately");
  assert.ok(weekHeaderIndex < weekSwipeIndex, "week date header should be outside moving day columns");
  assert.match(source, /renderMonthDayGrid\(buildMonthGrid\(swipeTransition\.fromMonth\)/);
  assert.match(source, /renderWeekColumns\(buildWeekDays\(swipeTransition\.fromDate\)/);
});

test("Posts calendar week view removes all-day row from the week grid", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const weekTimeGutter = source.slice(
    source.indexOf("const renderWeekTimeGutter"),
    source.indexOf("const renderWeekHeader"),
  );
  const weekCss = source.slice(source.indexOf(".posts-calendar-week-grid"));

  assert.doesNotMatch(weekTimeGutter, /all-day/);
  assert.doesNotMatch(weekTimeGutter, /posts-calendar-all-day-label/);
  assert.doesNotMatch(weekCss, /posts-calendar-all-day-label/);
  assert.match(source, /\.posts-calendar-week-time-gutter\{[^}]*display:block/);
});

test("Posts calendar month and week headers place dividers per view", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const topbarCss = source.match(/\.posts-calendar-topbar\{[^}]*\}/)?.[0] ?? "";

  assert.match(source, /className="posts-calendar-topbar"/);
  assert.doesNotMatch(topbarCss, /border-bottom/);
  assert.doesNotMatch(source, /posts-calendar-topbar\.with-divider/);
  assert.match(source, /\.posts-calendar-month-weekdays\{[^}]*border-bottom:1px solid var\(--dborder\)/);
  assert.doesNotMatch(source.match(/\.posts-calendar-month-weekdays\{[^}]*\}/)?.[0] ?? "", /border-top/);
  assert.match(source, /\.posts-calendar-week-header\{[^}]*border-top:1px solid var\(--dborder\)/);
  assert.match(source, /\.posts-calendar-week-header\{[^}]*border-bottom:1px solid var\(--dborder\)/);
  assert.match(source, /\.posts-calendar-week-columns\{[^}]*border-top:1px solid var\(--dborder\)/);
});

test("Posts calendar day view removes all-day row and its top divider", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const dayView = source.slice(source.indexOf("const renderDayView"), source.indexOf("const renderMonthSwipeTransitionView"));
  const dayCss = source.slice(source.indexOf(".posts-calendar-week-shell"));

  assert.doesNotMatch(dayView, /all-day/);
  assert.doesNotMatch(dayView, /posts-calendar-day-all-day/);
  assert.doesNotMatch(dayCss, /posts-calendar-day-all-day/);
  assert.doesNotMatch(source, /with-divider/);
});

test("Posts calendar swipe handlers avoid passive listener preventDefault warnings", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const wheelHandler = source.slice(
    source.indexOf("const handleCalendarWheel"),
    source.indexOf("const handleCalendarTouchStart"),
  );
  const touchEndHandler = source.slice(
    source.indexOf("const handleCalendarTouchEnd"),
    source.indexOf("const handleWeekTimelineScroll"),
  );

  assert.doesNotMatch(wheelHandler, /preventDefault\(/);
  assert.doesNotMatch(touchEndHandler, /preventDefault\(/);
});

test("Posts calendar edit inspector keeps fixed actions visible and profile beside the title", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const inspector = source.slice(
    source.indexOf("function CalendarEditInspector"),
    source.indexOf("function CalendarEditMediaStrip"),
  );
  const css = source.slice(source.indexOf(".posts-calendar-edit-inspector"));
  const bodyStart = inspector.indexOf('<div className="posts-calendar-edit-body">');
  const footerCloseIndex = inspector.indexOf("\n        </footer>", bodyStart);
  const footerIndex = inspector.indexOf('<footer className="posts-calendar-edit-footer">');
  const titleRowStart = inspector.indexOf('className="posts-calendar-edit-title-row"');
  const titleRowEnd = inspector.indexOf('<button type="button" aria-label="Close editor"', titleRowStart);
  const titleRow = inspector.slice(titleRowStart, titleRowEnd);

  assert.ok(bodyStart >= 0, "edit body should be present");
  assert.ok(footerIndex > bodyStart, "footer should remain after the scrollable body");
  assert.ok(footerCloseIndex > footerIndex, "footer should close before the article closes");
  assert.match(inspector, /boundsRect/);
  assert.match(inspector, /getBoundedCalendarPopoverPlacement/);
  assert.match(inspector, /"--popover-available-height": `\$\{placement\.availableHeight\}px`/);
  assert.match(source, /function getCalendarEditorBoundsRect/);
  assert.match(source, /closest\("\.posts-calendar-month-view, \.posts-calendar-week-grid, \.posts-calendar-day-grid"\)/);
  assert.match(css, /max-height:min\(calc\(100dvh - 24px\),var\(--popover-available-height,calc\(100dvh - 24px\)\)\)/);
  assert.match(css, /\.posts-calendar-edit-header,\s*\.posts-calendar-edit-footer\{[^}]*flex:0 0 auto/);
  assert.match(css, /\.posts-calendar-edit-body\{[^}]*flex:1 1 auto/);
  assert.match(inspector, />Cancel</);
  assert.ok(titleRowStart >= 0, "title row should group title and profile");
  assert.ok(titleRow.indexOf("<h2>Edit post</h2>") < titleRow.indexOf("posts-calendar-popover-profile"));
});

test("Posts calendar details popover anchors to the selected event button", async () => {
  const source = await readFile(calendarViewPath, "utf8");

  assert.match(source, /getAnchoredPopoverPlacement/);
  assert.match(source, /selectedPostTarget/);
  assert.match(source, /handleSelectPost/);
  assert.match(source, /getElementRect/);
  assert.match(source, /getBoundingClientRect\(\)/);
  assert.match(source, /anchorRect=\{selectedPostTarget\.anchorRect\}/);
  assert.match(source, /data-side=\{placement\.side\}/);
  assert.match(source, /--popover-left/);
  assert.match(source, /--popover-arrow-y/);
  assert.match(source, /posts-calendar-popover-open/);
  assert.match(source, /\.posts-calendar-popover\[data-side="right"\]::before/);
  assert.doesNotMatch(source, /background:color-mix\(in srgb,var\(--overlay\) 48%,transparent\)/);
});

test("Posts calendar details popover mirrors list view post details", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const popover = source.slice(
    source.indexOf("function EventPopover"),
    source.indexOf("function CalendarEditInspector"),
  );
  const detailsHelpers = source.slice(source.indexOf("function CalendarPostDetailGrid"));

  assert.match(popover, /<CalendarPostDetailGrid post=\{post\} meta=\{meta\} \/>/);
  assert.match(popover, /<CalendarPostResults post=\{post\} \/>/);
  assert.match(detailsHelpers, /function CalendarPostDetailGrid/);
  assert.match(detailsHelpers, /Caption/);
  assert.match(detailsHelpers, /Mode/);
  assert.match(detailsHelpers, /Created/);
  assert.match(detailsHelpers, /Scheduled/);
  assert.match(detailsHelpers, /Published/);
  assert.match(detailsHelpers, /function CalendarPostResults/);
  assert.match(detailsHelpers, /function CalendarPostResultCard/);
  assert.match(detailsHelpers, /title="Open original post"/);
  assert.match(detailsHelpers, /postUrlFor\(result\.platform/);
  assert.match(detailsHelpers, /Published successfully\./);
  assert.match(detailsHelpers, /Submitted settings/);
  assert.match(detailsHelpers, /buildSubmittedRows/);
  assert.match(source, /\.posts-calendar-detail-grid\{display:grid;grid-template-columns:repeat\(3,minmax\(0,1fr\)\)/);
  assert.doesNotMatch(detailsHelpers, /label="Caption"[^\n]+wide/);
  assert.match(source, /\.posts-calendar-result-card/);
  assert.match(source, /\.posts-calendar-submitted-panel/);
});
