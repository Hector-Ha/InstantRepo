export type AppView = "setup" | "repos" | "vault" | "settings";

const viewLabels: Record<AppView, string> = {
  setup: "Setup",
  repos: "Installed Repos",
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
    <nav className="app-nav" aria-label="Main views">
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
