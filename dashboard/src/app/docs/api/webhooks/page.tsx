import type { Metadata } from "next";
import { WebhooksContent } from "./content";

export const metadata: Metadata = {
  title: "Webhooks — Event Reference | UniPost API Docs",
  description: "Complete reference for UniPost webhook events: post.published, post.failed, account.connected, and more. Includes payload shapes, signature verification, and setup examples.",
  keywords: ["unipost webhooks", "social media api webhooks", "post published webhook", "webhook signature verification"],
};

export default function WebhooksPage() {
  return <WebhooksContent />;
}
