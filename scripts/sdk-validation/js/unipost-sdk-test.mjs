/**
 * @unipost/sdk Validation Test
 *
 * Tests all supported API operations against the live UniPost API.
 *
 * Setup:
 *   1. npm install
 *   2. Get an API key from https://app.unipost.dev → API Keys
 *   3. UNIPOST_API_KEY=up_live_xxx node unipost-sdk-test.mjs
 *
 * Or set directly below:
 */

import crypto from 'node:crypto';
import { UniPost, verifyWebhookSignature } from '../../../sdk/javascript/dist/index.mjs';

// ─── Config ───────────────────────────────────────────────────────────────────

const API_KEY = process.env.UNIPOST_API_KEY || 'YOUR_API_KEY_HERE';
const API_URL = process.env.UNIPOST_API_URL || 'https://api.unipost.dev';

// After running listAccounts(), paste one account_id here for post tests.
// Leave empty to skip post creation tests (safe for first run).
let TEST_ACCOUNT_ID = process.env.TEST_ACCOUNT_ID || '';

// Track post IDs created during the test run for cleanup.
const createdPostIds = [];
const createdWebhookIds = [];

// ─── Test runner ──────────────────────────────────────────────────────────────

let passed = 0;
let failed = 0;
const results = [];

async function test(name, fn) {
  process.stdout.write(`  ${name} ... `);
  try {
    const result = await fn();
    console.log('✅ PASS');
    passed++;
    results.push({ name, status: 'pass', data: result });
    return result;
  } catch (err) {
    console.log(`❌ FAIL — ${err.message}`);
    failed++;
    results.push({ name, status: 'fail', error: err.message });
    return null;
  }
}

function section(title) {
  console.log(`\n${'─'.repeat(50)}`);
  console.log(`  ${title}`);
  console.log('─'.repeat(50));
}

// ─── Main ─────────────────────────────────────────────────────────────────────

