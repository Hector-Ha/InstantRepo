import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { AppNav } from "./src/AppNav";

test("AppNav presents dashboard depth from overview to details", () => {
  const html = renderToStaticMarkup(
    <AppNav activeView="overview" onChange={() => {}} />,
  );

  expect(html).toContain("Overview");
  expect(html).toContain("Repositories");
  expect(html).toContain("Environment");
  expect(html).toContain("Setup Steps");
  expect(html).toContain("Env Vault");
  expect(html).toContain("Settings");
  expect(html).not.toContain(">Setup<");
  expect(html).toContain('aria-pressed="true"');
});
