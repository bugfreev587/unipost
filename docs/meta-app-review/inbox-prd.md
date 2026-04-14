# Meta Inbox v1 — PRD for Meta Scope Review and Demo

> Audience: UniPost product + engineering + design
> Goal: define the narrowest Inbox feature that justifies the newly requested Meta scopes and is realistic to demo in App Review
> Status: v1 proposal
> Date: April 13, 2026

---

## 1. TL;DR

We should not build a generic "social inbox" for Meta review.

We should build a narrow, review-friendly `Meta Inbox` inside the UniPost dashboard that lets a business:

1. publish content with UniPost,
2. receive Instagram comments, Instagram DMs, and Threads replies,
3. view those inbound interactions in one dashboard inbox,
4. reply as a human agent,
5. mark conversations resolved,
6. use lightweight post/account context to decide what to respond to first.

This is the smallest product surface that:

- maps cleanly to the newly requested scopes,
- is easy for a Meta reviewer to understand,
- looks like a real business tool instead of a permissions demo,
- stays consistent with UniPost's core positioning as social infrastructure.

---

## 2. Product framing

### 2.1 What this is

`Meta Inbox` is a dashboard feature for handling post-publication engagement on Instagram and Threads.

It is specifically for:

- Instagram direct messages
- Instagram post comments
- Threads post replies

### 2.2 What this is not

This is not:

- a full CRM
- a cross-network support suite
- a moderation platform for every social network
- a generic "community management" product

For App Review, the story must stay narrow:

> UniPost helps businesses publish content and then handle the resulting comments, replies, and messages from the same dashboard.

---

## 3. Success criteria

This feature is successful if a Meta reviewer can watch one short demo and clearly see:

1. why each requested scope is necessary,
2. what user-facing surface the scopes power,
3. that the actions are human-operated and business-appropriate,
4. that UniPost is not over-collecting data or using scopes for unrelated workflows.

Internally, the feature is successful if the same demo also gives UniPost a believable customer-facing Inbox story.

---

## 4. Requested scopes and product justification

### 4.1 Instagram

| Scope | Why Meta Inbox needs it |
|---|---|
| `instagram_business_basic` | Identify the connected Instagram business account and display account context in the inbox |
| `instagram_business_manage_messages` | Read and reply to Instagram business DMs |
| `instagram_business_content_publish` | Link inbox conversations back to posts published through UniPost |
| `instagram_business_manage_insights` | Show lightweight engagement context for prioritization |
| `instagram_business_manage_comments` | Read and reply to comments on Instagram posts |
| `instagram_manage_comments` | Support comment moderation / reply workflows |
| `public_profile` | Display sender/profile identity where available |
| `Human Agent` | Clearly support the "human replies to user messages" workflow |

### 4.2 Threads

| Scope | Why Meta Inbox needs it |
|---|---|
| `threads_basic` | Identify the connected Threads account and show author/profile context |
| `threads_content_publish` | Link replies back to UniPost-published Threads posts |
| `threads_read_replies` | Read replies on Threads posts |
| `threads_manage_replies` | Reply to Threads replies from UniPost |
| `threads_manage_insights` | Show lightweight engagement context in the inbox |

---

## 5. Core use cases

We should build and demonstrate only these three use cases.

### Use case 1: Instagram DM support

1. Business connects Instagram account to UniPost
2. End user sends a DM on Instagram
3. UniPost Inbox shows the message
4. A human support agent replies from UniPost
5. The thread is marked resolved

This is the clearest justification for:

- `instagram_business_manage_messages`
- `Human Agent`

### Use case 2: Instagram comment moderation and reply

1. Business publishes an Instagram post through UniPost
2. End user comments on that post
3. UniPost Inbox groups the comment under the original post
4. A human operator replies inline
5. The conversation remains associated with the source post

This justifies:

- `instagram_business_manage_comments`
- `instagram_manage_comments`
- `instagram_business_content_publish`

### Use case 3: Threads reply management

1. Business publishes a Threads post through UniPost
2. End user replies to that post
3. UniPost Inbox shows the reply under the original thread
4. A human operator replies from UniPost
5. The exchange is tracked as resolved/open

This justifies:

- `threads_read_replies`
- `threads_manage_replies`
- `threads_content_publish`

---

## 6. UX principles

### 6.1 Make the originating post a first-class object

For comments and replies, the user should never have to ask:

> "What post is this comment attached to?"

The right pane should always anchor the conversation to a post context card with:

- platform
- connected account
- caption preview
- media thumbnail if available
- post stats
- external "View on Instagram / Threads" action

### 6.2 Make the queue behave like an inbox, not a post browser

The left rail for comments and replies must be ordered by:

- newest unread activity first,
- then newest open activity,
- then resolved activity later

Never sort the queue by original post publish time.

### 6.3 Human workflow must be obvious

Especially for Instagram DMs, the interface must visibly communicate:

- this is being handled by a human agent,
- the conversation has a status,
- the reply is attributable to the business/operator,
- there is a lifecycle beyond "message exists".

### 6.4 Empty states must be operationally honest

`No messages` is not enough.

The UI must distinguish:

- no inbox items yet
- sync failed
- missing permission
- reconnect required
- no supported accounts connected

This matters both for real users and for App Review demos.

---

## 7. Information architecture

### 7.1 Dashboard nav

Add a top-level dashboard item:

- `Inbox`

### 7.2 Tabs

The Inbox page should have three tabs:

- `Comments`
- `DMs`
- `Threads`

