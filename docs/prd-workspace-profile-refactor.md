# UniPost — Workspace + Profile Architecture Refactor PRD
**Rename Project to Profile, introduce Workspace as the top-level security boundary**
Version 1.1 | April 2026

---

## 1. Background

### 1.1 Current Problem

The existing `Project` entity serves two conflicting roles:
- Security isolation boundary (API Key binding, data isolation)
- Account grouping (brand separation)

When a user wants to publish a post across accounts in different "Projects", they must manually switch Projects and recreate the post — violating the core value of "publish once, post everywhere".

### 1.2 Solution

Introduce `Workspace` as the top-level security boundary. Rename the existing `Project` concept to `Profile`, making it a lightweight brand-grouping container.

---

## 2. New Hierarchy

```
User
  └── Workspace (top-level, security isolation boundary)
        ├── API Keys          ← Workspace level
        ├── Social Posts      ← Workspace level
        ├── Billing / Quota   ← Workspace level
        ├── Media             ← Workspace level
        │
        ├── Profile A (brand grouping, lightweight)
        │     └── Social Accounts (X, LinkedIn, Bluesky...)
        │
        └── Profile B
              └── Social Accounts (YouTube, TikTok...)
```

### 2.1 Responsibility Breakdown

**Workspace:**
- Security boundary; data isolated between Workspaces
- Owns API Keys (one API Key maps to one entire Workspace)
- Quota / Billing computed at Workspace level
- White-label platform credentials configured at Workspace level
- Per-account monthly limit configured at Workspace level

**Profile (formerly Project):**
- Lightweight brand-grouping container
- Owns Social Accounts
- One Workspace can have multiple Profiles
- Does NOT own independent API Keys
- Customizable name, branding (logo, display name, primary color)

**Social Post:**
- Belongs to Workspace, not to any single Profile
- At creation time, user can select any accounts from any Profile within the Workspace
- Each post's platform_results execute independently per account_id

**API Key:**
- Bound to Workspace
- Can operate on all accounts across all Profiles in the Workspace
- Auth check: `api_key.workspace_id == social_account.profile.workspace_id`

---

## 3. Cross-Profile Posting Scenario

```
Scenario:
  Workspace "UniPost"
    ├── Profile "Official"  → X @unipostdev, Bluesky unipostdev.bsky
    └── Profile "Personal"  → X @yuxiaobohit, LinkedIn Xiaobo Yu

User creates one post, selecting:
  ☑ Official / X @unipostdev
  ☑ Personal / LinkedIn Xiaobo Yu

→ Single operation, publishes to accounts across two Profiles
→ Post belongs to Workspace, not to any one Profile
→ Each platform executes independently (Worker), no interference
```

Per-platform override keyed by account_id:
```
  Official / X        → [Edit override] (custom copy)
  Personal / LinkedIn → [Edit override] (custom copy)
```

---

## 4. Database Changes

### 4.1 New workspaces table

```sql
CREATE TABLE workspaces (
  id                        TEXT PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id                   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name                      TEXT NOT NULL DEFAULT 'Default',
  per_account_monthly_limit INTEGER,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspaces_user_id ON workspaces(user_id);
```

Note: `per_account_monthly_limit` moves here from the old projects table (added in migration 024).

### 4.2 projects table renamed to profiles

```sql
-- Step 1: Rename the table (preserves all existing data)
ALTER TABLE projects RENAME TO profiles;

-- Step 2: Add workspace_id (populated by migration before dropping owner_id)
ALTER TABLE profiles
  ADD COLUMN workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE;

-- Step 3: Populate workspace_id from owner_id via workspaces lookup
-- (see Section 4.8 for full migration)

-- Step 4: Make workspace_id NOT NULL after population
ALTER TABLE profiles ALTER COLUMN workspace_id SET NOT NULL;

-- Step 5: Drop owner_id (now redundant — ownership flows through workspace)
ALTER TABLE profiles DROP COLUMN owner_id;

-- Step 6: Drop per_account_monthly_limit (moved to workspaces)
ALTER TABLE profiles DROP COLUMN per_account_monthly_limit;

-- Retained columns: id, name, branding_logo_url, branding_display_name,
--   branding_primary_color, workspace_id, created_at, updated_at
-- Note: projects.mode was already dropped in migration 022.

CREATE INDEX idx_profiles_workspace_id ON profiles(workspace_id);
```

### 4.3 api_keys: project_id → workspace_id

