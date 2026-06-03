# AGENTS.md

For code review tasks, use `REVIEWERS.md` as the task guide. `REVIEWERS.md` is authoritative for review stance, output shape, and review checks.

Me agent. Me follow this file for implementation, docs, triage, QA, and repo work. Keep talk caveman: short words, clear words, no fancy fluff.

## Project

InstantRepo is a Go + Wails app that helps users prepare local repos safely.

- Backend engine: `internal/service`, `internal/analyzer`, `internal/store`.
- CLI/API entry: `cmd/instantrepo`.
- Desktop app: `cmd/instantrepo-wails`.
- Frontend: React + Vite + TypeScript in `cmd/instantrepo-wails/frontend`.
- CLI mirror parser and app-data policy: `internal/command`.
- Local metadata store: SQLite in `internal/store`; no raw secrets.

## Agent Skills

- Use `triage` when changing issue state or preparing issue work.
- Use `to-prd` only when turning discussion into a PRD issue.
- Use `to-issues` only after PRD/plan is clear enough for slices.
- Use `tdd` when implementing issue work or fixing bugs.
- Use `handoff` before stopping long work another agent must continue.
- Use `caveman-commit` for commit messages.
- Use `caveman-review` for review comments.
- Use `create-readme` for README changes.
- Use `create-agentsmd` for AGENTS.md changes.
- Use `vercel-react-best-practices` for React/Next work.

## Context

Read `CONTEXT.md` and `docs/adr/` before roadmap or domain-sensitive work.

Issue tracker:

- GitHub Issues: `Hector-Ha/InstantRepo`.
- Issue tracker docs: `docs/agents/issue-tracker.md`.
- Triage labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`.

Roadmap state:

- Env Draft foundation is complete through #15-#24.
- Architecture cleanup is complete through #25-#27.
- CLI mirror foundation is complete through #35-#40.
- Verify GitHub before saying #34 or #41 is closed.
- Current Env Draft follow-up work is under #42-#45.

Key docs:

- CLI mirror contract: `docs/cli-mirror.md`.
- Private QA boundary: `docs/adr/0003-use-private-local-qa-harness-with-safe-cli-surfaces.md`.
- Manual QA plan: `test/TEST_PLAN.md`.

## Private QA

`.qa-local/` is ignored and private.

- Do not commit `.qa-local/`.
- Use `.qa-local/` only when tester asks for QA/private harness work.
- Public issues/docs must not include secrets or private harness details.
- Public app must not add QA-only backdoors, hidden debug bridges, raw shell endpoints, approval bypasses, or shipped QA overlays.
- Release/user artifacts must not include `.qa-local/`, QA overlays, private reports, screenshots, evidence, or private shim code.

## Package Manager

- Use `bun` before `pnpm` before `npm`.
- Use `bunx` before `npx`.

## Setup

```bash
cd cmd/instantrepo-wails/frontend
bun install
go mod download
```

## Common Commands

Desktop app:

```bash
cd cmd/instantrepo-wails
wails dev
wails build -clean
```

Frontend:

```bash
cd cmd/instantrepo-wails/frontend
bun test
bun run build
```

Go tests:

```bash
go test ./...
go test ./internal/service
go test ./internal/service -run TestName
```

CLI mirror:

```bash
go run ./cmd/instantrepo version --json
go run ./cmd/instantrepo shell info --json
go run ./cmd/instantrepo repo analyze --path C:\path\to\repo --json
go run ./cmd/instantrepo repo preflight --repo https://github.com/user/repo --destination C:\work --json
go run ./cmd/instantrepo repo import --repo https://github.com/user/repo --destination C:\work --json
go run ./cmd/instantrepo repo execute --path C:\path\to\repo --step install-node-deps --approve --json
go run ./cmd/instantrepo repo list --json
go run ./cmd/instantrepo repo details --id 123 --json
go run ./cmd/instantrepo repo diagnostics --path C:\path\to\repo --json
go run ./cmd/instantrepo env draft generate --path C:\path\to\repo --json
go run ./cmd/instantrepo env draft save --path C:\path\to\repo --file C:\path\to\draft.json --json
go run ./cmd/instantrepo env raw save --path C:\path\to\repo --file C:\path\to\.env --json
go run ./cmd/instantrepo env vault list --json
go run ./cmd/instantrepo settings contribution get --json
go run ./cmd/instantrepo settings ai-env-review get --json
```

Isolated app data:

```bash
go run ./cmd/instantrepo --app-data-dir C:\temp\instantrepo-app-data repo analyze --path C:\path\to\repo --json
```

`INSTANTREPO_APP_DATA_DIR` also isolates CLI and Wails metadata. Path must be absolute and must not be home, drive root, repo root, target repo, or inside target repo.

No-ship guard:

```bash
go test ./internal/command -run TestPrivateQALocalWorkspaceRemainsIgnored
git check-ignore .qa-local/
```

Before done:

- Run `go test ./...` when Go code changed.
- Run `bun test` and `bun run build` when frontend code changed.
- Run `wails build -clean` when Wails bindings or exposed app methods changed.

## Code Style

- Keep Go code simple. Use `gofmt`.
- Keep errors wrapped with useful context.
- Keep domain structs in `internal/domain`.
- Keep analyzer logic in `internal/analyzer`.
- Keep execution/planning in `internal/service`.
- Keep SQL details in `internal/store`.
- Keep CLI parsing/app-data policy in `internal/command`.
- Do not mix UI code with service engine code.
- Do not edit generated Wails files by hand unless generation is broken.
- Do not commit build outputs unless user asks.

## React Rules

- Keep derived values in render or memo, not effect.
- Use primitive effect deps where possible.
- Use functional state updates for state based on old state.
- Memoize only real expensive work.
- Import direct modules. Avoid large barrel imports.
- Clean up global listeners.

## Safety

InstantRepo inspects and can run code from unknown repos.

- Never auto-run repo commands without explicit user request or approval path.
- Keep approval gates for risky steps.
- Treat scripts, installers, binaries, and shell files as risky evidence.
- Preserve existing `.env` values. Never print secrets.
- Do not write real secret values into docs or tests.
- Do not store service credential values in SQLite, logs, issues, PRs, diagnostics, or docs.
- User Env Vault stores raw values only in the OS credential store. If unavailable, fail closed.
- OS credential keys for User Env Vault must be namespaced by Local App Database identity.
- SQLite may store Env Vault metadata, fingerprints, approvals, prompt suppressions, and use records only.
- Vault-backed Env Draft values must stay masked in draft JSON and frontend state; resolve only at save time.
- Env Default Catalog rules are data-only. They must not run commands or bypass approval gates.
- AI Env Review must never send raw secrets, vault values, full `.env` files, or full source files by default.
- Source Fix Suggestions are informational; do not auto-edit source for that flow.
- Unknown public repos should be tested in disposable machine or VM.

## Docs And Git

- README is for humans. Keep it short and useful.
- AGENTS.md is for agents. Keep exact commands and project rules.
- REVIEWERS.md is for review work.
- Update `CONTEXT.md` when Env Draft rules change.
- Update ADR only for hard-to-reverse architectural choices.
- Commit messages should be short caveman style, state what changed, and not mention tools/agents unless user asks.
