import test from "node:test";
import assert from "node:assert/strict";
import { getConnectErrorPresentation } from "../src/lib/connect-errors.ts";

test("Facebook Hosted Connect reasons have actionable presentations", () => {
  assert.deepEqual(
    getConnectErrorPresentation("facebook_page_not_available", "facebook"),
    {
      title: "Facebook Page unavailable",
      body: "We couldn’t find a Facebook Page this account can manage or has allowed UniPost to access. Make sure this Facebook account manages a Page and that UniPost is allowed to access it, or ask a Page admin to grant you access. Then open the original connection link and try again.",
    },
  );
  assert.deepEqual(
    getConnectErrorPresentation("facebook_page_permission_required", "facebook"),
    {
      title: "Facebook Page permission required",
      body: "Your Facebook account can access a Page, but it doesn’t have permission to publish content. Ask a Page admin to grant you Facebook content-management access, then open the original connection link and try again.",
    },
  );
  assert.equal(
    getConnectErrorPresentation("unexpected-provider-body", "facebook").body,
    "Facebook authorization couldn’t be completed. Please try again later or contact the developer who sent you the link.",
  );
});

test("non-Facebook unknown reasons preserve existing behavior", () => {
  assert.equal(getConnectErrorPresentation("legacy_reason", "linkedin").body, "legacy_reason");
});
