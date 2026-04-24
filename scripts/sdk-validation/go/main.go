package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/unipost-dev/sdk-go/unipost"
)

var (
	passed                     int
	failed                     int
	skipped                    int
	failures                   []string
	createdPostIDs             []string
	createdWebhookIDs          []string
	createdMediaIDs            []string
	createdPlatformCredentials []string
)

func test(name string, fn func() error) {
	fmt.Printf("  %s ... ", name)
	if err := fn(); err != nil {
		fmt.Printf("❌ FAIL — %s\n", err)
		failed++
		failures = append(failures, fmt.Sprintf("%s: %s", name, err))
		return
	}
	fmt.Println("✅ PASS")
	passed++
}

func skip(name, reason string) {
	fmt.Printf("  %s ... ⏭ SKIP — %s\n", name, reason)
	skipped++
}

func section(title string) {
	fmt.Printf("\n%s\n  %s\n%s\n", "──────────────────────────────────────────────────", title, "──────────────────────────────────────────────────")
}

func assert(condition bool, message string) error {
	if !condition {
		return errors.New(message)
	}
	return nil
}

func ptrString(value string) *string {
	return &value
}

func pickStableProfile(profiles []unipost.Profile, workspace *unipost.Workspace) *unipost.Profile {
	if len(profiles) == 0 {
		return nil
	}

	for i := range profiles {
		if !strings.HasPrefix(profiles[i].Name, "SDK ") {
			return &profiles[i]
		}
	}

	return &profiles[0]
}

