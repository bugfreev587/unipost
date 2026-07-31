import assert from "node:assert/strict";
import test from "node:test";

import {
  isPinterestBoardSnapshotFresh,
  reconcilePinterestBoardSelection,
  type PinterestBoardSnapshot,
} from "./pinterest-boards.ts";

const fetchedAt = Date.UTC(2026, 6, 31, 12, 0, 0);

function snapshot(overrides: Partial<PinterestBoardSnapshot> = {}): PinterestBoardSnapshot {
  return {
    accountId: "account_pin_1",
    environment: "production",
    fetchedAt,
    boardIds: ["101", "202"],
    ...overrides,
  };
}

test("reconcilePinterestBoardSelection keeps a current selection", () => {
  assert.equal(
    reconcilePinterestBoardSelection("202", snapshot(), snapshot({ fetchedAt: fetchedAt + 1_000 })),
    "202",
  );
});

test("reconcilePinterestBoardSelection clears a missing board", () => {
  assert.equal(
    reconcilePinterestBoardSelection("202", snapshot(), snapshot({ boardIds: ["101"] })),
    "",
  );
});

test("reconcilePinterestBoardSelection clears on account or environment change", () => {
  assert.equal(
    reconcilePinterestBoardSelection("202", snapshot(), snapshot({ accountId: "account_pin_2" })),
    "",
  );
  assert.equal(
    reconcilePinterestBoardSelection("202", snapshot(), snapshot({ environment: "sandbox" })),
    "",
  );
});

test("reconcilePinterestBoardSelection does not accept an unproven initial selection", () => {
  assert.equal(reconcilePinterestBoardSelection("202", null, snapshot()), "202");
  assert.equal(reconcilePinterestBoardSelection("303", null, snapshot()), "");
});

test("isPinterestBoardSnapshotFresh uses a non-sliding 60 second TTL", () => {
  const value = snapshot();
  assert.equal(isPinterestBoardSnapshotFresh(value, fetchedAt + 59_999), true);
  assert.equal(isPinterestBoardSnapshotFresh(value, fetchedAt + 60_000), false);
  assert.equal(isPinterestBoardSnapshotFresh(value, fetchedAt - 1), false);
});
