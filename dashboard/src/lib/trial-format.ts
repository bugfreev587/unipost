import type {
  AdminWorkspaceTrial,
  TrialMutationTrial,
  WorkspaceTrial,
  WorkspaceTrialHistoryEntry,
} from "./api";

export type TrialBadgeTone = "neutral" | "info" | "success" | "warning" | "danger";

export interface TrialTimelineItem {
  label: string;
  value: string;
}

export interface TrialFormatInput {
  id: string;
  kind: string;
  plan_id: string;
  duration_days: number;
  status: string;
  granted_at?: string;
  scheduled_start_at?: string;
  started_at?: string;
  ends_at?: string;
  activated_at?: string;
  canceled_at?: string;
  revoked_at?: string;
  superseded_at?: string;
  completed_at?: string;
  superseded_by_plan_id?: string;
  post_trial_price_cents?: number;
  cancel_at_period_end?: boolean;
  changing_plan_forfeits_trial?: boolean;
  terminal_reason?: string;
  failure_reason?: string;
}

export type TrialFormatSource =
  | WorkspaceTrial
  | WorkspaceTrialHistoryEntry
  | AdminWorkspaceTrial
  | TrialMutationTrial;

export interface TrialFormatOptions {
  now?: Date | string | number;
  locale?: string;
  timeZone?: string;
  currency?: string;
}

export interface FormattedWorkspaceTrial {
  badge: { label: string; tone: TrialBadgeTone };
  headline: string;
  timeline: TrialTimelineItem[];
  remainingDays: number | null;
  terminalReason: string | null;
}

const DAY_MS = 24 * 60 * 60 * 1000;

const STATUS_BADGES: Record<string, FormattedWorkspaceTrial["badge"]> = {
  provisioning: { label: "Setting up", tone: "info" },
  pending_activation: { label: "Trial available", tone: "success" },
  checkout_pending: { label: "Activation in progress", tone: "info" },
  scheduled: { label: "Scheduled", tone: "info" },
  active: { label: "Trial active", tone: "success" },
  completed: { label: "Completed", tone: "neutral" },
  canceled: { label: "Canceled", tone: "warning" },
  revoked: { label: "Revoked", tone: "warning" },
  superseded: { label: "Ended", tone: "neutral" },
  failed: { label: "Failed", tone: "danger" },
};

const TERMINAL_REASONS: Record<string, string> = {
  trial_completed: "Trial completed",
  renewal_canceled: "Renewal canceled",
  offer_revoked: "Offer revoked",
  plan_changed: "Plan changed",
  trial_unavailable: "Trial unavailable",
};

const STATUS_TERMINAL_REASONS: Record<string, string> = {
  completed: "Trial completed",
  canceled: "Renewal canceled",
  revoked: "Offer revoked",
  superseded: "Plan changed",
  failed: "Trial unavailable",
};

const PLAN_LABELS: Record<string, string> = {
  api: "API",
  basic: "Basic",
  growth: "Growth",
  team: "Team",
};

function planLabel(planId: string): string {
  if (PLAN_LABELS[planId]) return PLAN_LABELS[planId];
  const normalized = planId.trim().replaceAll("_", " ");
  if (!normalized) return "Plan";
  return normalized.replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function validDate(value: Date | string | number | undefined): Date | null {
  if (value === undefined) return null;
  const date = value instanceof Date ? new Date(value.getTime()) : new Date(value);
  return Number.isFinite(date.getTime()) ? date : null;
}

function formatDate(value: string | undefined, options: TrialFormatOptions): string | null {
  const date = validDate(value);
  if (!date) return null;
  return new Intl.DateTimeFormat(options.locale ?? "en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    ...(options.timeZone ? { timeZone: options.timeZone } : {}),
  }).format(date);
}

function formatMonthlyPrice(cents: number | undefined, options: TrialFormatOptions): string | null {
  if (typeof cents !== "number" || !Number.isFinite(cents) || cents < 0) return null;
  return `${new Intl.NumberFormat(options.locale ?? "en-US", {
    style: "currency",
    currency: options.currency ?? "USD",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(cents / 100)}/month`;
}

function datedValue(date: string | null, suffix: string | null): string | null {
  if (date && suffix) return `${date} · ${suffix}`;
  return date ?? suffix;
}

function pushDate(timeline: TrialTimelineItem[], label: string, value: string | null): void {
  if (value) timeline.push({ label, value });
}

