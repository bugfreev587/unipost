import assert from "node:assert/strict";
import { createRequire } from "node:module";
import { readFile } from "node:fs/promises";
import { test } from "node:test";

const read = (path) => readFile(new URL(`../../${path}`, import.meta.url), "utf8");
const requireFromDashboard = createRequire(new URL("../../dashboard/package.json", import.meta.url));
const { load: parseYaml } = requireFromDashboard("js-yaml");

const publishingRestrictionScript =
  "node --test src/lib/publishing-restrictions.test.ts tests/admin-publishing-restrictions-source.test.mjs tests/create-post-drawer-restrictions-refresh.test.mjs tests/publishing-restrictions-customer-source.test.mjs tests/post-result-errors.test.mts";
const postgresTestsByPackage = {
  "./internal/db": [
    "TestMigrationGatePostgresApplies125AfterVerifiedBackupThenContinues131",
    "TestMigrationGatePostgresFailureBeforeVerificationLeaves124Unchanged",
    "TestMigrationGatePostgresExcludesHistoricalRunMigrationsUntilBackupVerified",
    "TestMigrationGatePostgresConcurrentPreDeploysCreateOneBackup",
    "TestMigrationGatePostgresReplacementAfterLockedOrphanCreatesFreshBackup",
    "TestRequireCurrentSchemaRejects124AndAccepts131",
    "TestRequireCurrentSchemaRejectsNewerDatabaseAsUnsafeRollback",
    "TestPublishingRestrictionFailedRecipientUpgradeConvergesAfterExecuted124",
    "TestPublishingRestrictionRecipientOwnerSnapshotUpgradeAndDown",
    "TestCreateEmailSendAttemptAuditPreservesTerminalSentRecord",
    "TestCreateEmailSendAttemptAuditStillRetriesFailedRecord",
    "TestCreateSkippedEmailSendAttemptAuditPreservesTerminalSentRecord",
    "TestCreateSkippedEmailSendAttemptAuditUpdatesNonSentRecords",
    "TestMigration127UpgradeRollingCompatibilityAndDown",
    "TestPostPublishedOutboxSnapshotsFirstPostBeforeDelayedDispatch",
    "TestPublishingRestrictionRetentionDeadlineSurvivesMediaHardDelete",
    "TestPublishingRestrictionRetentionDeadlineNotProjectedForTextOnlyPost",
    "TestPublishingRestrictionRetentionRepairsUnsafeMetadataBeforeHardDelete",
  ],
  "./internal/handler": [
    "TestFinalizeRestrictedPostDeliveryJobPostgresLeaseAtomicity",
    "TestFinalizeRestrictedPostDeliveryJobRequiresEveryMediaBeforeMutation",
    "TestFinalizeRestrictedPostDeliveryJobCleanupOrdering",
    "TestFinalizeRestrictedPostDeliveryJobAllowsEmptyMediaSet",
    "TestFinalizeRestrictedPostDeliveryJobConcurrentResultsConvergeParentStatus",
    "TestRestrictedAndPublishedFinalizersConvergeParentPartial",
    "TestRestrictedAndOrdinaryTerminalFailureConvergeParentFailed",
    "TestProcessPostDeliveryJobPublishedEarlyExitRefreshesParent",
    "TestRecoverStalePublishedResultRefreshesParent",
    "TestCancelDeliveryJobReturnsMissingParentRefreshError",
    "TestCancelDeliveryJobReturnsParentStatusUpdateError",
    "TestCancelDeliveryJobDoesNotOverwriteConcurrentWorkerClaim",
    "TestHandleJobDispatchFailureStaleLeaseHasNoSharedSideEffects",
    "TestRecoverStaleDeliveryJobLostLeaseHasNoSharedSideEffects",
    "TestProcessPostDeliveryJobProviderSuccessSurvivesLostLeaseAndRetryConverges",
    "TestPublishTokenCallbackLostLeaseDoesNotOverwriteNewOwnerToken",
    "TestTerminalParentRefreshWaiterPreservesCommittedPublishedAt",
    "TestTerminalParentEventPublishesAfterStatusCommit",
    "TestOrdinaryFinalizationPublishedRaceAndFacebookProcessingAuthority",
    "TestDispatchFailureParentUpdateErrorRollsBackEntireTerminalTransition",
    "TestFacebookProviderPollDefinitiveFailureAndPublishedRace",
    "TestEnqueueRetryAndTerminalFinalizerSerializePostProjection",
    "TestTerminalParentEventSerializesBeforeManualRetry",
    "TestTerminalParentEventSkipsStaleProjectionWhenRetryWins",
    "TestTerminalRetentionSortsSharedMediaLocksAcrossPosts",
    "TestCancelDeliveryJobTerminalResultAndPostCommitRetention",
    "TestRestrictedFinalizationPublishesParentTerminalEventExactlyOnce",
    "TestScheduledQuotaSnapshotPostgresCountsOnlyAdmissionAllowedTargets",
    "TestClaimScheduledPostDoesNotDoubleCountOwnQuotaReservation",
    "TestUpdateScheduledPostEnforcesFreeQuotaDeltaAtomically",
    "TestConcurrentFreeScheduledCreatesSerializeQuotaAdmission",
    "TestConcurrentScheduledIdempotencyReplayPrecedesMutableCapacityGates",
    "TestConcurrentScheduledIdempotencyDifferentPayloadConflicts",
    "TestScheduledIdempotencySerializesPolicyAndEnqueueBehindKeyLock",
    "TestScheduledIdempotencyPolicyUsesTransactionAtMaxConnsOne",
    "TestScheduledQuotaHoldIdempotencyPhaseAUsesLegacyIndexAndNewLookup",
    "TestConcurrentDueReservationGrowthSerializesAndCrossMonthUsesFullUnits",
    "TestAtomicMonthlySnapshotRejectsDuringTerminalReservationConversion",
    "TestPublishQuotaHoldUsesSamePeriodDeltaAndCrossPeriodFullUnits",
    "TestConcurrentCrossPeriodQuotaHoldPublishReservesCurrentExecutionCapacity",
    "TestConcurrentDueMixedPostKeepsAdmissionReservedTargetWhenRestrictionLiftExceedsQuota",
    "TestDueMixedPostUsesPartialHeadroomDeterministically",
    "TestDueMixedPostReusesReservationReleasedByCurrentRestriction",
    "TestDueCrossMonthPostUsesOnlyCurrentPeriodPartialHeadroom",
    "TestScheduledQuotaTargetSnapshotRejectsMissingMalformedAndForgedBindings",
    "TestDuePostFailsClosedForMissingMalformedAndForgedQuotaSnapshots",
    "TestDueRepeatedAccountSnapshotPreservesEveryThreadIndex",
    "TestScheduledOrdinaryRetentionFailureDoesNotAbortClaim",
    "TestDraftOrdinaryRetentionFailureDoesNotAbortClaim",
    "TestScheduledInvalidMetadataCommitsTerminalFailure",
  ],
  "./internal/paidquota": [
    "TestPostgresHoldServiceUsesScheduledQuotaSnapshotWithLegacyFallback",
  ],
  "./internal/testdbguard": [
    "TestOpenValidatedRejectsRemoteFallbackBeforeOpen",
    "TestOpenValidatedRejectsRemotePrimaryBeforeOpen",
    "TestOpenValidatedAllowsAllLoopbackMultiHost",
  ],
  "./internal/worker": [
    "TestFacebookVideoStatusWorkerTerminalCoordinatorPostgres",
    "TestTerminalPostEventOutboxClaimUsesSkipLockedAndPerPostOrder",
    "TestTerminalPostEventOutboxClaimInvalidatesEventSupersededByRetry",
    "TestTerminalPostEventOutboxClaimKeepsTerminalEventAfterSameStatusMetadataUpdate",
    "TestTerminalPostEventOutboxReleasesSmallPoolBeforeSinkIO",
    "TestTerminalPostEventOutboxStaleClaimCannotOverwriteReclaimedDelivery",
    "TestConcurrentFirstPostElectionProducesOneLoopsLifecycleSend",
    "TestWorkspaceFirstPostElectionRollbackAndHistoricalSafety",
    "TestRecoveryCampaignUsesCurrentCanonicalIdentityAndRejectsReassignedEmail",
    "TestPublishingRestrictionSameEmailOwnersPreserveEligibilityAndRecovery",
    "TestPublishingRestrictionMigration126OldWriterCapturesSameEmailOwnersForRecovery",
    "TestPublishingRestrictionMigration126HistoricalFallbackRejectsEmailTakeover",
    "TestPublishingRestrictionAudienceAndAdminCountsUseProfileWorkspace",
    "TestPublishingRestrictionBulkRetryExcludesUnknownOutcomeRecipient",
    "TestPublishingRestrictionBulkRetryIncludesDefinitiveAndPreSendFailuresOnly",
    "TestPublishingRestrictionManualRetryRejectsExpiredProviderIdempotencyWindow",
    "TestPublishingRestrictionRecipientSentTransitionAggregatesCampaignAtomically",
    "TestPublishingRestrictionStaleSendingReconcilesSentAuditWithoutReclaiming",
    "TestPublishingRestrictionStaleSendingRecoversDefinitiveFailureAsRetryable",
    "TestPublishingRestrictionDefinitiveFailureRejectsLostSendClaim",
    "TestPublishingRestrictionCampaignRefreshRejectsMissingCampaign",
    "TestPublishingRestrictionAuditFinalizationFailureReconcilesWithoutDuplicateSend",
    "TestPublishingRestrictionCanceledDefinitiveFailureRecoversAfterRecipientWriteFailure",
    "TestPublishingRestrictionRecipientSentRecoversAfterTransientCampaignRefreshFailure",
    "TestPublishingRestrictionClaimReconcilesRunningCampaignAfterPriorRefreshFailure",
    "TestPublishingRestrictionRecipientClaimSkipsRowLockedByConcurrentTransaction",
    "TestPublishingRestrictionStaleSendingTerminatesAndCannotBeReclaimed",
    "TestPublishingRestrictionStaleLinkedPreSendFailureRetriesWithFreshAuditAttempt",
    "TestPublishingRestrictionStalePreSendClaimRecoversWithoutUnknownOutcome",
    "TestPublishingRestrictionDelayedAuditSentCannotRacePastStaleReconciliation",
  ],
};
const postgresTestSelector = `^(?:${Object.values(postgresTestsByPackage).flat().join("|")})$`;

