import type { EnvDraft, EnvDraftValue } from "./types";
import {
  applyRawTargetContent,
  clearVaultBinding,
  renderRawTarget,
  updateEnvDraftValue,
  vaultBindingLabel,
} from "./envDraft";

type DraftMode = "structured" | "raw";

function confidenceLabel(value: number) {
  return `${Math.round(value * 100)}%`;
}

function valueMeta(value: EnvDraftValue) {
  return [
    value.valueClass,
    value.provenance.source,
    confidenceLabel(value.confidence),
  ]
    .filter(Boolean)
    .join(" · ");
}

export function EnvDraftPanel({
  draft,
  mode,
  selectedRawTarget,
  onModeChange,
  onSelectedRawTargetChange,
  onChange,
}: {
  draft: EnvDraft;
  mode: DraftMode;
  selectedRawTarget: string;
  onModeChange: (mode: DraftMode) => void;
  onSelectedRawTargetChange: (relativePath: string) => void;
  onChange: (draft: EnvDraft) => void;
}) {
  const activeTarget =
    draft.targets.find((target) => target.relativePath === selectedRawTarget) ??
    draft.targets[0] ??
    null;

  return (
    <div className="env-draft-panel">
      <div className="segmented-control" aria-label="Env draft view mode">
        <button
          type="button"
          className={mode === "structured" ? "active" : ""}
          onClick={() => onModeChange("structured")}
        >
          Structured
        </button>
        <button
          type="button"
          className={mode === "raw" ? "active" : ""}
          onClick={() => onModeChange("raw")}
        >
          Raw Target
        </button>
      </div>

      {mode === "raw" && activeTarget ? (
        <div className="raw-target-editor">
          <label className="field">
            <span>Target</span>
            <select
              value={activeTarget.relativePath}
              onChange={(event) => onSelectedRawTargetChange(event.target.value)}
            >
              {draft.targets.map((target) => (
                <option key={target.relativePath} value={target.relativePath}>
                  {target.relativePath}
                </option>
              ))}
            </select>
          </label>
          <textarea
            value={renderRawTarget(activeTarget)}
            onChange={(event) =>
              onChange(
                applyRawTargetContent(
                  draft,
                  activeTarget.relativePath,
                  event.target.value,
                ),
              )
            }
            spellCheck={false}
            aria-label={`Raw editor for ${activeTarget.relativePath}`}
          />
        </div>
      ) : null}

      {mode === "structured" ? (
        <div className="env-target-list">
          {draft.targets.map((target) => (
            <section
              className="env-target"
              key={target.relativePath}
              aria-labelledby={`env-target-${target.relativePath}`}
            >
              <div className="env-target__header">
                <h3 id={`env-target-${target.relativePath}`}>
                  {target.relativePath}
                </h3>
                <span>{target.values.length} values</span>
              </div>
              <div className="env-value-list">
                {target.values.map((value) => (
                  <div className="env-value-row" key={value.name}>
                    <div className="env-value-row__main">
                      <label>
                        <span>{value.name}</span>
                        {value.vaultBinding ? (
                          <div className="vault-tag">
                            <strong>{vaultBindingLabel(value.vaultBinding)}</strong>
                            <code>{value.vaultBinding.fingerprint}</code>
                            <button
                              type="button"
                              onClick={() =>
                                onChange(
                                  clearVaultBinding(
                                    draft,
                                    target.relativePath,
                                    value.name,
                                  ),
                                )
                              }
                            >
                              Remove
                            </button>
                          </div>
                        ) : (
                          <input
                            value={value.value}
                            type={value.secret ? "password" : "text"}
                            onChange={(event) =>
                              onChange(
                                updateEnvDraftValue(
                                  draft,
                                  target.relativePath,
                                  value.name,
                                  event.target.value,
                                ),
                              )
                            }
                          />
                        )}
                      </label>
                      <small>{valueMeta(value)}</small>
                    </div>
                    {(value.attention?.length ?? 0) > 0 ? (
                      <ul className="env-attention">
                        {value.attention?.map((item) => (
                          <li key={item}>{item}</li>
                        ))}
                      </ul>
                    ) : null}
                  </div>
                ))}
              </div>
            </section>
          ))}
        </div>
      ) : null}
    </div>
  );
}
