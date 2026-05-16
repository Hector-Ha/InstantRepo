import { expect, test } from "bun:test";
import {
  bridgeContractOutdatedMessage,
  createDesktopApi,
  desktopSessionUnavailableMessage,
  expectedBridgeContractVersion,
  getDesktopApp,
  missingBridgeContractMessage,
} from "./src/desktopApi";

test("desktop API reports missing Wails bridge with user-facing message", () => {
  expect(() => getDesktopApp({})).toThrow(desktopSessionUnavailableMessage);
});

test("desktop API calls Wails bridge when it exists", async () => {
  const api = createDesktopApi({
    go: {
      main: {
        App: {
          ShellInfo: async () => ({
            bridgeContractVersion: expectedBridgeContractVersion,
          }),
          ListInstalledRepos: async () => ({ repos: [] }),
          GenerateEnvDraft: async () => ({ repoPath: "C:\\repo", targets: [] }),
          ListEnvVaultEntries: async () => ({ entries: [] }),
          RevealEnvVaultEntry: async () => ({
            entryId: 1,
            value: "sk-test",
            revealUntil: "2026-01-01T00:00:00Z",
          }),
          EnvContributionSettings: async () => ({
            settings: {
              publicEnvPatternsEnabled: true,
              privateLocalEnvPatternsEnabled: false,
              consentShown: false,
              updatedAt: "",
            },
            queue: { count: 0 },
          }),
          ClearEnvContributionQueue: async () => ({
            settings: {
              publicEnvPatternsEnabled: true,
              privateLocalEnvPatternsEnabled: false,
              consentShown: true,
              updatedAt: "",
            },
            queue: { count: 0 },
          }),
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).resolves.toEqual({ repos: [] });
  await expect(api.GenerateEnvDraft("C:\\repo")).resolves.toEqual({
    repoPath: "C:\\repo",
    targets: [],
  });
  await expect(api.ListEnvVaultEntries()).resolves.toEqual({ entries: [] });
  await expect(
    api.RevealEnvVaultEntry({ entryId: 1, confirmed: true }),
  ).resolves.toMatchObject({ entryId: 1, value: "sk-test" });
  await expect(api.EnvContributionSettings()).resolves.toMatchObject({
    settings: { publicEnvPatternsEnabled: true },
  });
  await expect(api.ClearEnvContributionQueue()).resolves.toMatchObject({
    queue: { count: 0 },
  });
});

test("desktop API reports missing bridge contract metadata", async () => {
  const api = createDesktopApi({
    go: {
      main: {
        App: {
          ShellInfo: async () => ({}),
          ListInstalledRepos: async () => ({ repos: [] }),
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).rejects.toThrow(
    missingBridgeContractMessage,
  );
});

test("desktop API reports missing ShellInfo bridge method", async () => {
  const api = createDesktopApi({
    go: {
      main: {
        App: {
          ListInstalledRepos: async () => ({ repos: [] }),
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).rejects.toThrow(
    missingBridgeContractMessage,
  );
});

test("desktop API reports outdated bridge contract metadata", async () => {
  const api = createDesktopApi({
    go: {
      main: {
        App: {
          ShellInfo: async () => ({ bridgeContractVersion: "old" }),
          ListInstalledRepos: async () => ({ repos: [] }),
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).rejects.toThrow(
    bridgeContractOutdatedMessage("old"),
  );
});
