import type { Metadata } from "next";
import { AgentPostContent } from "./content";

export const metadata: Metadata = {
  title: "AgentPost — AI Social Media Posting Tool | UniPost",
  description:
    "Describe what you shipped and AI posts it to Twitter, LinkedIn, Instagram, Threads, TikTok, YouTube, and Bluesky. Free. No install required.",
  keywords: [
    "ai social media posting tool",
    "agentpost",
    "ai post to multiple platforms",
    "social media ai tool free",
    "post to twitter linkedin instagram ai",
  ],
};

export default function AgentPostPage() {
  return <AgentPostContent />;
}
