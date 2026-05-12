import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { EnvVaultPrompt } from "./src/EnvVaultPrompt";

test("EnvVaultPrompt shows value-free save choices", () => {
  const html = renderToStaticMarkup(
    <EnvVaultPrompt
      candidate={{
        repoPath: "C:\\Repos\\demo",
        targetRelativePath: ".env",
        variableName: "OPENAI_API_KEY",
        provider: "openai",
        fingerprintFragment: "abc123",
      }}
      displayName=""
      onDisplayNameChange={() => {}}
      onSave={() => {}}
      onDismiss={() => {}}
      onSuppress={() => {}}
    />,
  );

  expect(html).toContain("OPENAI_API_KEY");
  expect(html).toContain("Save to Vault");
  expect(html).toContain("Not now");
  expect(html).toContain("Never ask for this var");
  expect(html).not.toContain("sk-");
});
