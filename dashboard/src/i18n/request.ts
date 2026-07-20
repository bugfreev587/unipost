import { getRequestConfig } from "next-intl/server";
import { defaultLocale, isReleasedLocale, type ReleasedLocale } from "./locales";

import enCommon from "../../messages/en/common.json";
import enMarketing from "../../messages/en/marketing.json";
import enNavigation from "../../messages/en/navigation.json";
import enPricing from "../../messages/en/pricing.json";
import esCommon from "../../messages/es/common.json";
import esMarketing from "../../messages/es/marketing.json";
import esNavigation from "../../messages/es/navigation.json";
import esPricing from "../../messages/es/pricing.json";

const catalogs = {
  en: { common: enCommon, navigation: enNavigation, marketing: enMarketing, pricing: enPricing },
  es: { common: esCommon, navigation: esNavigation, marketing: esMarketing, pricing: esPricing },
} satisfies Record<ReleasedLocale, Record<string, unknown>>;

export default getRequestConfig(async ({ requestLocale }) => {
  const requestedLocale = await requestLocale;
  const locale = isReleasedLocale(requestedLocale) ? requestedLocale : defaultLocale;

  return {
    locale,
    messages: catalogs[locale],
  };
});
