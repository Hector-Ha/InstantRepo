import type { ClonePreflightResponse } from "./clonePreflight";
import type {
  AIEnvReviewSettings,
  AnalyzeSnapshot,
  EnvContributionSettings,
  EnvContributionSettingsResponse,
  EnvDraft,
  EnvVaultApprovalRequest,
  EnvVaultManagerResponse,
  EnvVaultRevealRequest,
  EnvVaultRevealResponse,
  EnvVaultSaveRequest,
  EnvVaultSaveResponse,
  EnvVaultUpdateRequest,
  ExecuteResponse,
  InstalledRepoDetailsResponse,
  InstalledRepoManagerResponse,
} from "./types";

export const desktopSessionUnavailableMessage =
  "Desktop controls are unavailable in this browser session. Open the InstantRepo desktop window to use this action.";
export const expectedBridgeContractVersion = "2026-05-bridge-1";
export const missingBridgeContractMessage =
  "Desktop controls are unavailable because bridge contract metadata is missing. Update InstantRepo and reload the desktop window.";
export function bridgeContractOutdatedMessage(actual: string) {
  return `Desktop controls are unavailable because bridge contract ${actual} is outdated. Expected ${expectedBridgeContractVersion}. Update InstantRepo and reload the desktop window.`;
}

export function canUseLocalEnvDraftFallback(error: unknown): boolean {
  const message =
    error instanceof Error ? error.message : typeof error === "string" ? error : "";
  return (
    message === desktopSessionUnavailableMessage ||
    message === missingBridgeContractMessage ||
    message.startsWith("Desktop controls are unavailable because bridge contract ") ||
    message ===
      'Desktop action "GenerateEnvDraft" is unavailable in this desktop session.'
  );
}

export interface DesktopAppBridge {
  AIEnvReviewSettings(): Promise<AIEnvReviewSettings>;
  AnalyzeRepository(repoURL: string, localPath: string): Promise<AnalyzeSnapshot>;
  ClonePreflight(
    repoURL: string,
    destinationRoot: string,
  ): Promise<ClonePreflightResponse>;
  ClearEnvContributionQueue(): Promise<EnvContributionSettingsResponse>;
  EnvContributionSettings(): Promise<EnvContributionSettingsResponse>;
  ExecuteStep(
    repoURL: string,
    localPath: string,
    stepID: string,
    approved: boolean,
  ): Promise<ExecuteResponse>;
  ExportRepoDiagnostics(localPath: string): Promise<unknown>;
  GenerateEnvDraft(localPath: string): Promise<EnvDraft>;
  ImportRepository(
    repoURL: string,
    destinationRoot: string,
  ): Promise<AnalyzeSnapshot>;
  InstalledRepoDetails(
    installedRepoID: number,
  ): Promise<InstalledRepoDetailsResponse>;
  ApproveEnvVaultEntry(approval: EnvVaultApprovalRequest): Promise<void>;
  ListEnvVaultEntries(): Promise<EnvVaultManagerResponse>;
  ListInstalledRepos(): Promise<InstalledRepoManagerResponse>;
  MarkEnvVaultEntryStatus(entryID: number, status: string): Promise<void>;
  OpenDirectory(): Promise<string>;
  RemoveEnvVaultEntry(entryID: number): Promise<void>;
  RevealEnvVaultEntry(
    req: EnvVaultRevealRequest,
  ): Promise<EnvVaultRevealResponse>;
  RecordEnvContributionConsent(
    publicEnabled: boolean,
  ): Promise<EnvContributionSettingsResponse>;
  RevokeEnvVaultApproval(approvalID: number): Promise<void>;
  SaveEnvDraft(localPath: string, draft: EnvDraft): Promise<ExecuteResponse>;
  SaveEnvContributionSettings(
    settings: EnvContributionSettings,
  ): Promise<EnvContributionSettingsResponse>;
  SaveAIEnvReviewSettings(
    settings: AIEnvReviewSettings,
  ): Promise<AIEnvReviewSettings>;
  SaveEnvFile(localPath: string, content: string): Promise<ExecuteResponse>;
  SaveEnvVaultCredential(
    req: EnvVaultSaveRequest,
  ): Promise<EnvVaultSaveResponse>;
  ShellInfo(): Promise<Record<string, string>>;
  SuppressEnvVaultPrompt(suppression: {
    repoPath: string;
    targetRelativePath: string;
    variableName: string;
  }): Promise<void>;
  UpdateEnvVaultEntry(
    req: EnvVaultUpdateRequest,
  ): Promise<EnvVaultSaveResponse>;
}

