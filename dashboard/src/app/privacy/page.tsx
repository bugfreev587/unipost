import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy Policy — UniPost",
};

export default function PrivacyPage() {
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
          Privacy Policy
        </h1>
        <p className="text-sm text-zinc-500 mb-10">Last updated: April 13, 2026</p>

        <div className="space-y-8 text-zinc-700 text-[15px] leading-relaxed">
          <p>
            This Privacy Policy explains how UniPost (&quot;UniPost,&quot; &quot;we,&quot; &quot;us,&quot; or &quot;our&quot;) collects,
            uses, discloses, and safeguards information when you access or use the UniPost platform,
            website, API, and related services (collectively, the &quot;Service&quot;).
          </p>
          <p>
            If you do not agree with the practices described in this Privacy Policy, please do not
            use the Service.
          </p>

          <section className="bg-zinc-50 border border-zinc-200 rounded-lg p-5">
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">Summary of Key Points</h2>
            <ul className="list-disc pl-6 space-y-1">
              <li>We collect only the data necessary to operate a social media API platform.</li>
              <li>Social media access tokens are encrypted at rest using AES-256-GCM.</li>
              <li>We do not store social media passwords — only encrypted OAuth tokens.</li>
              <li>Payment information is handled entirely by Stripe.</li>
              <li>We do not sell personal data and do not use advertising trackers.</li>
              <li>Content is published to third-party platforms on your behalf; we do not host published content.</li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">1. Information We Collect</h2>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">1.1 Information You Provide</h3>
            <p>When you register for or use the Service, we may collect:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Name and email address (via Clerk authentication)</li>
              <li>Account preferences and project settings</li>
              <li>Communications you send to us</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">1.2 Social Media Account Data</h3>
            <p>When you connect social media accounts, we collect and store:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>OAuth access tokens and refresh tokens (encrypted with AES-256-GCM)</li>
              <li>Platform account identifiers (e.g., Bluesky DID, LinkedIn person URN)</li>
              <li>Account display name and avatar URL</li>
              <li>Platform-specific metadata needed for posting</li>
            </ul>
            <p className="mt-2">
              We never store social media passwords or app passwords. For Bluesky, the app password
              is used only to create a session and is immediately discarded.
            </p>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">1.3 API Usage Data</h3>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>API request logs (method, path, status, duration)</li>
              <li>Post content and media URLs you submit via the API</li>
              <li>Post results and platform response data</li>
              <li>Monthly usage counts for billing purposes</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">1.4 Billing Information</h3>
            <p>All payment processing is handled by Stripe. We store only:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Stripe customer ID and subscription ID</li>
              <li>Subscription plan and billing status</li>
            </ul>
            <p className="mt-2">
              We do not store credit card numbers or payment instrument details.
              See{" "}
              <a href="https://stripe.com/privacy" target="_blank" rel="noopener noreferrer" className="text-blue-600 hover:text-blue-800 underline">
                Stripe&apos;s Privacy Policy
              </a>.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">2. How We Use Your Information</h2>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Provide, operate, and maintain the Service</li>
              <li>Authenticate users and manage accounts</li>
              <li>Connect and manage social media accounts on your behalf</li>
              <li>Publish content to social platforms via their APIs</li>
              <li>Process subscriptions and enforce usage limits</li>
              <li>Deliver webhook notifications</li>
              <li>Refresh expiring OAuth tokens automatically</li>
              <li>Send service-related notifications</li>
              <li>Detect and prevent fraud, abuse, or security incidents</li>
              <li>Comply with legal obligations</li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">3. Sharing of Information</h2>
            <p>We share information only in the following limited circumstances:</p>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">Service Providers</h3>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li><strong>Clerk</strong> — authentication and user identity</li>
              <li><strong>Stripe</strong> — subscription billing and payments</li>
              <li><strong>Railway</strong> — API server hosting</li>
              <li><strong>Vercel</strong> — dashboard and website hosting</li>
              <li><strong>Social platforms</strong> — content is published to platforms you connect (X, Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube)</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">Legal Requirements</h3>
            <p>
              We may disclose information if required by law, subpoena, court order, or governmental request.
            </p>

            <p className="mt-4 font-medium">
              We do not sell personal information and do not share data with advertisers.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">4. Cookies and Tracking</h2>
            <p>
              We use essential cookies only, primarily for authentication and session management via Clerk.
            </p>
            <p className="mt-2">We do not use:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Advertising cookies</li>
              <li>Cross-site tracking</li>
              <li>Behavioral profiling</li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">5. YouTube API Services</h2>
            <p>
              UniPost uses YouTube API Services to enable users to publish videos to their own
              YouTube channels through our unified publishing API.
            </p>
            <p className="mt-2">
              By connecting your YouTube account to UniPost, you agree to be bound by the{" "}
              <a
                href="https://www.youtube.com/t/terms"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:text-blue-800 underline"
              >
                YouTube Terms of Service
              </a>.
            </p>
            <p className="mt-2">
              Our use and transfer of information received from Google APIs adheres to the{" "}
              <a
                href="https://developers.google.com/terms/api-services-user-data-policy"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:text-blue-800 underline"
              >
                Google API Services User Data Policy
              </a>
              , including the Limited Use requirements.
            </p>
            <p className="mt-2">
              Please also review the{" "}
              <a
                href="http://www.google.com/policies/privacy"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:text-blue-800 underline"
              >
                Google Privacy Policy
              </a>.
            </p>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">5.1 Data We Access from YouTube</h3>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Channel name, channel ID, and channel thumbnail to confirm the correct account connection</li>
              <li>Video upload status and related metadata needed to confirm successful publishing</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">5.2 Data We Store</h3>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>OAuth access and refresh tokens, encrypted at rest</li>
              <li>YouTube channel identifiers linked to your UniPost workspace</li>
              <li>Metadata for videos published through UniPost</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">5.3 How We Use YouTube Data</h3>
            <p>
              We use YouTube data solely to connect your account, publish videos you instruct us to
              publish, and confirm delivery status inside UniPost.
            </p>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">5.4 What We Do Not Do with YouTube Data</h3>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>We do not use YouTube data for advertising</li>
              <li>We do not share YouTube data with third parties except as required to provide the publishing service</li>
              <li>We do not use YouTube data to train machine learning models</li>
            </ul>

            <h3 className="text-lg font-medium text-zinc-800 mt-4 mb-2">5.5 Revoking Access</h3>
            <p>
              You can revoke UniPost&apos;s access to your YouTube account at any time by visiting{" "}
              <a
                href="https://security.google.com/settings/security/permissions"
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-600 hover:text-blue-800 underline"
              >
                Google Security Settings
              </a>
              , or by disconnecting the account from your UniPost dashboard.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">6. Data Retention</h2>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Account and project data: retained while your account is active</li>
              <li>Social media tokens: retained while the account is connected; deleted on disconnect</li>
              <li>Post records and results: retained while your account is active</li>
              <li>API request logs: retained for up to 90 days</li>
            </ul>
            <p className="mt-2">
              Upon account deletion, personal data and tokens are removed within 30 days, unless
              retention is required by law.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">7. Your Rights</h2>
            <p>Depending on your jurisdiction, you may have the right to:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Access your personal information</li>
              <li>Correct inaccurate information</li>
              <li>Request deletion of your personal data</li>
              <li>Export your data</li>
              <li>Disconnect social media accounts at any time</li>
            </ul>
            <p className="mt-2">
              Requests may be submitted to{" "}
              <a href="mailto:support@unipost.dev" className="text-blue-600 hover:text-blue-800 underline">
                support@unipost.dev
              </a>.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">8. Security</h2>
            <p>We implement industry-standard safeguards, including:</p>
            <ul className="list-disc pl-6 mt-2 space-y-1">
              <li>Encrypted data transmission (HTTPS)</li>
              <li>AES-256-GCM encryption for social media tokens at rest</li>
              <li>SHA-256 hashing for API keys (plaintext never stored)</li>
              <li>Secure authentication via Clerk</li>
              <li>Payment data handled exclusively by Stripe (PCI-compliant)</li>
              <li>Structured JSON logging for security monitoring</li>
            </ul>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">9. Children&apos;s Privacy</h2>
            <p>
              The Service is not intended for individuals under 16 years of age. We do not knowingly
              collect personal information from children.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">10. International Data Transfers</h2>
            <p>
              UniPost is hosted in the United States. By using the Service, you acknowledge that your
              data will be processed and stored in the United States.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">11. Changes to This Policy</h2>
            <p>
              We may update this Privacy Policy periodically. Material changes will be posted on
              this page with an updated &quot;Last updated&quot; date.
            </p>
          </section>

          <section>
            <h2 className="text-xl font-semibold text-zinc-900 mb-3">12. Contact</h2>
            <p>
              Email:{" "}
              <a href="mailto:support@unipost.dev" className="text-blue-600 hover:text-blue-800 underline">
                support@unipost.dev
              </a>
            </p>
            <p>
              Website:{" "}
              <a href="https://unipost.dev" className="text-blue-600 hover:text-blue-800 underline">
                https://unipost.dev
              </a>
            </p>
          </section>
        </div>
      </div>
    </div>
  );
}
