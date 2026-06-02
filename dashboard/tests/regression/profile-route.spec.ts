import { expect, test } from "@playwright/test";
import { getCanonicalProjectPath } from "../../src/lib/profile-route";

const profiles = [
  { id: "default-profile" },
  { id: "secondary-profile" },
];

test.describe("project profile routing", () => {
  test("redirects stale project profile routes to the first available profile", () => {
    expect(
      getCanonicalProjectPath({
        pathname: "/projects/stale-profile/accounts/native",
        profiles,
      }),
    ).toBe("/projects/default-profile/accounts/native");
  });

  test("keeps valid project profile routes unchanged", () => {
    expect(
      getCanonicalProjectPath({
        pathname: "/projects/secondary-profile/accounts/native",
        profiles,
      }),
    ).toBeNull();
  });

  test("does not rewrite the new profile route", () => {
    expect(
      getCanonicalProjectPath({
        pathname: "/projects/new",
        profiles,
      }),
    ).toBeNull();
  });
});