```sql
ALTER TABLE api_keys RENAME COLUMN project_id TO workspace_id;
-- FK target changes from profiles(id) to workspaces(id)
-- (handled via DROP + ADD CONSTRAINT in migration)

DROP INDEX idx_api_keys_project_id;
CREATE INDEX idx_api_keys_workspace_id ON api_keys(workspace_id);
```

### 4.4 social_accounts: project_id → profile_id

```sql
ALTER TABLE social_accounts RENAME COLUMN project_id TO profile_id;

DROP INDEX idx_social_accounts_project_id;
CREATE INDEX idx_social_accounts_profile_id ON social_accounts(profile_id);

-- Also update indexes from migration 020 that reference project_id:
-- social_accounts_managed_unique_idx  → recreate with profile_id
-- social_accounts_ext_user_idx        → recreate with profile_id
```

### 4.5 social_posts: project_id → workspace_id

```sql
ALTER TABLE social_posts RENAME COLUMN project_id TO workspace_id;
-- FK target changes from profiles(id) to workspaces(id)

-- Recreate indexes from migration 018 and 019:
-- social_posts_project_idempotency_uniq → workspace_id
-- social_posts_project_created_idx      → workspace_id
```

### 4.6 Other tables: project_id → workspace_id

```sql
-- platform_credentials: project_id → workspace_id
ALTER TABLE platform_credentials RENAME COLUMN project_id TO workspace_id;
-- UNIQUE(project_id, platform) → UNIQUE(workspace_id, platform)

-- subscriptions: project_id → workspace_id (billing at Workspace level)
ALTER TABLE subscriptions RENAME COLUMN project_id TO workspace_id;

-- usage: project_id → workspace_id
ALTER TABLE usage RENAME COLUMN project_id TO workspace_id;
-- UNIQUE(project_id, period) → UNIQUE(workspace_id, period)

-- webhooks: project_id → workspace_id
ALTER TABLE webhooks RENAME COLUMN project_id TO workspace_id;

-- media: project_id → workspace_id
ALTER TABLE media RENAME COLUMN project_id TO workspace_id;
```

### 4.7 Other tables: project_id → profile_id

```sql
-- oauth_states: project_id → profile_id
-- (OAuth connects a social account to a specific Profile)
ALTER TABLE oauth_states RENAME COLUMN project_id TO profile_id;

-- connect_sessions: project_id → profile_id
-- (Connect sessions create social accounts under a specific Profile)
ALTER TABLE connect_sessions RENAME COLUMN project_id TO profile_id;
```

### 4.8 users table: update FK columns

```sql
-- default_project_id → default_profile_id
-- last_project_id → last_profile_id
ALTER TABLE users RENAME COLUMN default_project_id TO default_profile_id;
ALTER TABLE users RENAME COLUMN last_project_id TO last_profile_id;
-- FK targets remain pointing at profiles(id) (table was renamed, not recreated)
```

### 4.9 Migration Strategy (zero user cost)

```sql
-- Current state: 3 registered users, 0 API Keys in use
-- Clean migration, no backwards compatibility needed

-- Step 1: Create workspaces table
-- Step 2: For each existing user, create a default Workspace
INSERT INTO workspaces (id, user_id, name)
SELECT gen_random_uuid(), id, 'Default'
FROM users;

-- Step 3: Rename projects → profiles, add workspace_id column
-- Step 4: Populate workspace_id by joining through owner_id → user_id → workspace
UPDATE profiles p
SET workspace_id = w.id
FROM workspaces w
WHERE p.owner_id = w.user_id;

-- Step 5: Make workspace_id NOT NULL, drop owner_id
-- Step 6: For workspace-level tables (api_keys, social_posts, etc.),
--         populate workspace_id from the profile's workspace_id
-- Step 7: For profile-level tables (social_accounts, oauth_states, connect_sessions),
--         simply rename project_id → profile_id (FK target is the same table, just renamed)
-- Step 8: Drop old indexes, create new ones
-- Step 9: Update users table column names
```

---

## 5. API Changes

### 5.1 New Workspace API

```
GET    /v1/workspaces                    List current user's Workspaces (typically just one)
PATCH  /v1/workspaces/:id               Update Workspace name and settings
```

Note: Currently each user has only one Workspace. Multi-Workspace creation is out of scope.

### 5.2 Profile API (renamed from Project API)

```
Previously:
  GET    /v1/projects
  POST   /v1/projects
  GET    /v1/projects/:id
  PATCH  /v1/projects/:id
  DELETE /v1/projects/:id

Now:
  GET    /v1/profiles
  POST   /v1/profiles
  GET    /v1/profiles/:id
  PATCH  /v1/profiles/:id
  DELETE /v1/profiles/:id
```

