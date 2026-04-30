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
- Draft or update `.env` with safe defaults, while keeping existing values.
- Detect Docker Compose services like Postgres, MongoDB, Redis, and MySQL.
- Flag scripts, installers, and binaries before run.
- Classify setup steps as `required`, `recommended`, `optional`, `manual`, or `uncertain`.
- Execute one approved step at a time.

## Project Shape

```text
cmd/instantrepo             Go CLI and API entrypoint
cmd/instantrepo-wails       Wails desktop app backend
cmd/instantrepo-wails/frontend
                            React + Vite UI, built with Bun
internal/analyzer           Repo, README, runtime, env, and service detection
internal/service            Planning, execution, env writing, repo clone flow
internal/api                HTTP endpoints
internal/domain             Shared response and plan types
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

Manual MVP test plan live in `test/TEST_PLAN.md`. Repo tracking sheet live in `test/repo-matrix.csv`.

## Trust Model

InstantRepo trust stronger evidence first:

1. Lockfiles and config.
2. Manifests and runtime files.
3. Env templates and Docker Compose files.
4. `README.md` as support.
5. Guessing.

README commands can help, but do not beat manifest-backed commands.

## Next Work

- Stream live logs while step runs.
- Improve version match logic.
- Add monorepo support.
- Add more manifests and package managers.
- Package desktop app for Windows and later macOS.
- Add optional reputation scan for risky files.
