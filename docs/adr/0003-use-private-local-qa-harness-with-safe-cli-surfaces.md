# Use private local QA harness with safe CLI surfaces

InstantRepo will keep automated QA orchestration private/local instead of committing a QA runner, QA bridge, scenario pack, or evidence tooling to the public production repository. The private harness may live under a gitignored `.qa-local` folder or a private QA repo, drive the real frontend through a Wails-style browser shim when useful, and call production-safe CLI commands that mirror app operations.

The public app may grow CLI subcommands, JSON output, app-data isolation, and stable diagnostics when those surfaces are safe for users and agents. It must not add a QA-only backdoor, hidden debug bridge, approval bypass, raw shell endpoint, or shipped QA overlay. QA evidence, reports, scenario packs, and security-sensitive harness logic stay local/private; GitHub issues may receive credential-free summaries only after tester direction.
