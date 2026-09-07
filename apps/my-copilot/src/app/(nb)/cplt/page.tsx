import type { Metadata } from "next";
import { Box, VStack, HGrid, Heading, CopyButton, BodyShort, BodyLong, Theme } from "@navikt/ds-react";
import NextLink from "next/link";
import { CpltConfigExplorer } from "@/components/cplt-config-explorer";
import { fetchCpltConfigKeys } from "@/lib/cplt-config";
import {
  ShieldLockIcon,
  TerminalIcon,
  CloudIcon,
  CheckmarkCircleIcon,
  WrenchIcon,
  MagnifyingGlassIcon,
} from "@navikt/aksel-icons";

const PAGE_TITLE = "cplt: Sandbox for AI coding agents";
const PAGE_DESCRIPTION =
  "cplt runs GitHub Copilot CLI, OpenCode, Antigravity, Pi, Claude Code, goose or a plain shell inside a kernel-level sandbox, so the agent can work on your project but cannot read your secrets.";

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  openGraph: { title: PAGE_TITLE, description: PAGE_DESCRIPTION, type: "website" },
  twitter: { card: "summary_large_image", title: PAGE_TITLE, description: PAGE_DESCRIPTION },
};

/* ---------- Design tokens ---------- */

/* Two brand colours, declared once in globals.css: the dark ground and the green
   accent. --cplt-accent-ink is that same green darkened for text on light. */
const GROUND = "var(--cplt-ground)";
const ACCENT = "var(--cplt-accent)";
const ACCENT_INK = "var(--cplt-accent-ink)";

/* One padding rhythm for every section, light and dark alike. */
const SECTION_PADDING_BLOCK = { xs: "space-24", md: "space-40" } as const;
const SECTION_PADDING_INLINE = { xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" } as const;
const SECTION_GAP = { xs: "space-16", md: "space-24" } as const;

/* Three type sizes: 1.125rem lead (Aksel "large"), 0.875rem body (Aksel "small"),
   0.75rem for terminal and code. */
const CODE_SIZE = "0.75rem";

const CARD_STYLE = {
  background: "var(--ax-bg-default)",
  border: "1px solid var(--ax-border-neutral-subtle)",
  boxShadow: "0 1px 3px rgba(0, 0, 0, 0.04)",
} as const;

const TERMINAL_BG = "#1e1e1e";
const TERMINAL_FG = "#d4d4d4";
const TERMINAL_MUTED = "#a5acb6";

/* ---------- Data ---------- */

const INSTALL_COMMAND = "brew install navikt/tap/cplt";
const INSTALL_SCRIPT = "curl -fsSL https://raw.githubusercontent.com/navikt/cplt/main/install.sh | bash";

const ARTICLE_HREF = "/en/news/sandbox-confines-the-process-not-the-token";

const SECURITY_TABLE = [
  { resource: "Project directory (read/write)", without: "allowed", with: "allowed" },
  { resource: "Secrets (.env*, .pem, .key, SSH keys)", without: "exposed", with: "blocked" },
  { resource: "Cloud credentials (~/.aws, ~/.azure)", without: "exposed", with: "blocked" },
  { resource: "Build tool homes (~/.m2, ~/.gradle, ~/.cargo)", without: "allowed", with: "allowed" },
  {
    resource: "Tool credential files (~/.m2/settings.xml, ~/.gradle/gradle.properties)",
    without: "exposed",
    with: "blocked",
  },
  { resource: "Git hooks‡, /tmp execution, SSH agent", without: "exposed", with: "blocked" },
  { resource: "Outbound network (HTTPS)", without: "exposed", with: "filtered" },
  { resource: "Private IPs, and localhost on macOS†", without: "exposed", with: "blocked" },
  {
    resource: "Destructive git/gh commands (push to default branch, force push, merge, delete)",
    without: "exposed",
    with: "blocked",
  },
  { resource: "Copilot auth and tool caches (read-only)", without: "allowed", with: "allowed" },
];

/* The footnote text below the table, repeated as abbr titles so the qualifier is
   available where the marker sits rather than a dozen rows further down. */
const FOOTNOTES: Record<string, string> = {
  "*": "Routed through CONNECT proxy. Telemetry and non-allowlisted domains are blocked.",
  "†": "On Linux, localhost is not blocked and UDP is unrestricted unless proxy.forced is on.",
  "‡": "Git hooks are write-protected on macOS. On Linux .git/hooks stays writable unless bubblewrap is installed.",
};

/* Only † and ‡ are footnote markers inside a resource name. The asterisk in
   ".env*" is a glob, not a marker. */
function withFootnoteMarkers(text: string) {
  return text.split(/([†‡])/).map((part, i) =>
    FOOTNOTES[part] ? (
      <abbr key={i} title={FOOTNOTES[part]}>
        {part}
      </abbr>
    ) : (
      part
    )
  );
}

type Status = {
  label: React.ReactNode;
  bg: string;
  color: string;
  Icon?: typeof CheckmarkCircleIcon;
  marker?: string;
};

const STATUSES: Record<string, Status> = {
  allowed: {
    label: "Allowed",
    bg: "var(--ax-bg-success-soft)",
    color: "var(--ax-text-success)",
    Icon: CheckmarkCircleIcon,
  },
  exposed: {
    label: "Exposed",
    bg: "var(--ax-bg-danger-soft)",
    color: "var(--ax-text-danger)",
    marker: "⚠",
  },
  blocked: {
    label: "Protected",
    bg: "var(--ax-bg-success-soft)",
    color: "var(--ax-text-success)",
    Icon: ShieldLockIcon,
  },
  filtered: {
    label: (
      <>
        Filtered<abbr title={FOOTNOTES["*"]}>*</abbr>
      </>
    ),
    bg: "var(--ax-bg-warning-soft)",
    color: "var(--ax-text-warning)",
    Icon: CloudIcon,
  },
};

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
    <main lang="en">
      <HeroSection stars={stars} />
      <SecurityTableSection />
      <GuardsSection />
      <TeamConfigSection />
      <HowItWorksSection />
      <ProxySection />
      <InitSection />
      <ConfigSection configKeys={configKeys} />
      <PolicySection />
      <FooterSection />
    </main>
  );
}

/* ---------- Hero ---------- */

