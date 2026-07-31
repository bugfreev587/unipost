export type PinterestBoardEnvironment = "production" | "sandbox";

export type PinterestBoardSnapshot = {
  accountId: string;
  environment: PinterestBoardEnvironment;
  fetchedAt: number;
  boardIds: readonly string[];
};

export const PINTEREST_BOARD_SNAPSHOT_TTL_MS = 60_000;

export function reconcilePinterestBoardSelection(
  selectedBoardId: string,
  previous: PinterestBoardSnapshot | null,
  current: PinterestBoardSnapshot,
): string {
  const selected = selectedBoardId.trim();
  if (!selected) return "";
  if (
    previous &&
    (previous.accountId !== current.accountId || previous.environment !== current.environment)
  ) {
    return "";
  }
  return current.boardIds.includes(selected) ? selected : "";
}

export function isPinterestBoardSnapshotFresh(
  snapshot: PinterestBoardSnapshot,
  now: number,
): boolean {
  const age = now - snapshot.fetchedAt;
  return age >= 0 && age < PINTEREST_BOARD_SNAPSHOT_TTL_MS;
}

export function pinterestBoardEnvironment(sandboxMode: boolean): PinterestBoardEnvironment {
  return sandboxMode ? "sandbox" : "production";
}
