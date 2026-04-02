const APP_URL = process.env.NEXT_PUBLIC_APP_URL || "https://app.unipost.dev";

export default function LandingPage() {
  return (
    <div className="flex flex-col min-h-screen">
      {/* Nav */}
      <header className="border-b border-zinc-200">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <span className="text-xl font-bold">UniPost</span>
          <div className="flex items-center gap-4">
            <a
              href={APP_URL}
              className="text-sm font-medium text-zinc-600 hover:text-zinc-900 transition-colors"
            >
              Log in
            </a>
            <a
              href={APP_URL}
              className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white hover:bg-zinc-800 transition-colors"
            >
              Get Started
            </a>
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
          <a
            href={APP_URL}
            className="inline-flex items-center justify-center rounded-md bg-zinc-900 px-8 py-3 text-base font-medium text-white hover:bg-zinc-800 transition-colors"
          >
            Get Started Free
          </a>
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

      {/* CTA */}
      <section className="bg-zinc-900 text-white">
        <div className="max-w-6xl mx-auto px-6 py-16 text-center">
          <h2 className="text-3xl font-bold mb-4">Ready to get started?</h2>
          <p className="text-lg text-zinc-400 mb-8">
            Create your free account and start integrating in minutes.
          </p>
          <a
            href={APP_URL}
            className="inline-flex items-center justify-center rounded-md bg-white px-8 py-3 text-base font-medium text-zinc-900 hover:bg-zinc-100 transition-colors"
          >
            Sign Up Free
          </a>
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
