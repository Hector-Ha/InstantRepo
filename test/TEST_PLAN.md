# InstantRepo Test Plan

## Goal

Validate that InstantRepo can:

- clone a repository into a user-selected folder
- detect stack, tools, env files, services, and setup commands
- classify commands as `required`, `recommended`, `optional`, `manual`, or `uncertain`
- generate or update `.env`
- guide the user through unresolved external secrets
- execute safe setup steps in a usable order

## Test Strategy

Do not start with random public repos.

Use staged testing:

1. Local fixture repos you control
2. Trusted public repos from major orgs or your own repos
3. Curated community repos with clear docs
4. Unknown public repos only after manual inspection

The app executes local commands. Treat unknown repos as untrusted code.

## Safety Rules

- Use a disposable VM or secondary machine for unknown repos.
- Review detected steps before running them.
- Do not run `manual` or clearly suspicious steps automatically.
- Stop testing a repo if the app surfaces `high` safety findings unless you intentionally want to inspect that case.
- Prefer repos with permissive licenses and clear setup docs.

## Test Environments

Minimum environments to cover:

- Windows 11 with Node, Python, Git, Docker available
- Windows 11 with some dependencies intentionally missing

Optional later:

- macOS
- clean Windows VM with almost nothing installed

## Test Tiers

### Tier 0: Controlled Fixtures

Purpose:

- verify analyzer logic deterministically
- verify `.env` generation
- verify command classification

Repo types:

- simple Node app
- Node app with README-only extra command
- Python app with `requirements.txt`
- Go app
- app with `.env.example`
- app with Docker Compose and Postgres
- app with external secret only
- intentionally broken repo

### Tier 1: Trusted Public Repos

Purpose:

- validate real-world setup against known-good repositories

Sources:

- your own repos
- repos from major orgs
- small starter repos with clean docs

### Tier 2: Curated Community Repos

Purpose:

- broaden coverage without going fully random

Requirements:

- clear README
- active maintenance
- obvious setup path

### Tier 3: Unknown Public Repos

Purpose:

- stress test safety and ambiguity handling

Requirements:

- manual pre-review first
- run in disposable environment

## Scenario Matrix

Each candidate repo should map to at least one scenario:

1. Node app with `package.json`
2. Node app with `pnpm`
3. Node app where README adds commands not present in manifests
4. Python app with `requirements.txt`
5. Python app with `pyproject.toml`
6. Go app
7. Repo with `.env.example`
8. Repo with Docker Compose
9. Repo with local Postgres
10. Repo with external secrets only
11. Monorepo
12. Library or non-runnable repo
13. Broken or stale repo
14. Repo with suspicious scripts or binaries

## Per-Repo Test Flow

For each repo:

1. Record repo metadata in `repo-matrix.csv`.
2. Build and open `InstantRepo.exe`.
3. Paste repo URL.
4. Choose destination folder.
5. Click `Clone & Analyze`.
6. Review `Overview`.
7. Review `Env Setup`.
8. Review `Steps`.
9. Confirm that:
   - stack detection is correct
   - tool requirements are correct
   - service detection is correct
   - `.env` handling is correct
   - command classification is reasonable
10. Generate `.env` draft if applicable.
11. Fill any required external values if you want to continue.
12. Run `required` steps first.
13. Run `recommended` steps if needed.
14. Run `optional` steps only when relevant.
15. Record result and defects.

## Validation Checklist

### Clone and Import

- repo cloned into the selected folder
- repo source type shown correctly
- app does not crash during import

### Stack Detection

- project type is correct
- package manager is correct
- version hints are detected correctly

### Tool Detection

- missing tools are detected correctly
- installed tools are not falsely marked missing
- install suggestions match OS

### README Handling

- README-backed commands are extracted from relevant sections
- manifest-backed commands remain primary
- README-only commands are shown as lower-trust candidates
- confirmation sources are displayed correctly

### Command Classification

- install steps are usually `required`
- primary run command is usually `recommended`
- build/test-only commands are usually `optional`
- unresolved user actions remain `manual`
- unclear commands remain `uncertain`

### Env Handling

- `.env.example` or similar is detected
- `.env` draft is created or updated
- existing values are preserved
- safe local defaults are filled when appropriate
- external secrets remain unresolved
- external secret instructions are clear

### Service Handling

- Docker Compose is detected when present
- local Postgres/Redis/Mongo/MySQL services are detected when present
- service startup steps appear in the plan

### Execution

- steps run in a sensible order
- logs are understandable
- execution result status is correct
- failures are surfaced clearly

### Safety

- suspicious files are flagged
- manual review steps are not auto-run
- high-risk repo behavior is visible before execution

## Result Labels

Use one of these per repo:

- `PASS`
- `PASS_WITH_GAPS`
- `FAIL_ANALYSIS`
- `FAIL_ENV`
- `FAIL_EXECUTION`
- `FAIL_SAFETY`
- `BLOCKED_EXTERNAL_SECRET`

## Exit Criteria For MVP Validation

Suggested minimum target:

- 15 to 20 curated repos tested
- all Tier 0 fixture repos pass
- at least:
  - 3 Node repos
  - 3 Python repos
  - 2 Go repos
  - 3 repos with `.env`
  - 3 repos with Docker Compose
  - 3 repos with external secrets
- no desktop app crash during import/analyze flow
- no false auto-execution of `manual` steps

## Defect Reporting Format

For each defect, record:

- repo URL
- scenario
- exact step where behavior diverged
- expected result
- actual result
- logs
- whether issue is analyzer, planner, env, UI, or executor

## Recommended Initial Order

1. Run all Tier 0 fixtures.
2. Run 5 trusted public repos.
3. Run env-heavy and Docker-heavy repos.
4. Run 2 to 3 broken/stale repos intentionally.
5. Move to curated community repos.
6. Only then sample unknown public repos.
