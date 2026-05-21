"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useAuth } from "@clerk/nextjs";
import { ArrowRight, BarChart3, Camera, CheckCircle2, Clock, FileText, MessageCircle, ThumbsUp, Video } from "lucide-react";
import { PlatformIcon } from "@/components/platform-icons";
import { getMe } from "@/lib/api";
import { FEATURE_FLAG_KEYS } from "@/lib/feature-flags";
import { useFeatureFlags } from "@/lib/use-feature-flags";

export function PlatformAnalyticsList({ profileId }: { profileId: string }) {
  const { getToken } = useAuth();
  const { flags, loading } = useFeatureFlags();
  const [isAdmin, setIsAdmin] = useState(false);
  const tiktokEnabled = flags[FEATURE_FLAG_KEYS.tiktokAnalyticsScopes];
  const facebookEnabled = isAdmin;

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const token = await getToken();
        if (!token) return;
        const res = await getMe(token);
        if (!cancelled) setIsAdmin(!!res.data.is_admin);
      } catch {
        if (!cancelled) setIsAdmin(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [getToken]);

  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(260px, 1fr))", gap: 14 }}>
      <Link
        href={`/projects/${profileId}/analytics/platforms/instagram`}
        style={{
          display: "block",
          textDecoration: "none",
          color: "inherit",
          border: "1px solid var(--dborder)",
          borderRadius: 8,
          background: "var(--surface1)",
          padding: 16,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 18 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="instagram" /></div>
            <div>
              <div style={{ color: "var(--dtext)", fontWeight: 700 }}>Instagram</div>
              <div style={{ color: "var(--dmuted)", fontSize: 12 }}>Business profile, media, insights</div>
            </div>
          </div>
          <ArrowRight style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
        </div>
        <div style={{ display: "grid", gap: 9, fontSize: 13 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <CheckCircle2 style={{ width: 14, height: 14, color: "var(--success)" }} />
            instagram_business_basic / manage_insights
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <Camera style={{ width: 14, height: 14 }} />
            Recent media with reach, likes, comments, shares, saves
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <Clock style={{ width: 14, height: 14 }} />
            Live data from connected Business accounts
          </div>
        </div>
      </Link>

      <Link
        href={`/projects/${profileId}/analytics/platforms/threads`}
        style={{
          display: "block",
          textDecoration: "none",
          color: "inherit",
          border: "1px solid var(--dborder)",
          borderRadius: 8,
          background: "var(--surface1)",
          padding: 16,
        }}
      >
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 18 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <div className="platform-icon-wrap"><PlatformIcon platform="threads" /></div>
            <div>
              <div style={{ color: "var(--dtext)", fontWeight: 700 }}>Threads</div>
              <div style={{ color: "var(--dmuted)", fontSize: 12 }}>Profile, posts, account insights</div>
            </div>
          </div>
          <ArrowRight style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
        </div>
        <div style={{ display: "grid", gap: 9, fontSize: 13 }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <CheckCircle2 style={{ width: 14, height: 14, color: "var(--success)" }} />
            threads_basic / threads_manage_insights
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <MessageCircle style={{ width: 14, height: 14 }} />
            Views, likes, replies, reposts, and quotes
          </div>
          <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
            <Clock style={{ width: 14, height: 14 }} />
            Live data from connected Threads profiles
          </div>
        </div>
      </Link>

      {!loading && tiktokEnabled ? (
        <Link
          href={`/projects/${profileId}/analytics/platforms/tiktok`}
          style={{
            display: "block",
            textDecoration: "none",
            color: "inherit",
            border: "1px solid var(--dborder)",
            borderRadius: 8,
            background: "var(--surface1)",
            padding: 16,
          }}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 18 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <div className="platform-icon-wrap"><PlatformIcon platform="tiktok" /></div>
              <div>
                <div style={{ color: "var(--dtext)", fontWeight: 700 }}>TikTok</div>
                <div style={{ color: "var(--dmuted)", fontSize: 12 }}>Profile, stats, videos</div>
              </div>
            </div>
            <ArrowRight style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
          </div>
          <div style={{ display: "grid", gap: 9, fontSize: 13 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <CheckCircle2 style={{ width: 14, height: 14, color: "var(--success)" }} />
              user.info.profile / user.info.stats / video.list
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <Video style={{ width: 14, height: 14 }} />
              Public video inventory and post-level metrics
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <Clock style={{ width: 14, height: 14 }} />
              Live data after account reconnect
            </div>
          </div>
        </Link>
      ) : null}

      {!loading && facebookEnabled ? (
        <Link
          href={`/projects/${profileId}/analytics/platforms/facebook`}
          style={{
            display: "block",
            textDecoration: "none",
            color: "inherit",
            border: "1px solid var(--dborder)",
            borderRadius: 8,
            background: "var(--surface1)",
            padding: 16,
          }}
        >
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12, marginBottom: 18 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <div className="platform-icon-wrap"><PlatformIcon platform="facebook" /></div>
              <div>
                <div style={{ color: "var(--dtext)", fontWeight: 700 }}>Facebook Page</div>
                <div style={{ color: "var(--dmuted)", fontSize: 12 }}>Page profile, posts, engagement</div>
              </div>
            </div>
            <ArrowRight style={{ width: 16, height: 16, color: "var(--dmuted2)" }} />
          </div>
          <div style={{ display: "grid", gap: 9, fontSize: 13 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <CheckCircle2 style={{ width: 14, height: 14, color: "var(--success)" }} />
              pages_read_engagement / read_insights
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <FileText style={{ width: 14, height: 14 }} />
              Published Page posts with message, media, and permalink
            </div>
            <div style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--dmuted)" }}>
              <ThumbsUp style={{ width: 14, height: 14 }} />
              Likes, comments, shares, clicks, and Page Insights
            </div>
          </div>
        </Link>
      ) : null}

      <div style={{ border: "1px dashed var(--dborder2)", borderRadius: 8, background: "var(--surface1)", padding: 16, color: "var(--dmuted)" }}>
        <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
          <BarChart3 style={{ width: 18, height: 18 }} />
          <div style={{ color: "var(--dtext)", fontWeight: 700 }}>More platforms</div>
        </div>
        <div style={{ fontSize: 13, lineHeight: 1.55 }}>
          YouTube channel stats, Instagram account insights, and X account metrics can use this same drilldown pattern later.
        </div>
      </div>
    </div>
  );
}
