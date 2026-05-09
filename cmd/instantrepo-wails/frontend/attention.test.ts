import { expect, test } from "bun:test";
import { getMissingRequiredTools, getSafetyAttention } from "./src/attention";
import type { AnalyzeSnapshot } from "./src/types";

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
