import crypto from 'node:crypto';
import { UniPost, UniPostError, verifyWebhookSignature } from '../../../sdk/javascript/dist/index.mjs';

const API_KEY = process.env.UNIPOST_API_KEY || 'YOUR_API_KEY_HERE';
const TEST_ACCOUNT_ID_ENV = process.env.TEST_ACCOUNT_ID || '';
const TEST_PUBLISH_NOW = process.env.TEST_PUBLISH_NOW === 'true';

let passed = 0;
let failed = 0;
let skipped = 0;
const failures = [];

const createdPostIds = [];
const createdWebhookIds = [];
const createdMediaIds = [];
const createdPlatformCredentialKeys = [];

function section(title) {
  console.log(`\n${'─'.repeat(50)}`);
  console.log(`  ${title}`);
  console.log('─'.repeat(50));
}

async function test(name, fn) {
  process.stdout.write(`  ${name} ... `);
  try {
    const result = await fn();
    console.log('✅ PASS');
    passed += 1;
    return result;
  } catch (error) {
    console.log(`❌ FAIL — ${error.message}`);
    failed += 1;
    failures.push(`${name}: ${error.message}`);
    return null;
  }
}

function skip(name, reason) {
  console.log(`  ${name} ... ⏭ SKIP — ${reason}`);
  skipped += 1;
  return null;
}

function assert(condition, message) {
  if (!condition) throw new Error(message);
}

async function expectApiError(name, fn, expectedCodes = []) {
  return test(name, async () => {
    try {
      await fn();
    } catch (error) {
      if (!(error instanceof UniPostError)) {
        throw error;
      }
      if (expectedCodes.length > 0 && !expectedCodes.includes(error.code)) {
        throw new Error(`Expected ${expectedCodes.join('/')} but got ${error.code || 'unknown'}`);
      }
      return error.code;
    }
    throw new Error('Expected API error');
  });
}

async function cleanup(client) {
  if (createdWebhookIds.length || createdMediaIds.length || createdPostIds.length || createdPlatformCredentialKeys.length) {
    section('Cleanup');
  }

  for (const webhookId of createdWebhookIds.splice(0)) {
    try {
      await client.webhooks.delete(webhookId);
      console.log(`  🧹 Deleted webhook ${webhookId.slice(0, 8)}...`);
    } catch (error) {
      console.log(`  ⚠ Failed to delete webhook ${webhookId.slice(0, 8)}... (${error.message})`);
    }
  }

  for (const mediaId of createdMediaIds.splice(0)) {
    try {
      await client.media.delete(mediaId);
      console.log(`  🧹 Deleted media ${mediaId.slice(0, 8)}...`);
    } catch (error) {
      console.log(`  ⚠ Failed to delete media ${mediaId.slice(0, 8)}... (${error.message})`);
    }
  }

  for (const postId of createdPostIds.splice(0)) {
    try {
      await client.posts.delete(postId);
      console.log(`  🧹 Deleted post ${postId.slice(0, 8)}...`);
    } catch (error) {
      console.log(`  ⚠ Failed to delete post ${postId.slice(0, 8)}... (${error.message})`);
    }
  }

  for (const { workspaceId, platform } of createdPlatformCredentialKeys.splice(0)) {
    try {
      await client.platformCredentials.delete(workspaceId, platform);
      console.log(`  🧹 Deleted platform credential ${platform}`);
    } catch (error) {
      console.log(`  ⚠ Failed to delete platform credential ${platform}... (${error.message})`);
    }
  }
}

