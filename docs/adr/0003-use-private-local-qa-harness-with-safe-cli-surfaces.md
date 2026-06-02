# Use private local QA harness with safe CLI surfaces

InstantRepo will keep automated QA orchestration private/local instead of committing a QA runner, QA bridge, scenario pack, or evidence tooling to the public production repository. Private/local QA may exercise real product surfaces and call production-safe CLI commands that mirror app operations, but public docs and public code must stay policy-focused and omit private harness mechanics.

The public app exposes CLI subcommands, JSON output, app-data isolation, stable diagnostics, Env Vault operations, settings, and shell/version metadata when those surfaces are safe for users and agents. It must not add a QA-only backdoor, hidden debug bridge, approval bypass, raw shell endpoint, or shipped QA overlay. QA evidence, reports, scenario packs, and security-sensitive harness logic stay local/private; GitHub issues may receive credential-free summaries only after tester direction.

When private QA or CLI smoke work uses isolated app data, public vault commands still use the real production credential-store behavior. Credential target keys must be scoped by Local App Database identity so isolated app data cannot collide with default app data even when both SQLite databases assign the same Env Vault entry IDs.

Public docs for the CLI mirror live in `docs/cli-mirror.md`. They may describe command names, `--json`, structured errors, app-data isolation, version metadata, and the local/private QA boundary, but they must not document private harness internals. `.qa-local/` must stay ignored and no user release artifact may include private QA files, overlays, reports, screenshots, evidence, or shim code. `OpenDirectory()` remains a desktop dialog; CLI uses explicit path arguments instead.
