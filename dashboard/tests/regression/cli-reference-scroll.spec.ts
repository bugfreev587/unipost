import { expect, test, type Locator, type Page } from "@playwright/test";

async function firstVisibleLineNumber(row: Locator) {
  return row.evaluate((element) => {
    const editor = element.querySelector(".monaco-editor");
    const editorRect = editor?.getBoundingClientRect();
    if (!editorRect) return null;

    const lineNumbers = [...element.querySelectorAll(".line-numbers")]
      .map((node) => {
        const rect = node.getBoundingClientRect();
        return {
          text: node.textContent?.trim() || null,
          top: rect.top,
          bottom: rect.bottom,
          height: rect.height,
        };
      })
      .filter((line) => (
        line.text
        && line.height > 0
        && line.bottom > editorRect.top + 1
        && line.top < editorRect.bottom - 1
      ))
      .sort((a, b) => a.top - b.top);

    return lineNumbers[0]?.text ?? null;
  });
}

async function scrollResponse(page: Page, row: Locator) {
  const editor = row.locator(".monaco-editor").first();
  const box = await editor.boundingBox();
  expect(box).not.toBeNull();

  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);
  await page.mouse.wheel(0, 700);
}

test("CLI reference response returns to the first line when reopened", async ({ page }) => {
  await page.goto("/docs/cli/reference", { waitUntil: "networkidle" });

  const row = page.locator("details.cli-command-row", { hasText: "auth login --setup-token" }).first();
  await row.locator("summary").click();
  await expect(row.locator(".monaco-editor").first()).toBeVisible();

  await expect.poll(() => firstVisibleLineNumber(row)).toBe("1");

  await scrollResponse(page, row);
  await expect.poll(() => firstVisibleLineNumber(row)).not.toBe("1");

  await row.locator("summary").click();
  await expect(row).not.toHaveAttribute("open", "");

  await row.locator("summary").click();
  await expect(row.locator(".monaco-editor").first()).toBeVisible();

  await expect.poll(() => firstVisibleLineNumber(row)).toBe("1");
});
