import { expect, test } from "bun:test";
import {
  createDesktopApi,
  desktopSessionUnavailableMessage,
  getDesktopApp,
} from "./src/desktopApi";

test("desktop API reports missing Wails bridge with user-facing message", () => {
  expect(() => getDesktopApp({})).toThrow(desktopSessionUnavailableMessage);
});

test("desktop API calls Wails bridge when it exists", async () => {
  const api = createDesktopApi({
    go: {
      main: {
        App: {
          ListInstalledRepos: async () => ({ repos: [] }),
          GenerateEnvDraft: async () => ({ repoPath: "C:\\repo", targets: [] }),
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).resolves.toEqual({ repos: [] });
  await expect(api.GenerateEnvDraft("C:\\repo")).resolves.toEqual({
    repoPath: "C:\\repo",
    targets: [],
  });
});