function HeroSection({ stars }: { stars: number | null }) {
  return (
    <Theme theme="dark" hasBackground={false} asChild>
      <section style={{ background: GROUND, color: "var(--ax-text-neutral)" }}>
        <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-7xl mx-auto">
          <VStack gap={{ xs: "space-20", md: "space-32" }}>
            {/* Headline */}
            <VStack gap="space-12" className="text-center">
              <Heading size="xlarge" level="1">
                <code style={{ fontFamily: "monospace", fontWeight: 800 }}>cplt</code> keeps your AI agent sandboxed.
              </Heading>
              <BodyLong
                size="large"
                className="max-w-2xl mx-auto"
                style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
              >
                Kernel-level isolation for AI coding agents. Your secrets stay secret. Enforced by the OS, not by trust.
              </BodyLong>
              <BodyShort size="small" style={{ textAlign: "center" }}>
                <NextLink href={ARTICLE_HREF} className="underline" style={{ color: ACCENT }}>
                  Read the argument: a sandbox confines the process, not the token
                </NextLink>
              </BodyShort>
              {/* GitHub badge */}
              <div className="flex justify-center">
                <NextLink
                  href="https://github.com/navikt/cplt"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 rounded-full px-3.5 py-1.5 no-underline transition-all"
                  style={{
                    background: "rgba(255, 255, 255, 0.06)",
                    border: "1px solid rgba(255, 255, 255, 0.12)",
                    color: "var(--ax-text-neutral-subtle)",
                    fontSize: "0.875rem",
                  }}
                >
                  <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor" aria-hidden>
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                  </svg>
                  navikt/cplt
                  {stars !== null && (
                    <span
                      role="img"
                      aria-label={`${stars} GitHub stars`}
                      className="inline-flex items-center gap-1 rounded-full px-2 py-0.5"
                      style={{ background: "rgba(255, 255, 255, 0.08)", fontSize: CODE_SIZE }}
                    >
                      ★ {stars}
                    </span>
                  )}
                </NextLink>
              </div>
            </VStack>

            {/* Demo recording in window chrome */}
            <div className="max-w-4xl mx-auto w-full">
              <div
                className="rounded-xl overflow-hidden"
                style={{
                  border: "1px solid rgba(255, 255, 255, 0.1)",
                  boxShadow: "0 8px 40px rgba(0, 0, 0, 0.5)",
                }}
              >
                {/* Window chrome */}
                <div
                  className="flex items-center gap-2 px-4 py-2.5"
                  style={{ background: TERMINAL_BG, borderBottom: "1px solid rgba(255, 255, 255, 0.06)" }}
                >
                  <div className="flex gap-1.5" aria-hidden>
                    <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#ff5f57" }} />
                    <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#febc2e" }} />
                    <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#28c840" }} />
                  </div>
                  <span className="font-mono ml-2" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                    cplt: sandboxed Copilot session
                  </span>
                </div>
                {/* Recording. No autoplay, so it never moves until asked. */}
                <video
                  controls
                  muted
                  loop
                  playsInline
                  preload="metadata"
                  poster="/demos/cplt-demo-poster.jpg"
                  width={1024}
                  height={600}
                  className="w-full h-auto"
                  style={{ display: "block", background: "#0c0c0c" }}
                  aria-label="Sandboxed session recording: the agent tries to read ~/.ssh and to post data to an external endpoint. The sandbox denies both."
                >
                  <source src="/demos/cplt-demo.webm" type="video/webm" />
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src="/demos/cplt-demo.gif"
                    alt="The agent tries to read ~/.ssh and to post data to an external endpoint. The sandbox denies both."
                    className="w-full"
                    style={{ display: "block" }}
                  />
                </video>
              </div>
            </div>

            {/* Install CTA */}
            <div className="flex flex-col items-center gap-4">
              <div
                className="rounded-lg px-4 py-2.5 flex items-center gap-3 max-w-full overflow-x-auto"
                style={{
                  background: "rgba(255, 255, 255, 0.04)",
                  border: "1px solid rgba(255, 255, 255, 0.08)",
                }}
              >
                <code className="font-mono" style={{ fontSize: CODE_SIZE, color: "var(--ax-text-neutral-subtle)" }}>
                  {INSTALL_COMMAND}
                </code>
                <CopyButton copyText={INSTALL_COMMAND} size="small" />
              </div>
              <div
                className="rounded-lg px-4 py-2 flex items-center gap-3 max-w-full overflow-x-auto"
                style={{
                  background: "rgba(255, 255, 255, 0.04)",
                  border: "1px solid rgba(255, 255, 255, 0.08)",
                }}
              >
                <code className="font-mono" style={{ fontSize: CODE_SIZE, color: "var(--ax-text-neutral-subtle)" }}>
                  {INSTALL_SCRIPT}
                </code>
                <CopyButton copyText={INSTALL_SCRIPT} size="small" />
              </div>
              <BodyShort size="small" style={{ color: ACCENT, textAlign: "center" }}>
                macOS (Apple Seatbelt) · Linux (Landlock + seccomp-BPF) · Windows: WSL2 only
              </BodyShort>
              <BodyLong
                size="small"
                className="max-w-xl"
                style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
              >
                cplt has no Windows sandbox backend. On WSL2 it is an ordinary Linux install and the sandbox is
                kernel-enforced, so use the install script and run it inside your Linux distribution.
              </BodyLong>
            </div>
          </VStack>
        </Box>
      </section>
    </Theme>
  );
}

/* ---------- Security Table ---------- */

function StatusCell({ status, columnLabel }: { status: Status; columnLabel: string }) {
  const { Icon } = status;
  return (
    <td role="cell" data-label={columnLabel} style={{ background: status.bg }}>
      <span className="inline-flex items-center gap-1.5" style={{ color: status.color, fontWeight: 600 }}>
        {Icon ? <Icon fontSize="1rem" style={{ flexShrink: 0 }} aria-hidden /> : null}
        {status.marker ? (
          <span aria-hidden style={{ flexShrink: 0 }}>
            {status.marker}
          </span>
        ) : null}
        <span>{status.label}</span>
      </span>
    </td>
  );
}

