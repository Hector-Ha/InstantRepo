import type { EnvVaultPromptCandidate } from "./types";

export function EnvVaultPrompt({
  candidate,
  displayName,
  onDisplayNameChange,
  onSave,
  onDismiss,
  onSuppress,
}: {
  candidate: EnvVaultPromptCandidate;
  displayName: string;
  onDisplayNameChange: (value: string) => void;
  onSave: () => void;
  onDismiss: () => void;
  onSuppress: () => void;
}) {
  return (
    <div className="modal-backdrop" role="presentation">
      <section className="vault-prompt" role="dialog" aria-modal="true">
        <div className="section-heading">
          <div>
            <h2>Save to Env Vault?</h2>
            <p>
              You entered {candidate.variableName}. Save it for future matching
              repos?
            </p>
          </div>
        </div>
        <label className="field">
          <span>Display name</span>
          <input
            value={displayName}
            onChange={(event) => onDisplayNameChange(event.target.value)}
            placeholder={`${candidate.provider} ${candidate.fingerprintFragment}`}
          />
        </label>
        <div className="vault-prompt-actions">
          <button type="button" onClick={onSave}>
            Save to Vault
          </button>
          <button type="button" onClick={onDismiss}>
            Not now
          </button>
          <button type="button" onClick={onSuppress}>
            Never ask for this var
          </button>
        </div>
      </section>
    </div>
  );
}
