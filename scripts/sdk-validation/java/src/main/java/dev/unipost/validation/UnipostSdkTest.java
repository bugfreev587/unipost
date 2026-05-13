package dev.unipost.validation;

import com.fasterxml.jackson.databind.JsonNode;
import dev.unipost.APIError;
import dev.unipost.Page;
import dev.unipost.UniPost;
import dev.unipost.WebhookVerifier;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

public final class UnipostSdkTest {
    private static final String API_KEY = env("UNIPOST_API_KEY", "");
    private static final String BASE_URL = env("BASE_URL", "https://api.unipost.dev");
    private static final boolean TEST_PUBLISH_NOW = "true".equalsIgnoreCase(env("TEST_PUBLISH_NOW", "false"));

    private static int passed;
    private static int failed;
    private static int skipped;
    private static final List<String> failures = new ArrayList<>();
    private static final List<String> createdPostIds = new ArrayList<>();
    private static final List<String> createdWebhookIds = new ArrayList<>();
    private static final List<String> createdMediaIds = new ArrayList<>();
    private static final List<String> createdPlatformCredentialKeys = new ArrayList<>();

    private UnipostSdkTest() {
    }

    public static void main(String[] args) {
        banner();
        if (API_KEY.isBlank()) {
            System.err.println("❌ Please set UNIPOST_API_KEY");
            System.exit(1);
        }

        UniPost client = UniPost.builder().apiKey(API_KEY).baseUrl(BASE_URL).build();
        String testAccountId = env("TEST_ACCOUNT_ID", "");

        JsonNode workspace = null;
        JsonNode firstProfile = null;
        JsonNode firstAccount = null;
        JsonNode draftPost = null;
        JsonNode scheduledPost = null;
        JsonNode firstPost = null;

        section("1. Public catalogs");
        test("platforms.capabilities()", () -> {
            JsonNode res = client.platforms().capabilities();
            assertTrue(res.has("schema_version"), "Expected schema_version");
        });
        test("plans.list()", () -> {
            List<JsonNode> res = client.plans().list();
            assertTrue(!res.isEmpty(), "Expected at least one plan");
        });

        section("2. Workspace & profiles");
        workspace = testValue("workspace.get()", client.workspace()::get);
        if (workspace != null) {
            JsonNode finalWorkspace = workspace;
            test("workspace.update() — no-op", () -> {
                Map<String, Object> body = map();
                if (!finalWorkspace.path("per_account_monthly_limit").isMissingNode()
                        && !finalWorkspace.path("per_account_monthly_limit").isNull()) {
                    body.put("per_account_monthly_limit", finalWorkspace.path("per_account_monthly_limit").asInt());
                } else {
                    body.put("per_account_monthly_limit", null);
                }
                JsonNode updated = client.workspace().update(body);
                assertEquals(finalWorkspace.path("id").asText(), updated.path("id").asText(), "Expected same workspace");
            });
        }

        Page<JsonNode> profilesPage = testValue("profiles.list()", client.profiles()::list);
        List<JsonNode> profiles = profilesPage == null ? List.of() : profilesPage.getData();
        firstProfile = pickStableProfile(profiles, workspace);

        if (firstProfile != null) {
            JsonNode finalFirstProfile = firstProfile;
            test("profiles.get()", () -> {
                JsonNode res = client.profiles().get(finalFirstProfile.path("id").asText());
                assertEquals(finalFirstProfile.path("id").asText(), res.path("id").asText(), "Expected matching profile");
            });
            test("profiles.create() + delete()", () -> {
                JsonNode created = client.profiles().create(map(
                        "name", "SDK JAVA Temp " + Instant.now().getEpochSecond(),
                        "branding_display_name", "SDK JAVA Temp"
                ));
                assertTrue(created.hasNonNull("id"), "Expected created profile id");
                client.profiles().delete(created.path("id").asText());
            });
            test("profiles.update() — no-op", () -> {
                Map<String, Object> body = map("name", finalFirstProfile.path("name").asText());
                maybePut(body, "branding_logo_url", finalFirstProfile.path("branding_logo_url"));
                maybePut(body, "branding_display_name", finalFirstProfile.path("branding_display_name"));
                maybePut(body, "branding_primary_color", finalFirstProfile.path("branding_primary_color"));
                JsonNode updated = client.profiles().update(finalFirstProfile.path("id").asText(), body);
                assertEquals(finalFirstProfile.path("id").asText(), updated.path("id").asText(), "Expected matching updated profile");
            });
        } else {
            skip("profiles.get()", "No profiles available");
            skip("profiles.create() + delete()", "No profiles available");
            skip("profiles.update() — no-op", "No profiles available");
        }

        section("3. Accounts");
        Page<JsonNode> accountsPage = testValue("accounts.list()", client.accounts()::list);
        List<JsonNode> accounts = accountsPage == null ? List.of() : accountsPage.getData();
        firstAccount = accounts.isEmpty() ? null : accounts.get(0);
        JsonNode tikTokAccount = findByPlatform(accounts, "tiktok");
        JsonNode facebookAccount = findByPlatform(accounts, "facebook");
        if (firstAccount != null && testAccountId.isBlank()) {
            JsonNode safest = Objects.requireNonNullElse(findByPlatform(accounts, "bluesky"), firstAccount);
            testAccountId = safest.path("id").asText();
            System.out.println("\n  Using TEST_ACCOUNT_ID=" + testAccountId + " for safe draft/scheduled tests");
        }
        if (firstAccount != null) {
            JsonNode finalFirstAccount = firstAccount;
            test("accounts.get()", () -> {
                JsonNode res = client.accounts().get(finalFirstAccount.path("id").asText());
                assertEquals(finalFirstAccount.path("id").asText(), res.path("id").asText(), "Expected matching account");
            });
            test("accounts.health()", () -> {
                JsonNode res = client.accounts().health(finalFirstAccount.path("id").asText());
                assertTrue(res.has("status"), "Expected status");
            });
            test("accounts.capabilities()", () -> {
                JsonNode res = client.accounts().capabilities(finalFirstAccount.path("id").asText());
                assertTrue(res.isObject(), "Expected object");
            });
        } else {
            skip("accounts.get()", "No accounts available");
            skip("accounts.health()", "No accounts available");
            skip("accounts.capabilities()", "No accounts available");
        }
        if (tikTokAccount != null) {
            JsonNode finalTikTokAccount = tikTokAccount;
            test("accounts.tikTokCreatorInfo()", () -> {
                JsonNode res = client.accounts().tikTokCreatorInfo(finalTikTokAccount.path("id").asText());
                assertTrue(res.isObject(), "Expected object");
            });
        } else {
            skip("accounts.tikTokCreatorInfo()", "No TikTok account connected");
        }
        if (facebookAccount != null) {
            JsonNode finalFacebookAccount = facebookAccount;
            test("accounts.facebookPageInsights()", () -> {
                try {
                    JsonNode res = client.accounts().facebookPageInsights(finalFacebookAccount.path("id").asText());
                    assertTrue(res.isObject(), "Expected object");
                } catch (APIError error) {
                    if (matchesCode(error, "forbidden", "facebook_disabled", "FACEBOOK_DISABLED", "not_found")
                            || containsAny(error.getMessage(), "facebook integration is not enabled")) {
                        return;
                    }
                    throw error;
                }
            });
        } else {
            skip("accounts.facebookPageInsights()", "No Facebook account connected");
        }
        String invalidConnectProfileId = finalProfileId(firstProfile);
        expectApiError(
                "accounts.connect() — invalid credentials negative path",
                () -> client.accounts().connect(map(
                        "profile_id", invalidConnectProfileId,
                        "platform", "bluesky",
                        "credentials", map("identifier", "invalid", "password", "invalid")
                )),
                List.of("auth_error", "unauthorized", "UNAUTHORIZED", "validation_error")
        );

        section("4. Media, connect sessions, users");
        JsonNode createdMedia = testValue("media.upload()", () -> client.media().upload(map(
                "filename", "sdk-validation.png",
                "content_type", "image/png",
                "size_bytes", 128,
                "content_hash", "sdk-java-" + Instant.now().toEpochMilli()
        )));
        if (createdMedia != null) {
            String mediaId = firstPresent(createdMedia, "id", "media_id", "mediaId");
            if (mediaId != null) {
                createdMediaIds.add(mediaId);
                String finalMediaId = mediaId;
                test("media.get()", () -> {
                    JsonNode res = client.media().get(finalMediaId);
                    assertEquals(finalMediaId, firstPresent(res, "id", "media_id", "mediaId"), "Expected matching media");
                });
            }
        }

        if (firstProfile != null) {
            JsonNode finalFirstProfile1 = firstProfile;
            JsonNode connectSession = testValue("connect.createSession() — youtube", () -> client.connect().createSession(map(
                    "platform", "youtube",
                    "profile_id", finalFirstProfile1.path("id").asText(),
                    "external_user_id", "sdk-java-user-" + Instant.now().getEpochSecond(),
                    "external_user_email", "sdk-java@example.com",
                    "return_url", "https://example.com/return"
            )));
            if (connectSession != null) {
                assertEquals("youtube", connectSession.path("platform").asText(), "Expected youtube connect session");
            }
            if (connectSession != null && connectSession.hasNonNull("id")) {
                test("connect.getSession()", () -> {
                    JsonNode res = client.connect().getSession(connectSession.path("id").asText());
                    assertEquals(connectSession.path("id").asText(), res.path("id").asText(), "Expected matching session");
                });
            }
            test("connect.getConnectUrl()", () -> {
                JsonNode res = client.connect().getConnectUrl(map(
                        "profile_id", finalFirstProfile1.path("id").asText(),
                        "platform", "linkedin",
                        "redirect_url", "https://example.com/callback"
                ));
                assertTrue(res.has("auth_url"), "Expected auth_url");
            });
        } else {
            skip("connect.createSession()", "No profile available");
            skip("connect.getSession()", "No profile available");
            skip("connect.getConnectUrl()", "No profile available");
        }

        Page<JsonNode> usersPage = testValue("users.list()", client.users()::list);
        List<JsonNode> users = usersPage == null ? List.of() : usersPage.getData();
        if (!users.isEmpty()) {
            JsonNode user = users.get(0);
            test("users.get()", () -> {
                String externalUserId = firstPresent(user, "external_user_id", "id");
                JsonNode res = client.users().get(externalUserId);
                assertTrue(res.has("external_user_id") || res.has("id"), "Expected managed user fields");
            });
        } else {
            skip("users.get()", "No managed users available");
        }

        section("5. Webhooks");
        test("verify_webhook_signature()", UnipostSdkTest::testWebhookSignature);
        JsonNode webhook = testValue("webhooks.create()", () -> client.webhooks().create(map(
                "name", "SDK Java Test",
                "url", "https://example.com/unipost/webhook",
                "events", List.of("post.published"),
                "active", true,
                "secret", "whsec_sdk_java"
        )));
        if (webhook != null && webhook.hasNonNull("id")) {
            String webhookId = webhook.path("id").asText();
            createdWebhookIds.add(webhookId);
            test("webhooks.list()", () -> {
                Page<JsonNode> page = client.webhooks().list();
                assertTrue(page.getData().stream().anyMatch(node -> webhookId.equals(node.path("id").asText())), "Expected created webhook in list");
            });
            test("webhooks.get()", () -> {
                JsonNode res = client.webhooks().get(webhookId);
                assertEquals(webhookId, res.path("id").asText(), "Expected matching webhook");
            });
            test("webhooks.update()", () -> {
                JsonNode res = client.webhooks().update(webhookId, map("name", "SDK Java Updated"));
                assertEquals(webhookId, res.path("id").asText(), "Expected matching updated webhook");
            });
            test("webhooks.rotate()", () -> {
                JsonNode res = client.webhooks().rotate(webhookId);
                assertTrue(res.has("secret"), "Expected rotated secret");
            });
        }

        section("6. Platform credentials");
        String platformKey = "sdk-java-" + Instant.now().getEpochSecond();
        test("platformCredentials.create()/list()/delete()", () -> {
            JsonNode created = client.platformCredentials().create(map(
                    "platform", platformKey,
                    "client_id", "client-id",
                    "client_secret", "client-secret"
            ));
            assertTrue(created.has("platform"), "Expected platform credential");
            createdPlatformCredentialKeys.add(platformKey);
            Page<JsonNode> listed = client.platformCredentials().list();
            assertTrue(listed.getData().stream().anyMatch(node -> platformKey.equals(node.path("platform").asText())), "Expected credential in list");
            client.platformCredentials().delete(platformKey);
            createdPlatformCredentialKeys.remove(platformKey);
        });

        section("6b. API keys");
        test("apiKeys.list()", () -> {
            Page<JsonNode> page = client.apiKeys().list();
            assertTrue(page.getMeta().containsKey("total") || page.getData() != null, "Expected page payload");
        });
        test("apiKeys.create()/revoke()", () -> {
            JsonNode created = client.apiKeys().create(map("name", "SDK Java Validation Key"));
            assertTrue(created.hasNonNull("id"), "Expected api key id");
            assertTrue(firstPresent(created, "prefix", "key_prefix") != null, "Expected api key prefix");
            client.apiKeys().revoke(created.path("id").asText());
        });

        section("7. Posts");
        String validationAccountId = testAccountId;
        test("posts.validate()", () -> {
            JsonNode res = client.posts().validate(map(
                    "caption", "SDK Java validation",
                    "account_ids", validationAccountId.isBlank() ? List.of() : List.of(validationAccountId)
            ));
            assertTrue(res.isObject(), "Expected validation payload");
        });
        Page<JsonNode> postsPage = testValue("posts.list()", () -> client.posts().list(Map.of("limit", 5)));
        List<JsonNode> posts = postsPage == null ? List.of() : postsPage.getData();
        firstPost = posts.isEmpty() ? null : posts.get(0);

        if (!testAccountId.isBlank()) {
            String finalTestAccountId = testAccountId;
            draftPost = testValue("posts.create() — draft", () -> client.posts().create(map(
                    "caption", "SDK Java draft " + Instant.now().getEpochSecond(),
                    "account_ids", List.of(finalTestAccountId),
                    "status", "draft"
            )));
            if (draftPost != null && draftPost.hasNonNull("id")) {
                String draftPostId = draftPost.path("id").asText();
                createdPostIds.add(draftPostId);
                test("posts.get()", () -> {
                    JsonNode res = client.posts().get(draftPostId);
                    assertEquals(draftPostId, res.path("id").asText(), "Expected matching post");
                });
                test("posts.getQueue()", () -> {
                    JsonNode res = client.posts().getQueue(draftPostId);
                    assertTrue(res.isObject(), "Expected queue snapshot");
                });
                test("posts.analytics()", () -> {
                    List<JsonNode> res = client.posts().analytics(draftPostId);
                    assertNotNull(res, "Expected analytics list");
                });
                test("posts.update()", () -> {
                    JsonNode res = client.posts().update(draftPostId, map(
                            "caption", "SDK Java draft updated",
                            "account_ids", List.of(finalTestAccountId)
                    ));
                    assertEquals(draftPostId, res.path("id").asText(), "Expected matching updated post");
                });
                test("posts.previewLink()", () -> {
                    JsonNode res = client.posts().previewLink(draftPostId);
                    assertTrue(res.has("preview_url") || res.has("url"), "Expected preview link");
                });
                test("posts.archive()", () -> {
                    JsonNode res = client.posts().archive(draftPostId);
                    assertEquals(draftPostId, res.path("id").asText(), "Expected archived post");
                });
                test("posts.restore()", () -> {
                    JsonNode res = client.posts().restore(draftPostId);
                    assertEquals(draftPostId, res.path("id").asText(), "Expected restored post");
                });
            }

            scheduledPost = testValue("posts.create() — scheduled", () -> client.posts().create(map(
                    "caption", "SDK Java scheduled " + Instant.now().getEpochSecond(),
                    "account_ids", List.of(finalTestAccountId),
                    "scheduled_at", Instant.now().plus(2, ChronoUnit.DAYS).toString()
            )));
            if (scheduledPost != null && scheduledPost.hasNonNull("id")) {
                String scheduledPostId = scheduledPost.path("id").asText();
                createdPostIds.add(scheduledPostId);
                test("posts.update() — scheduled post", () -> {
                    JsonNode res = client.posts().update(scheduledPostId, map(
                            "caption", "SDK Java scheduled updated",
                            "scheduled_at", Instant.now().plus(3, ChronoUnit.DAYS).toString()
                    ));
                    assertEquals(scheduledPostId, res.path("id").asText(), "Expected updated scheduled post");
                });
                test("posts.cancel()", () -> {
                    JsonNode res = client.posts().cancel(scheduledPostId);
                    assertEquals(scheduledPostId, res.path("id").asText(), "Expected canceled post");
                });
            }

            test("posts.bulkCreate()", () -> {
                List<JsonNode> res = client.posts().bulkCreate(List.of(
                        map("caption", "SDK Java bulk 1", "account_ids", List.of(finalTestAccountId), "status", "draft"),
                        map("caption", "SDK Java bulk 2", "account_ids", List.of(finalTestAccountId), "status", "draft")
                ));
                assertEquals(2, res.size(), "Expected bulk results");
            });

            if (TEST_PUBLISH_NOW && draftPost != null && draftPost.hasNonNull("id")) {
                String draftPostId = draftPost.path("id").asText();
                test("posts.publish()", () -> {
                    JsonNode res = client.posts().publish(draftPostId);
                    assertEquals(draftPostId, res.path("id").asText(), "Expected published post");
                });
            } else {
                skip("posts.publish()", "Set TEST_PUBLISH_NOW=true to run live publish");
            }
        } else {
            skip("posts.create() — draft", "No TEST_ACCOUNT_ID or connected accounts available");
            skip("posts.get()", "No draft post available");
            skip("posts.getQueue()", "No draft post available");
            skip("posts.analytics()", "No draft post available");
            skip("posts.update()", "No draft post available");
            skip("posts.previewLink()", "No draft post available");
            skip("posts.archive()", "No draft post available");
            skip("posts.restore()", "No draft post available");
            skip("posts.create() — scheduled", "No TEST_ACCOUNT_ID or connected accounts available");
            skip("posts.update() — scheduled post", "No scheduled post available");
            skip("posts.cancel()", "No scheduled post available");
            skip("posts.bulkCreate()", "No TEST_ACCOUNT_ID or connected accounts available");
            skip("posts.publish()", "No TEST_ACCOUNT_ID or connected accounts available");
        }

        if (firstPost != null) {
            JsonNode failedResult = findFailedResult(firstPost);
            if (failedResult != null) {
                String postId = firstPost.path("id").asText();
                String resultId = failedResult.path("id").asText();
                test("posts.retryResult() — conditional live retry", () -> {
                    JsonNode res = client.posts().retryResult(postId, resultId);
                    assertTrue(res.has("status"), "Expected retry result payload");
                });
            } else {
                skip("posts.retryResult()", "No failed post result available to retry safely");
            }
        } else {
            skip("posts.retryResult()", "No posts available");
        }

        section("8. Delivery jobs");
        test("deliveryJobs.list()", () -> {
            Page<JsonNode> page = client.deliveryJobs().list(Map.of("limit", 5));
            assertNotNull(page.getData(), "Expected jobs page");
        });
        test("deliveryJobs.summary()", () -> {
            JsonNode res = client.deliveryJobs().summary();
            assertTrue(res.isObject(), "Expected summary object");
        });
        Page<JsonNode> retryableJobs = client.deliveryJobs().list(Map.of("limit", 20, "states", List.of("pending", "retrying")));
        if (!retryableJobs.getData().isEmpty()) {
            JsonNode retryableJob = retryableJobs.getData().get(0);
            String jobId = retryableJob.path("id").asText();
            test("deliveryJobs.retry()/cancel() — conditional", () -> {
                JsonNode retried = client.deliveryJobs().retry(jobId);
                assertEquals(jobId, retried.path("id").asText(), "Expected retried job");
                JsonNode canceled = client.deliveryJobs().cancel(jobId);
                assertEquals(jobId, canceled.path("id").asText(), "Expected canceled job");
            });
        } else {
            skip("deliveryJobs.retry()/cancel()", "No pending/retrying delivery job available");
        }

        section("9. Analytics, usage, oauth");
        Instant fromTs = Instant.now().minus(30, ChronoUnit.DAYS);
        Instant toTs = Instant.now();
        String from = fromTs.toString().substring(0, 10);
        String to = toTs.toString().substring(0, 10);
        Map<String, Object> analyticsRange = map("from", from, "to", to);
        test("analytics.summary()", () -> {
            JsonNode res = client.analytics().summary(analyticsRange);
            assertTrue(res.isObject(), "Expected summary object");
        });
        test("analytics.trend()", () -> {
            JsonNode res = client.analytics().trend(analyticsRange);
            assertTrue(res.isObject(), "Expected trend object");
        });
        test("analytics.byPlatform()", () -> {
            List<JsonNode> res = client.analytics().byPlatform(analyticsRange);
            assertNotNull(res, "Expected by-platform list");
        });
        test("analytics.rollup()", () -> {
            JsonNode res = client.analytics().rollup(map(
                    "from", fromTs.toString(),
                    "to", toTs.toString(),
                    "granularity", "day"
            ));
            assertTrue(res.isObject(), "Expected rollup object");
        });
        test("usage.get()", () -> {
            JsonNode res = client.usage().get();
            assertTrue(res.isObject(), "Expected usage object");
        });
        test("oauth.connect()", () -> {
            try {
                JsonNode res = client.oauth().connect("bluesky", Map.of("redirect_url", "https://example.com/callback"));
                assertTrue(res.has("url") || res.has("auth_url"), "Expected auth URL");
            } catch (APIError error) {
                if (matchesCode(error, "unauthorized", "validation_error", "not_supported")
                        || containsAny(error.getMessage(), "does not support oauth")) {
                    return;
                }
                throw error;
            }
        });

        cleanup(client);
        summaryAndExit();
    }

