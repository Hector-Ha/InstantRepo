import { ClonePreflight as runClonePreflight } from "./desktopApi";

export type ClonePreflightAction =
  | "clone"
  | "clone-with-attention"
  | "open-existing"
  | "choose-different-folder"
  | "free-disk-space";

export interface InstalledRepo {
  id: number;
  rawUrl?: string;
  normalizedUrl?: string;
  localPath: string;
  status: string;
  createdAt: string;
  updatedAt: string;
  lastAnalyzedAt: string;
}

export interface CloneDiskStatus {
  status: "ok" | "warn" | "block";
  freeBytes?: number;
  reason?: string;
}

export interface ClonePreflightMessage {
  severity: string;
  text: string;
}

export interface ClonePreflightResponse {
  repoUrl: string;
  normalizedUrl: string;
  destinationRoot: string;
  destinationWritable: boolean;
  targetPath: string;
  targetExists: boolean;
  targetEmpty: boolean;
  duplicateRepos: InstalledRepo[];
  pathConflict: boolean;
  pathConflictRepos: InstalledRepo[];
  disk: CloneDiskStatus;
  recommendedAction: ClonePreflightAction;
  messages: ClonePreflightMessage[];
}

export type ClonePreflightPlan =
  | { kind: "clone"; message: string }
  | { kind: "confirm-clone"; message: string }
  | { kind: "open-existing"; localPath: string; message: string }
  | { kind: "block"; message: string };

interface ClonePreflightPlanOptions {
  forceClone?: boolean;
}

export function clonePreflight(repoURL: string, destinationRoot: string) {
  return runClonePreflight(repoURL, destinationRoot);
}

export function planClonePreflightFlow(
  preflight: ClonePreflightResponse,
  options: ClonePreflightPlanOptions = {},
): ClonePreflightPlan {
  const message = summarizePreflightMessages(preflight);

  switch (preflight.recommendedAction) {
    case "clone":
      return { kind: "clone", message };
    case "clone-with-attention":
      return { kind: "confirm-clone", message };
    case "open-existing": {
      if (options.forceClone) {
        return { kind: "clone", message };
      }
      const existing = preflight.duplicateRepos[0];
      if (!existing?.localPath) {
        return {
          kind: "block",
          message: "Existing clone was detected, but its local path is unavailable.",
        };
      }
      return {
        kind: "open-existing",
        localPath: existing.localPath,
        message,
      };
    }
    case "choose-different-folder":
    case "free-disk-space":
      return { kind: "block", message };
    default:
      return {
        kind: "block",
        message: "Clone preflight returned an unknown recommendation.",
      };
  }
}

export function summarizePreflightMessages(preflight: ClonePreflightResponse) {
  const messages = preflight.messages
    .map((message) => message.text.trim())
    .filter(Boolean);

  if (messages.length > 0) {
    return messages.join("\n");
  }
  return `Ready to clone into ${preflight.targetPath}.`;
}
