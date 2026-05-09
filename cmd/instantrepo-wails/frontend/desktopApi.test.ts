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
        },
      },
    },
  });

  await expect(api.ListInstalledRepos()).resolves.toEqual({ repos: [] });
});
