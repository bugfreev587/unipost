import { notFound } from "next/navigation";
import { setRequestLocale } from "next-intl/server";
import { isReleasedLocale, releasedLocales } from "@/i18n/locales";

export function generateStaticParams() {
  return releasedLocales.map((locale) => ({ locale }));
}

export default async function LocaleLayout({
  children,
  params,
}: Readonly<{
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}>) {
  const { locale } = await params;
  if (!isReleasedLocale(locale)) notFound();

  setRequestLocale(locale);
  return children;
}
