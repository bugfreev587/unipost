import { redirect } from "next/navigation";

export default function DraftsPage() {
  redirect("/docs/api/posts/drafts/create");
}
