import { useEffect, useState } from "react";
import type {
  EnvContributionSettings,
  EnvContributionSettingsResponse,
} from "./types";

function queueLabel(count: number) {
  return count === 1 ? "1 queued" : `${count} queued`;
}

export function SettingsView({
  response,
  loading,
  onRefresh,
  onSaveSettings,
  onRecordConsent,
  onClearQueue,
}: {
  response: EnvContributionSettingsResponse | null;
  loading: boolean;
  onRefresh: () => void;
  onSaveSettings: (settings: EnvContributionSettings) => void;
  onRecordConsent: (publicEnabled: boolean) => void;
  onClearQueue: () => void;
}) {
  const settings = response?.settings;
  const [consentPublicEnabled, setConsentPublicEnabled] = useState(
    settings?.publicEnvPatternsEnabled ?? true,
  );

  useEffect(() => {
    setConsentPublicEnabled(settings?.publicEnvPatternsEnabled ?? true);
  }, [settings?.publicEnvPatternsEnabled]);

  return (
    <section className="card" aria-labelledby="section-settings">
      <div className="section-heading">
        <div>
          <h2 id="section-settings">Settings</h2>
          <p>Local app controls for value-free env pattern contribution.</p>
        </div>
        <button type="button" onClick={onRefresh} disabled={loading}>
          Refresh
        </button>
      </div>

      {!response || !settings ? (
        <div className="empty-state">
          <div className="empty-state-icon">□</div>
          <p>Loading local settings.</p>
        </div>
      ) : (
        <div className="settings-layout">
          {!settings.consentShown ? (
            <section className="settings-panel consent-panel">
              <h3>Env Pattern Contribution</h3>
              <label className="checkbox-row">
                <input
                  type="checkbox"
                  checked={consentPublicEnabled}
                  onChange={(event) =>
                    setConsentPublicEnabled(event.currentTarget.checked)
                  }
                />
                <span>Public repos</span>
              </label>
              <button
                type="button"
                onClick={() => onRecordConsent(consentPublicEnabled)}
              >
                Save choice
              </button>
            </section>
          ) : null}

          <section className="settings-panel">
            <h3>Env Pattern Contribution</h3>
            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={settings.publicEnvPatternsEnabled}
                onChange={(event) =>
                  onSaveSettings({
                    ...settings,
                    consentShown: true,
                    publicEnvPatternsEnabled: event.currentTarget.checked,
                  })
                }
              />
              <span>Public repos</span>
            </label>
            <label className="checkbox-row">
              <input
                type="checkbox"
                checked={settings.privateLocalEnvPatternsEnabled}
                onChange={(event) =>
                  onSaveSettings({
                    ...settings,
                    consentShown: true,
                    privateLocalEnvPatternsEnabled: event.currentTarget.checked,
                  })
                }
              />
              <span>Private/local repos</span>
            </label>
          </section>

          <section className="settings-panel queue-panel">
            <div>
              <h3>Offline Queue</h3>
              <p>{queueLabel(response.queue.count)}</p>
            </div>
            <button
              type="button"
              onClick={onClearQueue}
              disabled={loading || response.queue.count === 0}
            >
              Clear queue
            </button>
          </section>
        </div>
      )}
    </section>
  );
}