async function main() {
  console.log('\n╔══════════════════════════════════════════════════╗');
  console.log('║     @unipost/sdk — API Validation Test           ║');
  console.log('╚══════════════════════════════════════════════════╝\n');

  if (API_KEY === 'YOUR_API_KEY_HERE') {
    console.error('❌ Please set UNIPOST_API_KEY environment variable');
    console.error('   UNIPOST_API_KEY=up_live_xxx node unipost-sdk-test.mjs');
    process.exit(1);
  }

  const client = new UniPost({ apiKey: API_KEY });

  // ── 1. Accounts ─────────────────────────────────────────────────────────────
  section('1. Accounts — list connected social accounts');

  const accounts = await test('listAccounts()', async () => {
    const res = await client.accounts.list();
    if (!res || !Array.isArray(res.data)) throw new Error('Expected data array');
    if (!res.meta || typeof res.meta.total !== 'number') throw new Error('Expected meta.total');
    if (typeof res.meta.limit !== 'number') throw new Error('Expected meta.limit');
    if (res.data.length === 0) throw new Error('No accounts found — connect at least one');
    return res.data;
  });

  if (accounts) {
    console.log(`\n  Found ${accounts.length} connected accounts:`);
    accounts.forEach(a => {
      console.log(`    • [${a.platform.padEnd(10)}] ${a.account_name || a.id}  (id: ${a.id})`);
    });

    // Print first Bluesky account id as a good safe test target
    const bluesky = accounts.find(a => a.platform === 'bluesky');
    if (bluesky && !TEST_ACCOUNT_ID) {
      console.log(`\n  💡 Tip: Run with TEST_ACCOUNT_ID=${bluesky.id} to test post creation`);
    }
  }

  if (!TEST_ACCOUNT_ID && accounts?.length) {
    const safestAccount = accounts.find(a => a.platform === 'bluesky') || accounts[0];
    TEST_ACCOUNT_ID = safestAccount.id;
    console.log(`\n  Using TEST_ACCOUNT_ID=${TEST_ACCOUNT_ID} for safe draft/scheduled tests`);
  }

  // ── 2. Profiles — list (raw API, pending SDK v0.2.0) ──────────────────────
  section('2. Profiles — list & filter accounts by profile');

  const profilesRes = await test('GET /v1/profiles', async () => {
    const res = await fetch(`${API_URL}/v1/profiles`, {
      headers: { Authorization: `Bearer ${API_KEY}` },
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const json = await res.json();
    if (!Array.isArray(json.data)) throw new Error('Expected data array');
    return json.data;
  });

  if (profilesRes && profilesRes.length > 0) {
    console.log(`\n  Found ${profilesRes.length} profiles:`);
    profilesRes.forEach(p => console.log(`    • ${p.name}  (id: ${p.id})`));

    // Test profile_id filter on accounts
    const firstProfile = profilesRes[0];
    await test(`GET /v1/social-accounts?profile_id=${firstProfile.id.slice(0, 8)}...`, async () => {
      const res = await fetch(`${API_URL}/v1/social-accounts?profile_id=${firstProfile.id}`, {
        headers: { Authorization: `Bearer ${API_KEY}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const json = await res.json();
      if (!Array.isArray(json.data)) throw new Error('Expected data array');
      console.log(`    → ${json.data.length} accounts in profile "${firstProfile.name}"`);
      return json.data;
    });
  }

  // ── 3. Webhooks — signature + CRUD ─────────────────────────────────────────
  section('3. Webhooks — signature verification & subscription CRUD');

  await test('verifyWebhookSignature()', async () => {
    const payload = JSON.stringify({
      event: 'post.published',
      timestamp: new Date().toISOString(),
      data: { id: 'post_test_123' },
    });
    const secret = 'whsec_test_local';
    const signature = `sha256=${crypto.createHmac('sha256', secret).update(payload).digest('hex')}`;
    const valid = await verifyWebhookSignature({ payload, signature, secret });
    if (!valid) throw new Error('Expected signature to verify');
    return true;
  });

  const webhook = await test('webhooks.create()', async () => {
    const res = await client.webhooks.create({
      url: 'https://example.com/unipost-webhook-test',
      events: ['post.published', 'post.partial', 'post.failed'],
    });
    if (!res?.id) throw new Error('Expected webhook id');
    if (!res.secret || !res.secret.startsWith('whsec_')) throw new Error('Expected generated signing secret');
    return res;
  });

  if (webhook?.id) {
    createdWebhookIds.push(webhook.id);

    await test('webhooks.list()', async () => {
      const res = await client.webhooks.list();
      if (!Array.isArray(res?.data)) throw new Error('Expected data array');
      if (!res.data.find((item) => item.id === webhook.id)) throw new Error('Created webhook not found in list');
      if (!res.meta || typeof res.meta.total !== 'number') throw new Error('Expected meta.total');
      if (typeof res.meta.limit !== 'number') throw new Error('Expected meta.limit');
      return res.data;
    });

    await test(`webhooks.get("${webhook.id.slice(0, 8)}...")`, async () => {
      const res = await client.webhooks.get(webhook.id);
      if (res.id !== webhook.id) throw new Error('Wrong webhook returned');
      if ('secret' in res) throw new Error('Read response should not contain plaintext secret');
      return res;
    });

    await test('webhooks.update()', async () => {
      const res = await client.webhooks.update(webhook.id, {
        active: false,
        events: ['post.failed'],
      });
      if (res.active !== false) throw new Error('Expected active=false');
      if (!Array.isArray(res.events) || res.events.length !== 1 || res.events[0] !== 'post.failed') {
        throw new Error('Expected updated events');
      }
      return res;
    });

    await test('webhooks.rotate()', async () => {
      const res = await client.webhooks.rotate(webhook.id);
      if (!res.secret || !res.secret.startsWith('whsec_')) throw new Error('Expected rotated secret');
      return res;
    });
  }

  // ── 4. Posts — list ─────────────────────────────────────────────────────────
  section('4. Posts — list & get');

  const posts = await test('posts.list()', async () => {
    const res = await client.posts.list({ limit: 5 });
    if (!res || !Array.isArray(res.data)) throw new Error('Expected data array');
    if (res.meta && res.meta.next_cursor !== undefined && res.nextCursor !== res.meta.next_cursor) {
      throw new Error('Expected nextCursor to mirror meta.next_cursor');
    }
    return res.data;
  });

  if (posts && posts.length > 0) {
    const firstPost = posts[0];
    console.log(`\n  First post: "${(firstPost.caption || '').slice(0, 60)}..."`);
    console.log(`  Status: ${firstPost.status}  |  Platform results: ${firstPost.results?.length ?? 0}`);

    await test(`posts.get("${firstPost.id.slice(0, 8)}...")`, async () => {
      const res = await client.posts.get(firstPost.id);
      if (!res?.id) throw new Error('Expected post object with id');
      return res;
    });

    await test(`posts.getQueue("${firstPost.id.slice(0, 8)}...")`, async () => {
      const res = await client.posts.getQueue(firstPost.id);
      if (!res?.post?.id) throw new Error('Expected queue snapshot with post');
      if (!Array.isArray(res.jobs)) throw new Error('Expected jobs array');
      return res;
    });
  } else {
    console.log('\n  No posts yet — skipping posts.get() and posts.getQueue() tests');
  }

  // ── 5. Posts — create (only if TEST_ACCOUNT_ID is set) ──────────────────────
  section('5. Posts — create (draft mode, no actual publishing)');

  if (!TEST_ACCOUNT_ID) {
    console.log('  ⏭  Skipped — set TEST_ACCOUNT_ID env var to run post creation tests');
    console.log('     Example: TEST_ACCOUNT_ID=<account_id> node unipost-sdk-test.mjs');
  } else {
    const timestamp = new Date().toISOString();
    const caption = `SDK validation test — ${timestamp} [auto-generated, will be deleted]`;

    // Test: create a draft (safe, not actually published)
    let draftPostId = null;
    const draft = await test('posts.create() — draft mode', async () => {
      const res = await client.posts.create({
        caption,
        accountIds: [TEST_ACCOUNT_ID],
        status: 'draft',
      });
      if (!res?.id) throw new Error('Expected post object with id');
      if (res.status !== 'draft') throw new Error(`Expected status=draft, got ${res.status}`);
      return res;
    });

    if (draft) {
      draftPostId = draft.id;
      createdPostIds.push(draftPostId);
      console.log(`\n  Created draft post: ${draftPostId}`);
    }

    // Test: create a scheduled post (10 min in future)
    const scheduledAt = new Date(Date.now() + 10 * 60 * 1000).toISOString();
    let scheduledPostId = null;
    const scheduled = await test('posts.create() — scheduled mode (10 min future)', async () => {
      const res = await client.posts.create({
        caption: `SDK scheduled test — ${timestamp}`,
        accountIds: [TEST_ACCOUNT_ID],
        scheduledAt,
      });
      if (!res?.id) throw new Error('Expected post object with id');
      if (res.status !== 'scheduled') throw new Error(`Expected status=scheduled, got ${res.status}`);
      return res;
    });

    if (scheduled) {
      scheduledPostId = scheduled.id;
      createdPostIds.push(scheduledPostId);
      console.log(`  Created scheduled post: ${scheduledPostId}`);
      console.log(`  Scheduled for: ${scheduledAt}`);
    }

    // Test: cancel the scheduled post
    if (scheduledPostId) {
      await test(`posts.cancel("${scheduledPostId.slice(0, 8)}...")`, async () => {
        const res = await client.posts.cancel(scheduledPostId);
        if (!res) throw new Error('Expected response data');
        return res;
      });
      console.log('  Cancelled scheduled post ✓');
    }

    // Test: create NOW post (real publish — only if explicitly confirmed)
    const publishNow = process.env.TEST_PUBLISH_NOW === 'true';
    if (publishNow) {
      await test('posts.create() — publish NOW (real post)', async () => {
        const res = await client.posts.create({
          caption: `[SDK Test] Hello from local @unipost/sdk 🚀 ${timestamp}`,
          accountIds: [TEST_ACCOUNT_ID],
        });
        if (!res?.id) throw new Error('Expected post object with id');
        return res;
      });
    } else {
      console.log('\n  ⏭  Real publish skipped (set TEST_PUBLISH_NOW=true to enable)');
    }
  }

  // ── 6. Analytics ────────────────────────────────────────────────────────────
  section('6. Analytics');

  const now = new Date();
  const thirtyDaysAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);

  await test('analytics.rollup() — last 30 days', async () => {
    const res = await client.analytics.rollup({
      from: thirtyDaysAgo.toISOString(),
      to: now.toISOString(),
      granularity: 'day',
    });
    if (!res) throw new Error('Expected rollup data');
    return res;
  });

  // ── Cleanup ─────────────────────────────────────────────────────────────────
  if (createdWebhookIds.length > 0 || createdPostIds.length > 0) {
    section('7. Cleanup');
  }

  if (createdWebhookIds.length > 0) {
    for (const id of createdWebhookIds) {
      try {
        await client.webhooks.delete(id);
        console.log(`  🧹 Deleted webhook ${id.slice(0, 8)}...`);
      } catch {
        console.log(`  ⚠  Failed to delete webhook ${id.slice(0, 8)}... (non-fatal)`);
      }
    }
  }

  if (createdPostIds.length > 0) {
    for (const id of createdPostIds) {
      try {
        await fetch(`${API_URL}/v1/social-posts/${id}`, {
          method: 'DELETE',
          headers: { Authorization: `Bearer ${API_KEY}` },
        });
        console.log(`  🗑  Deleted ${id.slice(0, 8)}...`);
      } catch {
        console.log(`  ⚠  Failed to delete ${id.slice(0, 8)}... (non-fatal)`);
      }
    }
  }

  // ── Summary ──────────────────────────────────────────────────────────────────
  console.log('\n╔══════════════════════════════════════════════════╗');
  console.log(`║  Results: ${String(passed).padStart(2)} passed  ${String(failed).padStart(2)} failed                    ║`);
  console.log('╚══════════════════════════════════════════════════╝\n');

  if (failed > 0) {
    console.log('Failed tests:');
    results
      .filter(r => r.status === 'fail')
      .forEach(r => console.log(`  ❌ ${r.name}: ${r.error}`));
    process.exit(1);
  } else {
    console.log('🎉 All tests passed! local @unipost/sdk is working correctly.\n');
  }
}

main().catch(err => {
  console.error('\n💥 Unexpected error:', err);
  process.exit(1);
});
