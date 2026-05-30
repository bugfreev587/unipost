"use client";

import { useAuth } from "@clerk/nextjs";
import { ChevronLeft, ChevronRight, List, Plus, X } from "lucide-react";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState, type CSSProperties } from "react";
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
  bucketPostByLocalDay,
  getPostStatusGroup,
  getProfileCalendarColor,
  shouldShowPostForStatusFilter,
  type CalendarStatusFilter,
  type CalendarStatusGroup,
} from "./calendar-model";

const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

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
  const profileId = params.id;
  const { getToken } = useAuth();
  const workspaceId = useWorkspaceId();
  const [posts, setPosts] = useState<SocialPost[]>([]);
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [accounts, setAccounts] = useState<SocialAccount[]>([]);
  const [selectedProfileIds, setSelectedProfileIds] = useState<Set<string>>(new Set());
  const [selectedPlatforms, setSelectedPlatforms] = useState<Set<string>>(new Set());
  const [statusFilter, setStatusFilter] = useState<CalendarStatusFilter>("all");
  const [visibleMonth, setVisibleMonth] = useState(() => startOfMonth(new Date()));
  const [filtersInitialized, setFiltersInitialized] = useState(false);
  const [selectedPostId, setSelectedPostId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const timezone = useMemo(() => Intl.DateTimeFormat().resolvedOptions().timeZone || "Local time", []);

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

  const monthCells = useMemo(() => buildMonthGrid(visibleMonth), [visibleMonth]);
  const postsByDay = useMemo(() => {
    const byDay = new Map<string, SocialPost[]>();
    for (const cell of monthCells) byDay.set(cell.dateKey, []);
    for (const post of filteredPosts) {
      const dateKey = bucketPostByLocalDay(post);
      if (!dateKey || !byDay.has(dateKey)) continue;
      byDay.get(dateKey)?.push(post);
    }
    for (const dayPosts of byDay.values()) {
      dayPosts.sort((a, b) => getPostTimeValue(a) - getPostTimeValue(b));
    }
    return byDay;
  }, [filteredPosts, monthCells]);

  const selectedPost = useMemo(
    () => posts.find((post) => post.id === selectedPostId) || null,
    [posts, selectedPostId],
  );

  const monthTitle = useMemo(
    () => visibleMonth.toLocaleDateString("en-US", { month: "long", year: "numeric" }),
    [visibleMonth],
  );

  const visiblePostCount = filteredPosts.filter((post) => {
    const key = bucketPostByLocalDay(post);
    return Boolean(key && postsByDay.has(key));
  }).length;

  const handleCreated = useCallback(async () => {
    await loadData();
  }, [loadData]);

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
            <h1>{monthTitle}</h1>
            <span>{loading ? "Loading posts" : `${visiblePostCount} posts in view`}</span>
          </div>

          <div className="posts-calendar-toolbar" aria-label="Calendar controls">
            <div className="posts-calendar-segment" aria-label="Calendar view mode">
              <button type="button" disabled>Day</button>
              <button type="button" disabled>Week</button>
              <button type="button" className="active">Month</button>
            </div>

            <div className="posts-calendar-month-nav">
              <button type="button" aria-label="Previous month" onClick={() => setVisibleMonth((date) => addMonths(date, -1))}>
                <ChevronLeft size={16} />
              </button>
              <button type="button" onClick={() => setVisibleMonth(startOfMonth(new Date()))}>Today</button>
              <button type="button" aria-label="Next month" onClick={() => setVisibleMonth((date) => addMonths(date, 1))}>
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

        <div className="posts-calendar-grid" aria-label={`${monthTitle} posts`}>
          {WEEKDAYS.map((weekday) => (
            <div key={weekday} className="posts-calendar-weekday">{weekday}</div>
          ))}
          {monthCells.map((cell) => {
            const dayPosts = postsByDay.get(cell.dateKey) || [];
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
                      onClick={() => setSelectedPostId(post.id)}
                    />
                  ))}
                  {dayPosts.length > visibleDayPosts.length ? (
                    <button type="button" className="posts-calendar-more" onClick={() => setSelectedPostId(dayPosts[4].id)}>
                      + {dayPosts.length - visibleDayPosts.length} more
                    </button>
                  ) : null}
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {selectedPost ? (
        <EventPopover
          post={selectedPost}
          profile={getPrimaryProfile(selectedPost, profilesById)}
          color={getPostColor(selectedPost, profilesById, profileColors)}
          timezone={timezone}
          profileId={profileId}
          onClose={() => setSelectedPostId(null)}
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
  onClick: () => void;
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

function EventPopover({
  post,
  profile,
  color,
  timezone,
  profileId,
  onClose,
}: {
  post: SocialPost;
  profile: Profile | null;
  color: string;
  timezone: string;
  profileId: string;
  onClose: () => void;
}) {
  const status = getPostStatusGroup(post);
  const meta = STATUS_META[status];
  const platforms = getPostPlatforms(post);
  return (
    <div className="posts-calendar-popover-layer" role="presentation" onMouseDown={onClose}>
      <article
        className="posts-calendar-popover"
        role="dialog"
        aria-modal="true"
        aria-label="Post details"
        onMouseDown={(event) => event.stopPropagation()}
        style={{ "--event-color": color } as CSSProperties}
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
  const source =
    post.status === "scheduled" && post.scheduled_at
      ? post.scheduled_at
      : post.published_at || post.created_at;
  if (!source) return null;
  const date = new Date(source);
  return Number.isNaN(date.getTime()) ? null : date;
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
.posts-calendar-main{min-width:0;display:flex;flex-direction:column;background:var(--surface)}
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
.posts-calendar-grid{flex:1;min-height:640px;display:grid;grid-template-columns:repeat(7,minmax(0,1fr));grid-template-rows:34px repeat(6,minmax(104px,1fr));background:var(--dborder);gap:1px}
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
.posts-calendar-popover-layer{position:fixed;inset:0;background:color-mix(in srgb,var(--overlay) 48%,transparent);display:flex;align-items:flex-start;justify-content:flex-end;padding:96px 30px 30px;z-index:90}
.posts-calendar-popover{width:min(420px,calc(100vw - 36px));background:var(--surface-raised);border:1px solid var(--dborder);border-radius:16px;box-shadow:0 24px 70px color-mix(in srgb,var(--shadow-color) 160%,transparent);padding:16px}
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
@media (max-width: 680px){.posts-calendar-fullheight{border-radius:12px}.posts-calendar-sidebar{grid-template-columns:1fr}.posts-calendar-title-block h1{font-size:26px}.posts-calendar-segment button{min-width:54px}.posts-calendar-grid{overflow-x:auto;grid-template-columns:repeat(7,minmax(132px,1fr))}.posts-calendar-popover-layer{padding:78px 12px 16px}.posts-calendar-popover{width:100%}}
`;
