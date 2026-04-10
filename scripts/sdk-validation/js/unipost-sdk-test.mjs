/**
 * @unipost/sdk Validation Test
 *
 * Tests all supported API operations against the live UniPost API.
 *
 * Setup:
 *   1. npm install @unipost/sdk
 *   2. Get an API key from https://app.unipost.dev → API Keys
 *   3. UNIPOST_API_KEY=up_live_xxx node unipost-sdk-test.mjs
 *
 * Or set directly below:
 */

import { UniPost } from '@unipost/sdk';

// ─── Config ───────────────────────────────────────────────────────────────────

const API_KEY = process.env.UNIPOST_API_KEY || 'YOUR_API_KEY_HERE';

// After running listAccounts(), paste one account_id here for post tests.
// Leave empty to skip post creation tests (safe for first run).
const TEST_ACCOUNT_ID = process.env.TEST_ACCOUNT_ID || '';

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

  // ── 2. Posts — list ─────────────────────────────────────────────────────────
  section('2. Posts — list & get');

  const posts = await test('posts.list()', async () => {
    const res = await client.posts.list({ limit: 5 });
    if (!res || !Array.isArray(res.data)) throw new Error('Expected data array');
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
  } else {
    console.log('\n  No posts yet — skipping posts.get() test');
  }

  // ── 3. Posts — create (only if TEST_ACCOUNT_ID is set) ──────────────────────
  section('3. Posts — create (draft mode, no actual publishing)');

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
          caption: `[SDK Test] Hello from @unipost/sdk v0.1.0 🚀 ${timestamp}`,
          accountIds: [TEST_ACCOUNT_ID],
        });
        if (!res?.id) throw new Error('Expected post object with id');
        return res;
      });
    } else {
      console.log('\n  ⏭  Real publish skipped (set TEST_PUBLISH_NOW=true to enable)');
    }
  }

  // ── 4. Analytics ────────────────────────────────────────────────────────────
  section('4. Analytics');

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
    console.log('🎉 All tests passed! @unipost/sdk is working correctly.\n');
  }
}

main().catch(err => {
  console.error('\n💥 Unexpected error:', err);
  process.exit(1);
});
