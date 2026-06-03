export type AppView =
  | "overview"
  | "repos"
  | "environment"
  | "steps"
  | "vault"
  | "settings";

const viewLabels: Record<AppView, string> = {
  overview: "Overview",
  repos: "Repositories",
  environment: "Environment",
  steps: "Setup Steps",
  vault: "Env Vault",
  settings: "Settings",
};

export function AppNav({
  activeView,
  onChange,
}: {
  activeView: AppView;
  onChange: (view: AppView) => void;
}) {
  return (
    <nav className="app-nav" aria-label="Product areas">
      {(Object.keys(viewLabels) as AppView[]).map((view) => (
        <button
          key={view}
          type="button"
          className={`nav-tab ${activeView === view ? "active" : ""}`}
          onClick={() => onChange(view)}
          aria-pressed={activeView === view}
        >
          {viewLabels[view]}
        </button>
      ))}
    </nav>
  );
}
