// Sprint 4 PR4 hotfix: dedicated layout for the public Connect pages
// so they're not subject to the dashboard's dark-theme root body
// background. The Connect page is end-user-facing (not the project
// owner) and needs to look clean + neutral, not match the dark
// dashboard chrome.
//
// Implementation: a fixed-position div that covers the entire
// viewport with #fafafa, sitting on top of the dashboard's dark
// body background. zIndex doesn't matter because there's nothing
// else on these routes — but the position:fixed + inset:0 makes
// the page behave like its own document.

export default function ConnectLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <div
      style={{
        position: "fixed",
        inset: 0,
        background: "#fafafa",
        color: "#111",
        overflow: "auto",
        WebkitFontSmoothing: "antialiased",
      }}
    >
      {children}
    </div>
  );
}