### 5.3 Social Accounts API Changes

```
Previously: accounts queried/associated via project_id
Now: accounts associated via profile_id, accessed through Workspace-level API Key

GET /v1/accounts              → Returns ALL accounts across ALL Profiles in Workspace
GET /v1/accounts?profile_id=  → Filter by Profile
```

### 5.4 Social Posts API Changes

```
POST /v1/social-posts
  body: {
    caption: "...",
    account_ids: ["sa_xxx", "sa_yyy"],  // Can span Profiles within same Workspace
    scheduled_at: "...",                 // Optional
    publish_mode: "now" | "schedule" | "draft"
  }

Auth check:
  - api_key's workspace_id
  - Each account_id's social_account.profile.workspace_id
  - Must match, otherwise 403

Note: No project_id / profile_id parameter needed.
      Post belongs to Workspace; account association determined by account_ids.
```

### 5.5 Deprecated Parameters

```
Removed:
  - All project_id parameters across all APIs
  - POST /v1/social-posts profile_id parameter (if any)

Added:
  - GET /v1/accounts?profile_id= (filter accounts by Profile)
  - GET /v1/social-posts?profile_id= (filter posts by Profile, via account JOIN)
```

Note on `?profile_id=` post filtering: Since posts belong to Workspace (not Profile),
this filter requires a JOIN through `social_post_results → social_accounts → profiles`.
Acceptable at current scale; revisit indexing if data volume grows significantly.

---

## 6. Dashboard Changes

### 6.1 Navigation Structure

```
Previously:
  Left sidebar top: Project selector (dropdown)
  All page URLs: /projects/[id]/xxx

Now:
  Left sidebar top: Workspace name (fixed, no switching needed)
  Sidebar addition: Profiles section (shows all Profiles for account management)
  URL structure:
    /workspace/xxx       (Workspace-level: Posts, API Keys, Analytics)
    /profiles/[id]       (Profile-level: Connections, account management)
```

### 6.2 Connections Page

```
Previously:
  Shows accounts for the current Project

Now:
  Shows Profile selector + accounts per Profile
  User can switch Profiles or see aggregated view of all Profiles

  UI mockup:
  ┌─────────────────────────────────────┐
  │ Connections                         │
  │                                     │
  │ Profile: [Official ▼]  [+ New]      │
  │                                     │
  │ Official Profile                    │
  │ ├── X @unipostdev        ● Active   │
  │ └── Bluesky unipostdev   ● Active   │
  │                                     │
  │ Personal Profile                    │
  │ ├── X @yuxiaobohit       ● Active   │
  │ └── LinkedIn Xiaobo Yu   ● Active   │
  │                                     │
  │ [+ Connect account]                 │
  └─────────────────────────────────────┘
```

### 6.3 Create Post Drawer

```
Right-side "Post to" section groups accounts by Profile:

  Post to

  Profile: Official
  ☑ X  @unipostdev
  ☑ 🦋 unipostdev.bsky.social

  Profile: Personal
  ☐ X  @yuxiaobohit
  ☑ 💼 Xiaobo Yu (LinkedIn)

Notes:
  - Cross-Profile account selection enabled
  - Per-platform override keyed by account_id
  - Single Post covers all selected accounts
```

### 6.4 Settings Pages

```
Previously: Project Settings (project_id level)

Now: Two levels of Settings:
  Workspace Settings:
    - Workspace name
    - White-label credential configuration
    - Per-account monthly limit
    - Billing
    - Danger zone (delete Workspace)

  Profile Settings (click into a specific Profile):
    - Profile name
    - Profile branding (logo, display name, primary color)
    - Danger zone (delete Profile)
```

---

## 7. Authorization Changes

### 7.1 Dashboard (Clerk Session Auth)

```
Previously:
  GetProjectByIDAndOwner(projectID, userID) — single-hop check

Now:
  Workspace routes: GetWorkspaceByIDAndOwner(workspaceID, userID) — single-hop
  Profile routes:   GetProfileByIDAndWorkspaceOwner(profileID, userID) — single query
                    with JOIN: profiles → workspaces WHERE workspaces.user_id = $userID
```

### 7.2 API Key Auth Middleware

```
Previously:
  auth.APIKeyMiddleware injects ProjectIDKey into context
  Handlers call auth.GetProjectID(ctx)

Now:
  auth.APIKeyMiddleware injects WorkspaceIDKey into context
  Handlers call auth.GetWorkspaceID(ctx)
```

