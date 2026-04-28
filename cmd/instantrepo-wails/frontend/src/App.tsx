import { useMemo, useState } from 'react'
import { createEnvDraft, createInitialActivity, createSnapshot, defaultStepStates } from './mock'
import type { ActivityEntry, AnalyzeSnapshot, ExecutionStep, StepStatus } from './types'

const initialRepoUrl = 'https://github.com/example/instantrepo-demo'
const initialFolder = 'C:\\Users\\Admin\\Desktop\\Sandbox'
const initialSnapshot = createSnapshot(initialRepoUrl, initialFolder)

function pause(ms: number) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

function buildStepStateMap(snapshot: AnalyzeSnapshot): Record<string, StepStatus> {
  const next: Record<string, StepStatus> = { ...defaultStepStates }

  for (const step of snapshot.plan.steps) {
    const manual = step.type.includes('review') || step.command.toLowerCase().startsWith('manual ')
    next[step.id] = manual ? 'manual' : 'pending'
  }

  return next
}

function confidenceLabel(value: number) {
  return `${Math.round(value * 100)}%`
}

function toneClass(tone: ActivityEntry['tone']) {
  switch (tone) {
    case 'success':
      return 'success'
    case 'warning':
      return 'warning'
    case 'critical':
      return 'critical'
    default:
      return 'info'
  }
}

function statusClass(status: string) {
  switch (status.toLowerCase()) {
    case 'running':
    case 'busy':
      return 'warning'
    case 'succeeded':
    case 'resolved':
    case 'available':
    case 'ready':
      return 'success'
    case 'failed':
    case 'missing':
    case 'user_required':
      return 'critical'
    default:
      return 'muted'
  }
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  )
}

function StepButton({
  step,
  status,
  active,
  onSelect,
}: {
  step: ExecutionStep
  status: StepStatus
  active: boolean
  onSelect: () => void
}) {
  return (
    <button type="button" className={`step-button ${active ? 'active' : ''}`} onClick={onSelect}>
      <div className="step-button__top">
        <strong>{step.title}</strong>
        <span className={`status-badge ${statusClass(status)}`}>{status}</span>
      </div>
      <p>{step.reason}</p>
    </button>
  )
}

