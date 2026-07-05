import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const calendarViewPath = path.join(root, "src/components/posts/calendar/posts-calendar-view.tsx");

test("Posts calendar view keeps the requested calendar controls and drawer integration", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const segment = source.slice(
    source.indexOf('<div className="posts-calendar-segment"'),
    source.indexOf('<div className="posts-calendar-month-nav"'),
  );
  const dayButtonStart = segment.indexOf('className={calendarMode === "day"');
  const weekButtonStart = segment.indexOf('className={calendarMode === "week"');
  const dayButton = segment.slice(dayButtonStart, segment.indexOf("</button>", dayButtonStart));
  const weekButton = segment.slice(weekButtonStart, segment.indexOf("</button>", weekButtonStart));

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
  assert.doesNotMatch(dayButton, /aria-disabled="true"/);
  assert.doesNotMatch(dayButton, /disabled/);
  assert.doesNotMatch(dayButton, /not available in v1/);
  assert.match(dayButton, /onClick=\{\(\) => \{/);
  assert.match(dayButton, />\s*Day/);
  assert.doesNotMatch(weekButton, /aria-disabled="true"/);
  assert.doesNotMatch(weekButton, /disabled/);
  assert.doesNotMatch(weekButton, /not available in v1/);
  assert.match(weekButton, /onClick=\{\(\) => \{/);
  assert.match(weekButton, />\s*Week/);
  assert.match(source, /getMonthDayPostLayout\(dayPosts\)/);
  assert.match(source, /handleSelectDayOverflow/);
  assert.match(source, /DayOverflowPopover/);
  assert.match(source, /aria-haspopup="dialog"/);
  assert.doesNotMatch(source, /handleSelectPost\(dayPosts\[4\]\.id/);
  assert.match(source, /nextParams\.set\("view",\s*mode\)/);
  assert.match(source, /router\.replace\(`\$\{pathname\}\$\{query \? `\?\$\{query\}` : ""\}`,\s*\{\s*scroll:\s*false\s*\}\)/);
  assert.match(source, /renderWeekView/);
  assert.match(source, /renderDayView/);
  assert.match(source, /handleCalendarWheel/);
  assert.match(source, /handleCalendarTouchStart/);
  assert.match(source, /handleCalendarTouchMove/);
  assert.match(source, /handleCalendarTouchEnd/);
  assert.match(source, /handleCalendarTouchCancel/);
  assert.match(source, /CalendarSnapState/);
  assert.match(source, /SNAP_TRANSITION_MS/);
  assert.match(source, /WHEEL_SNAP_IDLE_MS/);
  assert.match(source, /snapOffsetRef/);
  assert.match(source, /wheelSnapTimerRef/);
  assert.match(source, /snapTransitionTimerRef/);
  assert.match(source, /clearCalendarSnap/);
  assert.match(source, /settleCalendarSnap/);
  assert.match(source, /getCalendarSnapUnitPx/);
  assert.match(source, /getCalendarSnapSteps/);
  assert.match(source, /getCalendarSnapOffset/);
  assert.match(source, /shiftCalendarDateBySnapSteps/);
  assert.match(source, /buildRollingWeekDays/);
  assert.match(source, /posts-calendar-month-track/);
  assert.match(source, /posts-calendar-week-track/);
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
  assert.match(source, /clampCalendarSnapOffset/);
  assert.match(source, /clearTimeout\(wheelSnapTimerRef\.current\)/);
  assert.match(source, /setTimeout\(\(\) => \{/);
  assert.match(source, /onTouchStart=\{interactive \? handleCalendarTouchStart : undefined\}/);
  assert.match(source, /onTouchMove=\{interactive \? handleCalendarTouchMove : undefined\}/);
  assert.match(source, /onTouchEnd=\{interactive \? handleCalendarTouchEnd : undefined\}/);
  assert.match(source, /onTouchCancel=\{interactive \? handleCalendarTouchCancel : undefined\}/);
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
  assert.match(source, /--calendar-week-template:repeat\(9,minmax\(var\(--calendar-week-day-min\),1fr\)\)/);
  assert.match(source, /weekScrollbarWidth/);
  assert.match(source, /offsetWidth - node\.clientWidth/);
  assert.match(source, /posts-calendar-week-header-inner/);
  assert.match(source, /posts-calendar-week-scrollbar-spacer/);
  assert.match(source, /\.posts-calendar-week-header\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) minmax\(0,1fr\)/);
  assert.match(source, /\.posts-calendar-week-header-inner\{[^}]*grid-template-columns:var\(--calendar-week-template\)/);
  assert.match(source, /\.posts-calendar-week-header-row\{[^}]*overflow:hidden/);
  assert.match(source, /\.posts-calendar-week-body\{[^}]*grid-template-columns:var\(--calendar-time-gutter\) minmax\(0,1fr\)/);
  assert.match(source, /\.posts-calendar-week-columns\{[^}]*grid-template-columns:var\(--calendar-week-template\)/);
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

test("Posts calendar mobile layout prioritizes the calendar over filters", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const mobileCss = source.slice(source.indexOf("@media (max-width: 680px)"));

  assert.match(source, /mobileFiltersOpen/);
  assert.match(source, /className="posts-calendar-filter-toggle"/);
  assert.match(source, /aria-controls="posts-calendar-filters"/);
  assert.match(source, /id="posts-calendar-filters"/);
  assert.match(source, /data-mobile-open=\{mobileFiltersOpen \? "true" : "false"\}/);
  assert.match(mobileCss, /\.posts-calendar-filter-toggle\{[^}]*display:inline-flex/);
  assert.match(mobileCss, /\.posts-calendar-sidebar:not\(\[data-mobile-open="true"\]\)\{[^}]*display:none/);
  assert.match(mobileCss, /\.posts-calendar-month-weekdays,\.posts-calendar-month-days\{[^}]*min-width:0[^}]*width:100%/);
  assert.match(mobileCss, /\.posts-calendar-event-caption,\.posts-calendar-event-time\{[^}]*display:none/);
  assert.doesNotMatch(mobileCss, /\.posts-calendar-month-weekdays,\.posts-calendar-month-days\{[^}]*min-width:924px/);
});

test("Posts calendar snap tracks keep fixed gutters outside moving date tracks", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const monthView = source.slice(source.indexOf("const renderMonthView"), source.indexOf("const renderWeekTimeGutter"));
  const weekHeader = source.slice(source.indexOf("const renderWeekHeader"), source.indexOf("const renderWeekColumns"));
  const weekColumns = source.slice(source.indexOf("const renderWeekColumns"), source.indexOf("const renderWeekView"));
  const weekView = source.slice(source.indexOf("const renderWeekView"));
  const monthWeekdayIndex = monthView.indexOf("{renderMonthWeekdayHeader()}");
  const monthTrackIndex = monthView.indexOf("posts-calendar-month-track");
  const weekHeaderIndex = weekView.indexOf("{renderWeekHeader(");
  const weekBodyIndex = weekView.indexOf('className="posts-calendar-week-body"');

  assert.match(monthView, /className="posts-calendar-month-shell"/);
  assert.doesNotMatch(monthView, /className="posts-calendar-month-view"[\s\S]*\{renderMonthWeekdayHeader\(\)\}/);
  assert.doesNotMatch(weekColumns, /posts-calendar-week-heading-row/);
  assert.match(weekView, /className="posts-calendar-week-shell"/);
  assert.ok(monthWeekdayIndex >= 0, "month weekday header should render as its own static row");
  assert.ok(monthTrackIndex >= 0, "month should render a fixed track for date cells");
  assert.ok(monthWeekdayIndex < monthTrackIndex, "month weekday header should stay outside the moving date track");
  assert.ok(weekHeaderIndex >= 0, "week date header should render before the time grid");
  assert.ok(weekBodyIndex >= 0, "week body should render after the header");
  assert.ok(weekHeaderIndex < weekBodyIndex, "week date header should stay outside the time grid");
  assert.match(weekHeader, /posts-calendar-week-header-inner posts-calendar-week-track/);
  assert.match(weekColumns, /posts-calendar-week-columns posts-calendar-week-track/);
  assert.match(source, /\.posts-calendar-month-track\{[^}]*width:100%/);
  assert.match(source, /\.posts-calendar-month-track\{[^}]*height:100%/);
  assert.match(source, /renderMonthDayGrid\(cells,\s*"posts-calendar-month-days posts-calendar-month-track"\)/);
  assert.doesNotMatch(source, /rollingMonthCells/);
  assert.doesNotMatch(source, /buildRollingMonthGrid/);
  assert.match(source, /renderWeekColumns\(interactive \? rollingWeekDays : days/);
  assert.doesNotMatch(source, /posts-calendar-swipe-viewport/);
});

test("Posts calendar event chips separate status color from profile background color", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const calendarEvent = source.slice(
    source.indexOf("function CalendarEventButton"),
    source.indexOf("function TimeLabels"),
  );
  const timedEvent = source.slice(
    source.indexOf("function TimedPostButton"),
    source.indexOf("function EventPopover"),
  );

  assert.match(source, /getCalendarStatusColor/);
  assert.match(calendarEvent, /const statusColor = getCalendarStatusColor\(status\)/);
  assert.match(timedEvent, /const statusColor = getCalendarStatusColor\(status\)/);
  assert.match(calendarEvent, /"--event-status-color": statusColor/);
  assert.match(calendarEvent, /"--event-profile-color": color/);
  assert.match(timedEvent, /"--event-status-color": statusColor/);
  assert.match(timedEvent, /"--event-profile-color": color/);
  assert.match(source, /\.posts-calendar-event-rail\{[^}]*background:var\(--event-status-color\)/);
  assert.match(source, /\.posts-calendar-event-status\{[^}]*color:var\(--event-status-color\)/);
  assert.match(source, /\.posts-calendar-event\{[^}]*background:color-mix\(in srgb,var\(--event-profile-color\)/);
  assert.match(source, /\.posts-calendar-timed-event\{[^}]*background:color-mix\(in srgb,var\(--event-profile-color\)/);
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
  assert.doesNotMatch(source.match(/\.posts-calendar-week-header\{[^}]*\}/)?.[0] ?? "", /border-top/);
  assert.match(source, /\.posts-calendar-week-body\{[^}]*position:relative/);
  assert.match(source, /\.posts-calendar-week-body::before\{[^}]*top:0/);
  assert.match(source, /\.posts-calendar-week-body::before\{[^}]*border-top:1px solid var\(--dborder\)/);
  assert.doesNotMatch(source.match(/\.posts-calendar-week-columns\{[^}]*\}/)?.[0] ?? "", /border-top/);
});

test("Posts calendar day view removes all-day row and its top divider", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const dayView = source.slice(source.indexOf("const renderDayView"), source.indexOf("const renderMonthSwipeTransitionView"));
  const dayCss = source.slice(source.indexOf(".posts-calendar-week-shell"));

  assert.doesNotMatch(dayView, /all-day/);
  assert.doesNotMatch(dayView, /posts-calendar-day-all-day/);
  assert.doesNotMatch(dayCss, /posts-calendar-day-all-day/);
  assert.match(source, /\.posts-calendar-day-grid \.posts-calendar-time-label:first-child\{[^}]*border-top:0/);
  assert.doesNotMatch(source, /with-divider/);
});

test("Posts calendar shades weekend panels while keeping the week header neutral", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const monthDayGrid = source.slice(source.indexOf("const renderMonthDayGrid"), source.indexOf("const renderMonthView"));
  const monthWeekdayHeader = source.slice(source.indexOf("const renderMonthWeekdayHeader"), source.indexOf("const renderMonthDayGrid"));
  const weekHeader = source.slice(source.indexOf("const renderWeekHeader"), source.indexOf("const renderWeekColumns"));
  const weekColumns = source.slice(source.indexOf("const renderWeekColumns"), source.indexOf("const renderWeekView"));
  const dayView = source.slice(source.indexOf("const renderDayView"), source.indexOf("const renderMonthSwipeTransitionView"));
  const timedColumn = source.slice(source.indexOf("function TimedPostColumn"), source.indexOf("function TimedPostButton"));

  assert.match(source, /function isWeekendDate\(date: Date\)/);
  assert.match(monthWeekdayHeader, /index === 0 \|\| index === 6/);
  assert.match(monthWeekdayHeader, /posts-calendar-weekday[^\n]+weekend/);
  assert.match(monthDayGrid, /isWeekendDate\(cell\.date\) \? "weekend" : ""/);
  assert.doesNotMatch(weekHeader, /posts-calendar-week-heading[^\n]+weekend/);
  assert.doesNotMatch(weekHeader, /isWeekendDate\(day\.date\) \? "weekend" : ""/);
  assert.match(weekColumns, /isWeekend=\{isWeekendDate\(day\.date\)\}/);
  assert.match(dayView, /posts-calendar-day-grid \$\{isWeekendDate\(visibleDate\) \? "weekend" : ""\}/);
  assert.match(dayView, /isWeekend=\{isWeekendDate\(visibleDate\)\}/);
  assert.match(timedColumn, /isWeekend: boolean/);
  assert.match(timedColumn, /posts-calendar-time-column \$\{isWeekend \? "weekend" : ""\}/);
  assert.match(source, /--calendar-weekend-surface:/);
  assert.match(source, /\.dark \.posts-calendar-fullheight\{--calendar-weekend-surface:/);
  assert.match(source, /\.posts-calendar-day\.weekend\{[^}]*background:var\(--calendar-weekend-surface\)/);
  assert.doesNotMatch(source, /\.posts-calendar-week-heading\.weekend/);
  assert.match(source, /\.posts-calendar-time-column\.weekend\{[^}]*background-color:var\(--calendar-weekend-surface\)/);
  assert.match(source, /\.posts-calendar-day-grid\.weekend \.posts-calendar-day-column-wrap\{[^}]*background:var\(--calendar-weekend-surface\)/);
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
  const popover = source.slice(
    source.indexOf("function EventPopover"),
    source.indexOf("function CalendarPostDetailGrid"),
  );

  assert.match(source, /selectedPostTarget/);
  assert.match(source, /handleSelectPost/);
  assert.match(source, /getElementRect/);
  assert.match(source, /getBoundingClientRect\(\)/);
  assert.match(source, /anchorRect=\{selectedPostTarget\.anchorRect\}/);
  assert.match(source, /boundsRect=\{selectedPostTarget\.boundsRect\}/);
  assert.match(popover, /boundsRect/);
  assert.match(popover, /getBoundedCalendarPopoverPlacement/);
  assert.match(popover, /"--popover-available-height": `\$\{placement\.availableHeight\}px`/);
  assert.doesNotMatch(popover, /height: `\$\{placement\.availableHeight\}px`/);
  assert.match(popover, /className="posts-calendar-popover-content"/);
  assert.match(popover, /verticalStrategy: "anchor"/);
  assert.match(popover, /data-side=\{placement\.side\}/);
  assert.match(popover, /--popover-left/);
  assert.match(popover, /--popover-arrow-y/);
  assert.match(source, /posts-calendar-popover-open/);
  assert.match(source, /\.posts-calendar-popover\{[^}]*box-sizing:border-box/);
  assert.match(source, /\.posts-calendar-popover\{[^}]*overflow:visible/);
  assert.match(source, /\.posts-calendar-popover-content\{[^}]*max-height:min\(calc\(100dvh - 26px\),calc\(var\(--popover-available-height,calc\(100dvh - 24px\)\) - 2px\)\)/);
  assert.match(source, /\.posts-calendar-popover-content\{[^}]*overflow:auto/);
  assert.match(source, /\.posts-calendar-popover\[data-side="right"\]::before/);
  assert.doesNotMatch(source, /background:color-mix\(in srgb,var\(--overlay\) 48%,transparent\)/);
});

test("Posts calendar event buttons expose complete accessible labels", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const eventButton = source.slice(
    source.indexOf("function CalendarEventButton"),
    source.indexOf("function TimeLabels"),
  );
  const labelHelper = source.slice(
    source.indexOf("function getCalendarEventAccessibleLabel"),
    source.indexOf("function CalendarEventButton"),
  );

  assert.match(eventButton, /const accessibleLabel = getCalendarEventAccessibleLabel/);
  assert.match(eventButton, /timezone:\s*string/);
  assert.match(source, /<CalendarEventButton[\s\S]*timezone=\{timezone\}/);
  assert.match(source, /<TimedPostColumn[\s\S]*timezone=\{timezone\}/);
  assert.match(eventButton, /aria-label=\{accessibleLabel\}/);
  assert.match(eventButton, /title=\{accessibleLabel\}/);
  assert.match(labelHelper, /profile\?\.name/);
  assert.match(labelHelper, /formatPlatformLabel/);
  assert.match(labelHelper, /formatPostDateTime/);
  assert.match(labelHelper, /timezone/);
  assert.match(labelHelper, /meta\.label/);
});

test("Posts calendar create flow refreshes data after drawer success", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const handleCreated = source.slice(
    source.indexOf("const handleCreated = useCallback"),
    source.indexOf("const handleEdited = useCallback"),
  );
  const drawer = source.slice(
    source.indexOf("<CreatePostDrawer"),
    source.indexOf("</section>"),
  );

  assert.match(handleCreated, /await loadData\(\)/);
  assert.match(handleCreated, /\[loadData\]/);
  assert.match(drawer, /open=\{drawerOpen\}/);
  assert.match(drawer, /onOpenChange=\{setDrawerOpen\}/);
  assert.match(drawer, /onCreated=\{handleCreated\}/);
});

test("Posts calendar keeps empty workspace paths usable", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const profilesFilter = source.slice(
    source.indexOf('<FilterSection title="Profiles">'),
    source.indexOf('<FilterSection title="Platforms">'),
  );
  const platformsFilter = source.slice(
    source.indexOf('<FilterSection title="Platforms">'),
    source.indexOf('<FilterSection title="Status">'),
  );

  assert.match(profilesFilter, /profiles\.length === 0/);
  assert.match(profilesFilter, /No profiles found/);
  assert.match(platformsFilter, /platformOptions\.length === 0/);
  assert.match(platformsFilter, /No connected platforms/);
  assert.match(source, /renderMonthView\(\)/);
  assert.match(source, /<CreatePostDrawer/);
});