function assertUnconditional(item, label) {
  assert.equal(item.if, undefined, `${label} must not have an if condition`);
  assert.equal(
    Object.hasOwn(item, "continue-on-error"),
    false,
    `${label} must not define continue-on-error`,
  );
}

function requiredStep(job, name, jobName) {
  const matches = (job.steps || []).filter((step) => step.name === name);
  assert.equal(matches.length, 1, `${jobName} must have exactly one ${name} step`);
  assertUnconditional(matches[0], `${jobName} / ${name}`);
  return matches[0];
}

function assertPublishingRestrictionCIContracts({ packageJson, workflow }) {
  assert.equal(
    packageJson.scripts["test:publishing-restrictions"],
    publishingRestrictionScript,
  );

  const parsed = parseYaml(workflow);
  const dashboardJob = parsed.jobs?.dashboard;
  assert.ok(dashboardJob, "CI must define jobs.dashboard");
  assertUnconditional(dashboardJob, "jobs.dashboard");
  const contractsStep = requiredStep(
    dashboardJob,
    "Run publishing restriction frontend contracts",
    "jobs.dashboard",
  );
  assert.equal(contractsStep.run, "npm run test:publishing-restrictions");
  const buildStep = requiredStep(dashboardJob, "Build dashboard", "jobs.dashboard");
  assert.equal(buildStep.run, "npm run build");
  assert.ok(
    dashboardJob.steps.indexOf(contractsStep) < dashboardJob.steps.indexOf(buildStep),
    "publishing restriction contracts must run before the dashboard build",
  );

  const postgresJob = parsed.jobs?.["api-postgres-integration"];
  assert.ok(postgresJob, "CI must define jobs.api-postgres-integration");
  assertUnconditional(postgresJob, "jobs.api-postgres-integration");
  const postgresService = postgresJob.services?.postgres;
  assert.ok(postgresService, "jobs.api-postgres-integration must define its PostgreSQL service");
  assert.equal(postgresService.image, "postgres:16-alpine");
  assert.deepEqual(postgresService.ports, ["5432:5432"]);
  assert.deepEqual(postgresService.env, {
    POSTGRES_USER: "postgres",
    POSTGRES_PASSWORD: "test",
    POSTGRES_DB: "unipost_test",
  });
  assert.equal(
    postgresJob.env?.PUBLISHING_RESTRICTION_TEST_DATABASE_URL,
    "postgresql://postgres:test@127.0.0.1:5432/unipost_test?sslmode=disable",
  );

  const postgresStep = requiredStep(
    postgresJob,
    "Run publishing restriction and migration gate PostgreSQL integration tests",
    "jobs.api-postgres-integration",
  );
  const selectorMatch = postgresStep.run.match(/^test_selector='([^']+)'$/m);
  assert.ok(selectorMatch, "PostgreSQL step must assign one literal test_selector");
  assert.equal(selectorMatch[1], postgresTestSelector);
  assert.match(postgresStep.run, /^set -euo pipefail$/m);
  assert.match(
    postgresStep.run,
    /go test -tags=integration "\$package" -list "\$test_selector"/,
  );
  for (const [packageName, testNames] of Object.entries(postgresTestsByPackage)) {
    const expectedCall = [
      `assert_exact_tests ${packageName} \\`,
      ...testNames.map((testName, index) => (
        `  ${testName}${index === testNames.length - 1 ? "" : " \\"}`
      )),
    ].join("\n");
    assert.ok(
      postgresStep.run.includes(expectedCall),
      `PostgreSQL step must enumerate the exact ${packageName} test set`,
    );
  }
  assert.match(
    postgresStep.run,
    /go test -tags=integration \.\/internal\/db \.\/internal\/handler \.\/internal\/paidquota \.\/internal\/testdbguard \.\/internal\/worker \\\n\s+-run "\$test_selector" -count=1 -v/,
  );
}

