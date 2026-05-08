# InstantRepo Context

InstantRepo helps low-code users turn a repo URL or local folder into a prepared local repo. It must keep setup easy while making command risk and missing user action visible.

## Language

**Guarded Setup**:
Default flow where user reviews a setup plan and approves one setup step at a time.
_Avoid_: manual mode, safe mode

**Auto Setup**:
Opt-in batch flow that runs bounded setup steps after user accepts a danger modal.
_Avoid_: one-click install, blind auto-run

**Setup Plan**:
Ordered list of setup steps, attention items, env needs, services, and safety findings for one repo.
_Avoid_: checklist, script

**Setup Step**:
One actionable repo setup operation, such as preparing env, starting local services, or installing repo dependencies.
_Avoid_: task, command

**Attention**:
User-visible issue that needs awareness or action but is not itself command danger.
_Avoid_: risk, blocker

**Risk**:
Danger level of executing a command or touching local machine state.
_Avoid_: attention, warning

**System Tool**:
Machine-level tool outside the repo, such as Git, Node, Bun, Python, Go, Docker, Wails, winget, or brew.
_Avoid_: dependency, package

**Repo Dependency**:
Project dependency installed for the repo, such as Node packages, Python venv packages, or Go modules.
_Avoid_: system tool

**Install Script Policy**:
User choice for whether repo dependency install may run package lifecycle scripts.
_Avoid_: safer install commands

**Lifecycle Script**:
Package-manager script that runs during dependency install, such as `preinstall`, `install`, `postinstall`, or `prepare`.
_Avoid_: setup script

**Command Review**:
Classification path for commands that may be useful but need context before **Auto Setup** can run them.
_Avoid_: manual review

**Allowed Command List**:
User-editable policy list that lets known command patterns bypass repeated review.
_Avoid_: whitelist, safe list

**AI Advisor**:
Optional assistant that reviews shallow repo context and uncertain commands when deterministic checks are not confident.
_Avoid_: AI runner, source of truth

**AI Provider Key**:
User-provided credential for an external AI provider used by **AI Advisor**.
_Avoid_: built-in secret, repo key

**AI Review Log**:
Local diagnostic record of **AI Advisor** command decisions, kept outside the app UI.
_Avoid_: step history, user-facing activity

**Repo Diagnostic Export**:
User-triggered export of setup diagnostics for one installed repo.
_Avoid_: telemetry, cloud log

**Setup Session**:
One analyze flow and following setup actions for the same repo until app closes, repo changes, or user starts a new setup run.
_Avoid_: step run, app session

**Env Draft**:
Generated `.env` file content with preserved values, local defaults, placeholders, and instructions.
_Avoid_: secrets file, config dump

**Installed Repo**:
Tracked local repo with saved source URL, local path, lifecycle status, and setup step history.
_Avoid_: downloaded repo, clone

**Clone History**:
Local app metadata that records repo URLs, normalized URLs, clone paths, and installed repo state.
_Avoid_: registry, remote sync

**Local App Database**:
SQLite database owned by InstantRepo for local metadata, setup sessions, and status tracking.
_Avoid_: JSON tracker, server database

**Launch**:
User action that starts the prepared app after setup is complete.
_Avoid_: auto setup, run step

## Relationships

