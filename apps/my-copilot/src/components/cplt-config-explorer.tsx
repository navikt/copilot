"use client";

import { useState, useMemo } from "react";
import { CopyButton } from "@navikt/ds-react";

const CONFIG_KEYS = [
  {
    key: "proxy.enabled",
    type: "bool",
    default: "true",
    description: "Enable the CONNECT proxy for network filtering.",
    example: "cplt config set proxy.enabled false",
    section: "proxy",
  },
  {
    key: "proxy.port",
    type: "integer",
    default: "0 (ephemeral)",
    description: "Port for the CONNECT proxy. 0 means OS-assigned.",
    example: "cplt config set proxy.port 9090",
    section: "proxy",
  },
  {
    key: "proxy.blocked_domains",
    type: "path",
    default: "(empty)",
    description: "Path to file listing domains to block. Re-read every 5 seconds — no restart needed.",
    example: "cplt config set proxy.blocked_domains ~/my-blocklist.txt",
    section: "proxy",
  },
  {
    key: "proxy.allowed_domains",
    type: "path",
    default: "(empty)",
    description: "Path to allowlist. When set, only listed domains are permitted (fail-closed).",
    example: "cplt config set proxy.allowed_domains ~/my-allowlist.txt",
    section: "proxy",
  },
  {
    key: "proxy.log_file",
    type: "path",
    default: "(empty)",
    description: "Path to audit log. All proxy traffic is logged here regardless of log_level.",
    example: "cplt config set proxy.log_file ~/.config/cplt/proxy.log",
    section: "proxy",
  },
  {
    key: "proxy.log_level",
    type: "enum",
    default: "none",
    description: "Stderr verbosity for proxy output. Options: none, error, blocked, all.",
    example: "cplt config set proxy.log_level blocked",
    section: "proxy",
  },
  {
    key: "proxy.allow_private_domains",
    type: "string[]",
    default: "[]",
    description: "Domains allowed to resolve to private IP addresses (e.g. intranet).",
    example: "cplt config set proxy.allow_private_domains intern.nav.no",
    section: "proxy",
  },
  {
    key: "sandbox.validate",
    type: "bool",
    default: "true",
    description: "Validate sandbox profile before applying.",
    example: "cplt config set sandbox.validate false",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_env_files",
    type: "bool",
    default: "false",
    description: "Allow reading .env* files in the project directory.",
    example: "cplt config set sandbox.allow_env_files true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_lifecycle_scripts",
    type: "bool",
    default: "false",
    description: "Allow npm/yarn lifecycle scripts (postinstall, etc.).",
    example: "cplt config set sandbox.allow_lifecycle_scripts true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_gpg_signing",
    type: "bool",
    default: "false",
    description: "Allow GPG commit signing (requires ~/.gnupg access).",
    example: "cplt config set sandbox.allow_gpg_signing true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_jvm_attach",
    type: "bool",
    default: "false",
    description: "Allow JVM Attach API unix sockets for MockK/Mockito inline mocking.",
    example: "cplt config set sandbox.allow_jvm_attach true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_localhost_any",
    type: "bool",
    default: "false",
    description: "Allow outbound connections to any localhost port.",
    example: "cplt config set sandbox.allow_localhost_any true",
    section: "sandbox",
  },
  {
    key: "sandbox.scratch_dir",
    type: "bool",
    default: "true",
    description: "Redirect TMPDIR to a safe scratch directory (prevents write-then-exec from /tmp).",
    example: "cplt config set sandbox.scratch_dir false",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_tmp_exec",
    type: "bool",
    default: "false",
    description: "Allow execution from /tmp. Dangerous — prefer scratch_dir.",
    example: "cplt config set sandbox.allow_tmp_exec true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_cache_exec",
    type: "string[]",
    default: "[]",
    description: "Allow exec from specific ~/Library/Caches subdirectories (e.g. ms-playwright, pnpm/dlx).",
    example: "cplt config set sandbox.allow_cache_exec ms-playwright",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_cache_exec_any",
    type: "bool",
    default: "false",
    description: "Allow exec from all of ~/Library/Caches. Dangerous.",
    example: "cplt config set sandbox.allow_cache_exec_any true",
    section: "sandbox",
  },
  {
    key: "sandbox.inherit_env",
    type: "bool",
    default: "false",
    description: "Pass all environment variables to the sandbox. Dangerous — exposes secrets.",
    example: "cplt config set sandbox.inherit_env true",
    section: "sandbox",
  },
  {
    key: "sandbox.pass_env",
    type: "string[]",
    default: "[]",
    description: "Specific environment variables to pass through to the sandboxed process.",
    example: "cplt config set sandbox.pass_env ANTHROPIC_API_KEY",
    section: "sandbox",
  },
  {
    key: "allow.read",
    type: "path[]",
    default: "[]",
    description: "Additional paths to allow reading inside the sandbox.",
    example: "cplt config set allow.read ~/Desktop",
    section: "allow",
  },
  {
    key: "allow.write",
    type: "path[]",
    default: "[]",
    description: "Additional paths to allow writing inside the sandbox.",
    example: "cplt config set allow.write ~/output",
    section: "allow",
  },
  {
    key: "allow.ports",
    type: "integer[]",
    default: "[]",
    description: "Additional outbound TCP ports to allow (beyond 443).",
    example: "cplt config set allow.ports 8080",
    section: "allow",
  },
  {
    key: "allow.localhost",
    type: "integer[]",
    default: "[]",
    description: "Localhost ports to allow outbound connections to.",
    example: "cplt config set allow.localhost 3000",
    section: "allow",
  },
  {
    key: "deny.paths",
    type: "path[]",
    default: "[]",
    description: "Additional paths to explicitly deny (even if parent is allowed).",
    example: "cplt config set deny.paths ~/secret-project",
    section: "deny",
  },
  {
    key: "sandbox.quiet",
    type: "bool",
    default: "false",
    description: "Suppress startup config summary. Errors and warnings always shown.",
    example: "cplt config set sandbox.quiet true",
    section: "sandbox",
  },
  {
    key: "sandbox.allow_docker",
    type: "bool",
    default: "false",
    description: "Expose Docker daemon socket. Dangerous — container volumes bypass sandbox.",
    example: "cplt config set sandbox.allow_docker true",
    section: "sandbox",
  },
] as const;

