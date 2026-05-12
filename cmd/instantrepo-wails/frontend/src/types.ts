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

export type EnvVaultStatus =
  | "ready"
  | "needs_review"
  | "action_needed"
  | "invalid";

export interface EnvVaultEntry {
  id: number;
  provider: string;
  variableName: string;
  displayName: string;
  fingerprintFragment: string;
  status: EnvVaultStatus;
  createdAt: string;
  updatedAt: string;
}

export interface EnvVaultApproval {
  id: number;
  entryId: number;
  repoPath: string;
  targetRelativePath: string;
  variableName: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

export interface EnvVaultApprovalRequest {
  entryId: number;
  repoPath: string;
  targetRelativePath: string;
  variableName: string;
}

export interface EnvVaultUsageLocation {
  repoPath: string;
  targetRelativePath: string;
  variableName: string;
  lastUsedAt: string;
  useCount: number;
}

export interface EnvVaultUsageSummary {
  totalUseCount: number;
  locations: EnvVaultUsageLocation[];
}

export interface EnvVaultManagerEntry extends EnvVaultEntry {
  usage: EnvVaultUsageSummary;
  approvals: EnvVaultApproval[];
}

export interface EnvVaultManagerResponse {
  entries: EnvVaultManagerEntry[];
}

export interface EnvVaultSaveRequest {
  provider: string;
  variableName: string;
  displayName?: string;
  value: string;
}

export interface EnvVaultSaveResponse {
  entry?: EnvVaultEntry;
  needsReview: boolean;
  reviewMessage?: string;
}

export interface EnvVaultUpdateRequest {
  entryId: number;
  displayName?: string;
  updateValue?: boolean;
  value?: string;
}

export interface EnvVaultRevealRequest {
  entryId: number;
  confirmed: boolean;
}

export interface EnvVaultRevealResponse {
  entryId: number;
  value: string;
  revealUntil: string;
}

export interface EnvContributionSettings {
  publicEnvPatternsEnabled: boolean;
  privateLocalEnvPatternsEnabled: boolean;
  consentShown: boolean;
  updatedAt: string;
}

export interface EnvContributionQueueStatus {
  count: number;
  oldestCreatedAt?: string;
}

export interface EnvContributionSettingsResponse {
  settings: EnvContributionSettings;
  queue: EnvContributionQueueStatus;
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
