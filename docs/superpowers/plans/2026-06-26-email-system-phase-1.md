# Email System Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the PRD's Phase 1 email event registry and template-contract documentation without changing live email sending behavior.

**Architecture:** Add a small backend registry package that defines canonical user-email events, Loops template environment variables, delivery classes, ownership, idempotency, and external Loops workflow audit expectations. Add `docs/email-templates.md` as the human-readable contract source, and add tests that keep the registry, current `api/cmd/api/main.go` Loops wiring, and docs aligned.

**Tech Stack:** Go backend, standard `testing` package, Markdown documentation, `rg`-verified source consistency.

---

### Task 1: Backend Email Registry

**Files:**
- Create: `api/internal/emailregistry/registry.go`
- Create: `api/internal/emailregistry/registry_test.go`

- [ ] **Step 1: Write failing tests for required Phase 1 events**

Test that the registry exposes every canonical event from `docs/prd-email-system-consolidation.md`, including existing Loops template env vars from `api/cmd/api/main.go` and newly planned Phase 1 env vars.

- [ ] **Step 2: Run registry tests and verify they fail**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -count=1`

- [ ] **Step 3: Implement the registry**

Define `Event`, `DeliveryClass`, and `Registry()` in `api/internal/emailregistry/registry.go`.

- [ ] **Step 4: Run registry tests and verify they pass**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -count=1`

### Task 2: Template Contracts Doc and Source Consistency

**Files:**
- Create: `docs/email-templates.md`
- Modify: `api/internal/emailregistry/registry_test.go`

- [ ] **Step 1: Write failing tests for docs/source consistency**

Add tests that:
- each registry entry with a `LOOPS_*_TRANSACTIONAL_ID` appears in `docs/email-templates.md`
- each currently wired `LOOPS_*_TRANSACTIONAL_ID` in `api/cmd/api/main.go` has a registry entry
- external Loops audit entries exist for `user_signed_up`, `post_failed`, and `plan_changed`

- [ ] **Step 2: Run registry tests and verify they fail**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -count=1`

- [ ] **Step 3: Add template contract documentation**

Create `docs/email-templates.md` with one contract per registry entry, plus the Loops dashboard audit checklist.

- [ ] **Step 4: Run registry tests and verify they pass**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -count=1`

### Task 3: Validation and Commit

**Files:**
- `api/internal/emailregistry/registry.go`
- `api/internal/emailregistry/registry_test.go`
- `docs/email-templates.md`
- `docs/superpowers/plans/2026-06-26-email-system-phase-1.md`

- [ ] **Step 1: Run focused backend tests**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry -count=1`

- [ ] **Step 2: Run broader backend tests for touched package dependencies**

Run: `cd api && GOCACHE=/tmp/unipost-go-build go test ./internal/emailregistry ./internal/loops ./internal/quotaemail -count=1`

- [ ] **Step 3: Check formatting and diff hygiene**

Run: `gofmt -w api/internal/emailregistry/registry.go api/internal/emailregistry/registry_test.go`
Run: `git diff --check`

- [ ] **Step 4: Commit Phase 1**

Run: `git add api/internal/emailregistry docs/email-templates.md docs/superpowers/plans/2026-06-26-email-system-phase-1.md && git commit -m "feat: add email event registry"`
