// UniPost Go SDK Validation Test
//
// Setup:
//   1. go mod init unipost-sdk-test && go get github.com/unipost-dev/sdk-go
//   2. UNIPOST_API_KEY=up_live_xxx go run main.go
//   3. UNIPOST_API_KEY=up_live_xxx TEST_ACCOUNT_ID=<id> go run main.go

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/unipost-dev/sdk-go/unipost"
)

var (
	passed   int
	failed   int
	failures []string
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

	if len(accounts) > 0 {
		fmt.Printf("\n  Found %d connected accounts:\n", len(accounts))
		for _, a := range accounts {
			name := a.AccountName
			if name == "" {
				name = a.ID
			}
			fmt.Printf("    • [%-10s] %s  (id: %s)\n", a.Platform, name, a.ID)
		}
	}

	// ── 2. Posts — list & get ─────────────────────────────────────────────────
	section("2. Posts — list & get")

	var firstPostID string
	test("Posts.List()", func() error {
		res, err := client.Posts.List(ctx, &unipost.ListPostsParams{Limit: 5})
		if err != nil {
			return err
		}
		if len(res.Data) > 0 {
			firstPostID = res.Data[0].ID
			caption := res.Data[0].Caption
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
	}

	// ── 3. Posts — create ─────────────────────────────────────────────────────
	section("3. Posts — create (draft mode, no actual publishing)")

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
		fmt.Println("🎉 All tests passed! sdk-go is working correctly.")
	}
}
