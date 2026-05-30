const SECRET_PATTERNS = [
  /(password|passcode|verification code|2fa|token|secret|api key)\s*[:=]?\s*\S+/gi,
  /sk-ant-[A-Za-z0-9_-]+/g,
  /Bearer\s+[A-Za-z0-9._-]+/g,
];

export async function collectPageObservation(page, { jobId = "", stepKey = "" } = {}) {
  const currentUrl = typeof page.url === "function" ? page.url() : "";
  const title = await safePageTitle(page);
  const texts = await page
    .locator("body")
    .allInnerTexts()
    .catch(() => []);
  const domHints = await page
    .evaluate(() => {
      return Array.from(document.querySelectorAll("[data-review-step],button,a,input,textarea,select"))
        .filter((el) => {
          const rect = el.getBoundingClientRect();
          return rect.width > 0 && rect.height > 0;
        })
        .slice(0, 80)
        .map((el) => {
          const reviewStep = el.getAttribute("data-review-step") || "";
          return {
            role: el.getAttribute("role") || el.tagName.toLowerCase(),
            text: (el.innerText || el.value || el.getAttribute("aria-label") || el.getAttribute("placeholder") || "")
              .trim()
              .slice(0, 160),
            selector_hint: reviewStep ? `[data-review-step='${reviewStep}']` : "",
          };
        });
    })
    .catch(() => []);

  return redactObservation({
    job_id: jobId,
    step_key: stepKey,
    current_url: currentUrl,
    page_title: title,
    visible_text: texts.join("\n").slice(0, 12000),
    dom_hints: domHints,
  });
}

export function redactObservation(observation = {}) {
  return {
    ...observation,
    visible_text: redactText(observation.visible_text || ""),
    dom_hints: (observation.dom_hints || [])
      .filter((hint) => {
        const joined = `${hint.role || ""} ${hint.text || ""} ${hint.selector_hint || ""}`.toLowerCase();
        return !joined.includes("password") && !joined.includes("verification code") && !joined.includes("2fa");
      })
      .map((hint) => ({ ...hint, text: redactText(hint.text || "") })),
  };
}

async function safePageTitle(page) {
  if (typeof page.title !== "function") {
    return "";
  }
  try {
    return await page.title();
  } catch {
    return "";
  }
}

function redactText(value) {
  let out = String(value || "");
  for (const pattern of SECRET_PATTERNS) {
    out = out.replace(pattern, "[redacted]");
  }
  return out;
}
