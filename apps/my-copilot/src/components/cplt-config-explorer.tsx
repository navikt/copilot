"use client";

import { useState, useMemo } from "react";
import { CopyButton } from "@navikt/ds-react";
import type { CpltConfigKey } from "@/lib/cplt-config";

type ConfigItem = CpltConfigKey & { example: string };

// Sections are no longer used in the flat nav-pilot config
type Section = "general";

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  bool: { bg: "#dbeafe", text: "#1e40af" },
  string: { bg: "#fef3c7", text: "#92400e" },
  "string[]": { bg: "#ede9fe", text: "#5b21b6" },
  "integer[]": { bg: "#fce7f3", text: "#9d174d" },
  integer: { bg: "#fce7f3", text: "#9d174d" },
};

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
  const [activeSection, setActiveSection] = useState<Section | "all" | null>(null);

  const items: ConfigItem[] = useMemo(() => configKeys.map((k) => ({ ...k, example: makeExample(k) })), [configKeys]);

  const hasActiveFilter = search.length > 0 || activeSection !== null;

  const filtered = useMemo(() => {
    if (!hasActiveFilter) return [];
    const q = search.toLowerCase();
    return items.filter((item) => {
      if (!q) return true;
      return item.key.toLowerCase().includes(q) || item.description.toLowerCase().includes(q);
    });
  }, [search, hasActiveFilter, items]);

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
            border: "1px solid #e2e8f0",
            fontSize: "0.875rem",
            background: "white",
            outline: "none",
          }}
        />
        <div className="flex gap-1.5 flex-wrap">
          <button
            onClick={() => setActiveSection(activeSection === "all" ? null : "all")}
            className="rounded-full font-medium cursor-pointer"
            style={{
              padding: "0.375rem 0.875rem",
              fontSize: "0.75rem",
              border: "1px solid",
              borderColor: activeSection === "all" ? "#10b981" : "#e2e8f0",
              background: activeSection === "all" ? "#ecfdf5" : "white",
              color: activeSection === "all" ? "#065f46" : "#64748b",
              transition: "all 150ms",
            }}
          >
            Show All
          </button>
        </div>
      </div>

      {/* Results count */}
      {hasActiveFilter && (
        <p style={{ color: "#94a3b8", fontSize: "0.75rem", margin: "0 0 0.75rem" }}>
          {filtered.length} {filtered.length === 1 ? "option" : "options"}
        </p>
      )}

      {/* Config list */}
      <div className="flex flex-col gap-3">
        {!hasActiveFilter && (
          <p className="text-center py-8" style={{ color: "#94a3b8", fontSize: "0.875rem" }}>
            Type to search or select a section to browse {items.length} config options.
          </p>
        )}

        {filtered.map((item) => {
          const typeColor = TYPE_COLORS[item.type] || { bg: "#f1f5f9", text: "#475569" };
          return (
            <div
              key={item.key}
              className="rounded-lg"
              style={{
                background: "white",
                border: "1px solid #e2e8f0",
                padding: "1rem 1.25rem",
              }}
            >
              {/* Header row */}
              <div className="flex flex-wrap items-center gap-2 mb-1.5">
                <code className="font-mono font-bold" style={{ color: "#059669", fontSize: "0.875rem" }}>
                  {item.key}
                </code>
                <span
                  className="rounded-full font-medium"
                  style={{
                    padding: "0.125rem 0.5rem",
                    fontSize: "0.625rem",
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
                      fontSize: "0.625rem",
                      background: "#fef2f2",
                      color: "#dc2626",
                    }}
                  >
                    ⚠ dangerous
                  </span>
                )}
                <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem", marginLeft: "auto" }}>
                  default: {item.default || '""'}
                </span>
              </div>

              {/* Description */}
              <p style={{ color: "#475569", fontSize: "0.8125rem", margin: "0 0 0.75rem", lineHeight: 1.5 }}>
                {item.description}
              </p>

              {/* Example */}
              <div
                className="rounded-md flex items-center gap-2"
                style={{ background: "#1e1e1e", padding: "0.4rem 0.75rem" }}
              >
                <code
                  className="font-mono whitespace-nowrap overflow-x-auto flex-1"
                  style={{ fontSize: "0.7rem", color: "#d4d4d4" }}
                >
                  {item.example}
                </code>
                <CopyButton copyText={item.example} size="xsmall" style={{ color: "white" }} />
              </div>
            </div>
          );
        })}

        {hasActiveFilter && filtered.length === 0 && (
          <p className="text-center py-8" style={{ color: "#94a3b8", fontSize: "0.875rem" }}>
            No config options match your search.
          </p>
        )}
      </div>
    </div>
  );
}