- A **Setup Plan** belongs to exactly one repo path.
- A **Setup Plan** contains zero or more **Setup Steps**.
- **Auto Setup** runs only bounded **Setup Steps** from the **Setup Plan**.
- **Auto Setup** stops before **Launch**.
- **Launch** uses the stored successful launch command when available, with refresh/re-analyze available separately.
- **Attention** may block **Auto Setup** even when **Risk** is low.
- A **System Tool** may be missing and create **Attention** without being a high **Risk** by itself.
- Installing a **System Tool** is outside **Auto Setup**.
- Default **Install Script Policy** favors compatibility by allowing normal install scripts.
- **Install Script Policy** is chosen in the **Auto Setup** danger modal because it changes compatibility versus caution.
- Alternate **Install Script Policy** can skip install scripts, but may break repo setup.
- Suspicious **Lifecycle Scripts** stop **Auto Setup** before dependency install and create visible **Risk**.
- **Command Review** uses deterministic rules first, then **AI Advisor** when enabled and needed.
- **AI Advisor** is optional and depends on a configured **AI Provider Key**.
- **AI Provider Key** is stored in the operating system credential store, not repo files or clone history.
- **AI Advisor** receives only shallow context, such as repo link, manifests, setup docs snippets, env names without values, entrypoint names, and command under review.
- **AI Advisor** runs in background and is last resort; deterministic checks should avoid provider calls when not needed.
- **Auto Setup** pauses at **Command Review** while **AI Advisor** evaluates; high confidence resumes automatically, timeout or low confidence asks user.
- **AI Advisor** command review has a five minute maximum wait.
- **AI Advisor** can be enabled or disabled globally and per repo.
- **AI Advisor** shallow context sharing is enabled by default when AI is configured, with an Advanced option to disable it.
- **AI Advisor** decisions are kept in a hidden local **AI Review Log**, not shown in normal app UI and not stored as setup step history.
- **AI Review Log** keeps metadata only and expires entries after 30 days.
- **Repo Diagnostic Export** can bundle logs and setup diagnostics for one repo when user requests it.
- **Repo Diagnostic Export** includes repo identity, app/environment versions, analysis summary, setup plan, step statuses, exit codes, redacted truncated logs, and **AI Review Log** metadata for that repo.
- **Repo Diagnostic Export** must exclude env values, API keys, full AI prompts, full AI responses, and full source files.
- Secret redaction happens before persistent log storage and again during **Repo Diagnostic Export**.
- Live UI logs also redact likely secrets.
- Persistent repo setup logs are bounded to 7 days and the last 10 setup sessions.
- A **Setup Session** starts with analysis and groups following guarded or auto setup actions for the same repo.
- **Guarded Setup** and **Auto Setup** both create **Setup Session** logs.
- Product order is manual setup quality first, deterministic automation second, small **AI Advisor** integration last.
- Foundation work comes before **Auto Setup**: clone preflight, **Clone History**, **Installed Repo** records, **Setup Session** records, step status/log persistence, and diagnostic export skeleton.
- Foundation implementation should start backend-first, then UI consumes store/preflight/session behavior.
- Clone preflight returns destination writability, derived target path, folder existence/emptiness, duplicate repos, path conflict, disk status, recommended action, and user-facing messages.
- Without **AI Advisor**, deterministic allowed commands can still run, but review commands ask the user.
- High-confidence **AI Advisor** decisions may let **Auto Setup** proceed with a visible note.
- Low-confidence **AI Advisor** decisions require user confirmation.
- Users can inspect and adjust the **Allowed Command List**.
- An **Installed Repo** is recorded in **Clone History**.
- **Clone History** is local app metadata and is not written into cloned repos.
- **Clone History**, **Installed Repos**, and **Setup Sessions** live in the **Local App Database**.
- **Local App Database** must follow security-first defaults: no secrets, strict file permissions where possible, migrations, bounded data, and redacted logs.
- **Local App Database** does not require encryption at rest while it stores no sensitive values; add encryption only if sensitive data must be stored later.
- **Local App Database** uses versioned migrations from the first release.
- **Local App Database** access belongs in `internal/store`; service orchestration stays in `internal/service`, analyzer logic stays in `internal/analyzer`.
- `internal/store` exposes domain-specific methods and hides SQL details from service code.
- `internal/service` defines small persistence interfaces it needs; `internal/store` implements them with SQLite.
- First schema includes `installed_repos`, `setup_sessions`, `step_runs`, `app_settings`, and `schema_migrations`; **AI Advisor** tables come later.
- Repo URLs are rarely sensitive for this product and may be stored in the **Local App Database** for duplicate detection and manager UX.
- Local repo paths may be mildly sensitive but are stored locally because manager and launch need them.
- **Repo Diagnostic Export** can include local path by default, with a later privacy mode option to redact paths.
- Redacted setup logs live as local files referenced by the **Local App Database**, not as large blobs inside SQLite.
- Manager UI shows **Installed Repos** first; **Setup Sessions** live in a details/history view.

## Example Dialogue

> **Dev:** "Can **Auto Setup** install Docker if it is missing?"
> **Domain expert:** "No. Missing Docker is **Attention**. Installing Docker is a separate guided system action, not part of **Auto Setup**."

> **Dev:** "Can **Auto Setup** start the app?"
> **Domain expert:** "No. Setup prepares the repo. **Launch** is separate because app commands may run forever or need user choices."

> **Dev:** "Can AI make a command run during **Auto Setup**?"
> **Domain expert:** "Only through **Command Review**. If **AI Advisor** is high-confidence, setup can proceed with a visible note; low confidence asks the user."

## Flagged Ambiguities

- "warning" was used for both **Attention** and **Risk**. Resolved: **Attention** means user must notice or act; **Risk** means command danger.
- "system tool" was unclear. Resolved: **System Tool** means machine-level tooling outside the repo, not project packages.
- "one-click auto-run" was too broad. Resolved: use **Auto Setup** for bounded setup batch, not blind execution.
- "safer install commands" was vague. Resolved: use **Install Script Policy**; default allows normal install scripts, advanced option can skip them with a breakage warning.
- Multi-command **Launch** is out of MVP. Current direction: support one primary launch command first; later consider user-defined launch groups that start multiple commands together.
