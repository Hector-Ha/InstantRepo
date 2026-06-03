import type {
  EnvVaultApprovalRequest,
  EnvVaultManagerEntry,
  EnvVaultStatus,
} from "./types";

const statusOptions: Array<{ label: string; value: EnvVaultStatus }> = [
  { label: "Ready", value: "ready" },
  { label: "Action Needed", value: "action_needed" },
  { label: "Invalid", value: "invalid" },
  { label: "Needs Review", value: "needs_review" },
];

function statusLabel(status: string) {
  return status.replace(/_/g, " ");
}

function usageLabel(count: number) {
  return count === 1 ? "1 use" : `${count} uses`;
}

export function EnvVaultManager({
  entries,
  loading,
  revealedValues,
  onRefresh,
  onReveal,
  onRename,
  onUpdateValue,
  onRemove,
  onStatusChange,
  onApprove,
  onRevokeApproval,
}: {
  entries: EnvVaultManagerEntry[];
  loading: boolean;
  revealedValues: Record<number, string>;
  onRefresh: () => void;
  onReveal: (entry: EnvVaultManagerEntry) => void;
  onRename: (entry: EnvVaultManagerEntry, displayName: string) => void;
  onUpdateValue: (entry: EnvVaultManagerEntry, value: string) => void;
  onRemove: (entry: EnvVaultManagerEntry) => void;
  onStatusChange: (entry: EnvVaultManagerEntry, status: EnvVaultStatus) => void;
  onApprove: (
    entry: EnvVaultManagerEntry,
    approval: EnvVaultApprovalRequest,
  ) => void;
  onRevokeApproval: (entry: EnvVaultManagerEntry, approvalID: number) => void;
}) {
  return (
    <section className="card" aria-labelledby="section-vault">
      <div className="section-heading">
        <div>
          <h2 id="section-vault">Env Vault</h2>
          <p>Manage reusable Service Credentials, approvals, and review states.</p>
        </div>
        <button type="button" onClick={onRefresh} disabled={loading}>
          Refresh
        </button>
      </div>

      {entries.length === 0 ? (
        <div className="empty-state">
          <div className="empty-state-icon">◇</div>
          <p>No saved Service Credentials.</p>
        </div>
      ) : (
        <div className="vault-manager-list">
          {entries.map((entry) => {
            const revealedValue = revealedValues[entry.id];
            const usageLocations = Array.isArray(entry.usage?.locations)
              ? entry.usage.locations
              : [];
            const totalUseCount = entry.usage?.totalUseCount ?? 0;
            const approvals = Array.isArray(entry.approvals)
              ? entry.approvals
              : [];
            return (
              <article className="vault-manager-entry" key={entry.id}>
                <div className="vault-manager-entry__top">
                  <div>
                    <h3>{entry.displayName}</h3>
                    <div className="vault-manager-entry__meta">
                      <span>{entry.provider}</span>
                      <span>{entry.variableName}</span>
                      <code>{entry.fingerprintFragment}</code>
                    </div>
                  </div>
                  <span className={`status-pill ${entry.status}`}>
                    {statusLabel(entry.status)}
                  </span>
                </div>

                <div className="vault-secret-preview" aria-live="polite">
                  <span>Value</span>
                  <code>{revealedValue ?? "••••••••••••"}</code>
                </div>

                <div className="vault-manager-actions">
                  <button type="button" onClick={() => onReveal(entry)}>
                    Reveal
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      const next = window.prompt(
                        "Display name",
                        entry.displayName,
                      );
                      if (next !== null) {
                        onRename(entry, next);
                      }
                    }}
                  >
                    Rename
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      const next = window.prompt("New credential value");
                      if (next !== null) {
                        onUpdateValue(entry, next);
                      }
                    }}
                  >
                    Update Value
                  </button>
                  <button type="button" onClick={() => onRemove(entry)}>
                    Remove
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      const repoPath = window.prompt("Repo path");
                      if (!repoPath) {
                        return;
                      }
                      const targetRelativePath =
                        window.prompt("Target env file", ".env") ?? "";
                      if (!targetRelativePath) {
                        return;
                      }
                      const variableName =
                        window.prompt("Env var", entry.variableName) ?? "";
                      if (!variableName) {
                        return;
                      }
                      onApprove(entry, {
                        entryId: entry.id,
                        repoPath,
                        targetRelativePath,
                        variableName,
                      });
                    }}
                  >
                    Add Approval
                  </button>
                </div>

                <div className="vault-status-actions" aria-label="Vault status actions">
                  {statusOptions.map((option) => (
                    <button
                      type="button"
                      key={option.value}
                      className={entry.status === option.value ? "active" : ""}
                      onClick={() => onStatusChange(entry, option.value)}
                    >
                      {option.label}
                    </button>
                  ))}
                </div>

                <div className="vault-manager-entry__details">
                  <section>
                    <h4>{usageLabel(totalUseCount)}</h4>
                    {usageLocations.length === 0 ? (
                      <p>No recorded use.</p>
                    ) : (
                      <ul>
                        {usageLocations.map((location) => (
                          <li
                            key={`${location.repoPath}-${location.targetRelativePath}-${location.variableName}`}
                          >
                            <strong>{location.targetRelativePath}</strong>
                            <span>{location.repoPath}</span>
                            <small>{usageLabel(location.useCount)}</small>
                          </li>
                        ))}
                      </ul>
                    )}
                  </section>

                  <section>
                    <h4>Approvals</h4>
                    {approvals.length === 0 ? (
                      <p>No repo approvals.</p>
                    ) : (
                      <ul>
                        {approvals.map((approval) => (
                          <li key={approval.id}>
                            <strong>{approval.targetRelativePath}</strong>
                            <span>{approval.repoPath}</span>
                            <small>{approval.status}</small>
                            {approval.status === "approved" ? (
                              <button
                                type="button"
                                onClick={() => onRevokeApproval(entry, approval.id)}
                              >
                                Revoke
                              </button>
                            ) : null}
                          </li>
                        ))}
                      </ul>
                    )}
                  </section>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </section>
  );
}