func main() {
	fmt.Println("\n╔══════════════════════════════════════════════════╗")
	fmt.Println("║     sdk-go — API Validation Test                 ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")

	apiKey := os.Getenv("UNIPOST_API_KEY")
	if apiKey == "" {
		fmt.Println("❌ Please set UNIPOST_API_KEY")
		os.Exit(1)
	}

	testAccountID := os.Getenv("TEST_ACCOUNT_ID")
	testPublishNow := os.Getenv("TEST_PUBLISH_NOW") == "true"
	ctx := context.Background()
	client := unipost.NewClient(unipost.WithAPIKey(apiKey))

	var workspace *unipost.Workspace
	var firstProfile *unipost.Profile
	var firstAccount *unipost.SocialAccount
	var firstPost *unipost.Post
	var draftPost *unipost.Post
	var scheduledPost *unipost.Post

	section("1. Public catalogs")

	test("Platforms.Capabilities()", func() error {
		payload, err := client.Platforms.Capabilities(ctx)
		if err != nil {
			return err
		}
		_, ok := payload["schema_version"].(string)
		if !ok {
			return fmt.Errorf("expected schema_version")
		}
		return nil
	})

	test("Plans.List()", func() error {
		plans, err := client.Plans.List(ctx)
		if err != nil {
			return err
		}
		if len(plans) == 0 {
			return fmt.Errorf("expected at least one plan")
		}
		return nil
	})

	section("2. Workspace & profiles")

	test("Workspace.Get()", func() error {
		ws, err := client.Workspace.Get(ctx)
		if err != nil {
			return err
		}
		if ws.ID == "" {
			return fmt.Errorf("expected workspace id")
		}
		workspace = ws
		return nil
	})

	if workspace != nil {
		test("Workspace.Update() — no-op", func() error {
			updated, err := client.Workspace.Update(ctx, workspace.PerAccountMonthlyLimit)
			if err != nil {
				return err
			}
			if updated.ID != workspace.ID {
				return fmt.Errorf("expected same workspace")
			}
			return nil
		})
	}

	test("Profiles.List()", func() error {
		page, err := client.Profiles.List(ctx)
		if err != nil {
			return err
		}
		if len(page.Data) == 0 {
			return fmt.Errorf("expected at least one profile")
		}
		firstProfile = pickStableProfile(page.Data, workspace)
		return nil
	})

	if firstProfile != nil {
		test("Profiles.Get()", func() error {
			profile, err := client.Profiles.Get(ctx, firstProfile.ID)
			if err != nil {
				return err
			}
			if profile.ID != firstProfile.ID {
				return fmt.Errorf("expected matching profile")
			}
			return nil
		})

		test("Profiles.Create() + Delete()", func() error {
			name := fmt.Sprintf("SDK GO Temp %d", time.Now().Unix())
			created, err := client.Profiles.Create(ctx, &unipost.CreateProfileParams{
				Name:                name,
				BrandingDisplayName: ptrString("SDK GO Temp"),
			})
			if err != nil {
				return err
			}
			if created.ID == "" {
				return fmt.Errorf("expected created profile id")
			}
			return client.Profiles.Delete(ctx, created.ID)
		})

		test("Profiles.Update() — no-op", func() error {
			params := &unipost.UpdateProfileParams{Name: &firstProfile.Name}
			if firstProfile.BrandingLogoURL != nil {
				params.BrandingLogoURL = firstProfile.BrandingLogoURL
			}
			if firstProfile.BrandingDisplayName != nil {
				params.BrandingDisplayName = firstProfile.BrandingDisplayName
			}
			if firstProfile.BrandingPrimaryColor != nil {
				params.BrandingPrimaryColor = firstProfile.BrandingPrimaryColor
			}
			profile, err := client.Profiles.Update(ctx, firstProfile.ID, params)
			if err != nil {
				return err
			}
			if profile.ID != firstProfile.ID {
				return fmt.Errorf("expected matching updated profile")
			}
			return nil
		})
	}

	section("3. Accounts")

	var accounts []unipost.SocialAccount
	test("Accounts.List()", func() error {
		items, err := client.Accounts.List(ctx, nil)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return fmt.Errorf("expected at least one account")
		}
		accounts = items
		firstAccount = &accounts[0]
		return nil
	})

	test("Accounts.ListPage()", func() error {
		page, err := client.Accounts.ListPage(ctx, nil)
		if err != nil {
			return err
		}
		if page.Meta.Total == nil || page.Meta.Limit == nil {
			return fmt.Errorf("expected meta.total/meta.limit")
		}
		return nil
	})

	if firstAccount != nil {
		if testAccountID == "" {
			for _, account := range accounts {
				if account.Platform == "bluesky" {
					testAccountID = account.ID
					break
				}
			}
			if testAccountID == "" {
				testAccountID = firstAccount.ID
			}
			fmt.Printf("\n  Using TEST_ACCOUNT_ID=%s for safe draft/scheduled tests\n", testAccountID)
		}

		test("Accounts.Get()", func() error {
			account, err := client.Accounts.Get(ctx, firstAccount.ID)
			if err != nil {
				return err
			}
			if account.ID != firstAccount.ID {
				return fmt.Errorf("expected matching account")
			}
			return nil
		})

		test("Accounts.Health()", func() error {
			health, err := client.Accounts.Health(ctx, firstAccount.ID)
			if err != nil {
				return err
			}
			if health.SocialAccountID != firstAccount.ID {
				return fmt.Errorf("expected matching account health")
			}
			return nil
		})

		test("Accounts.Capabilities()", func() error {
			payload, err := client.Accounts.Capabilities(ctx, firstAccount.ID)
			if err != nil {
				if apiErr, ok := err.(*unipost.APIError); ok && apiErr.Code == "not_found" {
					return nil
				}
				return err
			}
			if _, ok := payload["schema_version"].(string); !ok {
				return fmt.Errorf("expected schema_version")
			}
			return nil
		})
	}

	tiktokID := ""
	facebookID := ""
	for _, account := range accounts {
		if account.Platform == "tiktok" && tiktokID == "" {
			tiktokID = account.ID
		}
		if account.Platform == "facebook" && facebookID == "" {
			facebookID = account.ID
		}
	}

	if tiktokID != "" {
		test("Accounts.TikTokCreatorInfo()", func() error {
			payload, err := client.Accounts.TikTokCreatorInfo(ctx, tiktokID)
			if err != nil {
				return err
			}
			if _, ok := payload["creator_username"]; !ok {
				if _, ok = payload["creator_nickname"]; !ok {
					return fmt.Errorf("expected TikTok creator fields")
				}
			}
			return nil
		})
	} else {
		skip("Accounts.TikTokCreatorInfo()", "No TikTok account connected")
	}

	if facebookID != "" {
		test("Accounts.FacebookPageInsights()", func() error {
			_, err := client.Accounts.FacebookPageInsights(ctx, facebookID)
			if err != nil {
				if apiErr, ok := err.(*unipost.APIError); ok {
					if apiErr.Code == "forbidden" || apiErr.Code == "facebook_disabled" || apiErr.Code == "FACEBOOK_DISABLED" || apiErr.Code == "not_found" {
						return nil
					}
				}
				return err
			}
			return nil
		})
	} else {
		skip("Accounts.FacebookPageInsights()", "No Facebook account connected")
	}

	test("Accounts.Connect() — invalid credentials negative path", func() error {
		profileID := ""
		if firstProfile != nil {
			profileID = firstProfile.ID
		}
		_, err := client.Accounts.Connect(ctx, &unipost.ConnectAccountParams{
			ProfileID:   profileID,
			Platform:    "bluesky",
			Credentials: map[string]string{"identifier": "invalid", "password": "invalid"},
		})
		if err == nil {
			return fmt.Errorf("expected API error")
		}
		if apiErr, ok := err.(*unipost.APIError); ok {
			if apiErr.Code == "auth_error" || apiErr.Code == "unauthorized" || apiErr.Code == "validation_error" {
				return nil
			}
		}
		return err
	})

	section("4. Media, connect sessions, users")

	test("Media.Upload()", func() error {
		media, err := client.Media.Upload(ctx, &unipost.MediaUploadRequest{
			Filename:    "sdk-validation.png",
			ContentType: "image/png",
			SizeBytes:   128,
			ContentHash: fmt.Sprintf("sdk-go-%d", time.Now().Unix()),
		})
		if err != nil {
			return err
		}
		mediaID := media.MediaID
		if mediaID == "" {
			mediaID = media.ID
		}
		if mediaID == "" {
			return fmt.Errorf("expected media id")
		}
		createdMediaIDs = append(createdMediaIDs, mediaID)
		return nil
	})

	if len(createdMediaIDs) > 0 {
		test("Media.Get()", func() error {
			media, err := client.Media.Get(ctx, createdMediaIDs[0])
			if err != nil {
				return err
			}
			mediaID := media.MediaID
			if mediaID == "" {
				mediaID = media.ID
			}
			if mediaID != createdMediaIDs[0] {
				return fmt.Errorf("expected matching media")
			}
			return nil
		})
	}

	var connectSession *unipost.ConnectSession
	test("Connect.CreateSession()", func() error {
		session, err := client.Connect.CreateSession(ctx, &unipost.CreateConnectSessionParams{
			Platform:          "bluesky",
			ProfileID:         firstProfile.ID,
			ExternalUserID:    fmt.Sprintf("sdk-go-%d", time.Now().Unix()),
			ExternalUserEmail: "sdk-validation@example.com",
			ReturnURL:         "https://example.com/return",
		})
		if err != nil {
			return err
		}
		if session.ID == "" || session.URL == "" {
			return fmt.Errorf("expected connect session")
		}
		connectSession = session
		return nil
	})

	if connectSession != nil {
		test("Connect.GetSession()", func() error {
			session, err := client.Connect.GetSession(ctx, connectSession.ID)
			if err != nil {
				return err
			}
			if session.ID != connectSession.ID {
				return fmt.Errorf("expected matching session")
			}
			return nil
		})
	}

	var firstUser *unipost.ManagedUser
	test("Users.List()", func() error {
		users, err := client.Users.List(ctx)
		if err != nil {
			return err
		}
		if len(users) > 0 {
			firstUser = &users[0]
		}
		return nil
	})

	if firstUser != nil {
		test("Users.Get()", func() error {
			user, err := client.Users.Get(ctx, firstUser.ExternalUserID)
			if err != nil {
				return err
			}
			if user.ExternalUserID != firstUser.ExternalUserID {
				return fmt.Errorf("expected matching managed user")
			}
			return nil
		})
	} else {
		skip("Users.Get()", "No managed users available")
	}

	section("5. Webhooks")

	test("VerifyWebhookSignature()", func() error {
		payload := []byte(`{"event":"post.published","data":{"id":"post_test_123"}}`)
		secret := "whsec_test_local"
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !unipost.VerifyWebhookSignature(payload, signature, secret) {
			return fmt.Errorf("expected valid signature")
		}
		return nil
	})

	var webhook *unipost.WebhookSubscription
	test("Webhooks.Create()", func() error {
		active := false
		item, err := client.Webhooks.Create(ctx, &unipost.CreateWebhookParams{
			Name:   "SDK validation webhook",
			URL:    "https://example.com/unipost-webhook-test",
			Events: []string{"post.published", "post.partial", "post.failed"},
			Active: &active,
			Secret: "sdk-validation-secret",
		})
		if err != nil {
			return err
		}
		if item.ID == "" || item.Secret != "sdk-validation-secret" {
			return fmt.Errorf("expected webhook id and custom secret")
		}
		if item.Name != "SDK validation webhook" || item.Active {
			return fmt.Errorf("unexpected created webhook fields")
		}
		webhook = item
		createdWebhookIDs = append(createdWebhookIDs, item.ID)
		return nil
	})

	if webhook != nil {
		test("Webhooks.List()", func() error {
			items, err := client.Webhooks.List(ctx)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.ID == webhook.ID {
					return nil
				}
			}
			return fmt.Errorf("expected created webhook in list")
		})

		test("Webhooks.ListPage()", func() error {
			page, err := client.Webhooks.ListPage(ctx)
			if err != nil {
				return err
			}
			if page.Meta.Total == nil || page.Meta.Limit == nil {
				return fmt.Errorf("expected meta.total/meta.limit")
			}
			return nil
		})

		test("Webhooks.Get()", func() error {
			item, err := client.Webhooks.Get(ctx, webhook.ID)
			if err != nil {
				return err
			}
			if item.ID != webhook.ID || item.Secret != "" || item.Name != "SDK validation webhook" {
				return fmt.Errorf("unexpected get payload")
			}
			return nil
		})

		test("Webhooks.Update()", func() error {
			name := "Failure-only webhook"
			active := true
			item, err := client.Webhooks.Update(ctx, webhook.ID, &unipost.UpdateWebhookParams{
				Name:   &name,
				Active: &active,
				Events: []string{"post.failed"},
			})
			if err != nil {
				return err
			}
			if !item.Active || len(item.Events) != 1 || item.Name != name {
				return fmt.Errorf("unexpected update payload")
			}
			return nil
		})

		test("Webhooks.Rotate()", func() error {
			item, err := client.Webhooks.Rotate(ctx, webhook.ID)
			if err != nil {
				return err
			}
			if !strings.HasPrefix(item.Secret, "whsec_") {
				return fmt.Errorf("expected rotated secret")
			}
			return nil
		})
	}

	section("6. Platform credentials")

	if workspace != nil {
		platformName := fmt.Sprintf("sdk-go-%d", time.Now().Unix())
		test("PlatformCredentials.Create()/List()/Delete()", func() error {
			item, err := client.PlatformCredentials.Create(ctx, workspace.ID, &unipost.CreatePlatformCredentialParams{
				Platform:     platformName,
				ClientID:     "sdk-client-id",
				ClientSecret: "sdk-client-secret",
			})
			if err != nil {
				if apiErr, ok := err.(*unipost.APIError); ok && apiErr.Code == "forbidden" {
					skip("PlatformCredentials.Create()/List()/Delete()", "Plan-gated")
					return nil
				}
				return err
			}
			if item.Platform != platformName {
				return fmt.Errorf("expected created platform credential")
			}
			createdPlatformCredentials = append(createdPlatformCredentials, platformName)
			page, err := client.PlatformCredentials.List(ctx, workspace.ID)
			if err != nil {
				return err
			}
			found := false
			for _, cred := range page.Data {
				if cred.Platform == platformName {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("expected created credential in list")
			}
			if err := client.PlatformCredentials.Delete(ctx, workspace.ID, platformName); err != nil {
				return err
			}
			createdPlatformCredentials = createdPlatformCredentials[:len(createdPlatformCredentials)-1]
			return nil
		})
	}

	section("7. Posts")

	test("Posts.Validate()", func() error {
		res, err := client.Posts.Validate(ctx, &unipost.CreatePostParams{
			Caption:    "SDK validation draft",
			AccountIDs: maybeAccountIDs(testAccountID),
			Status:     "draft",
		})
		if err != nil {
			return err
		}
		if res == nil {
			return fmt.Errorf("expected validation payload")
		}
		return nil
	})

	test("Posts.List()", func() error {
		page, err := client.Posts.List(ctx, &unipost.ListPostsParams{Limit: 5})
		if err != nil {
			return err
		}
		if len(page.Data) > 0 {
			firstPost = &page.Data[0]
		}
		return nil
	})

	if firstPost != nil {
		test("Posts.Get()", func() error {
			post, err := client.Posts.Get(ctx, firstPost.ID)
			if err != nil {
				return err
			}
			if post.ID != firstPost.ID {
				return fmt.Errorf("expected matching post")
			}
			return nil
		})

		test("Posts.GetQueue()", func() error {
			queue, err := client.Posts.GetQueue(ctx, firstPost.ID)
			if err != nil {
				return err
			}
			if queue.Post.ID != firstPost.ID {
				return fmt.Errorf("expected queue snapshot")
			}
			return nil
		})

		test("Posts.Analytics()", func() error {
			items, err := client.Posts.Analytics(ctx, firstPost.ID, false)
			if err != nil {
				return err
			}
			if items == nil {
				return fmt.Errorf("expected analytics response")
			}
			return nil
		})
	}

	if testAccountID == "" {
		skip("Posts.Create()/Update()/PreviewLink()/Archive()/Restore()/Cancel()", "No TEST_ACCOUNT_ID available")
		skip("Posts.BulkCreate()", "No TEST_ACCOUNT_ID available")
		skip("Posts.Publish()", "No TEST_ACCOUNT_ID available")
	} else {
		test("Posts.Create() — draft", func() error {
			post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
				Caption:    fmt.Sprintf("Go SDK draft %s", time.Now().UTC().Format(time.RFC3339)),
				AccountIDs: []string{testAccountID},
				Status:     "draft",
			})
			if err != nil {
				return err
			}
			if post.ID == "" || post.Status != "draft" {
				return fmt.Errorf("expected draft post")
			}
			draftPost = post
			createdPostIDs = append(createdPostIDs, post.ID)
			return nil
		})

		if draftPost != nil {
			test("Posts.Update() — draft", func() error {
				caption := "Go SDK draft updated"
				post, err := client.Posts.Update(ctx, draftPost.ID, &unipost.UpdatePostParams{Caption: &caption, AccountIDs: []string{testAccountID}})
				if err != nil {
					return err
				}
				if post.ID != draftPost.ID {
					return fmt.Errorf("expected updated draft")
				}
				return nil
			})

			test("Posts.PreviewLink()", func() error {
				link, err := client.Posts.PreviewLink(ctx, draftPost.ID)
				if err != nil {
					return err
				}
				if link.URL == "" || link.Token == "" {
					return fmt.Errorf("expected preview link")
				}
				return nil
			})

			test("Posts.Archive()", func() error {
				post, err := client.Posts.Archive(ctx, draftPost.ID)
				if err != nil {
					return err
				}
				if post.ID != draftPost.ID {
					return fmt.Errorf("expected archived draft")
				}
				return nil
			})

			test("Posts.Restore()", func() error {
				post, err := client.Posts.Restore(ctx, draftPost.ID)
				if err != nil {
					return err
				}
				if post.ID != draftPost.ID {
					return fmt.Errorf("expected restored draft")
				}
				return nil
			})
		}

		test("Posts.Create() — scheduled", func() error {
			post, err := client.Posts.Create(ctx, &unipost.CreatePostParams{
				Caption:     fmt.Sprintf("Go SDK scheduled %s", time.Now().UTC().Format(time.RFC3339)),
				AccountIDs:  []string{testAccountID},
				ScheduledAt: time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339),
			})
			if err != nil {
				return err
			}
			if post.ID == "" || post.Status != "scheduled" {
				return fmt.Errorf("expected scheduled post")
			}
			scheduledPost = post
			createdPostIDs = append(createdPostIDs, post.ID)
			return nil
		})

		if scheduledPost != nil {
			test("Posts.Update() — scheduled", func() error {
				scheduledAt := time.Now().UTC().Add(20 * time.Minute).Format(time.RFC3339)
				post, err := client.Posts.Update(ctx, scheduledPost.ID, &unipost.UpdatePostParams{ScheduledAt: &scheduledAt})
				if err != nil {
					if apiErr, ok := err.(*unipost.APIError); ok && apiErr.Code == "validation_error" {
						return nil
					}
					return err
				}
				if post.ID != scheduledPost.ID {
					return fmt.Errorf("expected updated scheduled post")
				}
				return nil
			})

			test("Posts.Cancel()", func() error {
				post, err := client.Posts.Cancel(ctx, scheduledPost.ID)
				if err != nil {
					return err
				}
				if post.ID != scheduledPost.ID {
					return fmt.Errorf("expected canceled scheduled post")
				}
				return nil
			})
		}

		test("Posts.BulkCreate()", func() error {
			items, err := client.Posts.BulkCreate(ctx, []*unipost.CreatePostParams{
				{Caption: "Go bulk A", AccountIDs: []string{testAccountID}, Status: "draft"},
				{Caption: "Go bulk B", AccountIDs: []string{testAccountID}, Status: "draft"},
			})
			if err != nil {
				return err
			}
			if len(items) != 2 {
				return fmt.Errorf("expected two bulk result entries")
			}
			for _, item := range items {
				if item.Data == nil && item.Error == nil {
					return fmt.Errorf("expected bulk result payload")
				}
			}
			return nil
		})

		if testPublishNow && draftPost != nil {
			test("Posts.Publish() — live publish", func() error {
				post, err := client.Posts.Publish(ctx, draftPost.ID)
				if err != nil {
					return err
				}
				if post.ID != draftPost.ID {
					return fmt.Errorf("expected publish response")
				}
				return nil
			})
		} else {
			skip("Posts.Publish() — live publish", "Opt-in only (set TEST_PUBLISH_NOW=true)")
		}
	}

	failedResultID := ""
	if firstPost != nil {
		for _, result := range firstPost.Results {
			if result.Status == "failed" && result.ID != "" {
				failedResultID = result.ID
				break
			}
		}
	}
	if firstPost != nil && failedResultID != "" {
		test("Posts.RetryResult()", func() error {
			result, err := client.Posts.RetryResult(ctx, firstPost.ID, failedResultID)
			if err != nil {
				return err
			}
			if result.SocialAccountID == "" {
				return fmt.Errorf("expected retry result payload")
			}
			return nil
		})
	} else {
		skip("Posts.RetryResult()", "No failed result available")
	}

	section("8. Delivery jobs, analytics, usage, oauth")

	test("DeliveryJobs.List()", func() error {
		items, err := client.DeliveryJobs.List(ctx, &unipost.ListDeliveryJobsParams{Limit: 5})
		if err != nil {
			return err
		}
		if items == nil {
			return fmt.Errorf("expected jobs slice")
		}
		return nil
	})

	test("DeliveryJobs.Summary()", func() error {
		summary, err := client.DeliveryJobs.Summary(ctx)
		if err != nil {
			return err
		}
		if summary == nil {
			return fmt.Errorf("expected summary payload")
		}
		return nil
	})

	retryableJobs, _ := client.DeliveryJobs.List(ctx, &unipost.ListDeliveryJobsParams{Limit: 20, States: "pending,retrying"})
	if len(retryableJobs) > 0 {
		jobID := retryableJobs[0].ID
		test("DeliveryJobs.Retry()/Cancel()", func() error {
			if _, err := client.DeliveryJobs.Retry(ctx, jobID); err != nil {
				if apiErr, ok := err.(*unipost.APIError); !ok || (apiErr.Code != "queue_job_active" && apiErr.Code != "bad_request" && apiErr.Code != "conflict") {
					return err
				}
			}
			if _, err := client.DeliveryJobs.Cancel(ctx, jobID); err != nil {
				if apiErr, ok := err.(*unipost.APIError); !ok || (apiErr.Code != "bad_request" && apiErr.Code != "conflict") {
					return err
				}
			}
			return nil
		})
	} else {
		skip("DeliveryJobs.Retry()/Cancel()", "No retryable delivery jobs available")
	}

	from := time.Now().UTC().Add(-30 * 24 * time.Hour).Format("2006-01-02")
	to := time.Now().UTC().Format("2006-01-02")

	test("Analytics.Summary()", func() error {
		summary, err := client.Analytics.Summary(ctx, &unipost.AnalyticsQueryParams{From: from, To: to})
		if err != nil {
			return err
		}
		if _, ok := summary["posts"]; !ok {
			return fmt.Errorf("expected summary payload")
		}
		return nil
	})

	test("Analytics.Trend()", func() error {
		trend, err := client.Analytics.Trend(ctx, &unipost.AnalyticsQueryParams{From: from, To: to})
		if err != nil {
			return err
		}
		if _, ok := trend["dates"]; !ok {
			return fmt.Errorf("expected trend dates")
		}
		return nil
	})

	test("Analytics.ByPlatform()", func() error {
		rows, err := client.Analytics.ByPlatform(ctx, &unipost.AnalyticsQueryParams{From: from, To: to})
		if err != nil {
			return err
		}
		if rows == nil {
			return fmt.Errorf("expected by-platform rows")
		}
		return nil
	})

	test("Analytics.Rollup()", func() error {
		rollup, err := client.Analytics.Rollup(ctx, &unipost.AnalyticsRollupParams{
			From:        time.Now().UTC().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
			To:          time.Now().UTC().Format(time.RFC3339),
			Granularity: "day",
		})
		if err != nil {
			return err
		}
		if rollup == nil || rollup.Series == nil {
			return fmt.Errorf("expected rollup series")
		}
		return nil
	})

	test("Usage.Get()", func() error {
		usage, err := client.Usage.Get(ctx)
		if err != nil {
			return err
		}
		if usage.PostLimit < 0 {
			return fmt.Errorf("unexpected usage payload")
		}
		return nil
	})

	test("OAuth.Connect() — known backend path", func() error {
		_, err := client.OAuth.Connect(ctx, "bluesky", "https://example.com/callback")
		if err != nil {
			if apiErr, ok := err.(*unipost.APIError); ok && (apiErr.Code == "unauthorized" || apiErr.Code == "validation_error") {
				return nil
			}
			return err
		}
		return nil
	})

	cleanup(ctx, client, workspace)

	fmt.Println("\n╔══════════════════════════════════════════════════╗")
	fmt.Printf("║  Results: %2d passed  %2d failed  %2d skipped      ║\n", passed, failed, skipped)
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()

	if failed > 0 {
		fmt.Println("Failed tests:")
		for _, failure := range failures {
			fmt.Printf("  ❌ %s\n", failure)
		}
		os.Exit(1)
	}

	fmt.Println("🎉 All required Go SDK validations passed.")
}

