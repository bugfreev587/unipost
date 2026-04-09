# Show HN — AgentPost

**Status: DRAFT.** Iterate with Xiaobo before launch day.

## Title

```
Show HN: AgentPost – describe what you shipped, AI posts it everywhere
```

(72 characters. Within HN's title length cap. Verb-driven, no buzzword.
Two alternates if the primary feels off:)

```
Show HN: AgentPost – an open-source CLI that posts to every social network at once
Show HN: AgentPost – tell Claude what you shipped, watch it post everywhere
```

## URL

```
https://github.com/unipost-dev/agentpost
```

## Body (under 200 words)

Hi HN,

I built AgentPost because I kept skipping social posts after shipping
features. Writing the same news three different ways for Twitter, LinkedIn
and Bluesky was friction. So I'd cross-post the same caption everywhere
and it'd land flat on every platform, or I'd spend 15 minutes rewriting
the same announcement four times, or — most often — I just wouldn't
post.

AgentPost is the CLI version of "let Claude rewrite this for me." You
type `agentpost "shipped webhooks today 🎉"`, it pulls your connected
accounts, asks Claude to draft per-platform posts, renders previews in
the terminal with character counts, and publishes on Enter.

It runs on UniPost (which I also built — it's the publishing API that
handles the OAuth and rate limits under the hood). The CLI is MIT
licensed; the API is paid SaaS. Use either, both, or just steal the
prompt — `src/lib/prompt.ts` is the most important file in the project
and I want it forked.

Honest feedback welcome — especially on the prompt and the platform
support gaps. I'm tired of seeing "Excited to announce..." posts on
LinkedIn and AgentPost is one small attempt to make those go away.

Repo: https://github.com/unipost-dev/agentpost
30s demo: <TODO link to asciinema or vhs gif>

— Xiaobo

---

## Notes for the post

- 197 words. Right at the cap. Don't pad.
- Mentions UniPost twice (once as parent, once disclosing it's paid).
  Optimal balance — discoverable without coming across as marketing.
- Self-deprecating ("I just wouldn't post") + concrete pain point
  ("15 minutes rewriting") signals real founder-built-it energy.
- Explicit ask for prompt feedback gives commenters a low-friction
  way to engage. Comments drive the algorithm.
- "Steal the prompt" + explicit MIT license framing = no perceived
  rug pull risk.
- The "Excited to announce" line is a callback that's likely to land
  with HN's audience. Risk: feels too snarky. Cut if it doesn't pass
  the morning re-read.

## Posting checklist

- [ ] Asciinema recording captured + uploaded as a GIF (vhs is the
      tool — `vhs demo.tape > demo.gif`). Embed in the README first,
      then link to it from the Show HN post body.
- [ ] @unipost/agentpost is published as v0.1.0 on npm (PR9 deploy
      already pushed; need to tag the release in GitHub to trigger
      the publish workflow)
- [ ] github.com/unipost-dev/agentpost README renders correctly
      with the demo gif visible above the fold
- [ ] Twitter @yuxiaobohit + UniPost X account both have a fresh
      pinned tweet about AgentPost ready to post simultaneously
- [ ] LinkedIn post draft (see linkedin-post.md) ready in the
      LinkedIn compose box
- [ ] Loops email to existing UniPost signups queued for send at
      launch+15min (gives the post some HN momentum first)
- [ ] Anthropic key for the demo is hidden in the env, never visible
      on screen
- [ ] Browser tabs cleaned up before screen recording
- [ ] At least three friends know the launch is happening at 9am PT
      and will engage in the comments early
