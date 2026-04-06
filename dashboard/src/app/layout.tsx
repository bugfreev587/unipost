import type { Metadata } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { DM_Sans } from "next/font/google";
import { ClerkProvider } from "@clerk/nextjs";
import "./globals.css";

const dmSans = DM_Sans({
  variable: "--font-dm-sans",
  subsets: ["latin"],
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
  title: "UniPost",
  description: "Unified Social Media API for developers",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <ClerkProvider
      signInFallbackRedirectUrl="/"
      signUpFallbackRedirectUrl="/"
      afterSignOutUrl={process.env.NEXT_PUBLIC_LANDING_URL || "https://unipost.dev"}
      appearance={{
        variables: {
          colorBackground: "#ffffff",
          colorInputBackground: "#f5f5f5",
          colorText: "#1a1a1a",
          colorTextSecondary: "#666666",
          colorTextOnPrimaryBackground: "#ffffff",
          colorPrimary: "#10b981",
          colorDanger: "#ef4444",
          colorNeutral: "#1a1a1a",
          colorInputText: "#1a1a1a",
          borderRadius: "8px",
        },
        elements: {
          // Modal / card — white background, dark text
          card: "!bg-white !shadow-2xl",
          modalBackdrop: "!bg-black/60 !backdrop-blur-sm",
          // Nav tabs in profile modal
          navbar: "!bg-[#f8f8f8] !border-r !border-[#e5e5e5]",
          navbarButton: "!text-[#666] hover:!text-[#1a1a1a] hover:!bg-[#f0f0f0]",
          "navbarButton__active": "!text-[#10b981] !bg-[#10b98110]",
          // Page content
          pageScrollBox: "!bg-white",
          page: "!bg-white",
          // Header text
          headerTitle: "!text-[#1a1a1a]",
          headerSubtitle: "!text-[#888]",
          // Profile details
          profileSectionTitle: "!text-[#1a1a1a]",
          profileSectionTitleText: "!text-[#1a1a1a]",
          profileSectionContent: "!text-[#444]",
          profileSectionPrimaryButton: "!text-[#10b981]",
          // Form labels & inputs
          formFieldLabel: "!text-[#444]",
          formFieldInput: "!bg-[#f5f5f5] !border-[#e0e0e0] !text-[#1a1a1a]",
          formButtonPrimary: "!bg-[#10b981] !text-white",
          formButtonReset: "!text-[#888] hover:!text-[#1a1a1a]",
          // Accordion / sections
          accordionTriggerButton: "!text-[#1a1a1a]",
          accordionContent: "!text-[#444]",
          // Badges
          badge: "!text-[#666] !bg-[#f0f0f0] !border-[#e0e0e0]",
          badgePrimary: "!text-[#10b981] !bg-[#10b98110]",
          // Table rows
          tableHead: "!text-[#888]",
          // Footer
          footer: "!bg-[#f8f8f8] !border-t !border-[#e5e5e5]",
          footerAction: "!text-[#888]",
          footerActionLink: "!text-[#888]",
          // User button popover (small dropdown) — keep dark
          userButtonPopoverCard: "!bg-[#141414] !border !border-[#242424] !w-[240px]",
          userButtonPopoverMain: "!w-[240px]",
          userButtonPopoverActions: "!bg-transparent",
          userButtonPopoverActionButton: "!text-[#ededed] hover:!bg-[#1a1a1a]",
          userButtonPopoverActionButtonText: "!text-[#ededed]",
          userButtonPopoverActionButtonIcon: "!text-[#888]",
          userButtonPopoverFooter: "!hidden",
          userPreviewMainIdentifier: "!text-[#f0f0f0]",
          userPreviewSecondaryIdentifier: "!text-[#666]",
        },
      }}
    >
      <html
        lang="en"
        className={`dark ${dmSans.variable} ${geistSans.variable} ${geistMono.variable} h-full antialiased`}
        style={{ fontFamily: "var(--font-dm-sans), system-ui, sans-serif" }}
      >
        <body className="min-h-full flex flex-col" style={{ background: "#080808", color: "#ededed" }}>
          {children}
        </body>
      </html>
    </ClerkProvider>
  );
}
