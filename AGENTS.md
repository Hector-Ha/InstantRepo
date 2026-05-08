# AGENTS.md

Me agent. Me follow this file. Keep talk caveman: short words, clear words, no fancy fluff.

## Project

InstantRepo be Go + Wails app. It helps user set up local repos.

- Backend engine: Go in `internal`.
- CLI/API entry: `cmd/instantrepo`.
- Desktop app: Wails in `cmd/instantrepo-wails`.
- Frontend: React + Vite + TypeScript in `cmd/instantrepo-wails/frontend`.
- Tests: Go tests beside code, plus manual plan in `test/TEST_PLAN.md`.

## Agent skills

### Issue tracker

Issues live in GitHub Issues for `Hector-Ha/InstantRepo`. See `docs/agents/issue-tracker.md`.

### Triage labels

Use default triage labels: `needs-triage`, `needs-info`, `ready-for-agent`, `ready-for-human`, `wontfix`. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context repo: root `CONTEXT.md`, future ADRs in `docs/adr/`. See `docs/agents/domain.md`.

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
- Unknown public repos should be tested in disposable machine or VM.

## Docs

- README is for humans. Keep it short and useful.
- AGENTS.md is for agents. Keep exact commands and project rules.
- Use caveman talk in project docs unless user asks otherwise.

## PR / Commit

- Commit message should be short caveman style.
- State what changed.
- Do not mention tools or agent unless user asks.
