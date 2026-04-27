# InstantRepo

InstantRepo is an early MVP for a local repo setup agent. It analyzes a GitHub repository or local folder, detects common project requirements, performs a lightweight safety scan, inspects the local machine, and returns a structured setup plan.

## Desktop App

There is now a Windows desktop entrypoint that wraps the Go engine in a native workflow:

- paste a GitHub or GitLab repo URL
- choose the destination folder on disk
- clone and analyze the repository
- generate or refresh a `.env` draft
- paste external secrets into the built-in env editor
- run install and setup steps from the app

Build the desktop app:

```bash
go build -o InstantRepoDesktop.exe ./cmd/instantrepo-desktop
```

## Current MVP

- Analyze a GitHub repo URL by shallow-cloning it locally
- Analyze a local repo path directly
- Detect basic Node.js, Python, and Go project signals
- Detect common local tools like `git`, `node`, `npm`, `python`, and `docker`
- Detect `.env` templates and classify variables as auto-fillable or user-required
- Detect Docker Compose-backed local services like Postgres, MongoDB, Redis, and MySQL
- Generate or update `.env` files with safe local defaults and placeholders for unresolved secrets
- Attach provider-specific instructions for values that must come from services like OpenAI or MongoDB Atlas
- Flag suspicious files like scripts and installers before execution
- Return a JSON plan with requirements, gaps, and suggested steps
- Execute a selected plan step locally with an approval gate
- Offer a Windows desktop workflow on top of the engine

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
- Add packaging for the Windows desktop app and later macOS desktop packaging
- Add provider-specific integrations if we ever want real account creation or authenticated secret retrieval
- Add VirusTotal or similar reputation checks as an optional remote scan layer
