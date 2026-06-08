import { Suspense } from "react";
import Script from "next/script";
import { GoogleAnalyticsPageView } from "@/components/marketing/google-analytics-page-view";

const GA_MEASUREMENT_ID_PATTERN = /^G-[A-Z0-9]+$/;

type GoogleAnalyticsProps = {
  measurementId?: string;
};

export function GoogleAnalytics({
  measurementId = process.env.NEXT_PUBLIC_GA_MEASUREMENT_ID,
}: GoogleAnalyticsProps = {}) {
  const gaMeasurementId = measurementId?.trim();

  if (!gaMeasurementId || !GA_MEASUREMENT_ID_PATTERN.test(gaMeasurementId)) {
    return null;
  }

  const encodedMeasurementId = encodeURIComponent(gaMeasurementId);

  return (
    <>
      <Script
        id="google-analytics-loader"
        src={`https://www.googletagmanager.com/gtag/js?id=${encodedMeasurementId}`}
        strategy="afterInteractive"
      />
      <Script id="google-analytics-init" strategy="afterInteractive">
        {`
          window.dataLayer = window.dataLayer || [];
          function gtag(){window.dataLayer.push(arguments);}
          window.gtag = gtag;
          gtag("js", new Date());
          gtag("config", "${gaMeasurementId}", { send_page_view: false });
        `}
      </Script>
      <Suspense fallback={null}>
        <GoogleAnalyticsPageView measurementId={gaMeasurementId} />
      </Suspense>
    </>
  );
}