export default function App() {
  const [repoUrl, setRepoUrl] = useState(initialRepoUrl)
  const [folderPath, setFolderPath] = useState(initialFolder)
  const [snapshot, setSnapshot] = useState(initialSnapshot)
  const [envText, setEnvText] = useState(createEnvDraft(initialSnapshot))
  const [stepStates, setStepStates] = useState<Record<string, StepStatus>>(buildStepStateMap(initialSnapshot))
  const [selectedStepId, setSelectedStepId] = useState(initialSnapshot.plan.steps[0]?.id ?? '')
  const [activity, setActivity] = useState<ActivityEntry[]>(createInitialActivity(initialSnapshot))
  const [busyLabel, setBusyLabel] = useState<string | null>(null)

  const selectedStep = useMemo(
    () => snapshot.plan.steps.find((step) => step.id === selectedStepId) ?? snapshot.plan.steps[0],
    [selectedStepId, snapshot.plan.steps],
  )

  const unresolvedEnv = useMemo(
    () => snapshot.plan.env.variables.filter((item) => item.currentStatus === 'user_required'),
    [snapshot.plan.env.variables],
  )

  const missingTools = useMemo(
    () => snapshot.environment.tools.filter((tool) => !tool.available),
    [snapshot.environment.tools],
  )

  const appendActivity = (tone: ActivityEntry['tone'], label: string, message: string) => {
    const next: ActivityEntry = {
      id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
      time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      tone,
      label,
      message,
    }

    setActivity((current) => [next, ...current].slice(0, 5))
  }

  const resetWorkspace = (nextSnapshot: AnalyzeSnapshot, modeLabel: string) => {
    setSnapshot(nextSnapshot)
    setEnvText(createEnvDraft(nextSnapshot))
    setStepStates(buildStepStateMap(nextSnapshot))
    setSelectedStepId(nextSnapshot.plan.steps[0]?.id ?? '')
    appendActivity(
      'success',
      modeLabel,
      `${nextSnapshot.analysis.projectName} loaded with ${nextSnapshot.plan.steps.length} setup steps.`,
    )
  }

  const handleAnalyze = async (mode: 'clone' | 'local' | 'refresh') => {
    const labels = {
      clone: 'Cloning and analyzing repository...',
      local: 'Analyzing existing folder...',
      refresh: 'Refreshing analysis...',
    }

    setBusyLabel(labels[mode])
    await pause(mode === 'refresh' ? 450 : 800)

    const nextSnapshot = createSnapshot(repoUrl, folderPath)
    const actionLabel = mode === 'clone' ? 'Clone and Analyze' : mode === 'local' ? 'Analyze Folder' : 'Refresh'
    resetWorkspace(nextSnapshot, actionLabel)
    setBusyLabel(null)
  }

  const handleGenerateEnv = async () => {
    setBusyLabel('Generating .env draft...')
    await pause(400)
    setEnvText(createEnvDraft(snapshot))
    setBusyLabel(null)
    appendActivity('success', 'Draft Generated', `Updated ${snapshot.plan.env.targetPath ?? '.env'}.`)
  }

  const handleSaveEnv = async () => {
    setBusyLabel('Saving .env file...')
    await pause(350)
    setBusyLabel(null)
    appendActivity('success', 'Env Saved', `Saved ${snapshot.plan.env.targetPath ?? '.env'}.`)
  }

  const handleRunStep = async () => {
    if (!selectedStep) {
      return
    }

    setBusyLabel(`Running ${selectedStep.title}...`)
    setStepStates((current) => ({ ...current, [selectedStep.id]: 'running' }))
    appendActivity('info', 'Step Started', selectedStep.command)
    await pause(selectedStep.id === 'compose-services' ? 900 : 650)
    setStepStates((current) => ({ ...current, [selectedStep.id]: 'succeeded' }))
    setBusyLabel(null)
    appendActivity('success', 'Step Finished', `${selectedStep.title} completed successfully.`)
  }

  const handleSuggestFolder = () => {
    const repoName = repoUrl.trim().split('/').pop()?.replace(/\.git$/i, '') || 'instantrepo-demo'
    setFolderPath(`C:\\Users\\Admin\\Desktop\\Workspaces\\${repoName}`)
  }

  return (
    <div className="app">
      <header className="page-header">
        <div>
          <h1>InstantRepo</h1>
          <p>Analyze a repository, prepare the environment file, then run the next setup step.</p>
        </div>
        <div className={`status-banner ${statusClass(busyLabel ? 'busy' : 'ready')}`}>
          <strong>{busyLabel ?? 'Ready'}</strong>
          <span>{busyLabel ? 'Please wait for the current action to finish.' : 'Choose an action below to continue.'}</span>
        </div>
      </header>

      <main className="page-grid">
        <section className="card">
          <div className="section-heading">
            <div>
              <h2>1. Choose Repository</h2>
              <p>Paste a remote URL or work from an existing local folder.</p>
            </div>
          </div>

          <div className="field-group">
            <label className="field">
              <span>Repository URL</span>
              <input value={repoUrl} onChange={(event) => setRepoUrl(event.target.value)} placeholder="https://github.com/owner/repo" />
            </label>

            <label className="field">
              <span>Destination Folder</span>
              <input value={folderPath} onChange={(event) => setFolderPath(event.target.value)} placeholder="C:\\Projects\\repo" />
            </label>
          </div>

          <div className="button-row">
            <button type="button" className="button button-primary" onClick={() => void handleAnalyze('clone')} disabled={busyLabel !== null}>
              Clone and Analyze
            </button>
            <button type="button" className="button" onClick={() => void handleAnalyze('local')} disabled={busyLabel !== null}>
              Analyze Existing Folder
            </button>
            <button type="button" className="button" onClick={() => void handleAnalyze('refresh')} disabled={busyLabel !== null}>
              Refresh Analysis
            </button>
            <button type="button" className="button button-subtle" onClick={handleSuggestFolder} disabled={busyLabel !== null}>
              Suggest Folder
            </button>
          </div>
        </section>

        <section className="card">
          <div className="section-heading">
            <div>
              <h2>2. Current Summary</h2>
              <p>Review what InstantRepo found before editing files or running commands.</p>
            </div>
          </div>

          <div className="summary-grid">
            <div className="summary-panel">
              <SummaryRow label="Project" value={snapshot.analysis.projectName} />
              <SummaryRow label="Type" value={snapshot.analysis.projectType} />
              <SummaryRow label="Confidence" value={confidenceLabel(snapshot.analysis.confidence)} />
              <SummaryRow label="Repository Path" value={snapshot.source.path} />
              <SummaryRow label="Setup Steps" value={String(snapshot.plan.steps.length)} />
            </div>

            <div className="summary-panel">
              <h3>Attention Needed</h3>
              <ul className="plain-list">
                {missingTools.map((tool) => (
                  <li key={tool.name}>
                    Missing tool: <strong>{tool.name}</strong>
                  </li>
                ))}
                {unresolvedEnv.map((item) => (
                  <li key={item.name}>
                    Required secret: <strong>{item.name}</strong>
                  </li>
                ))}
                {missingTools.length === 0 && unresolvedEnv.length === 0 ? <li>No immediate blockers detected.</li> : null}
              </ul>
            </div>
          </div>
        </section>

        <section className="card">
          <div className="section-heading">
            <div>
              <h2>3. Environment File</h2>
              <p>Generate the draft first, then edit values directly and save the file.</p>
            </div>
            <div className="button-row">
              <button type="button" className="button" onClick={() => void handleGenerateEnv()} disabled={busyLabel !== null}>
                Generate Draft
              </button>
              <button type="button" className="button button-primary" onClick={() => void handleSaveEnv()} disabled={busyLabel !== null}>
                Save .env
              </button>
            </div>
          </div>

          <div className="env-info">
            {snapshot.plan.env.variables.map((item) => (
              <div className="env-row" key={item.name}>
                <div>
                  <strong>{item.name}</strong>
                  <p>{item.instructions[0] ?? 'No additional instructions.'}</p>
                </div>
                <span className={`status-badge ${statusClass(item.currentStatus)}`}>{item.currentStatus}</span>
              </div>
            ))}
          </div>

          <textarea value={envText} onChange={(event) => setEnvText(event.target.value)} spellCheck={false} />
        </section>

        <section className="card">
          <div className="section-heading">
            <div>
              <h2>4. Setup Steps</h2>
              <p>Select one step, read what it does, then run it.</p>
            </div>
          </div>

          <div className="steps-layout">
            <div className="steps-list">
              {snapshot.plan.steps.map((step) => (
                <StepButton
                  key={step.id}
                  step={step}
                  status={stepStates[step.id] ?? 'pending'}
                  active={step.id === selectedStep?.id}
                  onSelect={() => setSelectedStepId(step.id)}
                />
              ))}
            </div>

            {selectedStep ? (
              <div className="step-details">
                <div className="step-details__header">
                  <div>
                    <h3>{selectedStep.title}</h3>
                    <p>{selectedStep.reason}</p>
                  </div>
                  <button type="button" className="button button-primary" onClick={() => void handleRunStep()} disabled={busyLabel !== null}>
                    Run Selected Step
                  </button>
                </div>

                <div className="summary-panel">
                  <SummaryRow label="Status" value={stepStates[selectedStep.id] ?? 'pending'} />
                  <SummaryRow label="Importance" value={selectedStep.importance} />
                  <SummaryRow label="Risk" value={selectedStep.risk} />
                  <SummaryRow label="Approval" value={selectedStep.requiresApproval ? 'required' : 'not required'} />
                </div>

                <div className="command-box">
                  <span>Command</span>
                  <code>{selectedStep.command}</code>
                </div>

                <div>
                  <h4>Confirmed By</h4>
                  <ul className="plain-list">
                    {selectedStep.confirmedBy.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </div>
              </div>
            ) : null}
          </div>
        </section>

        <section className="card">
          <div className="section-heading">
            <div>
              <h2>Recent Events</h2>
              <p>Simple feedback for the last actions taken in this session.</p>
            </div>
          </div>

          <div className="activity-list">
            {activity.map((entry) => (
              <div className={`activity-item ${toneClass(entry.tone)}`} key={entry.id}>
                <div className="activity-item__header">
                  <strong>{entry.label}</strong>
                  <span>{entry.time}</span>
                </div>
                <p>{entry.message}</p>
              </div>
            ))}
          </div>
        </section>
      </main>
    </div>
  )
}
