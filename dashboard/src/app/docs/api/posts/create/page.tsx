import type { Metadata } from "next";
import { CreatePostContent } from "./content";

export const metadata: Metadata = {
  title: "POST /v1/social-posts — Create Social Post | UniPost API Docs",
  description: "Publish content to Instagram, LinkedIn, Twitter, TikTok, YouTube, Bluesky, and Threads with a single API call. Full reference with code examples in JavaScript, Python, Go, and cURL.",
  keywords: [
    "unipost api post",
    "social media api post endpoint",
    "publish to multiple platforms api",
    "post to instagram api javascript",
    "unified social media api reference",
  ],
};

export default function CreatePostPage() {
  return <CreatePostContent />;
}
