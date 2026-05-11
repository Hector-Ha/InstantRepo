import type { ClonePreflightResponse } from "./clonePreflight";
import type {
  AnalyzeSnapshot,
  EnvDraft,
  ExecuteResponse,
  InstalledRepoDetailsResponse,
  InstalledRepoManagerResponse,
} from "./types";

export const desktopSessionUnavailableMessage =
  "Desktop controls are unavailable in this browser session. Open the InstantRepo desktop window to use this action.";

export interface DesktopAppBridge {
  AnalyzeRepository(repoURL: string, localPath: string): Promise<AnalyzeSnapshot>;
  ClonePreflight(
    repoURL: string,
    destinationRoot: string,
  ): Promise<ClonePreflightResponse>;
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
  ListInstalledRepos(): Promise<InstalledRepoManagerResponse>;
  OpenDirectory(): Promise<string>;
  SaveEnvDraft(localPath: string, draft: EnvDraft): Promise<ExecuteResponse>;
  SaveEnvFile(localPath: string, content: string): Promise<ExecuteResponse>;
  ShellInfo(): Promise<unknown>;
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

export function createDesktopApi(source?: DesktopBridgeSource) {
  const call = <K extends keyof DesktopAppBridge>(methodName: K) =>
    getDesktopMethod(source ?? currentWindow(), methodName);

  return {
    AnalyzeRepository(repoURL: string, localPath: string) {
      return call("AnalyzeRepository")(repoURL, localPath);
    },
    ClonePreflight(repoURL: string, destinationRoot: string) {
      return call("ClonePreflight")(repoURL, destinationRoot);
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
    ListInstalledRepos() {
      return call("ListInstalledRepos")();
    },
    OpenDirectory() {
      return call("OpenDirectory")();
    },
    SaveEnvFile(localPath: string, content: string) {
      return call("SaveEnvFile")(localPath, content);
    },
    SaveEnvDraft(localPath: string, draft: EnvDraft) {
      return call("SaveEnvDraft")(localPath, draft);
    },
    ShellInfo() {
      return call("ShellInfo")();
    },
  };
}

export const desktopApi = createDesktopApi();

export const AnalyzeRepository = desktopApi.AnalyzeRepository;
export const ClonePreflight = desktopApi.ClonePreflight;
export const ExecuteStep = desktopApi.ExecuteStep;
export const ExportRepoDiagnostics = desktopApi.ExportRepoDiagnostics;
export const GenerateEnvDraft = desktopApi.GenerateEnvDraft;
export const ImportRepository = desktopApi.ImportRepository;
export const InstalledRepoDetails = desktopApi.InstalledRepoDetails;
export const ListInstalledRepos = desktopApi.ListInstalledRepos;
export const OpenDirectory = desktopApi.OpenDirectory;
export const SaveEnvFile = desktopApi.SaveEnvFile;
export const SaveEnvDraft = desktopApi.SaveEnvDraft;
export const ShellInfo = desktopApi.ShellInfo;
