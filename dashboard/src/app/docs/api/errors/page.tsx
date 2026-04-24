import { redirect } from "next/navigation";

export default function ErrorsPage() {
  redirect("/docs/api/profiles/list");
}
