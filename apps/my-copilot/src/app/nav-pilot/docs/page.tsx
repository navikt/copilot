import { Heading, BodyShort, BodyLong, Box, HGrid, Label, VStack, Tag } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { AltInstall } from "@/components/alt-install";
import { FileExplorer } from "@/components/file-explorer";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import { BackToTop } from "@/components/back-to-top";
import {
  TerminalIcon,
  ArrowsCirclepathIcon,
  CompassIcon,
  MagnifyingGlassIcon,
  TasklistIcon,
  Buildings3Icon,
  WrenchIcon,
  CheckmarkIcon,
  DocPencilIcon,
  PersonGroupIcon,
  LightBulbIcon,
  LayersIcon,
  HandShakeHeartIcon,
  ComponentIcon,
} from "@navikt/aksel-icons";
import { PipelineFlow } from "@/components/pipeline-flow";
import type { Metadata } from "next";
import NextLink from "next/link";

export const metadata: Metadata = {
  title: "nav-pilot dokumentasjon",
  description: "Dokumentasjon for nav-pilot — Navs AI-utviklerverktøy for GitHub Copilot.",
};

/* ═══════════════════════════════════════════════════════════════
   Table of Contents structure
   ═══════════════════════════════════════════════════════════════ */

const DOC_SECTIONS: TocItem[] = [
  {
    id: "introduksjon",
    label: "Introduksjon",
    children: [
      { id: "hva-er-nav-pilot", label: "Hva er nav-pilot?" },
      { id: "hvorfor-nav-pilot", label: "Hvorfor nav-pilot?" },
      { id: "hva-nav-pilot-vet", label: "Hva nav-pilot vet" },
    ],
  },
  {
    id: "kom-i-gang",
    label: "Kom i gang",
    children: [
      { id: "installasjon", label: "Installasjon (5 min)" },
      { id: "personlig-installasjon", label: "Personlig installasjon (valgfritt)" },
      { id: "første-kommandoer", label: "Første kommandoer" },
    ],
  },
  {
    id: "collections",
    label: "Collections",
    children: [
      { id: "tilgjengelige-collections", label: "Tilgjengelige collections" },
      { id: "planning-skills", label: "Planning skills" },
    ],
  },
  {
    id: "planleggingspipelinen",
    label: "Planleggingspipelinen",
    children: [
      { id: "fire-faser", label: "De fire fasene" },
      { id: "skills-i-detalj", label: "Skills i detalj" },
    ],
  },
  {
    id: "sync-og-oppdatering",
    label: "Sync og oppdatering",
    children: [
      { id: "automatisk-sync", label: "Automatisk sync" },
      { id: "lokal-sync", label: "Lokal sync" },
      { id: "tilpasse-sync", label: "Tilpasse synkronisering" },
      { id: "sync-faq", label: "FAQ" },
    ],
  },
  {
    id: "cli-referanse",
    label: "CLI-referanse",
    children: [
      { id: "installer-cli", label: "Installer CLI" },
      { id: "oppgrader-cli", label: "Oppgrader CLI" },
      { id: "kommandooversikt", label: "Kommandooversikt" },
    ],
  },
  {
    id: "slik-fungerer-det",
    label: "Slik fungerer det",
    children: [
      { id: "filstruktur", label: "Filstruktur" },
      { id: "livssyklus", label: "Livssyklus" },
    ],
  },
  {
    id: "ressurser",
    label: "Ressurser",
    children: [
      { id: "arkitektur", label: "Arkitektur" },
      { id: "designprinsipper", label: "Designprinsipper" },
      { id: "lenker", label: "Lenker" },
    ],
  },
];

/* ═══════════════════════════════════════════════════════════════
   Data
   ═══════════════════════════════════════════════════════════════ */

const COLLECTIONS = [
  {
    name: "kotlin-backend",
    description: "Kotlin/Ktor og Spring Boot på Nais",
    agents: 6,
    skills: 13,
    bestFor: "Backend-API-er og hendelseskonsumenter",
    details: {
      agents: "auth, kafka, nais, observability, security-champion, nav-pilot",
      skills:
        "api-design, flyway-migration, java-to-kotlin, kotlin-app-config, ktor-scaffold, observability-setup, security-review, threat-model, tokenx-auth, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions:
        "kotlin-ktor, kotlin-spring, security-owasp, testing, testing-kotlin, github-actions, docker, database",
      prompts: "ktor-endpoint, spring-boot-endpoint, kafka-topic, nais-manifest",
    },
  },
  {
    name: "frontend",
    description: "Rammeverk-uavhengig frontend (Astro, Remix, Vite …)",
    agents: 4,
    skills: 7,
    bestFor: "Frontends som ikke bruker Next.js",
    details: {
      agents: "accessibility, aksel, forfatter, nav-pilot",
      skills:
        "aksel-spacing, playwright-testing, web-design-reviewer, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions: "testing, testing-typescript, accessibility, github-actions, docker",
      prompts: "aksel-component, nais-manifest",
    },
  },
  {
    name: "nextjs-frontend",
    description: "Next.js med Aksel Design System",
    agents: 4,
    skills: 7,
    bestFor: "Innbygger- og saksbehandler-frontends",
    details: {
      agents: "accessibility, aksel, forfatter, nav-pilot",
      skills:
        "aksel-spacing, playwright-testing, web-design-reviewer, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions: "nextjs-aksel, performance, testing, testing-typescript, accessibility, github-actions, docker",
      prompts: "aksel-component, nextjs-api-route, nais-manifest",
    },
  },
  {
    name: "fullstack",
    description: "Komplett stack (backend + frontend)",
    agents: 10,
    skills: 16,
    bestFor: "Team som eier hele stacken",
    details: {
      agents:
        "accessibility, aksel, auth, code-review, forfatter, kafka, nais, observability, security-champion, nav-pilot",
      skills:
        "aksel-spacing, api-design, flyway-migration, java-to-kotlin, kotlin-app-config, ktor-scaffold, observability-setup, playwright-testing, security-review, threat-model, tokenx-auth, web-design-reviewer, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions:
        "kotlin-ktor, kotlin-spring, golang, nextjs-aksel, performance, security-owasp, testing, testing-kotlin, testing-typescript, accessibility, github-actions, docker, database",
      prompts:
        "ktor-endpoint, spring-boot-endpoint, kafka-topic, nais-manifest, aksel-component, nextjs-api-route, golang-service",
    },
  },
  {
    name: "platform",
    description: "Nais, observability, sikkerhet og Go",
    agents: 4,
    skills: 8,
    bestFor: "Plattform- og DevOps-team",
    details: {
      agents: "nais, observability, security-champion, nav-pilot",
      skills:
        "observability-setup, security-review, threat-model, workstation-security, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions: "golang, security-owasp, github-actions, docker",
      prompts: "golang-service, nais-manifest",
    },
  },
];

