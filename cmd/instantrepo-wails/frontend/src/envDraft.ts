import type {
  AnalyzeSnapshot,
  EnvDraft,
  EnvDraftTarget,
  EnvVaultBinding,
} from "./types";

const envLinePattern = /^\s*(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)\s*$/;

function cloneDraft(draft: EnvDraft): EnvDraft {
  return {
    ...draft,
    targets: draft.targets.map((target) => ({
      ...target,
      values: target.values.map((value) => ({ ...value })),
    })),
  };
}

export function updateEnvDraftValue(
  draft: EnvDraft,
  relativePath: string,
  name: string,
  value: string,
): EnvDraft {
  const next = cloneDraft(draft);
  for (const target of next.targets) {
    if (target.relativePath !== relativePath) {
      continue;
    }
    for (const item of target.values) {
      if (item.name === name) {
        item.value = value;
        item.vaultBinding = undefined;
      }
    }
  }
  return next;
}

export function clearVaultBinding(
  draft: EnvDraft,
  relativePath: string,
  name: string,
): EnvDraft {
  const next = cloneDraft(draft);
  for (const target of next.targets) {
    if (target.relativePath !== relativePath) {
      continue;
    }
    for (const item of target.values) {
      if (item.name === name) {
        item.vaultBinding = undefined;
        item.value = "";
      }
    }
  }
  return next;
}

export function renderRawTarget(target: EnvDraftTarget): string {
  return (
    target.values
      .map((value) => {
        const rendered = value.vaultBinding
          ? `<vault:${value.vaultBinding.fingerprint}>`
          : value.value;
        return `${value.name}=${rendered}`;
      })
      .join("\n") + "\n"
  );
}

export function applyRawTargetContent(
  draft: EnvDraft,
  relativePath: string,
  content: string,
): EnvDraft {
  const values = parseRawValues(content);
  const next = cloneDraft(draft);
  for (const target of next.targets) {
    if (target.relativePath !== relativePath) {
      continue;
    }
    for (const item of target.values) {
      if (!values.has(item.name)) {
        if (item.vaultBinding) {
          item.value = "";
          item.vaultBinding = undefined;
        }
        continue;
      }
      const nextValue = values.get(item.name) ?? "";
      if (nextValue.startsWith("<vault:") && nextValue.endsWith(">")) {
        continue;
      }
      item.value = nextValue;
      item.vaultBinding = undefined;
    }
    const knownNames = new Set(target.values.map((item) => item.name));
    for (const [name, value] of values) {
      if (knownNames.has(name)) {
        continue;
      }
      target.values.push({
        name,
        value,
        secret: false,
        confidence: 1,
        provenance: { source: "draft" },
      });
    }
  }
  return next;
}

export function vaultBindingLabel(binding: EnvVaultBinding): string {
  return binding.displayName?.trim() || binding.label?.trim() || "Vault value";
}

export function canBuildEnvDraftFromPlan(snapshot: AnalyzeSnapshot): boolean {
  if (snapshot.plan.env.targetPath?.trim()) {
    return true;
  }
  return snapshot.plan.env.variables.some((item) => item.targetDir?.trim());
}

function parseRawValues(content: string) {
  const values = new Map<string, string>();
  for (const line of content.split("\n")) {
    const match = envLinePattern.exec(line.trimEnd());
    if (!match) {
      continue;
    }
    values.set(match[1], cleanRawEnvValue(match[2]));
  }
  return values;
}

function cleanRawEnvValue(value: string) {
  const trimmed = value.trim();
  if (
    (trimmed.startsWith('"') && trimmed.endsWith('"')) ||
    (trimmed.startsWith("'") && trimmed.endsWith("'"))
  ) {
    return trimmed.slice(1, -1);
  }
  return trimmed;
}
