# InstantRepo

InstantRepo is an early MVP for a local repo setup agent. It analyzes a GitHub repository or local folder, detects common project requirements, performs a lightweight safety scan, inspects the local machine, and returns a structured setup plan.

## Desktop App

The Windows UI is now the Wails app under `cmd/instantrepo-wails`.

Frontend setup:

```bash
cd cmd/instantrepo-wails/frontend
bun install
```

Run the app in development:

```bash
cd cmd/instantrepo-wails
wails dev
```

Build a Windows executable:

```bash
cd cmd/instantrepo-wails
wails build -clean
```

With the current `wails.json`, the build output is named `InstantRepo.exe`.

## Current MVP

- Analyze a GitHub repo URL by shallow-cloning it locally
- Analyze a local repo path directly
- Detect basic Node.js, Python, and Go project signals
- Detect common local tools like `git`, `node`, `npm`, `python`, and `docker`
- Parse `README.md` as a secondary evidence source for install, run, env, and service commands
- Detect `.env` templates and classify variables as auto-fillable or user-required
- Detect Docker Compose-backed local services like Postgres, MongoDB, Redis, and MySQL
- Generate or update `.env` files with safe local defaults and placeholders for unresolved secrets
- Attach provider-specific instructions for values that must come from services like OpenAI or MongoDB Atlas
- Flag suspicious files like scripts and installers before execution
- Return a JSON plan with requirements, gaps, and classified steps
- Execute a selected plan step locally with an approval gate
- Offer a Wails-based Windows desktop workflow on top of the engine

## Trust Model

InstantRepo now uses a file-first trust model:

1. lockfiles and explicit config
2. manifests and runtime config
3. env templates and Docker Compose files
4. `README.md` as supporting evidence
5. heuristics

README commands are parsed and surfaced, but they do not override manifest-backed commands. Steps are classified as:

- `required`
- `recommended`
- `optional`
- `manual`
- `uncertain`

## Run as CLI

```bash
go run ./cmd/instantrepo -repo https://github.com/user/repo
```

Or analyze a local path:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo
```

Execute a planned step locally:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo -step install-node-deps -approve
```

Prepare the repo `.env` file:

```bash
go run ./cmd/instantrepo -path C:\path\to\repo -step create-env-file -approve
```

## Run as API

```bash
go run ./cmd/instantrepo -serve :8080
```

Then call:

```bash
curl -X POST http://localhost:8080/analyze ^
  -H "Content-Type: application/json" ^
  -d "{\"repoUrl\":\"https://github.com/user/repo\"}"
```

Execute a specific step:

```bash
curl -X POST http://localhost:8080/execute ^
  -H "Content-Type: application/json" ^
  -d "{\"localPath\":\"C:\\path\\to\\repo\",\"stepId\":\"install-node-deps\",\"approveRisky\":true}"
```

## Next Steps

- Stream live logs instead of returning only final stdout/stderr
- Improve version matching
- Expand manifest support and monorepo detection
- Add packaging for the Wails desktop app and later macOS desktop packaging
- Add provider-specific integrations if we ever want real account creation or authenticated secret retrieval
- Add VirusTotal or similar reputation checks as an optional remote scan layer
