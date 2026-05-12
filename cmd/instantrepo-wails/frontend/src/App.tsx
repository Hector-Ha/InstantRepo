import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AnalyzeRepository,
  ApproveEnvVaultEntry,
  ExecuteStep,
  GenerateEnvDraft,
  ImportRepository,
  InstalledRepoDetails,
  ListEnvVaultEntries,
  ListInstalledRepos,
  MarkEnvVaultEntryStatus,
  OpenDirectory,
  RemoveEnvVaultEntry,
  RevealEnvVaultEntry,
  RevokeEnvVaultApproval,
  SaveEnvDraft,
  SaveEnvVaultCredential,
  SuppressEnvVaultPrompt,
  UpdateEnvVaultEntry,
} from "./desktopApi";
import type {
  ActivityEntry,
  AnalyzeSnapshot,
  EnvDraft,
  EnvVaultApprovalRequest,
  EnvVaultManagerEntry,
  EnvVaultPromptCandidate,
  EnvVaultStatus,
  ExecuteResponse,
  ExecutionStep,
  InstalledRepoDetailsResponse,
  InstalledRepoManagerResponse,
  InstalledRepoSummary,
  StepStatus,
} from "./types";
import { AppNav, type AppView } from "./AppNav";
import { EnvDraftPanel } from "./EnvDraftPanel";
import { EnvVaultManager } from "./EnvVaultManager";
import { EnvVaultPrompt } from "./EnvVaultPrompt";
import { redactLikelySecrets } from "./redaction";
import { getMissingRequiredTools, getSafetyAttention } from "./attention";
import {
  clonePreflight,
  planClonePreflightFlow,
  summarizePreflightMessages,
  type ClonePreflightPlan,
  type ClonePreflightResponse,
} from "./clonePreflight";

const initialRepoUrl = "https://github.com/example/instantrepo-demo";
const initialFolder = "C:\\Users\\Admin\\Desktop\\Workspaces";

function buildStepStateMap(
  snapshot: AnalyzeSnapshot,
  previous: Record<string, StepStatus> = {},
): Record<string, StepStatus> {
  const next: Record<string, StepStatus> = {};

  for (const step of snapshot.plan.steps) {
    const manual = !isExecutableStep(step);
    const previousStatus = previous[step.id];
    next[step.id] = manual
      ? "manual"
      : previousStatus && previousStatus !== "manual"
        ? previousStatus
        : "pending";
  }

  return next;
}

function buildEnvDraft(snapshot: AnalyzeSnapshot): EnvDraft {
  const targetPath = snapshot.plan.env.targetPath ?? `${snapshot.source.path}\\.env`;
  const relativePath = targetPath.split(/[\\/]/).pop() ?? ".env";
  return {
    repoPath: snapshot.source.path,
    targets: [
      {
        relativePath,
        absolutePath: targetPath,
        originalContent: "",
        values: snapshot.plan.env.variables.map((item) => ({
          name: item.name,
          value: item.suggestedValue ?? "",
          secret: item.secret,
          confidence: item.confidence ?? 0.5,
          instructions: item.instructions,
          provenance: { source: "draft" },
        })),
      },
    ],
  };
}

function vaultPromptValue(
  draft: EnvDraft | null,
  candidate: EnvVaultPromptCandidate,
) {
  const target = draft?.targets.find(
    (item) => item.relativePath === candidate.targetRelativePath,
  );
  return (
    target?.values.find((item) => item.name === candidate.variableName)?.value ??
    ""
  );
}

function confidenceLabel(value: number) {
  return `${Math.round(value * 100)}%`;
}

function toneClass(tone: ActivityEntry["tone"]) {
  switch (tone) {
    case "success":
      return "success";
    case "warning":
      return "warning";
    case "critical":
      return "critical";
    default:
      return "info";
  }
}

function statusClass(status: string) {
  switch (status.toLowerCase()) {
    case "running":
    case "busy":
    case "drafted":
    case "version_mismatch":
      return "warning";
    case "succeeded":
    case "resolved":
    case "available":
    case "ready":
    case "satisfied":
      return "success";
    case "failed":
    case "missing":
    case "user_required":
    case "needs_credentials":
      return "critical";
    default:
      return "muted";
  }
}

function statusIcon(status: string) {
  switch (status.toLowerCase()) {
    case "succeeded":
    case "resolved":
    case "available":
    case "ready":
    case "satisfied":
      return "✓";
    case "failed":
    case "missing":
      return "✗";
    case "running":
    case "busy":
      return null; // Will use pulsing dot
    case "manual":
      return "◎";
    default:
      return "●";
  }
}

function toErrorMessage(error: unknown) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  if (typeof error === "string" && error.trim() !== "") {
    return error;
  }
  if (error && typeof error === "object" && "message" in error) {
    const message = error.message;
    if (typeof message === "string" && message.trim() !== "") {
      return message;
    }
  }
  return "Unexpected error";
}

function isExecutableStep(step: ExecutionStep) {
  const command = step.command.trim().toLowerCase();
  return (
    command !== "" &&
    !command.startsWith("manual ") &&
    !step.type.includes("review")
  );
}

function buildCloneConfirmationMessage(preflight: ClonePreflightResponse) {
  return `Clone with attention?\n\n${summarizePreflightMessages(preflight)}\n\nContinue cloning into:\n${preflight.targetPath}`;
}

