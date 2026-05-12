import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { SettingsView } from "./src/SettingsView";

test("SettingsView renders contribution consent and queue controls", () => {
  const html = renderToStaticMarkup(
    <SettingsView
      response={{
        settings: {
          publicEnvPatternsEnabled: true,
          privateLocalEnvPatternsEnabled: false,
          consentShown: false,
          updatedAt: "",
        },
        queue: { count: 2 },
      }}
      loading={false}
      onRefresh={() => {}}
      onSaveSettings={() => {}}
      onRecordConsent={() => {}}
      onClearQueue={() => {}}
    />,
  );

  expect(html).toContain("Env Pattern Contribution");
  expect(html).toContain("Public repos");
  expect(html).toContain("Private/local repos");
  expect(html).toContain("2 queued");
  expect(html).toContain("Clear queue");
  expect(html).toContain("checked");
});
