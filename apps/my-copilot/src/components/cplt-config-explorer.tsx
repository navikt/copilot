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

/* Keys the generic example below gets wrong: enum values, paths, repo- or
   local-only keys, and values `config set` refuses without --force
   (security_confirmation in navikt/cplt src/config/editing.rs). */
const EXAMPLE_OVERRIDES: Record<string, string> = {
  "proxy.enabled": "cplt config set proxy.enabled false --force",
  "proxy.blocked_domains": 'cplt config set proxy.blocked_domains "~/.config/cplt/blocked-domains.txt"',
  "proxy.allowed_domains": 'cplt config set proxy.allowed_domains "~/.config/cplt/allowed-domains.txt"',
  "proxy.log_file": 'cplt config set proxy.log_file "~/.cache/cplt/proxy.log"',
  "proxy.log_level": "cplt config set proxy.log_level blocked",
  "proxy.timeout": "cplt config set proxy.timeout 120",
  "proxy.upstream": 'cplt config set proxy.upstream "http://proxy.example.com:8080"',
  "proxy.upstream_no_proxy": "cplt config set proxy.upstream_no_proxy intern.example.com",
  "proxy.allow_private_domains": "cplt config set proxy.allow_private_domains intern.example.com",
  "allow.exec": 'cplt config set allow.exec "/opt/toolchain"',
  "allow.socket": 'cplt config set allow.socket "~/.colima/default/docker.sock"',
  "allow.domains": "cplt config set allow.domains registry.example.com",
  "deny.paths": 'cplt config set deny.paths "~/secrets"',
  "deny.env": "cplt config set --repo deny.env VAULT_TOKEN",
  "sandbox.agent": "cplt config set sandbox.agent opencode",
  "sandbox.preset": "cplt config set sandbox.preset strict",
  "sandbox.validate": "cplt config set sandbox.validate false --force",
  "sandbox.audit": "cplt config set sandbox.audit false --force",
  "sandbox.pass_env": "cplt config set sandbox.pass_env MY_VAR",
  "sandbox.repo_dirs": "cplt config set --local sandbox.repo_dirs ../other-repo",
  "sandbox.allow_cache_exec": "cplt config set sandbox.allow_cache_exec ms-playwright",
  "sandbox.worktree_walk_max_dirs": "cplt config set sandbox.worktree_walk_max_dirs 200000",
  "gh_guard.enabled": "cplt config set gh_guard.enabled false --force",
  "gh_guard.mode": "cplt config set gh_guard.mode audit --force",
  "gh_guard.scope_check": "cplt config set gh_guard.scope_check false --force",
  "gh_guard.block_auth_token": "cplt config set gh_guard.block_auth_token false --force",
  "gh_guard.unknown_command": "cplt config set gh_guard.unknown_command allow --force",
  "git_guard.enabled": "cplt config set git_guard.enabled false --force",
  "git_guard.mode": "cplt config set git_guard.mode audit --force",
  "git_guard.prevent_push": "cplt config set git_guard.prevent_push false --force",
  "git_guard.prevent_force_push": "cplt config set git_guard.prevent_force_push false --force",
  // `config set` cannot write an array of tables: add a [[git_guard.allow_push]] block to the config file.
  "git_guard.allow_push": "cplt config explain git_guard.allow_push",
  "shell.skip": "cplt config set shell.skip goose",
};

function makeExample(item: CpltConfigKey): string {
  const override = EXAMPLE_OVERRIDES[item.key];
  if (override) return override;
  switch (item.type) {
    case "bool": {
      const value = item.default === "true" ? "false" : "true";
      // Turning on a dangerous key needs --force.
      return `cplt config set ${item.key} ${value}${item.dangerous && value === "true" ? " --force" : ""}`;
    }
    case "integer":
      return `cplt config set ${item.key} 8080`;
    case "string":
      return `cplt config set ${item.key} "value"`;
    case "string[]":
      // One value per call: `config set` refuses a comma.
      return `cplt config set ${item.key} "~/shared-libs"`;
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
                {/* Wraps instead of scrolling, so the 60-odd examples add no tab stops (#1195). */}
                <code
                  className="font-mono whitespace-pre-wrap break-all flex-1 min-w-0"
                  style={{ fontSize: CODE_SIZE, color: "#d4d4d4" }}
                >
                  {item.example}
                </code>
                <CopyButton
                  title={`Copy the ${item.key} example`}
                  activeText="Copied!"
                  copyText={item.example}
                  size="small"
                  className="shrink-0"
                  style={{ color: "white" }}
                />
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
