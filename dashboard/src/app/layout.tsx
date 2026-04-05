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
          colorBackground: "#0f0f0f",
          colorInputBackground: "#141414",
          colorText: "#ededed",
          colorTextSecondary: "#666666",
          colorPrimary: "#10b981",
          colorDanger: "#ef4444",
          borderRadius: "6px",
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
