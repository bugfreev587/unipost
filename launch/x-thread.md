# X / Twitter Launch Thread

**Status: DRAFT.** 5 tweets. Goal: drive traffic to the GitHub repo
and the Show HN post in the first 30 minutes after launch.

Post all 5 from `@yuxiaobohit` (the founder account, not a brand
account). Personal voice converts better than brand voice on X.

---

## Tweet 1 — hook + visual

```
just shipped agentpost — it's a CLI that turns one line into platform-perfect posts and publishes them everywhere

`agentpost "shipped webhooks today 🎉"` → claude drafts per-platform posts → preview → enter → done

mit license, free, github link below 👇
```

Attach: **the asciinema gif of the CLI in action.** This is the
post that drives the impressions; the gif is what makes people stop
scrolling.

— 256 chars. Under the 280 cap with room for one more emoji if it
needs energy.

---

## Tweet 2 — why

```
why?

i kept skipping social posts after shipping features. writing the same news three different ways for twitter, linkedin and bluesky was friction.

so i'd cross-post the same caption everywhere and it'd land flat on every platform

now it takes 5 seconds
```

— 261 chars.

---

## Tweet 3 — how it works (technical)

```
under the hood:

1. CLI calls UniPost (the publishing API i built last year) for your connected accounts + per-platform limits
2. claude rewrites your one-liner per platform with a hand-tuned prompt
3. ink TUI shows previews with character counts
4. enter publishes to all
```

— 271 chars.

---

## Tweet 4 — the prompt is the product

```
the most important file in the project is src/lib/prompt.ts — the system prompt that tells claude how to write for each platform

twitter punchy, linkedin professional, bluesky casual, no buzzword openers, no hashtag spam, never invent facts

PRs welcome, fork the prompt
```

— 274 chars.

---

## Tweet 5 — call to action

```
github.com/unipost-dev/agentpost

`npm install -g @unipost/agentpost && agentpost init`

honest feedback wanted, especially on the prompt. show HN post: <TODO Show HN URL>

(this very tweet was drafted by agentpost. i had to fight the urge to add a 🎉 to it)
```

— 275 chars. The last line is the meta-joke that lands with developers
and signals the product works on itself.

---

## Notes

- Tweet 1 is the only one with the gif. The rest are text-only — Twitter's
  algorithm decays threads with multiple media attachments.
- Don't reply to tweet 1 with the others — post them as a real thread
  via "Add another tweet" so they all show in the same expanded view.
- Quote-tweet your own thread from the @unipost X account (separate brand
  account if it exists) at +15min to amplify reach.
- Reply to early commenters within 5 minutes for the first hour. This
  is more important than the original tweets for the algorithm.

## Posting time

**Tuesday April 28 2026, 9:00 AM PT.** Same time as the Show HN post,
not after — the X traffic and the HN voting curve compound.