    private static void testWebhookSignature() throws Exception {
        String secret = "whsec_validation";
        String payload = "{\"id\":\"evt_test\"}";
        String signature = "sha256=" + hmacHex(secret, payload);
        assertTrue(WebhookVerifier.verifySignature(secret, payload, signature), "Expected valid webhook signature");
    }

    private static JsonNode findFailedResult(JsonNode post) {
        JsonNode results = post.path("results");
        if (!results.isArray()) return null;
        for (JsonNode result : results) {
            if ("failed".equalsIgnoreCase(result.path("status").asText())) {
                return result;
            }
        }
        return null;
    }

    private static JsonNode pickStableProfile(List<JsonNode> profiles, JsonNode workspace) {
        if (profiles.isEmpty()) return null;
        String defaultProfileId = workspace == null ? null : firstPresent(workspace, "default_profile_id");
        if (defaultProfileId != null) {
            for (JsonNode profile : profiles) {
                if (defaultProfileId.equals(profile.path("id").asText())) {
                    return profile;
                }
            }
        }
        for (JsonNode profile : profiles) {
            if (!profile.path("name").asText("").startsWith("SDK ")) {
                return profile;
            }
        }
        return profiles.get(0);
    }

    private static JsonNode findByPlatform(List<JsonNode> accounts, String platform) {
        for (JsonNode account : accounts) {
            if (platform.equals(account.path("platform").asText())) {
                return account;
            }
        }
        return null;
    }

