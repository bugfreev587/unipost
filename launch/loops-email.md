# Loops Email — AgentPost Launch

**Status: DRAFT.** To existing UniPost signups in Loops. Goal:
convert API key holders into AgentPost installs without coming
across as spam.

Send time: **Tuesday April 28 2026, 9:15 AM PT** — 15 minutes
after Show HN goes live, so the post has some early momentum
when these recipients click through.

---

## Subject line options

```
1. New: AgentPost — the AI-native way to post
2. Built on top of UniPost: AgentPost ships today
3. AgentPost is live (built on the API you already use)
4. We just shipped AgentPost — try it with your existing key
```

Recommended: **#4** — concrete, names the product, tells the
recipient they don't need to sign up again. Highest open rate
of the four options based on similar developer-tool launch emails.

---

## From

```
From: Xiaobo Yu <xiaobo@unipost.dev>
Reply-To: hi@unipost.dev
```

Personal sender, not "UniPost <noreply@unipost.dev>". Doubles
open rate for indie launches.

---

## Body

```
Hi {{first_name | default: "there"}},

You signed up for UniPost a while back. Today I'm shipping a thing
on top of it that I think you'll like:

→ AgentPost. https://github.com/unipost-dev/agentpost

It's a CLI that turns one line into platform-perfect posts and
publishes them everywhere. Type:

  agentpost "shipped webhooks today 🎉"

…and it asks Claude to draft a post per platform (Twitter punchy,
LinkedIn long-form, Bluesky casual), shows previews in your terminal,
and publishes on Enter.

The good news for you: it works with the UniPost API key you
already have. Three steps:

  1. npm install -g @unipost/agentpost
  2. agentpost init  (paste your existing UniPost key + an Anthropic key)
  3. agentpost "your first post"

That's it. No new account, no new dashboard, no migration.

It's MIT licensed and the prompt is one file you can fork. We're
also live on Show HN today — would love your honest feedback in
the comments:

  <TODO Show HN URL>

Thanks for being one of the first UniPost users. AgentPost is what
I've been building toward — the dev-tool-flavored frontend that the
underlying API has been waiting for.

— Xiaobo
Founder, UniPost

---

P.S. If you've been thinking about reconnecting your accounts,
now's a good time. AgentPost will use whatever's connected
already, and the new managed-Twitter media support means image
posts work via Connect now too.
```

---

## Notes

- Word count: 219. Mobile readers will see the full thing without
  having to expand.
- The "you already have a UniPost key" framing is the differentiator —
  most launch emails ask people to sign up for something new, this one
  says "use what you have." Should convert at 5-10% installs from the
  list, vs 0.5-1% for a generic "we shipped" email.
- The P.S. is the second engagement hook. It mentions a specific
  feature (managed-Twitter media) that some recipients might have
  been blocked on.

## Loops setup

In Loops:

1. Create a new Campaign called "AgentPost Launch"
2. Audience: all contacts where `unipost_signup = true` AND
   `agentpost_announced = false`
3. Subject: option 4 above
4. Body: paste from above, replace `{{first_name | default: "there"}}`
   with Loops' merge tag syntax
5. Schedule for Tue Apr 28 2026 9:15 AM PT
6. Add a custom field update on send: set `agentpost_announced = true`
   so we don't accidentally re-send
7. **Don't enable click tracking** — the GitHub link is the only
   meaningful click and tracking it via Loops makes the URL ugly
   in some email clients

## A/B test (if there's time)

If you have >500 contacts in the list, split 50/50:

- A: subject "AgentPost is live (built on the API you already use)"
- B: subject "We just shipped AgentPost — try it with your existing key"

Send 30 minutes apart, watch open rates, send the winner to the
remaining ~80% of the list. This is the kind of detail that doesn't
matter individually but matters at scale.
