// UniPost Go SDK Validation Test
//
// Setup:
//   1. go mod tidy
//   2. UNIPOST_API_KEY=up_live_xxx go run main.go
//   3. UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> go run main.go

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/unipost-dev/sdk-go/unipost"
)

var (
	passed            int
	failed            int
	failures          []string
	createdPostIDs    []string
	createdWebhookIDs []string
)

func test(name string, fn func() error) {
	fmt.Printf("  %s ... ", name)
	if err := fn(); err != nil {
		fmt.Printf("❌ FAIL — %s\n", err)
		failed++
		failures = append(failures, fmt.Sprintf("%s: %s", name, err))
	} else {
		fmt.Println("✅ PASS")
		passed++
	}
}

func section(title string) {
	fmt.Printf("\n%s\n  %s\n%s\n", "──────────────────────────────────────────────────", title, "──────────────────────────────────────────────────")
}

func main() {
	fmt.Println("\n╔══════════════════════════════════════════════════╗")
	fmt.Println("║     sdk-go — API Validation Test                 ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")

	apiKey := os.Getenv("UNIPOST_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ Please set UNIPOST_API_KEY environment variable")
		os.Exit(1)
	}

	testAccountID := os.Getenv("TEST_ACCOUNT_ID")
	ctx := context.Background()

	client := unipost.NewClient(unipost.WithAPIKey(apiKey))

	// ── 1. Accounts ───────────────────────────────────────────────────────────
	section("1. Accounts — list connected social accounts")

	var accounts []unipost.SocialAccount
	test("Accounts.List()", func() error {
		accs, err := client.Accounts.List(ctx, nil)
		if err != nil {
			return err
		}
		accounts = accs
		return nil
	})
	test("Accounts.ListPage()", func() error {
		page, err := client.Accounts.ListPage(ctx, nil)
		if err != nil {
			return err
		}
		if page.Meta.Total == nil || page.Meta.Limit == nil {
			return fmt.Errorf("expected meta.total and meta.limit")
		}
		return nil
	})

	if len(accounts) > 0 {
		fmt.Printf("\n  Found %d connected accounts:\n", len(accounts))
		for _, a := range accounts {
			name := a.AccountName
			if name == "" {
				name = a.ID
			}
			fmt.Printf("    • [%-10s] %s  (id: %s)\n", a.Platform, name, a.ID)
		}
		if testAccountID == "" {
			for _, acc := range accounts {
				if acc.Platform == "bluesky" {
					testAccountID = acc.ID
					break
				}
			}
			if testAccountID == "" {
				testAccountID = accounts[0].ID
			}
			fmt.Printf("\n  Using TEST_ACCOUNT_ID=%s for safe draft/scheduled tests\n", testAccountID)
		}
	}

	// ── 2. Profiles — raw API smoke ───────────────────────────────────────────
	section("2. Profiles — list & filter accounts by profile")

	var profiles []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	test("GET /v1/profiles", func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.unipost.dev/v1/profiles", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		var envelope struct {
			Data []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
			return err
		}
		profiles = envelope.Data
		return nil
	})
	if len(profiles) > 0 {
		fmt.Printf("\n  Found %d profiles:\n", len(profiles))
		for _, p := range profiles {
			fmt.Printf("    • %s  (id: %s)\n", p.Name, p.ID)
		}
		firstProfile := profiles[0]
		test(fmt.Sprintf("GET /v1/social-accounts?profile_id=%s...", firstProfile.ID[:8]), func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.unipost.dev/v1/social-accounts?profile_id="+firstProfile.ID, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Authorization", "Bearer "+apiKey)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			var envelope struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
				return err
			}
			fmt.Printf("    → %d accounts in profile %q\n", len(envelope.Data), firstProfile.Name)
			return nil
		})
	}

	// ── 3. Webhooks — signature + CRUD ────────────────────────────────────────
	section("3. Webhooks — signature verification & subscription CRUD")

	test("VerifyWebhookSignature()", func() error {
		payload := []byte(`{"event":"post.published","data":{"id":"post_test_123"}}`)
		secret := "whsec_test_local"
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !unipost.VerifyWebhookSignature(payload, signature, secret) {
			return fmt.Errorf("expected signature to verify")
		}
		return nil
	})

	var createdWebhook *unipost.WebhookSubscription
	test("Webhooks.Create()", func() error {
		wh, err := client.Webhooks.Create(ctx, &unipost.CreateWebhookParams{
			URL:    "https://example.com/unipost-webhook-test",
			Events: []string{"post.published", "post.partial", "post.failed"},
		})
		if err != nil {
			return err
		}
		if wh.ID == "" || !strings.HasPrefix(wh.Secret, "whsec_") {
			return fmt.Errorf("expected webhook id and plaintext secret")
		}
		createdWebhook = wh
		createdWebhookIDs = append(createdWebhookIDs, wh.ID)
		return nil
	})

	if createdWebhook != nil {
		test("Webhooks.List()", func() error {
			items, err := client.Webhooks.List(ctx)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.ID == createdWebhook.ID {
					return nil
				}
			}
			return fmt.Errorf("created webhook not found in list")
		})
		test("Webhooks.ListPage()", func() error {
			page, err := client.Webhooks.ListPage(ctx)
			if err != nil {
				return err
			}
			if page.Meta.Total == nil || page.Meta.Limit == nil {
				return fmt.Errorf("expected meta.total and meta.limit")
			}
			return nil
		})

		test(fmt.Sprintf("Webhooks.Get(\"%s...\")", createdWebhook.ID[:8]), func() error {
			item, err := client.Webhooks.Get(ctx, createdWebhook.ID)
			if err != nil {
				return err
			}
			if item.ID != createdWebhook.ID || item.Secret != "" {
				return fmt.Errorf("unexpected get payload")
			}
			return nil
		})

		test("Webhooks.Update()", func() error {
			active := false
			item, err := client.Webhooks.Update(ctx, createdWebhook.ID, &unipost.UpdateWebhookParams{
				Active: &active,
				Events: []string{"post.failed"},
			})
			if err != nil {
				return err
			}
			if item.Active || len(item.Events) != 1 || item.Events[0] != "post.failed" {
				return fmt.Errorf("unexpected update payload")
			}
			return nil
		})

		test("Webhooks.Rotate()", func() error {
			item, err := client.Webhooks.Rotate(ctx, createdWebhook.ID)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(item.Secret, "whsec_") {
				return fmt.Errorf("expected rotated secret")
			}
			return nil
		})
	}

	// ── 4. Posts — list & get ─────────────────────────────────────────────────
	section("4. Posts — list & get")

	var firstPostID string
	test("Posts.List()", func() error {
		res, err := client.Posts.List(ctx, &unipost.ListPostsParams{Limit: 5})
		if err != nil {
			return err
		}
		if len(res.Data) > 0 {
			firstPostID = res.Data[0].ID
			caption := ""
			if res.Data[0].Caption != nil {
				caption = *res.Data[0].Caption
			}
			if len(caption) > 60 {
				caption = caption[:60]
			}
			fmt.Printf("\n  First post: \"%s...\"  Status: %s\n", caption, res.Data[0].Status)
		}
		return nil
	})

	if firstPostID != "" {
		test(fmt.Sprintf("Posts.Get(\"%s...\")", firstPostID[:8]), func() error {
			post, err := client.Posts.Get(ctx, firstPostID)
			if err != nil {
				return err
			}
			if post.ID == "" {
				return fmt.Errorf("expected post with id")
			}
			return nil
		})
		test(fmt.Sprintf("Posts.GetQueue(\"%s...\")", firstPostID[:8]), func() error {
			queue, err := client.Posts.GetQueue(ctx, firstPostID)
			if err != nil {
				return err
			}
			if queue.Post.ID == "" {
				return fmt.Errorf("expected queue snapshot")
			}
			return nil
		})
	}

	// ── 5. Posts — create ─────────────────────────────────────────────────────
	section("5. Posts — create (draft mode, no actual publishing)")

	if testAccountID == "" {
		fmt.Println("  ⏭  Skipped — set TEST_ACCOUNT_ID env var to run post creation tests")
	} else {
		timestamp := time.Now().UTC().Format(time.RFC3339)

		// Draft
		var draftID string
		test("Posts.Create() — draft", func() error {
			post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
				Caption:    fmt.Sprintf("Go SDK test — %s", timestamp),
				AccountIDs: []string{testAccountID},
				Status:     "draft",
			})
			if err != nil {
				return err
			}
			if post.Status != "draft" {
				return fmt.Errorf("expected draft, got %s", post.Status)
			}
			draftID = post.ID
			return nil
		})
		if draftID != "" {
			createdPostIDs = append(createdPostIDs, draftID)
			fmt.Printf("\n  Created draft: %s\n", draftID)
		}

		// Scheduled
		scheduledAt := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
		var scheduledID string
		test("Posts.Create() — scheduled", func() error {
			post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
				Caption:     fmt.Sprintf("Go SDK scheduled — %s", timestamp),
				AccountIDs:  []string{testAccountID},
				ScheduledAt: scheduledAt,
			})
			if err != nil {
				return err
			}
			if post.Status != "scheduled" {
				return fmt.Errorf("expected scheduled, got %s", post.Status)
			}
			scheduledID = post.ID
			return nil
		})

		if scheduledID != "" {
			createdPostIDs = append(createdPostIDs, scheduledID)
			fmt.Printf("  Created scheduled: %s\n", scheduledID)
			test(fmt.Sprintf("Posts.Cancel(\"%s...\")", scheduledID[:8]), func() error {
				_, err := client.Posts.Cancel(ctx, scheduledID)
				return err
			})
			fmt.Println("  Cancelled scheduled post ✓")
		}

		if os.Getenv("TEST_PUBLISH_NOW") == "true" {
			test("Posts.Create() — publish NOW", func() error {
				post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
					Caption:    fmt.Sprintf("[SDK Test] Hello from Go SDK 🚀 %s", timestamp),
					AccountIDs: []string{testAccountID},
				})
				if err != nil {
					return err
				}
				if post.ID == "" {
					return fmt.Errorf("expected post with id")
				}
				return nil
			})
		} else {
			fmt.Println("\n  ⏭  Real publish skipped (set TEST_PUBLISH_NOW=true)")
		}
	}

	// ── 6. Analytics ──────────────────────────────────────────────────────────
	section("6. Analytics")
	now := time.Now().UTC()
	thirtyDaysAgo := now.Add(-30 * 24 * time.Hour)
	test("Analytics.Rollup()", func() error {
		res, err := client.Analytics.Rollup(ctx, &unipost.AnalyticsRollupParams{
			From:        thirtyDaysAgo.Format(time.RFC3339),
			To:          now.Format(time.RFC3339),
			Granularity: "day",
		})
		if err != nil {
			return err
		}
		if res == nil {
			return fmt.Errorf("expected rollup data")
		}
		return nil
	})

	// ── Cleanup ───────────────────────────────────────────────────────────────
	if len(createdWebhookIDs) > 0 || len(createdPostIDs) > 0 {
		section("7. Cleanup")
	}
	if len(createdWebhookIDs) > 0 {
		for _, id := range createdWebhookIDs {
			if err := client.Webhooks.Delete(ctx, id); err != nil {
				fmt.Printf("  ⚠  Failed to delete webhook %s... (non-fatal)\n", id[:8])
			} else {
				fmt.Printf("  🧹 Deleted webhook %s...\n", id[:8])
			}
		}
	}
	if len(createdPostIDs) > 0 {
		apiURL := os.Getenv("UNIPOST_API_URL")
		if apiURL == "" {
			apiURL = "https://api.unipost.dev"
		}
		for _, id := range createdPostIDs {
			req, _ := http.NewRequest("DELETE", apiURL+"/v1/social-posts/"+id, nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			if _, err := http.DefaultClient.Do(req); err != nil {
				fmt.Printf("  ⚠  Failed to delete %s... (non-fatal)\n", id[:8])
			} else {
				fmt.Printf("  🗑  Deleted %s...\n", id[:8])
			}
		}
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	fmt.Printf("\n╔══════════════════════════════════════════════════╗\n")
	fmt.Printf("║  Results: %2d passed  %2d failed                    ║\n", passed, failed)
	fmt.Printf("╚══════════════════════════════════════════════════╝\n\n")

	if failed > 0 {
		fmt.Println("Failed tests:")
		for _, f := range failures {
			fmt.Printf("  ❌ %s\n", f)
		}
		os.Exit(1)
	} else {
		fmt.Println("🎉 All tests passed! local sdk-go is working correctly.")
	}
}
