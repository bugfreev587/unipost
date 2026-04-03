import { MarketingNav, MarketingCTA, MarketingCTALight } from "@/components/marketing/nav";

export default function LandingPage() {
  return (
    <div className="flex flex-col min-h-screen">
      {/* Nav */}
      <header className="border-b border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <span className="text-xl font-bold">UniPost</span>
          <div className="flex items-center gap-6">
            <a href="/docs" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors">Docs</a>
            <a href="/pricing" className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors">Pricing</a>
            <MarketingNav />
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="flex-1 flex items-center">
        <div className="max-w-6xl mx-auto px-6 py-24 text-center">
          <h1 className="text-5xl font-bold tracking-tight mb-6 max-w-3xl mx-auto leading-tight text-zinc-900">
            Ship social media integrations in hours, not weeks.
          </h1>
          <p className="text-xl text-zinc-500 max-w-2xl mx-auto mb-10">
            UniPost gives your app a unified API to post, schedule, and analyze
            across all major social platforms.
          </p>
          <MarketingCTA />
        </div>
      </section>

      {/* Code Demo */}
      <section className="bg-zinc-50 border-t border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <h2 className="text-3xl font-bold text-center mb-10 text-zinc-900">
            Three lines of code
          </h2>
          <div className="max-w-2xl mx-auto">
            <pre className="bg-white border border-zinc-200 rounded-lg p-6 text-sm font-mono overflow-x-auto text-zinc-800">
              <code>{`const post = await unipost.posts.create({
  caption: "Hello from UniPost!",
  social_accounts: ["sa_instagram_123", "sa_linkedin_456"]
});`}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="border-t border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <h2 className="text-3xl font-bold text-center mb-12 text-zinc-900">
            Everything you need
          </h2>
          <div className="grid md:grid-cols-3 gap-8">
            <div className="text-center">
              <h3 className="font-semibold mb-2 text-zinc-900">Unified API</h3>
              <p className="text-sm text-zinc-500">
                One API to post across Instagram, TikTok, LinkedIn, YouTube, X,
                Pinterest, and Bluesky.
              </p>
            </div>
            <div className="text-center">
              <h3 className="font-semibold mb-2 text-zinc-900">
                Unlimited Accounts
              </h3>
              <p className="text-sm text-zinc-500">
                Connect as many social accounts as you need. No per-account
                fees.
              </p>
            </div>
            <div className="text-center">
              <h3 className="font-semibold mb-2 text-zinc-900">
                Developer First
              </h3>
              <p className="text-sm text-zinc-500">
                Built for developers with clean APIs, SDKs, and comprehensive
                documentation.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* Pricing */}
      <section className="bg-zinc-50 border-t border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <h2 className="text-3xl font-bold text-center mb-4 text-zinc-900">
            Simple, transparent pricing
          </h2>
          <p className="text-center text-zinc-500 mb-12">
            Start free. Scale as you grow.
          </p>
          <div className="grid md:grid-cols-4 gap-4 max-w-4xl mx-auto">
            {[
              { name: "Free", price: "$0", posts: "100 posts/mo" },
              { name: "Starter", price: "$10", posts: "1,000 posts/mo" },
              { name: "Growth", price: "$50", posts: "5,000 posts/mo", popular: true },
              { name: "Scale", price: "$150", posts: "20,000 posts/mo" },
            ].map((plan) => (
              <div
                key={plan.name}
                className={`rounded-lg border p-6 text-center ${
                  plan.popular
                    ? "border-zinc-900 bg-white ring-1 ring-zinc-900"
                    : "border-zinc-200 bg-white"
                }`}
              >
                {plan.popular && (
                  <p className="text-xs font-semibold text-zinc-900 mb-2 uppercase">
                    Most Popular
                  </p>
                )}
                <h3 className="font-semibold text-zinc-900">{plan.name}</h3>
                <p className="text-3xl font-bold text-zinc-900 my-2">
                  {plan.price}
                  <span className="text-sm font-normal text-zinc-500">/mo</span>
                </p>
                <p className="text-sm text-zinc-500">{plan.posts}</p>
              </div>
            ))}
          </div>
          <p className="text-center text-sm text-zinc-500 mt-6">
            All plans include unlimited accounts, all platforms, and API access.
          </p>
        </div>
      </section>

      {/* CTA */}
      <section className="bg-zinc-900 text-white">
        <div className="max-w-6xl mx-auto px-6 py-16 text-center">
          <h2 className="text-3xl font-bold mb-4">Ready to get started?</h2>
          <p className="text-lg text-zinc-400 mb-8">
            Create your free account and start integrating in minutes.
          </p>
          <MarketingCTALight />
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-zinc-200 py-8">
        <div className="max-w-6xl mx-auto px-6 text-center text-sm text-zinc-500">
          &copy; {new Date().getFullYear()} UniPost. All rights reserved.
        </div>
      </footer>
    </div>
  );
}