test("AGENTS enforces exclusive branch and worktree ownership", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /one exclusive branch and one exclusive worktree/i);
  assert.match(agents, /must never use another conversation's branch or worktree/i);
  assert.match(agents, /verify the absolute worktree path and current branch/i);
});

test("AGENTS requires preview acceptance before dev", async () => {
  const agents = await read("AGENTS.md");
  assert.match(agents, /Draft pull request.*to `dev`/s);
  assert.match(agents, /Railway PR Environment/);
  assert.match(agents, /Vercel Preview/);
  assert.match(agents, /must not merge.*`dev`.*Preview Acceptance.*success/is);
});

test("AGENTS treats every non-success regression result as a hard stop", async () => {
  const agents = await read("AGENTS.md");
  for (const result of ["fails", "errors", "times out", "is cancelled", "is skipped", "cannot start"]) {
    assert.ok(agents.includes(result), `missing hard-stop result: ${result}`);
  }
  assert.match(agents, /exact failure message and relevant log excerpt/i);
});

test("CI covers every integration branch without production runtime targets", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  for (const branch of ["dev", "staging", "main"]) {
    assert.ok(workflow.includes(`- ${branch}`), `CI does not cover ${branch}`);
  }
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);
  assert.match(workflow, /NEXT_PUBLIC_API_URL: http:\/\/localhost:8080/);
  assert.match(
    workflow,
    /NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY:.*NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY/,
  );
});