test("Posts calendar switches from details popover to edit inspector without keeping the details popover open", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const openEditPost = source.slice(
    source.indexOf("const openEditPost = useCallback"),
    source.indexOf("const renderMonthWeekdayHeader"),
  );

  assert.match(openEditPost, /if \(!selectedPostTarget\) return/);
  assert.match(openEditPost, /setEditingPostTarget\(selectedPostTarget\)/);
  assert.match(openEditPost, /setSelectedPostTarget\(null\)/);
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
  assert.match(popover, /href=\{`\/projects\/\$\{profileId\}\/posts\/list\?post=\$\{encodeURIComponent\(post\.id\)\}`\}/);
  assert.match(popover, />\s*Open in List\s*</);
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

test("Posts calendar details popover preserves platform icon rendering inside chips", async () => {
  const source = await readFile(calendarViewPath, "utf8");
  const popover = source.slice(
    source.indexOf("function EventPopover"),
    source.indexOf("function CalendarPostDetailGrid"),
  );

  assert.match(popover, /className="posts-calendar-popover-platform-chip"/);
  assert.match(popover, /<AccountDestinationIcon platform=\{platform\} size=\{14\} \/>/);
  assert.match(source, /\.posts-calendar-popover-platform-chip\{[^}]*display:inline-flex/);
  assert.doesNotMatch(source, /\.posts-calendar-popover-platforms span\{/);
});
