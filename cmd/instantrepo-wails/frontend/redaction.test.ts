import { expect, test } from "bun:test";
import { redactLikelySecrets } from "./src/redaction";

test("redacts likely secrets from UI activity text", () => {
  const redacted = redactLikelySecrets(
    "OPENAI_API_KEY=sk-live-secret npm run setup --token abc123 Bearer xyz789 https://user:pass@example.com",
  );

  expect(redacted).not.toContain("sk-live-secret");
  expect(redacted).not.toContain("abc123");
  expect(redacted).not.toContain("xyz789");
  expect(redacted).not.toContain("user:pass");
  expect(redacted).toContain("[REDACTED]");
});