const PLANNING_SKILLS = [
  {
    name: "$nav-deep-interview",
    purpose: "Strukturert intervju som avdekker blinde flekker (personvern, auth, avhengigheter)",
    details: [
      "Personvern og data — PII-kategorier, dataklassifisering, sletteregler",
      "Plattform og auth — caller-type, avhengigheter, feilhåndtering",
      "Observerbarhet — forretningsmetrikker, varsling, on-call",
      "Team og prosess — avhengigheter, deadlines, erfaring",
    ],
    refs: "data-classification.md, blind-spots.md (25+ vanlige blinde flekker fra ekte Nav-repoer)",
  },
  {
    name: "$nav-plan",
    purpose: "Arkitekturbeslutningstrær → konkret Nais-manifest, CI/CD og prosjektstruktur",
    details: [
      "Auth-beslutningstre — fra caller-type til Nais-konfigurasjon",
      "Kommunikasjonstre — REST, Kafka, SSE",
      "Database-tre — PostgreSQL, BigQuery, Redis, stateless",
      "accessPolicy-tre — inbound og outbound regler",
    ],
    refs: "decision-trees.md, nais-templates.md (5 arketyper)",
  },
  {
    name: "$nav-architecture-review",
    purpose: "Flerperspektiv-review → Architecture Decision Record (ADR)",
    details: [
      "Arkitektur — passer dette i Navs arkitektur? Enklere alternativer?",
      "Sikkerhet — data, auth, tilgang, PII",
      "Plattform — Nais, ressurser, observerbarhet, CI/CD",
    ],
    refs: "adr-template.md, nav-principles.md (Team First, essensiell kompleksitet, DORA)",
  },
  {
    name: "$nav-troubleshoot",
    purpose: "Diagnostiske trær for vanlige Nav-plattformproblemer",
    details: [
      "Pod krasjer (CrashLoopBackOff) — status → logs → events → ressurser",
      "401/403 — token → issuer → audience → expiry → JWKS → accessPolicy",
      "Kafka consumer lag — konsument oppe? → feil i log? → poison pill?",
      "DB-tilkobling feiler — Cloud SQL oppe? → env-vars? → Flyway? → pool exhaustion?",
      "Treg responstid — Prometheus → Tempo trace → DB EXPLAIN",
      "Deploy feiler — Actions-feil? → Nais deploy-feil? → pod starter ikke?",
    ],
    refs: "diagnostic-trees.md",
  },
];

const CLI_COMMANDS = [
  { command: "nav-pilot", description: "Interaktivt: installer, oppgrader eller start Copilot-sandkassen (cplt)" },
  { command: "nav-pilot install <collection>", description: "Installer en collection i repoet ditt" },
  {
    command: "nav-pilot install --user",
    description: "Installer agenter, skills og instruksjoner til ~/.copilot (alle repoer)",
  },
  { command: "nav-pilot install --dry-run <collection>", description: "Forhåndsvis hva som installeres" },
  { command: "nav-pilot install --force <collection>", description: "Overskriv lokalt endrede filer" },
  { command: "nav-pilot list", description: "Vis tilgjengelige collections" },
  { command: "nav-pilot list --items", description: "Vis alle tilgjengelige agenter, skills, etc." },
  { command: "nav-pilot add <type> <name>", description: "Installer enkeltkomponent (agent, skill, etc.)" },
  { command: "nav-pilot status", description: "Vis installerte filer og integritet" },
  { command: "nav-pilot uninstall", description: "Fjern alle installerte filer" },
  { command: "nav-pilot sync", description: "Sjekk om oppdateringer finnes (exit 1 hvis ja)" },
  { command: "nav-pilot sync --apply", description: "Oppdater filer direkte" },
  { command: "nav-pilot sync --json", description: "Maskinlesbar JSON-output" },
  {
    command: "<command> --json",
    description: "Globalt flagg: JSON-output på alle kommandoer (install, add, status, sync, list, export)",
  },
  { command: "nav-pilot env", description: "Skriv shell-eksport for Copilot CLI-integrasjon" },
  { command: "nav-pilot update", description: "Oppdater nav-pilot CLI til nyeste versjon" },
  { command: "nav-pilot feedback", description: "Rapporter feil — åpner GitHub issue med diagnostikk" },
  { command: "nav-pilot feedback --feature", description: "Foreslå ny funksjonalitet" },
  { command: "nav-pilot export opencode", description: "Eksporter til .opencode/-format (OpenCode / oh-my-openagent)" },
  { command: "nav-pilot export opencode --user", description: "Eksporter til ~/.config/opencode/ (globalt)" },
  { command: "nav-pilot export opencode --dry-run", description: "Forhåndsvis hva som eksporteres" },
  { command: "nav-pilot version", description: "Vis versjonsinformasjon" },
];

/* ═══════════════════════════════════════════════════════════════
   Page Component
   ═══════════════════════════════════════════════════════════════ */

