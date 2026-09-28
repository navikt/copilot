import { BodyLong, BodyShort, Box, HStack, Heading, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import type { ReactNode } from "react";
import { Table, TableBody, TableDataCell, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { CpltConfigExplorer } from "@/components/cplt-config-explorer";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { fetchCpltConfigKeys } from "@/lib/cplt-config";
import { InstallPicker } from "@/components/install-picker";
import { CPLT_APT_INSTALL, CPLT_BREW_INSTALL, CPLT_INSTALL_SCRIPT } from "@/lib/install-commands";

const PAGE_TITLE = "Sandkassen cplt";
const PAGE_DESCRIPTION =
  "cplt kjører GitHub Copilot CLI, OpenCode, Antigravity, Pi, Claude Code, goose eller et vanlig skall i en sandkasse som kjernen i operativsystemet håndhever. Agenten kan jobbe i prosjektet ditt, men kan ikke lese hemmelighetene dine.";

export const metadata: Metadata = {
  title: PAGE_TITLE,
  description: PAGE_DESCRIPTION,
  openGraph: { title: PAGE_TITLE, description: PAGE_DESCRIPTION, type: "website" },
  twitter: { card: "summary_large_image", title: PAGE_TITLE, description: PAGE_DESCRIPTION },
};

const TOC: TocItem[] = [
  { id: "installer", label: "Installer cplt" },
  { id: "sikkerhetsgrense", label: "Hva agenten får tilgang til" },
  { id: "vakter", label: "Vakter for gh og git" },
  { id: "felles-konfig", label: "Felles konfig i repoet" },
  { id: "slik-virker-det", label: "Slik virker det" },
  { id: "nettverk", label: "Nettverksproxy" },
  { id: "cplt-init", label: "Finn oppsettet automatisk" },
  { id: "innstillinger", label: "Alle innstillinger" },
  { id: "krav-i-nav", label: "Krav i Nav" },
  { id: "apen-kildekode", label: "Åpen kildekode" },
];

const GH = "https://github.com/navikt/cplt";
const INSTALL_COMMAND = CPLT_BREW_INSTALL;
const ARTICLE_HREF = "/en/news/sandbox-confines-the-process-not-the-token";
const c = (s: string) => <code className={code}>{s}</code>;

/* ---------- Data ---------- */

type StatusKey = "allowed" | "exposed" | "blocked" | "filtered";

const SECURITY_TABLE: { resource: string; without: StatusKey; with: StatusKey }[] = [
  { resource: "Prosjektmappa (lese og skrive)", without: "allowed", with: "allowed" },
  { resource: "Hemmeligheter (.env*, .pem, .key, SSH-nøkler)", without: "exposed", with: "blocked" },
  { resource: "Skynøkler (~/.aws, ~/.azure)", without: "exposed", with: "blocked" },
  { resource: "Mapper for byggeverktøy (~/.m2, ~/.gradle, ~/.cargo)", without: "allowed", with: "allowed" },
  {
    resource: "Innloggingsfiler for verktøy (~/.m2/settings.xml, ~/.gradle/gradle.properties)",
    without: "exposed",
    with: "blocked",
  },
  { resource: "Git-hooks‡, kjøring fra /tmp, SSH-agenten", without: "exposed", with: "blocked" },
  { resource: "Utgående nettverk (HTTPS)", without: "exposed", with: "filtered" },
  { resource: "Private IP-adresser (og localhost på macOS†)", without: "exposed", with: "blocked" },
  {
    resource: "Destruktive git- og gh-kommandoer (push til standardgrenen, force push, merge, sletting)",
    without: "exposed",
    with: "blocked",
  },
  { resource: "Copilot-innlogging og cache for verktøy (bare lesing)", without: "allowed", with: "allowed" },
];

/* The footnote text below the table, repeated as abbr titles so the qualifier is
   available where the marker sits rather than a dozen rows further down. */
const FOOTNOTES: Record<string, string> = {
  "*": "Går gjennom en CONNECT-proxy. Proxyen stopper telemetri og domener som ikke står på tillatlista.",
  "†": "På Linux blir ikke localhost stoppet, og UDP er ikke begrenset, med mindre proxy.forced er på.",
  "‡": "Git-hooks er skrivebeskyttet på macOS. På Linux kan .git/hooks endres hvis ikke bubblewrap er installert.",
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

const STATUSES: Record<StatusKey, { label: ReactNode; variant: "success" | "error" | "warning" }> = {
  allowed: { label: "Tillatt", variant: "success" },
  exposed: { label: "Ikke beskyttet", variant: "error" },
  blocked: { label: "Beskyttet", variant: "success" },
  filtered: {
    label: (
      <>
        Filtrert<abbr title={FOOTNOTES["*"]}>*</abbr>
      </>
    ),
    variant: "warning",
  },
};

const GH_GUARD_LEVELS = [
  { level: "Lese", examples: "gh issue list, gh pr view", access: "Tillatt", variant: "success" as const },
  { level: "Skrive", examples: "gh pr create, gh issue edit", access: "Bare eget repo", variant: "warning" as const },
  { level: "Destruktiv", examples: "gh repo delete, gh pr merge", access: "Alltid stoppet", variant: "error" as const },
];

const ECOSYSTEMS = [
  { name: "JVM", detail: "Gradle, Maven" },
  { name: "Node.js", detail: "npm, pnpm" },
  { name: "Docker", detail: "Compose" },
  { name: "Python", detail: "pip, uv" },
  { name: "Spring Boot", detail: "8080 og PostgreSQL" },
  { name: "Ktor", detail: "8080" },
  { name: "Next.js", detail: "3000" },
  { name: "Flyway", detail: "PostgreSQL på 5432" },
  { name: "Playwright", detail: "nettlesere" },
  { name: "Rust og Go", detail: "standardoppsett" },
];

const STEPS = [
  {
    title: "Installer",
    command: INSTALL_COMMAND,
    description:
      "Homebrew på macOS, apt-arkivet på Debian og Ubuntu (også i WSL2), installasjonsskriptet på alt annet.",
  },
  {
    title: "Sett opp",
    command: "cplt init --write",
    description: "cplt finner verktøyene i prosjektet og lager konfig for sandkassen.",
  },
  {
    title: "Start agenten",
    command: 'cplt -- -p "fix the tests"',
    lang: "en",
    description: "Agenten jobber som vanlig, men kan ikke lese hemmelighetene dine.",
  },
];

const TEAM_TOML = `# Strammer inn sandkassen, gjelder uten godkjenning
[deny]
env = ["VAULT_TOKEN", "NPM_TOKEN"]

# Løsner på sandkassen, virker ikke før \`cplt trust accept\`
[propose]
allow_localhost_any = true

[propose.allow]
ports = [5432]
localhost = [3000]`;

// Program output, shown as cplt prints it.
const GUARD_OUTPUT = `⛔ sandbox restriction: \`gh pr merge\` is not allowed.
This command is classified as destructive
and blocked by gh guard.

Please note this for the human operator
and continue with your remaining work.`;

const INIT_OUTPUT = `$ cplt init
Detected:
  Spring Boot  application.yml + spring-boot-starter
  Flyway       db/migration/ directory
  Docker       Dockerfile + compose.yml
  Gradle       build.gradle.kts
  .env         .env.example found

Generated .cplt.toml:

# Deny access to sensitive env vars
[deny]
env = ["DB_PASSWORD", "API_KEY"]

[propose]
allow_jvm_attach = true

[propose.allow]
ports = [5432]
localhost = [8080]

⚠ allow_docker  Docker detected, grants broad access

Run cplt init --write to save`;

const SHELL_INSTALL_OUTPUT = `$ cplt --shell-install
✓ Added to ~/.zshrc
✓ copilot → cplt (sandboxed)

Restart your shell or: source ~/.zshrc`;

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

function Section({ children }: { children: ReactNode }) {
  return (
    <section>
      <VStack gap="space-16">{children}</VStack>
    </section>
  );
}

function Sub({ title, children }: { title: string; children: ReactNode }) {
  return (
    <VStack gap="space-8">
      <Heading size="xsmall" level="3">
        {title}
      </Heading>
      {children}
    </VStack>
  );
}

/* ---------- Page ---------- */

export default async function CpltPage() {
  const [stars, configKeys] = await Promise.all([getStarCount(), fetchCpltConfigKeys()]);
  return (
    <DocPage
      title={PAGE_TITLE}
      description="cplt holder KI-agenten i en sandkasse. Kjernen i operativsystemet håndhever grensene, så hemmelighetene dine blir der de er."
      toc={TOC}
    >
      <InstallSection stars={stars} />
      <SecurityTableSection />
      <GuardsSection />
      <TeamConfigSection />
      <HowItWorksSection />
      <ProxySection />
      <InitSection />
      <Section>
        <LinkableHeading id="innstillinger" size="medium" level="2">
          Alle innstillinger
        </LinkableHeading>
        <BodyLong>
          Alle innstillingene med forklaring. Søk på navn eller beskrivelse. Beskrivelsene kommer fra cplt og er på
          engelsk.
        </BodyLong>
        <CpltConfigExplorer configKeys={configKeys} />
      </Section>
      <PolicySection />
      <OpenSourceSection />
    </DocPage>
  );
}

/* ---------- Install ---------- */

function InstallSection({ stars }: { stars: number | null }) {
  return (
    <Section>
      <LinkableHeading id="installer" size="medium" level="2">
        Installer cplt
      </LinkableHeading>
      <BodyLong>
        Agenten jobber i prosjektet ditt som vanlig, men får ikke lese hemmeligheter, SSH-nøkler eller skynøkler. Det er
        operativsystemet som stopper den, ikke agenten selv.
      </BodyLong>
      <BodyShort>
        <NextLink href={ARTICLE_HREF} hrefLang="en" className={linkClass}>
          Hvorfor: en sandkasse begrenser prosessen, ikke tokenet (engelsk)
        </NextLink>
      </BodyShort>

      <Box borderWidth="1" borderColor="neutral-subtle" borderRadius="8" className="overflow-hidden">
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
          className="block w-full h-auto"
          aria-label="Opptak fra en økt i sandkassen: agenten prøver å lese ~/.ssh og å sende data til en ekstern adresse. Sandkassen stopper begge."
        >
          <source src="/demos/cplt-demo.webm" type="video/webm" />
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/demos/cplt-demo.gif"
            alt="Agenten prøver å lese ~/.ssh og å sende data til en ekstern adresse. Sandkassen stopper begge."
            className="block w-full"
          />
        </video>
      </Box>

      <InstallPicker
        lang="nb"
        mac={INSTALL_COMMAND}
        linux={CPLT_INSTALL_SCRIPT}
        apt={CPLT_APT_INSTALL}
        windowsNote="cplt har ingen sandkasse på Windows. Installer den i Linux-distroen din under WSL2, der kjernen håndhever sandkassen."
      />
      <BodyLong>
        Arkivet bygges på nytt hver time fra siste versjon. Kom en versjon for noen minutter siden, kan det derfor ta
        opptil en time før du får installert den. Arkivet er et vanlig apt-repo som speiler versjonene våre, ikke en
        pakke i distribusjonen med egen vedlikeholder. På andre distribusjoner, i CI eller når en proxy stopper arkivet,
        bruker du <code className={`${code} break-all`}>{CPLT_INSTALL_SCRIPT}</code>.
      </BodyLong>
      <BodyLong>
        cplt virker på macOS (Apple Seatbelt) og Linux (Landlock og seccomp-BPF), og på Windows bare i WSL2.
      </BodyLong>
      <BodyShort>
        Kildekoden ligger i{" "}
        <a href={GH} className={linkClass}>
          navikt/cplt på GitHub
        </a>
        {stars !== null && (
          <>
            {" "}
            <span role="img" aria-label={`${stars} stjerner på GitHub`}>
              (★ {stars})
            </span>
          </>
        )}
        .
      </BodyShort>
    </Section>
  );
}

/* ---------- Security table ---------- */

const SECURITY_COLS = ["Ressurs", "Uten cplt", "Med cplt"];

function StatusTag({ status }: { status: StatusKey }) {
  const s = STATUSES[status];
  return (
    <Tag size="small" variant={s.variant}>
      {s.label}
    </Tag>
  );
}

function SecurityTableSection() {
  return (
    <Section>
      <LinkableHeading id="sikkerhetsgrense" size="medium" level="2">
        Hva agenten får tilgang til
      </LinkableHeading>
      <BodyLong>Hva agenten kommer til, og hva som er stengt. Kjernen håndhever grensene.</BodyLong>
      <div className="overflow-x-auto">
        <Table size="small" className="table-stack" role="table" aria-labelledby="sikkerhetsgrense">
          <HeaderRow stack cells={SECURITY_COLS} />
          <TableBody role="rowgroup">
            {SECURITY_TABLE.map((row) => (
              <TableRow role="row" key={row.resource}>
                <TableHeaderCell scope="row" role="rowheader">
                  {withFootnoteMarkers(row.resource)}
                </TableHeaderCell>
                <TableDataCell role="cell" data-label={SECURITY_COLS[1]}>
                  <StatusTag status={row.without} />
                </TableDataCell>
                <TableDataCell role="cell" data-label={SECURITY_COLS[2]}>
                  <StatusTag status={row.with} />
                </TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
      <BodyShort size="small" textColor="subtle">
        *{FOOTNOTES["*"]}
        <br />
        †På Linux blir ikke localhost stoppet, og UDP er ikke begrenset, med mindre {c("proxy.forced")} er på.
        <br />
        ‡Git-hooks er skrivebeskyttet på macOS. På Linux kan {c(".git/hooks")} endres hvis ikke bubblewrap er
        installert.
      </BodyShort>
      <BodyLong>
        Kjernen håndhever grensene for filsystemet og systemkallene: Apple Seatbelt på macOS, Landlock og seccomp-BPF på
        Linux. Er {c("bwrap")} installert, isolerer bubblewrap i tillegg med egne namespaces. Vaktene for git og gh
        virker på en annen måte: med egne programmer først i PATH og et kommandofilter som ikke fanger alt.
      </BodyLong>
      <BodyLong>
        Bubblewrap er ikke en grense for nettverket. Sandkassen deler med vilje nettverket med maskinen, slik at proxyen
        virker. Rotfilsystemet monteres skrivebeskyttet, og Landlock styrer tilgangen. Kjente begrensninger står i{" "}
        <a href={`${GH}/blob/main/SECURITY.md`} className={linkClass}>
          SECURITY.md
        </a>
        .
      </BodyLong>
    </Section>
  );
}

/* ---------- Guards ---------- */

function GuardsSection() {
  return (
    <Section>
      <LinkableHeading id="vakter" size="medium" level="2">
        Vakter for gh og git
      </LinkableHeading>
      <BodyLong>
        Vaktene stopper GitHub- og git-operasjoner som ikke kan angres. Agenten kan committe og lage grener, men ikke
        pushe til main eller merge pull requests.
      </BodyLong>

      <Sub title="gh-vakten har tre nivåer">
        <BodyLong>
          Alt er stoppet til det er tillatt. Vakten kjenner 132 {c("gh")}-kommandoer: 51 er tillatt, 64 er stoppet, og
          17 sjekkes mot repoet du står i.
        </BodyLong>
        <div className="overflow-x-auto">
          <Table size="small" className="table-stack" role="table">
            <HeaderRow stack cells={["Nivå", "Eksempler", "Tilgang"]} />
            <TableBody role="rowgroup">
              {GH_GUARD_LEVELS.map((row) => (
                <TableRow role="row" key={row.level}>
                  <TableHeaderCell scope="row" role="rowheader">
                    {row.level}
                  </TableHeaderCell>
                  <TableDataCell role="cell" data-label="Eksempler">
                    {c(row.examples)}
                  </TableDataCell>
                  <TableDataCell role="cell" data-label="Tilgang">
                    <Tag size="small" variant={row.variant}>
                      {row.access}
                    </Tag>
                  </TableDataCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
        <BodyLong>
          {c("gh api")} kan bare kalle {c("/repos/{current-repo}/...")}.
        </BodyLong>
      </Sub>

      <Sub title="git-vakten beskytter standardgrenen">
        <BodyLong>
          Vakten stopper push til standardgrenen og force push til alle grener. Push til en annen gren går gjennom, så
          kontrollen skjer i pull requesten og ikke ved push. Commit, nye grener og rebase går fint.
        </BodyLong>
        <BodyLong>Vil du stoppe all push?</BodyLong>
        <CodeBlock compact>{"cplt config set git_guard.protect_default_branch_only false"}</CodeBlock>
      </Sub>

      <Sub title="Dette ser agenten">
        <BodyLong>Når en vakt stopper en kommando, får agenten denne meldingen:</BodyLong>
        <div lang="en">
          <CodeBlock compact>{GUARD_OUTPUT}</CodeBlock>
        </div>
      </Sub>

      <Sub title="Slå av vaktene">
        <BodyLong>
          Begge vaktene er på som standard. Slå dem av for én kjøring med {c("--no-gh-guard")} eller{" "}
          {c("--no-git-guard")}. Vil du bare se hva de ville stoppet, setter du {c("mode: audit")}.
        </BodyLong>
        <CodeBlock compact>{"cplt --no-gh-guard --no-git-guard"}</CodeBlock>
      </Sub>
    </Section>
  );
}

/* ---------- Team configuration ---------- */

function TeamConfigSection() {
  return (
    <Section>
      <LinkableHeading id="felles-konfig" size="medium" level="2">
        Felles konfig i repoet
      </LinkableHeading>
      <BodyLong>
        Commit {c(".cplt.toml")} til repoet, så får alle utviklerne samme sandkasse. cplt leser fila fra git HEAD, ikke
        fra arbeidskopien, så agenten kan ikke løsne på den.
      </BodyLong>
      <CodeBlock compact>{TEAM_TOML}</CodeBlock>

      <Sub title="[deny] gjelder med en gang">
        <BodyLong>
          {c("[deny]")} kan bare stramme inn sandkassen: stoppe miljøvariabler og nekte tilgang til filstier. Ingen
          trenger å godkjenne den.
        </BodyLong>
      </Sub>

      <Sub title="[propose] må godkjennes">
        <BodyLong>
          {c("[propose]")} ber om flere tillatelser. Hver utvikler godkjenner med {c("cplt trust accept --all")}.
          Godkjenningen gjelder innholdet, så en endring i blokka gjør den ugyldig.
        </BodyLong>
      </Sub>

      <Sub title="Agenten kan ikke endre sin egen sandkasse">
        <BodyLong>
          {c(".cplt.toml")} leses fra git HEAD, så en endring i arbeidskopien endrer ingenting. Fila er dessuten
          skrivebeskyttet i sandkassen. En {c("[deny]")}-blokk som er committet, gjelder derfor alltid. Den kan bare
          stramme inn, så det er ingenting å godkjenne. En {c("[propose]")}-blokk virker ikke før utvikleren kjører{" "}
          {c("cplt trust accept")}, og endres blokka, må den godkjennes på nytt.
        </BodyLong>
      </Sub>
    </Section>
  );
}

/* ---------- How it works ---------- */

function HowItWorksSection() {
  return (
    <Section>
      <LinkableHeading id="slik-virker-det" size="medium" level="2">
        Slik virker det
      </LinkableHeading>
      <BodyLong>Tre steg, så kjører agenten i sandkassen.</BodyLong>
      <VStack as="ol" gap="space-16">
        {STEPS.map((step, i) => (
          <li key={step.title}>
            <Sub title={`${i + 1}. ${step.title}`}>
              <div lang={"lang" in step ? step.lang : undefined}>
                <CodeBlock compact>{step.command}</CodeBlock>
              </div>
              <BodyLong>{step.description}</BodyLong>
            </Sub>
          </li>
        ))}
      </VStack>
      <BodyLong>
        Copilot CLI er standard. {c("--agent")} tar {c("copilot")}, {c("opencode")}, {c("gemini")}, {c("antigravity")},{" "}
        {c("pi")}, {c("claude")}, {c("goose")} og {c("shell")}. Den siste er et skall i sandkassen uten KI.
      </BodyLong>

      <Sub title="Gjør cplt til standard">
        <BodyLong>
          Kjør {c("cplt --shell-install")}, så kjører {c("copilot")} alltid i sandkassen.
        </BodyLong>
        <div lang="en">
          <CodeBlock compact>{SHELL_INSTALL_OUTPUT}</CodeBlock>
        </div>
      </Sub>
    </Section>
  );
}

/* ---------- Proxy and network ---------- */

const T_SUBTLE = "var(--ax-text-neutral-subtle)";
const T_SUCCESS = "var(--ax-text-success)";
const T_DANGER = "var(--ax-text-danger)";
const T_WARNING = "var(--ax-text-warning)";

function ProxyDiagram() {
  return (
    <svg
      viewBox="0 0 820 300"
      className="w-full"
      style={{ maxWidth: "820px", display: "block" }}
      role="img"
      aria-label="Diagram over hvordan cplt sender og filtrerer utgående trafikk"
    >
      <rect width="820" height="300" rx="12" fill="var(--ax-bg-default)" stroke="var(--ax-border-neutral-subtle)" />

      {/* Sandbox container (wraps agent + proxy) */}
      <rect
        x="20"
        y="20"
        width="440"
        height="260"
        rx="10"
        fill="var(--ax-bg-success-soft)"
        stroke={T_SUCCESS}
        strokeWidth="1.5"
        strokeDasharray="6 3"
      />
      <text x="40" y="44" fill={T_SUCCESS} fontSize="13" fontWeight="600">
        cplt-sandkassen
      </text>

      {/* Agent box */}
      <rect
        x="45"
        y="70"
        width="120"
        height="160"
        rx="8"
        fill="var(--ax-bg-neutral-moderate)"
        stroke="var(--ax-border-neutral)"
      />
      <text
        x="105"
        y="145"
        textAnchor="middle"
        fill="var(--ax-text-neutral)"
        fontSize="13"
        fontWeight="600"
        fontFamily="monospace"
      >
        KI-agent
      </text>
      <text x="105" y="167" textAnchor="middle" fill={T_SUBTLE} fontSize="11" fontFamily="monospace">
        curl, fetch, git
      </text>

      <line x1="165" y1="150" x2="225" y2="150" stroke={T_SUBTLE} strokeWidth="2" markerEnd="url(#arrowGray)" />

      {/* Proxy box */}
      <rect
        x="225"
        y="70"
        width="215"
        height="160"
        rx="8"
        fill="var(--ax-bg-default)"
        stroke={T_SUCCESS}
        strokeWidth="1.5"
      />
      <text x="332" y="96" textAnchor="middle" fill={T_SUCCESS} fontSize="14" fontWeight="700">
        CONNECT-proxy
      </text>
      <text x="332" y="114" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        localhost:ephemeral
      </text>

      <rect x="243" y="122" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
      <text x="333" y="138" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        Blokkliste og tillatliste
      </text>
      <rect x="243" y="150" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
      <text x="333" y="166" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        Filter for private IP-er
      </text>
      <rect x="243" y="178" width="180" height="24" rx="4" fill="var(--ax-bg-neutral-moderate)" />
      <text x="333" y="194" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        Vern mot DNS-rebinding
      </text>

      <rect x="258" y="208" width="150" height="30" rx="6" fill="var(--ax-bg-warning-soft)" stroke={T_WARNING} />
      <text x="333" y="228" textAnchor="middle" fill={T_WARNING} fontSize="11" fontWeight="600">
        Logg ✓
      </text>

      {/* Allowed path */}
      <line x1="440" y1="115" x2="540" y2="85" stroke={T_SUCCESS} strokeWidth="2" markerEnd="url(#arrowGreen)" />
      <text x="498" y="86" textAnchor="middle" fill={T_SUCCESS} fontSize="11" fontWeight="600">
        ✓ Tillatt
      </text>
      <rect x="540" y="50" width="260" height="76" rx="8" fill="var(--ax-bg-success-soft)" stroke={T_SUCCESS} />
      <text x="670" y="74" textAnchor="middle" fill={T_SUCCESS} fontSize="13" fontWeight="600">
        Internett
      </text>
      <text x="670" y="94" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        github.com, npm, PyPI, api.openai.com
      </text>
      <text x="670" y="112" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        På tillatlista eller ikke på blokklista
      </text>

      {/* Blocked path */}
      <line x1="440" y1="170" x2="540" y2="200" stroke={T_DANGER} strokeWidth="2" markerEnd="url(#arrowRed)" />
      <text x="498" y="198" textAnchor="middle" fill={T_DANGER} fontSize="11" fontWeight="600">
        ✗ Stoppet
      </text>
      <rect x="540" y="172" width="260" height="76" rx="8" fill="var(--ax-bg-danger-soft)" stroke={T_DANGER} />
      <text x="670" y="196" textAnchor="middle" fill={T_DANGER} fontSize="13" fontWeight="600">
        Forkastet
      </text>
      <text x="670" y="216" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        webhook.site, ngrok.io, pastebin.com
      </text>
      <text x="670" y="234" textAnchor="middle" fill={T_SUBTLE} fontSize="11">
        169.254.x.x, 10.x.x.x, tunneltjenester
      </text>

      <text x="470" y="272" fill={T_SUBTLE} fontSize="11" fontFamily="monospace">
        proxy.blocked_domains
      </text>
      <text x="470" y="289" fill={T_SUBTLE} fontSize="11">
        47 domener · lastes hvert 5. sekund
      </text>
      <text x="660" y="272" fill={T_SUBTLE} fontSize="11" fontFamily="monospace">
        proxy.allowed_domains
      </text>
      <text x="660" y="289" fill={T_SUBTLE} fontSize="11">
        Streng, stenger ved feil
      </text>

      <defs>
        <marker id="arrowGray" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill={T_SUBTLE} />
        </marker>
        <marker id="arrowGreen" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill={T_SUCCESS} />
        </marker>
        <marker id="arrowRed" markerWidth="8" markerHeight="6" refX="8" refY="3" orient="auto">
          <polygon points="0 0, 8 3, 0 6" fill={T_DANGER} />
        </marker>
      </defs>
    </svg>
  );
}

function ProxySection() {
  return (
    <Section>
      <LinkableHeading id="nettverk" size="medium" level="2">
        Nettverksproxy
      </LinkableHeading>
      <BodyLong>
        En lokal CONNECT-proxy filtrerer og logger HTTPS-trafikken fra agenten. Som standard slipper kjernen fortsatt
        gjennom trafikk direkte ut på {c(":443")}, så proxyen ser bare det som sendes til den. Slå på{" "}
        {c("proxy.forced")} for å gjøre proxyen obligatorisk.
      </BodyLong>

      {/* Hidden below md, where its labels would render too small to read; the
          text below says the same. */}
      <div className="hidden md:block">
        <ProxyDiagram />
      </div>
      <BodyLong className="md:hidden">
        Trafikk fra agenten ({c("curl")}, {c("fetch")}, {c("git")}) går til CONNECT-proxyen på{" "}
        {c("localhost:ephemeral")} inne i sandkassen. Proxyen bruker blokklista og tillatlista, et filter for private
        IP-adresser og vern mot DNS-rebinding, og skriver en logg. Tillatt trafikk når internett: github.com, npm, PyPI,
        api.openai.com og alt som står på tillatlista eller ikke står på blokklista. Annen trafikk forkastes:
        webhook.site, ngrok.io, pastebin.com, 169.254.x.x, 10.x.x.x og tunneltjenester. {c("proxy.blocked_domains")} har
        47 domener og lastes på nytt hvert 5. sekund. {c("proxy.allowed_domains")} er streng og stenger ved feil.
      </BodyLong>

      <Sub title="Tving all trafikk gjennom proxyen">
        <BodyLong>
          Som standard slipper kjernen gjennom trafikk direkte på {c(":443")}. En rå socket eller en {c("HTTPS_PROXY")}{" "}
          som ikke er satt, kan derfor gå utenom proxyen. {c("proxy.forced")} lukker den veien: proxyen blir
          obligatorisk, og kjernen slipper bare trafikk ut til proxyporten. Starter ikke proxyen, starter ikke agenten
          heller.
        </BodyLong>
        <BodyLong>
          macOS låser trafikken helt til {c("localhost:<proxy_port>")}. Linux fjerner regelen for {c(":443")}, men
          Landlock filtrerer bare på port, så en smal åpning på proxyporten gjenstår. Det er en kjent begrensning, og
          den følges opp i cplt.
        </BodyLong>
        <CodeBlock compact>{"cplt config set proxy.forced true"}</CodeBlock>
      </Sub>

      <Sub title="Bak en annen proxy">
        <BodyLong>
          Er du bak en proxy på jobben? {c("proxy.upstream")} sender CONNECT-tunnelene videre gjennom den, så du slipper
          å slå av proxyen i cplt. cplt filtrerer domener, logger og sjekker porter <em>før</em> tunnelen sendes videre.
          Et mål som er stoppet, når derfor aldri den andre proxyen. Brukernavn og passord i adressen (basic auth)
          støttes, men bare over http, ikke https.
        </BodyLong>
        <CodeBlock compact>{'cplt config set proxy.upstream "http://proxy.example.com:8080"'}</CodeBlock>
      </Sub>
    </Section>
  );
}

/* ---------- cplt init ---------- */

function InitSection() {
  return (
    <Section>
      <LinkableHeading id="cplt-init" size="medium" level="2">
        Finn oppsettet automatisk
      </LinkableHeading>
      <BodyLong>
        {c("cplt init")} leter i prosjektet etter byggefiler, rammeverk og mønstre, og lager riktig {c(".cplt.toml")}{" "}
        for deg.
      </BodyLong>
      <div lang="en">
        <CodeBlock compact>{INIT_OUTPUT}</CodeBlock>
      </div>

      <Sub title="15 økosystemer">
        <BodyLong>
          For hvert økosystem vet cplt hvilke tillatelser det trenger i sandkassen. Farlige tillatelser får en advarsel.
          Blant økosystemene:
        </BodyLong>
        <Bullets>
          {ECOSYSTEMS.map((eco) => (
            <li key={eco.name}>
              {eco.name} ({eco.detail})
            </li>
          ))}
        </Bullets>
      </Sub>

      <Sub title="Personlig konfig med --global">
        <BodyLong>
          {c("cplt init --global")} finner Gradle wrapper, nettlesere for Playwright, GPG-signering og andre agenter på
          maskinen din. Den skriver til {c("~/.config/cplt/config.toml")}.
        </BodyLong>
      </Sub>
    </Section>
  );
}

/* ---------- Nav policy ---------- */

function PolicySection() {
  return (
    <Section>
      <LinkableHeading id="krav-i-nav" size="medium" level="2">
        Krav i Nav
      </LinkableHeading>
      <Box background="info-soft" borderRadius="8" padding="space-16">
        <VStack gap="space-8">
          <BodyLong>
            Alt arbeid med KI-agenter på Nav-utstyr skal isoleres, også arbeid du gjør for deg selv. Vi anbefaler cplt.
            Velger du noe annet, må det isolere like godt, og du er selv ansvarlig for å sette det opp.
          </BodyLong>
          <BodyShort>
            <NextLink href="/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr" className={linkClass}>
              Sandboxing er påkrevd på Nav-utstyr
            </NextLink>
          </BodyShort>
        </VStack>
      </Box>
    </Section>
  );
}

/* ---------- Open source ---------- */

function OpenSourceSection() {
  return (
    <Section>
      <LinkableHeading id="apen-kildekode" size="medium" level="2">
        Åpen kildekode
      </LinkableHeading>
      <BodyLong>Stol på kjernen, ikke på agenten. cplt er åpen kildekode med MIT-lisens og er laget i Nav.</BodyLong>
      <HStack as="ul" gap="space-24" wrap>
        <li>
          <a href={GH} className={linkClass}>
            GitHub
          </a>
        </li>
        <li>
          <a href={`${GH}/blob/main/SECURITY.md`} className={linkClass}>
            Sikkerhetspolicy
          </a>
        </li>
        <li>
          <a href={`${GH}/blob/main/LICENSE`} className={linkClass}>
            MIT-lisens
          </a>
        </li>
      </HStack>
    </Section>
  );
}
