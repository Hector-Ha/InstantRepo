import { expect, test } from "bun:test";
import {
  applyRawTargetContent,
  canBuildEnvDraftFromPlan,
  clearVaultBinding,
  renderRawTarget,
  updateEnvDraftValue,
} from "./src/envDraft";
import type { AnalyzeSnapshot, EnvDraft } from "./src/types";

function twoTargetDraft(): EnvDraft {
  return {
    repoPath: "C:\\Repos\\app",
    targets: [
      {
        relativePath: "api\\.env",
        absolutePath: "C:\\Repos\\app\\api\\.env",
        originalContent: "",
        values: [
          {
            name: "API_URL",
            value: "http://localhost:8080",
            secret: false,
            confidence: 0.9,
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
            name: "API_URL",
            value: "http://localhost:5173",
            secret: false,
            confidence: 0.9,
            provenance: { source: "allocator" },
          },
        ],
      },
    ],
  };
}

test("structured draft edit is scoped by target file and variable name", () => {
  const edited = updateEnvDraftValue(
    twoTargetDraft(),
    "web\\.env",
    "API_URL",
    "http://localhost:3000",
  );

  expect(edited.targets[0].values[0].value).toBe("http://localhost:8080");
  expect(edited.targets[1].values[0].value).toBe("http://localhost:3000");
});

test("raw target edits update only the selected target", () => {
	const edited = applyRawTargetContent(
		twoTargetDraft(),
		"web\\.env",
    "API_URL=http://localhost:3000\n",
  );

  expect(edited.targets[0].values[0].value).toBe("http://localhost:8080");
	expect(edited.targets[1].values[0].value).toBe("http://localhost:3000");
});

test("raw target edits keep new assignments", () => {
  const edited = applyRawTargetContent(
    twoTargetDraft(),
    "web\\.env",
    "API_URL=http://localhost:3000\nGREETING=hello\n",
  );

  expect(edited.targets[1].values.map((value) => value.name)).toContain(
    "GREETING",
  );
  expect(
    edited.targets[1].values.find((value) => value.name === "GREETING")?.value,
  ).toBe("hello");
});

test("raw target edits parse quoted values as env values", () => {
  const edited = applyRawTargetContent(
    twoTargetDraft(),
    "web\\.env",
    'API_URL="http://localhost:3000/hello world"\n',
  );

  expect(edited.targets[1].values[0].value).toBe(
    "http://localhost:3000/hello world",
  );
  expect(renderRawTarget(edited.targets[1])).toContain(
    "API_URL=http://localhost:3000/hello world",
  );
});

test("vault bindings render masked and can be cleared for manual entry", () => {
	const draft = twoTargetDraft();
	draft.targets[0].values[0] = {
    ...draft.targets[0].values[0],
    value: "",
    vaultBinding: { fingerprint: "abc123ff", displayName: "OpenAI dev key" },
  };

  expect(renderRawTarget(draft.targets[0])).toContain(
    "API_URL=<vault:abc123ff>",
  );

  const cleared = clearVaultBinding(draft, "api\\.env", "API_URL");
  expect(cleared.targets[0].values[0].vaultBinding).toBeUndefined();
  expect(cleared.targets[0].values[0].value).toBe("");
});

test("deleting a raw vault line clears the vault binding", () => {
  const draft = twoTargetDraft();
  draft.targets[0].values[0] = {
    ...draft.targets[0].values[0],
    value: "",
    vaultBinding: { fingerprint: "abc123ff", displayName: "OpenAI dev key" },
  };

  const edited = applyRawTargetContent(draft, "api\\.env", "\n");

  expect(edited.targets[0].values[0].vaultBinding).toBeUndefined();
  expect(edited.targets[0].values[0].value).toBe("");
});

test("env draft generation is skipped without writable target", () => {
  const snapshot = {
    plan: {
      env: {
        variables: [
          {
            name: "OPENAI_API_KEY",
            currentStatus: "missing",
            fillStrategy: "user_required",
          },
        ],
      },
    },
  } as AnalyzeSnapshot;

  expect(canBuildEnvDraftFromPlan(snapshot)).toBe(false);
});