---

## 8. Terminology Replacement Checklist

All code, UI text, and API docs:

```
Go backend:
  Project struct      → Workspace struct / Profile struct
  project_id field    → workspace_id / profile_id
  ProjectID variable  → WorkspaceID / ProfileID
  GetProject()        → GetWorkspace() / GetProfile()
  CreateProject()     → CreateWorkspace() / CreateProfile()
  ProjectIDKey        → WorkspaceIDKey

API layer:
  /v1/projects        → /v1/profiles
  project_id param    → profile_id param (or removed)

Frontend (Next.js):
  /projects/[id]      → URL structure redesigned
  "Project" text      → "Profile"
  Project selector    → Profile selector
  "project_id"        → "profile_id"

Dashboard UI text:
  "Create project"    → "Create profile"
  "My projects"       → "My profiles"
  "Project settings"  → "Profile settings"
  "Select project"    → "Select profile"
```

---

## 9. PR Breakdown

```
PR1: Database Migration
  - Create workspaces table
  - Rename projects → profiles, add workspace_id
  - api_keys, social_posts, webhooks, platform_credentials,
    subscriptions, usage, media → workspace_id
  - social_accounts, oauth_states, connect_sessions → profile_id
  - users: default_project_id → default_profile_id, last_project_id → last_profile_id
  - Migrate data for existing 3 users
  - Recreate all affected indexes and unique constraints
  Estimate: 1 day

PR2: Backend API Refactor
  - Workspace handlers (CRUD)
  - /v1/projects → /v1/profiles
  - All project_id → workspace_id / profile_id
  - Auth middleware: ProjectIDKey → WorkspaceIDKey
  - social-posts auth check at workspace level
  - cross-Profile account_ids validation logic
  - Profile auth: JOIN-based ownership check
  Estimate: 2 days

PR3: Dashboard Frontend Refactor
  - URL structure redesign
  - Connections page: Profile-grouped display
  - Create Post drawer: Profile-grouped account selector
  - Settings page: Workspace Settings + Profile Settings
  - All "Project" text → "Profile"
  - Sidebar navigation updates
  Estimate: 3 days

PR4: API Documentation Update
  - docs.unipost.dev: all project-related content
  - platform-capabilities.ts updates
  Estimate: 0.5 days

Total: ~6.5 working days
```

---

## 10. Acceptance Criteria

**Data layer:**
```
□ workspaces table exists; each user has one default Workspace
□ profiles table replaces projects, includes workspace_id FK
□ api_keys linked via workspace_id (not profile_id)
□ social_posts linked via workspace_id
□ social_accounts linked via profile_id
□ oauth_states linked via profile_id
□ connect_sessions linked via profile_id
□ media linked via workspace_id
□ usage linked via workspace_id
□ subscriptions linked via workspace_id (UNIQUE on workspace_id)
□ webhooks, platform_credentials linked via workspace_id
□ users table: default_profile_id, last_profile_id columns
□ All indexes and unique constraints recreated with new column names
□ Existing 3 users' data correctly migrated
□ social_post_results rows from a single post can span multiple Profiles (verified)
```

**API layer:**
```
□ GET /v1/profiles returns Profiles under current Workspace
□ POST /v1/social-posts account_ids can span Profiles
□ Cross-Profile account_ids allowed within same Workspace
□ Mixed-Workspace account_ids returns 403
□ API Key effective at Workspace level (not bound to single Profile)
□ auth.GetWorkspaceID(ctx) replaces auth.GetProjectID(ctx)
```

**Dashboard layer:**
```
□ Connections page groups accounts by Profile
□ Create Post drawer groups account selector by Profile
□ Cross-Profile account selection works for single post
□ Per-platform override keyed by account_id (not by Profile)
□ All UI text: "Project" → "Profile"
□ Settings split into Workspace-level and Profile-level
□ URL structure uses /workspace/xxx and /profiles/[id]
```

---

## 11. Out of Scope

```
❌ Multi-Workspace support (each user currently has one Workspace)
❌ Workspace member management (team collaboration — future PRD)
❌ Profile-level independent Quota limits
❌ API Key scoping to specific Profiles (currently full Workspace access)
```

---

*UniPost Workspace + Profile Architecture Refactor PRD v1.1*
*Migration cost: minimal (3 registered users, 0 API Keys in use)*
*Estimated effort: 4 PRs, ~6.5 working days*
