import { expect, test } from "bun:test";
import {
  planClonePreflightFlow,
  summarizePreflightMessages,
  type ClonePreflightResponse,
} from "./src/clonePreflight";

const basePreflight: ClonePreflightResponse = {
  repoUrl: "https://github.com/example/app",
  normalizedUrl: "https://github.com/example/app",
  destinationRoot: "C:\\Repos",
  destinationWritable: true,
  targetPath: "C:\\Repos\\app",
  targetExists: false,
  targetEmpty: true,
  duplicateRepos: [],
  pathConflict: false,
  pathConflictRepos: [],
  disk: { status: "ok", freeBytes: 1024 },
  recommendedAction: "clone",
  messages: [],
};

test("clone preflight opens duplicate repo by default", () => {
  const plan = planClonePreflightFlow({
    ...basePreflight,
    recommendedAction: "open-existing",
    duplicateRepos: [
      {
        id: 7,
        localPath: "C:\\Repos\\existing-app",
        status: "analyzed",
        createdAt: "2026-05-08T12:00:00Z",
        updatedAt: "2026-05-08T12:00:00Z",
        lastAnalyzedAt: "2026-05-08T12:00:00Z",
      },
    ],
  });

  expect(plan.kind).toBe("open-existing");
  expect(plan.localPath).toBe("C:\\Repos\\existing-app");
});

test("clone preflight can clone another copy when user confirms duplicate", () => {
  const plan = planClonePreflightFlow(
    {
      ...basePreflight,
      recommendedAction: "open-existing",
      duplicateRepos: [
        {
          id: 7,
          localPath: "C:\\Repos\\existing-app",
          status: "analyzed",
          createdAt: "2026-05-08T12:00:00Z",
          updatedAt: "2026-05-08T12:00:00Z",
          lastAnalyzedAt: "2026-05-08T12:00:00Z",
        },
      ],
    },
    { forceClone: true },
  );

  expect(plan.kind).toBe("clone");
});

test("clone preflight asks confirmation for clone with attention", () => {
  const plan = planClonePreflightFlow({
    ...basePreflight,
    recommendedAction: "clone-with-attention",
    messages: [{ severity: "warning", text: "Disk space could not be measured." }],
  });

  expect(plan.kind).toBe("confirm-clone");
  expect(plan.message).toContain("Disk space could not be measured.");
});

test("clone preflight blocks path and disk failures", () => {
  const pathPlan = planClonePreflightFlow({
    ...basePreflight,
    recommendedAction: "choose-different-folder",
    messages: [{ severity: "critical", text: "Target folder is not empty." }],
  });
  const diskPlan = planClonePreflightFlow({
    ...basePreflight,
    recommendedAction: "free-disk-space",
    messages: [{ severity: "critical", text: "Not enough free disk space." }],
  });

  expect(pathPlan.kind).toBe("block");
  expect(pathPlan.message).toContain("Target folder is not empty.");
  expect(diskPlan.kind).toBe("block");
  expect(diskPlan.message).toContain("Not enough free disk space.");
});

test("preflight summary includes fallback target path", () => {
  expect(summarizePreflightMessages({ ...basePreflight, messages: [] })).toContain(
    "C:\\Repos\\app",
  );
});
