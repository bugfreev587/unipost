import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { DM_Sans, Fira_Code } from "next/font/google";
import { ClerkProvider } from "@clerk/nextjs";
import { ThemeProvider } from "@/components/theme-provider";
import { SiteFooterGate } from "@/components/marketing/site-footer";
import "./globals.css";

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
});

const firaCode = Fira_Code({
  variable: "--font-fira-code",
  subsets: ["latin"],
  weight: ["400", "500"],
});

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  metadataBase: new URL(process.env.NEXT_PUBLIC_BASE_URL || "https://unipost.dev"),
  title: "UniPost",
  description: "Unified Social Media API for developers",
  icons: {
    icon: [
      { url: "/brand/unipost-favicon-32.png", sizes: "32x32", type: "image/png" },
      { url: "/brand/unipost-icon-64.png", sizes: "64x64", type: "image/png" },
      { url: "/brand/unipost-icon-128.png", sizes: "128x128", type: "image/png" },
      { url: "/brand/unipost-icon-dark.png", sizes: "512x512", type: "image/png" },
    ],
    shortcut: "/brand/unipost-favicon-32.png",
    apple: "/brand/unipost-icon-128.png",
  },
};

const themeInitScript = `
(() => {
  try {
    const storageKey = "unipost-theme";
    const storedTheme = localStorage.getItem(storageKey) || "system";
    const prefersDark = window.matchMedia("(prefers-color-scheme: dark)").matches;
    const resolvedTheme = storedTheme === "system" ? (prefersDark ? "dark" : "light") : storedTheme;
    const root = document.documentElement;
    root.classList.toggle("dark", resolvedTheme === "dark");
    root.classList.toggle("light", resolvedTheme === "light");
    root.style.colorScheme = resolvedTheme;
    root.dataset.theme = resolvedTheme;
  } catch (_) {}
})();
`;

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <ClerkProvider
      signInForceRedirectUrl="/"
      signUpForceRedirectUrl="/"
      afterSignOutUrl={process.env.NEXT_PUBLIC_LANDING_URL || "https://unipost.dev"}
      appearance={{
        variables: {
          colorBackground: "var(--clerk-bg)",
          colorInputBackground: "var(--clerk-muted-bg)",
          colorText: "var(--clerk-text)",
          colorTextSecondary: "var(--clerk-text-secondary)",
          colorTextOnPrimaryBackground: "var(--primary-foreground)",
          colorPrimary: "var(--clerk-primary)",
          colorDanger: "var(--clerk-danger)",
          colorNeutral: "var(--clerk-neutral)",
          colorInputText: "var(--clerk-text)",
          borderRadius: "8px",
        },
        elements: {
          // Modal / card
          card: "!bg-[var(--clerk-card-bg)] !text-[var(--clerk-text)] !border !border-[var(--clerk-border)] !shadow-2xl",
          modalBackdrop: "!bg-[var(--clerk-overlay)] !backdrop-blur-sm",
          // Nav tabs in profile modal
          navbar: "!bg-[var(--clerk-muted-bg)] !border-r !border-[var(--clerk-border)]",
          navbarButton: "!text-[var(--clerk-text-secondary)] hover:!text-[var(--clerk-text)] hover:!bg-[var(--clerk-muted-bg)]",
          "navbarButton__active": "!text-[var(--clerk-primary)] !bg-[var(--accent-dim)]",
          // Page content
          pageScrollBox: "!bg-[var(--clerk-card-bg)]",
          page: "!bg-[var(--clerk-card-bg)]",
          // Header text
          headerTitle: "!text-[var(--clerk-text)]",
          headerSubtitle: "!text-[var(--clerk-text-secondary)]",
          // Profile details
          profileSectionTitle: "!text-[var(--clerk-text)]",
          profileSectionTitleText: "!text-[var(--clerk-text)]",
          profileSectionContent: "!text-[var(--clerk-text-secondary)]",
          profileSectionPrimaryButton: "!text-[var(--clerk-primary)]",
          // Form labels & inputs
          formFieldLabel: "!text-[var(--clerk-text-secondary)]",
          formFieldInput: "!bg-[var(--clerk-muted-bg)] !border-[var(--clerk-border)] !text-[var(--clerk-text)]",
          formButtonPrimary: "!bg-[var(--clerk-primary)] !text-[var(--primary-foreground)]",
          formButtonReset: "!text-[var(--clerk-text-secondary)] hover:!text-[var(--clerk-text)]",
          // Accordion / sections
          accordionTriggerButton: "!text-[var(--clerk-text)]",
          accordionContent: "!text-[var(--clerk-text-secondary)]",
          // Badges
          badge: "!text-[var(--clerk-text-secondary)] !bg-[var(--clerk-muted-bg)] !border-[var(--clerk-border)]",
          badgePrimary: "!text-[var(--clerk-primary)] !bg-[var(--accent-dim)]",
          // Table rows
          tableHead: "!text-[var(--clerk-text-secondary)]",
          // Footer
          footer: "!bg-[var(--clerk-muted-bg)] !border-t !border-[var(--clerk-border)]",
          footerAction: "!text-[var(--clerk-text-secondary)]",
          footerActionLink: "!text-[var(--clerk-text-secondary)]",
          // User button popover
          userButtonPopoverCard: "!bg-[var(--surface-raised)] !border !border-[var(--border)] !w-[240px]",
          userButtonPopoverMain: "!w-[240px]",
          userButtonPopoverActions: "!bg-transparent",
          userButtonPopoverActionButton: "!text-[var(--text)] hover:!bg-[var(--surface2)]",
          userButtonPopoverActionButtonText: "!text-[var(--text)]",
          userButtonPopoverActionButtonIcon: "!text-[var(--text-muted)]",
          userButtonPopoverFooter: "!hidden",
          userPreviewMainIdentifier: "!text-[var(--text)]",
          userPreviewSecondaryIdentifier: "!text-[var(--text-muted)]",
        },
      }}
    >
      <html
        lang="en"
        suppressHydrationWarning
        className={`${dmSans.variable} ${firaCode.variable} ${geistSans.variable} ${geistMono.variable} h-full antialiased`}
        style={{ fontFamily: "var(--font-dm-sans), system-ui, sans-serif" }}
      >
        <head>
          <script dangerouslySetInnerHTML={{ __html: themeInitScript }} />
        </head>
        <body className="min-h-full flex flex-col bg-[var(--app-bg)] text-[var(--text)]">
          <ThemeProvider>
            {children}
            <SiteFooterGate />
          </ThemeProvider>
        </body>
      </html>
    </ClerkProvider>
  );
}