    private static String firstPresent(JsonNode node, String... names) {
        for (String name : names) {
            JsonNode child = node.path(name);
            if (!child.isMissingNode() && !child.isNull() && !child.asText().isBlank()) {
                return child.asText();
            }
        }
        return null;
    }

    private static void maybePut(Map<String, Object> body, String key, JsonNode value) {
        if (value != null && !value.isMissingNode() && !value.isNull()) {
            body.put(key, value.asText());
        }
    }

    private static void banner() {
        System.out.println("\n╔══════════════════════════════════════════════════╗");
        System.out.println("║    sdk-java — API Validation Test               ║");
        System.out.println("╚══════════════════════════════════════════════════╝\n");
    }

    private static void section(String title) {
        System.out.println("\n──────────────────────────────────────────────────");
        System.out.println("  " + title);
        System.out.println("──────────────────────────────────────────────────");
    }

    private static void test(String name, ThrowingRunnable fn) {
        System.out.print("  " + name + " ... ");
        try {
            fn.run();
            System.out.println("✅ PASS");
            passed++;
        } catch (Exception error) {
            if (isPlanGated(error)) {
                System.out.println("⏭ SKIP — Plan-gated (" + ((APIError) error).getCode() + ")");
                skipped++;
                return;
            }
            System.out.println("❌ FAIL — " + error.getMessage());
            failed++;
            failures.add(name + ": " + error.getMessage());
        }
    }

