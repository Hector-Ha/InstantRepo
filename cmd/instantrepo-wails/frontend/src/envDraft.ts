import type { EnvDraft, EnvDraftTarget } from "./types";

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
        continue;
      }
      const nextValue = values.get(item.name) ?? "";
      if (nextValue.startsWith("<vault:") && nextValue.endsWith(">")) {
        continue;
      }
      item.value = nextValue;
      item.vaultBinding = undefined;
    }
  }
  return next;
}

function parseRawValues(content: string) {
  const values = new Map<string, string>();
  for (const line of content.split("\n")) {
    const match = envLinePattern.exec(line.trimEnd());
    if (!match) {
      continue;
    }
    values.set(match[1], match[2].trim());
  }
  return values;
}