test("CI makes the dashboard SEO regression blocking", async () => {
  const workflow = await read(".github/workflows/ci.yml");
  assert.match(
    workflow,
    /Run dashboard SEO source regression[\s\S]*npm run test:seo/,
  );
});

test("CI makes the publishing restriction contracts blocking in their required jobs", async () => {
  const packageJson = JSON.parse(await read("dashboard/package.json"));
  const workflow = await read(".github/workflows/ci.yml");
  assertPublishingRestrictionCIContracts({ packageJson, workflow });
});

test("publishing restriction CI guard rejects semantic workflow mutations", async (t) => {
  const packageJson = JSON.parse(await read("dashboard/package.json"));
  const workflow = await read(".github/workflows/ci.yml");
  const frontendStep = [
    "      - name: Run publishing restriction frontend contracts",
    "        run: npm run test:publishing-restrictions",
  ].join("\n");

  const mutations = {
    "continue-on-error frontend step": (source) => source.replace(
      frontendStep,
      `${frontendStep}\n        continue-on-error: true`,
    ),
    "expression continue-on-error frontend step": (source) => source.replace(
      frontendStep,
      `${frontendStep}\n        continue-on-error: \${{ true }}`,
    ),
    "disabled PostgreSQL job": (source) => source.replace(
      "  api-postgres-integration:\n",
      "  api-postgres-integration:\n    if: false\n",
    ),
    "expression continue-on-error PostgreSQL job": (source) => source.replace(
      "  api-postgres-integration:\n",
      "  api-postgres-integration:\n    continue-on-error: ${{ matrix.experimental }}\n",
    ),
    "frontend command in the wrong job": (source) => source
      .replace(`${frontendStep}\n\n`, "")
      .replace("\n  dashboard:\n", `\n${frontendStep}\n\n  dashboard:\n`),
    "comment-only frontend command": (source) => source.replace(
      frontendStep,
      "      # - name: Run publishing restriction frontend contracts run: npm run test:publishing-restrictions",
    ),
    "unanchored PostgreSQL selector": (source) => source.replace(
      "test_selector='^(?:",
      "test_selector='(?:",
    ),
  };

  for (const [name, mutate] of Object.entries(mutations)) {
    await t.test(name, () => {
      assert.throws(
        () => assertPublishingRestrictionCIContracts({ packageJson, workflow: mutate(workflow) }),
        undefined,
        `${name} must be rejected`,
      );
    });
  }
});