export default function NavPilotDocs() {
  return (
    <main>
      <PageHero
        title="nav-pilot dokumentasjon"
        description="Alt du trenger for å komme i gang med nav-pilot."
        badge={
          <Tag variant="info" size="small" className="uppercase tracking-wide">
            Beta
          </Tag>
        }
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            {/* ── Left sidebar: Table of Contents ── */}
            <aside className="hidden lg:block w-56 shrink-0">
              <div className="sticky top-6">
                <TableOfContents items={DOC_SECTIONS} />
              </div>
            </aside>

            {/* ── Main content ── */}
            <div className="min-w-0 flex-1">
              <VStack gap={{ xs: "space-32", md: "space-40" }}>
                <IntroductionSection />
                <QuickStartSection />
                <CollectionsSection />
                <PipelineSection />
                <SyncSection />
                <CliReferenceSection />
                <HowItWorksSection />
                <ResourcesSection />
              </VStack>
            </div>
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 1: Introduksjon
   ═══════════════════════════════════════════════════════════════ */

function IntroductionSection() {
  return (
    <section id="introduksjon">
      <VStack gap="space-24">
        {/* What is nav-pilot */}
        <div id="hva-er-nav-pilot">
          <LinkableHeading size="medium" level="2">
            Hva er nav-pilot?
          </LinkableHeading>
          <BodyLong className="mt-3 mb-6" style={{ color: "#475569" }}>
            nav-pilot gjør GitHub Copilot til en Nav-ekspert. I stedet for å huske alle mønstrene, beslutningstrærne og
            fellene selv — spør{" "}
            <code
              className="text-sm font-mono rounded px-1.5 py-0.5"
              style={{ background: "#f1f5f9", color: "#3b82f6" }}
            >
              @nav-pilot
            </code>
            .
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            nav-pilot er en samling av <strong>én agent, fire skills og fire collections</strong> som koder inn Navs
            institusjonelle kunnskap som kjørbare arbeidsflyter. CLI-verktøyet installerer markdown-filer — selve
            AI-funksjonaliteten kjøres av GitHub Copilot.
          </BodyLong>

          {/* Component overview cards */}
          <div className="mt-6 grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))" }}>
            {[
              { name: "@nav-pilot", desc: "Planleggingsagent — din inngangsport", color: "#3b82f6", Icon: CompassIcon },
              {
                name: "$nav-deep-interview",
                desc: "Avdekker blinde flekker (personvern, auth, avhengigheter)",
                color: "#a78bfa",
                Icon: MagnifyingGlassIcon,
              },
              {
                name: "$nav-plan",
                desc: "Beslutningstrær → Nais-manifest, CI/CD, prosjektstruktur",
                color: "#60a5fa",
                Icon: TasklistIcon,
              },
              {
                name: "$nav-architecture-review",
                desc: "Flerperspektiv-review → ADR",
                color: "#2dd4bf",
                Icon: Buildings3Icon,
              },
              {
                name: "$nav-troubleshoot",
                desc: "Diagnostikk for pod-krasj, 401-er, Kafka-lag, DB-feil",
                color: "#fb923c",
                Icon: WrenchIcon,
              },
            ].map((c) => (
              <div
                key={c.name}
                className="rounded-lg overflow-hidden"
                style={{ background: "white", border: "1px solid #e2e8f0" }}
              >
                <div style={{ height: "3px", background: c.color }} />
                <div style={{ padding: "0.75rem 1rem" }}>
                  <div className="flex items-center gap-2">
                    <c.Icon aria-hidden fontSize="1.25rem" style={{ color: c.color }} />
                    <code className="text-sm font-mono font-semibold" style={{ color: c.color }}>
                      {c.name}
                    </code>
                  </div>
                  <BodyShort size="small" className="mt-1.5" style={{ color: "#475569" }}>
                    {c.desc}
                  </BodyShort>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* At a Glance — collection links */}
        <div>
          <Heading size="small" level="3" className="mb-4" style={{ color: "#334155" }}>
            Velg din stack
          </Heading>
          <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-4">
            {COLLECTIONS.map((c, i) => {
              const colors = ["#6366f1", "#06b6d4", "#8b5cf6", "#10b981"];
              const color = colors[i % colors.length];
              return (
                <a
                  key={c.name}
                  href="#tilgjengelige-collections"
                  className="no-underline block rounded-lg border overflow-hidden transition-all hover:shadow-md"
                  style={{ borderColor: "#e2e8f0" }}
                >
                  <div style={{ height: "3px", background: color }} />
                  <div style={{ padding: "1rem" }}>
                    <Label size="small" style={{ color }}>
                      {c.name}
                    </Label>
                    <BodyShort size="small" className="mt-1" style={{ color: "#64748b" }}>
                      {c.description}
                    </BodyShort>
                  </div>
                </a>
              );
            })}
          </HGrid>
        </div>

        {/* Why nav-pilot */}
        <div id="hvorfor-nav-pilot">
          <LinkableHeading size="small" level="3">
            Hvorfor nav-pilot?
          </LinkableHeading>
          <BodyLong className="mt-3" style={{ color: "#475569" }}>
            oh-my-openagent og lignende verktøy bygger bedre <em>orkestrering</em> — multi-agent-delegering,
            parallellkjøring og selvkorrigering. nav-pilot bygger bedre <em>kunnskap</em>. Orkestrering blir
            standardvare — institusjonell kunnskap er vanskelig å kopiere.
          </BodyLong>
          <div className="overflow-x-auto mt-4">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}></th>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#94a3b8" }}>
                    oh-my-openagent
                  </th>
                  <th className="text-left py-2 font-semibold" style={{ color: "#10b981" }}>
                    nav-pilot ✦
                  </th>
                </tr>
              </thead>
              <tbody>
                {[
                  ["Fokus", "Orkestrering og multi-agent", "Institusjonell kunnskap"],
                  ["Inngangspunkt", "ultrawork (terminal)", "Terminal, VS Code, JetBrains, GitHub.com"],
                  ["Kunnskap", "Generisk koding", "Navs kunnskapsbase"],
                  ["Auth", "Vet ikke hva TokenX er", "Velger riktig auth basert på caller-type"],
                  ["Plattform", "Vet ikke hva Nais er", "Genererer Nais-manifest med riktig accessPolicy"],
                  ["Oppdateringer", "git pull / manuelt", "Auto-sync workflow (ukentlig PR)"],
                ].map(([feature, generic, navPilot]) => (
                  <tr key={feature} style={{ borderBottom: "1px solid #e2e8f0" }}>
                    <td className="py-2.5 pr-4 font-medium" style={{ color: "#334155" }}>
                      {feature}
                    </td>
                    <td className="py-2.5 pr-4" style={{ color: "#cbd5e1" }}>
                      <span className="mr-1.5" style={{ color: "#e2e8f0" }}>
                        –
                      </span>
                      {generic}
                    </td>
                    <td
                      className="py-2.5 rounded-sm"
                      style={{ color: "#475569", background: "#f0fdf4", paddingLeft: "0.5rem" }}
                    >
                      <span className="inline-flex items-center gap-1.5">
                        <CheckmarkIcon aria-hidden fontSize="0.875rem" style={{ color: "#10b981", flexShrink: 0 }} />
                        {navPilot}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* What nav-pilot knows */}
        <div id="hva-nav-pilot-vet">
          <LinkableHeading size="small" level="3">
            Hva nav-pilot vet som Copilot ikke vet
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Copilot er god på kode, men vet ingenting om:
          </BodyShort>
          <VStack gap="space-4" className="mt-4">
            {[
              "At innbyggere bruker ID-porten, men saksbehandlere bruker Azure AD",
              <>
                At du trenger <code className="font-mono text-xs">accessPolicy.inbound</code> i Nais-manifestet, ellers
                kan ingen kalle tjenesten din
              </>,
              "At HikariCP default pool (10) er for stort for containere — start med 3",
              "At du aldri skal sette CPU-limits i Nais (bare requests)",
              "At PII aldri skal logges — logg sakId, ikke fnr",
              "At Chainguard-images er standard i Nav, ikke distroless",
              <>
                At Rapids &amp; Rivers-meldinger trenger <code className="font-mono text-xs">@event_name</code> og{" "}
                <code className="font-mono text-xs">demandValue</code>
              </>,
            ].map((item, i) => (
              <div
                key={i}
                className="flex items-start gap-3 rounded-lg"
                style={{ padding: "0.5rem 0.75rem", background: "#f0fdf4" }}
              >
                <CheckmarkIcon
                  aria-hidden
                  style={{ color: "#10b981", fontSize: "0.875rem", marginTop: "0.125rem", flexShrink: 0 }}
                />
                <BodyShort size="small" style={{ color: "#475569" }}>
                  {item}
                </BodyShort>
              </div>
            ))}
          </VStack>
          <BodyShort size="small" className="mt-4" style={{ color: "#64748b", fontStyle: "italic" }}>
            Denne kunnskapen er kodet inn i nav-pilots beslutningstrær, blinde-flekker-sjekklister og diagnostiske trær.
          </BodyShort>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 2: Kom i gang
   ═══════════════════════════════════════════════════════════════ */

function QuickStartSection() {
  return (
    <section id="kom-i-gang">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Kom i gang
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Fra null til fungerende nav-pilot på 5 minutter.
          </BodyLong>
        </div>

        <div id="installasjon">
          <LinkableHeading size="small" level="3">
            Installasjon (5 min)
          </LinkableHeading>

          <div className="mt-4">
            <div className="flex items-center gap-2 mb-2">
              <span
                className="flex items-center justify-center rounded-full font-bold text-xs"
                style={{ width: "1.5rem", height: "1.5rem", background: "#dbeafe", color: "#2563eb" }}
              >
                1
              </span>
              <Label size="small" style={{ color: "#334155" }}>
                Installer nav-pilot CLI
              </Label>
            </div>
            <CodeBlock compact>{`brew install navikt/tap/nav-pilot`}</CodeBlock>
            <AltInstall />
          </div>

          <div className="mt-6">
            <div className="flex items-center gap-2 mb-2">
              <span
                className="flex items-center justify-center rounded-full font-bold text-xs"
                style={{ width: "1.5rem", height: "1.5rem", background: "#dbeafe", color: "#2563eb" }}
              >
                2
              </span>
              <Label size="small" style={{ color: "#334155" }}>
                Installer en collection i repoet ditt
              </Label>
            </div>
            <CodeBlock compact>
              {`cd /path/to/your/repo
nav-pilot`}
            </CodeBlock>
          </div>

          <div className="mt-6">
            <div className="flex items-center gap-2 mb-2">
              <span
                className="flex items-center justify-center rounded-full font-bold text-xs"
                style={{ width: "1.5rem", height: "1.5rem", background: "#dbeafe", color: "#2563eb" }}
              >
                3
              </span>
              <Label size="small" style={{ color: "#334155" }}>
                Bruk nav-pilot
              </Label>
            </div>
            <BodyLong className="mt-1 mb-3" size="small" style={{ color: "#64748b" }}>
              Du kan bruke nav-pilot på tre måter — velg den som passer deg best:
            </BodyLong>
            <div className="space-y-4">
              <div>
                <Label size="small" style={{ color: "#64748b" }}>
                  Terminal (GitHub Copilot CLI)
                </Label>
                <div className="mt-1">
                  <CodeBlock compact>
                    {`copilot --agent nav-pilot --prompt "Jeg trenger en ny tjeneste som behandler dagpengesøknader"`}
                  </CodeBlock>
                </div>
              </div>
              <div>
                <Label size="small" style={{ color: "#64748b" }}>
                  VS Code / JetBrains (Copilot Chat)
                </Label>
                <div className="mt-1">
                  <CodeBlock compact>
                    {`@nav-pilot Jeg trenger en ny tjeneste som behandler dagpengesøknader`}
                  </CodeBlock>
                </div>
              </div>
              <div>
                <Label size="small" style={{ color: "#64748b" }}>
                  nav-pilot CLI (interaktiv)
                </Label>
                <div className="mt-1">
                  <CodeBlock compact>{`nav-pilot`}</CodeBlock>
                </div>
                <BodyLong className="mt-1" size="small" style={{ color: "#94a3b8" }}>
                  Starter interaktiv modus — sjekker oppdateringer og tilbyr å starte Copilot med valgt agent.
                </BodyLong>
              </div>
            </div>
          </div>
        </div>

        <div id="personlig-installasjon">
          <LinkableHeading size="small" level="3">
            Personlig installasjon (valgfritt)
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            I tillegg til repo-installasjon kan du installere agenter, skills og instruksjoner til hjemmemappen. De blir
            da tilgjengelige i <em>alle</em> repoer uten å endre hvert enkelt.
          </BodyLong>
          <div className="mt-4">
            <CodeBlock compact>{`nav-pilot install --user`}</CodeBlock>
          </div>
          <BodyLong className="mt-3" size="small" style={{ color: "#64748b" }}>
            Filene installeres til <code className="font-mono text-xs">~/.copilot/</code>. Agenter og skills plukkes opp
            automatisk av alle Copilot-klienter. Instruksjoner krever{" "}
            <code className="font-mono text-xs">COPILOT_CUSTOM_INSTRUCTIONS_DIRS</code> og fungerer kun med Copilot CLI
            — nav-pilot setter denne automatisk i interaktiv modus.
          </BodyLong>
          <BodyLong className="mt-2" size="small" style={{ color: "#64748b" }}>
            For direkte bruk av cplt, legg til i shell-profilen:
          </BodyLong>
          <div className="mt-2">
            <CodeBlock compact>{`eval "$(nav-pilot env)"`}</CodeBlock>
          </div>
        </div>

        <div id="første-kommandoer">
          <LinkableHeading size="small" level="3">
            Første kommandoer
          </LinkableHeading>
          <BodyLong className="mt-2 mb-4" style={{ color: "#475569" }}>
            Etter installasjon kan du bruke disse kommandoene for å komme i gang:
          </BodyLong>
          <div className="space-y-3">
            {[
              { label: "Se hva som ble installert", cmd: "nav-pilot status" },
              { label: "Vis alle tilgjengelige collections", cmd: "nav-pilot list" },
              {
                label: "Installer en ekstra agent eller skill",
                cmd: "nav-pilot add agent security-champion\nnav-pilot add skill postgresql-review",
              },
              { label: "Sjekk om det finnes oppdateringer", cmd: "nav-pilot sync" },
              { label: "Rapporter feil eller foreslå forbedringer", cmd: "nav-pilot feedback" },
            ].map((item) => (
              <div key={item.label}>
                <Label size="small" style={{ color: "#64748b" }}>
                  {item.label}
                </Label>
                <div className="mt-1">
                  <CodeBlock compact>{item.cmd}</CodeBlock>
                </div>
              </div>
            ))}
          </div>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 3: Collections
   ═══════════════════════════════════════════════════════════════ */

function CollectionsSection() {
  return (
    <section id="collections">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Collections
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Collections er ferdigpakkede sett med agenter, skills, instruksjoner og prompts organisert etter
            team-arketype. Velg din stack og få en komplett, testet pakke.
          </BodyLong>
        </div>

        {/* Overview table */}
        <div id="tilgjengelige-collections">
          <LinkableHeading size="small" level="3">
            Tilgjengelige collections
          </LinkableHeading>

          <div className="overflow-x-auto mt-4">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  {["Collection", "Beskrivelse", "Agenter", "Skills", "Best for"].map((h) => (
                    <th key={h} className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {COLLECTIONS.map((c) => (
                  <tr key={c.name} style={{ borderBottom: "1px solid #e2e8f0" }}>
                    <td className="py-3 pr-4">
                      <code
                        className="text-sm font-mono rounded px-1.5 py-0.5 font-semibold"
                        style={{ background: "#f1f5f9", color: "#3b82f6" }}
                      >
                        {c.name}
                      </code>
                    </td>
                    <td className="py-3 pr-4" style={{ color: "#475569" }}>
                      {c.description}
                    </td>
                    <td className="py-3 pr-4 text-center" style={{ color: "#475569" }}>
                      {c.agents}
                    </td>
                    <td className="py-3 pr-4 text-center" style={{ color: "#475569" }}>
                      {c.skills}
                    </td>
                    <td className="py-3" style={{ color: "#475569" }}>
                      {c.bestFor}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Planning skills table */}
        <div id="planning-skills">
          <LinkableHeading size="small" level="3">
            Planning skills
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Alle collections inkluderer fire planning skills som utgjør <strong>nav-pilot-pipelinen</strong>:
          </BodyShort>
          <div className="overflow-x-auto">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                    Skill
                  </th>
                  <th className="text-left py-2 font-semibold" style={{ color: "#334155" }}>
                    Formål
                  </th>
                </tr>
              </thead>
              <tbody>
                {PLANNING_SKILLS.map((s) => (
                  <tr key={s.name} style={{ borderBottom: "1px solid #e2e8f0" }}>
                    <td className="py-2 pr-4">
                      <code
                        className="text-sm font-mono rounded px-1.5 py-0.5"
                        style={{ background: "#f1f5f9", color: "#3b82f6" }}
                      >
                        {s.name}
                      </code>
                    </td>
                    <td className="py-2" style={{ color: "#475569" }}>
                      {s.purpose}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 4: Planleggingspipelinen
   ═══════════════════════════════════════════════════════════════ */

function PipelineSection() {
  return (
    <section id="planleggingspipelinen">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Planleggingspipelinen
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot jobber i fire faser med eksplisitte stopp mellom hver. Du bestemmer når du går videre — nav-pilot
            foreslår, du godkjenner.
          </BodyLong>
        </div>

        {/* Pipeline diagram */}
        <div id="fire-faser">
          <LinkableHeading size="small" level="3">
            De fire fasene
          </LinkableHeading>

          <div className="mt-6">
            <PipelineFlow />
          </div>
        </div>

        {/* Skills in detail */}
        <div id="skills-i-detalj">
          <LinkableHeading size="small" level="3">
            Skills i detalj
          </LinkableHeading>

          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155", whiteSpace: "nowrap" }}>
                    Skill
                  </th>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                    Formål
                  </th>
                  <th className="text-left py-2 font-semibold" style={{ color: "#334155" }}>
                    Dekker
                  </th>
                </tr>
              </thead>
              <tbody>
                {PLANNING_SKILLS.map((skill) => (
                  <tr key={skill.name} style={{ borderBottom: "1px solid #e2e8f0", verticalAlign: "top" }}>
                    <td className="py-3 pr-4" style={{ whiteSpace: "nowrap" }}>
                      <code className="text-xs font-mono font-medium" style={{ color: "#475569" }}>
                        {skill.name}
                      </code>
                    </td>
                    <td className="py-3 pr-4" style={{ color: "#475569" }}>
                      {skill.purpose}
                    </td>
                    <td className="py-3" style={{ color: "#64748b" }}>
                      <div className="flex flex-wrap gap-1.5">
                        {skill.details.map((d) => {
                          const label = d.split("—")[0].trim();
                          return (
                            <span
                              key={d}
                              className="inline-block text-xs rounded-full px-2 py-0.5"
                              style={{ background: "#f1f5f9", color: "#475569" }}
                            >
                              {label}
                            </span>
                          );
                        })}
                      </div>
                      <BodyShort size="small" className="mt-1.5" style={{ color: "#94a3b8", fontSize: "0.6875rem" }}>
                        {skill.refs}
                      </BodyShort>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 5: Sync og oppdatering
   ═══════════════════════════════════════════════════════════════ */

function SyncSection() {
  return (
    <section id="sync-og-oppdatering">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Sync og oppdatering
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Copilot-tilpasninger i navikt/copilot oppdateres jevnlig. Hold repoet ditt oppdatert med automatisk sync
            eller lokale kommandoer.
          </BodyLong>
        </div>

        {/* Sync workflows */}
        <VStack gap="space-16">
          <div id="automatisk-sync">
            <div className="flex items-center gap-2 mb-2">
              <ArrowsCirclepathIcon fontSize="1.125rem" style={{ color: "#64748b" }} aria-hidden />
              <Heading size="xsmall" level="3">
                Automatisk sync
              </Heading>
            </div>
            <BodyShort size="small" className="mb-4" style={{ color: "#475569" }}>
              GitHub Actions-workflow som åpner PR-er automatisk — som Dependabot, men for Copilot-tilpasninger. PR-en
              viser hvilke filer som er oppdatert med lenker til kilderepoet.
            </BodyShort>
            <Label size="small" className="mb-1" style={{ color: "#64748b" }}>
              copilot-sync.yml
            </Label>
            <CodeBlock compact>
              {`name: Copilot Customization Sync
on:
  schedule:
    - cron: '0 7 * * 1'  # Mandager kl 07:00
  workflow_dispatch:
jobs:
  sync:
    uses: navikt/copilot/.github/workflows/copilot-customization-sync.yml@main
    permissions:
      contents: write
      pull-requests: write`}
            </CodeBlock>
          </div>

          <div id="lokal-sync">
            <div className="flex items-center gap-2 mb-2">
              <TerminalIcon fontSize="1.125rem" style={{ color: "#64748b" }} aria-hidden />
              <Heading size="xsmall" level="3">
                Lokal sync
              </Heading>
            </div>
            <BodyShort size="small" className="mb-4" style={{ color: "#475569" }}>
              Bruk CLI-verktøyet for å sjekke og oppdatere filer lokalt. Sammenligner SHA-256-hasher mellom lokale filer
              og kilderepoet.
            </BodyShort>
            <div className="space-y-3">
              {[
                { label: "Sjekk om oppdateringer finnes", cmd: "nav-pilot sync" },
                { label: "Oppdater filer direkte", cmd: "nav-pilot sync --apply" },
                { label: "Maskinlesbar JSON-output", cmd: "nav-pilot sync --json" },
              ].map((item) => (
                <div key={item.label}>
                  <Label size="small" style={{ color: "#64748b" }}>
                    {item.label}
                  </Label>
                  <div className="mt-1">
                    <CodeBlock compact>{item.cmd}</CodeBlock>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </VStack>

        {/* Detection logic */}
        <div>
          <Heading size="xsmall" level="3" className="mb-3" style={{ color: "#334155" }}>
            Hvordan nav-pilot finner filer
          </Heading>
          <BodyShort size="small" className="mb-3" style={{ color: "#475569" }}>
            <strong>State-baserte repoer</strong> (brukte <code className="font-mono text-xs">nav-pilot install</code>):
            state-filen sporer nøyaktig hvilke filer som ble installert.
          </BodyShort>
          <BodyShort size="small" className="mb-3" style={{ color: "#475569" }}>
            <strong>Klassiske repoer</strong> (kopierte filer manuelt): nav-pilot auto-oppdager filer som også finnes i
            kilderepoet:
          </BodyShort>
          <ul className="text-sm space-y-1" style={{ color: "#64748b", paddingLeft: "1.25rem" }}>
            <li>
              <code className="font-mono text-xs">.github/agents/*.agent.md</code>
            </li>
            <li>
              <code className="font-mono text-xs">.github/instructions/*.instructions.md</code>
            </li>
            <li>
              <code className="font-mono text-xs">.github/prompts/*.prompt.md</code>
            </li>
            <li>
              <code className="font-mono text-xs">.github/skills/*/</code> (hele kataloger)
            </li>
          </ul>
          <BodyShort size="small" className="mt-3" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            AGENTS.md og .github/copilot-instructions.md oppdateres aldri automatisk — de er alltid repo-spesifikke.
          </BodyShort>
        </div>

        {/* Tilpasse synkronisering */}
        <div id="tilpasse-sync">
          <div className="flex items-center gap-2 mb-2">
            <WrenchIcon fontSize="1.125rem" style={{ color: "#64748b" }} aria-hidden />
            <Heading size="xsmall" level="3">
              Tilpasse synkronisering
            </Heading>
          </div>
          <BodyShort size="small" className="mb-3" style={{ color: "#475569" }}>
            Trenger du å fjerne rammeverk-spesifikke filer (f.eks. Next.js-instruksjoner i et Astro-prosjekt)? Opprett{" "}
            <code className="font-mono text-xs">.github/copilot-sync.json</code> med overrides:
          </BodyShort>
          <CodeBlock compact>
            {`{
  "overrides": {
    ".github/instructions/nextjs-aksel.instructions.md": { "action": "delete" },
    ".github/instructions/performance.instructions.md": { "action": "delete" },
    ".github/prompts/nextjs-api-route.prompt.md": { "action": "delete" }
  }
}`}
          </CodeBlock>
          <BodyShort size="small" className="mt-3" style={{ color: "#475569" }}>
            Filer med <code className="font-mono text-xs">{`"action": "delete"`}</code> blir fjernet ved neste sync og
            ikke lagt til igjen. Alternativt kan du installere <code className="font-mono text-xs">frontend</code>
            -collectionet som allerede utelater Next.js-spesifikke filer.
          </BodyShort>
          <BodyShort size="small" className="mt-2" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Sletter du en fil manuelt uten override, markeres den som «ignorert» og gjenopprettes ikke av sync. Legg den
            til igjen med <code className="font-mono text-xs">nav-pilot add</code> hvis du ombestemmer deg.
          </BodyShort>
        </div>

        {/* FAQ */}
        <div id="sync-faq">
          <LinkableHeading size="small" level="3">
            FAQ
          </LinkableHeading>
          <VStack gap="space-12" className="mt-4">
            {[
              {
                q: "Trenger jeg en GitHub-token eller secret?",
                a: "Nei. Workflowen bruker standard GITHUB_TOKEN og leser offentlige kildefiler.",
              },
              {
                q: "Hva om jeg har tilpasset en fil lokalt?",
                a: "PR-en viser diff. Du kan gjennomgå, merge selektivt, eller lukke den. Workflowen tvinger aldri oppdateringer.",
              },
              {
                q: "Kan jeg sjekke oppdateringer lokalt uten CI?",
                a: "Ja. Kjør nav-pilot sync for å sjekke, eller nav-pilot sync --apply for å oppdatere direkte.",
              },
              {
                q: "Hvordan er dette forskjellig fra Dependabot?",
                a: "Samme konsept — automatiske oppdaterings-PR-er — men for Copilot-tilpasningsfiler. Sammenligner SHA-256-hasher i stedet for semantisk versjonering.",
              },
              {
                q: "Hva om jeg sletter en fil manuelt?",
                a: "Filen markeres som «ignorert» og legges ikke tilbake ved neste sync. Vil du ha den tilbake, kjør nav-pilot add <type> <name>.",
              },
              {
                q: "Kan jeg fjerne filer som ikke passer mitt rammeverk?",
                a: "Ja. Opprett .github/copilot-sync.json med overrides, eller installer frontend-collectionet som allerede utelater Next.js-spesifikke filer.",
              },
            ].map((faq) => (
              <div
                key={faq.q}
                className="rounded-lg"
                style={{ padding: "1rem 1.25rem", background: "#f8fafc", borderLeft: "3px solid #3b82f6" }}
              >
                <div className="flex items-start gap-3">
                  <span
                    className="flex-shrink-0 flex items-center justify-center rounded-full font-bold text-xs mt-0.5"
                    style={{ width: "1.25rem", height: "1.25rem", background: "#dbeafe", color: "#2563eb" }}
                  >
                    ?
                  </span>
                  <div>
                    <Heading size="xsmall" level="4" className="mb-1.5" style={{ color: "#334155" }}>
                      {faq.q}
                    </Heading>
                    <BodyShort size="small" style={{ color: "#475569" }}>
                      {faq.a}
                    </BodyShort>
                  </div>
                </div>
              </div>
            ))}
          </VStack>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 6: CLI-referanse
   ═══════════════════════════════════════════════════════════════ */

function CliReferenceSection() {
  return (
    <section id="cli-referanse">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            CLI-referanse
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            <code className="font-mono text-sm">nav-pilot</code> er et rent installasjonsverktøy skrevet i Go uten
            avhengigheter. All AI-funksjonalitet ligger i markdown-filer som kjøres av GitHub Copilot.
          </BodyLong>
        </div>

        {/* Installation */}
        <div id="installer-cli">
          <LinkableHeading size="small" level="3">
            Installer CLI
          </LinkableHeading>
          <div className="mt-4">
            <CodeBlock compact>{`brew install navikt/tap/nav-pilot`}</CodeBlock>
            <AltInstall />
          </div>
        </div>

        {/* Upgrade */}
        <div id="oppgrader-cli">
          <LinkableHeading size="small" level="3">
            Oppgrader CLI
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot sjekker automatisk om det finnes en nyere versjon ved oppstart. Du kan oppgradere på to måter:
          </BodyLong>
          <div className="mt-4 space-y-3">
            {[
              { label: "Selvoppdatering", cmd: "nav-pilot update" },
              { label: "Via Homebrew", cmd: "brew update && brew upgrade nav-pilot" },
            ].map((item) => (
              <div key={item.cmd}>
                <BodyShort size="small" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                  {item.label}
                </BodyShort>
                <CodeBlock compact>{item.cmd}</CodeBlock>
              </div>
            ))}
          </div>
          <Box background="neutral-soft" padding="space-16" borderRadius="8" className="mt-4">
            <Heading size="xsmall" level="4" style={{ color: "#334155" }}>
              Feilsøking: «already installed»
            </Heading>
            <BodyLong size="small" className="mt-2" style={{ color: "#475569" }}>
              Hvis <code className="font-mono text-xs">brew upgrade</code> sier at nav-pilot allerede er oppdatert men
              versjonen er gammel, skyldes det at den lokale tap-cachen ikke er oppdatert. Kjør{" "}
              <code className="font-mono text-xs">brew update</code> først. Dersom det feiler med tilgangsfeil:
            </BodyLong>
            <div className="mt-2">
              <CodeBlock
                compact
              >{`sudo chown -R $(whoami) /opt/homebrew\nbrew update && brew upgrade nav-pilot`}</CodeBlock>
            </div>
          </Box>
        </div>

        {/* Command reference */}
        <div id="kommandooversikt">
          <LinkableHeading size="small" level="3">
            Kommandooversikt
          </LinkableHeading>

          <div className="overflow-x-auto mt-4">
            <table className="w-full text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  <th className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                    Kommando
                  </th>
                  <th className="text-left py-2 font-semibold" style={{ color: "#334155" }}>
                    Beskrivelse
                  </th>
                </tr>
              </thead>
              <tbody>
                {CLI_COMMANDS.map((cmd) => (
                  <tr key={cmd.command} style={{ borderBottom: "1px solid #e2e8f0" }}>
                    <td className="py-2 pr-4">
                      <code
                        className="text-xs font-mono rounded px-1.5 py-0.5 whitespace-nowrap"
                        style={{ background: "#f1f5f9", color: "#3b82f6" }}
                      >
                        {cmd.command}
                      </code>
                    </td>
                    <td className="py-2" style={{ color: "#475569" }}>
                      {cmd.description}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>

        {/* Advanced examples — basics are shown in "Kom i gang" and "Sync og oppdatering" */}
        <div>
          <Heading size="xsmall" level="3" className="mb-4" style={{ color: "#334155" }}>
            Oppskrifter
          </Heading>

          <VStack gap="space-12">
            <div>
              <Label size="small" className="mb-2" style={{ color: "#64748b" }}>
                Installer collection med forhåndsvisning
              </Label>
              <div className="space-y-3">
                {[
                  { label: "Se hva som installeres", cmd: "nav-pilot install --dry-run kotlin-backend" },
                  { label: "Installer", cmd: "nav-pilot install kotlin-backend" },
                  { label: "Installer i annet repo", cmd: "nav-pilot install --target /path/to/repo kotlin-backend" },
                  {
                    label: "Overskriv lokalt endrede filer",
                    cmd: "nav-pilot install --force kotlin-backend",
                  },
                ].map((item) => (
                  <div key={item.cmd}>
                    <BodyShort size="small" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                      {item.label}
                    </BodyShort>
                    <CodeBlock compact>{item.cmd}</CodeBlock>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <Label size="small" className="mb-2" style={{ color: "#64748b" }}>
                Eksporter til andre verktøy
              </Label>
              <div className="space-y-3">
                {[
                  { label: "Eksporter til OpenCode-format", cmd: "nav-pilot export opencode" },
                  { label: "Eksporter globalt (alle repoer)", cmd: "nav-pilot export opencode --user" },
                  { label: "Forhåndsvis hva som eksporteres", cmd: "nav-pilot export opencode --dry-run" },
                ].map((item) => (
                  <div key={item.cmd}>
                    <BodyShort size="small" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                      {item.label}
                    </BodyShort>
                    <CodeBlock compact>{item.cmd}</CodeBlock>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <Label size="small" className="mb-2" style={{ color: "#64748b" }}>
                Scripting og CI/CD
              </Label>
              <div className="space-y-3">
                {[
                  { label: "JSON-output for alle kommandoer", cmd: "nav-pilot status --json | jq ." },
                  { label: "Sjekk oppdateringer i CI (exit 1 = oppdateringer finnes)", cmd: "nav-pilot sync --json" },
                  { label: "Installer i CI med JSON-resultat", cmd: "nav-pilot install --json kotlin-backend" },
                ].map((item) => (
                  <div key={item.cmd}>
                    <BodyShort size="small" style={{ color: "#94a3b8", fontSize: "0.75rem" }}>
                      {item.label}
                    </BodyShort>
                    <CodeBlock compact>{item.cmd}</CodeBlock>
                  </div>
                ))}
              </div>
              <Box background="neutral-soft" padding="space-12" borderRadius="8" className="mt-3">
                <BodyShort size="small" style={{ color: "#475569" }}>
                  <strong>Exit-koder:</strong> 0 = suksess, 1 = feil eller oppdateringer tilgjengelig (sync), 2 =
                  sync-sjekk feilet. <code className="font-mono text-xs">--json</code> fungerer på install, add, status,
                  sync, list og export.
                </BodyShort>
              </Box>
            </div>
          </VStack>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 7: Slik fungerer det
   ═══════════════════════════════════════════════════════════════ */

function HowItWorksSection() {
  return (
    <section id="slik-fungerer-det">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Slik fungerer det
          </LinkableHeading>
          <BodyLong className="mt-3" style={{ color: "#475569" }}>
            nav-pilot installerer markdown-filer i repoet ditt. GitHub Copilot leser filene og tilpasser forslagene sine
            automatisk. Klikk på filene under for å se hva de gjør.
          </BodyLong>
        </div>

        {/* Interactive file explorer */}
        <div id="filstruktur">
          <LinkableHeading size="small" level="3">
            Filstruktur
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Dette er filene som installeres i{" "}
            <code className="text-xs font-mono rounded px-1 py-0.5" style={{ background: "#f1f5f9" }}>
              .github/
            </code>
            -mappen din. Klikk for detaljer.
          </BodyShort>
          <FileExplorer />
        </div>

        {/* Lifecycle */}
        <div id="livssyklus">
          <LinkableHeading size="small" level="3">
            Livssyklus
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Fra installasjon til daglig bruk — slik henger delene sammen.
          </BodyShort>
          <div className="flex flex-col" style={{ gap: "2px" }}>
            {[
              {
                step: "1",
                title: "Installer",
                desc: "nav-pilot install kopierer markdown-filer fra navikt/copilot til .github/ i repoet ditt.",
                cmd: "nav-pilot install kotlin-backend",
                accent: "#3b82f6",
                bg: "#eff6ff",
              },
              {
                step: "2",
                title: "Commit og push",
                desc: "Filene committes som vanlig kode. Hele teamet får samme Copilot-tilpasning.",
                cmd: "git add .github/ && git commit",
                accent: "#7c3aed",
                bg: "#f5f3ff",
              },
              {
                step: "3",
                title: "Copilot leser automatisk",
                desc: "GitHub Copilot oppdager filene og tilpasser seg. Instruksjoner aktiveres automatisk, agenter kalles med @.",
                cmd: "@nav-pilot planlegg et nytt API",
                accent: "#059669",
                bg: "#ecfdf5",
              },
              {
                step: "4",
                title: "Hold oppdatert",
                desc: "nav-pilot sjekker automatisk for nye versjoner. Bruk sync for å oppdatere filene når nye skills eller forbedringer er tilgjengelige.",
                cmd: "nav-pilot sync --apply",
                accent: "#d97706",
                bg: "#fffbeb",
              },
            ].map((phase, i, arr) => (
              <div
                key={phase.step}
                className="flex items-start gap-4"
                style={{
                  padding: "1rem 1.25rem",
                  background: phase.bg,
                  borderRadius: i === 0 ? "10px 10px 0 0" : i === arr.length - 1 ? "0 0 10px 10px" : "0",
                }}
              >
                <div
                  className="flex-shrink-0 flex items-center justify-center rounded-full font-bold"
                  style={{
                    width: "2rem",
                    height: "2rem",
                    background: "white",
                    color: phase.accent,
                    fontSize: "0.875rem",
                    boxShadow: "0 1px 3px rgba(0,0,0,0.08)",
                  }}
                >
                  {phase.step}
                </div>
                <div className="flex-1">
                  <Label size="small" style={{ color: phase.accent }}>
                    {phase.title}
                  </Label>
                  <BodyShort size="small" style={{ color: "#475569" }}>
                    {phase.desc}
                  </BodyShort>
                  <code
                    className="text-xs font-mono mt-1 inline-block rounded px-1.5 py-0.5"
                    style={{ background: "rgba(255,255,255,0.7)", color: "#64748b" }}
                  >
                    {phase.cmd}
                  </code>
                </div>
              </div>
            ))}
          </div>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 8: Ressurser
   ═══════════════════════════════════════════════════════════════ */

function ResourcesSection() {
  return (
    <section id="ressurser">
      <VStack gap="space-16">
        <LinkableHeading size="medium" level="2">
          Ressurser
        </LinkableHeading>

        {/* Architecture — stacked layers */}
        <div id="arkitektur">
          <LinkableHeading size="small" level="3">
            Arkitektur
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            nav-pilot er bygget på tre lag:
          </BodyShort>
          <div className="flex flex-col" style={{ gap: "2px" }}>
            {[
              {
                label: "Instruksjoner",
                desc: "Alltid aktive — Nav-mønstre, kodestandarder, anti-patterns. Hver Copilot-sesjon er Nav-bevisst automatisk.",
                Icon: DocPencilIcon,
                bg: "#eff6ff",
                accent: "#3b82f6",
              },
              {
                label: "@nav-pilot agent",
                desc: "Én inngangsport — ruter til riktig fase og skill. Delegerer til @auth, @nais, @kafka, @security-champion.",
                Icon: PersonGroupIcon,
                bg: "#f5f3ff",
                accent: "#7c3aed",
              },
              {
                label: "Skills",
                desc: "Byggeklosser — intervju, plan, review, feilsøking. Brukes via @nav-pilot eller alene.",
                Icon: WrenchIcon,
                bg: "#ecfdf5",
                accent: "#059669",
              },
            ].map((layer, i) => (
              <div
                key={layer.label}
                className="flex items-center gap-4"
                style={{
                  padding: "1rem 1.25rem",
                  background: layer.bg,
                  borderRadius: i === 0 ? "10px 10px 0 0" : i === 2 ? "0 0 10px 10px" : "0",
                }}
              >
                <div
                  className="flex-shrink-0 flex items-center justify-center rounded-full"
                  style={{
                    width: "2.5rem",
                    height: "2.5rem",
                    background: "white",
                    boxShadow: "0 1px 3px rgba(0,0,0,0.08)",
                  }}
                >
                  <layer.Icon aria-hidden fontSize="1.25rem" style={{ color: layer.accent }} />
                </div>
                <div className="flex-1">
                  <Label size="small" style={{ color: layer.accent }}>
                    Lag {i + 1}: {layer.label}
                  </Label>
                  <BodyShort size="small" style={{ color: "#475569" }}>
                    {layer.desc}
                  </BodyShort>
                </div>
              </div>
            ))}
          </div>
        </div>

        {/* Design principles — grid cards */}
        <div id="designprinsipper">
          <LinkableHeading size="small" level="3">
            Designprinsipper
          </LinkableHeading>
          <HGrid columns={{ xs: 1, sm: 2, md: 3 }} gap="space-4" className="mt-4">
            {[
              {
                title: "Kunnskap, ikke orkestrering",
                desc: "Institusjonell kunnskap er varig — orkestrering blir standardvare.",
                Icon: LightBulbIcon,
              },
              {
                title: "Tynn ruter, tykke skills",
                desc: "Lett agent som delegerer. Skills har beslutningstrær og sjekklister.",
                Icon: LayersIcon,
              },
              {
                title: "Eksplisitte stopp",
                desc: "nav-pilot foreslår, du godkjenner, nav-pilot fortsetter.",
                Icon: HandShakeHeartIcon,
              },
              {
                title: "Arketype først",
                desc: "«Hva bygger du?» bestemmer stack, auth og Nais-konfig.",
                Icon: Buildings3Icon,
              },
              {
                title: "Minimalt CLI",
                desc: "Go-binær uten avhengigheter. All AI kjøres av Copilot.",
                Icon: ComponentIcon,
              },
            ].map((p) => (
              <div
                key={p.title}
                className="flex flex-col items-start rounded-lg border"
                style={{ padding: "1rem 1.25rem", borderColor: "#e2e8f0" }}
              >
                <div
                  className="flex items-center justify-center rounded-lg mb-2"
                  style={{ width: "2.25rem", height: "2.25rem", background: "#f1f5f9" }}
                >
                  <p.Icon aria-hidden fontSize="1.125rem" style={{ color: "#475569" }} />
                </div>
                <Label size="small" className="mb-1">
                  {p.title}
                </Label>
                <BodyShort size="small" style={{ color: "#64748b" }}>
                  {p.desc}
                </BodyShort>
              </div>
            ))}
          </HGrid>
        </div>

        {/* Links */}
        <div id="lenker">
          <LinkableHeading size="small" level="3">
            Lenker
          </LinkableHeading>
          <div className="mt-4 grid gap-3" style={{ gridTemplateColumns: "repeat(4, 1fr)" }}>
            {[
              {
                label: "GitHub-repo",
                href: "https://github.com/navikt/copilot",
                desc: "Kildekode og issues",
              },
              {
                label: "Alle verktøy",
                href: "/verktoy",
                desc: "Installer enkeltkomponenter",
              },
              {
                label: "God praksis",
                href: "/praksis",
                desc: "Lær å bruke Copilot effektivt",
              },
              {
                label: "Nais-dokumentasjon",
                href: "https://doc.nais.io",
                desc: "Plattformdokumentasjon",
              },
            ].map((link) => (
              <NextLink
                key={link.label}
                href={link.href}
                className="no-underline block rounded-lg border transition-all hover:shadow-md"
                style={{ borderColor: "#e2e8f0", padding: "0.75rem 1rem" }}
                {...(link.href.startsWith("http") ? { target: "_blank", rel: "noopener noreferrer" } : {})}
              >
                <Label size="small" style={{ color: "#3b82f6" }}>
                  {link.label} →
                </Label>
                <BodyShort size="small" className="mt-0.5" style={{ color: "#64748b" }}>
                  {link.desc}
                </BodyShort>
              </NextLink>
            ))}
          </div>
        </div>
      </VStack>
    </section>
  );
}
