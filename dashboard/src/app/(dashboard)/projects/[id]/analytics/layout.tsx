import { PlanGate } from "@/components/dashboard/plan-gate";

export default function AnalyticsLayout({ children }: { children: React.ReactNode }) {
  return <PlanGate feature="analytics">{children}</PlanGate>;
}
