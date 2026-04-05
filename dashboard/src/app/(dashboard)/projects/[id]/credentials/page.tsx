"use client";

import { useParams } from "next/navigation";
import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function CredentialsPage() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();

  useEffect(() => {
    router.replace(`/projects/${id}/settings`);
  }, [id, router]);

  return (
    <div className="text-[13px] text-[#525252]">
      Redirecting to settings...
    </div>
  );
}
