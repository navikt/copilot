// cplt on Windows: the WSL2 route, for readers of the /cplt landing page. Same
// rules as /cplt: English, cplt's dark design, not the ki-utvikling/Aksel look.
// The cplt README's WSL2 section is the source; this page is the short version.
import type { Metadata } from "next";
import { Box, VStack, Heading, BodyLong, Theme } from "@navikt/ds-react";
import NextLink from "next/link";
import { Command } from "@/components/install-picker";
import { CPLT_APT_INSTALL } from "@/lib/install-commands";

const PAGE_TITLE = "cplt on Windows (WSL2)";
const PAGE_DESCRIPTION =
  "cplt has no Windows sandbox. Install it and your agent inside WSL2, keep the project in the Linux filesystem, and check the result with cplt doctor.";

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  openGraph: { title: PAGE_TITLE, description: PAGE_DESCRIPTION, type: "website" },
};

const README_WSL = "https://github.com/navikt/cplt#windows-wsl2";
const PTY_DOCS = "https://github.com/navikt/cplt/blob/main/docs/known-impacts.md#terminal-devices-and-allocating-a-pty";
const WSL_ISSUE = "https://github.com/navikt/cplt/issues/189";

const POWERSHELL = `wsl --install    # WSL2 and Ubuntu, then reboot
wsl --update     # a current Microsoft kernel, for the full Landlock ABI`;

const TOOLS = `# Node 22 or newer. Ubuntu 26.04 ships it; on 24.04 (Node 18) use nvm or fnm instead.
sudo apt update && sudo apt install -y nodejs npm

# GitHub CLI, and log in
sudo apt install -y gh
gh auth login

# The agent, installed in the distro
npm install -g @github/copilot`;

const PROJECT = `mkdir -p ~/src && cd ~/src
git clone https://github.com/<org>/<repo>.git
cd <repo> && cplt`;

const PTY_FIX = `cplt --allow-write /dev -- -p "run the pexpect suite"`;

const COPY = { copyTitle: "Copy", copied: "Copied!" };

function ExternalLink({ href, children }: { href: string; children: React.ReactNode }) {
  return (
    <NextLink
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className="underline"
      style={{ color: "var(--cplt-accent)" }}
    >
      {children}
    </NextLink>
  );
}

function Step({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <VStack gap="space-12" as="section" aria-labelledby={id}>
      <Heading size="medium" level="2" id={id}>
        {title}
      </Heading>
      {children}
    </VStack>
  );
}

const muted = { color: "var(--ax-text-neutral-subtle)" };

export default function CpltWindowsPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1} lang="en">
      <Theme theme="dark" hasBackground={false} asChild>
        <div style={{ background: "var(--cplt-ground)", color: "var(--ax-text-neutral)" }}>
          <Box
            paddingBlock={{ xs: "space-24", md: "space-40" }}
            paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
            className="max-w-3xl mx-auto"
          >
            <VStack gap={{ xs: "space-24", md: "space-32" }}>
              <VStack gap="space-12">
                <NextLink
                  href="/cplt"
                  className="underline"
                  style={{ color: "var(--cplt-accent)", fontSize: "0.875rem" }}
                >
                  ← cplt
                </NextLink>
                <Heading size="xlarge" level="1">
                  cplt on Windows, through WSL2
                </Heading>
                <BodyLong size="large" style={muted}>
                  cplt has no Windows sandbox. It enforces its rules with Apple Seatbelt on macOS and Landlock on Linux,
                  so on Windows it runs inside WSL2, where it is an ordinary Linux install. The agent, your tools and
                  your project all belong inside the Linux distribution, not on the Windows side.
                </BodyLong>
              </VStack>

              <Step id="powershell" title="1. In PowerShell, once">
                <Command command={POWERSHELL} {...COPY} />
                <BodyLong size="small" style={muted}>
                  An install still on the 6.6 kernel gets an older Landlock ABI: file rules hold, but port rules do not,
                  and network filtering falls back to cplt&apos;s proxy. <code>wsl --update</code> fixes that.
                  Don&apos;t set an <code>lsm=</code> list without <code>landlock</code> in <code>.wslconfig</code>;
                  that turns off the enforcement cplt depends on.
                </BodyLong>
              </Step>

              <Step id="inside" title="2. Everything else inside the distribution">
                <BodyLong size="small" style={muted}>
                  Open Ubuntu from Windows Terminal, or run <code>wsl</code>, and install the tools there.
                </BodyLong>
                <Command command={TOOLS} {...COPY} />
                <BodyLong size="small" style={muted}>
                  Don&apos;t install Copilot CLI on Windows. WSL appends the Windows <code>PATH</code> to the
                  distribution&apos;s, so a Windows install shows up as{" "}
                  <code>/mnt/c/Users/&lt;you&gt;/AppData/Roaming/npm/copilot</code>, and it cannot run in the Linux
                  sandbox. cplt names that cause when it finds one.
                </BodyLong>
                <BodyLong size="small" style={muted}>
                  Then cplt, from the apt archive:
                </BodyLong>
                <Command command={CPLT_APT_INSTALL} {...COPY} />
              </Step>

              <Step id="project" title="3. Keep the project in the Linux filesystem">
                <Command command={PROJECT} {...COPY} />
                <BodyLong size="small" style={muted}>
                  Work in <code>~/src</code>, not under <code>/mnt/c</code>. Files on the Windows side are much slower
                  from WSL, and nobody has yet confirmed how Landlock enforces its rules there.
                </BodyLong>
              </Step>

              <Step id="doctor" title="4. Check the result">
                <Command command="cplt doctor" {...COPY} />
                <BodyLong size="small" style={muted}>
                  It prints the kernel version and the Landlock ABI it found, and fails if your agent comes from the
                  Windows side.
                </BodyLong>
              </Step>

              <Step id="terminals" title="Known limit: tools that open their own terminal">
                <BodyLong size="small" style={muted}>
                  Inside cplt, <code>pexpect</code>, <code>pty.spawn()</code>, <code>node-pty</code>,{" "}
                  <code>script</code> and <code>tmux</code> fail, with errors such as <code>forkpty(3) failed</code>.
                  The sandbox blocks the numbered terminal devices, because most of them belong to your other windows.
                  Your own session, pagers and colours keep working.
                </BodyLong>
                <BodyLong size="small" style={muted}>
                  Run such a command outside cplt. If it has to run inside, grant the devices back for that one session:
                </BodyLong>
                <Command command={PTY_FIX} {...COPY} />
                <BodyLong size="small" style={muted}>
                  For that session a compromised agent can also write to and read from your other terminal windows, so
                  don&apos;t put the grant in <code>config.toml</code>. Details:{" "}
                  <ExternalLink href={PTY_DOCS}>terminal devices in known impacts</ExternalLink>.
                </BodyLong>
              </Step>

              <Step id="help" title="Getting help">
                <BodyLong size="small" style={muted}>
                  Send the output of <code>cplt doctor --verbose</code> and the exact command that failed. The WSL2
                  route is not yet verified end to end on a real install, so tell us what happened in{" "}
                  <ExternalLink href={WSL_ISSUE}>navikt/cplt#189</ExternalLink>, whether it worked or not. The full
                  version of this guide is the{" "}
                  <ExternalLink href={README_WSL}>WSL2 section of the cplt README</ExternalLink>.
                </BodyLong>
              </Step>
            </VStack>
          </Box>
        </div>
      </Theme>
    </main>
  );
}
