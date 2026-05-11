export type StepStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "manual";

export interface RepoSource {
  type: string;
  repoUrl?: string;
  path: string;
}

export interface DetectedTool {
  name: string;
  version?: string;
  available: boolean;
  path?: string;
}

export interface EnvironmentReport {
  os: string;
  arch: string;
  tools: DetectedTool[];
}

export interface RequirementGap {
  tool: string;
  requiredVersion: string;
  installedVersion?: string;
  status: string;
}

export interface ToolRequirement {
  tool: string;
  versionConstraint: string;
  source: string;
  required: boolean;
}

export interface EnvVarRequirement {
  name: string;
  source: string;
  required: boolean;
  secret: boolean;
  confidence?: number;
  currentStatus: string;
  fillStrategy: string;
  service?: string;
  suggestedValue?: string;
  instructions?: string[];
}

export interface EnvironmentConfig {
  templatePath?: string;
  targetPath?: string;
  targetExists: boolean;
  variables: EnvVarRequirement[];
}

export interface EnvValueProvenance {
  source: string;
}

export interface EnvVaultBinding {
  entryId?: number;
  provider?: string;
  variableName?: string;
  fingerprint: string;
  displayName?: string;
  label?: string;
  status?: string;
}

export interface EnvDraftValue {
  name: string;
  value: string;
  secret: boolean;
  confidence: number;
  valueClass?: string;
  instructions?: string[];
  attention?: string[];
  provenance: EnvValueProvenance;
  vaultBinding?: EnvVaultBinding;
  hasExistingValue?: boolean;
}

export interface EnvVaultPromptCandidate {
  repoPath: string;
  targetRelativePath: string;
  variableName: string;
  provider: string;
  fingerprintFragment: string;
}

export interface EnvVaultPromptSuppression {
  repoPath: string;
  targetRelativePath: string;
  variableName: string;
}

export interface EnvDraftTarget {
  relativePath: string;
  absolutePath: string;
  originalContent: string;
  values: EnvDraftValue[];
}

export interface EnvDraft {
  repoPath: string;
  targets: EnvDraftTarget[];
}

export interface ServiceDependency {
  name: string;
  scope: string;
  provisioning: string;
  source: string;
  status: string;
  details?: string;
  instructions?: string[];
}

export interface SafetyFinding {
  severity: string;
  summary: string;
  filePath?: string;
}

export interface SafetyReport {
  riskLevel: string;
  findings: SafetyFinding[];
}

export interface ExecutionStep {
  id: string;
  title: string;
  command: string;
  cwd: string;
  type: string;
  importance: string;
  risk: string;
  requiresApproval: boolean;
  evidenceSource?: string;
  confirmedBy?: string[];
  confidence: number;
  reason: string;
}

export interface SetupPlan {
  projectName: string;
  projectType: string;
  confidence: number;
  evidence: string[];
  gaps: RequirementGap[];
  env: EnvironmentConfig;
  services: ServiceDependency[];
  steps: ExecutionStep[];
  safety: SafetyReport;
  unknowns: string[];
}

export interface RepositoryAnalysis {
  projectName: string;
  projectType: string;
  repoPath: string;
  confidence: number;
  evidence: string[];
  requirements?: ToolRequirement[];
  env?: EnvironmentConfig;
  services?: ServiceDependency[];
  steps?: ExecutionStep[];
  unknowns?: string[];
}

export interface AnalyzeSnapshot {
  source: RepoSource;
  analysis: RepositoryAnalysis;
  environment: EnvironmentReport;
  plan: SetupPlan;
}

export interface ExecutionResult {
  stepId: string;
  command: string;
  cwd: string;
  processId: number;
  exitCode: number;
  stdout: string;
  stderr: string;
  duration: string;
  succeeded: boolean;
}

export interface ExecuteResponse extends AnalyzeSnapshot {
  result: ExecutionResult;
  vaultPromptCandidates?: EnvVaultPromptCandidate[];
}

export interface InstalledRepoSummary {
  id: number;
  projectName: string;
  localPath: string;
  status: string;
  lastAnalyzedAt: string;
  lastSetupAt: string;
  lastActivityAt: string;
}

export interface SetupSessionSummary {
  id: number;
  installedRepoId: number;
  repoPath: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface InstalledRepoManagerResponse {
  repos: InstalledRepoSummary[];
}

export interface InstalledRepoDetailsResponse {
  repo: InstalledRepoSummary;
  setupSessions: SetupSessionSummary[];
}

export interface ActivityEntry {
  id: string;
  time: string;
  tone: "info" | "success" | "warning" | "critical";
  label: string;
  message: string;
}
