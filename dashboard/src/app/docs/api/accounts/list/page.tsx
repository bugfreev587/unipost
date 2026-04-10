import type { Metadata } from "next";
import { ListAccountsContent } from "./content";

export const metadata: Metadata = {
  title: "GET /v1/social-accounts — List Connected Accounts | UniPost API Docs",
  description: "List all connected social media accounts in your workspace. Filter by platform and external_user_id. Returns account status, connection type, and platform metadata.",
  keywords: ["unipost api list accounts", "social media api accounts endpoint", "get connected social accounts api"],
};

export default function ListAccountsPage() {
  return <ListAccountsContent />;
}