const SECTIONS = ["all", "proxy", "sandbox", "allow", "deny"] as const;
type Section = (typeof SECTIONS)[number];

const TYPE_COLORS: Record<string, { bg: string; text: string }> = {
  bool: { bg: "#dbeafe", text: "#1e40af" },
  path: { bg: "#fef3c7", text: "#92400e" },
  "path[]": { bg: "#fef3c7", text: "#92400e" },
  "string[]": { bg: "#ede9fe", text: "#5b21b6" },
  "integer[]": { bg: "#fce7f3", text: "#9d174d" },
  integer: { bg: "#fce7f3", text: "#9d174d" },
  enum: { bg: "#d1fae5", text: "#065f46" },
};

export function CpltConfigExplorer() {
  const [search, setSearch] = useState("");
  const [activeSection, setActiveSection] = useState<Section>("all");

  const hasActiveFilter = search.length > 0 || activeSection !== "all";

  const filtered = useMemo(() => {
    if (!hasActiveFilter) return [];
    const q = search.toLowerCase();
    return CONFIG_KEYS.filter((item) => {
      if (activeSection !== "all" && item.section !== activeSection) return false;
      if (!q) return true;
      return item.key.toLowerCase().includes(q) || item.description.toLowerCase().includes(q);
    });
  }, [search, activeSection, hasActiveFilter]);

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
          {SECTIONS.map((s) => (
            <button
              key={s}
              onClick={() => setActiveSection(s)}
              className="rounded-full font-medium cursor-pointer"
              style={{
                padding: "0.375rem 0.875rem",
                fontSize: "0.75rem",
                border: "1px solid",
                borderColor: activeSection === s ? "#10b981" : "#e2e8f0",
                background: activeSection === s ? "#ecfdf5" : "white",
                color: activeSection === s ? "#065f46" : "#64748b",
                transition: "all 150ms",
              }}
            >
              {s === "all" ? "All" : s}
            </button>
          ))}
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
            Type to search or select a section to browse {CONFIG_KEYS.length} config options.
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
                <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem", marginLeft: "auto" }}>
                  default: {item.default}
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
