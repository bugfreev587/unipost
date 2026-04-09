# LinkedIn Launch Post

**Status: DRAFT.** ~300 words. Professional tone, but not buzzword-heavy.
LinkedIn rewards posts that look like they were written by a human
talking to other humans.

Post from `Xiaobo Yu` personal account (NOT the company page — personal
posts get 5-10x the reach on LinkedIn for indie launches).

---

## Body

```
I built a thing.

It's called AgentPost, and it solves a problem I've had for two years:
I ship features, and then I don't post about them.

The friction is real. Writing the same news three different ways for
Twitter, LinkedIn, and Bluesky takes 15 minutes. Cross-posting the same
caption everywhere lands flat on every platform. So most weeks, I'd
just... not post. Features would ship. Customers would notice eventually
or not at all.

AgentPost is the third option.

You type one line — `agentpost "shipped webhooks today"` — and it
asks Claude to rewrite that line per platform. Twitter gets a punchy
version. LinkedIn gets a longer, more reflective one. Bluesky gets
the casual lowercase version that lands with that crowd. You see all
three previews in your terminal, character counts and all, and you
press Enter to publish them everywhere at once.

I built it because I was tired of the friction. I'm releasing it
because I think a lot of indie developers feel the same way.

A few decisions worth flagging:

→ It's MIT licensed and on npm. `npm install -g @unipost/agentpost`.
→ The prompt is one file you can fork and tune. I want it to get
  better than what I shipped today.
→ It runs on UniPost (the publishing API I built last year). The
  CLI is the open-source frontend; UniPost handles the OAuth, rate
  limits, and 47 different platform quirks under the hood.
→ It is NOT a scheduler, NOT an analytics tool, NOT a growth hack
  thing. It is one command to publish, and it always shows you a
  preview before anything goes live.

GitHub: github.com/unipost-dev/agentpost

If you ship things and you've been quietly skipping the announce-it
step, this is for you.

Honest feedback welcome — I'm reading every comment.
```

— 312 words. Slightly over the 300 target but the rhythm wants it.

---

## Notes

- Opens with "I built a thing." — bare, anti-buzzword, signals
  non-corporate. Specifically chosen to clash with the "Excited to
  announce" openers it's quietly mocking.
- The middle section ("AgentPost is the third option") is the
  pivot — first we set up the problem, then we name the solution.
  LinkedIn rewards this structure (problem → solution → details).
- Bullet list at the end gives the post visual rhythm + makes it
  scannable. LinkedIn hides anything below the "...more" cutoff
  unless you click expand, so the bullets need to be the lower
  half (post-cutoff) where engagement signals matter most.
- "If you ship things..." is the call-to-action in the second-person
  voice. Direct, specific.
- "Honest feedback welcome" gives commenters permission to be
  critical, which paradoxically generates more positive engagement.

## Image attachment

Attach the same demo gif used on Show HN + the X thread. LinkedIn
auto-converts gifs to looping videos which actually display better
than static images.

## Posting time

**Tuesday April 28 2026, 9:00 AM PT** — same time as Show HN.
LinkedIn's algorithm is most active 9-10am PT on weekdays anyway.

## Engagement playbook

- Reply to every comment within the first hour. LinkedIn's algorithm
  heavily weights "author replies" and they boost the post for the
  next few hours.
- Don't reply with "Thanks!" — reply with a follow-up question or
  a specific acknowledgment. The algorithm rewards reply length and
  reply diversity.
- If someone asks "does it work for X?" answer concretely with a
  yes/no, not a "great question."
