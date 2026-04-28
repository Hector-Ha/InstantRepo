import type { ActivityEntry, AnalyzeSnapshot, StepStatus } from './types'

const defaultRepoURL = 'https://github.com/example/instantrepo-demo'
const defaultPath = 'C:\\Repos\\instantrepo-demo'

function nowLabel() {
  return new Date().toLocaleTimeString([], {
    hour: '2-digit',
    minute: '2-digit',
  })
}

function repoNameFromInput(repoUrl: string, folderPath: string) {
  const cleanRepo = repoUrl.trim().replace(/\/+$/, '')
  if (cleanRepo) {
    const lastSegment = cleanRepo.split('/').pop()
    if (lastSegment) {
      return lastSegment.replace(/\.git$/i, '')
    }
  }

  const cleanPath = folderPath.trim().replace(/[\\/]+$/, '')
  if (cleanPath) {
    const lastSegment = cleanPath.split(/[\\/]/).pop()
    if (lastSegment) {
      return lastSegment
    }
  }

  return 'instantrepo-demo'
}

function repoPathFromInput(repoName: string, folderPath: string) {
  const base = folderPath.trim() || 'C:\\Repos'
  return `${base.replace(/[\\/]+$/, '')}\\${repoName}`
}

export function createSnapshot(repoUrl: string, folderPath: string): AnalyzeSnapshot {
  const projectName = repoNameFromInput(repoUrl, folderPath)
  const sourcePath = repoPathFromInput(projectName, folderPath || defaultPath)
  const sourceUrl = repoUrl.trim() || defaultRepoURL

  return {
    source: {
      type: sourceUrl.includes('gitlab') ? 'gitlab' : 'github',
      repoUrl: sourceUrl,
      path: sourcePath,
    },
    analysis: {
      projectName,
      projectType: 'monorepo web application',
      repoPath: sourcePath,
      confidence: 0.92,
      evidence: [
        'package.json, pnpm-lock.yaml, and docker-compose.yml detected',
        '.env.example contains both local and external service variables',
        'README includes bootstrap, dev, and background service instructions',
        'frontend and worker packages share a common TypeScript workspace',
      ],
    },
    environment: {
      os: 'windows',
      arch: 'amd64',
      tools: [
        { name: 'git', available: true, version: '2.49.0' },
        { name: 'bun', available: true, version: '1.3.3' },
        { name: 'docker', available: true, version: '27.5.1' },
        { name: 'psql', available: false, version: '' },
      ],
    },
    plan: {
      projectName,
      projectType: 'monorepo web application',
      confidence: 0.92,
      evidence: [
        'workspace root exposes app, api, and worker packages',
        'docker-compose maps postgres and redis containers',
        'README references Bun, Postgres, Redis, and provider secrets',
      ],
      gaps: [
        {
          tool: 'postgres',
          requiredVersion: '14+',
          installedVersion: '',
          status: 'missing local database client',
        },
        {
          tool: 'redis',
          requiredVersion: '7+',
          installedVersion: '',
          status: 'available through Docker Compose only',
        },
      ],
      env: {
        templatePath: `${sourcePath}\\.env.example`,
        targetPath: `${sourcePath}\\.env`,
        targetExists: true,
        variables: [
          {
            name: 'NODE_ENV',
            source: '.env.example',
            required: true,
            secret: false,
            currentStatus: 'resolved',
            fillStrategy: 'auto_fillable',
            suggestedValue: 'development',
            instructions: ['Keep in development while running local workflows.'],
          },
          {
            name: 'DATABASE_URL',
            source: '.env.example',
            required: true,
            secret: true,
            currentStatus: 'drafted',
            fillStrategy: 'auto_fillable',
            service: 'postgres',
            suggestedValue: 'postgresql://postgres:postgres@localhost:5432/app_dev',
            instructions: ['Local compose stack expects postgres on port 5432.'],
          },
          {
            name: 'REDIS_URL',
            source: '.env.example',
            required: true,
            secret: false,
            currentStatus: 'drafted',
            fillStrategy: 'auto_fillable',
            service: 'redis',
            suggestedValue: 'redis://localhost:6379',
            instructions: ['Use the local compose service when running workers.'],
          },
          {
            name: 'OPENAI_API_KEY',
            source: '.env.example',
            required: true,
            secret: true,
            currentStatus: 'user_required',
            fillStrategy: 'user_required',
            service: 'openai',
            instructions: [
              'Paste a live provider key before running AI-backed features.',
              'Do not commit this value.',
            ],
          },
        ],
      },
      services: [
        {
          name: 'postgres',
          scope: 'local',
          provisioning: 'docker-compose',
          source: 'docker-compose.yml',
          status: 'available',
          details: 'Primary relational store for app and worker packages.',
          instructions: ['Start with docker compose up -d postgres'],
        },
        {
          name: 'redis',
          scope: 'local',
          provisioning: 'docker-compose',
          source: 'docker-compose.yml',
          status: 'available',
          details: 'Backs cache, queues, and rate limit state.',
          instructions: ['Start with docker compose up -d redis'],
        },
        {
          name: 'openai',
          scope: 'external',
          provisioning: 'manual',
          source: '.env.example',
          status: 'needs_credentials',
          details: 'Remote LLM provider required for AI actions.',
          instructions: ['Paste API key into OPENAI_API_KEY.'],
        },
      ],
      steps: [
        {
          id: 'install-deps',
          title: 'Install workspace dependencies',
          command: 'bun install',
          cwd: sourcePath,
          type: 'install',
          importance: 'required',
          risk: 'low',
          requiresApproval: false,
          evidenceSource: 'package.json',
          confirmedBy: ['package.json', 'bun.lockb'],
          confidence: 0.97,
          reason: 'Workspace dependencies must exist before any package can build or run.',
        },
        {
          id: 'compose-services',
          title: 'Start local services',
          command: 'docker compose up -d postgres redis',
          cwd: sourcePath,
          type: 'service-start',
          importance: 'required',
          risk: 'medium',
          requiresApproval: true,
          evidenceSource: 'docker-compose.yml',
          confirmedBy: ['docker-compose.yml', 'README.md'],
          confidence: 0.88,
          reason: 'The application and workers expect Postgres and Redis before boot.',
        },
        {
          id: 'prepare-env',
          title: 'Prepare .env values',
          command: 'instantrepo internal:prepare-env',
          cwd: sourcePath,
          type: 'env-setup',
          importance: 'required',
          risk: 'low',
          requiresApproval: false,
          evidenceSource: '.env.example',
          confirmedBy: ['.env.example'],
          confidence: 0.94,
          reason: 'The stack includes both local defaults and unresolved provider secrets.',
        },
        {
          id: 'start-dev',
          title: 'Boot the development stack',
          command: 'bun run dev',
          cwd: sourcePath,
          type: 'run',
          importance: 'recommended',
          risk: 'low',
          requiresApproval: false,
          evidenceSource: 'README.md',
          confirmedBy: ['README.md', 'package.json scripts'],
          confidence: 0.84,
          reason: 'Start the app once services and env values are ready.',
        },
      ],
      safety: {
        riskLevel: 'medium',
        findings: [
          {
            severity: 'low',
            summary: 'Lifecycle scripts present in workspace dependencies.',
            filePath: 'package.json',
          },
          {
            severity: 'medium',
            summary: 'Docker compose starts network-accessible services.',
            filePath: 'docker-compose.yml',
          },
        ],
      },
      unknowns: [
        'Production-only analytics credentials are not available in the template.',
        'Worker concurrency settings are documented but not enforced in manifests.',
      ],
    },
  }
}

