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
          colorBackground: "#111111",
          colorInputBackground: "#1a1a1a",
          colorText: "#ededed",
          colorTextSecondary: "#888888",
          colorTextOnPrimaryBackground: "#000000",
          colorPrimary: "#10b981",
          colorDanger: "#ef4444",
          colorNeutral: "#ededed",
          colorInputText: "#ededed",
          borderRadius: "8px",
        },
        elements: {
          // Modal / card backgrounds
          card: "!bg-[#111111] !border !border-[#1e1e1e] !shadow-2xl",
          modalBackdrop: "!bg-black/60 !backdrop-blur-sm",
          // Nav tabs in profile
          navbar: "!bg-[#0a0a0a] !border-r !border-[#1e1e1e]",
          navbarButton: "!text-[#888] hover:!text-[#ededed] hover:!bg-[#1a1a1a]",
          "navbarButton__active": "!text-[#10b981] !bg-[#10b98115]",
          // Page content
          pageScrollBox: "!bg-[#111111]",
          page: "!bg-[#111111]",
          // Header text
          headerTitle: "!text-[#ededed]",
          headerSubtitle: "!text-[#666]",
          // Profile details
          profileSectionTitle: "!text-[#ededed]",
          profileSectionTitleText: "!text-[#ededed]",
          profileSectionContent: "!text-[#aaa]",
          profileSectionPrimaryButton: "!text-[#10b981]",
          // Form labels & inputs
          formFieldLabel: "!text-[#aaa]",
          formFieldInput: "!bg-[#1a1a1a] !border-[#242424] !text-[#ededed]",
          formButtonPrimary: "!bg-[#10b981] !text-[#000]",
          formButtonReset: "!text-[#888] hover:!text-[#ededed]",
          // Accordion / sections
          accordionTriggerButton: "!text-[#ededed]",
          accordionContent: "!text-[#aaa]",
          // Badges
          badge: "!text-[#aaa] !bg-[#1a1a1a] !border-[#242424]",
          badgePrimary: "!text-[#10b981] !bg-[#10b98115]",
          // Table rows
          tableHead: "!text-[#666]",
          // Footer
          footer: "!bg-[#0a0a0a] !border-t !border-[#1e1e1e]",
          footerAction: "!text-[#666]",
          footerActionLink: "!text-[#666]",
          // User button popover (dropdown)
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
