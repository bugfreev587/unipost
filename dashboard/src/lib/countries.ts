let countryNames: Intl.DisplayNames | null = null;

export function normalizeCountryCode(code?: string | null): string {
  const normalized = (code || "").trim().toUpperCase();
  if (!/^[A-Z]{2}$/.test(normalized) || normalized === "XX" || normalized === "T1") {
    return "";
  }
  return normalized;
}

export function countryNameFromCode(code?: string | null): string {
  const normalized = normalizeCountryCode(code);
  if (!normalized) return "";
  try {
    countryNames ??= new Intl.DisplayNames(["en"], { type: "region" });
    return countryNames.of(normalized) || normalized;
  } catch {
    return normalized;
  }
}

export function countryDisplay(code?: string | null): string {
  const normalized = normalizeCountryCode(code);
  if (!normalized) return "Unknown";
  const name = countryNameFromCode(normalized);
  return name ? `${name} (${normalized})` : normalized;
}