function formatTimeline(trial: TrialFormatInput, options: TrialFormatOptions): TrialTimelineItem[] {
  const timeline: TrialTimelineItem[] = [];
  const granted = formatDate(trial.granted_at, options);
  const scheduledStart = formatDate(trial.scheduled_start_at, options);
  const started = formatDate(trial.started_at ?? trial.activated_at, options);
  const ends = formatDate(trial.ends_at, options);
  const monthlyPrice = formatMonthlyPrice(trial.post_trial_price_cents, options);

  switch (trial.status) {
    case "provisioning":
      pushDate(timeline, "Granted", granted);
      timeline.push({ label: "Setup", value: "In progress" });
      break;
    case "pending_activation":
      pushDate(timeline, "Granted", granted);
      timeline.push({ label: "Activation", value: "Complete checkout to start" });
      break;
    case "checkout_pending":
      pushDate(timeline, "Granted", granted);
      timeline.push({ label: "Activation", value: "Checkout in progress" });
      break;
    case "scheduled":
      if (trial.kind === "paid_same_plan") {
        pushDate(timeline, "Current period ends", scheduledStart);
      }
      pushDate(timeline, "Trial starts", scheduledStart);
      pushDate(timeline, "Trial ends", ends);
      if (trial.cancel_at_period_end) {
        pushDate(timeline, "Access ends", ends);
      } else {
        pushDate(timeline, "Billing resumes", datedValue(ends, monthlyPrice));
      }
      break;
    case "active":
      pushDate(timeline, "Trial started", started);
      pushDate(timeline, "Trial ends", ends);
      if (trial.cancel_at_period_end) {
        pushDate(timeline, "Access ends", ends);
      } else {
        pushDate(timeline, "Billing resumes", datedValue(ends, monthlyPrice));
      }
      break;
    case "completed":
      pushDate(timeline, "Granted", granted);
      pushDate(timeline, "Trial started", started);
      pushDate(timeline, "Completed", formatDate(trial.completed_at ?? trial.ends_at, options));
      break;
    case "canceled":
      pushDate(timeline, "Granted", granted);
      pushDate(timeline, "Trial started", started);
      pushDate(timeline, "Canceled", formatDate(trial.canceled_at ?? trial.ends_at, options));
      break;
    case "revoked":
      pushDate(timeline, "Granted", granted);
      pushDate(timeline, "Revoked", formatDate(trial.revoked_at, options));
      break;
    case "superseded":
      pushDate(timeline, "Granted", granted);
      pushDate(timeline, "Trial started", started);
      pushDate(timeline, "Plan changed", formatDate(trial.superseded_at, options));
      if (trial.superseded_by_plan_id) {
        timeline.push({ label: "New plan", value: planLabel(trial.superseded_by_plan_id) });
      }
      break;
    case "failed":
      pushDate(timeline, "Granted", granted);
      timeline.push({ label: "Setup", value: "Unavailable" });
      break;
  }
  return timeline;
}

function headline(trial: TrialFormatInput): string {
  const plan = planLabel(trial.plan_id);
  const duration = `${trial.duration_days}-day`;
  switch (trial.status) {
    case "provisioning": return `${plan} trial setup in progress`;
    case "pending_activation": return `${duration} ${plan} trial available`;
    case "checkout_pending": return `${plan} trial activation in progress`;
    case "scheduled": return `${duration} ${plan} trial scheduled`;
    case "active": return `${plan} trial active`;
    case "completed": return `${plan} trial completed`;
    case "canceled": return `${plan} trial canceled`;
    case "revoked": return `${plan} trial offer revoked`;
    case "superseded": return `${plan} trial ended after plan change`;
    case "failed": return `${plan} trial unavailable`;
    default: return "Trial status unavailable";
  }
}

function remainingDays(trial: TrialFormatInput, options: TrialFormatOptions): number | null {
  if (trial.status !== "active") return null;
  const end = validDate(trial.ends_at);
  const now = validDate(options.now ?? new Date());
  if (!end || !now) return null;
  return Math.max(0, Math.ceil((end.getTime() - now.getTime()) / DAY_MS));
}

/**
 * Produces the single user-facing trial vocabulary shared by Pricing,
 * Settings Billing, Admin Billing, and Trial History. Dates use the viewer's
 * local timezone unless a caller explicitly supplies one.
 */
export function formatWorkspaceTrial(
  source: TrialFormatSource | TrialFormatInput,
  options: TrialFormatOptions = {},
): FormattedWorkspaceTrial {
  const trial: TrialFormatInput = source;
  const knownBadge = STATUS_BADGES[trial.status];
  const reasonKey = trial.terminal_reason ?? trial.failure_reason;
  return {
    badge: knownBadge ?? { label: "Unknown", tone: "neutral" },
    headline: headline(trial),
    timeline: formatTimeline(trial, options),
    remainingDays: remainingDays(trial, options),
    terminalReason: (reasonKey && TERMINAL_REASONS[reasonKey]) || STATUS_TERMINAL_REASONS[trial.status] || null,
  };
}
