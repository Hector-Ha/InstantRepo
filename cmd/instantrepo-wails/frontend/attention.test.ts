import { expect, test } from "bun:test";
import {
  envRequirementKey,
  envRequirementTargetLabel,
  getMissingRequiredTools,
  getSafetyAttention,
  getUnresolvedEnv,
} from "./src/attention";
import type { AnalyzeSnapshot, EnvVarRequirement } from "./src/types";

test("missing tool attention ignores unavailable tools not required by plan", () => {
  const snapshot = {
    environment: {
      tools: [
        { name: "bun", available: true },
        { name: "pnpm", available: false },
      ],
    },
    plan: {
      gaps: [
        {
          tool: "node",
          requiredVersion: "unspecified",
          installedVersion: "v23.6.0",
          status: "satisfied",
        },
        {
          tool: "bun",
          requiredVersion: "",
          installedVersion: "1.3.3",
          status: "satisfied",
        },
      ],
    },
  } as AnalyzeSnapshot;

  expect(getMissingRequiredTools(snapshot)).toEqual([]);
});

test("missing tool attention keeps required missing plan gaps", () => {
  const snapshot = {
    environment: {
      tools: [{ name: "docker", available: false }],
    },
    plan: {
      gaps: [
        {
          tool: "docker",
          requiredVersion: "",
          status: "missing",
        },
      ],
    },
  } as AnalyzeSnapshot;

  expect(getMissingRequiredTools(snapshot).map((gap) => gap.tool)).toEqual([
    "docker",
  ]);
});

test("missing tool attention keeps version mismatches", () => {
  const snapshot = {
    plan: {
      gaps: [
        {
          tool: "node",
          requiredVersion: ">=20",
          installedVersion: "18.19.0",
          status: "version_mismatch",
        },
      ],
    },
  } as AnalyzeSnapshot;

  expect(getMissingRequiredTools(snapshot).map((gap) => gap.tool)).toEqual([
    "node",
  ]);
});

test("safety attention exposes setup script findings", () => {
  const snapshot = {
    plan: {
      safety: {
        riskLevel: "medium",
        findings: [
          {
            severity: "medium",
            summary: "Shell script present",
            filePath: "C:\\Repos\\AltShift\\setup.sh",
          },
        ],
      },
    },
  } as AnalyzeSnapshot;

  expect(getSafetyAttention(snapshot)).toEqual([
    {
      severity: "medium",
      summary: "Shell script present",
      filePath: "C:\\Repos\\AltShift\\setup.sh",
    },
  ]);
});

test("env attention uses fill strategy for required values", () => {
  const snapshot = {
    plan: {
      env: {
        variables: [
          {
            name: "SHARED_SECRET",
            currentStatus: "missing",
            fillStrategy: "user_required",
          },
          {
            name: "DATABASE_URL",
            currentStatus: "missing",
            fillStrategy: "auto_fillable",
          },
          {
            name: "OPENAI_API_KEY",
            currentStatus: "configured",
            fillStrategy: "user_required",
          },
        ],
      },
    },
  } as AnalyzeSnapshot;

  expect(getUnresolvedEnv(snapshot).map((item) => item.name)).toEqual([
    "SHARED_SECRET",
  ]);
});

test("env requirement identity includes target dir", () => {
  const repoPath = "C:\\Repos\\multi";
  const apiVar: EnvVarRequirement = {
    name: "SHARED_SECRET",
    source: "code scan",
    required: true,
    secret: true,
    currentStatus: "missing",
    fillStrategy: "user_required",
    targetDir: "C:\\Repos\\multi\\api",
  };
  const workerVar: EnvVarRequirement = {
    ...apiVar,
    targetDir: "C:\\Repos\\multi\\worker",
  };

  expect(envRequirementKey(apiVar)).not.toBe(envRequirementKey(workerVar));
  expect(envRequirementTargetLabel(apiVar, repoPath)).toBe("api");
  expect(envRequirementTargetLabel(workerVar, repoPath)).toBe("worker");
});
