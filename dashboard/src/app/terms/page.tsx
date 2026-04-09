import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms of Service — UniPost",
};

export default function TermsPage() {
  return (
    <div className="min-h-screen bg-white">
      <div className="max-w-3xl mx-auto px-6 py-16">
        {/*
          UniPost app icon. TikTok's developer review compares the icon
          shown on the linked Privacy / Terms pages against the one
          uploaded to the dev portal — if they don't match, production
          access is rejected. The file under /public/unipost-logo.png is
          byte-identical to the asset uploaded to TikTok.
        */}
        <a href="/" aria-label="UniPost home" className="inline-block mb-8">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/unipost-logo.png"
            alt="UniPost"
            width={96}
            height={96}
            className="block rounded-2xl"
          />
        </a>
        <h1 className="text-3xl font-bold text-zinc-900 mb-2">
          Terms of Service
        </h1>
        <p className="text-sm text-zinc-500 mb-10">Last updated: April 2, 2026</p>

        <div className="space-y-8 text-zinc-700 text-[15px] leading-relaxed">
          <p>
            These Terms of Service (&quot;Terms&quot;) are a legally binding agreement between you
            (&quot;you&quot;, &quot;Customer&quot;) and UniPost (&quot;UniPost&quot;, &quot;we&quot;, &quot;us&quot;, &quot;our&quot;) governing your
            access to and use of the UniPost website, dashboard, API, and related services
            (collectively, the &quot;Service&quot;).
          </p>
          <p>If you do not agree to these Terms, you may not use the Service.</p>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">1. Agreement to Terms</h2>
            <p>
              By accessing or using the Service, you acknowledge that you have read, understood,
              and agree to be bound by these Terms and any policies referenced herein, including
              our <a href="/privacy" className="text-blue-600 hover:text-blue-800 underline">Privacy Policy</a>.
            </p>
            <p className="mt-2">
              We may update these Terms from time to time. We will update the &quot;Last updated&quot; date.
              Your continued use of the Service after changes become effective constitutes acceptance
              of the updated Terms.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">2. Eligibility</h2>
            <p>
              You must be at least 18 years old and have the legal capacity to enter into these Terms.
              If you use the Service on behalf of an entity, you represent that you have authority to
              bind that entity to these Terms.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">3. The Service</h2>
            <p>
              UniPost is a unified social media API that enables developers to integrate posting,
              scheduling, and analytics across multiple social platforms through a single API.
            </p>
            <p className="mt-2">Core capabilities include:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Connecting social media accounts (X, Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube)</li>
              <li>Publishing content to multiple platforms simultaneously via API</li>
              <li>Managing API keys for programmatic access</li>
              <li>Webhook notifications for post status updates</li>
              <li>OAuth-based social account authorization</li>
            </ul>
            <p className="mt-2">
              UniPost acts as a pass-through service. Content is published to third-party platforms
              using their respective APIs. UniPost does not host, moderate, or control content published
              to those platforms.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">4. Account Registration and Security</h2>
            <p>
              You may be required to create an account (e.g., via our authentication provider Clerk).
              You agree to provide accurate, current, and complete information and to keep it updated.
            </p>
            <p className="mt-2">You are responsible for:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Maintaining the confidentiality of your credentials and API keys</li>
              <li>All activity occurring under your account or API keys</li>
              <li>Any content published through the Service using your credentials</li>
            </ul>
            <p className="mt-2">
              You must promptly notify us of any suspected unauthorized access or security breach.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">5. Subscription Plans, Fees, and Payment</h2>
            <p>
              UniPost offers Free and paid plans. Plan features, limits, and pricing are described on
              our website and may change over time.
            </p>
            <h3 className="text-lg font-medium text-zinc-900 mt-4 mb-2">5.1 Payment Processor</h3>
            <p>
              Paid subscriptions are billed through Stripe. By subscribing, you authorize UniPost
              (through Stripe) to charge your selected payment method on a recurring basis until you
              cancel or your subscription ends.
            </p>
            <h3 className="text-lg font-medium text-zinc-900 mt-4 mb-2">5.2 Usage Limits</h3>
            <p>
              Each plan includes a monthly post limit. We use a soft-block approach: exceeding your
              limit will not immediately interrupt your service, but continued overuse may require
              upgrading your plan.
            </p>
            <h3 className="text-lg font-medium text-zinc-900 mt-4 mb-2">5.3 Cancellation</h3>
            <p>
              You may cancel a paid subscription at any time through your account settings. Unless
              required by law, we do not provide refunds for partial billing periods.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">6. Acceptable Use</h2>
            <p>You agree not to use the Service to:</p>
            <ol className="list-decimal pl-6 mt-2 space-y-1">
              <li>Violate any law, regulation, or third-party rights</li>
              <li>Circumvent, disable, or interfere with security or access controls</li>
              <li>Publish spam, misleading, or harmful content to social platforms</li>
              <li>Reverse engineer, decompile, or attempt to extract source code</li>
              <li>Abuse API rate limits or engage in automated activity that degrades service integrity</li>
              <li>Evade plan limits, usage restrictions, or billing controls</li>
              <li>Use the Service to compete with UniPost using non-public information</li>
              <li>Store or transmit malware through the Service</li>
            </ol>
            <p className="mt-2">
              We may suspend or terminate access if we believe you are violating these Terms.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">7. API Keys and Access Tokens</h2>
            <p>
              API keys grant programmatic access to the Service. You are solely responsible for
              securing your API keys. Treat them as passwords — do not share them publicly, commit
              them to version control, or embed them in client-side code.
            </p>
            <p className="mt-2">
              Social media access tokens obtained through OAuth are encrypted at rest using
              AES-256-GCM. We do not store plaintext social media passwords or app passwords.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">8. Third-Party Services</h2>
            <p>
              The Service integrates with third-party services including but not limited to: Clerk
              (authentication), Stripe (payments), Bluesky, LinkedIn, Meta/Instagram, Threads,
              TikTok, and YouTube. Your use of third-party services is subject to their respective
              terms and privacy policies. UniPost is not responsible for third-party services&apos;
              availability, security, or practices.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">9. Intellectual Property</h2>
            <p>
              The Service (including software, design, and documentation) is owned by UniPost and
              protected by intellectual property laws. Subject to these Terms, UniPost grants you a
              limited, non-exclusive, non-transferable license to access and use the Service.
            </p>
            <p className="mt-2">
              You retain ownership of content you publish through the Service.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">10. Disclaimer of Warranties</h2>
            <p>
              TO THE MAXIMUM EXTENT PERMITTED BY LAW, THE SERVICE IS PROVIDED &quot;AS IS&quot; AND &quot;AS
              AVAILABLE,&quot; WITHOUT WARRANTIES OF ANY KIND, WHETHER EXPRESS, IMPLIED, OR STATUTORY,
              INCLUDING IMPLIED WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE,
              AND NON-INFRINGEMENT.
            </p>
            <p className="mt-2">
              We do not warrant that the Service will be uninterrupted, secure, or error-free, or
              that content will be successfully published to any third-party platform.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">11. Limitation of Liability</h2>
            <p>
              TO THE MAXIMUM EXTENT PERMITTED BY LAW, UNIPOST WILL NOT BE LIABLE FOR ANY INDIRECT,
              INCIDENTAL, SPECIAL, CONSEQUENTIAL, EXEMPLARY, OR PUNITIVE DAMAGES, INCLUDING LOST
              PROFITS, LOST REVENUE, LOSS OF DATA, OR GOODWILL.
            </p>
            <p className="mt-2">
              OUR TOTAL LIABILITY FOR ALL CLAIMS WILL NOT EXCEED THE AMOUNT YOU PAID TO UNIPOST IN
              THE TWELVE (12) MONTHS BEFORE THE EVENT GIVING RISE TO THE CLAIM.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">12. Indemnification</h2>
            <p>
              You agree to defend, indemnify, and hold harmless UniPost from any claims, damages,
              or expenses arising from your use of the Service, your violation of these Terms, or
              content you publish through the Service.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">13. Termination</h2>
            <p>
              We may suspend or terminate your access at any time if we believe you violated these
              Terms. Upon termination, your right to use the Service stops immediately, and API keys
              will be revoked.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">14. Governing Law</h2>
            <p>
              These Terms are governed by the laws of the State of California. Any dispute will be
              brought in the state or federal courts located in San Francisco County, California.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">15. Contact</h2>
            <p>
              Questions about these Terms:{" "}
              <a href="mailto:support@unipost.dev" className="text-blue-600 hover:text-blue-800 underline">
                support@unipost.dev
              </a>
            </p>
          </section>
        </div>
      </div>
    </div>
  );
}
