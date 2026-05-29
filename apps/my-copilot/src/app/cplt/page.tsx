import type { Metadata } from "next";
import { Box, VStack, HGrid, Heading, CopyButton } from "@navikt/ds-react";
import NextLink from "next/link";
import { CpltConfigExplorer } from "@/components/cplt-config-explorer";
import { fetchCpltConfigKeys } from "@/lib/cplt-config";
import {
  ShieldLockIcon,
  TerminalIcon,
  BugIcon,
  CloudIcon,
  CheckmarkCircleIcon,
  CogRotationIcon,
  PersonGroupIcon,
  WrenchIcon,
  MagnifyingGlassIcon,
} from "@navikt/aksel-icons";

export const metadata: Metadata = {
  title: "cplt — Sandbox for AI coding agents",
  description:
    "cplt runs GitHub Copilot CLI, OpenCode, or a plain shell inside a kernel-level sandbox so the agent can work on your project but cannot access your secrets.",
  openGraph: {
    title: "cplt — Sandbox for AI coding agents",
    description:
      "Kernel-level isolation for AI coding agents. Your secrets stay secret — enforced by the OS, not by trust.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "cplt — Sandbox for AI coding agents",
    description:
      "Kernel-level isolation for AI coding agents. Your secrets stay secret — enforced by the OS, not by trust.",
  },
};

/* ---------- Data ---------- */

const INSTALL_COMMAND = "brew install navikt/tap/cplt";
const INSTALL_SCRIPT = "curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash";

const PROTECTIONS = [
  {
    Icon: ShieldLockIcon,
    color: "#f87171",
    title: "Filesystem Isolation",
    description:
      "Kernel-level blocks on secrets, credentials, keys, and .env files. Your ~/.ssh, ~/.aws, and registry credentials are invisible to the agent.",
  },
  {
    Icon: CloudIcon,
    color: "#10b981",
    title: "Network Control",
    description:
      "CONNECT proxy intercepts all outbound traffic. Blocklist or allowlist mode, private IP protection, full audit logging.",
  },
  {
    Icon: BugIcon,
    color: "#34d399",
    title: "Environment Hardening",
    description:
      "npm lifecycle scripts disabled, safe env var allowlist, git hooks write-protected, no exec from /tmp.",
  },
  {
    Icon: TerminalIcon,
    color: "#6ee7b7",
    title: "Multi-platform Enforcement",
    description:
      "Same policy on macOS (Seatbelt) and Linux (Landlock + seccomp-BPF). Kernel-enforced — no userspace bypass.",
  },
];

const SECURITY_TABLE = [
  { resource: "Project directory (read/write)", without: "allowed", with: "allowed" },
  { resource: "Secrets (.env*, .pem, .key, SSH keys)", without: "exposed", with: "blocked" },
  { resource: "Credentials (~/.aws, ~/.azure, ~/.m2, ~/.gradle, ~/.cargo)", without: "exposed", with: "blocked" },
  { resource: "Git hooks, /tmp execution, SSH agent", without: "exposed", with: "blocked" },
  { resource: "Outbound network (HTTPS)", without: "exposed", with: "filtered" },
  { resource: "Private IPs and localhost", without: "exposed", with: "blocked" },
  { resource: "Destructive git/gh commands (push, merge, delete)", without: "exposed", with: "blocked" },
  { resource: "Copilot auth and tool caches (read-only)", without: "allowed", with: "allowed" },
];

const AGENTS = [
  {
    name: "GitHub Copilot CLI",
    command: 'cplt -- -p "fix the tests"',
    description: "Default. Runs Copilot CLI in sandbox with full filesystem and network isolation.",
    Icon: TerminalIcon,
    color: "#10b981",
  },
  {
    name: "OpenCode",
    command: "cplt --agent opencode",
    description: "Runs OpenCode in sandbox. Same kernel-level protections, different AI agent.",
    Icon: CogRotationIcon,
    color: "#34d399",
  },
  {
    name: "Shell",
    command: "cplt --agent shell",
    description: "A sandboxed shell with no AI. Useful for testing what the sandbox allows and blocks.",
    Icon: PersonGroupIcon,
    color: "#6ee7b7",
  },
];

/* ---------- Helpers ---------- */

async function getStarCount(): Promise<number | null> {
  try {
    const res = await fetch("https://api.github.com/repos/navikt/cplt", {
      next: { revalidate: 3600 },
      headers: { Accept: "application/vnd.github.v3+json" },
    });
    if (!res.ok) return null;
    const data = await res.json();
    return data.stargazers_count ?? null;
  } catch {
    return null;
  }
}

/* ---------- Page ---------- */

export default async function CpltPage() {
  const [stars, configKeys] = await Promise.all([getStarCount(), fetchCpltConfigKeys()]);
  return (
    <main>
      <HeroSection stars={stars} />
      <SecurityTableSection />
      <ProtectionsSection />
      <ProxySection />
      <GuardsSection />
      <MultiAgentSection />
      <TeamConfigSection />
      <InitSection />
      <ConfigSection configKeys={configKeys} />
      <HowItWorksSection />
      <FooterSection />
    </main>
  );
}

/* ---------- Hero ---------- */

