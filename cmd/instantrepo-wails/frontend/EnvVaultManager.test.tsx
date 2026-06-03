import { expect, test } from "bun:test";
import { renderToStaticMarkup } from "react-dom/server";
import { EnvVaultManager } from "./src/EnvVaultManager";
import type { EnvVaultManagerEntry } from "./src/types";

const noop = () => {};

const entry: EnvVaultManagerEntry = {
  id: 7,
  provider: "openai",
  variableName: "OPENAI_API_KEY",
  displayName: "OpenAI work key",
  fingerprintFragment: "abc123def456",
  status: "ready",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
  usage: {
    totalUseCount: 3,
    locations: [
      {
        repoPath: "C:\\Repos\\demo",
        targetRelativePath: ".env",
        variableName: "OPENAI_API_KEY",
        lastUsedAt: "2026-01-02T00:00:00Z",
        useCount: 3,
      },
    ],
  },
  approvals: [
    {
      id: 11,
      entryId: 7,
      repoPath: "C:\\Repos\\demo",
      targetRelativePath: ".env",
      variableName: "OPENAI_API_KEY",
      status: "approved",
      createdAt: "2026-01-01T00:00:00Z",
      updatedAt: "2026-01-01T00:00:00Z",
    },
  ],
};

test("EnvVaultManager renders value-free credential metadata and actions", () => {
  const html = renderToStaticMarkup(
    <EnvVaultManager
      entries={[entry]}
      loading={false}
      revealedValues={{}}
      onRefresh={noop}
      onReveal={noop}
      onRename={noop}
      onUpdateValue={noop}
      onRemove={noop}
      onStatusChange={noop}
      onApprove={noop}
      onRevokeApproval={noop}
    />,
  );

  expect(html).toContain("OpenAI work key");
  expect(html).toContain("openai");
  expect(html).toContain("OPENAI_API_KEY");
  expect(html).toContain("abc123def456");
  expect(html).toContain("3 uses");
  expect(html).toContain("C:\\Repos\\demo");
  expect(html).toContain("Reveal");
  expect(html).not.toContain("sk-");
});

test("EnvVaultManager treats null usage and approval lists as empty", () => {
  const sparseEntry = {
    ...entry,
    usage: {
      totalUseCount: 0,
      locations: null,
    },
    approvals: null,
  } as unknown as EnvVaultManagerEntry;

  const html = renderToStaticMarkup(
    <EnvVaultManager
      entries={[sparseEntry]}
      loading={false}
      revealedValues={{}}
      onRefresh={noop}
      onReveal={noop}
      onRename={noop}
      onUpdateValue={noop}
      onRemove={noop}
      onStatusChange={noop}
      onApprove={noop}
      onRevokeApproval={noop}
    />,
  );

  expect(html).toContain("No recorded use.");
  expect(html).toContain("No repo approvals.");
});