    private static <T> T testValue(String name, ThrowingSupplier<T> fn) {
        final Holder<T> holder = new Holder<>();
        test(name, () -> holder.value = fn.get());
        return holder.value;
    }

    private static void expectApiError(String name, ThrowingSupplier<JsonNode> fn, List<String> expectedCodes) {
        test(name, () -> {
            try {
                fn.get();
            } catch (APIError error) {
                if (!expectedCodes.isEmpty() && !expectedCodes.contains(error.getCode())) {
                    throw new IllegalStateException("Expected " + String.join("/", expectedCodes) + " but got " + error.getCode());
                }
                return;
            }
            throw new IllegalStateException("Expected API error");
        });
    }

    private static void skip(String name, String reason) {
        System.out.println("  " + name + " ... ⏭ SKIP — " + reason);
        skipped++;
    }

    private static boolean isPlanGated(Exception error) {
        if (!(error instanceof APIError)) return false;
        String code = ((APIError) error).getCode();
        if (code == null || code.isBlank()) return false;
        return List.of(
                "plan_feature_not_available",
                "plan_platform_not_allowed",
                "profile_limit_reached",
                "member_limit_reached"
        ).contains(code);
    }

    private static void cleanup(UniPost client) {
        if (createdWebhookIds.isEmpty() && createdMediaIds.isEmpty() && createdPostIds.isEmpty() && createdPlatformCredentialKeys.isEmpty()) {
            return;
        }
        section("Cleanup");
        for (String webhookId : new ArrayList<>(createdWebhookIds)) {
            try {
                client.webhooks().delete(webhookId);
                System.out.println("  🧹 Deleted webhook " + webhookId.substring(0, Math.min(8, webhookId.length())) + "...");
            } catch (Exception error) {
                System.out.println("  ⚠ Failed to delete webhook " + webhookId + " (" + error.getMessage() + ")");
            }
        }
        for (String mediaId : new ArrayList<>(createdMediaIds)) {
            try {
                client.media().delete(mediaId);
                System.out.println("  🧹 Deleted media " + mediaId.substring(0, Math.min(8, mediaId.length())) + "...");
            } catch (Exception error) {
                System.out.println("  ⚠ Failed to delete media " + mediaId + " (" + error.getMessage() + ")");
            }
        }
        for (String postId : new ArrayList<>(createdPostIds)) {
            try {
                client.posts().delete(postId);
                System.out.println("  🧹 Deleted post " + postId.substring(0, Math.min(8, postId.length())) + "...");
            } catch (Exception error) {
                System.out.println("  ⚠ Failed to delete post " + postId + " (" + error.getMessage() + ")");
            }
        }
        for (String platform : new ArrayList<>(createdPlatformCredentialKeys)) {
            try {
                client.platformCredentials().delete(platform);
                System.out.println("  🧹 Deleted platform credential " + platform);
            } catch (Exception error) {
                System.out.println("  ⚠ Failed to delete platform credential " + platform + " (" + error.getMessage() + ")");
            }
        }
    }