func maybeAccountIDs(id string) []string {
	if id == "" {
		return nil
	}
	return []string{id}
}

func cleanup(ctx context.Context, client *unipost.Client, workspace *unipost.Workspace) {
	if len(createdWebhookIDs) > 0 || len(createdMediaIDs) > 0 || len(createdPostIDs) > 0 || len(createdPlatformCredentials) > 0 {
		section("Cleanup")
	}

	for _, webhookID := range createdWebhookIDs {
		if err := client.Webhooks.Delete(ctx, webhookID); err != nil {
			fmt.Printf("  ⚠ Failed to delete webhook %s... (%v)\n", webhookID[:8], err)
		} else {
			fmt.Printf("  🧹 Deleted webhook %s...\n", webhookID[:8])
		}
	}

	for _, mediaID := range createdMediaIDs {
		if err := client.Media.Delete(ctx, mediaID); err != nil {
			fmt.Printf("  ⚠ Failed to delete media %s... (%v)\n", mediaID[:8], err)
		} else {
			fmt.Printf("  🧹 Deleted media %s...\n", mediaID[:8])
		}
	}

	for _, postID := range createdPostIDs {
		if err := client.Posts.Delete(ctx, postID); err != nil {
			fmt.Printf("  ⚠ Failed to delete post %s... (%v)\n", postID[:8], err)
		} else {
			fmt.Printf("  🧹 Deleted post %s...\n", postID[:8])
		}
	}

	if workspace != nil {
		for _, platformName := range createdPlatformCredentials {
			if err := client.PlatformCredentials.Delete(ctx, workspace.ID, platformName); err != nil {
				fmt.Printf("  ⚠ Failed to delete platform credential %s... (%v)\n", platformName, err)
			} else {
				fmt.Printf("  🧹 Deleted platform credential %s\n", platformName)
			}
		}
	}
}
