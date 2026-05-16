import { PlatformAnalyticsList } from "./platform-analytics-list";

export default async function AnalyticsPlatformsPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  return (
    <>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", gap: 20, marginBottom: 24 }}>
        <div>
          <div className="dt-page-title">Platform Analytics</div>
          <div className="dt-subtitle" style={{ maxWidth: 720 }}>
            Platform-specific account insights and extended metrics beyond the cross-platform Posts view.
          </div>
        </div>
      </div>

      <PlatformAnalyticsList profileId={id} />
    </>
  );
}
