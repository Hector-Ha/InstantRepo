# REVIEWERS.md

Use this file when the user asks to review code, review a diff, review a PR, inspect a branch, or use any review workflow. For review tasks, this file overrides `AGENTS.md` on stance and output shape.

## Review Stance

- Lead with findings. Sort by severity.
- Use file and line references for every finding.
- Focus on bugs, regressions, security risks, data loss, broken contracts, missing tests, and unsafe behavior.
- Do not praise, summarize first, or spend space on style nits unless they hide a real bug.
- If no issues found, say that clearly and name residual risk or test gaps.
- Do not edit code during review unless user asks for fixes.
- Do not commit during review unless user asks.
- Use `caveman-review` for terse review comments.

## Review Output

Preferred shape:

```text
Findings
- P1 path/file.go:123 - Problem. Impact. Fix.
- P2 path/file.tsx:45 - Problem. Impact. Fix.

Open Questions
- ...

Test Gaps
- ...
```

Severity:

- `P0`: blocks release, data loss, secret leak, command bypass.
- `P1`: likely user-visible break or major safety/security issue.
- `P2`: edge-case bug, missing guard, important test gap.
- `P3`: maintainability issue with concrete future cost.

## Scope Prep

Before reviewing:

- Check `git status --short --branch`.
- Identify base and changed files: `git diff --stat`, `git diff --name-status`, `git diff`.
- If reviewing a PR/issue, consult GitHub and linked PRD/issues.
- Read `CONTEXT.md` and relevant ADRs for domain-sensitive changes.
- Read `docs/cli-mirror.md` for CLI mirror contract changes.
- Do not inspect or quote `.qa-local/` unless tester asks for private QA review.

Useful commands:

```bash
git status --short --branch
git diff --stat
git diff --name-status
git diff
gh issue view <number> --comments
gh pr view <number> --comments --json title,body,files,commits,reviews
go test ./...
cd cmd/instantrepo-wails/frontend
bun test
bun run build
```

## Project Map

- Backend service: `internal/service`.
- Analyzer and env target inference: `internal/analyzer`.
- Domain types and JSON contracts: `internal/domain`, `internal/contract`.
- SQLite local metadata: `internal/store`.
- CLI mirror and app-data policy: `internal/command`.
- Wails backend surface: `cmd/instantrepo-wails/app.go`.
- React frontend: `cmd/instantrepo-wails/frontend/src`.
- Env Draft UI: `cmd/instantrepo-wails/frontend/src/EnvDraftPanel.tsx`.

## Review Checklists

General:

- Does behavior match issue/PRD acceptance criteria?
- Are errors wrapped with useful context?
- Are tests at the right layer and narrow enough to fail for the bug?
- Did generated files/build outputs get committed by mistake?
- Did public docs expose private QA internals?

Safety:

- No auto-run of repo commands without explicit request or approval.
- Risky setup steps keep approval gates.
- Scripts, installers, binaries, and shell files stay risky evidence.
- Unknown repo behavior never becomes trusted input.

Secrets:

- No raw secrets in logs, docs, tests, diagnostics, issues, PRs, SQLite, or frontend state.
- User Env Vault raw values stay only in OS credential store.
- Vault values stay masked until save/reveal flow explicitly resolves them.
- Isolated app-data credential keys stay scoped by Local App Database identity.

Env Draft:

- Existing `.env` values are preserved.
- Service Credentials stay blank or vault-backed only after approval.
- Generated Local Secrets are local-only, not vault values.
- Dev Defaults have provenance and confidence.
- Env Target Inference writes only local/dev targets, not production env files.
- AI Env Review uses bounded context and structured Env Patch only.

CLI Mirror:

- Subcommands keep `--json` success envelope and structured error shape.
- Nonzero exits return JSON errors when `--json` is set.
- Metadata-only commands validate app data dirs without creating Local App Database dirs.
- `OpenDirectory()` remains desktop-only; CLI uses explicit path args.
- Secret-capable commands mask by default and require explicit reveal.

Frontend:

- Derived values stay in render or memo, not effect.
- Effects use primitive deps where possible.
- State based on old state uses functional updates.
- Global listeners are deduped and cleaned up.
- Bridge contract mismatch shows a clear unavailable/outdated state.

Private QA Boundary:

- `.qa-local/` remains ignored and untracked.
- Public app has no QA-only backdoor, raw shell endpoint, hidden debug bridge, approval bypass, or shipped QA overlay.
- Public issues/docs stay credential-free and omit private harness details.

## Verification Expectations

- For Go changes, prefer `go test ./...`.
- For frontend changes, run `bun test` and `bun run build`.
- For Wails exposed methods or bindings, run `wails build -clean` when feasible.
- For private QA boundary changes, run:

```bash
go test ./internal/command -run TestPrivateQALocalWorkspaceRemainsIgnored
git check-ignore .qa-local/
```

State any command not run and why.