test("Preview Acceptance installs dashboard dependencies before release contracts", async () => {
  const workflow = await read(".github/workflows/preview-acceptance.yml");
  const previewJob = parseYaml(workflow).jobs?.preview;
  assert.ok(previewJob, "Preview Acceptance must define jobs.preview");

  const installStep = requiredStep(
    previewJob,
    "Install dashboard dependencies",
    "jobs.preview",
  );
  assert.equal(installStep["working-directory"], "dashboard");
  assert.match(installStep.run, /^set -euo pipefail$/m);
  assert.match(installStep.run, /echo "dashboard-dependencies" > \.\.\/artifacts\/preview\/current-stage/);
  assert.match(installStep.run, /npm ci 2>&1 \| tee \.\.\/artifacts\/preview\/npm-ci\.log/);

  const contractsStep = requiredStep(
    previewJob,
    "Validate release contracts",
    "jobs.preview",
  );
  assert.match(contractsStep.run, /^set -euo pipefail$/m);
  assert.match(contractsStep.run, /echo "release-contracts" > artifacts\/preview\/current-stage/);
  assert.match(
    contractsStep.run,
    /node --test scripts\/preview\/\*\.test\.mjs 2>&1 \| tee artifacts\/preview\/release-contracts\.log/,
  );

  assert.ok(
    previewJob.steps.indexOf(installStep) < previewJob.steps.indexOf(contractsStep),
    "dashboard dependencies must be installed before release contracts load js-yaml",
  );
  assert.equal(
    previewJob.steps.filter((step) => /(?:^|\s)npm ci(?:\s|$)/.test(step.run || "")).length,
    1,
    "Preview Acceptance must install dashboard dependencies exactly once",
  );
});

