import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { EnvDraftPanel } from "./src/EnvDraftPanel";
import type { EnvDraft } from "./src/types";

const noop = () => {};

test("EnvDraftPanel renders grouped target files and metadata", () => {
  const draft: EnvDraft = {
    repoPath: "C:\\Repos\\app",
    targets: [
      {
        relativePath: "api\\.env",
        absolutePath: "C:\\Repos\\app\\api\\.env",
        originalContent: "",
        values: [
          {
            name: "DATABASE_URL",
            value: "postgres://localhost/app",
            secret: true,
            confidence: 0.86,
            valueClass: "dev_default",
            attention: ["Check local database port."],
            provenance: { source: "allocator" },
          },
        ],
      },
      {
        relativePath: "web\\.env",
        absolutePath: "C:\\Repos\\app\\web\\.env",
        originalContent: "",
        values: [
          {
            name: "VITE_API_URL",
            value: "http://localhost:8080",
            secret: false,
            confidence: 0.82,
            provenance: { source: "allocator" },
          },
        ],
      },
    ],
  };

  const html = renderToStaticMarkup(
    <EnvDraftPanel
      draft={draft}
      mode="structured"
      selectedRawTarget="api\\.env"
      onModeChange={noop}
      onSelectedRawTargetChange={noop}
      onChange={noop}
    />,
  );

  expect(html).toContain("api\\.env");
  expect(html).toContain("web\\.env");
  expect(html).toContain("DATABASE_URL");
  expect(html).toContain("dev_default · allocator · 86%");
  expect(html).toContain("Check local database port.");
});