    private static void summaryAndExit() {
        System.out.println("\n──────────────────────────────────────────────────");
        System.out.println("Summary");
        System.out.println("──────────────────────────────────────────────────");
        System.out.println("  Passed:  " + passed);
        System.out.println("  Skipped: " + skipped);
        System.out.println("  Failed:  " + failed);
        if (!failures.isEmpty()) {
            System.out.println("\nFailures:");
            for (String failure : failures) {
                System.out.println("  - " + failure);
            }
        }
        if (failed > 0) {
            System.exit(1);
        }
    }

    private static void assertTrue(boolean condition, String message) {
        if (!condition) {
            throw new IllegalStateException(message);
        }
    }

    private static void assertNotNull(Object value, String message) {
        if (value == null) {
            throw new IllegalStateException(message);
        }
    }

    private static void assertEquals(Object expected, Object actual, String message) {
        if (!Objects.equals(expected, actual)) {
            throw new IllegalStateException(message + " (expected=" + expected + ", actual=" + actual + ")");
        }
    }

    @SafeVarargs
    private static Map<String, Object> map(Object... pairs) {
        Map<String, Object> out = new LinkedHashMap<>();
        for (int i = 0; i < pairs.length; i += 2) {
            String key = String.valueOf(pairs[i]);
            Object value = pairs[i + 1];
            if (value != null) {
                out.put(key, value);
            }
        }
        return out;
    }

