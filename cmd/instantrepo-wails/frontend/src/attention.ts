import type {
  AnalyzeSnapshot,
  EnvVarRequirement,
  RequirementGap,
  SafetyFinding,
} from "./types";

export function getMissingRequiredTools(
  snapshot?: AnalyzeSnapshot | null,
): RequirementGap[] {
  return (
    snapshot?.plan.gaps.filter(
      (gap) => gap.status === "missing" || gap.status === "version_mismatch",
    ) ?? []
  );
}

export function getSafetyAttention(
  snapshot?: AnalyzeSnapshot | null,
): SafetyFinding[] {
  return snapshot?.plan.safety.findings ?? [];
}

export function getUnresolvedEnv(
  snapshot?: AnalyzeSnapshot | null,
): EnvVarRequirement[] {
  return (
    snapshot?.plan.env.variables.filter(
      (item) =>
        item.currentStatus !== "configured" &&
        item.fillStrategy === "user_required",
    ) ?? []
  );
}

export function envRequirementKey(item: EnvVarRequirement): string {
  return `${item.name}\u0000${item.targetDir ?? ""}`;
}

export function envRequirementTargetLabel(
  item: EnvVarRequirement,
  repoPath?: string,
): string {
  const targetDir = item.targetDir?.trim();
  if (!targetDir) {
    return "";
  }
  const repo = repoPath?.trim();
  if (!repo) {
    return targetDir;
  }
  const repoParts = splitPath(repo);
  const targetParts = splitPath(targetDir);
  if (targetParts.length <= repoParts.length) {
    return targetDir;
  }
  for (let index = 0; index < repoParts.length; index += 1) {
    if (repoParts[index].toLowerCase() !== targetParts[index].toLowerCase()) {
      return targetDir;
    }
  }
  return targetParts.slice(repoParts.length).join("\\") || targetDir;
}

function splitPath(path: string): string[] {
  return path.split(/[\\/]+/).filter(Boolean);
}
