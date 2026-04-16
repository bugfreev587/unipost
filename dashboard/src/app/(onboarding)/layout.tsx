// Route group `(onboarding)` — doesn't create a URL segment, just
// a shared full-screen layout for /welcome that's distinct from the
// dashboard shell (no sidebar, no project context).

export default function OnboardingLayout({ children }: { children: React.ReactNode }) {
  return (
    <div style={{
      minHeight: "100vh",
      background: "var(--background, #080808)",
      display: "flex",
      alignItems: "center",
      justifyContent: "center",
    }}>
      {children}
    </div>
  );
}
