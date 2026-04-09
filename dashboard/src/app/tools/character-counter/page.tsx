import type { Metadata } from "next";
import { CharacterCounterContent } from "./content";

export const metadata: Metadata = {
  title: "Social Media Character Counter — All 7 Platforms | UniPost",
  description:
    "Check your post length for Twitter (280), LinkedIn (3,000), Instagram (2,200), Threads (500), TikTok (2,200), YouTube (5,000), and Bluesky (300) at once. Free, no sign-up.",
  keywords: [
    "twitter character counter",
    "social media character counter",
    "instagram caption length checker",
    "linkedin post character limit",
    "bluesky character limit",
    "threads character limit",
    "social media post length checker",
  ],
};

export default function CharacterCounterPage() {
  return <CharacterCounterContent />;
}
