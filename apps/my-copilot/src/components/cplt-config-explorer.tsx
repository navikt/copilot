"use client";

import { useState, useMemo } from "react";
import { CopyButton, Tag, VStack } from "@navikt/ds-react";
import type { CpltConfigKey } from "@/lib/cplt-config";

type ConfigItem = CpltConfigKey & { example: string };

// Aksel tag colours per value type, so they follow dark mode.
const TYPE_VARIANTS: Record<string, "info" | "warning" | "alt1" | "alt3"> = {
  bool: "info",
  string: "warning",
  "string[]": "alt1",
  "integer[]": "alt3",
  integer: "alt3",
};

const CODE_SIZE = "0.75rem";

function makeExample(item: CpltConfigKey): string {
  switch (item.type) {
    case "bool":
      return `cplt config set ${item.key} ${item.default === "true" ? "false" : "true"}`;
    case "integer":
      return `cplt config set ${item.key} 8080`;
    case "string":
      return `cplt config set ${item.key} "value"`;
    case "string[]":
      return `cplt config set ${item.key} "value1,value2"`;
    case "integer[]":
      return `cplt config set ${item.key} 3000`;
    default:
      return `cplt config set ${item.key} "value"`;
  }
}

export function CpltConfigExplorer({ configKeys }: { configKeys: CpltConfigKey[] }) {
  const [search, setSearch] = useState("");

  const items: ConfigItem[] = useMemo(() => configKeys.map((k) => ({ ...k, example: makeExample(k) })), [configKeys]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return items;
    return items.filter((item) => item.key.toLowerCase().includes(q) || item.description.toLowerCase().includes(q));
  }, [search, items]);

  return (
    <VStack gap="space-12">
      {/* Search + filter */}
      <div className="flex flex-col sm:flex-row gap-3">
        <input
          type="text"
          placeholder="Søk i innstillingene…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          aria-label="Søk i innstillingene"
          className="rounded-lg font-mono flex-1"
          style={{
            padding: "0.625rem 1rem",
            border: "1px solid var(--ax-border-neutral-subtle)",
            fontSize: "0.875rem",
            background: "var(--ax-bg-default)",
            outline: "none",
          }}
        />
      </div>

      {/* Results count */}
      <p style={{ color: "var(--ax-text-neutral-subtle)", fontSize: CODE_SIZE, margin: 0 }}>
        {filtered.length} {filtered.length === 1 ? "innstilling" : "innstillinger"}
      </p>

      {/* Config list. Capped so the reference does not swallow the page. */}
      <div
        className="flex flex-col gap-3"
        role="region"
        aria-label="Innstillinger for cplt"
        tabIndex={0}
        style={{ maxHeight: "32rem", overflowY: "auto", paddingRight: "0.5rem" }}
      >
        {filtered.map((item) => {
          return (
            <div
              key={item.key}
              className="rounded-lg"
              style={{
                background: "var(--ax-bg-default)",
                border: "1px solid var(--ax-border-neutral-subtle)",
                padding: "1rem 1.25rem",
              }}
            >
              {/* Header row */}
              <div className="flex flex-wrap items-center gap-2" style={{ marginBottom: "var(--ax-space-6)" }}>
                <code className="font-mono font-bold" style={{ color: "var(--ax-text-neutral)", fontSize: "0.875rem" }}>
                  {item.key}
                </code>
                <Tag size="xsmall" variant={TYPE_VARIANTS[item.type] ?? "neutral"}>
                  {item.type}
                </Tag>
                {item.dangerous && (
                  <Tag size="xsmall" variant="error">
                    ⚠ farlig
                  </Tag>
                )}
                <span
                  className="font-mono"
                  style={{ color: "var(--ax-text-neutral-subtle)", fontSize: CODE_SIZE, marginLeft: "auto" }}
                >
                  standard: {item.default || '""'}
                </span>
              </div>

              {/* Description */}
              <p
                style={{
                  color: "var(--ax-text-neutral-subtle)",
                  fontSize: "0.875rem",
                  margin: "0 0 var(--ax-space-12)",
                  lineHeight: 1.5,
                }}
              >
                {item.description}
              </p>

              {/* Example */}
              <div
                className="rounded-md flex items-center gap-2"
                style={{ background: "var(--ax-bg-neutral-soft)", padding: "0.4rem 0.75rem" }}
              >
                <code
                  className="font-mono whitespace-nowrap overflow-x-auto flex-1"
                  style={{ fontSize: CODE_SIZE, color: "var(--ax-text-neutral)" }}
                >
                  {item.example}
                </code>
                <CopyButton copyText={item.example} size="small" />
              </div>
            </div>
          );
        })}

        {filtered.length === 0 && (
          <p
            className="text-center"
            style={{ paddingBlock: "var(--ax-space-32)", color: "var(--ax-text-neutral-subtle)", fontSize: "0.875rem" }}
          >
            Ingen innstillinger passer til søket.
          </p>
        )}
      </div>
    </VStack>
  );
}