interface DesktopBridgeGo {
  main?: {
    App?: Partial<DesktopAppBridge>;
  };
}

export interface DesktopBridgeSource {
  go?: DesktopBridgeGo;
}

declare global {
  interface Window {
    go?: DesktopBridgeGo;
  }
}

function currentWindow(): DesktopBridgeSource | undefined {
  return typeof window === "undefined" ? undefined : window;
}

export function getDesktopApp(
  source: DesktopBridgeSource | undefined = currentWindow(),
) {
  const app = source?.go?.main?.App;
  if (!app) {
    throw new Error(desktopSessionUnavailableMessage);
  }
  return app;
}

function getDesktopMethod<K extends keyof DesktopAppBridge>(
  source: DesktopBridgeSource | undefined,
  methodName: K,
): DesktopAppBridge[K] {
  const app = getDesktopApp(source);
  const method = app[methodName];
  if (typeof method !== "function") {
    throw new Error(
      `Desktop action "${String(methodName)}" is unavailable in this desktop session.`,
    );
  }
  return method.bind(app) as DesktopAppBridge[K];
}

function bridgeContractVersionFrom(info: unknown) {
  if (!info || typeof info !== "object") {
    return "";
  }
  const version = (info as { bridgeContractVersion?: unknown })
    .bridgeContractVersion;
  return typeof version === "string" ? version.trim() : "";
}

async function verifyBridgeContract(
  source: DesktopBridgeSource | undefined,
) {
  const app = getDesktopApp(source);
  if (typeof app.ShellInfo !== "function") {
    throw new Error(missingBridgeContractMessage);
  }
  const info = await app.ShellInfo();
  const actual = bridgeContractVersionFrom(info);
  if (!actual) {
    throw new Error(missingBridgeContractMessage);
  }
  if (actual !== expectedBridgeContractVersion) {
    throw new Error(bridgeContractOutdatedMessage(actual));
  }
}

async function invokeDesktopMethod<K extends keyof DesktopAppBridge>(
  source: DesktopBridgeSource | undefined,
  methodName: K,
  args: Parameters<DesktopAppBridge[K]>,
  checkBridge: boolean,
): Promise<Awaited<ReturnType<DesktopAppBridge[K]>>> {
  if (checkBridge) {
    await verifyBridgeContract(source);
  }
  const method = getDesktopMethod(source, methodName) as (
    ...args: Parameters<DesktopAppBridge[K]>
  ) => ReturnType<DesktopAppBridge[K]>;
  return await method(...args);
}

