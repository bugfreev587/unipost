# @unipost/sdk

Official UniPost API client for JavaScript and TypeScript.
Post to 7 social platforms with one API call.

## Installation

```bash
npm install @unipost/sdk
```

## Quick Start

```typescript
import { UniPost } from '@unipost/sdk'

// Reads UNIPOST_API_KEY from environment automatically
const client = new UniPost()

const post = await client.posts.create({
  caption: 'Hello from UniPost! 🚀',
  accountIds: ['sa_twitter_xxx', 'sa_linkedin_xxx'],
})
```

## Usage

### List Accounts

```typescript
const { data: accounts } = await client.accounts.list()

// Filter by platform
const twitterAccounts = await client.accounts.list({ platform: 'twitter' })
```

### Create Posts

```typescript
// Immediate publish
const post = await client.posts.create({
  caption: 'Hello world!',
  accountIds: ['sa_twitter_xxx'],
})

// Scheduled
const post = await client.posts.create({
  caption: 'Scheduled post',
  accountIds: ['sa_twitter_xxx'],
  scheduledAt: '2026-04-28T09:00:00Z',
})

// Per-platform captions
const post = await client.posts.create({
  platformPosts: [
    { accountId: 'sa_twitter_xxx', caption: 'Short tweet 🐦' },
    { accountId: 'sa_linkedin_xxx', caption: 'Longer LinkedIn version...' },
  ],
})

// Twitter thread
const post = await client.posts.create({
  platformPosts: [
    { accountId: 'sa_twitter_xxx', caption: '1/ First', threadPosition: 1 },
    { accountId: 'sa_twitter_xxx', caption: '2/ Second', threadPosition: 2 },
  ],
})

// Save as draft
const draft = await client.posts.create({
  caption: 'Work in progress',
  accountIds: ['sa_twitter_xxx'],
  status: 'draft',
})

// Publish a draft
await client.posts.publish('post_xxx')
```

### Query Posts

```typescript
// List with filters
const { data, nextCursor } = await client.posts.list({
  status: 'published',
  platform: 'twitter',
  limit: 25,
})

// Auto-paginate
for await (const post of client.posts.listAll({ status: 'published' })) {
  console.log(post.id, post.status)
}

// Get analytics
const analytics = await client.posts.analytics('post_xxx')
```

### Media Upload

```typescript
// Two-step upload
const { mediaId, uploadUrl } = await client.media.upload({
  filename: 'photo.jpg',
  contentType: 'image/jpeg',
  sizeBytes: 1048576,
})

await fetch(uploadUrl, { method: 'PUT', body: fileBuffer })

// Or upload from file (Node.js)
const mediaId = await client.media.uploadFile('./photo.jpg')
```

### Connect (Managed Users)

```typescript
const session = await client.connect.createSession({
  platform: 'twitter',
  externalUserId: 'your_user_123',
  returnUrl: 'https://yourapp.com/callback',
})
// Send session.url to your user
```

### Webhook Verification

```typescript
import { verifyWebhookSignature } from '@unipost/sdk'

const isValid = await verifyWebhookSignature({
  payload: req.body,
  signature: req.headers['x-unipost-signature'],
  secret: process.env.UNIPOST_WEBHOOK_SECRET!,
})
```

## Error Handling

```typescript
import { UniPost, AuthError, RateLimitError, UniPostError } from '@unipost/sdk'

try {
  await client.posts.create({ ... })
} catch (error) {
  if (error instanceof AuthError) {
    // 401 - API key invalid
  } else if (error instanceof RateLimitError) {
    // 429 - retry after error.retryAfter seconds
  } else if (error instanceof UniPostError) {
    // Other API error
    console.error(error.status, error.code, error.message)
  }
}
```

## TypeScript

Full type definitions included. IDE autocomplete works out of the box.

```typescript
import type { Post, SocialAccount, Platform, CreatePostParams } from '@unipost/sdk'
```

## Runtime Support

- Node.js 18+
- Bun
- Deno
- Browsers
- Edge runtimes (Cloudflare Workers, Vercel Edge)

Zero production dependencies.

## License

MIT
