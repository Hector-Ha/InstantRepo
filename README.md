# InstantRepo

InstantRepo be local setup helper. Give it Git repo URL or folder. It clone or read repo, detect stack, check local tools, find env needs, scan for risky files, then make setup plan.

It have three faces:

- Wails desktop app for Windows.
- Go CLI for quick analyze and step run.
- HTTP API for tool use.

> [!CAUTION]
> InstantRepo can run commands from repos. Treat unknown repos as unsafe. Read plan and safety notes before approve run.

## What It Does

- Clone GitHub, GitLab, or other Git URL into chosen folder.
- Analyze local repo folder.
- Detect Node.js, Python, and Go projects.
- Detect local tools like `git`, `node`, `bun`, `npm`, `pnpm`, `python`, `go`, and `docker`.
- Read `README.md` for install, run, env, and service hints.
- Detect `.env` templates and needed secret values.
- Draft or update grouped local `.env` targets with safe defaults, while keeping existing values.
- Store approved service credentials through the operating system credential store for reuse in Env Drafts.
- Detect Docker Compose services like Postgres, MongoDB, Redis, and MySQL.
- Flag scripts, installers, and binaries before run.
- Classify setup steps as `required`, `recommended`, `optional`, `manual`, or `uncertain`.
- Execute one approved step at a time.

## Env Draft Direction

`.env` setup is core product work. Current app can draft or update grouped local env files with safe defaults, topology-aware local URLs and ports, generated local secrets, and vault-backed service credential references while keeping existing values.

Current Env Draft foundation includes:

- App Topology first: detect frontend, backend, workers, databases, caches, and providers before guessing URLs or ports.
- Env Default Catalog: classify dev defaults, generated local secrets, service credentials, and provider config through app-shipped rules.
- Multi-target Save All: handle root, client, server, and weird local `.env*` files in one view.
- User Env Vault backend: store approved service credentials in the OS credential store and keep only metadata, approvals, fingerprints, and use records in the Local App Database.
- Deepened Env Draft internals: target inference and save policy now live behind smaller behavior-tested modules.

Remaining roadmap:

- Env Vault Manager: dedicated UI for saved credentials, approvals, usage, reveal, action-needed state, and removal.
- Env Pattern Contribution: opt-in value-free env-name/context reports for future catalog patches.
- AI Env Review: bounded review of low-confidence non-secret defaults through validated Env Patches, not raw secrets or full repo files.

See `CONTEXT.md` and `docs/adr/0002-use-catalog-driven-env-drafts.md` for the product rules.

## Project Shape

```text
cmd/instantrepo             Go CLI and API entrypoint
cmd/instantrepo-wails       Wails desktop app backend
cmd/instantrepo-wails/frontend
                            React + Vite UI, built with Bun
internal/analyzer           Repo, README, runtime, env, and service detection
internal/service            Planning, execution, env writing, Env Vault, repo clone flow
internal/api                HTTP endpoints
internal/domain             Shared response and plan types
internal/store              SQLite local metadata, setup sessions, Env Vault metadata
test                        Manual MVP test plan and repo matrix
```

## Prereqs

- Go `1.26.2`
- Bun `1.3.3` or newer
- Wails CLI for desktop dev and build
- Git

Install frontend deps:

```bash
cd cmd/instantrepo-wails/frontend
bun install
```

## Desktop App

Run dev app:

```bash
cd cmd/instantrepo-wails
wails dev
```

Build Windows app:

```bash
cd cmd/instantrepo-wails
wails build -clean
```

Output app:

```text
cmd/instantrepo-wails/build/bin/InstantRepo.exe
```

## CLI

Analyze repo URL:

```bash
go run ./cmd/instantrepo -repo https://github.com/user/repo
```

Analyze local path:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo
```

Run one plan step:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo -step install-node-deps -approve
```

Prepare `.env`:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo -step create-env-file -approve
```

## API

Start server:

```bash
go run ./cmd/instantrepo -serve :8080
```

Analyze:

```bash
curl -X POST http://localhost:8080/analyze ^
  -H "Content-Type: application/json" ^
  -d "{\"repoUrl\":\"https://github.com/user/repo\"}"
```

Run step:

```bash
curl -X POST http://localhost:8080/execute ^
  -H "Content-Type: application/json" ^
  -d "{\"localPath\":\"C:\\path\\to\\repo\",\"stepId\":\"install-node-deps\",\"approveRisky\":true}"
```

## Test

Run Go tests:

```bash
go test ./...
```

Build frontend:

```bash
cd cmd/instantrepo-wails/frontend
bun run build
```

Run frontend behavior tests:

```bash
cd cmd/instantrepo-wails/frontend
bun test
```

Manual MVP test plan live in `test/TEST_PLAN.md`. Repo tracking sheet live in `test/repo-matrix.csv`.

## Trust Model

InstantRepo trust stronger evidence first:

1. Lockfiles and config.
2. Manifests and runtime files.
3. Env templates and Docker Compose files.
4. `README.md` as support.
5. Guessing.

README commands can help, but do not beat manifest-backed commands.

## Active Roadmap

Env Draft foundation is tracked in GitHub Issues:

1. [#15 PRD: Catalog-driven Env Drafts and Vault-backed Credentials](https://github.com/Hector-Ha/InstantRepo/issues/15) is the parent track.
2. [#16 Env Draft model + safe Save All](https://github.com/Hector-Ha/InstantRepo/issues/16) is done.
3. [#17 Env target inference](https://github.com/Hector-Ha/InstantRepo/issues/17) is done.
4. [#18 Env Default Catalog](https://github.com/Hector-Ha/InstantRepo/issues/18) is done.
5. [#19 App Topology + allocator](https://github.com/Hector-Ha/InstantRepo/issues/19) is done.
6. [#20 Structured Env UI](https://github.com/Hector-Ha/InstantRepo/issues/20) is done.
7. [#21 User Env Vault backend](https://github.com/Hector-Ha/InstantRepo/issues/21) has backend foundation in master and should be verified before moving to UI work.
8. [#22 Env Vault Manager](https://github.com/Hector-Ha/InstantRepo/issues/22) is next UI work after #21 is accepted.
9. [#23 Env Pattern Contribution](https://github.com/Hector-Ha/InstantRepo/issues/23) follows the vault manager unless maintainer reprioritizes telemetry earlier.
10. [#24 AI Env Review + Env Patch](https://github.com/Hector-Ha/InstantRepo/issues/24) should follow deterministic and vault foundations.

Architecture cleanup also landed after #20:

- [#25 Deepen setup architecture after Env Draft foundation](https://github.com/Hector-Ha/InstantRepo/issues/25)
- [#26 Deepen setup safety scan with ignored generated folders](https://github.com/Hector-Ha/InstantRepo/issues/26)
- [#27 Deepen Env Draft foundation interfaces](https://github.com/Hector-Ha/InstantRepo/issues/27)

## Next Work

- Stream live logs while step runs.
- Improve version match logic.
- Build Env Draft foundation from the active roadmap.
- Add more manifests, package managers, and topology detectors.
- Package desktop app for Windows and later macOS.
- Add optional reputation scan for risky files.