export const defaultStepStates = {
  'install-deps': 'pending',
  'compose-services': 'pending',
  'prepare-env': 'pending',
  'start-dev': 'pending',
} satisfies Record<string, StepStatus>

export function createEnvDraft(snapshot: AnalyzeSnapshot) {
  const lines = [
    '# Generated by InstantRepo',
    `# Updated ${new Date().toISOString()}`,
    '',
    'NODE_ENV=development',
    'DATABASE_URL=postgresql://postgres:postgres@localhost:5432/app_dev',
    'REDIS_URL=redis://localhost:6379',
    '',
    '# Action required for OPENAI_API_KEY.',
    '# Paste a live provider key before running AI-backed features.',
    'OPENAI_API_KEY=',
  ]

  if (snapshot.plan.env.targetPath) {
    lines.unshift(`# Target ${snapshot.plan.env.targetPath}`)
  }

  return lines.join('\n')
}

export function createInitialActivity(snapshot: AnalyzeSnapshot): ActivityEntry[] {
  return [
    {
      id: 'a-1',
      time: nowLabel(),
      tone: 'info',
      label: 'Shell Ready',
      message: `New Wails-ready control room mounted for ${snapshot.analysis.projectName}.`,
    },
    {
      id: 'a-2',
      time: nowLabel(),
      tone: 'warning',
      label: 'Endpoint Deferred',
      message: 'The interface is using a local mock adapter until the Go endpoints are wired in.',
    },
  ]
}