export function createDesktopApi(source?: DesktopBridgeSource) {
  const call =
    <K extends keyof DesktopAppBridge>(
      methodName: K,
      checkBridge = true,
    ) =>
    (...args: Parameters<DesktopAppBridge[K]>) =>
      invokeDesktopMethod(
        source ?? currentWindow(),
        methodName,
        args,
        checkBridge,
      ) as ReturnType<DesktopAppBridge[K]>;

  return {
    AIEnvReviewSettings() {
      return call("AIEnvReviewSettings")();
    },
    AnalyzeRepository(repoURL: string, localPath: string) {
      return call("AnalyzeRepository")(repoURL, localPath);
    },
    ClonePreflight(repoURL: string, destinationRoot: string) {
      return call("ClonePreflight")(repoURL, destinationRoot);
    },
    ClearEnvContributionQueue() {
      return call("ClearEnvContributionQueue")();
    },
    EnvContributionSettings() {
      return call("EnvContributionSettings")();
    },
    ExecuteStep(
      repoURL: string,
      localPath: string,
      stepID: string,
      approved: boolean,
    ) {
      return call("ExecuteStep")(repoURL, localPath, stepID, approved);
    },
    ExportRepoDiagnostics(localPath: string) {
      return call("ExportRepoDiagnostics")(localPath);
    },
    GenerateEnvDraft(localPath: string) {
      return call("GenerateEnvDraft")(localPath);
    },
    ImportRepository(repoURL: string, destinationRoot: string) {
      return call("ImportRepository")(repoURL, destinationRoot);
    },
    InstalledRepoDetails(installedRepoID: number) {
      return call("InstalledRepoDetails")(installedRepoID);
    },
    ApproveEnvVaultEntry(approval: EnvVaultApprovalRequest) {
      return call("ApproveEnvVaultEntry")(approval);
    },
    ListEnvVaultEntries() {
      return call("ListEnvVaultEntries")();
    },
    ListInstalledRepos() {
      return call("ListInstalledRepos")();
    },
    MarkEnvVaultEntryStatus(entryID: number, status: string) {
      return call("MarkEnvVaultEntryStatus")(entryID, status);
    },
    OpenDirectory() {
      return call("OpenDirectory")();
    },
    RemoveEnvVaultEntry(entryID: number) {
      return call("RemoveEnvVaultEntry")(entryID);
    },
    RevealEnvVaultEntry(req: EnvVaultRevealRequest) {
      return call("RevealEnvVaultEntry")(req);
    },
    RecordEnvContributionConsent(publicEnabled: boolean) {
      return call("RecordEnvContributionConsent")(publicEnabled);
    },
    RevokeEnvVaultApproval(approvalID: number) {
      return call("RevokeEnvVaultApproval")(approvalID);
    },
    SaveEnvContributionSettings(settings: EnvContributionSettings) {
      return call("SaveEnvContributionSettings")(settings);
    },
    SaveAIEnvReviewSettings(settings: AIEnvReviewSettings) {
      return call("SaveAIEnvReviewSettings")(settings);
    },
    SaveEnvFile(localPath: string, content: string) {
      return call("SaveEnvFile")(localPath, content);
    },
    SaveEnvDraft(localPath: string, draft: EnvDraft) {
      return call("SaveEnvDraft")(localPath, draft);
    },
    SaveEnvVaultCredential(req: EnvVaultSaveRequest) {
      return call("SaveEnvVaultCredential")(req);
    },
    ShellInfo() {
      return call("ShellInfo", false)();
    },
    SuppressEnvVaultPrompt(suppression: {
      repoPath: string;
      targetRelativePath: string;
      variableName: string;
    }) {
      return call("SuppressEnvVaultPrompt")(suppression);
    },
    UpdateEnvVaultEntry(req: EnvVaultUpdateRequest) {
      return call("UpdateEnvVaultEntry")(req);
    },
  };
}

export const desktopApi = createDesktopApi();

export const AnalyzeRepository = desktopApi.AnalyzeRepository;
export const ClonePreflight = desktopApi.ClonePreflight;
export const ClearEnvContributionQueue = desktopApi.ClearEnvContributionQueue;
export const GetAIEnvReviewSettings = desktopApi.AIEnvReviewSettings;
export const GetEnvContributionSettings = desktopApi.EnvContributionSettings;
export const ExecuteStep = desktopApi.ExecuteStep;
export const ExportRepoDiagnostics = desktopApi.ExportRepoDiagnostics;
export const GenerateEnvDraft = desktopApi.GenerateEnvDraft;
export const ImportRepository = desktopApi.ImportRepository;
export const InstalledRepoDetails = desktopApi.InstalledRepoDetails;
export const ApproveEnvVaultEntry = desktopApi.ApproveEnvVaultEntry;
export const ListEnvVaultEntries = desktopApi.ListEnvVaultEntries;
export const ListInstalledRepos = desktopApi.ListInstalledRepos;
export const MarkEnvVaultEntryStatus = desktopApi.MarkEnvVaultEntryStatus;
export const OpenDirectory = desktopApi.OpenDirectory;
export const RemoveEnvVaultEntry = desktopApi.RemoveEnvVaultEntry;
export const RevealEnvVaultEntry = desktopApi.RevealEnvVaultEntry;
export const RecordEnvContributionConsent =
  desktopApi.RecordEnvContributionConsent;
export const RevokeEnvVaultApproval = desktopApi.RevokeEnvVaultApproval;
export const SaveEnvContributionSettings =
  desktopApi.SaveEnvContributionSettings;
export const SaveAIEnvReviewSettings = desktopApi.SaveAIEnvReviewSettings;
export const SaveEnvFile = desktopApi.SaveEnvFile;
export const SaveEnvDraft = desktopApi.SaveEnvDraft;
export const SaveEnvVaultCredential = desktopApi.SaveEnvVaultCredential;
export const ShellInfo = desktopApi.ShellInfo;
export const SuppressEnvVaultPrompt = desktopApi.SuppressEnvVaultPrompt;
export const UpdateEnvVaultEntry = desktopApi.UpdateEnvVaultEntry;
