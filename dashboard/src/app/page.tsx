import { SignInButton, SignUpButton } from "@clerk/nextjs";
import { Button } from "@/components/ui/button";


export default function LandingPage() {
  return (
    <div className="flex flex-col min-h-screen">
      {/* Nav */}
      <header className="border-b">
        <div className="max-w-6xl mx-auto px-6 h-16 flex items-center justify-between">
          <span className="text-xl font-bold">UniPost</span>
          <div className="flex items-center gap-4">
            <SignInButton mode="redirect" fallbackRedirectUrl="/dashboard">
              <Button variant="ghost" size="sm">
                Log in
              </Button>
            </SignInButton>
            <SignUpButton mode="redirect" fallbackRedirectUrl="/dashboard">
              <Button size="sm">Get Started</Button>
            </SignUpButton>
          </div>
        </div>
      </header>

      {/* Hero */}
      <section className="flex-1 flex items-center">
        <div className="max-w-6xl mx-auto px-6 py-24 text-center">
          <h1 className="text-5xl font-bold tracking-tight mb-6 max-w-3xl mx-auto leading-tight">
            Ship social media integrations in hours, not weeks.
          </h1>
          <p className="text-xl text-muted-foreground max-w-2xl mx-auto mb-10">
            UniPost gives your app a unified API to post, schedule, and analyze
            across all major social platforms.
          </p>
          <SignUpButton mode="redirect" fallbackRedirectUrl="/dashboard">
            <Button size="lg" className="text-base px-8">
              Get Started Free
            </Button>
          </SignUpButton>
        </div>
      </section>

      {/* Code Demo */}
      <section className="bg-muted/50 border-t">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <h2 className="text-3xl font-bold text-center mb-10">
            Three lines of code
          </h2>
          <div className="max-w-2xl mx-auto">
            <pre className="bg-card border rounded-lg p-6 text-sm font-mono overflow-x-auto">
              <code>{`const post = await unipost.posts.create({
  caption: "Hello from UniPost!",
  social_accounts: ["sa_instagram_123", "sa_linkedin_456"]
});`}</code>
            </pre>
          </div>
        </div>
      </section>

      {/* Features */}
      <section className="border-t">
        <div className="max-w-6xl mx-auto px-6 py-20">
          <h2 className="text-3xl font-bold text-center mb-12">
            Everything you need
          </h2>
          <div className="grid md:grid-cols-3 gap-8">
            <div className="text-center">
              <h3 className="font-semibold mb-2">Unified API</h3>
              <p className="text-sm text-muted-foreground">
                One API to post across Instagram, TikTok, LinkedIn, YouTube, X,
                Pinterest, and Bluesky.
              </p>
            </div>
            <div className="text-center">
              <h3 className="font-semibold mb-2">Unlimited Accounts</h3>
              <p className="text-sm text-muted-foreground">
                Connect as many social accounts as you need. No per-account
                fees.
              </p>
            </div>
            <div className="text-center">
              <h3 className="font-semibold mb-2">Developer First</h3>
              <p className="text-sm text-muted-foreground">
                Built for developers with clean APIs, SDKs, and comprehensive
                documentation.
              </p>
            </div>
          </div>
        </div>
      </section>

      {/* CTA */}
      <section className="bg-primary text-primary-foreground">
        <div className="max-w-6xl mx-auto px-6 py-16 text-center">
          <h2 className="text-3xl font-bold mb-4">Ready to get started?</h2>
          <p className="text-lg opacity-80 mb-8">
            Create your free account and start integrating in minutes.
          </p>
          <SignUpButton mode="redirect" fallbackRedirectUrl="/dashboard">
            <Button
              size="lg"
              variant="secondary"
              className="text-base px-8"
            >
              Sign Up Free
            </Button>
          </SignUpButton>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t py-8">
        <div className="max-w-6xl mx-auto px-6 text-center text-sm text-muted-foreground">
          &copy; {new Date().getFullYear()} UniPost. All rights reserved.
        </div>
      </footer>
    </div>
  );
}