    private static String env(String key, String fallback) {
        String value = System.getenv(key);
        return value == null ? fallback : value;
    }

    private static String finalProfileId(JsonNode profile) {
        return profile == null ? null : profile.path("id").asText();
    }

    private static boolean matchesCode(APIError error, String... expectedCodes) {
        String code = error.getCode();
        if (code == null || code.isBlank()) return false;
        for (String expectedCode : expectedCodes) {
            if (expectedCode.equalsIgnoreCase(code)) {
                return true;
            }
        }
        return false;
    }

    private static boolean containsAny(String value, String... expectedParts) {
        if (value == null || value.isBlank()) return false;
        String normalized = value.toLowerCase();
        for (String expectedPart : expectedParts) {
            if (normalized.contains(expectedPart.toLowerCase())) {
                return true;
            }
        }
        return false;
    }

    private static String hmacHex(String secret, String payload) throws Exception {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(secret.getBytes(StandardCharsets.UTF_8), "HmacSHA256"));
        byte[] digest = mac.doFinal(payload.getBytes(StandardCharsets.UTF_8));
        StringBuilder out = new StringBuilder(digest.length * 2);
        for (byte b : digest) {
            out.append(String.format("%02x", b));
        }
        return out.toString();
    }

    @FunctionalInterface
    private interface ThrowingRunnable {
        void run() throws Exception;
    }

    @FunctionalInterface
    private interface ThrowingSupplier<T> {
        T get() throws Exception;
    }

    private static final class Holder<T> {
        private T value;
    }
}