This is intentionally simple for review. We can unify or expand later.

### 7.3 Layout

Use a two-pane layout:

- left rail: queue
- right pane: detail

This is simpler and more review-friendly than a denser three-column CRM layout.

---

## 8. Detailed interaction design

### 8.1 Page header

Top bar:

- `Inbox` title
- unread count
- `Mark all read`
- `Sync`

Optional secondary actions later:

- `Archive`
- `Filter`

### 8.2 Comments tab

Left rail groups items by source post.

Each row shows:

- post thumbnail or media placeholder
- platform icon
- connected account handle
- caption preview
- latest inbound interaction time
- unread badge
- comment/reply count
- open/resolved state

Right pane shows:

1. `PostContextCard`
2. conversation stream
3. inline reply box

Each comment item should support:

- `Reply`
- `Mark read`
- `Mark unread`
- `Resolve`

Owner replies should render with:

- `you` label
- subtle emerald treatment

### 8.3 Threads tab

Same structure as Comments.

Do not invent a separate interaction model for Threads in v1.
The data source differs, but the moderation UX should stay nearly identical.

### 8.4 DMs tab

Left rail groups by conversation/contact.

Each row shows:

- avatar
- platform badge
- contact handle
- latest message preview
- unread indicator
- assigned/open/resolved status
- `Human agent` badge where applicable

Right pane shows:

- thread header
- conversation history
- reply composer
- status controls

Required DM statuses:

- `Open`
- `Assigned`
- `Resolved`
- `Human agent`

At minimum, these can be demo states in v1, but they must be visible.

---

## 9. Required improvements over the current draft

These are not optional polish items. They should be part of the v1 spec.

### 9.1 Queue ordering

Change left rail ordering from post publish time to latest unresolved inbound activity.

Suggested sort priority:

1. unread + newest inbound timestamp
2. open + newest inbound timestamp
3. resolved + newest inbound timestamp

### 9.2 DM workflow state

Add explicit state presentation for DMs:

- assigned owner
- open / resolved
- human-agent handling

Even if assignment is mocked in demo data, the state needs to exist visually.

### 9.3 Error and reconnect state cards

Replace generic empty states with explicit system state cards:

- `Reconnect required`
- `Missing permission`
- `Sync failed`
- `No connected Instagram or Threads accounts`
- `No messages yet`

### 9.4 Real reply interaction in the demo

Comments and replies must support a real interactive mock:

- click `Reply`
- composer opens
- send action inserts the reply into the visible thread

It is acceptable for this to be frontend-local in the design demo, but it cannot be only a dead affordance.

### 9.5 Better search behavior

Comments / Threads search should include:

- post caption
- commenter handle
- comment/reply text

DM search should include:

- contact handle
- message body

---

## 10. Demo script for Meta review

This is the exact narrative we should optimize the product around.

### Demo A: Instagram DM

1. Show Instagram account connected in UniPost
2. Show end user sending a DM from Instagram
3. Refresh UniPost Inbox or show webhook-driven arrival
4. Open the DM thread
5. Show `Human agent` and `Open`
6. Reply from UniPost
7. Show the reply on Instagram
8. Mark conversation `Resolved`

### Demo B: Instagram comment

1. Publish an Instagram post through UniPost
2. Show a user comment on the live Instagram post
3. Open Inbox `Comments`
4. Show the comment grouped under the original post
5. Reply inline from UniPost
6. Show the reply on Instagram

### Demo C: Threads reply

1. Publish a Threads post through UniPost
2. Show a user reply on Threads
3. Open Inbox `Threads`
4. Show the reply grouped under the original thread
5. Reply from UniPost
6. Show resolved state

---

## 11. Implementation scope for v1

### 11.1 Must-have

- dashboard `Inbox` nav item
- Inbox page
- Comments / DMs / Threads tabs
- queue + detail layout
- post context card
- inline reply interaction
- DM status badges
- sync/error/reconnect state cards
- workspace-scoped list API integration

### 11.2 Nice-to-have

- archive
- assign to teammate
- bulk mark read
- platform dropdown filter
- SLA / response-time metrics

### 11.3 Out of scope for v1

- non-Meta platforms
- sentiment analysis
- auto-replies
- AI assistant suggestions
- shared inbox permissions model
- full moderation rule engine

---

## 12. Backend implications

The design should assume these backend realities:

1. Threads replies are currently more poll-driven than webhook-driven
2. Sync failures and missing scope states must be visible in UI
3. Connected account scope state needs to be trustworthy
4. Comment/reply ingestion must support enough post history to be believable

This PRD does not redefine backend contracts, but the UI must not hide those realities.

---

## 13. Open questions

1. Should `Comments` and `Threads` remain separate tabs in v1, or unify under one moderation queue with filters?
2. Should `Archive` ship in v1, or wait until reply + resolved states are stable?
3. Do we want assignment to be visual-only in the demo, or connected to real backend state?
4. Should the first implementation support nested reply trees, or just one visible reply level?

---

## 14. Recommendation

Proceed with a narrow `Meta Inbox v1` implementation, not a generic Inbox.

Execution order:

1. finalize this PRD
2. update the design mock to reflect queue ordering, DM states, error states, and real reply interaction
3. wire the demo UI into dashboard navigation
4. integrate the existing Inbox API
5. iterate on webhook / sync reliability separately

This gives UniPost the best chance of:

- passing Meta review,
- telling a coherent product story,
- and shipping a believable Inbox without overbuilding.
