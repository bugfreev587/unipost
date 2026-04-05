"use client";

import { UserProfile } from "@clerk/nextjs";

export default function AccountPage() {
  return (
    <div>
      <div style={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", marginBottom: 24 }}>
        <div>
          <div style={{ fontSize: 18, fontWeight: 600, letterSpacing: -0.4, color: "var(--dtext)" }}>Account</div>
          <div style={{ fontSize: 12.5, color: "var(--dmuted)", marginTop: 3 }}>Manage your personal information and security settings.</div>
        </div>
      </div>
      <UserProfile
        routing="hash"
        appearance={{
          elements: {
            rootBox: "w-full",
            card: "!bg-transparent !shadow-none !border-0 !w-full !max-w-full",
            navbar: "!hidden",
            pageScrollBox: "!bg-transparent !p-0",
            page: "!bg-transparent",
          },
        }}
      />
    </div>
  );
}