function HeroSection({ stars }: { stars: number | null }) {
  return (
    <section
      className="dark-section"
      style={{
        background: "linear-gradient(165deg, #0a0f0c 0%, #0d2118 35%, #143d2b 65%, #0a1f14 100%)",
        color: "white",
      }}
    >
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-20", md: "space-32" }}>
          {/* Headline */}
          <VStack gap="space-12" className="text-center">
            <Heading size="xlarge" level="1">
              <code style={{ fontFamily: "monospace", fontWeight: 800 }}>cplt</code> — Your AI agent is sandboxed.
            </Heading>
            <p
              className="max-w-2xl mx-auto"
              style={{ color: "#94a3b8", fontSize: "1.125rem", lineHeight: 1.7, marginBlock: 0, textAlign: "center" }}
            >
              Kernel-level isolation for AI coding agents. Your secrets stay secret — enforced by the OS, not by trust.
            </p>
            {/* GitHub badge */}
            <div className="flex justify-center">
              <NextLink
                href="https://github.com/navikt/cplt"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 rounded-full px-3.5 py-1.5 no-underline transition-all"
                style={{
                  background: "rgba(255,255,255,0.06)",
                  border: "1px solid rgba(255,255,255,0.12)",
                  color: "rgba(255,255,255,0.7)",
                  fontSize: "0.8125rem",
                }}
              >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
                navikt/cplt
                {stars !== null && (
                  <span
                    className="inline-flex items-center gap-1 rounded-full px-2 py-0.5"
                    style={{ background: "rgba(255,255,255,0.08)", fontSize: "0.75rem" }}
                  >
                    ★ {stars}
                  </span>
                )}
              </NextLink>
            </div>
          </VStack>

          {/* Demo GIF in window chrome */}
          <div className="max-w-4xl mx-auto w-full">
            <div
              className="rounded-xl overflow-hidden"
              style={{
                border: "1px solid rgba(255,255,255,0.1)",
                boxShadow: "0 8px 40px rgba(0,0,0,0.5)",
              }}
            >
              {/* Window chrome */}
              <div
                className="flex items-center gap-2 px-4 py-2.5"
                style={{ background: "#1a1a1e", borderBottom: "1px solid rgba(255,255,255,0.06)" }}
              >
                <div className="flex gap-1.5">
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#ff5f57" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#febc2e" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#28c840" }} />
                </div>
                <span className="font-mono ml-2" style={{ color: "#6b7280", fontSize: "0.75rem" }}>
                  cplt — sandboxed Copilot session
                </span>
              </div>
              {/* GIF */}
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                src="/demos/cplt-demo.gif"
                alt="cplt demo: Copilot agent attempts to read credentials and exfiltrate data, all blocked by cplt sandbox"
                className="w-full"
                style={{ display: "block", background: "#0c0c0c" }}
              />
            </div>
          </div>

          {/* Install CTA */}
          <div className="flex flex-col items-center gap-4">
            <div
              className="rounded-lg px-4 py-2.5 flex items-center gap-3 max-w-full overflow-x-auto"
              style={{
                background: "rgba(255,255,255,0.04)",
                border: "1px solid rgba(255,255,255,0.08)",
              }}
            >
              <code className="font-mono" style={{ fontSize: "0.8rem", color: "rgba(255,255,255,0.7)" }}>
                {INSTALL_COMMAND}
              </code>
              <CopyButton copyText={INSTALL_COMMAND} size="xsmall" style={{ color: "white" }} />
            </div>
            <div
              className="rounded-lg px-4 py-2 flex items-center gap-3 max-w-full overflow-x-auto"
              style={{
                background: "rgba(255,255,255,0.02)",
                border: "1px solid rgba(255,255,255,0.05)",
              }}
            >
              <code className="font-mono" style={{ fontSize: "0.7rem", color: "rgba(255,255,255,0.5)" }}>
                {INSTALL_SCRIPT}
              </code>
              <CopyButton copyText={INSTALL_SCRIPT} size="xsmall" style={{ color: "white" }} />
            </div>
            <p style={{ color: "#a7f3d0", fontSize: "0.8125rem", margin: 0 }}>
              macOS (Apple Seatbelt) · Linux (Landlock + seccomp-BPF)
            </p>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Security Table ---------- */

function SecurityTableSection() {
  return (
    <section style={{ background: "#f8fafc" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Security boundary
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              What your agent can and cannot access — enforced at the kernel level.
            </p>
          </div>

          <div className="w-full overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
            <div
              className="rounded-xl overflow-hidden min-w-max sm:min-w-0"
              style={{ border: "1px solid #1a3326", boxShadow: "0 4px 12px rgba(0,0,0,0.08)" }}
            >
              {/* Header */}
              <div className="grid gap-0" style={{ gridTemplateColumns: "3fr 1fr 1fr", background: "#0c1a14" }}>
                <div className="px-6 py-4 flex items-center" style={{ borderRight: "1px solid rgba(255,255,255,0.1)" }}>
                  <p
                    className="font-semibold uppercase tracking-wider"
                    style={{ color: "rgba(255,255,255,0.5)", fontSize: "0.7rem", margin: 0, letterSpacing: "0.08em" }}
                  >
                    Resource
                  </p>
                </div>
                <div
                  className="px-4 py-4 flex items-center justify-center"
                  style={{ borderRight: "1px solid rgba(255,255,255,0.1)" }}
                >
                  <p
                    className="font-semibold uppercase tracking-wider"
                    style={{ color: "rgba(255,255,255,0.5)", fontSize: "0.7rem", margin: 0, letterSpacing: "0.08em" }}
                  >
                    Without cplt
                  </p>
                </div>
                <div className="px-4 py-4 flex items-center justify-center">
                  <p
                    className="font-semibold uppercase tracking-wider"
                    style={{ color: "rgba(255,255,255,0.5)", fontSize: "0.7rem", margin: 0, letterSpacing: "0.08em" }}
                  >
                    With cplt
                  </p>
                </div>
              </div>

              {/* Rows */}
              {SECURITY_TABLE.map((row, i) => (
                <div
                  key={row.resource}
                  className="grid gap-0"
                  style={{
                    gridTemplateColumns: "3fr 1fr 1fr",
                    borderTop: "1px solid #e2e8f0",
                  }}
                >
                  <div
                    className="px-6 py-3.5 flex items-center"
                    style={{
                      borderRight: "1px solid #e2e8f0",
                      background: i % 2 === 0 ? "#f8fafc" : "white",
                    }}
                  >
                    <p style={{ color: "#1e293b", fontSize: "0.875rem", margin: 0 }}>{row.resource}</p>
                  </div>
                  {/* Without cplt column */}
                  <div
                    className="px-4 py-3.5 flex items-center justify-center gap-1.5"
                    style={{
                      borderRight: "1px solid #e2e8f0",
                      background:
                        row.without === "exposed"
                          ? i % 2 === 0
                            ? "#fef2f2"
                            : "#fff5f5"
                          : i % 2 === 0
                            ? "#f0fdf4"
                            : "#f7fef9",
                    }}
                  >
                    {row.without === "exposed" ? (
                      <>
                        <span style={{ color: "#dc2626", fontSize: "0.8rem" }} aria-hidden>
                          ⚠
                        </span>
                        <p style={{ color: "#dc2626", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>Exposed</p>
                      </>
                    ) : (
                      <>
                        <CheckmarkCircleIcon
                          fontSize="0.875rem"
                          style={{ color: "#22c55e", flexShrink: 0 }}
                          aria-hidden
                        />
                        <p style={{ color: "#166534", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>Allowed</p>
                      </>
                    )}
                  </div>
                  {/* With cplt column */}
                  <div
                    className="px-4 py-3.5 flex items-center justify-center gap-1.5"
                    style={{
                      background:
                        row.with === "blocked"
                          ? i % 2 === 0
                            ? "rgba(16, 185, 129, 0.06)"
                            : "rgba(16, 185, 129, 0.03)"
                          : row.with === "filtered"
                            ? i % 2 === 0
                              ? "rgba(234, 179, 8, 0.06)"
                              : "rgba(234, 179, 8, 0.03)"
                            : i % 2 === 0
                              ? "#f0fdf4"
                              : "#f7fef9",
                    }}
                  >
                    {row.with === "blocked" ? (
                      <>
                        <ShieldLockIcon fontSize="0.875rem" style={{ color: "#10b981", flexShrink: 0 }} aria-hidden />
                        <p style={{ color: "#10b981", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>Protected</p>
                      </>
                    ) : row.with === "filtered" ? (
                      <>
                        <CloudIcon fontSize="0.875rem" style={{ color: "#d97706", flexShrink: 0 }} aria-hidden />
                        <p style={{ color: "#d97706", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>Filtered*</p>
                      </>
                    ) : (
                      <>
                        <CheckmarkCircleIcon
                          fontSize="0.875rem"
                          style={{ color: "#22c55e", flexShrink: 0 }}
                          aria-hidden
                        />
                        <p style={{ color: "#166534", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>Allowed</p>
                      </>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>

          <p className="text-center" style={{ color: "#64748b", fontSize: "0.8125rem", margin: 0 }}>
            *Routed through CONNECT proxy — telemetry and non-allowlisted domains are blocked.
            <br />
            All blocks are enforced by the operating system kernel. No userspace bypass is possible.
          </p>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Protections ---------- */

function ProtectionsSection() {
  return (
    <section className="dark-section" style={{ background: "#0c1a14", color: "white" }}>
      <Box
        paddingBlock={{ xs: "space-16", md: "space-32" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3" style={{ color: "white" }}>
              Your agent sees the code, not your secrets.
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#94a3b8", marginBlock: 0, textAlign: "center" }}>
              Four layers of kernel-enforced protection — no userspace bypass possible.
            </p>
          </div>

          <HGrid columns={{ xs: 1, sm: 2 }} gap="space-16">
            {PROTECTIONS.map((p) => (
              <div
                key={p.title}
                className="rounded-xl flex items-start gap-4"
                style={{
                  padding: "1.25rem",
                  background: "rgba(255,255,255,0.03)",
                  border: "1px solid rgba(255,255,255,0.06)",
                }}
              >
                <div
                  className="flex items-center justify-center rounded-lg shrink-0"
                  style={{
                    width: "2.5rem",
                    height: "2.5rem",
                    background: `${p.color}15`,
                    border: `1px solid ${p.color}30`,
                  }}
                >
                  <p.Icon fontSize="1.25rem" style={{ color: p.color }} aria-hidden />
                </div>
                <div>
                  <p className="font-semibold mb-1" style={{ color: "white", fontSize: "0.9rem", margin: 0 }}>
                    {p.title}
                  </p>
                  <p style={{ color: "#94a3b8", fontSize: "0.8125rem", lineHeight: 1.6, margin: 0 }}>{p.description}</p>
                </div>
              </div>
            ))}
          </HGrid>
        </VStack>
      </Box>

      {/* Gradient transition to light */}
      <div
        className="h-40"
        style={{
          background: `linear-gradient(to bottom,
            #0c1a14 0%,
            #0f1f18 8%,
            #14281f 16%,
            #1c3529 26%,
            #2a4a3a 36%,
            #436b56 48%,
            #6a9478 58%,
            #95b8a2 68%,
            #bdd4c6 78%,
            #dde9e1 87%,
            #eef4f0 93%,
            #f8fafc 100%
          )`,
        }}
      />
    </section>
  );
}

/* ---------- Proxy & Network ---------- */

function ProxySection() {
  return (
    <section style={{ background: "#f8fafc" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Network proxy
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              All outbound traffic routes through a local CONNECT proxy. Block, allow, or audit — your choice.
            </p>
          </div>

          {/* SVG network flow diagram */}
          <div className="w-full overflow-x-auto">
            <svg
              viewBox="0 0 820 300"
              className="w-full"
              style={{ maxWidth: "820px", margin: "0 auto", display: "block" }}
              role="img"
              aria-label="Network proxy flow diagram showing how cplt routes and filters outbound traffic"
            >
              {/* Background */}
              <rect width="820" height="300" rx="12" fill="white" stroke="#e2e8f0" strokeWidth="1" />

              {/* Sandbox container (wraps agent + proxy) */}
              <rect
                x="20"
                y="20"
                width="440"
                height="260"
                rx="10"
                fill="#f0fdf4"
                stroke="#10b981"
                strokeWidth="1.5"
                strokeDasharray="6 3"
              />
              <text x="40" y="42" fill="#166534" fontSize="10" fontWeight="600">
                cplt sandbox
              </text>

              {/* Agent box (same height as proxy: y=70, h=160) */}
              <rect x="45" y="70" width="120" height="160" rx="8" fill="#0c1a14" />
              <text
                x="105"
                y="145"
                textAnchor="middle"
                fill="#4ade80"
                fontSize="11"
                fontWeight="600"
                fontFamily="monospace"
              >
                AI Agent
              </text>
              <text x="105" y="165" textAnchor="middle" fill="#94a3b8" fontSize="9" fontFamily="monospace">
                curl, fetch, git
              </text>

              {/* Arrow: Agent → Proxy */}
              <line x1="165" y1="150" x2="225" y2="150" stroke="#94a3b8" strokeWidth="2" markerEnd="url(#arrowGray)" />

              {/* Proxy box */}
              <rect x="225" y="70" width="215" height="160" rx="8" fill="white" stroke="#10b981" strokeWidth="1.5" />
              <text x="332" y="95" textAnchor="middle" fill="#166534" fontSize="12" fontWeight="700">
                CONNECT Proxy
              </text>
              <text x="332" y="112" textAnchor="middle" fill="#64748b" fontSize="9">
                localhost:ephemeral
              </text>

              {/* Proxy checks */}
              <rect x="250" y="122" width="165" height="22" rx="4" fill="#f1f5f9" />
              <text x="332" y="137" textAnchor="middle" fill="#475569" fontSize="9">
                Blocklist / Allowlist
              </text>
              <rect x="250" y="150" width="165" height="22" rx="4" fill="#f1f5f9" />
              <text x="332" y="165" textAnchor="middle" fill="#475569" fontSize="9">
                Private IP filter
              </text>
              <rect x="250" y="178" width="165" height="22" rx="4" fill="#f1f5f9" />
              <text x="332" y="193" textAnchor="middle" fill="#475569" fontSize="9">
                DNS rebinding protection
              </text>

              {/* Audit log below proxy */}
              <rect x="268" y="210" width="130" height="32" rx="6" fill="#fffbeb" stroke="#d97706" strokeWidth="1" />
              <text x="333" y="230" textAnchor="middle" fill="#92400e" fontSize="9" fontWeight="600">
                Audit log ✓
              </text>

              {/* Arrow: Proxy → Internet (allowed path) */}
              <line x1="440" y1="115" x2="540" y2="85" stroke="#22c55e" strokeWidth="2" markerEnd="url(#arrowGreen)" />
              <text x="500" y="88" textAnchor="middle" fill="#166534" fontSize="9" fontWeight="600">
                ✓ Allowed
              </text>

              {/* Internet box */}
              <rect x="540" y="55" width="250" height="70" rx="8" fill="#f0fdf4" stroke="#22c55e" strokeWidth="1" />
              <text x="665" y="78" textAnchor="middle" fill="#166534" fontSize="11" fontWeight="600">
                Internet
              </text>
              <text x="665" y="95" textAnchor="middle" fill="#64748b" fontSize="9">
                github.com, npm, PyPI, api.openai.com
              </text>
              <text x="665" y="111" textAnchor="middle" fill="#64748b" fontSize="8">
                Allowlisted or not in blocklist
              </text>

              {/* Arrow: Proxy → Dropped (blocked path) */}
              <line x1="440" y1="170" x2="540" y2="200" stroke="#dc2626" strokeWidth="2" markerEnd="url(#arrowRed)" />
              <text x="500" y="198" textAnchor="middle" fill="#dc2626" fontSize="9" fontWeight="600">
                ✗ Blocked
              </text>

              {/* Dropped box */}
              <rect x="540" y="175" width="250" height="70" rx="8" fill="#fef2f2" stroke="#dc2626" strokeWidth="1" />
              <text x="665" y="198" textAnchor="middle" fill="#dc2626" fontSize="11" fontWeight="600">
                Dropped
              </text>
              <text x="665" y="215" textAnchor="middle" fill="#64748b" fontSize="9">
                webhook.site, ngrok.io, pastebin.com
              </text>
              <text x="665" y="231" textAnchor="middle" fill="#64748b" fontSize="8">
                169.254.x.x, 10.x.x.x, tunneling services
              </text>

              {/* Config labels along bottom */}
              <text x="540" y="275" fill="#475569" fontSize="8.5" fontFamily="monospace">
                proxy.blocked_domains
              </text>
              <text x="540" y="289" fill="#64748b" fontSize="7.5">
                ~70 domains · hot-reload every 5s
              </text>

              <text x="700" y="275" fill="#475569" fontSize="8.5" fontFamily="monospace">
                proxy.allowed_domains
              </text>
              <text x="700" y="289" fill="#64748b" fontSize="7.5">
                Fail-closed strict mode
              </text>

              {/* Arrow markers */}
              <defs>
                <marker id="arrowGray" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="#94a3b8" />
                </marker>
                <marker id="arrowGreen" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="#22c55e" />
                </marker>
                <marker id="arrowRed" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="#dc2626" />
                </marker>
              </defs>
            </svg>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Guards ---------- */

const GH_GUARD_LEVELS = [
  { level: "Read", examples: "gh issue list, gh pr view", access: "Allowed", color: "#22c55e" },
  { level: "Write", examples: "gh pr create, gh issue edit", access: "Own repo only", color: "#f59e0b" },
  { level: "Destructive", examples: "gh repo delete, gh pr merge", access: "Always blocked", color: "#dc2626" },
];

function GuardsSection() {
  return (
    <section style={{ background: "white" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              gh guard &amp; git guard
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Block destructive GitHub and git operations. The agent can commit and branch — but not push to main or
              merge PRs.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            {/* gh guard */}
            <div
              className="rounded-xl overflow-hidden"
              style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
            >
              <div className="px-5 py-4" style={{ borderBottom: "1px solid #e2e8f0" }}>
                <Heading size="xsmall" level="3">
                  gh guard — three-tier policy
                </Heading>
                <p style={{ color: "#64748b", fontSize: "0.8125rem", margin: "0.25rem 0 0" }}>
                  Default-deny engine classifying 150+ <code style={{ fontSize: "0.75rem" }}>gh</code> commands.
                </p>
              </div>
              <div>
                {GH_GUARD_LEVELS.map((row, i) => (
                  <div
                    key={row.level}
                    className="flex items-center gap-3 px-5 py-3"
                    style={{ borderTop: i > 0 ? "1px solid #f1f5f9" : undefined }}
                  >
                    <div
                      className="rounded-full shrink-0"
                      style={{ width: "8px", height: "8px", background: row.color }}
                    />
                    <div className="flex-1 min-w-0">
                      <p className="font-semibold" style={{ color: "#1e293b", fontSize: "0.8125rem", margin: 0 }}>
                        {row.level}
                      </p>
                      <p style={{ color: "#94a3b8", fontSize: "0.75rem", margin: 0 }}>{row.examples}</p>
                    </div>
                    <span
                      className="shrink-0 font-semibold rounded-full px-2.5 py-0.5"
                      style={{ fontSize: "0.6875rem", background: `${row.color}15`, color: row.color }}
                    >
                      {row.access}
                    </span>
                  </div>
                ))}
              </div>
              <div className="px-5 py-3" style={{ background: "#f8fafc", borderTop: "1px solid #e2e8f0" }}>
                <p style={{ color: "#64748b", fontSize: "0.75rem", margin: 0 }}>
                  <code style={{ fontSize: "0.6875rem" }}>gh api</code> calls restricted to{" "}
                  <code style={{ fontSize: "0.6875rem" }}>/repos/&#123;current-repo&#125;/...</code>
                </p>
              </div>
            </div>

            {/* git guard + enable commands */}
            <VStack gap="space-16">
              <div
                className="rounded-xl overflow-hidden"
                style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
              >
                <div className="px-5 py-4" style={{ borderBottom: "1px solid #e2e8f0" }}>
                  <Heading size="xsmall" level="3">
                    git guard — push protection
                  </Heading>
                  <p style={{ color: "#64748b", fontSize: "0.8125rem", margin: "0.25rem 0 0" }}>
                    Blocks <code style={{ fontSize: "0.75rem" }}>push</code>,{" "}
                    <code style={{ fontSize: "0.75rem" }}>request-pull</code>, and{" "}
                    <code style={{ fontSize: "0.75rem" }}>send-pack</code>. Commit, branch, rebase — all fine.
                  </p>
                </div>
                <div className="px-5 py-3" style={{ background: "#f8fafc" }}>
                  <p style={{ color: "#64748b", fontSize: "0.75rem", margin: 0 }}>
                    Feature branches only?{" "}
                    <code style={{ fontSize: "0.6875rem" }}>
                      cplt config set git_guard.protect_default_branch_only true
                    </code>
                  </p>
                </div>
              </div>

              {/* Enable commands */}
              <div
                className="rounded-xl overflow-hidden"
                style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
              >
                <div
                  className="flex items-center justify-between px-4 py-2"
                  style={{ background: "#1e1e1e", borderBottom: "1px solid #333" }}
                >
                  <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                    Enable guards
                  </span>
                  <CopyButton
                    copyText="cplt config set gh_guard.enabled true && cplt config set git_guard.enabled true"
                    size="xsmall"
                    style={{ color: "white" }}
                  />
                </div>
                <pre
                  className="p-4 font-mono leading-relaxed overflow-x-auto"
                  style={{ margin: 0, fontSize: "0.75rem", color: "#d4d4d4", background: "#1e1e1e" }}
                >
                  <span style={{ color: "#4ade80" }}>$</span>
                  {" cplt config set gh_guard.enabled true\n"}
                  <span style={{ color: "#4ade80" }}>$</span>
                  {" cplt config set git_guard.enabled true"}
                </pre>
              </div>

              {/* What agent sees */}
              <div
                className="rounded-xl overflow-hidden"
                style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
              >
                <div
                  className="flex items-center justify-between px-4 py-2"
                  style={{ background: "#1e1e1e", borderBottom: "1px solid #333" }}
                >
                  <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                    What the agent sees
                  </span>
                </div>
                <pre
                  className="p-4 font-mono leading-relaxed overflow-x-auto"
                  style={{ margin: 0, fontSize: "0.7rem", color: "#d4d4d4", background: "#1e1e1e" }}
                >
                  <span style={{ color: "#f87171" }}>⛔ sandbox restriction:</span>
                  {" `gh pr merge` is not allowed.\n"}
                  {"This command is classified as destructive\n"}
                  {"and blocked by gh guard.\n\n"}
                  <span style={{ color: "#94a3b8" }}>
                    Please note this for the human operator{"\n"}and continue with your remaining work.
                  </span>
                </pre>
              </div>
            </VStack>
          </HGrid>

          <p className="text-center" style={{ color: "#64748b", fontSize: "0.8125rem", margin: 0 }}>
            Start with <code style={{ fontSize: "0.75rem" }}>--gh-guard --git-guard</code> flags for a single run, or{" "}
            <code style={{ fontSize: "0.75rem" }}>mode: audit</code> to observe before enforcing.
          </p>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Multi-Agent ---------- */

function MultiAgentSection() {
  return (
    <section style={{ background: "#f8fafc" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Multi-agent support
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Same sandbox, different agents. Choose the AI that fits your workflow.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
            {AGENTS.map((agent) => (
              <div
                key={agent.name}
                className="rounded-xl overflow-hidden flex flex-col h-full"
                style={{
                  background: "white",
                  border: "1px solid #e2e8f0",
                  boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
                }}
              >
                <div style={{ height: "3px", background: agent.color }} />
                <Box padding={{ xs: "space-16", md: "space-20" }} className="flex-1 flex flex-col">
                  <div className="flex items-center gap-3 mb-3">
                    <div
                      className="flex items-center justify-center rounded-lg"
                      style={{
                        width: "2.25rem",
                        height: "2.25rem",
                        background: `${agent.color}15`,
                        border: `1px solid ${agent.color}30`,
                      }}
                    >
                      <agent.Icon fontSize="1.125rem" style={{ color: agent.color }} aria-hidden />
                    </div>
                    <Heading size="xsmall" level="3">
                      {agent.name}
                    </Heading>
                  </div>
                  <p
                    className="flex-1"
                    style={{ color: "#64748b", fontSize: "0.8125rem", lineHeight: 1.6, margin: "0 0 0.75rem" }}
                  >
                    {agent.description}
                  </p>
                  <div
                    className="rounded-lg overflow-x-auto flex items-center gap-2"
                    style={{ background: "#1e1e1e", padding: "0.5rem 0.75rem" }}
                  >
                    <code
                      className="font-mono whitespace-nowrap flex-1"
                      style={{ fontSize: "0.75rem", color: "#d4d4d4" }}
                    >
                      {agent.command}
                    </code>
                    <CopyButton copyText={agent.command} size="xsmall" style={{ color: "white" }} />
                  </div>
                </Box>
              </div>
            ))}
          </HGrid>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Team Configuration ---------- */

function TeamConfigSection() {
  return (
    <section style={{ background: "white" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Team configuration
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Commit sandbox settings to your repo. Everyone gets the right config automatically.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            {/* TOML example */}
            <div
              className="rounded-xl overflow-hidden"
              style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
            >
              <div
                className="flex items-center justify-between px-4 py-2"
                style={{ background: "#1e1e1e", borderBottom: "1px solid #333" }}
              >
                <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                  .cplt.toml
                </span>
                <CopyButton
                  copyText={`[deny]\nenv = ["VAULT_TOKEN", "NPM_TOKEN"]\n\n[propose]\nallow_localhost_any = true\n\n[propose.allow]\nports = [5432]\nlocalhost = [3000]`}
                  size="xsmall"
                  style={{ color: "white" }}
                />
              </div>
              <pre
                className="p-4 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: "0.75rem", color: "#d4d4d4", background: "#1e1e1e" }}
              >
                <span style={{ color: "#94a3b8" }}># Tighten automatically — no approval needed</span>
                {"\n"}
                <span style={{ color: "#569cd6" }}>[deny]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>env</span>
                <span style={{ color: "#d4d4d4" }}> = [</span>
                <span style={{ color: "#ce9178" }}>&quot;VAULT_TOKEN&quot;</span>
                <span style={{ color: "#d4d4d4" }}>, </span>
                <span style={{ color: "#ce9178" }}>&quot;NPM_TOKEN&quot;</span>
                <span style={{ color: "#d4d4d4" }}>]</span>
                {"\n\n"}
                <span style={{ color: "#94a3b8" }}># Propose — requires `cplt trust` approval</span>
                {"\n"}
                <span style={{ color: "#569cd6" }}>[propose]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>allow_localhost_any</span>
                <span style={{ color: "#d4d4d4" }}> = </span>
                <span style={{ color: "#569cd6" }}>true</span>
                {"\n\n"}
                <span style={{ color: "#569cd6" }}>[propose.allow]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>ports</span>
                <span style={{ color: "#d4d4d4" }}> = [</span>
                <span style={{ color: "#b5cea8" }}>5432</span>
                <span style={{ color: "#d4d4d4" }}>]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>localhost</span>
                <span style={{ color: "#d4d4d4" }}> = [</span>
                <span style={{ color: "#b5cea8" }}>3000</span>
                <span style={{ color: "#d4d4d4" }}>]</span>
              </pre>
            </div>

            {/* Explanation */}
            <VStack gap="space-16">
              <div className="flex items-start gap-3">
                <div
                  className="flex items-center justify-center rounded-lg shrink-0 mt-0.5"
                  style={{
                    width: "2.25rem",
                    height: "2.25rem",
                    background: "#10b98115",
                    border: "1px solid #10b98130",
                  }}
                >
                  <ShieldLockIcon fontSize="1.125rem" style={{ color: "#10b981" }} aria-hidden />
                </div>
                <div>
                  <p className="font-semibold" style={{ color: "#0f172a", fontSize: "0.875rem", margin: 0 }}>
                    [deny] — auto-applied
                  </p>
                  <p style={{ color: "#64748b", fontSize: "0.8125rem", lineHeight: 1.6, margin: "0.25rem 0 0" }}>
                    Can only tighten the sandbox. Block env vars, deny file paths — no approval needed.
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div
                  className="flex items-center justify-center rounded-lg shrink-0 mt-0.5"
                  style={{
                    width: "2.25rem",
                    height: "2.25rem",
                    background: "#3b82f615",
                    border: "1px solid #3b82f630",
                  }}
                >
                  <CheckmarkCircleIcon fontSize="1.125rem" style={{ color: "#3b82f6" }} aria-hidden />
                </div>
                <div>
                  <p className="font-semibold" style={{ color: "#0f172a", fontSize: "0.875rem", margin: 0 }}>
                    [propose] — requires approval
                  </p>
                  <p style={{ color: "#64748b", fontSize: "0.8125rem", lineHeight: 1.6, margin: "0.25rem 0 0" }}>
                    Request additional permissions. Each developer approves with{" "}
                    <code style={{ fontSize: "0.75rem" }}>cplt trust accept --all</code>. Content-pinned — any change
                    invalidates the approval.
                  </p>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div
                  className="flex items-center justify-center rounded-lg shrink-0 mt-0.5"
                  style={{
                    width: "2.25rem",
                    height: "2.25rem",
                    background: "#f59e0b15",
                    border: "1px solid #f59e0b30",
                  }}
                >
                  <WrenchIcon fontSize="1.125rem" style={{ color: "#f59e0b" }} aria-hidden />
                </div>
                <div>
                  <p className="font-semibold" style={{ color: "#0f172a", fontSize: "0.875rem", margin: 0 }}>
                    Tamper-proof
                  </p>
                  <p style={{ color: "#64748b", fontSize: "0.8125rem", lineHeight: 1.6, margin: "0.25rem 0 0" }}>
                    Read from git HEAD, not the working tree. The agent cannot modify its own sandbox config.
                  </p>
                </div>
              </div>
            </VStack>
          </HGrid>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- cplt init ---------- */

const ECOSYSTEMS = [
  { name: "JVM", detail: "Gradle / Maven", color: "#f97316" },
  { name: "Node.js", detail: "npm / pnpm", color: "#22c55e" },
  { name: "Docker", detail: "Compose", color: "#2563eb" },
  { name: "Python", detail: "pip / uv", color: "#eab308" },
  { name: "Spring Boot", detail: "8080 + PG", color: "#22c55e" },
  { name: "Ktor", detail: "8080", color: "#a855f7" },
  { name: "Next.js", detail: "3000", color: "#0ea5e9" },
  { name: "Flyway", detail: "PG 5432", color: "#dc2626" },
  { name: "Playwright", detail: "browsers", color: "#f472b6" },
  { name: "Rust / Go", detail: "defaults", color: "#94a3b8" },
];

function InitSection() {
  return (
    <section className="dark-section" style={{ background: "#0c1a14", color: "white" }}>
      <Box
        paddingBlock={{ xs: "space-16", md: "space-32" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3" style={{ color: "white" }}>
              Auto-detect your project
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#94a3b8", marginBlock: 0, textAlign: "center" }}>
              <code style={{ color: "#4ade80", fontSize: "0.875rem" }}>cplt init</code> scans your project for build
              files, frameworks, and patterns — then generates the right{" "}
              <code style={{ color: "#4ade80", fontSize: "0.875rem" }}>.cplt.toml</code> automatically.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            {/* Terminal mock */}
            <div className="rounded-xl overflow-hidden" style={{ border: "1px solid rgba(255,255,255,0.1)" }}>
              <div
                className="flex items-center gap-2 px-4 py-2"
                style={{ background: "#161b22", borderBottom: "1px solid rgba(255,255,255,0.06)" }}
              >
                <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                  $ cplt init
                </span>
              </div>
              <pre
                className="p-4 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: "0.7rem", color: "#d4d4d4", background: "#0d1117" }}
              >
                <span style={{ color: "#4ade80" }}>Detected:</span>
                {"\n"}
                {"  "}
                <span style={{ color: "#58a6ff" }}>Spring Boot</span>
                {"  application.yml + spring-boot-starter\n"}
                {"  "}
                <span style={{ color: "#58a6ff" }}>Flyway</span>
                {"       db/migration/ directory\n"}
                {"  "}
                <span style={{ color: "#58a6ff" }}>Docker</span>
                {"       Dockerfile + compose.yml\n"}
                {"  "}
                <span style={{ color: "#58a6ff" }}>Gradle</span>
                {"       build.gradle.kts\n"}
                {"  "}
                <span style={{ color: "#58a6ff" }}>.env</span>
                {"         .env.example found\n"}
                {"\n"}
                <span style={{ color: "#4ade80" }}>Generated .cplt.toml:</span>
                {"\n\n"}
                <span style={{ color: "#94a3b8" }}># Deny access to sensitive env vars</span>
                {"\n"}
                <span style={{ color: "#569cd6" }}>[deny]</span>
                {"\n"}
                {"env = ["}
                <span style={{ color: "#ce9178" }}>&quot;DB_PASSWORD&quot;</span>
                {", "}
                <span style={{ color: "#ce9178" }}>&quot;API_KEY&quot;</span>
                {"]\n\n"}
                <span style={{ color: "#569cd6" }}>[propose]</span>
                {"\n"}
                {"allow_jvm_attach = "}
                <span style={{ color: "#569cd6" }}>true</span>
                {"\n\n"}
                <span style={{ color: "#569cd6" }}>[propose.allow]</span>
                {"\n"}
                {"ports = ["}
                <span style={{ color: "#b5cea8" }}>5432</span>
                {"]\n"}
                {"localhost = ["}
                <span style={{ color: "#b5cea8" }}>8080</span>
                {"]\n\n"}
                <span style={{ color: "#f59e0b" }}>⚠ allow_docker</span>
                {"  Docker detected — grants broad access\n\n"}
                <span style={{ color: "#94a3b8" }}>Run </span>
                <span style={{ color: "#4ade80" }}>cplt init --write</span>
                <span style={{ color: "#94a3b8" }}> to save</span>
              </pre>
            </div>

            {/* Ecosystems grid */}
            <VStack gap="space-16">
              <div>
                <p className="font-semibold mb-2" style={{ color: "white", fontSize: "0.875rem", margin: 0 }}>
                  15 ecosystem detectors
                </p>
                <p style={{ color: "#94a3b8", fontSize: "0.8125rem", lineHeight: 1.6, margin: "0.25rem 0 0" }}>
                  Each detector knows which sandbox permissions the ecosystem needs. Dangerous permissions get risk
                  warnings.
                </p>
              </div>

              <div className="flex flex-wrap gap-2">
                {ECOSYSTEMS.map((eco) => (
                  <div
                    key={eco.name}
                    className="rounded-lg flex items-center gap-2"
                    style={{
                      padding: "0.375rem 0.75rem",
                      background: "rgba(255,255,255,0.04)",
                      border: "1px solid rgba(255,255,255,0.08)",
                    }}
                  >
                    <div className="rounded-full" style={{ width: "6px", height: "6px", background: eco.color }} />
                    <span style={{ color: "white", fontSize: "0.75rem", fontWeight: 600 }}>{eco.name}</span>
                    <span style={{ color: "#64748b", fontSize: "0.6875rem" }}>{eco.detail}</span>
                  </div>
                ))}
              </div>

              <div
                className="rounded-lg flex items-start gap-3"
                style={{
                  padding: "0.75rem 1rem",
                  background: "rgba(255,255,255,0.03)",
                  border: "1px solid rgba(255,255,255,0.06)",
                }}
              >
                <MagnifyingGlassIcon fontSize="1rem" style={{ color: "#94a3b8", marginTop: "0.125rem" }} aria-hidden />
                <div>
                  <p className="font-semibold" style={{ color: "white", fontSize: "0.8125rem", margin: 0 }}>
                    Personal config with --global
                  </p>
                  <p style={{ color: "#94a3b8", fontSize: "0.75rem", lineHeight: 1.5, margin: "0.25rem 0 0" }}>
                    Detects Gradle wrapper, Playwright browsers, GPG signing, and alternative agents on your machine.
                    Writes to <code style={{ fontSize: "0.6875rem" }}>~/.config/cplt/config.toml</code>.
                  </p>
                </div>
              </div>
            </VStack>
          </HGrid>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Configuration ---------- */

function ConfigSection({ configKeys }: { configKeys: import("@/lib/cplt-config").CpltConfigKey[] }) {
  return (
    <section style={{ background: "white" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Configuration
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Every option explained. Search by name or description.
            </p>
          </div>

          <CpltConfigExplorer configKeys={configKeys} />
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- How It Works ---------- */

function HowItWorksSection() {
  const steps = [
    {
      title: "Install",
      command: "brew install navikt/tap/cplt",
      description: "One command via Homebrew (macOS). Linux: see install script.",
      Icon: TerminalIcon,
      color: "#10b981",
    },
    {
      title: "Configure",
      command: "cplt init --write",
      description: "Detect your project's tooling and generate sandbox config.",
      Icon: MagnifyingGlassIcon,
      color: "#34d399",
    },
    {
      title: "Run Your Agent",
      command: 'cplt -- -p "fix the tests"',
      description: "Your agent works normally — but secrets are invisible.",
      Icon: ShieldLockIcon,
      color: "#4ade80",
    },
  ];

  return (
    <section style={{ background: "white" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              How it works
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Three steps from zero to sandboxed agent.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
            {steps.map((step, i) => (
              <div
                key={step.title}
                className="rounded-xl overflow-hidden flex flex-col"
                style={{
                  background: "#f8fafc",
                  border: "1px solid #e2e8f0",
                  boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
                }}
              >
                <div style={{ height: "3px", background: step.color }} />
                <Box padding={{ xs: "space-16", md: "space-20" }} className="flex-1 flex flex-col">
                  <div className="flex flex-col items-center text-center flex-1">
                    <div
                      className="flex items-center justify-center rounded-full mb-2"
                      style={{
                        width: "2.5rem",
                        height: "2.5rem",
                        background: `${step.color}18`,
                        border: `1.5px solid ${step.color}40`,
                      }}
                    >
                      <step.Icon fontSize="1.25rem" style={{ color: step.color }} aria-hidden />
                    </div>
                    <p
                      className="font-bold"
                      style={{ color: "#94a3b8", fontSize: "0.75rem", margin: 0, letterSpacing: "0.05em" }}
                    >
                      STEP {i + 1}
                    </p>
                    <Heading size="xsmall" level="3" style={{ marginTop: "0.25rem" }}>
                      {step.title}
                    </Heading>
                    <div
                      className="rounded-lg w-full overflow-x-auto flex items-center gap-2 mt-3"
                      style={{ background: "#1e1e1e", padding: "0.5rem 0.75rem" }}
                    >
                      <code
                        className="font-mono whitespace-nowrap flex-1"
                        style={{ fontSize: "0.75rem", color: "#d4d4d4" }}
                      >
                        {step.command}
                      </code>
                      <CopyButton copyText={step.command} size="xsmall" style={{ color: "white" }} />
                    </div>
                    <p
                      style={{
                        color: "#64748b",
                        fontSize: "0.8125rem",
                        lineHeight: 1.5,
                        margin: "0.75rem 0 0",
                        textAlign: "center",
                      }}
                    >
                      {step.description}
                    </p>
                  </div>
                </Box>
              </div>
            ))}
          </HGrid>

          {/* Shell setup tip */}
          <VStack gap="space-8" className="max-w-2xl mx-auto w-full">
            <Heading size="xsmall" level="3" className="text-center">
              Make it the default
            </Heading>
            <p className="text-center" style={{ color: "#64748b", fontSize: "0.8125rem", margin: 0 }}>
              Run <code style={{ fontSize: "0.8rem" }}>cplt --shell-install</code> so{" "}
              <code style={{ fontSize: "0.8rem" }}>copilot</code> always runs sandboxed.
            </p>
            <div
              className="rounded-xl w-full overflow-hidden"
              style={{ border: "1px solid #e2e8f0", boxShadow: "0 1px 3px rgba(0,0,0,0.04)" }}
            >
              <div
                className="flex items-center justify-between px-4 py-2"
                style={{ background: "#1e1e1e", borderBottom: "1px solid #333" }}
              >
                <span className="font-mono" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                  $ cplt --shell-install
                </span>
                <CopyButton copyText="cplt --shell-install" size="xsmall" style={{ color: "white" }} />
              </div>
              <pre
                className="p-4 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: "0.75rem", color: "#d4d4d4", background: "#1e1e1e" }}
              >
                <span style={{ color: "#4ade80" }}>✓</span>
                {" Added to ~/.zshrc\n"}
                <span style={{ color: "#4ade80" }}>✓</span> <span style={{ color: "#94a3b8" }}>copilot</span>
                {" → "}
                <span style={{ color: "#4ade80" }}>cplt</span>
                {" (sandboxed)\n\n"}
                <span style={{ color: "#94a3b8" }}>Restart your shell or: </span>
                <span style={{ color: "#d4d4d4" }}>source ~/.zshrc</span>
              </pre>
            </div>
          </VStack>

          <div className="flex flex-col items-center gap-3">
            <NextLink
              href="https://github.com/navikt/cplt"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-lg font-medium no-underline transition-all"
              style={{
                background: "linear-gradient(135deg, #10b981, #059669)",
                color: "white",
                fontSize: "0.9rem",
              }}
            >
              View on GitHub →
            </NextLink>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Footer ---------- */

function FooterSection() {
  return (
    <section
      className="dark-section"
      style={{
        background: "linear-gradient(165deg, #0a0f0c 0%, #0d2118 50%, #0a1f14 100%)",
        color: "white",
      }}
    >
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", md: "space-32" }}
        className="max-w-7xl mx-auto text-center"
      >
        <VStack gap="space-16" className="items-center">
          <Heading size="small" level="2" style={{ color: "white" }}>
            Trust the kernel, not the agent.
          </Heading>
          <p
            className="max-w-lg"
            style={{
              color: "rgba(255,255,255,0.5)",
              fontSize: "1.25rem",
              lineHeight: 1.7,
              marginBlock: 0,
              textAlign: "center",
              fontStyle: "italic",
            }}
          >
            Open source. MIT licensed.
          </p>
          <div className="flex flex-wrap gap-6 justify-center" style={{ fontSize: "0.875rem" }}>
            <NextLink
              href="https://github.com/navikt/cplt"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              GitHub
            </NextLink>
            <NextLink
              href="https://github.com/navikt/cplt/blob/main/SECURITY.md"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              Security Policy
            </NextLink>
            <NextLink
              href="https://github.com/navikt/cplt/blob/main/LICENSE"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              MIT License
            </NextLink>
          </div>
        </VStack>
      </Box>
    </section>
  );
}
