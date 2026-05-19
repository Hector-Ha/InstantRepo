export function EnvContributionConsentPanel({
  publicEnabled,
  loading,
  onPublicEnabledChange,
  onSave,
}: {
  publicEnabled: boolean;
  loading: boolean;
  onPublicEnabledChange: (enabled: boolean) => void;
  onSave: () => void;
}) {
  return (
    <section
      className="settings-panel contribution-consent-panel"
      aria-labelledby="env-contribution-consent-title"
    >
      <div>
        <h3 id="env-contribution-consent-title">Env Pattern Contribution</h3>
        <p>
          Share value-free env names from confirmed public repos. No env values
          or secrets are sent.
        </p>
      </div>
      <label className="checkbox-row">
        <input
          type="checkbox"
          checked={publicEnabled}
          onChange={(event) =>
            onPublicEnabledChange(event.currentTarget.checked)
          }
        />
        <span>Public repos</span>
      </label>
      <div className="vault-prompt-actions">
        <button
          type="button"
          className="button button-primary"
          onClick={onSave}
          disabled={loading}
        >
          Save choice
        </button>
      </div>
    </section>
  );
}
