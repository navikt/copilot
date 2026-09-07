"use client";

import { useState, useMemo } from "react";
import { CopyButton } from "@navikt/ds-react";
import type { CpltConfigKey } from "@/lib/cplt-config";

type ConfigItem = CpltConfigKey & { example: string };

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  bool: { bg: "#dbeafe", text: "#1e40af" },
  string: { bg: "#fef3c7", text: "#92400e" },
  "string[]": { bg: "#ede9fe", text: "#5b21b6" },
  "integer[]": { bg: "#fce7f3", text: "#9d174d" },
  integer: { bg: "#fce7f3", text: "#9d174d" },
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
    <div>
      {/* Search + filter */}
      <div className="flex flex-col sm:flex-row gap-3 mb-6">
        <input
          type="text"
          placeholder="Search config keys…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          aria-label="Search config keys"
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
      <p style={{ color: "var(--ax-text-neutral-subtle)", fontSize: CODE_SIZE, margin: "0 0 0.75rem" }}>
        {filtered.length} {filtered.length === 1 ? "option" : "options"}
      </p>

      {/* Config list. Capped so the reference does not swallow the landing page. */}
      <div
        className="flex flex-col gap-3"
        role="region"
        aria-label="Configuration options"
        tabIndex={0}
        style={{ maxHeight: "32rem", overflowY: "auto", paddingRight: "0.5rem" }}
      >
        {filtered.map((item) => {
          const typeColor = TYPE_COLORS[item.type] || { bg: "#f1f5f9", text: "#475569" };
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
              <div className="flex flex-wrap items-center gap-2 mb-1.5">
                <code className="font-mono font-bold" style={{ color: "var(--cplt-accent-ink)", fontSize: "0.875rem" }}>
                  {item.key}
                </code>
                <span
                  className="rounded-full font-medium"
                  style={{
                    padding: "0.125rem 0.5rem",
                    fontSize: CODE_SIZE,
                    background: typeColor.bg,
                    color: typeColor.text,
                  }}
                >
                  {item.type}
                </span>
                {item.dangerous && (
                  <span
                    className="rounded-full font-medium"
                    style={{
                      padding: "0.125rem 0.5rem",
                      fontSize: CODE_SIZE,
                      background: "#fef2f2",
                      color: "#b91c1c",
                    }}
                  >
                    ⚠ dangerous
                  </span>
                )}
                <span
                  className="font-mono"
                  style={{ color: "var(--ax-text-neutral-subtle)", fontSize: CODE_SIZE, marginLeft: "auto" }}
                >
                  default: {item.default || '""'}
                </span>
              </div>

              {/* Description */}
              <p
                style={{
                  color: "var(--ax-text-neutral-subtle)",
                  fontSize: "0.875rem",
                  margin: "0 0 0.75rem",
                  lineHeight: 1.5,
                }}
              >
                {item.description}
              </p>

              {/* Example */}
              <div
                className="rounded-md flex items-center gap-2"
                style={{ background: "#1e1e1e", padding: "0.4rem 0.75rem" }}
              >
                <code
                  className="font-mono whitespace-nowrap overflow-x-auto flex-1"
                  style={{ fontSize: CODE_SIZE, color: "#d4d4d4" }}
                >
                  {item.example}
                </code>
                <CopyButton copyText={item.example} size="small" style={{ color: "white" }} />
              </div>
            </div>
          );
        })}

        {filtered.length === 0 && (
          <p className="text-center py-8" style={{ color: "var(--ax-text-neutral-subtle)", fontSize: "0.875rem" }}>
            No config options match your search.
          </p>
        )}
      </div>
    </div>
  );
}
