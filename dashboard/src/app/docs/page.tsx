import { permanentRedirect } from "next/navigation";

export default function DocsHomePage() {
  permanentRedirect("/docs/quickstart");
}
