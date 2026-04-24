import { redirect } from "next/navigation";

export default function ProfilesIndexPage() {
  redirect("/docs/api/profiles/list");
}
