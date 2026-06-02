# CLI Mirror

InstantRepo CLI mirror is public product surface. It lets users, devs, and agents run the same safe app operations without Wails. It is not a private QA bridge.

## Contract

CLI mirror subcommands print human text by default. Add `--json` when another tool must read output.

Legacy root flags such as `-repo`, `-path`, `-step`, and `-serve` are compatibility entrypoints. They do not promise the stable success envelope below; integrations should use mirror subcommands.

JSON success uses one envelope:

```json
{
  "ok": true,
  "data": {},
  "metadata": {
    "appVersion": "dev",
    "gitCommit": "",
    "cliContractVersion": "2026-05-issue-40",
    "bridgeContractVersion": "2026-05-bridge-1"
  }
}
```

JSON errors go to stderr, return non-zero, and use this shape:

```json
{
  "ok": false,
  "error": {
    "code": "missing_target",
    "message": "repo URL or local path is required"
  },
  "metadata": {
    "appVersion": "dev",
    "cliContractVersion": "2026-05-issue-40",
    "bridgeContractVersion": "2026-05-bridge-1"
  }
}
```

Use contract metadata before depending on shape:

```bash
go run ./cmd/instantrepo version --json
go run ./cmd/instantrepo shell info --json
```

## App Data Isolation

Use temp app data when running smoke work, agent checks, or QA-adjacent local work:

```bash
go run ./cmd/instantrepo --app-data-dir C:\temp\instantrepo-app-data repo list --json
```

`INSTANTREPO_APP_DATA_DIR` also works for CLI and Wails launches. The path must be absolute and must not be home, drive root, repo root, target repo, or inside target repo. Bad overrides fail closed.

Commands that open app state prepare the app data directory before use. Metadata-only commands such as `version` and `shell info` validate the app data path but do not create the directory.

Env Vault still uses production credential-store behavior. Credential keys are scoped by Local App Database identity, so isolated app data cannot overwrite, reveal, or delete default app-data vault values by reusing row IDs.

## Command Map

Repository setup:

```bash
go run ./cmd/instantrepo repo analyze --path C:\path\to\repo --json
go run ./cmd/instantrepo repo analyze --repo https://github.com/user/repo --json
go run ./cmd/instantrepo repo preflight --repo https://github.com/user/repo --destination C:\work --json
go run ./cmd/instantrepo repo import --repo https://github.com/user/repo --destination C:\work --json
go run ./cmd/instantrepo repo clone --repo https://github.com/user/repo --destination C:\work --json
go run ./cmd/instantrepo repo execute --path C:\path\to\repo --step install-node-deps --approve --json
```

Installed repo history and diagnostics:

```bash
go run ./cmd/instantrepo repo list --json
go run ./cmd/instantrepo repo details --id 123 --json
go run ./cmd/instantrepo repo diagnostics --path C:\path\to\repo --json
go run ./cmd/instantrepo repo diagnostics --id 123 --json
```

Env Draft:

```bash
go run ./cmd/instantrepo env draft generate --path C:\path\to\repo --json
go run ./cmd/instantrepo env draft save --path C:\path\to\repo --file C:\path\to\draft.json --json
go run ./cmd/instantrepo env raw save --path C:\path\to\repo --file C:\path\to\.env --json
```

Env Vault:

```bash
go run ./cmd/instantrepo env vault list --json
go run ./cmd/instantrepo env vault save --provider openai --variable OPENAI_API_KEY --display-name "OpenAI dev key" --stdin --json
go run ./cmd/instantrepo env vault update --id 123 --display-name "OpenAI work key" --json
go run ./cmd/instantrepo env vault update --id 123 --stdin --json
go run ./cmd/instantrepo env vault remove --id 123 --json
go run ./cmd/instantrepo env vault approve --id 123 --repo-path C:\path\to\repo --target .env --variable OPENAI_API_KEY --json
go run ./cmd/instantrepo env vault revoke --approval-id 456 --json
go run ./cmd/instantrepo env vault status --id 123 --status action_needed --json
go run ./cmd/instantrepo env vault suppress --repo-path C:\path\to\repo --target .env --variable OPENAI_API_KEY --json
go run ./cmd/instantrepo env vault reveal --id 123 --confirm-reveal --json
```

Settings:

```bash
go run ./cmd/instantrepo settings contribution get --json
go run ./cmd/instantrepo settings contribution save --file C:\path\to\settings.json --json
go run ./cmd/instantrepo settings contribution save --stdin --json
go run ./cmd/instantrepo settings contribution consent --public-enabled true --json
go run ./cmd/instantrepo settings contribution clear-queue --json
go run ./cmd/instantrepo settings ai-env-review get --json
go run ./cmd/instantrepo settings ai-env-review save --file C:\path\to\settings.json --json
go run ./cmd/instantrepo settings ai-env-review save --stdin --json
```

## Desktop Dialogs

`OpenDirectory()` is Wails desktop UI only. CLI does not mirror that dialog literally. Use explicit `--path`, `--destination`, or `--repo-path` args instead.

## Private QA Boundary

`.qa-local/` is ignored and may contain private/local docs, scenarios, run reports, screenshots, logs, and evidence. Do not commit it.

Public GitHub issues must stay credential-free and must not include private harness implementation details. If private QA finds a bug, tester first reads the local report, then asks for a public issue summary if needed. That issue summary should name product behavior, reproduction steps, expected vs actual result, and safe evidence paths or redacted excerpts only.

Release/no-ship guard:

- Normal user artifacts must not include `.qa-local/`, private scenario packs, reports, screenshots, evidence, or QA overlays.
- Public app must not expose private QA shim code, raw shell endpoints, hidden debug bridges, or approval bypasses.
- `OpenDirectory()` stays a desktop dialog; path-taking CLI commands stay explicit.
- Keep this guard green:

```bash
go test ./internal/command -run TestPrivateQALocalWorkspaceRemainsIgnored
git check-ignore .qa-local/
```
