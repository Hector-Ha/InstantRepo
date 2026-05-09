import type { AnalyzeSnapshot, RequirementGap, SafetyFinding } from "./types";

export function getMissingRequiredTools(
  snapshot?: AnalyzeSnapshot | null,
): RequirementGap[] {
  return snapshot?.plan.gaps.filter((gap) => gap.status === "missing") ?? [];
}

export function getSafetyAttention(
  snapshot?: AnalyzeSnapshot | null,
): SafetyFinding[] {
  return snapshot?.plan.safety.findings ?? [];
}