function buildDuplicateConfirmationMessage(
  preflight: ClonePreflightResponse,
  plan: Extract<ClonePreflightPlan, { kind: "open-existing" }>,
) {
  return `Existing clone found:\n${plan.localPath}\n\nOpen existing clone? Choose Cancel to clone another copy into:\n${preflight.targetPath}`;
}

function formatManagerTime(value: string, emptyLabel: string) {
  if (!value || value.startsWith("0001-")) {
    return emptyLabel;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return emptyLabel;
  }
  return date.toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function SummaryRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="summary-row">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const icon = statusIcon(status);
  const isRunning =
    status.toLowerCase() === "running" || status.toLowerCase() === "busy";

  return (
    <span className={`status-badge ${statusClass(status)}`}>
      {isRunning ? (
        <span className="status-dot" />
      ) : icon ? (
        <span>{icon}</span>
      ) : null}
      {status}
    </span>
  );
}

function InstalledRepoManager({
  repos,
  details,
  selectedRepoId,
  loaded,
  loading,
  onRefresh,
  onShowDetails,
  onAnalyze,
}: {
  repos: InstalledRepoSummary[];
  details: InstalledRepoDetailsResponse | null;
  selectedRepoId: number | null;
  loaded: boolean;
  loading: boolean;
  onRefresh: () => void;
  onShowDetails: (repo: InstalledRepoSummary) => void;
  onAnalyze: (repo: InstalledRepoSummary) => void;
}) {
  return (
    <section className="card" aria-labelledby="section-manager">
      <div className="section-heading">
        <div>
          <h2 id="section-manager">
            <span className="section-number">1</span>Installed Repos
          </h2>
          <p>Local App Database entries remembered by InstantRepo.</p>
        </div>
        <button
          id="btn-refresh-manager"
          type="button"
          className="button button-subtle"
          onClick={onRefresh}
          disabled={loading}
          aria-label="Refresh Installed Repos"
        >
          Refresh
        </button>
      </div>

      {repos.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">□</div>
          <p>
            {loaded
              ? "No Installed Repos yet. Clone a repo or analyze a local folder to add it here."
              : "Loading Installed Repos from the Local App Database."}
          </p>
        </div>
      ) : (
        <div className="manager-layout">
          <div className="manager-list" aria-label="Installed Repos">
            {repos.map((repo) => (
              <article
                className={`manager-row ${repo.id === selectedRepoId ? "active" : ""}`}
                key={repo.id}
              >
                <div className="manager-row__body">
                  <div className="manager-row__title">
                    <strong>{repo.projectName}</strong>
                    <StatusBadge status={repo.status} />
                  </div>
                  <p>{repo.localPath}</p>
                  <div className="manager-meta">
                    <span>
                      Last activity{" "}
                      {formatManagerTime(repo.lastActivityAt, "not recorded")}
                    </span>
                    <span>
                      Last setup{" "}
                      {formatManagerTime(repo.lastSetupAt, "not recorded")}
                    </span>
                  </div>
                </div>
                <div className="manager-actions">
                  <button
                    type="button"
                    className="button button-subtle"
                    onClick={() => onShowDetails(repo)}
                    disabled={loading}
                  >
                    History
                  </button>
                  <button
                    type="button"
                    className="button"
                    onClick={() => onAnalyze(repo)}
                    disabled={loading}
                  >
                    Analyze
                  </button>
                </div>
              </article>
            ))}
          </div>

          {details ? (
            <details className="history-panel">
              <summary>
                Recent Setup Sessions for {details.repo.projectName}
              </summary>
              {details.setupSessions.length === 0 ? (
                <p className="history-empty">
                  No Setup Sessions recorded for this Installed Repo.
                </p>
              ) : (
                <div className="history-list">
                  {details.setupSessions.map((session) => (
                    <div className="history-item" key={session.id}>
                      <div>
                        <strong>Session #{session.id}</strong>
                        <p>{session.repoPath}</p>
                      </div>
                      <div className="history-item__meta">
                        <StatusBadge status={session.status} />
                        <span>
                          {formatManagerTime(session.updatedAt, "not recorded")}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </details>
          ) : (
            <div className="history-placeholder">
              <strong>Setup History</strong>
              <p>Select History on an Installed Repo to inspect recent Setup Sessions.</p>
            </div>
          )}
        </div>
      )}
    </section>
  );
}

function StepButton({
  step,
  status,
  active,
  onSelect,
}: {
  step: ExecutionStep;
  status: StepStatus;
  active: boolean;
  onSelect: () => void;
}) {
  return (
    <button
      type="button"
      className={`step-button ${active ? "active" : ""}`}
      onClick={onSelect}
      aria-label={`Select step: ${step.title}`}
      aria-pressed={active}
    >
      <div className="step-button__top">
        <strong>{step.title}</strong>
        <StatusBadge status={status} />
      </div>
      <p>{step.reason}</p>
    </button>
  );
}

export default function App() {
  const [activeView, setActiveView] = useState<AppView>("setup");
  const [repoUrl, setRepoUrl] = useState(initialRepoUrl);
  const [folderPath, setFolderPath] = useState(initialFolder);
  const [snapshot, setSnapshot] = useState<AnalyzeSnapshot | null>(null);
  const [envDraft, setEnvDraft] = useState<EnvDraft | null>(null);
  const [envDraftMode, setEnvDraftMode] = useState<"structured" | "raw">(
    "structured",
  );
  const [selectedRawTarget, setSelectedRawTarget] = useState("");
  const [stepStates, setStepStates] = useState<Record<string, StepStatus>>({});
  const [selectedStepId, setSelectedStepId] = useState("");
  const [activity, setActivity] = useState<ActivityEntry[]>([
    {
      id: "startup",
      time: new Date().toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      }),
      tone: "info",
      label: "Ready",
      message:
        "Desktop controls are live. Analyze a repository to load the real setup plan.",
    },
  ]);
  const [busyLabel, setBusyLabel] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [installedRepos, setInstalledRepos] = useState<InstalledRepoSummary[]>(
    [],
  );
  const [managerLoaded, setManagerLoaded] = useState(false);
  const [managerLoading, setManagerLoading] = useState(false);
  const [selectedManagerRepoId, setSelectedManagerRepoId] = useState<
    number | null
  >(null);
  const [selectedRepoDetails, setSelectedRepoDetails] =
    useState<InstalledRepoDetailsResponse | null>(null);
  const [vaultEntries, setVaultEntries] = useState<EnvVaultManagerEntry[]>([]);
  const [vaultLoading, setVaultLoading] = useState(false);
  const [revealedVaultValues, setRevealedVaultValues] = useState<
    Record<number, string>
  >({});
  const [vaultPromptCandidates, setVaultPromptCandidates] = useState<
    EnvVaultPromptCandidate[]
  >([]);
  const [vaultPromptDisplayName, setVaultPromptDisplayName] = useState("");

  const selectedStep = useMemo(
    () =>
      snapshot?.plan.steps.find((step) => step.id === selectedStepId) ??
      snapshot?.plan.steps[0] ??
      null,
    [selectedStepId, snapshot],
  );

  const unresolvedEnv = useMemo(
    () =>
      snapshot?.plan.env.variables.filter(
        (item) => item.currentStatus === "user_required",
      ) ?? [],
    [snapshot],
  );

  const missingTools = useMemo(
    () => getMissingRequiredTools(snapshot),
    [snapshot],
  );

  const safetyFindings = useMemo(
    () => getSafetyAttention(snapshot),
    [snapshot],
  );

  const appendActivity = useCallback((
    tone: ActivityEntry["tone"],
    label: string,
    message: string,
  ) => {
    const next: ActivityEntry = {
      id: `${Date.now()}-${Math.random().toString(16).slice(2)}`,
      time: new Date().toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      }),
      tone,
      label,
      message: redactLikelySecrets(message),
    };

    setActivity((current) => [next, ...current].slice(0, 6));
  }, []);

  const loadInstalledRepos = useCallback(async () => {
    setManagerLoading(true);
    try {
      const response =
        (await ListInstalledRepos()) as InstalledRepoManagerResponse;
      setInstalledRepos(response.repos ?? []);
      setManagerLoaded(true);
    } catch (error) {
      const message = toErrorMessage(error);
      setManagerLoaded(true);
      setErrorMessage(message);
      appendActivity("critical", "Manager Load Failed", message);
    } finally {
      setManagerLoading(false);
    }
  }, [appendActivity]);

  const loadVaultEntries = useCallback(async () => {
    setVaultLoading(true);
    try {
      const response = await ListEnvVaultEntries();
      setVaultEntries(response.entries ?? []);
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Load Failed", message);
    } finally {
      setVaultLoading(false);
    }
  }, [appendActivity]);

  useEffect(() => {
    void loadInstalledRepos();
  }, [loadInstalledRepos]);

  useEffect(() => {
    if (activeView === "vault") {
      void loadVaultEntries();
    }
  }, [activeView, loadVaultEntries]);

  const syncSnapshot = (nextSnapshot: AnalyzeSnapshot, nextEnvDraft?: EnvDraft) => {
    setSnapshot(nextSnapshot);
    if (nextEnvDraft !== undefined) {
      setEnvDraft(nextEnvDraft);
      setSelectedRawTarget((current) =>
        nextEnvDraft.targets.some((target) => target.relativePath === current)
          ? current
          : (nextEnvDraft.targets[0]?.relativePath ?? ""),
      );
    }
    setStepStates((current) => buildStepStateMap(nextSnapshot, current));
    setSelectedStepId((current) =>
      nextSnapshot.plan.steps.some((step) => step.id === current)
        ? current
        : (nextSnapshot.plan.steps[0]?.id ?? ""),
    );
  };

  const loadEnvDraft = async (
    localPath: string,
    nextSnapshot: AnalyzeSnapshot,
  ) => {
    try {
      return await GenerateEnvDraft(localPath);
    } catch (error) {
      appendActivity(
        "warning",
        "Draft Fallback",
        "Using a local draft preview because the backend draft could not be generated.",
      );
      return buildEnvDraft(nextSnapshot);
    }
  };

  const activeLocalPath = snapshot?.source.path ?? folderPath.trim();

  const handleChooseFolder = async () => {
    try {
      const selected = await OpenDirectory();
      if (selected) {
        setFolderPath(selected);
      }
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Folder Picker Failed", message);
    }
  };

  const handleAnalyze = async (mode: "clone" | "local" | "refresh") => {
    setErrorMessage(null);

    try {
      const cleanRepoUrl = repoUrl.trim();
      const cleanFolderPath = folderPath.trim();

      if (mode === "clone") {
        if (cleanRepoUrl === "") {
          throw new Error("Repository URL is required before cloning.");
        }
        if (cleanFolderPath === "") {
          throw new Error("Choose a destination folder before cloning.");
        }
      }

      if (mode !== "clone" && activeLocalPath === "") {
        throw new Error("A local repository path is required for analysis.");
      }

      setBusyLabel(
        mode === "clone"
          ? "Checking clone destination..."
          : mode === "local"
            ? "Analyzing existing folder..."
            : "Refreshing analysis...",
      );

      let response: AnalyzeSnapshot;
      let successLabel =
        mode === "clone"
          ? "Clone Complete"
          : mode === "local"
            ? "Folder Loaded"
            : "Analysis Refreshed";

      if (mode === "clone") {
        const preflight = await clonePreflight(cleanRepoUrl, cleanFolderPath);
        const plan = planClonePreflightFlow(preflight);

        if (plan.kind === "block") {
          setErrorMessage(plan.message);
          appendActivity("critical", "Clone Blocked", plan.message);
          return;
        }

        if (plan.kind === "open-existing") {
          appendActivity("warning", "Existing Clone Found", plan.message);
          if (window.confirm(buildDuplicateConfirmationMessage(preflight, plan))) {
            setBusyLabel("Opening existing clone...");
            response = await AnalyzeRepository("", plan.localPath);
            successLabel = "Existing Clone Opened";
          } else {
            const forcedPlan = planClonePreflightFlow(preflight, {
              forceClone: true,
            });
            appendActivity("warning", "Clone Another Copy", forcedPlan.message);
            setBusyLabel("Cloning and analyzing repository...");
            response = await ImportRepository(cleanRepoUrl, cleanFolderPath);
          }
        } else if (plan.kind === "confirm-clone") {
          appendActivity("warning", "Clone Needs Attention", plan.message);
          if (!window.confirm(buildCloneConfirmationMessage(preflight))) {
            setErrorMessage("Clone cancelled after preflight.");
            appendActivity("warning", "Clone Cancelled", plan.message);
            return;
          }
          setBusyLabel("Cloning and analyzing repository...");
          response = await ImportRepository(cleanRepoUrl, cleanFolderPath);
        } else {
          appendActivity("info", "Clone Preflight", plan.message);
          setBusyLabel("Cloning and analyzing repository...");
          response = await ImportRepository(cleanRepoUrl, cleanFolderPath);
        }
      } else {
        response = await AnalyzeRepository(
          "",
          mode === "refresh" ? activeLocalPath : cleanFolderPath,
        );
      }

      const draft = await loadEnvDraft(response.source.path, response);
      syncSnapshot(response, draft);

      appendActivity(
        "success",
        successLabel,
        `${response.analysis.projectName} is ready at ${response.source.path}.`,
      );
      await loadInstalledRepos();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Analyze Failed", message);
    } finally {
      setBusyLabel(null);
    }
  };

  const handleGenerateEnv = async () => {
    if (!snapshot) {
      setErrorMessage(
        "Analyze a repository before generating an environment draft.",
      );
      return;
    }

    setErrorMessage(null);
    setBusyLabel("Generating .env draft...");

    try {
      const draft = await loadEnvDraft(snapshot.source.path, snapshot);
      setEnvDraft(draft);
      setSelectedRawTarget(draft.targets[0]?.relativePath ?? "");
      appendActivity(
        "success",
        "Draft Generated",
        `Prepared a draft for ${snapshot.plan.env.targetPath ?? ".env"}.`,
      );
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Draft Failed", message);
    } finally {
      setBusyLabel(null);
    }
  };

  const handleSaveEnv = async () => {
    if (!snapshot || !envDraft) {
      setErrorMessage(
        "Generate an environment draft before saving.",
      );
      return;
    }

    setErrorMessage(null);
    setBusyLabel("Saving .env file...");

    try {
      const response = (await SaveEnvDraft(
        snapshot.source.path,
        envDraft,
      )) as ExecuteResponse;
      syncSnapshot(response, envDraft);
      setStepStates((current) => ({
        ...current,
        "create-env-file": "succeeded",
      }));
      appendActivity(
        "success",
        "Env Saved",
        response.result.stdout.trim() ||
          `Saved ${snapshot.plan.env.targetPath ?? ".env"}.`,
      );
      const candidates = response.vaultPromptCandidates ?? [];
      setVaultPromptCandidates(candidates);
      setVaultPromptDisplayName("");
      if (candidates.length > 0) {
        appendActivity(
          "info",
          "Vault Prompt Ready",
          `${candidates.length} saved credential prompt${candidates.length === 1 ? "" : "s"} ready.`,
        );
      }
      void loadInstalledRepos();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Save Failed", message);
    } finally {
      setBusyLabel(null);
    }
  };

  const handleVaultReveal = async (entry: EnvVaultManagerEntry) => {
    if (!window.confirm(`Reveal ${entry.displayName} temporarily?`)) {
      return;
    }
    try {
      const response = await RevealEnvVaultEntry({
        entryId: entry.id,
        confirmed: true,
      });
      setRevealedVaultValues((current) => ({
        ...current,
        [entry.id]: response.value,
      }));
      const revealUntil = new Date(response.revealUntil).getTime();
      const delay = Number.isFinite(revealUntil)
        ? Math.max(1000, revealUntil - Date.now())
        : 30000;
      window.setTimeout(() => {
        setRevealedVaultValues((current) => {
          const next = { ...current };
          delete next[entry.id];
          return next;
        });
      }, delay);
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Reveal Failed", message);
    }
  };

  const handleVaultRename = async (
    entry: EnvVaultManagerEntry,
    displayName: string,
  ) => {
    try {
      const response = await UpdateEnvVaultEntry({
        entryId: entry.id,
        displayName,
      });
      appendActivity(
        response.needsReview ? "warning" : "success",
        response.needsReview ? "Vault Review" : "Vault Updated",
        response.reviewMessage ?? `Updated ${entry.variableName}.`,
      );
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Update Failed", message);
    }
  };

  const handleVaultUpdateValue = async (
    entry: EnvVaultManagerEntry,
    value: string,
  ) => {
    try {
      const response = await UpdateEnvVaultEntry({
        entryId: entry.id,
        displayName: entry.displayName,
        updateValue: true,
        value,
      });
      appendActivity(
        response.needsReview ? "warning" : "success",
        response.needsReview ? "Vault Review" : "Vault Updated",
        response.reviewMessage ?? `Updated ${entry.variableName}.`,
      );
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Update Failed", message);
    }
  };

  const handleVaultRemove = async (entry: EnvVaultManagerEntry) => {
    if (!window.confirm(`Remove ${entry.displayName} from Env Vault?`)) {
      return;
    }
    try {
      await RemoveEnvVaultEntry(entry.id);
      setRevealedVaultValues((current) => {
        const next = { ...current };
        delete next[entry.id];
        return next;
      });
      appendActivity("success", "Vault Removed", `Removed ${entry.variableName}.`);
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Remove Failed", message);
    }
  };

  const handleVaultStatusChange = async (
    entry: EnvVaultManagerEntry,
    status: EnvVaultStatus,
  ) => {
    try {
      await MarkEnvVaultEntryStatus(entry.id, status);
      appendActivity("success", "Vault Status Updated", `${entry.variableName} is ${status}.`);
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Status Failed", message);
    }
  };

  const handleVaultApprove = async (
    _entry: EnvVaultManagerEntry,
    approval: EnvVaultApprovalRequest,
  ) => {
    try {
      await ApproveEnvVaultEntry(approval);
      appendActivity("success", "Vault Approved", `${approval.variableName} approval saved.`);
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Approval Failed", message);
    }
  };

  const handleVaultRevokeApproval = async (
    entry: EnvVaultManagerEntry,
    approvalID: number,
  ) => {
    try {
      await RevokeEnvVaultApproval(approvalID);
      appendActivity("success", "Vault Approval Revoked", `${entry.variableName} approval revoked.`);
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Approval Failed", message);
    }
  };

  const dismissVaultPrompt = () => {
    setVaultPromptCandidates((current) => current.slice(1));
    setVaultPromptDisplayName("");
  };

  const handleSaveVaultPrompt = async () => {
    const candidate = vaultPromptCandidates[0];
    if (!candidate) {
      return;
    }
    const value = vaultPromptValue(envDraft, candidate);
    if (!value.trim()) {
      appendActivity(
        "warning",
        "Vault Prompt Skipped",
        `${candidate.variableName} no longer has a value to save.`,
      );
      dismissVaultPrompt();
      return;
    }
    try {
      const response = await SaveEnvVaultCredential({
        provider: candidate.provider,
        variableName: candidate.variableName,
        displayName: vaultPromptDisplayName,
        value,
      });
      appendActivity(
        response.needsReview ? "warning" : "success",
        response.needsReview ? "Vault Review" : "Vault Saved",
        response.reviewMessage ?? `${candidate.variableName} saved to Env Vault.`,
      );
      dismissVaultPrompt();
      await loadVaultEntries();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Save Failed", message);
    }
  };

  const handleSuppressVaultPrompt = async () => {
    const candidate = vaultPromptCandidates[0];
    if (!candidate) {
      return;
    }
    try {
      await SuppressEnvVaultPrompt({
        repoPath: candidate.repoPath,
        targetRelativePath: candidate.targetRelativePath,
        variableName: candidate.variableName,
      });
      appendActivity(
        "info",
        "Vault Prompt Hidden",
        `${candidate.variableName} will not ask again for this repo target.`,
      );
      dismissVaultPrompt();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Vault Prompt Failed", message);
    }
  };

  const handleRunStep = async () => {
    if (!snapshot || !selectedStep) {
      setErrorMessage("Analyze a repository before running a setup step.");
      return;
    }

    if (!isExecutableStep(selectedStep)) {
      setErrorMessage(
        `Step "${selectedStep.title}" is informational only and cannot be executed automatically.`,
      );
      return;
    }

    const approveRisky =
      selectedStep.requiresApproval &&
      window.confirm(
        `Run "${selectedStep.title}"?\n\n${selectedStep.command}\n\nThis step requires approval.`,
      );

    if (selectedStep.requiresApproval && !approveRisky) {
      appendActivity(
        "warning",
        "Step Skipped",
        `Approval was declined for ${selectedStep.title}.`,
      );
      return;
    }

    setErrorMessage(null);
    setBusyLabel(`Running ${selectedStep.title}...`);
    setStepStates((current) => ({ ...current, [selectedStep.id]: "running" }));
    appendActivity(
      "info",
      "Step Started",
      redactLikelySecrets(selectedStep.command),
    );

    try {
      const response = (await ExecuteStep(
        snapshot.source.repoUrl ?? repoUrl.trim(),
        snapshot.source.path,
        selectedStep.id,
        approveRisky,
      )) as ExecuteResponse;

      syncSnapshot(response, envDraft ?? undefined);

      if (response.result.stepId === "create-env-file") {
        const draft = await loadEnvDraft(response.source.path, response);
        setEnvDraft(draft);
        setSelectedRawTarget(draft.targets[0]?.relativePath ?? "");
      }

      const nextStatus: StepStatus = response.result.succeeded
        ? "succeeded"
        : "failed";
      setStepStates((current) => ({
        ...current,
        [selectedStep.id]: nextStatus,
      }));

      appendActivity(
        response.result.succeeded ? "success" : "critical",
        response.result.succeeded ? "Step Finished" : "Step Failed",
        response.result.succeeded
          ? `${selectedStep.title} completed in ${response.result.duration}.`
          : `${selectedStep.title} exited with code ${response.result.exitCode}.`,
      );
      void loadInstalledRepos();
    } catch (error) {
      const message = toErrorMessage(error);
      setStepStates((current) => ({ ...current, [selectedStep.id]: "failed" }));
      setErrorMessage(message);
      appendActivity("critical", "Execution Failed", message);
    } finally {
      setBusyLabel(null);
    }
  };

  const handleSuggestFolder = () => {
    const repoName =
      repoUrl
        .trim()
        .split("/")
        .pop()
        ?.replace(/\.git$/i, "") || "instantrepo-demo";
    setFolderPath(`C:\\Users\\Admin\\Desktop\\Workspaces\\${repoName}`);
  };

  const handleShowInstalledRepoDetails = async (repo: InstalledRepoSummary) => {
    setSelectedManagerRepoId(repo.id);
    setManagerLoading(true);
    setErrorMessage(null);
    try {
      const response = (await InstalledRepoDetails(
        repo.id,
      )) as InstalledRepoDetailsResponse;
      setSelectedRepoDetails(response);
      appendActivity(
        "info",
        "History Loaded",
        `Loaded Setup Sessions for ${repo.projectName}.`,
      );
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "History Failed", message);
    } finally {
      setManagerLoading(false);
    }
  };

  const handleAnalyzeInstalledRepo = async (repo: InstalledRepoSummary) => {
    setFolderPath(repo.localPath);
    setErrorMessage(null);
    setBusyLabel(`Analyzing ${repo.projectName}...`);

    try {
      const response = (await AnalyzeRepository(
        "",
        repo.localPath,
      )) as AnalyzeSnapshot;
      const draft = await loadEnvDraft(response.source.path, response);
      syncSnapshot(response, draft);
      appendActivity(
        "success",
        "Installed Repo Loaded",
        `${response.analysis.projectName} is ready at ${response.source.path}.`,
      );
      await loadInstalledRepos();
    } catch (error) {
      const message = toErrorMessage(error);
      setErrorMessage(message);
      appendActivity("critical", "Analyze Failed", message);
    } finally {
      setBusyLabel(null);
    }
  };

  const bannerState = busyLabel ? "busy" : errorMessage ? "failed" : "ready";
  const runDisabled =
    busyLabel !== null || !selectedStep || !isExecutableStep(selectedStep);

  return (
    <div className="app">
      <header className="page-header">
        <div>
          <h1>InstantRepo</h1>
          <p>
            Analyze a repository, prepare the environment file, then run the
            next setup step.
          </p>
        </div>
        <div
          className={`status-banner ${statusClass(bannerState)}`}
          role="status"
          aria-live="polite"
        >
          <strong>
            {busyLabel ?? (errorMessage ? "Action Required" : "Ready")}
          </strong>
          <span>
            {busyLabel
              ? "Please wait for the current action to finish."
              : (errorMessage ?? "Choose an action below to continue.")}
          </span>
        </div>
      </header>

      <AppNav activeView={activeView} onChange={setActiveView} />

      <main className="page-grid">
        {activeView === "repos" ? (
          <InstalledRepoManager
            repos={installedRepos}
            details={selectedRepoDetails}
            selectedRepoId={selectedManagerRepoId}
            loaded={managerLoaded}
            loading={managerLoading || busyLabel !== null}
            onRefresh={() => void loadInstalledRepos()}
            onShowDetails={(repo) => void handleShowInstalledRepoDetails(repo)}
            onAnalyze={(repo) => void handleAnalyzeInstalledRepo(repo)}
          />
        ) : null}

        {activeView === "vault" ? (
          <EnvVaultManager
            entries={vaultEntries}
            loading={vaultLoading}
            revealedValues={revealedVaultValues}
            onRefresh={() => void loadVaultEntries()}
            onReveal={(entry) => void handleVaultReveal(entry)}
            onRename={(entry, displayName) =>
              void handleVaultRename(entry, displayName)
            }
            onUpdateValue={(entry, value) =>
              void handleVaultUpdateValue(entry, value)
            }
            onRemove={(entry) => void handleVaultRemove(entry)}
            onStatusChange={(entry, status) =>
              void handleVaultStatusChange(entry, status)
            }
            onApprove={(entry, approval) =>
              void handleVaultApprove(entry, approval)
            }
            onRevokeApproval={(entry, approvalID) =>
              void handleVaultRevokeApproval(entry, approvalID)
            }
          />
        ) : null}

        {activeView === "settings" ? (
          <section className="card" aria-labelledby="section-settings">
            <div className="section-heading">
              <div>
                <h2 id="section-settings">Settings</h2>
                <p>App settings and contribution controls land in later slices.</p>
              </div>
            </div>
            <div className="empty-state">
              <div className="empty-state-icon">□</div>
              <p>Local app configuration will appear here.</p>
            </div>
          </section>
        ) : null}

        {activeView === "setup" ? (
          <>

        {/* ── Section 1: Repository Input ── */}
        <section className="card" aria-labelledby="section-repo">
          <div className="section-heading">
            <div>
              <h2 id="section-repo">
                <span className="section-number">2</span>Choose Repository
              </h2>
              <p>Paste a remote URL or work from an existing local folder.</p>
            </div>
          </div>

          <div className="field-group">
            <label className="field">
              <span>Repository URL</span>
              <input
                id="input-repo-url"
                value={repoUrl}
                onChange={(event) => setRepoUrl(event.target.value)}
                placeholder="https://github.com/owner/repo"
                aria-label="Repository URL"
              />
            </label>

            <label className="field">
              <span>Destination Folder</span>
              <input
                id="input-folder-path"
                value={folderPath}
                onChange={(event) => setFolderPath(event.target.value)}
                placeholder="C:\Projects\repo"
                aria-label="Destination folder path"
              />
            </label>
          </div>

          <div className="button-row">
            <button
              id="btn-clone"
              type="button"
              className="button button-primary"
              onClick={() => void handleAnalyze("clone")}
              disabled={busyLabel !== null}
              aria-label="Clone repository and analyze"
            >
              Clone &amp; Analyze
            </button>
            <button
              id="btn-analyze-local"
              type="button"
              className="button"
              onClick={() => void handleAnalyze("local")}
              disabled={busyLabel !== null}
              aria-label="Analyze existing local folder"
            >
              Analyze Folder
            </button>
            <button
              id="btn-refresh"
              type="button"
              className="button"
              onClick={() => void handleAnalyze("refresh")}
              disabled={
                busyLabel !== null || (!snapshot && folderPath.trim() === "")
              }
              aria-label="Refresh analysis"
            >
              Refresh
            </button>
            <button
              id="btn-suggest-folder"
              type="button"
              className="button button-subtle"
              onClick={handleSuggestFolder}
              disabled={busyLabel !== null}
              aria-label="Auto-suggest destination folder"
            >
              Suggest Path
            </button>
            <button
              id="btn-browse-folder"
              type="button"
              className="button button-subtle"
              onClick={() => void handleChooseFolder()}
              disabled={busyLabel !== null}
              aria-label="Browse for folder"
            >
              Browse
            </button>
          </div>
        </section>

        {/* ── Section 2: Analysis Summary ── */}
        <section className="card" aria-labelledby="section-summary">
          <div className="section-heading">
            <div>
              <h2 id="section-summary">
                <span className="section-number">3</span>Analysis Summary
              </h2>
              <p>
                Review what InstantRepo found before editing files or running
                commands.
              </p>
            </div>
          </div>

          {snapshot ? (
            <div className="summary-grid">
              <div className="summary-panel">
                <SummaryRow
                  label="Project"
                  value={snapshot.analysis.projectName}
                />
                <SummaryRow
                  label="Type"
                  value={snapshot.analysis.projectType}
                />
                <SummaryRow
                  label="Confidence"
                  value={confidenceLabel(snapshot.analysis.confidence)}
                />
                <SummaryRow
                  label="Repository Path"
                  value={snapshot.source.path}
                />
                <SummaryRow
                  label="Setup Steps"
                  value={String(snapshot.plan.steps.length)}
                />
              </div>

              <div className="summary-panel">
                <h3>Attention Needed</h3>
                <ul className="plain-list">
                  {missingTools.map((tool) => (
                    <li key={tool.tool}>
                      Missing tool: <strong>{tool.tool}</strong>
                    </li>
                  ))}
                  {unresolvedEnv.map((item) => (
                    <li key={item.name}>
                      Required secret: <strong>{item.name}</strong>
                    </li>
                  ))}
                  {safetyFindings.map((finding) => (
                    <li key={`${finding.summary}-${finding.filePath ?? ""}`}>
                      Safety: <strong>{finding.summary}</strong>
                      {finding.filePath ? ` (${finding.filePath})` : ""}
                    </li>
                  ))}
                  {missingTools.length === 0 &&
                  unresolvedEnv.length === 0 &&
                  safetyFindings.length === 0 ? (
                    <li>No immediate blockers detected.</li>
                  ) : null}
                </ul>
              </div>
            </div>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">◇</div>
              <p>
                No repository analyzed yet. Choose a repository above to begin.
              </p>
            </div>
          )}
        </section>

        {/* ── Section 3: Environment File ── */}
        <section className="card" aria-labelledby="section-env">
          <div className="section-heading">
            <div>
              <h2 id="section-env">
                <span className="section-number">4</span>Environment File
              </h2>
              <p>
                Generate the draft first, then edit values directly and save the
                file.
              </p>
            </div>
            <div className="button-row">
              <button
                id="btn-generate-env"
                type="button"
                className="button"
                onClick={() => void handleGenerateEnv()}
                disabled={busyLabel !== null || !snapshot}
                aria-label="Generate environment draft"
              >
                Generate Draft
              </button>
              <button
                id="btn-save-env"
                type="button"
                className="button button-primary"
                onClick={() => void handleSaveEnv()}
                disabled={busyLabel !== null || !snapshot || !envDraft}
                aria-label="Save environment file"
              >
                Save All
              </button>
            </div>
          </div>

          {snapshot && envDraft ? (
            <>
              <div className="env-info">
                {snapshot.plan.env.variables.map((item) => (
                  <div className="env-row" key={item.name}>
                    <div>
                      <strong>{item.name}</strong>
                      <p>
                        {item.instructions?.[0] ??
                          "No additional instructions."}
                      </p>
                    </div>
                    <StatusBadge status={item.currentStatus} />
                  </div>
                ))}
              </div>

              <EnvDraftPanel
                draft={envDraft}
                mode={envDraftMode}
                selectedRawTarget={selectedRawTarget}
                onModeChange={setEnvDraftMode}
                onSelectedRawTargetChange={setSelectedRawTarget}
                onChange={setEnvDraft}
              />
            </>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">⬡</div>
              <p>
                Analyze a repository, then generate a structured Env Draft.
              </p>
            </div>
          )}
        </section>

        {/* ── Section 4: Setup Steps ── */}
        <section className="card" aria-labelledby="section-steps">
          <div className="section-heading">
            <div>
              <h2 id="section-steps">
                <span className="section-number">5</span>Setup Steps
              </h2>
              <p>Select one step, read what it does, then run it.</p>
            </div>
          </div>

          {snapshot ? (
            <div className="steps-layout">
              <div
                className="steps-list"
                role="listbox"
                aria-label="Setup steps"
              >
                {snapshot.plan.steps.map((step) => (
                  <StepButton
                    key={step.id}
                    step={step}
                    status={stepStates[step.id] ?? "pending"}
                    active={step.id === selectedStep?.id}
                    onSelect={() => setSelectedStepId(step.id)}
                  />
                ))}
              </div>

              {selectedStep ? (
                <div
                  className="step-details"
                  aria-label={`Details for ${selectedStep.title}`}
                >
                  <div className="step-details__header">
                    <div>
                      <h3>{selectedStep.title}</h3>
                      <p>{selectedStep.reason}</p>
                    </div>
                    <button
                      id="btn-run-step"
                      type="button"
                      className="button button-primary"
                      onClick={() => void handleRunStep()}
                      disabled={runDisabled}
                      aria-label={
                        isExecutableStep(selectedStep)
                          ? `Run ${selectedStep.title}`
                          : "Manual review required"
                      }
                    >
                      {isExecutableStep(selectedStep)
                        ? "Run Step"
                        : "Manual Review"}
                    </button>
                  </div>

                  <div className="summary-panel">
                    <SummaryRow
                      label="Status"
                      value={stepStates[selectedStep.id] ?? "pending"}
                    />
                    <SummaryRow
                      label="Importance"
                      value={selectedStep.importance}
                    />
                    <SummaryRow label="Risk" value={selectedStep.risk} />
                    <SummaryRow
                      label="Approval"
                      value={
                        selectedStep.requiresApproval
                          ? "required"
                          : "not required"
                      }
                    />
                  </div>

                  <div className="command-box">
                    <span>Command</span>
                    <code>{selectedStep.command}</code>
                  </div>

                  <div>
                    <h4>Confirmed By</h4>
                    <ul className="plain-list">
                      {(selectedStep.confirmedBy ?? []).map((item) => (
                        <li key={item}>{item}</li>
                      ))}
                    </ul>
                  </div>
                </div>
              ) : null}
            </div>
          ) : (
            <div className="empty-state">
              <div className="empty-state-icon">△</div>
              <p>The setup plan will appear here after analysis.</p>
            </div>
          )}
        </section>

        {/* ── Section 5: Activity Feed ── */}
        <section className="card" aria-labelledby="section-activity">
          <div className="section-heading">
            <div>
              <h2 id="section-activity">
                <span className="section-number">6</span>Recent Events
              </h2>
              <p>Simple feedback for the last actions taken in this session.</p>
            </div>
          </div>

          <div
            className="activity-list"
            role="log"
            aria-label="Recent activity"
          >
            {activity.map((entry) => (
              <div
                className={`activity-item ${toneClass(entry.tone)}`}
                key={entry.id}
              >
                <div className="activity-item__header">
                  <strong>{entry.label}</strong>
                  <span>{entry.time}</span>
                </div>
                <p>{entry.message}</p>
              </div>
            ))}
          </div>
        </section>
          </>
        ) : null}

        {vaultPromptCandidates[0] ? (
          <EnvVaultPrompt
            candidate={vaultPromptCandidates[0]}
            displayName={vaultPromptDisplayName}
            onDisplayNameChange={setVaultPromptDisplayName}
            onSave={() => void handleSaveVaultPrompt()}
            onDismiss={dismissVaultPrompt}
            onSuppress={() => void handleSuppressVaultPrompt()}
          />
        ) : null}
      </main>
    </div>
  );
}