function SecurityTableSection() {
  return (
    <section style={{ background: "var(--ax-bg-neutral-soft)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3" id="security-boundary">
              Security boundary
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              What your agent can and cannot access, enforced at the kernel level.
            </BodyLong>
          </div>

          <div
            className="rounded-xl overflow-hidden"
            style={{ border: "1px solid var(--ax-border-neutral-subtle)", boxShadow: "0 4px 12px rgba(0, 0, 0, 0.08)" }}
          >
            <table role="table" className="cplt-table" aria-labelledby="security-boundary">
              <thead role="rowgroup">
                <tr role="row">
                  <th role="columnheader" scope="col">
                    Resource
                  </th>
                  <th role="columnheader" scope="col">
                    Without cplt
                  </th>
                  <th role="columnheader" scope="col">
                    With cplt
                  </th>
                </tr>
              </thead>
              <tbody role="rowgroup">
                {SECURITY_TABLE.map((row) => (
                  <tr role="row" key={row.resource}>
                    <th role="rowheader" scope="row">
                      {withFootnoteMarkers(row.resource)}
                    </th>
                    <StatusCell status={STATUSES[row.without]} columnLabel="Without cplt" />
                    <StatusCell status={STATUSES[row.with]} columnLabel="With cplt" />
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          <VStack gap="space-8" className="max-w-3xl mx-auto">
            <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}>
              *Routed through CONNECT proxy. Telemetry and non-allowlisted domains are blocked.
              <br />
              †On Linux, localhost is not blocked and UDP is unrestricted unless{" "}
              <code style={{ fontSize: CODE_SIZE }}>proxy.forced</code> is on.
              <br />
              ‡Git hooks are write-protected on macOS. On Linux <code style={{ fontSize: CODE_SIZE }}>
                .git/hooks
              </code>{" "}
              stays writable unless bubblewrap is installed.
            </BodyLong>
            <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}>
              The filesystem and syscall layers are kernel-enforced: Apple Seatbelt on macOS, Landlock + seccomp-BPF on
              Linux, plus bubblewrap namespace isolation when <code style={{ fontSize: CODE_SIZE }}>bwrap</code> is
              installed. The git and gh guards are a different mechanism: PATH shims and a best-effort command filter.
              Bubblewrap is not a network boundary, because the host network is shared by design so the proxy keeps
              working, and the root filesystem is bind-mounted read-only with Landlock as the access-control layer.
              Known limits are written down in{" "}
              <NextLink
                href="https://github.com/navikt/cplt/blob/main/SECURITY.md"
                target="_blank"
                rel="noopener noreferrer"
                className="underline"
              >
                SECURITY.md
              </NextLink>
              .
            </BodyLong>
          </VStack>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Proxy & Network ---------- */

function ProxySection() {
  return (
    <section style={{ background: "var(--ax-bg-neutral-soft)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Network proxy
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              A local CONNECT proxy filters and logs the agent&apos;s HTTPS traffic. By default the kernel still permits
              direct outbound <code style={{ fontSize: CODE_SIZE }}>:443</code>, so the proxy only sees what is routed
              to it. Turn on <code style={{ fontSize: CODE_SIZE }}>proxy.forced</code> to make it mandatory.
            </BodyLong>
          </div>

          {/* SVG network flow diagram. Hidden below md, where its labels would
              render too small to read; the text equivalent below takes over. */}
          <div className="w-full overflow-x-auto hidden md:block">
            <svg
              viewBox="0 0 820 300"
              className="w-full"
              style={{ maxWidth: "820px", margin: "0 auto", display: "block" }}
              role="img"
              aria-label="Network proxy flow diagram showing how cplt routes and filters outbound traffic"
            >
              {/* Background */}
              <rect
                width="820"
                height="300"
                rx="12"
                fill="var(--ax-bg-default)"
                stroke="var(--ax-border-neutral-subtle)"
                strokeWidth="1"
              />

              {/* Sandbox container (wraps agent + proxy) */}
              <rect
                x="20"
                y="20"
                width="440"
                height="260"
                rx="10"
                fill="var(--ax-bg-success-soft)"
                stroke="var(--cplt-accent-ink)"
                strokeWidth="1.5"
                strokeDasharray="6 3"
              />
              <text x="40" y="44" fill="var(--cplt-accent-ink)" fontSize="13" fontWeight="600">
                cplt sandbox
              </text>

              {/* Agent box (same height as proxy: y=70, h=160) */}
              <rect x="45" y="70" width="120" height="160" rx="8" fill="var(--cplt-ground)" />
              <text
                x="105"
                y="145"
                textAnchor="middle"
                fill="var(--cplt-accent)"
                fontSize="13"
                fontWeight="600"
                fontFamily="monospace"
              >
                AI Agent
              </text>
              <text x="105" y="167" textAnchor="middle" fill="#cfd3d8" fontSize="11" fontFamily="monospace">
                curl, fetch, git
              </text>

              {/* Arrow: Agent → Proxy */}
              <line
                x1="165"
                y1="150"
                x2="225"
                y2="150"
                stroke="var(--ax-text-neutral-subtle)"
                strokeWidth="2"
                markerEnd="url(#arrowGray)"
              />

              {/* Proxy box */}
              <rect
                x="225"
                y="70"
                width="215"
                height="160"
                rx="8"
                fill="var(--ax-bg-default)"
                stroke="var(--cplt-accent-ink)"
                strokeWidth="1.5"
              />
              <text x="332" y="96" textAnchor="middle" fill="var(--cplt-accent-ink)" fontSize="14" fontWeight="700">
                CONNECT Proxy
              </text>
              <text x="332" y="114" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                localhost:ephemeral
              </text>

              {/* Proxy checks */}
              <rect x="243" y="122" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
              <text x="333" y="138" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                Blocklist / Allowlist
              </text>
              <rect x="243" y="150" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
              <text x="333" y="166" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                Private IP filter
              </text>
              <rect x="243" y="178" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
              <text x="333" y="194" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                DNS rebinding protection
              </text>

              {/* Audit log below proxy */}
              <rect
                x="258"
                y="208"
                width="150"
                height="30"
                rx="6"
                fill="var(--ax-bg-warning-soft)"
                stroke="var(--ax-text-warning)"
                strokeWidth="1"
              />
              <text x="333" y="228" textAnchor="middle" fill="var(--ax-text-warning)" fontSize="11" fontWeight="600">
                Audit log ✓
              </text>

              {/* Arrow: Proxy → Internet (allowed path) */}
              <line
                x1="440"
                y1="115"
                x2="540"
                y2="85"
                stroke="var(--cplt-accent-ink)"
                strokeWidth="2"
                markerEnd="url(#arrowGreen)"
              />
              <text x="498" y="86" textAnchor="middle" fill="var(--cplt-accent-ink)" fontSize="11" fontWeight="600">
                ✓ Allowed
              </text>

              {/* Internet box */}
              <rect
                x="540"
                y="50"
                width="260"
                height="76"
                rx="8"
                fill="var(--ax-bg-success-soft)"
                stroke="var(--cplt-accent-ink)"
                strokeWidth="1"
              />
              <text x="670" y="74" textAnchor="middle" fill="var(--cplt-accent-ink)" fontSize="13" fontWeight="600">
                Internet
              </text>
              <text x="670" y="94" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                github.com, npm, PyPI, api.openai.com
              </text>
              <text x="670" y="112" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                Allowlisted or not in blocklist
              </text>

              {/* Arrow: Proxy → Dropped (blocked path) */}
              <line
                x1="440"
                y1="170"
                x2="540"
                y2="200"
                stroke="var(--ax-text-danger)"
                strokeWidth="2"
                markerEnd="url(#arrowRed)"
              />
              <text x="498" y="198" textAnchor="middle" fill="var(--ax-text-danger)" fontSize="11" fontWeight="600">
                ✗ Blocked
              </text>

              {/* Dropped box */}
              <rect
                x="540"
                y="172"
                width="260"
                height="76"
                rx="8"
                fill="var(--ax-bg-danger-soft)"
                stroke="var(--ax-text-danger)"
                strokeWidth="1"
              />
              <text x="670" y="196" textAnchor="middle" fill="var(--ax-text-danger)" fontSize="13" fontWeight="600">
                Dropped
              </text>
              <text x="670" y="216" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                webhook.site, ngrok.io, pastebin.com
              </text>
              <text x="670" y="234" textAnchor="middle" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                169.254.x.x, 10.x.x.x, tunneling services
              </text>

              {/* Config labels along bottom */}
              <text x="470" y="272" fill="var(--ax-text-neutral-subtle)" fontSize="11" fontFamily="monospace">
                proxy.blocked_domains
              </text>
              <text x="470" y="289" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                47 domains · hot-reload every 5s
              </text>

              <text x="660" y="272" fill="var(--ax-text-neutral-subtle)" fontSize="11" fontFamily="monospace">
                proxy.allowed_domains
              </text>
              <text x="660" y="289" fill="var(--ax-text-neutral-subtle)" fontSize="11">
                Fail-closed strict mode
              </text>

              {/* Arrow markers */}
              <defs>
                <marker id="arrowGray" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="var(--ax-text-neutral-subtle)" />
                </marker>
                <marker id="arrowGreen" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="var(--cplt-accent-ink)" />
                </marker>
                <marker id="arrowRed" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
                  <polygon points="0 0, 8 3, 0 6" fill="var(--ax-text-danger)" />
                </marker>
              </defs>
            </svg>
          </div>

          {/* Text equivalent of the diagram, for narrow screens */}
          <div className="md:hidden rounded-xl px-5 py-4" style={CARD_STYLE}>
            <Heading size="xsmall" level="3">
              How the traffic flows
            </Heading>
            <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
              Agent traffic (curl, fetch, git) goes to the CONNECT proxy on{" "}
              <code style={{ fontSize: CODE_SIZE }}>localhost:ephemeral</code> inside the sandbox. The proxy applies its
              blocklist and allowlist, a private IP filter and DNS rebinding protection, and writes an audit log.
              Allowed traffic reaches the internet: github.com, npm, PyPI, api.openai.com, anything allowlisted or not
              in the blocklist. Blocked traffic is dropped: webhook.site, ngrok.io, pastebin.com, 169.254.x.x, 10.x.x.x,
              tunneling services. <code style={{ fontSize: CODE_SIZE }}>proxy.blocked_domains</code> holds 47 domains
              and hot-reloads every 5s; <code style={{ fontSize: CODE_SIZE }}>proxy.allowed_domains</code> is
              fail-closed strict mode.
            </BodyLong>
          </div>

          {/* Proxy-forced mode + upstream chaining */}
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            <div className="rounded-xl overflow-hidden flex flex-col" style={CARD_STYLE}>
              <div className="px-5 py-4 flex-1" style={{ borderBottom: "1px solid var(--ax-border-neutral-subtle)" }}>
                <Heading size="xsmall" level="3">
                  Opt-in proxy-forced mode
                </Heading>
                <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                  By default the kernel still allows direct outbound <code style={{ fontSize: CODE_SIZE }}>:443</code>,
                  so a raw socket or an unset <code style={{ fontSize: CODE_SIZE }}>HTTPS_PROXY</code> can skip the
                  proxy. <code style={{ fontSize: CODE_SIZE }}>proxy.forced</code> closes that bypass: the proxy becomes
                  mandatory and kernel-level egress is restricted to the proxy port only. Fails closed. If the proxy
                  cannot start, the agent does not launch. macOS pins fully to{" "}
                  <code style={{ fontSize: CODE_SIZE }}>localhost:&lt;proxy_port&gt;</code>; Linux drops the direct{" "}
                  <code style={{ fontSize: CODE_SIZE }}>:443</code> allow, but Landlock filtering is port-based, so a
                  narrow port-based residual remains. That is a deliberate limitation, tracked upstream.
                </BodyLong>
              </div>
              <div className="px-5 py-3 flex items-center gap-3" style={{ background: "var(--ax-bg-neutral-soft)" }}>
                <code
                  className="font-mono flex-1"
                  style={{ fontSize: CODE_SIZE, color: "var(--ax-text-neutral-subtle)" }}
                >
                  cplt config set proxy.forced true
                </code>
                <CopyButton copyText="cplt config set proxy.forced true" size="small" />
              </div>
            </div>

            <div className="rounded-xl overflow-hidden flex flex-col" style={CARD_STYLE}>
              <div className="px-5 py-4 flex-1" style={{ borderBottom: "1px solid var(--ax-border-neutral-subtle)" }}>
                <Heading size="xsmall" level="3">
                  Corporate / upstream proxy chaining
                </Heading>
                <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                  Behind a corporate proxy? <code style={{ fontSize: CODE_SIZE }}>proxy.upstream</code> forwards CONNECT
                  tunnels through it instead of forcing you to disable the cplt proxy. cplt applies its own domain
                  filtering, logging, and port checks <em>before</em> forwarding the tunnel upstream, so a blocked
                  target never reaches the corporate proxy. Optional basic-auth userinfo is supported; http scheme only.
                </BodyLong>
              </div>
              <div className="px-5 py-3 flex items-center gap-3" style={{ background: "var(--ax-bg-neutral-soft)" }}>
                <code
                  className="font-mono flex-1"
                  style={{ fontSize: CODE_SIZE, color: "var(--ax-text-neutral-subtle)" }}
                >
                  cplt config set proxy.upstream &quot;http://proxy.example.com:8080&quot;
                </code>
                <CopyButton copyText='cplt config set proxy.upstream "http://proxy.example.com:8080"' size="small" />
              </div>
            </div>
          </HGrid>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Guards ---------- */

const GH_GUARD_LEVELS = [
  {
    level: "Read",
    examples: "gh issue list, gh pr view",
    access: "Allowed",
    bg: "var(--ax-bg-success-soft)",
    color: "var(--ax-text-success)",
  },
  {
    level: "Write",
    examples: "gh pr create, gh issue edit",
    access: "Own repo only",
    bg: "var(--ax-bg-warning-soft)",
    color: "var(--ax-text-warning)",
  },
  {
    level: "Destructive",
    examples: "gh repo delete, gh pr merge",
    access: "Always blocked",
    bg: "var(--ax-bg-danger-soft)",
    color: "var(--ax-text-danger)",
  },
];

function GuardsSection() {
  return (
    <section style={{ background: "var(--ax-bg-default)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              gh guard &amp; git guard
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              Block destructive GitHub and git operations. The agent can commit and branch, but not push to main or
              merge PRs.
            </BodyLong>
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            {/* gh guard */}
            <div className="rounded-xl overflow-hidden" style={CARD_STYLE}>
              <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--ax-border-neutral-subtle)" }}>
                <Heading size="xsmall" level="3">
                  gh guard with a three-tier policy
                </Heading>
                <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                  Default-deny engine over 132 classified <code style={{ fontSize: CODE_SIZE }}>gh</code> commands: 51
                  allowed, 64 blocked, 17 scope-checked.
                </BodyLong>
              </div>
              <div>
                {GH_GUARD_LEVELS.map((row, i) => (
                  <div
                    key={row.level}
                    className="flex items-center gap-3 px-5 py-3"
                    style={{ borderTop: i > 0 ? "1px solid var(--ax-border-neutral-subtle)" : undefined }}
                  >
                    <div
                      className="rounded-full shrink-0"
                      style={{ width: "8px", height: "8px", background: row.color }}
                      aria-hidden
                    />
                    <div className="flex-1 min-w-0">
                      <BodyShort size="small" weight="semibold">
                        {row.level}
                      </BodyShort>
                      <BodyShort size="small" style={{ color: "var(--ax-text-neutral-subtle)" }}>
                        {row.examples}
                      </BodyShort>
                    </div>
                    <span
                      className="shrink-0 font-semibold rounded-full px-2.5 py-1"
                      style={{ fontSize: CODE_SIZE, background: row.bg, color: row.color }}
                    >
                      {row.access}
                    </span>
                  </div>
                ))}
              </div>
              <div
                className="px-5 py-3"
                style={{
                  background: "var(--ax-bg-neutral-soft)",
                  borderTop: "1px solid var(--ax-border-neutral-subtle)",
                }}
              >
                <BodyShort size="small" style={{ color: "var(--ax-text-neutral-subtle)" }}>
                  <code style={{ fontSize: CODE_SIZE }}>gh api</code> calls restricted to{" "}
                  <code style={{ fontSize: CODE_SIZE }}>/repos/&#123;current-repo&#125;/...</code>
                </BodyShort>
              </div>
            </div>

            {/* git guard + enable commands */}
            <VStack gap="space-16">
              <div className="rounded-xl overflow-hidden" style={CARD_STYLE}>
                <div className="px-5 py-4" style={{ borderBottom: "1px solid var(--ax-border-neutral-subtle)" }}>
                  <Heading size="xsmall" level="3">
                    git guard for push protection
                  </Heading>
                  <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                    Blocks pushes to the default branch, and force pushes everywhere. A push to a feature branch goes
                    through, so the review gate is the pull request rather than the push. Commit, branch, rebase. All
                    fine.
                  </BodyLong>
                </div>
                <div className="px-5 py-3" style={{ background: "var(--ax-bg-neutral-soft)" }}>
                  <BodyShort size="small" style={{ color: "var(--ax-text-neutral-subtle)" }}>
                    Refuse every push instead?{" "}
                    <code style={{ fontSize: CODE_SIZE }}>
                      cplt config set git_guard.protect_default_branch_only false
                    </code>
                  </BodyShort>
                </div>
              </div>

              {/* Opting out */}
              <div className="rounded-xl overflow-hidden" style={CARD_STYLE}>
                <div
                  className="flex items-center justify-between px-4 py-2"
                  style={{ background: TERMINAL_BG, borderBottom: "1px solid #333" }}
                >
                  <span className="font-mono" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                    Opt out for a single run
                  </span>
                  <CopyButton copyText="cplt --no-gh-guard --no-git-guard" size="small" style={{ color: "white" }} />
                </div>
                <pre
                  className="p-4 font-mono leading-relaxed overflow-x-auto"
                  style={{ margin: 0, fontSize: CODE_SIZE, color: TERMINAL_FG, background: TERMINAL_BG }}
                >
                  <span style={{ color: "#6ee7b7" }}>$</span>
                  {" cplt --no-gh-guard --no-git-guard"}
                </pre>
              </div>

              {/* What agent sees */}
              <div className="rounded-xl overflow-hidden" style={CARD_STYLE}>
                <div
                  className="flex items-center justify-between px-4 py-2"
                  style={{ background: TERMINAL_BG, borderBottom: "1px solid #333" }}
                >
                  <span className="font-mono" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                    What the agent sees
                  </span>
                </div>
                <pre
                  className="p-4 font-mono leading-relaxed overflow-x-auto"
                  style={{ margin: 0, fontSize: CODE_SIZE, color: TERMINAL_FG, background: TERMINAL_BG }}
                >
                  <span style={{ color: "#f87171" }}>⛔ sandbox restriction:</span>
                  {" `gh pr merge` is not allowed.\n"}
                  {"This command is classified as destructive\n"}
                  {"and blocked by gh guard.\n\n"}
                  <span style={{ color: TERMINAL_MUTED }}>
                    Please note this for the human operator{"\n"}and continue with your remaining work.
                  </span>
                </pre>
              </div>
            </VStack>
          </HGrid>

          <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}>
            Both guards are on by default. Opt out for a single run with{" "}
            <code style={{ fontSize: CODE_SIZE }}>--no-gh-guard</code> or{" "}
            <code style={{ fontSize: CODE_SIZE }}>--no-git-guard</code>, or set{" "}
            <code style={{ fontSize: CODE_SIZE }}>mode: audit</code> to observe before enforcing.
          </BodyLong>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Team Configuration ---------- */

function TeamConfigSection() {
  return (
    <section style={{ background: "var(--ax-bg-default)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Sandbox config read from git HEAD, not the working tree
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              Commit <code style={{ fontSize: CODE_SIZE }}>.cplt.toml</code> to your repo and every developer gets the
              same sandbox, without the agent being able to loosen it.
            </BodyLong>
          </div>

          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
            {/* TOML example */}
            <div className="rounded-xl overflow-hidden" style={CARD_STYLE}>
              <div
                className="flex items-center justify-between px-4 py-2"
                style={{ background: TERMINAL_BG, borderBottom: "1px solid #333" }}
              >
                <span className="font-mono" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                  .cplt.toml
                </span>
                <CopyButton
                  copyText={`[deny]\nenv = ["VAULT_TOKEN", "NPM_TOKEN"]\n\n[propose]\nallow_localhost_any = true\n\n[propose.allow]\nports = [5432]\nlocalhost = [3000]`}
                  size="small"
                  style={{ color: "white" }}
                />
              </div>
              <pre
                className="p-4 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: CODE_SIZE, color: TERMINAL_FG, background: TERMINAL_BG }}
              >
                <span style={{ color: TERMINAL_MUTED }}># Tightens the sandbox, applied without approval</span>
                {"\n"}
                <span style={{ color: "#569cd6" }}>[deny]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>env</span>
                <span style={{ color: TERMINAL_FG }}> = [</span>
                <span style={{ color: "#ce9178" }}>&quot;VAULT_TOKEN&quot;</span>
                <span style={{ color: TERMINAL_FG }}>, </span>
                <span style={{ color: "#ce9178" }}>&quot;NPM_TOKEN&quot;</span>
                <span style={{ color: TERMINAL_FG }}>]</span>
                {"\n\n"}
                <span style={{ color: TERMINAL_MUTED }}># Loosens the sandbox, inert until `cplt trust accept`</span>
                {"\n"}
                <span style={{ color: "#569cd6" }}>[propose]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>allow_localhost_any</span>
                <span style={{ color: TERMINAL_FG }}> = </span>
                <span style={{ color: "#569cd6" }}>true</span>
                {"\n\n"}
                <span style={{ color: "#569cd6" }}>[propose.allow]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>ports</span>
                <span style={{ color: TERMINAL_FG }}> = [</span>
                <span style={{ color: "#b5cea8" }}>5432</span>
                <span style={{ color: TERMINAL_FG }}>]</span>
                {"\n"}
                <span style={{ color: "#9cdcfe" }}>localhost</span>
                <span style={{ color: TERMINAL_FG }}> = [</span>
                <span style={{ color: "#b5cea8" }}>3000</span>
                <span style={{ color: TERMINAL_FG }}>]</span>
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
                    background: "var(--ax-bg-success-soft)",
                    border: "1px solid var(--ax-border-neutral-subtle)",
                  }}
                >
                  <ShieldLockIcon fontSize="1.125rem" style={{ color: ACCENT_INK }} aria-hidden />
                </div>
                <div>
                  <Heading size="xsmall" level="3">
                    [deny] is applied automatically
                  </Heading>
                  <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                    Can only tighten the sandbox. Block env vars and deny file paths. No approval needed.
                  </BodyLong>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div
                  className="flex items-center justify-center rounded-lg shrink-0 mt-0.5"
                  style={{
                    width: "2.25rem",
                    height: "2.25rem",
                    background: "var(--ax-bg-success-soft)",
                    border: "1px solid var(--ax-border-neutral-subtle)",
                  }}
                >
                  <CheckmarkCircleIcon fontSize="1.125rem" style={{ color: ACCENT_INK }} aria-hidden />
                </div>
                <div>
                  <Heading size="xsmall" level="3">
                    [propose] requires approval
                  </Heading>
                  <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                    Request additional permissions. Each developer approves with{" "}
                    <code style={{ fontSize: CODE_SIZE }}>cplt trust accept --all</code>. Content-pinned, so any change
                    invalidates the approval.
                  </BodyLong>
                </div>
              </div>

              <div className="flex items-start gap-3">
                <div
                  className="flex items-center justify-center rounded-lg shrink-0 mt-0.5"
                  style={{
                    width: "2.25rem",
                    height: "2.25rem",
                    background: "var(--ax-bg-success-soft)",
                    border: "1px solid var(--ax-border-neutral-subtle)",
                  }}
                >
                  <WrenchIcon fontSize="1.125rem" style={{ color: ACCENT_INK }} aria-hidden />
                </div>
                <div>
                  <Heading size="xsmall" level="3">
                    The agent cannot edit its own sandbox
                  </Heading>
                  <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                    <code style={{ fontSize: CODE_SIZE }}>.cplt.toml</code> is read from git HEAD, so an edit in the
                    working tree changes nothing, and the file is write-denied inside the sandbox anyway. A committed{" "}
                    <code style={{ fontSize: CODE_SIZE }}>[deny]</code> block therefore applies unconditionally: it can
                    only tighten the sandbox, so there is nothing to approve. A{" "}
                    <code style={{ fontSize: CODE_SIZE }}>[propose]</code> block is content-pinned and stays inert until
                    the developer runs <code style={{ fontSize: CODE_SIZE }}>cplt trust accept</code>, and any change to
                    the block invalidates that approval.
                  </BodyLong>
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
  { name: "JVM", detail: "Gradle / Maven" },
  { name: "Node.js", detail: "npm / pnpm" },
  { name: "Docker", detail: "Compose" },
  { name: "Python", detail: "pip / uv" },
  { name: "Spring Boot", detail: "8080 + PG" },
  { name: "Ktor", detail: "8080" },
  { name: "Next.js", detail: "3000" },
  { name: "Flyway", detail: "PG 5432" },
  { name: "Playwright", detail: "browsers" },
  { name: "Rust / Go", detail: "defaults" },
];

function InitSection() {
  return (
    <Theme theme="dark" hasBackground={false} asChild>
      <section style={{ background: GROUND, color: "var(--ax-text-neutral)" }}>
        <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
          <VStack gap={SECTION_GAP}>
            <div className="text-center">
              <Heading size="medium" level="2" className="mb-3">
                Auto-detect your project
              </Heading>
              <BodyLong
                size="large"
                className="max-w-2xl mx-auto"
                style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
              >
                <code style={{ color: ACCENT, fontSize: CODE_SIZE }}>cplt init</code> scans your project for build
                files, frameworks, and patterns, then generates the right{" "}
                <code style={{ color: ACCENT, fontSize: CODE_SIZE }}>.cplt.toml</code> automatically.
              </BodyLong>
            </div>

            <HGrid columns={{ xs: 1, md: 2 }} gap="space-16" className="items-stretch">
              {/* Terminal mock */}
              <div className="rounded-xl overflow-hidden" style={{ border: "1px solid rgba(255, 255, 255, 0.1)" }}>
                <div
                  className="flex items-center gap-2 px-4 py-2"
                  style={{ background: TERMINAL_BG, borderBottom: "1px solid rgba(255, 255, 255, 0.06)" }}
                >
                  <span className="font-mono" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                    $ cplt init
                  </span>
                </div>
                <pre
                  className="p-4 font-mono leading-relaxed overflow-x-auto"
                  style={{ margin: 0, fontSize: CODE_SIZE, color: TERMINAL_FG, background: "#0d1117" }}
                >
                  <span style={{ color: "#6ee7b7" }}>Detected:</span>
                  {"\n"}
                  {"  "}
                  <span style={{ color: "#79c0ff" }}>Spring Boot</span>
                  {"  application.yml + spring-boot-starter\n"}
                  {"  "}
                  <span style={{ color: "#79c0ff" }}>Flyway</span>
                  {"       db/migration/ directory\n"}
                  {"  "}
                  <span style={{ color: "#79c0ff" }}>Docker</span>
                  {"       Dockerfile + compose.yml\n"}
                  {"  "}
                  <span style={{ color: "#79c0ff" }}>Gradle</span>
                  {"       build.gradle.kts\n"}
                  {"  "}
                  <span style={{ color: "#79c0ff" }}>.env</span>
                  {"         .env.example found\n"}
                  {"\n"}
                  <span style={{ color: "#6ee7b7" }}>Generated .cplt.toml:</span>
                  {"\n\n"}
                  <span style={{ color: TERMINAL_MUTED }}># Deny access to sensitive env vars</span>
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
                  <span style={{ color: "#fbbf24" }}>⚠ allow_docker</span>
                  {"  Docker detected, grants broad access\n\n"}
                  <span style={{ color: TERMINAL_MUTED }}>Run </span>
                  <span style={{ color: "#6ee7b7" }}>cplt init --write</span>
                  <span style={{ color: TERMINAL_MUTED }}> to save</span>
                </pre>
              </div>

              {/* Ecosystems grid */}
              <VStack gap="space-16">
                <div>
                  <Heading size="xsmall" level="3">
                    15 ecosystem detectors
                  </Heading>
                  <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                    Each detector knows which sandbox permissions the ecosystem needs. Dangerous permissions get risk
                    warnings.
                  </BodyLong>
                </div>

                <ul className="flex flex-wrap gap-2 list-none p-0 m-0">
                  {ECOSYSTEMS.map((eco) => (
                    <li
                      key={eco.name}
                      className="rounded-lg flex items-center gap-2"
                      style={{
                        padding: "0.375rem 0.75rem",
                        background: "rgba(255, 255, 255, 0.04)",
                        border: "1px solid rgba(255, 255, 255, 0.08)",
                        fontSize: CODE_SIZE,
                      }}
                    >
                      <span style={{ fontWeight: 600 }}>{eco.name}</span>
                      <span style={{ color: "var(--ax-text-neutral-subtle)" }}>{eco.detail}</span>
                    </li>
                  ))}
                </ul>

                <div
                  className="rounded-lg flex items-start gap-3"
                  style={{
                    padding: "0.75rem 1rem",
                    background: "rgba(255, 255, 255, 0.04)",
                    border: "1px solid rgba(255, 255, 255, 0.08)",
                  }}
                >
                  <MagnifyingGlassIcon fontSize="1rem" style={{ color: ACCENT, marginTop: "0.25rem" }} aria-hidden />
                  <div>
                    <Heading size="xsmall" level="3">
                      Personal config with --global
                    </Heading>
                    <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.25rem" }}>
                      Detects Gradle wrapper, Playwright browsers, GPG signing, and alternative agents on your machine.
                      Writes to <code style={{ fontSize: CODE_SIZE }}>~/.config/cplt/config.toml</code>.
                    </BodyLong>
                  </div>
                </div>
              </VStack>
            </HGrid>
          </VStack>
        </Box>
      </section>
    </Theme>
  );
}

/* ---------- Configuration ---------- */

function ConfigSection({ configKeys }: { configKeys: import("@/lib/cplt-config").CpltConfigKey[] }) {
  return (
    <section style={{ background: "var(--ax-bg-default)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Configuration
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              Every option explained. Search by name or description.
            </BodyLong>
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
      command: INSTALL_COMMAND,
      description: "Homebrew on macOS, or the install script at the top of this page on Linux and WSL2.",
      Icon: TerminalIcon,
    },
    {
      title: "Configure",
      command: "cplt init --write",
      description: "Detect your project's tooling and generate sandbox config.",
      Icon: MagnifyingGlassIcon,
    },
    {
      title: "Run your agent",
      command: 'cplt -- -p "fix the tests"',
      description: "Your agent works normally, but your secrets are unreadable.",
      Icon: ShieldLockIcon,
    },
  ];

  return (
    <section style={{ background: "var(--ax-bg-default)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-5xl mx-auto">
        <VStack gap={SECTION_GAP}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              How it works
            </Heading>
            <BodyLong
              size="large"
              className="max-w-2xl mx-auto"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              Three steps from zero to sandboxed agent.
            </BodyLong>
          </div>

          <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
            {steps.map((step, i) => (
              <div
                key={step.title}
                className="rounded-xl overflow-hidden flex flex-col"
                style={{ ...CARD_STYLE, background: "var(--ax-bg-neutral-soft)" }}
              >
                <div style={{ height: "3px", background: ACCENT }} />
                <Box padding={{ xs: "space-16", md: "space-20" }} className="flex-1 flex flex-col">
                  <div className="flex flex-col items-center text-center flex-1">
                    <div
                      className="flex items-center justify-center rounded-full mb-2"
                      style={{
                        width: "2.5rem",
                        height: "2.5rem",
                        background: "var(--ax-bg-success-soft)",
                        border: "1.5px solid var(--ax-border-neutral-subtle)",
                      }}
                    >
                      <step.Icon fontSize="1.25rem" style={{ color: ACCENT_INK }} aria-hidden />
                    </div>
                    <BodyShort
                      size="small"
                      weight="semibold"
                      style={{ color: "var(--ax-text-neutral-subtle)", letterSpacing: "0.05em" }}
                    >
                      STEP {i + 1}
                    </BodyShort>
                    <Heading size="xsmall" level="3" style={{ marginTop: "0.25rem" }}>
                      {step.title}
                    </Heading>
                    <div
                      className="rounded-lg w-full overflow-x-auto flex items-center gap-2 mt-3"
                      style={{ background: TERMINAL_BG, padding: "0.5rem 0.75rem" }}
                    >
                      <code
                        className="font-mono whitespace-nowrap flex-1"
                        style={{ fontSize: CODE_SIZE, color: TERMINAL_FG }}
                      >
                        {step.command}
                      </code>
                      <CopyButton copyText={step.command} size="small" style={{ color: "white" }} />
                    </div>
                    <BodyLong
                      size="small"
                      style={{ color: "var(--ax-text-neutral-subtle)", marginTop: "0.75rem", textAlign: "center" }}
                    >
                      {step.description}
                    </BodyLong>
                  </div>
                </Box>
              </div>
            ))}
          </HGrid>

          <BodyLong
            size="small"
            className="max-w-2xl mx-auto"
            style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
          >
            Copilot CLI is the default. <code style={{ fontSize: CODE_SIZE }}>--agent</code> takes{" "}
            <code style={{ fontSize: CODE_SIZE }}>copilot</code>, <code style={{ fontSize: CODE_SIZE }}>opencode</code>,{" "}
            <code style={{ fontSize: CODE_SIZE }}>gemini</code>,{" "}
            <code style={{ fontSize: CODE_SIZE }}>antigravity</code>, <code style={{ fontSize: CODE_SIZE }}>pi</code>,{" "}
            <code style={{ fontSize: CODE_SIZE }}>claude</code>, <code style={{ fontSize: CODE_SIZE }}>goose</code> and{" "}
            <code style={{ fontSize: CODE_SIZE }}>shell</code>, the last being a sandboxed shell with no AI.
          </BodyLong>

          {/* Shell setup tip */}
          <VStack gap="space-8" className="max-w-2xl mx-auto w-full">
            <Heading size="xsmall" level="3" className="text-center">
              Make it the default
            </Heading>
            <BodyLong size="small" style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}>
              Run <code style={{ fontSize: CODE_SIZE }}>cplt --shell-install</code> so{" "}
              <code style={{ fontSize: CODE_SIZE }}>copilot</code> always runs sandboxed.
            </BodyLong>
            <div className="rounded-xl w-full overflow-hidden" style={CARD_STYLE}>
              <div
                className="flex items-center justify-between px-4 py-2"
                style={{ background: TERMINAL_BG, borderBottom: "1px solid #333" }}
              >
                <span className="font-mono" style={{ color: TERMINAL_MUTED, fontSize: CODE_SIZE }}>
                  $ cplt --shell-install
                </span>
                <CopyButton copyText="cplt --shell-install" size="small" style={{ color: "white" }} />
              </div>
              <pre
                className="p-4 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: CODE_SIZE, color: TERMINAL_FG, background: TERMINAL_BG }}
              >
                <span style={{ color: "#6ee7b7" }}>✓</span>
                {" Added to ~/.zshrc\n"}
                <span style={{ color: "#6ee7b7" }}>✓</span> <span style={{ color: TERMINAL_MUTED }}>copilot</span>
                {" → "}
                <span style={{ color: "#6ee7b7" }}>cplt</span>
                {" (sandboxed)\n\n"}
                <span style={{ color: TERMINAL_MUTED }}>Restart your shell or: </span>
                <span style={{ color: TERMINAL_FG }}>source ~/.zshrc</span>
              </pre>
            </div>
          </VStack>

          <div className="flex flex-col items-center gap-3">
            <NextLink
              href="https://github.com/navikt/cplt"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center gap-2 px-6 py-3 rounded-lg font-medium no-underline transition-all"
              style={{ background: ACCENT_INK, color: "white", fontSize: "0.875rem" }}
            >
              View on GitHub →
            </NextLink>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Nav policy ---------- */

function PolicySection() {
  return (
    <section style={{ background: "var(--ax-bg-neutral-soft)" }}>
      <Box paddingBlock={SECTION_PADDING_BLOCK} paddingInline={SECTION_PADDING_INLINE} className="max-w-3xl mx-auto">
        <div className="rounded-lg" style={{ ...CARD_STYLE, padding: "1.25rem" }}>
          <BodyLong size="small">
            <strong>For Nav employees:</strong> isolation is required for all AI agent work on Nav equipment, personal
            work included. cplt is the recommended way. Any other route has to give equivalent isolation, and that is
            your responsibility to set up.
          </BodyLong>
          <BodyShort size="small" style={{ marginTop: "0.75rem" }}>
            <NextLink href="/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr" lang="nb" hrefLang="nb" className="underline">
              Sandboxing er påkrevd på Nav-utstyr
            </NextLink>
          </BodyShort>
        </div>
      </Box>
    </section>
  );
}

/* ---------- Footer ---------- */

function FooterSection() {
  return (
    <Theme theme="dark" hasBackground={false} asChild>
      <section style={{ background: GROUND, color: "var(--ax-text-neutral)" }}>
        <Box
          paddingBlock={SECTION_PADDING_BLOCK}
          paddingInline={SECTION_PADDING_INLINE}
          className="max-w-7xl mx-auto text-center"
        >
          <VStack gap="space-16" className="items-center">
            <Heading size="small" level="2">
              Trust the kernel, not the agent.
            </Heading>
            <BodyLong
              size="small"
              className="max-w-lg"
              style={{ color: "var(--ax-text-neutral-subtle)", textAlign: "center" }}
            >
              Open source, MIT licensed, built at Nav.
            </BodyLong>
            <div className="flex flex-wrap gap-6 justify-center" style={{ fontSize: "0.875rem" }}>
              <NextLink
                href="https://github.com/navikt/cplt"
                target="_blank"
                rel="noopener noreferrer"
                className="no-underline transition-colors py-1.5"
                style={{ color: "var(--ax-text-neutral-subtle)" }}
              >
                GitHub
              </NextLink>
              <NextLink
                href="https://github.com/navikt/cplt/blob/main/SECURITY.md"
                target="_blank"
                rel="noopener noreferrer"
                className="no-underline transition-colors py-1.5"
                style={{ color: "var(--ax-text-neutral-subtle)" }}
              >
                Security Policy
              </NextLink>
              <NextLink
                href="https://github.com/navikt/cplt/blob/main/LICENSE"
                target="_blank"
                rel="noopener noreferrer"
                className="no-underline transition-colors py-1.5"
                style={{ color: "var(--ax-text-neutral-subtle)" }}
              >
                MIT License
              </NextLink>
            </div>
          </VStack>
        </Box>
      </section>
    </Theme>
  );
}