async function main() {
  console.log('\n╔══════════════════════════════════════════════════╗');
  console.log('║     @unipost/sdk — API Validation Test           ║');
  console.log('╚══════════════════════════════════════════════════╝\n');

  if (API_KEY === 'YOUR_API_KEY_HERE') {
    console.error('❌ Please set UNIPOST_API_KEY environment variable');
    process.exit(1);
  }

  const client = new UniPost({ apiKey: API_KEY });
  let testAccountId = TEST_ACCOUNT_ID_ENV;

  let workspace;
  let profiles = [];
  let accounts = [];
  let firstProfile;
  let firstPost;
  let draftPost;
  let scheduledPost;
  let createdWebhook;
  let createdMedia;
  let connectSession;

  section('1. Public catalogs');

  await test('platforms.capabilities()', async () => {
    const res = await client.platforms.capabilities();
    assert(typeof res.schema_version === 'string', 'Expected schema_version');
    assert(res.platforms && typeof res.platforms === 'object', 'Expected platforms map');
  });

  await test('plans.list()', async () => {
    const res = await client.plans.list();
    assert(Array.isArray(res), 'Expected plans array');
    assert(res.length > 0, 'Expected at least one plan');
  });

  section('2. Workspace & profiles');

  workspace = await test('workspace.get()', async () => {
    const res = await client.workspace.get();
    assert(res?.id, 'Expected workspace id');
    return res;
  });

  if (workspace) {
    await test('workspace.update() — no-op', async () => {
      const res = await client.workspace.update({
        perAccountMonthlyLimit: workspace.per_account_monthly_limit ?? null,
      });
      assert(res.id === workspace.id, 'Expected same workspace back');
    });
  }

  const profilesPage = await test('profiles.list()', async () => {
    const res = await client.profiles.list();
    assert(Array.isArray(res?.data), 'Expected profile data array');
    if (res.meta) {
      assert(typeof res.meta.total === 'number', 'Expected meta.total');
      assert(typeof res.meta.limit === 'number', 'Expected meta.limit');
    }
    return res;
  });
  profiles = profilesPage?.data || [];
  firstProfile = profiles[0];

  if (firstProfile) {
    await test('profiles.get()', async () => {
      const res = await client.profiles.get(firstProfile.id);
      assert(res.id === firstProfile.id, 'Expected matching profile');
    });

    await test('profiles.update() — no-op', async () => {
      const res = await client.profiles.update(firstProfile.id, {
        name: firstProfile.name,
        brandingLogoUrl: firstProfile.branding_logo_url ?? undefined,
        brandingDisplayName: firstProfile.branding_display_name ?? undefined,
        brandingPrimaryColor: firstProfile.branding_primary_color ?? undefined,
      });
      assert(res.id === firstProfile.id, 'Expected matching profile after update');
    });
  } else {
    skip('profiles.get()/update()', 'No profiles available');
    skip('profiles.update() — no-op', 'No profiles available');
  }

  section('3. Accounts');

  const accountsPage = await test('accounts.list()', async () => {
    const res = await client.accounts.list();
    assert(Array.isArray(res?.data), 'Expected accounts data array');
    assert(res.meta && typeof res.meta.total === 'number', 'Expected meta.total');
    assert(typeof res.meta.limit === 'number', 'Expected meta.limit');
    assert(res.data.length > 0, 'No connected accounts found');
    return res;
  });
  accounts = accountsPage?.data || [];
  const firstAccount = accounts[0];
  const tikTokAccount = accounts.find((account) => account.platform === 'tiktok');
  const facebookAccount = accounts.find((account) => account.platform === 'facebook');
  if (!testAccountId && firstAccount) {
    testAccountId = (accounts.find((account) => account.platform === 'bluesky') || firstAccount).id;
    console.log(`\n  Using TEST_ACCOUNT_ID=${testAccountId} for safe draft/scheduled tests`);
  }

  if (firstAccount) {
    await test('accounts.get()', async () => {
      const res = await client.accounts.get(firstAccount.id);
      assert(res.id === firstAccount.id, 'Expected matching account');
    });

    await test('accounts.health()', async () => {
      const res = await client.accounts.health(firstAccount.id);
      assert(res.social_account_id === firstAccount.id, 'Expected matching health account id');
    });

    await test('accounts.capabilities()', async () => {
      try {
        const res = await client.accounts.capabilities(firstAccount.id);
        assert(typeof res.schema_version === 'string', 'Expected schema_version');
        assert(res.capability && typeof res.capability === 'object', 'Expected capability');
      } catch (error) {
        if (!(error instanceof UniPostError) || !['not_found'].includes(error.code)) {
          throw error;
        }
      }
    });
  } else {
    skip('accounts.get()/health()/capabilities()', 'No accounts available');
    skip('accounts.health()', 'No accounts available');
    skip('accounts.capabilities()', 'No accounts available');
  }

  if (tikTokAccount) {
    await test('accounts.tikTokCreatorInfo()', async () => {
      const res = await client.accounts.tikTokCreatorInfo(tikTokAccount.id);
      assert(typeof res.creator_username === 'string' || typeof res.creator_nickname === 'string', 'Expected TikTok creator fields');
    });
  } else {
    skip('accounts.tikTokCreatorInfo()', 'No TikTok account connected');
  }

  if (facebookAccount) {
    await test('accounts.facebookPageInsights() — privileged path', async () => {
      try {
        const res = await client.accounts.facebookPageInsights(facebookAccount.id);
        assert(res && typeof res === 'object', 'Expected page insights payload');
      } catch (error) {
        if (error instanceof UniPostError && ['forbidden', 'facebook_disabled', 'FACEBOOK_DISABLED', 'not_found'].includes(error.code)) {
          return error.code;
        }
        throw error;
      }
    });
  } else {
    skip('accounts.facebookPageInsights()', 'No Facebook account connected');
  }

  await expectApiError(
    'accounts.connect() — invalid credentials negative path',
    () => client.accounts.connect({ platform: 'bluesky', credentials: { identifier: 'invalid', password: 'invalid' } }),
    ['auth_error', 'unauthorized', 'validation_error']
  );

  section('4. Media, connect sessions, users');

  createdMedia = await test('media.upload()', async () => {
    const res = await client.media.upload({
      filename: 'sdk-validation.png',
      contentType: 'image/png',
      sizeBytes: 128,
      contentHash: `sdk-js-${Date.now()}`,
    });
    assert(res.mediaId || res.media_id || res.id, 'Expected media id');
    createdMediaIds.push(res.mediaId || res.media_id || res.id);
    return res;
  });

  if (createdMedia) {
    await test('media.get()', async () => {
      const mediaId = createdMedia.mediaId || createdMedia.media_id || createdMedia.id;
      const res = await client.media.get(mediaId);
      assert((res.id || res.media_id) === mediaId, 'Expected matching media id');
    });
  }

  connectSession = await test('connect.createSession()', async () => {
    const res = await client.connect.createSession({
      platform: 'bluesky',
      profileId: firstProfile?.id,
      externalUserId: `sdk-js-${Date.now()}`,
      externalUserEmail: 'sdk-validation@example.com',
      returnUrl: 'https://example.com/return',
    });
    assert(res.id && res.url, 'Expected connect session id and url');
    return res;
  });

  if (connectSession?.id) {
    await test('connect.getSession()', async () => {
      const res = await client.connect.getSession(connectSession.id);
      assert(res.id === connectSession.id, 'Expected matching connect session');
    });
  }

  const usersPage = await test('users.list()', async () => {
    const res = await client.users.list();
    assert(Array.isArray(res?.data), 'Expected managed user array');
    return res;
  });

  if (usersPage?.data?.length) {
    await test('users.get()', async () => {
      const externalUserId = usersPage.data[0].external_user_id;
      const res = await client.users.get(externalUserId);
      assert(res.external_user_id === externalUserId, 'Expected matching managed user');
    });
  } else {
    skip('users.get()', 'No managed users available');
  }

  section('5. Webhooks');

  await test('verifyWebhookSignature()', async () => {
    const payload = JSON.stringify({ event: 'post.published', data: { id: 'post_test_123' } });
    const secret = 'whsec_test_local';
    const signature = `sha256=${crypto.createHmac('sha256', secret).update(payload).digest('hex')}`;
    assert(await verifyWebhookSignature({ payload, signature, secret }), 'Expected valid signature');
  });

  createdWebhook = await test('webhooks.create()', async () => {
    const res = await client.webhooks.create({
      url: 'https://example.com/unipost-webhook-test',
      events: ['post.published', 'post.partial', 'post.failed'],
    });
    assert(res.id && res.secret?.startsWith('whsec_'), 'Expected webhook id and secret');
    createdWebhookIds.push(res.id);
    return res;
  });

  if (createdWebhook?.id) {
    await test('webhooks.list()', async () => {
      const res = await client.webhooks.list();
      assert(Array.isArray(res?.data), 'Expected webhook data array');
      assert(res.data.find((item) => item.id === createdWebhook.id), 'Expected created webhook in list');
      assert(typeof res.meta?.total === 'number', 'Expected meta.total');
      assert(typeof res.meta?.limit === 'number', 'Expected meta.limit');
    });

    await test('webhooks.get()', async () => {
      const res = await client.webhooks.get(createdWebhook.id);
      assert(res.id === createdWebhook.id, 'Expected matching webhook');
      assert(!('secret' in res), 'Read payload should not expose secret');
    });

    await test('webhooks.update()', async () => {
      const res = await client.webhooks.update(createdWebhook.id, {
        active: false,
        events: ['post.failed'],
      });
      assert(res.active === false, 'Expected inactive webhook');
    });

    await test('webhooks.rotate()', async () => {
      const res = await client.webhooks.rotate(createdWebhook.id);
      assert(res.secret?.startsWith('whsec_'), 'Expected rotated secret');
    });
  }

  section('6. Platform credentials');

  if (workspace?.id) {
    const platformKey = `sdk-js-${Date.now()}`;
    await test('platformCredentials.create()/list()/delete()', async () => {
      try {
        const created = await client.platformCredentials.create(workspace.id, {
          platform: platformKey,
          clientId: 'sdk-client-id',
          clientSecret: 'sdk-client-secret',
        });
        assert(created.platform === platformKey, 'Expected created credential platform');
        createdPlatformCredentialKeys.push({ workspaceId: workspace.id, platform: platformKey });

        const listed = await client.platformCredentials.list(workspace.id);
        assert(Array.isArray(listed.data), 'Expected credential list');
        assert(listed.data.some((item) => item.platform === platformKey), 'Expected created credential in list');

        await client.platformCredentials.delete(workspace.id, platformKey);
        createdPlatformCredentialKeys.pop();
      } catch (error) {
        if (error instanceof UniPostError && error.code === 'forbidden') {
          console.log('⏭ SKIP — plan-gated');
          skipped += 1;
          return;
        }
        throw error;
      }
    });
  } else {
    skip('platformCredentials.create()/list()/delete()', 'No workspace context');
  }

  section('7. Posts');

  await test('posts.validate()', async () => {
    const res = await client.posts.validate({
      caption: 'SDK validation draft',
      accountIds: testAccountId ? [testAccountId] : [],
      status: 'draft',
    });
    assert(typeof res.valid === 'boolean', 'Expected validation result');
  });

  const postsPage = await test('posts.list()', async () => {
    const res = await client.posts.list({ limit: 5 });
    assert(Array.isArray(res.data), 'Expected posts data array');
    if (res.meta?.next_cursor !== undefined) {
      assert(res.nextCursor === res.meta.next_cursor, 'Expected nextCursor mirror');
    }
    return res;
  });
  firstPost = postsPage?.data?.[0];

  if (firstPost) {
    await test('posts.get()', async () => {
      const res = await client.posts.get(firstPost.id);
      assert(res.id === firstPost.id, 'Expected matching post');
    });

    await test('posts.getQueue()', async () => {
      const res = await client.posts.getQueue(firstPost.id);
      assert(res.post?.id === firstPost.id, 'Expected queue snapshot');
      assert(Array.isArray(res.jobs), 'Expected job list');
    });

    await test('posts.analytics()', async () => {
      const res = await client.posts.analytics(firstPost.id);
      assert(Array.isArray(res), 'Expected analytics array');
    });
  } else {
    skip('posts.get()', 'No posts available');
    skip('posts.getQueue()', 'No posts available');
    skip('posts.analytics()', 'No posts available');
  }

  if (!testAccountId) {
    skip('posts.create()/update()/preview/archive/restore/cancel/delete()', 'No TEST_ACCOUNT_ID available');
    skip('posts.bulkCreate()', 'No TEST_ACCOUNT_ID available');
    skip('posts.publish()', 'No TEST_ACCOUNT_ID available');
  } else {
    const timestamp = new Date().toISOString();

    draftPost = await test('posts.create() — draft', async () => {
      const res = await client.posts.create({
        caption: `SDK JS draft ${timestamp}`,
        accountIds: [testAccountId],
        status: 'draft',
      });
      assert(res.id && res.status === 'draft', 'Expected draft post');
      createdPostIds.push(res.id);
      return res;
    });

    if (draftPost?.id) {
      await test('posts.update() — draft', async () => {
        const res = await client.posts.update(draftPost.id, {
          caption: `${draftPost.caption || 'SDK JS draft'} updated`,
          accountIds: [testAccountId],
        });
        assert(res.id === draftPost.id, 'Expected updated draft');
      });

      await test('posts.previewLink()', async () => {
        const res = await client.posts.previewLink(draftPost.id);
        assert(typeof res.url === 'string' && typeof res.token === 'string', 'Expected preview link payload');
      });

      await test('posts.archive()', async () => {
        const res = await client.posts.archive(draftPost.id);
        assert(res.id === draftPost.id, 'Expected archived post');
      });

      await test('posts.restore()', async () => {
        const res = await client.posts.restore(draftPost.id);
        assert(res.id === draftPost.id, 'Expected restored post');
      });
    }

    scheduledPost = await test('posts.create() — scheduled', async () => {
      const scheduledAt = new Date(Date.now() + 15 * 60 * 1000).toISOString();
      const res = await client.posts.create({
        caption: `SDK JS scheduled ${timestamp}`,
        accountIds: [testAccountId],
        scheduledAt,
      });
      assert(res.id && res.status === 'scheduled', 'Expected scheduled post');
      createdPostIds.push(res.id);
      return res;
    });

    if (scheduledPost?.id) {
      await test('posts.update() — scheduled', async () => {
        try {
          const res = await client.posts.update(scheduledPost.id, {
            scheduledAt: new Date(Date.now() + 20 * 60 * 1000).toISOString(),
          });
          assert(res.id === scheduledPost.id, 'Expected updated scheduled post');
        } catch (error) {
          if (!(error instanceof UniPostError) || error.code !== 'validation_error') {
            throw error;
          }
        }
      });

      await test('posts.cancel()', async () => {
        const res = await client.posts.cancel(scheduledPost.id);
        assert(res.id === scheduledPost.id, 'Expected canceled post');
      });
    }

    await test('posts.bulkCreate()', async () => {
      const res = await client.posts.bulkCreate([
        {
          caption: `SDK JS bulk A ${timestamp}`,
          accountIds: [testAccountId],
          status: 'draft',
        },
        {
          caption: `SDK JS bulk B ${timestamp}`,
          accountIds: [testAccountId],
          status: 'draft',
        },
      ]);
      assert(Array.isArray(res) && res.length === 2, 'Expected two bulk result entries');
      for (const entry of res) {
        assert(entry.error || entry.data, 'Expected bulk result entry');
      }
    });

    if (TEST_PUBLISH_NOW && draftPost?.id) {
      await test('posts.publish() — live publish', async () => {
        const res = await client.posts.publish(draftPost.id);
        assert(res.id === draftPost.id, 'Expected published post response');
      });
    } else {
      skip('posts.publish() — live publish', 'Opt-in only (set TEST_PUBLISH_NOW=true)');
    }
  }

  if (firstPost?.results?.find((result) => result.status === 'failed')) {
    const failedResult = firstPost.results.find((result) => result.status === 'failed');
    await test('posts.retryResult() — conditional live retry', async () => {
      const res = await client.posts.retryResult(firstPost.id, failedResult.id);
      assert(res.social_account_id === failedResult.social_account_id, 'Expected retry result payload');
    });
  } else {
    skip('posts.retryResult()', 'No failed post result available to retry safely');
  }

  section('8. Delivery jobs, analytics, usage, oauth');

  await test('deliveryJobs.list()', async () => {
    const res = await client.deliveryJobs.list({ limit: 5 });
    const data = Array.isArray(res?.data) ? res.data : res;
    assert(Array.isArray(data), 'Expected delivery jobs array');
  });

  await test('deliveryJobs.summary()', async () => {
    const res = await client.deliveryJobs.summary();
    assert(res && typeof res === 'object', 'Expected summary object');
  });

  const retryableJobs = await client.deliveryJobs.list({ limit: 20, states: ['pending', 'retrying'] }).catch(() => ({ data: [] }));
  const retryableJob = (retryableJobs.data || retryableJobs || [])[0];
  if (retryableJob?.id) {
    await test('deliveryJobs.retry()/cancel() — conditional', async () => {
      try {
        const retried = await client.deliveryJobs.retry(retryableJob.id);
        assert(retried.id === retryableJob.id, 'Expected retried job');
      } catch (error) {
        if (!(error instanceof UniPostError) || !['queue_job_active', 'bad_request', 'conflict'].includes(error.code)) {
          throw error;
        }
      }

      try {
        const canceled = await client.deliveryJobs.cancel(retryableJob.id);
        assert(canceled.id === retryableJob.id, 'Expected canceled job');
      } catch (error) {
        if (!(error instanceof UniPostError) || !['bad_request', 'conflict'].includes(error.code)) {
          throw error;
        }
      }
    });
  } else {
    skip('deliveryJobs.retry()/cancel()', 'No pending/retrying delivery job available');
  }

  const from = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
  const to = new Date().toISOString().slice(0, 10);
  const fromTs = new Date(Date.now() - 30 * 24 * 60 * 60 * 1000).toISOString();
  const toTs = new Date().toISOString();

  await test('analytics.summary()', async () => {
    const res = await client.analytics.summary({ from, to });
    assert(res.posts && res.engagement, 'Expected summary payload');
  });

  await test('analytics.trend()', async () => {
    const res = await client.analytics.trend({ from, to });
    assert(Array.isArray(res.dates), 'Expected trend dates');
  });

  await test('analytics.byPlatform()', async () => {
    const res = await client.analytics.byPlatform({ from, to });
    assert(Array.isArray(res), 'Expected by-platform array');
  });

  await test('analytics.rollup()', async () => {
    const res = await client.analytics.rollup({ from: fromTs, to: toTs, granularity: 'day' });
    assert(Array.isArray(res.series), 'Expected rollup series');
  });

  await test('usage.get()', async () => {
    const res = await client.usage.get();
    assert(typeof res.post_count === 'number', 'Expected usage payload');
  });

  await test('oauth.connect() — known backend path', async () => {
    try {
      const res = await client.oauth.connect('bluesky', { redirectUrl: 'https://example.com/callback' });
      assert(typeof res.auth_url === 'string', 'Expected auth_url');
    } catch (error) {
      if (error instanceof UniPostError && ['unauthorized', 'validation_error'].includes(error.code)) {
        return 'backend currently does not expose an OAuth-capable public flow for this platform';
      }
      throw error;
    }
  });

  await cleanup(client);

  console.log('\n╔══════════════════════════════════════════════════╗');
  console.log(`║  Results: ${String(passed).padStart(2, ' ')} passed  ${String(failed).padStart(2, ' ')} failed  ${String(skipped).padStart(2, ' ')} skipped      ║`);
  console.log('╚══════════════════════════════════════════════════╝\n');

  if (failures.length > 0) {
    console.log('Failed tests:');
    for (const failure of failures) {
      console.log(`  ❌ ${failure}`);
    }
    process.exit(1);
  }

  console.log('🎉 All required JS SDK validations passed.\n');
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});
