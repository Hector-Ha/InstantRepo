# AGENTS.md

Me agent. Me follow this file. Keep talk caveman: short words, clear words, no fancy fluff.

## Project

InstantRepo be Go + Wails app. It helps user set up local repos.

- Backend engine: Go in `internal`.
- CLI/API entry: `cmd/instantrepo`.
- Desktop app: Wails in `cmd/instantrepo-wails`.
- Frontend: React + Vite + TypeScript in `cmd/instantrepo-wails/frontend`.
- Local metadata store: SQLite in `internal/store`.
- Tests: Go tests beside code, plus manual plan in `test/TEST_PLAN.md`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `Hector-Ha/InstantRepo`. See `docs/agents/issue-tracker.md`.

### Triage labels

Use default triage labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context repo: root `CONTEXT.md`, future ADRs in `docs/adr/`. See `docs/agents/domain.md`.

### Skill workflow

- Use `triage` when changing issue state or preparing an issue for human/agent work.
- Use `to-prd` only when turning a new discussion into a PRD issue.
- Use `to-issues` only after PRD/plan is clear enough for implementation slices.
- Use `tdd` when implementing issue work or fixing a bug.
- Use `handoff` before stopping long work that another agent must continue.
- Use `caveman-commit` for commit messages.

## Active Env Draft Track

Env Draft foundation is main current roadmap. Read `CONTEXT.md` and `docs/adr/0002-use-catalog-driven-env-drafts.md` before touching it.

Parent PRD:

- `#15` Catalog-driven Env Drafts and Vault-backed Credentials.

Implementation order:

1. `#16` Build structured Env Draft model with provenance and safe Save All. Done.
2. `#17` Infer local env targets from env files and code usage. Done.
3. `#18` Apply Env Default Catalog rules for secrets, credentials, and dev defaults. Done.
4. `#19` Detect App Topology and allocate coherent local dev values. Done.
5. `#20` Ship structured Env Draft UI with grouped targets and raw vault tags. Done.
6. `#21` Add User Env Vault backend with OS credential storage and approvals. Backend is in master; verify issue and close or finish gaps before #22.
7. `#22` Build Env Vault Manager for credentials, usage, and action-needed states.
8. `#23` Add Env Pattern Contribution settings, public filtering, and offline queue.
9. `#24` Add AI Env Review Bundle and Env Patch validation.

Do not skip dependencies unless maintainer says so. Each issue is meant as vertical slice, but issue `#22`, `#23`, and `#24` depend on earlier domain/backend foundation.

Architecture cleanup:

- `#25` PRD: Deepen setup architecture after Env Draft foundation. Closed.
- `#26` Deepen setup safety scan with ignored generated folders. Closed.
- `#27` Deepen Env Draft foundation interfaces for target inference and save policy. Closed.

These cleanup issues support #21-#24. They do not replace the Env Draft roadmap.

Current important Env modules:

- `internal/analyzer/env_target_inference.go` owns Env Target Inference orchestration.
- `internal/service/envdraft_save.go` owns Env Draft save policy.
- `internal/service/envvault.go` owns User Env Vault backend behavior.
- `internal/service/credential_store_windows.go` owns Windows credential-store access.
- `internal/store/sqlite.go` stores Local App Database metadata, never raw vault values.
- `cmd/instantrepo-wails/frontend/src/EnvDraftPanel.tsx` owns current structured Env Draft UI.

## Package Manager

- Bun good. Use `bun`.
- Use `pnpm` only if no Bun path.
- Use `npm` last.
- Use `bunx` before `npx`.

## Setup

Install frontend deps:

```bash
cd cmd/instantrepo-wails/frontend
bun install
```

Download Go deps:

```bash
go mod download
```

## Dev Commands

Run desktop app:

```bash
cd cmd/instantrepo-wails
wails dev
```

Run CLI analyze:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo
```

Run API:

```bash
go run ./cmd/instantrepo -serve :8080
```

Build desktop app:

```bash
cd cmd/instantrepo-wails
wails build -clean
```

Build frontend only:

```bash
cd cmd/instantrepo-wails/frontend
bun run build
```

## Test

Run all Go tests:

```bash
go test ./...
```

Run one package:

```bash
go test ./internal/service
```

Run one test:

```bash
go test ./internal/service -run TestName
```

Before done, run `go test ./...` when Go code changed. Run `bun run build` when frontend code changed.
Frontend tests use Bun test directly:

```bash
cd cmd/instantrepo-wails/frontend
bun test
```

Run `wails build -clean` when Wails bindings or exposed app methods change.

## Code Style

- Keep Go code simple. Use `gofmt`.
- Keep errors wrapped with useful context.
- Keep domain structs in `internal/domain`.
- Keep analyzer-only logic in `internal/analyzer`.
- Keep execution and planning in `internal/service`.
- Do not mix UI code with service engine code.
- Do not commit build outputs unless user ask.
- Do not edit generated Wails files by hand unless generation is broken.

## React Rules

Frontend is React app. Use good React ways:

- Keep derived values in render or memo, not effect.
- Use primitive effect deps where possible.
- Use functional state updates for state based on old state.
- Avoid heavy work during render; memoize real expensive work.
- Import direct modules. Avoid big barrel imports.
- Keep global listeners deduped and cleaned up.

## Safety

This app inspects and can run code from unknown repos. Be careful.

- Never auto-run repo commands without explicit user request or approval path.
- Keep approval gates for risky steps.
- Treat scripts, installers, binaries, and shell files as risky evidence.
- Preserve existing `.env` values. Never print secrets.
- Do not write real secret values into docs or tests.
- Do not store service credential values in SQLite, logs, issues, PRs, diagnostics, or docs.
- User Env Vault stores raw values only in the OS credential store. If it is unavailable, fail closed; do not add plaintext fallback.
- SQLite may store Env Vault metadata, fingerprints, approvals, prompt suppressions, and use records only.
- Vault-backed Env Draft values must stay masked in draft JSON and frontend state; resolve values only at save time.
- Env Default Catalog rules are data-only. Do not add rule behavior that runs commands or bypasses approval gates.
- AI Env Review must use bounded context and structured Env Patch; never send raw secrets, vault values, full `.env` files, or full source files by default.
- Source Fix Suggestions for env loader paths are informational in foundation; do not auto-edit source for that flow.
- Unknown public repos should be tested in disposable machine or VM.

## Docs

- README is for humans. Keep it short and useful.
- AGENTS.md is for agents. Keep exact commands and project rules.
- Use caveman talk in project docs unless user asks otherwise.
- When Env Draft rules change, update `CONTEXT.md`; update ADR only for hard-to-reverse architectural choices.

## PR / Commit

- Commit message should be short caveman style.
- State what changed.
- Do not mention tools or agent unless user asks.
