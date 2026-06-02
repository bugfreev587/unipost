import { redirect } from "next/navigation";

export default async function LegacyWhiteLabelPlatformPage({
  params,
}: {
  params: Promise<{ platform: string }>;
}) {
  const { platform } = await params;
  redirect(`/docs/platform-credentials/${platform}`);
}
