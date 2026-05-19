import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { EnvContributionConsentPanel } from "./src/EnvContributionConsentPanel";

test("first-run contribution consent stays non-modal beside setup action", () => {
  const html = renderToStaticMarkup(
    <section>
      <button type="button">Clone &amp; Analyze</button>
      <EnvContributionConsentPanel
        publicEnabled={true}
        loading={false}
        onPublicEnabledChange={() => {}}
        onSave={() => {}}
      />
    </section>,
  );

  expect(html).toContain("Clone &amp; Analyze");
  expect(html).toContain("Env Pattern Contribution");
  expect(html).toContain("value-free env names from confirmed public repos");
  expect(html).toContain("Public repos");
  expect(html).toContain("Save choice");
  expect(html).not.toContain("modal-backdrop");
  expect(html).not.toContain('role="dialog"');
  expect(html).not.toContain('aria-modal="true"');
});
