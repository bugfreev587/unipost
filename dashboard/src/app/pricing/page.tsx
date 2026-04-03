import type { Metadata } from "next";
import { MarketingCTA } from "@/components/marketing/nav";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Pricing — UniPost",
  description: "Simple, transparent pricing. Start free, scale as you grow.",
};

const PLANS = [
  { name: "Free", price: 0, posts: "100", features: ["All 6 platforms", "Unlimited API keys", "Webhook support", "Community support"], cta: "Get Started Free" },
  { name: "$10/mo", price: 10, posts: "1,000", features: ["Everything in Free", "White Label / BYOC", "Analytics", "Priority support"] },
  { name: "$25/mo", price: 25, posts: "2,500" },
  { name: "$50/mo", price: 50, posts: "5,000", popular: true },
  { name: "$75/mo", price: 75, posts: "10,000" },
  { name: "$150/mo", price: 150, posts: "20,000" },
  { name: "$300/mo", price: 300, posts: "40,000" },
  { name: "$500/mo", price: 500, posts: "100,000" },
  { name: "$1,000/mo", price: 1000, posts: "200,000" },
];

export default function PricingPage() {
  return (
    <div className="min-h-screen bg-white">
      {/* Nav */}
      <header className="border-b border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <Link href="/" className="text-xl font-bold">UniPost</Link>
          <div className="flex items-center gap-6">
            <Link href="/docs" className="text-sm font-medium text-zinc-600 hover:text-zinc-900">Docs</Link>
            <MarketingCTA />
          </div>
        </div>
      </header>

      <div className="max-w-6xl mx-auto px-6 py-16">
        <h1 className="text-4xl font-bold text-center text-zinc-900 mb-4">
          Simple, transparent pricing
        </h1>
        <p className="text-center text-lg text-zinc-500 mb-4">
          Start free. Scale as you grow. Only pay for what you use.
        </p>
        <p className="text-center text-sm text-zinc-400 mb-12">
          All plans include all 6 platforms, unlimited API keys, and webhook support.
          <br />
          Paid plans unlock White Label / BYOC (bring your own credentials).
        </p>

        {/* Main plans grid */}
        <div className="grid md:grid-cols-4 gap-4 mb-8">
          {PLANS.slice(0, 4).map((plan) => (
            <div
              key={plan.name}
              className={`rounded-xl border p-6 ${
                plan.popular
                  ? "border-zinc-900 ring-2 ring-zinc-900"
                  : "border-zinc-200"
              }`}
            >
              {plan.popular && (
                <p className="text-xs font-semibold text-zinc-900 mb-2 uppercase tracking-wide">
                  Most Popular
                </p>
              )}
              <h3 className="font-semibold text-zinc-900 text-lg">{plan.name === "Free" ? "Free" : plan.name.replace("/mo", "")}</h3>
              <p className="text-3xl font-bold text-zinc-900 my-3">
                {plan.price === 0 ? "$0" : `$${plan.price}`}
                <span className="text-sm font-normal text-zinc-500">/mo</span>
              </p>
              <p className="text-sm text-zinc-600 mb-4">{plan.posts} posts/month</p>
              {plan.features && (
                <ul className="space-y-2 text-sm text-zinc-600">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-center gap-2">
                      <span className="text-green-600">&#10003;</span> {f}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ))}
        </div>

        {/* Extended plans */}
        <div className="border border-zinc-200 rounded-xl p-6 mb-12">
          <h3 className="font-semibold text-zinc-900 mb-4">Higher volume plans</h3>
          <div className="grid grid-cols-5 gap-4">
            {PLANS.slice(4).map((plan) => (
              <div key={plan.name} className="text-center p-3 rounded-lg border border-zinc-100">
                <p className="font-semibold text-zinc-900">{plan.name.replace("/mo", "")}</p>
                <p className="text-sm text-zinc-500">{plan.posts} posts</p>
              </div>
            ))}
          </div>
        </div>

        {/* Feature comparison */}
        <div className="max-w-3xl mx-auto mb-16">
          <h3 className="font-semibold text-zinc-900 text-xl text-center mb-6">
            What&apos;s included in every plan
          </h3>
          <div className="grid grid-cols-2 gap-3 text-sm">
            {[
              "Bluesky, LinkedIn, Instagram, Threads, TikTok, YouTube",
              "Unlimited API keys",
              "Unlimited social accounts",
              "Multi-platform posting in one API call",
              "Webhook notifications",
              "Encrypted token storage (AES-256-GCM)",
              "Soft-limit quota (never blocks your users)",
              "API usage headers (X-UniPost-Usage)",
            ].map((f) => (
              <div key={f} className="flex items-center gap-2 py-2 border-b border-zinc-100">
                <span className="text-green-600">&#10003;</span>
                <span className="text-zinc-700">{f}</span>
              </div>
            ))}
          </div>
          <div className="mt-6 p-4 bg-zinc-50 rounded-lg border border-zinc-200 text-sm text-zinc-600">
            <p><strong>Paid plans only:</strong> White Label / BYOC — use your own OAuth app credentials so the authorization page shows your app name, not UniPost&apos;s.</p>
          </div>
        </div>

        {/* Enterprise */}
        <div className="text-center border border-zinc-200 rounded-xl p-8 mb-16">
          <h3 className="text-xl font-bold text-zinc-900 mb-2">Enterprise</h3>
          <p className="text-zinc-500 mb-4">
            Custom volume, SLA, dedicated support, and priority feature requests.
          </p>
          <a
            href="mailto:support@unipost.dev"
            className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-6 py-2 text-sm font-medium text-white hover:bg-zinc-800 transition-colors"
          >
            Contact Sales
          </a>
        </div>

        {/* FAQ */}
        <div className="max-w-2xl mx-auto">
          <h3 className="text-xl font-bold text-zinc-900 text-center mb-6">FAQ</h3>
          <div className="space-y-4 text-sm">
            {[
              ["What counts as a post?", "Each platform delivery counts as one post. Posting to 3 accounts = 3 posts."],
              ["What happens if I exceed my limit?", "We use a soft-limit approach. Your posts will continue to go through — we'll never hard-block your production traffic. You'll see warnings in the API response headers and dashboard."],
              ["Can I upgrade or downgrade anytime?", "Yes. Upgrades take effect immediately. Downgrades apply at the end of your billing period."],
              ["Do I need a credit card for the free plan?", "No. The free plan requires no credit card."],
              ["What is White Label / BYOC?", "Bring Your Own Credentials. You provide your own OAuth app credentials so that when users authorize, they see your app name instead of UniPost's. Available on all paid plans."],
            ].map(([q, a]) => (
              <div key={q} className="border-b border-zinc-100 pb-4">
                <p className="font-semibold text-zinc-900">{q}</p>
                <p className="text-zinc-600 mt-1">{a}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Footer */}
      <footer className="border-t border-zinc-200 py-8">
        <div className="max-w-6xl mx-auto px-6 text-center text-sm text-zinc-500">
          &copy; {new Date().getFullYear()} UniPost. All rights reserved.
          {" · "}
          <Link href="/terms" className="hover:text-zinc-700">Terms</Link>
          {" · "}
          <Link href="/privacy" className="hover:text-zinc-700">Privacy</Link>
          {" · "}
          <Link href="/docs" className="hover:text-zinc-700">Docs</Link>
        </div>
      </footer>
    </div>
  );
}