test("Preview Acceptance is fail-closed and tied to the exact PR head", async () => {
  const workflow = await read(".github/workflows/preview-acceptance.yml");
  const ciGates = await read("docs/ci-gates.md");
  for (const branch of ["dev", "staging"]) {
    assert.ok(
      workflow.includes(`- ${branch}`),
      `Preview workflow does not cover ${branch} pull requests`,
    );
  }
  for (const event of [
    "opened",
    "synchronize",
    "reopened",
    "ready_for_review",
    "labeled",
    "unlabeled",
    "closed",
  ]) {
    assert.ok(workflow.includes(event), `Preview workflow omits ${event}`);
  }
  assert.match(workflow, /github\.event\.pull_request\.head\.sha/);
  assert.match(workflow, /github\.event\.pull_request\.head\.repo\.full_name == github\.repository/);
  assert.match(workflow, /startsWith\(github\.event\.pull_request\.head\.ref, 'dev-'\)/);
  assert.match(workflow, /startsWith\(github\.event\.pull_request\.head\.ref, 'hotfix-'\)/);
  assert.match(workflow, /startsWith\(github\.event\.pull_request\.head\.ref, 'codex\/staging-'\)/);
  assert.equal(
    [...workflow.matchAll(/startsWith\(github\.event\.pull_request\.head\.ref, 'codex\/pr270-'\)/g)].length,
    2,
    "the PR 270 hardening branch must be accepted for both preview and cleanup",
  );
  assert.match(workflow, /vercel@50\.26\.1/);
  assert.match(workflow, /--prebuilt[\s\S]*--archive=tgz/);
  assert.match(
    workflow,
    /Build the Vercel Preview[\s\S]*export NEXT_PUBLIC_CLERK_PUBLISHABLE_KEY="\$\{\{ vars\.NEXT_PUBLIC_CLERK_DEVELOPMENT_PUBLISHABLE_KEY \}\}"[\s\S]*vercel@50\.26\.1 build/,
    "the prebuilt Preview must receive Clerk's public key explicitly because Vercel does not download sensitive values",
  );
  assert.match(workflow, /github\.run_id/);
  assert.match(workflow, /github\.run_attempt/);
  assert.match(workflow, /RAILWAY_API_TOKEN:.*secrets\.RAILWAY_API_TOKEN/);
  assert.match(workflow, /RAILWAY_PROJECT_ID:.*vars\.RAILWAY_PROJECT_ID/);
  assert.match(workflow, /railway-deployments\.mjs/);
  assert.doesNotMatch(
    workflow,
    /--cwd dashboard/,
    "the Vercel project already has dashboard as its Root Directory",
  );
  assert.match(workflow, /test:regression:preview/);
  assert.match(
    workflow,
    /Run deployed preview regression[\s\S]*VERCEL_AUTOMATION_BYPASS_SECRET:.*secrets\.VERCEL_AUTOMATION_BYPASS_SECRET/,
  );
  assert.doesNotMatch(workflow, /vercel-share-link\.mjs/);
  assert.doesNotMatch(workflow, /VERCEL_SHAREABLE_URL/);
  assert.match(
    workflow,
    /DASHBOARD_BASE_URL: https:\/\/\$\{\{ env\.PREVIEW_ALIAS_HOST \}\}/,
  );
  assert.doesNotMatch(
    workflow,
    /Run deployed preview regression[\s\S]*DASHBOARD_BASE_URL: \$\{\{ steps\.vercel\.outputs\.deployment_url \}\}/,
    "authenticated navigation must stay on the build-time Preview app host",
  );
  assert.match(workflow, /vercel-alias-cleanup\.mjs/);
  assert.match(
    workflow,
    /cleanup-preview-alias:[\s\S]*ref: \$\{\{ github\.event\.pull_request\.head\.sha \}\}/,
  );
  assert.match(workflow, /preview-failure-drill/);
  assert.match(workflow, /if: always\(\)/);
  assert.match(workflow, /if: failure\(\)/);
  assert.doesNotMatch(workflow, /https:\/\/api\.unipost\.dev/);
  assert.doesNotMatch(workflow, /pk_live_/);
  assert.match(
    ciGates,
    /Railway `preview-base` service `preview-api`[\s\S]*`CLERK_SECRET_KEY`[\s\S]*`sk_test_`/,
    "isolated Railway PR APIs must inherit the Development Clerk verifier secret",
  );

  const previewConfig = await read("dashboard/playwright.preview.config.ts");
  assert.doesNotMatch(previewConfig, /VERCEL_SHAREABLE_URL/);
  assert.doesNotMatch(previewConfig, /extraHTTPHeaders/);
  assert.match(previewConfig, /seo-preview\.spec\.ts/);
  assert.match(
    previewConfig,
    /trace:\s*"off"/,
    "preview traces must stay disabled because request headers contain the automation bypass secret",
  );

  const previewTest = await read("dashboard/tests/regression/preview-environment.spec.ts");
  assert.doesNotMatch(previewTest, /shareableURL/);
  assert.match(previewTest, /x-vercel-protection-bypass/);
  assert.match(previewTest, /x-vercel-set-bypass-cookie/);
  assert.match(
    previewTest,
    /VERCEL_AUTOMATION_BYPASS_SECRET\?\.trim\(\)/,
    "preview tests must strip accidental whitespace from the automation bypass secret",
  );

  const seoPreviewTest = await read(
    "dashboard/tests/regression/seo-preview.spec.ts",
  );
  assert.match(seoPreviewTest, /\/sitemap\.xml/);
  assert.match(seoPreviewTest, /maxRedirects:\s*0/);
  assert.match(seoPreviewTest, /noindex/i);
  assert.match(seoPreviewTest, /UniPost \| Social Media Posting API for Developers/);
  assert.match(
    seoPreviewTest,
    /VERCEL_AUTOMATION_BYPASS_SECRET\?\.trim\(\)/,
    "SEO preview tests must strip accidental whitespace from the automation bypass secret",
  );
  assert.doesNotMatch(
    seoPreviewTest,
    /x-vercel-set-bypass-cookie/,
    "SEO API requests must not ask Vercel to set a bypass cookie because the cookie handshake is a same-path redirect",
  );

  const previewManifest = await read("scripts/preview/write-manifest.mjs");
  assert.match(
    previewManifest,
    /dev-\|hotfix-/,
    "preview manifests must support required hotfix sync pull requests",
  );

  const proxy = await read("dashboard/src/proxy.ts");
  assert.match(proxy, /pathname === "\/__unipost-preview\.json"/);
  assert.match(
    proxy,
    /isVercelPreviewHost\(hostname\)[\s\S]*pathname === "\/"[\s\S]*url\.pathname = "\/marketing"/,
    "a combined Vercel Preview host must serve the public homepage instead of Clerk-protecting the app root",
  );
});

test("ordinary dashboard regression excludes deployed preview-only acceptance", async () => {
  const config = await read("dashboard/playwright.regression.config.ts");
  assert.match(
    config,
    /testIgnore:\s*\[[\s\S]*preview-environment\.spec\.ts/,
    "dashboard regression would collect the preview-only spec without its required deployment identity",
  );
  assert.match(config, /testIgnore:\s*\[[\s\S]*seo-preview\.spec\.ts/);
});
