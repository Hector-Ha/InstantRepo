export type StepStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'manual'

export interface RepoSource {
  type: string
  repoUrl?: string
  path: string
}

export interface DetectedTool {
  name: string
  version?: string
  available: boolean
  path?: string
}

export interface EnvironmentReport {
  os: string
  arch: string
  tools: DetectedTool[]
}

export interface RequirementGap {
  tool: string
  requiredVersion: string
  installedVersion?: string
  status: string
}

export interface EnvVarRequirement {
  name: string
  source: string
  required: boolean
  secret: boolean
  currentStatus: string
  fillStrategy: string
  service?: string
  suggestedValue?: string
  instructions: string[]
}

export interface EnvironmentConfig {
  templatePath?: string
  targetPath?: string
  targetExists: boolean
  variables: EnvVarRequirement[]
}

export interface ServiceDependency {
  name: string
  scope: string
  provisioning: string
  source: string
  status: string
  details?: string
  instructions: string[]
}

export interface SafetyFinding {
  severity: string
  summary: string
  filePath?: string
}

export interface SafetyReport {
  riskLevel: string
  findings: SafetyFinding[]
}

export interface ExecutionStep {
  id: string
  title: string
  command: string
  cwd: string
  type: string
  importance: string
  risk: string
  requiresApproval: boolean
  evidenceSource?: string
  confirmedBy: string[]
  confidence: number
  reason: string
}

export interface SetupPlan {
  projectName: string
  projectType: string
  confidence: number
  evidence: string[]
  gaps: RequirementGap[]
  env: EnvironmentConfig
  services: ServiceDependency[]
  steps: ExecutionStep[]
  safety: SafetyReport
  unknowns: string[]
}

export interface RepositoryAnalysis {
  projectName: string
  projectType: string
  repoPath: string
  confidence: number
  evidence: string[]
}

export interface AnalyzeSnapshot {
  source: RepoSource
  analysis: RepositoryAnalysis
  environment: EnvironmentReport
  plan: SetupPlan
}

export interface ActivityEntry {
  id: string
  time: string
  tone: 'info' | 'success' | 'warning' | 'critical'
  label: string
  message: string
}
