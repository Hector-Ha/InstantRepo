# Handoff: Env Draft Architecture Improve

## Context

Repo: `C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo`

User said GitHub issues `#16`, `#17`, `#18` are done, then asked for `$improve-codebase-architecture`. Project rules require caveman style, Bun over pnpm/npm, and active Env Draft roadmap awareness. I read `CONTEXT.md`, ADR `docs/adr/0002-use-catalog-driven-env-drafts.md`, and the architecture skill vocabulary.

## What Improved

Main friction found: Env Draft flow had two modules doing env file work.

- `EnvDraftManager` held new foundation behavior: Env Default Catalog, provenance, generated local secrets, multi-target Save All, rollback, retry, and outside-repo target validation.
- Old `EnvFileManager` still powered AppService preview, guarded env setup, value saves, and raw saves.
- Deletion test: deleting `EnvFileManager` did not lose useful depth; it mostly removed duplicate implementation and forced callers through the deeper Env Draft module.

Change made:

- [internal/service/app.go](C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo\internal\service\app.go) now uses `envDrafts *EnvDraftManager`.
- [internal/service/envdraft.go](C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo\internal\service\envdraft.go) now exposes `Preview`, `Prepare`, `ApplyValues`, and `SaveRaw` on top of `BuildDraft` and `SaveAll`.
- [internal/service/envtext.go](C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo\internal\service\envtext.go) keeps only shared env text helpers from old env file path.
- Deleted `internal/service/envfile.go` and `internal/service/envfile_test.go`.
- Added [internal/service/envdraft_prepare_test.go](C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo\internal\service\envdraft_prepare_test.go).
- Added AppService wiring test in [internal/service/app_installed_repo_test.go](C:\Users\Admin\Desktop\Projects\Personal_Projects\InstantRepo\internal\service\app_installed_repo_test.go): guarded env setup now uses catalog-driven draft behavior, generates `JWT_SECRET`, and leaves `OPENAI_API_KEY` blank.

## Why It Matters

Env setup now crosses one seam: `EnvDraftManager`.

Leverage: AppService gets catalog classification, generated local secrets, provenance rules, Save All validation, retry, and rollback through one interface.

Locality: future Env Draft changes should land in one module, not drift between old template rendering and new catalog rendering.

## Verification

Ran:

```powershell
go test ./...
```

Result: pass.

No frontend files changed, so `bun run build` was not run.

## Current Worktree State

Expected changed files:

- `internal/service/app.go`
- `internal/service/app_installed_repo_test.go`
- `internal/service/envdraft.go`
- `internal/service/envdraft_prepare_test.go`
- `internal/service/envtext.go`
- deleted `internal/service/envfile.go`
- deleted `internal/service/envfile_test.go`

## Follow-Up Ideas

1. For issue `#20`, replace raw string `GenerateEnvDraft` UI path with structured `EnvDraft` data so frontend can show grouped targets, provenance, confidence, and attention.
2. Make `SaveRaw` multi-target aware before raw editor supports multi-target output; current raw save still writes one target from `analysis.Env.TargetPath`.
3. `ApplyValues` currently applies by env var name across targets. Future structured UI should include target identity to avoid same-name cross-target edits.
4. Consider a small AppService-facing Env Draft interface only when a second adapter exists. Right now one adapter means concrete `EnvDraftManager` is fine.

## Suggested Skills Next

- `caveman`: repo requires short clear style.
- `tdd`: use if continuing issue work or bug fixes.
- `improve-codebase-architecture`: use for another architecture sweep after more Env Draft slices.
- `vercel-react-best-practices`: use for React work, especially structured Env Draft UI in issue `#20`.
