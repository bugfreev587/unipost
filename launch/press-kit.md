# Press Kit — AgentPost & UniPost

For anyone covering AgentPost (or UniPost) at launch. Last updated:
Sprint 4 PR10.

---

## What is AgentPost?

**AgentPost** is an open-source CLI that turns a one-line update
into platform-perfect social media posts and publishes them
everywhere — Twitter, LinkedIn, Bluesky, Threads, Instagram — in
one command.

It's designed for indie developers and ship-shippers who care about
posting their work but find the friction of writing the same news
three different ways too high. AgentPost uses Claude (Anthropic) to
draft per-platform posts, renders them as previews in your terminal,
and publishes only after you confirm.

License: MIT. Repo: <https://github.com/unipost-dev/agentpost>.
Install: `npm install -g @unipost/agentpost`.

## What is UniPost?

**UniPost** is a paid SaaS API for multi-platform social publishing.
It handles the OAuth flows, token refresh, rate limits, and
per-platform API quirks across Twitter, LinkedIn, Bluesky, Threads,
Instagram, TikTok, and YouTube. Developers integrate UniPost into
their products to give end users one-click connections + reliable
publishing.

UniPost is the engine AgentPost runs on. AgentPost is the open-
source frontend; UniPost is the rails.

URL: <https://unipost.dev>. Docs: <https://app.unipost.dev/docs>.

## The split

AgentPost (free, OSS, MIT) is the developer-facing CLI.
UniPost (paid SaaS API) is the infrastructure underneath.

**Why split it?** Because the CLI should be free, hackable, and
yours. The infrastructure that handles 47 different OAuth flows
and rate limits should be a managed service that nobody has to
maintain. This is the same playbook PostHog uses with ClickHouse
or Resend uses with Stripe — the OSS layer is what the community
sees and forks; the paid layer is what makes it sustainable.

---

## Logos & assets

**TODO**: drop SVG / PNG files in `assets/` directory before launch:

- `assets/agentpost-logo-black.svg` — wordmark on white
- `assets/agentpost-logo-white.svg` — wordmark on dark
- `assets/agentpost-icon.svg` — square icon for app dock / favicon
- `assets/unipost-logo-black.svg` — UniPost wordmark on white
- `assets/unipost-logo-white.svg` — UniPost wordmark on dark
- `assets/screenshot-cli.png` — terminal screenshot of `agentpost "..."`
- `assets/screenshot-preview.png` — Ink preview cards close-up
- `assets/screenshot-dashboard.png` — UniPost dashboard with managed users view
- `assets/demo.gif` — the asciinema/vhs recording used on Show HN + X

All logo files: white-label friendly (no UniPost in the AgentPost
files; no AgentPost in the UniPost files). Both are usable
independently in coverage.

---

## 100-word blurb

> AgentPost is an open-source CLI that solves a small but real
> friction: writing the same news three different ways for Twitter,
> LinkedIn, and Bluesky. You type `agentpost "shipped webhooks
> today 🎉"`, Claude drafts per-platform posts, you see previews in
> your terminal, and one keypress publishes everywhere. It's MIT
> licensed and the prompt is one file you can fork. AgentPost runs
> on UniPost — a paid API that handles the OAuth and rate limits
> under the hood — but the CLI is free, and the philosophy is that
> indie developers shouldn't pay for the part that touches their
> own machines.

(96 words.)

---

## 300-word blurb

> AgentPost is an open-source command-line tool for indie developers
> who ship things and want to post about it without the friction of
> writing the same news three different ways for three different
> platforms. You type one line — `agentpost "shipped webhooks today"` —
> and the CLI uses Claude (Anthropic) to draft platform-specific posts:
> punchy on Twitter, longer and more reflective on LinkedIn, casual
> and lowercase on Bluesky. Previews render in the terminal with
> per-platform character counts, and one keypress publishes them all.
>
> It's the OSS frontend for UniPost, a paid SaaS API for multi-
> platform publishing that handles the OAuth flows, token refresh,
> rate limits, and per-platform API quirks across seven networks.
> The CLI is free and MIT licensed; the API is paid. Developers can
> use either or both — UniPost has its own dashboard, MCP server,
> and REST API for non-CLI use cases.
>
> The most important file in AgentPost isn't code — it's the prompt
> at `src/lib/prompt.ts`. The prompt encodes platform-specific style
> guidance (no buzzword openers, no hashtag spam, never invent facts
> not in the input) and a small set of hand-curated few-shot examples.
> Improving the prompt directly improves output quality across every
> user, and PRs are welcome.
>
> AgentPost was built by Xiaobo Yu, a solo developer who got tired
> of either skipping social posts entirely or spending 15 minutes
> rewriting the same announcement four times. The tool is free to
> install (`npm install -g @unipost/agentpost`) and free to use with
> any UniPost API key. UniPost itself is free up to 100 posts/month;
> paid tiers start at $10/month.

(298 words.)

---

## Founder bio

> Xiaobo Yu is the founder of UniPost and AgentPost. Previously a
> software engineer at ByteDance. Currently building developer
> tools that respect the developer's time and the audience's
> intelligence — no buzzwords, no growth hacks, no engagement loops.
> Reach him at xiaobo@unipost.dev or @yuxiaobohit on most networks.

(Adjust bio per Xiaobo's preference. Drop the ByteDance reference
if you'd rather lead with UniPost.)

---

## FAQ for press

**Q: Is AgentPost free?**
A: Yes. The CLI is MIT licensed and free to install + use forever.
You'll need an Anthropic API key (typical cost: ~$0.001 per generated
draft) and a UniPost account (free tier covers 100 posts/month).

**Q: Why does AgentPost depend on UniPost?**
A: AgentPost is intentionally a thin client. The hard parts of
multi-platform publishing — OAuth, token refresh, rate limits, per-
platform quirks — are handled by UniPost so the CLI can stay small
(~500 lines of TypeScript) and hackable. You could fork AgentPost
to point at a different publishing backend, but the easier path is
to use the one that's already designed for it.

**Q: Does AgentPost store my Anthropic API key?**
A: Locally, in `~/.agentpost/config.json` with mode 0600 (only the
current user can read it). The key never leaves your machine —
AgentPost calls Anthropic's API directly from your terminal.

**Q: What about OpenAI / Gemini / Llama?**
A: Sprint 4 ships Claude only because it's what the prompt was
tuned against. OpenAI and Gemini adapters are on the v0.2 roadmap.

**Q: How is this different from Buffer / Hootsuite / Typefully?**
A: AgentPost is a CLI, not a SaaS dashboard. It's also AI-native
from the start — the headline UX is "describe the news in your own
words, let Claude draft per-platform copy" rather than "open the
compose box and write the same caption N times." It's designed for
people who already have a terminal open all day.

**Q: How is this different from Zapier / IFTTT / n8n auto-posters?**
A: Those are workflow automation tools that fire on triggers. AgentPost
is interactive — every post is a deliberate command from the user, with
a preview before publishing. There are no scheduled jobs, no webhook
triggers, no autoposting from RSS. It's a CLI you run when you want to
post, not a robot you deploy and forget about.

**Q: Is the prompt actually any good?**
A: Honest answer: v0.1's prompt is reasonable but not great. The most
useful contribution to the project right now is improving it. Fork
`src/lib/prompt.ts`, run a few drafts against your own writing style,
PR back what works.

---

## Contact

- **Press inquiries**: hi@unipost.dev
- **Founder**: xiaobo@unipost.dev
- **Twitter**: @yuxiaobohit (founder), @unipostdev (brand)
- **GitHub**: github.com/unipost-dev
