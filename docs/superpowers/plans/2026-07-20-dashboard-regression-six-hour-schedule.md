# Dashboard Regression Six-Hour Schedule Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run the scheduled production Dashboard Regression four times per day instead of once per hour.

**Architecture:** Keep the existing GitHub Actions workflow and change only its cron expression. Preserve manual dispatch, the Playwright regression suite, artifacts, environment selection, and failure notification behavior.

**Tech Stack:** GitHub Actions workflow YAML, Ruby/Psych for YAML syntax validation, Git

---

### Task 1: Change and verify the regression schedule

**Files:**
- Modify: `.github/workflows/dashboard-regression.yml:5`
- Reference: `docs/superpowers/specs/2026-07-20-dashboard-regression-six-hour-schedule-design.md`

- [ ] **Step 1: Run a failing schedule assertion**

Run:

```bash
ruby -e 'text = File.read(".github/workflows/dashboard-regression.yml"); abort "expected six-hour cron" unless text.include?(%q{cron: "15 */6 * * *"})'
```

Expected: exit non-zero with `expected six-hour cron`, proving the current hourly schedule does not meet the requirement.

- [ ] **Step 2: Make the minimal workflow change**

Change the schedule block to:

```yaml
on:
  schedule:
    - cron: "15 */6 * * *"
  workflow_dispatch:
```

Do not change any job, environment variable, test command, artifact, or notification configuration.

- [ ] **Step 3: Verify the new schedule assertion passes**

Run:

```bash
ruby -e 'text = File.read(".github/workflows/dashboard-regression.yml"); abort "expected six-hour cron" unless text.include?(%q{cron: "15 */6 * * *"}); abort "hourly cron still present" if text.include?(%q{cron: "15 * * * *"})'
```

Expected: exit 0 with no output.

- [ ] **Step 4: Validate YAML syntax and diff scope**

Run:

```bash
ruby -e 'require "yaml"; YAML.parse_file(".github/workflows/dashboard-regression.yml"); puts "workflow yaml valid"'
git diff --check
git diff -- .github/workflows/dashboard-regression.yml
```

Expected: `workflow yaml valid`, `git diff --check` exits 0, and the workflow diff contains only the cron change from `15 * * * *` to `15 */6 * * *`.

- [ ] **Step 5: Commit the workflow change**

Before staging or committing, verify the absolute worktree path is `/Users/xiaoboyu/.config/superpowers/worktrees/unipost/dev-preview-environment-guardrails` and the branch is `dev-preview-environment-guardrails`.

Run:

```bash
git add .github/workflows/dashboard-regression.yml
git diff --cached --check
git commit -m "ci: run dashboard regression every six hours"
```

Expected: one focused commit containing only `.github/workflows/dashboard-regression.yml`.

### Task 2: Publish through the normal development gate

**Files:**
- Audit: `.github/workflows/dashboard-regression.yml`
- Audit: `docs/superpowers/specs/2026-07-20-dashboard-regression-six-hour-schedule-design.md`
- Audit: `docs/superpowers/plans/2026-07-20-dashboard-regression-six-hour-schedule.md`

- [ ] **Step 1: Fetch and audit the exact branch delta**

Run:

```bash
git fetch origin
git log --oneline origin/dev..HEAD
git diff --name-status origin/dev...HEAD
```

Expected: every unique commit and file is identified. Stop before push or PR if the delta contains unrelated, unfinished, or unaccepted work.

- [ ] **Step 2: Push only the conversation-owned branch**

Before pushing, verify the absolute worktree path and branch again, then run:

```bash
git push origin dev-preview-environment-guardrails
```

Expected: the push updates only `origin/dev-preview-environment-guardrails`.

- [ ] **Step 3: Open or update a Draft PR to `dev`**

The PR must describe the old and new UTC schedule and list the local verification evidence. Do not merge while any required check, Preview Acceptance, deployment, or regression is pending or unsuccessful.

- [ ] **Step 4: Monitor exact-SHA gates**

Monitor GitHub CI, Railway PR Environment, Vercel Preview, deployed regression, and Codex browser acceptance for the PR head SHA. Any failure, error, timeout, cancellation, skip, missing result, or SHA mismatch is a hard stop and must be reported with the required failure evidence.

- [ ] **Step 5: Audit and merge to `dev` after every gate passes**

Repeat the commit/file audit immediately before merge. Merge only when the exact PR head SHA has passed every required gate and the delta contains no unrelated or unfinished work.

- [ ] **Step 6: Verify the official development state**

Wait for triggered development checks and deployments to complete. Confirm the workflow on `origin/dev` contains exactly `cron: "15 */6 * * *"`, then report development acceptance to the user. Do not promote to `staging` or `main` without the user's separate approval.
