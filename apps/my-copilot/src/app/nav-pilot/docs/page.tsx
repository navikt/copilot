import { Heading, BodyShort, BodyLong, Box, HGrid, Label, Table, VStack, Tag } from "@navikt/ds-react";
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
      { id: "isolasjon-er-pakrevd", label: "Isolasjon er påkrevd" },
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
      { id: "vanlige-oppgaver", label: "Vanlige oppgaver" },
    ],
  },
  {
    id: "klienter-og-konfig",
    label: "Klienter og konfigurasjon",
    children: [
      { id: "stotte-klienter", label: "Støttede klienter" },
      { id: "opencode", label: "OpenCode" },
      { id: "konfigurasjon", label: "Konfigurasjon" },
      { id: "konfig-nokler", label: "Konfigurasjonsnøkler" },
    ],
  },
  {
    id: "collections",
    label: "Collections",
    children: [
      { id: "tilgjengelige-collections", label: "Tilgjengelige collections" },
      { id: "innhold-i-hver-collection", label: "Innhold i hver collection" },
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
    id: "kompetansebevaring",
    label: "Kompetansebevaring",
    children: [
      { id: "gronn-rod-sone", label: "Grønn og rød sone" },
      { id: "demo-i-praksis", label: "Demo: I praksis" },
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
    id: "tilpasning",
    label: "Tilpasning",
    children: [
      { id: "team-egne-instruksjoner", label: "Team-egne instruksjoner" },
      { id: "prosjektkontekst-med-nav-pilot-init", label: "Prosjektkontekst med nav-pilot init" },
      { id: "overstyre-installerte-filer", label: "Overstyre installerte filer" },
      { id: "ignorere-enkeltkomponenter", label: "Ignorere enkeltkomponenter" },
    ],
  },
  {
    id: "lokal-modell",
    label: "Bakkemodellen (alfa)",
    children: [
      { id: "lokal-kom-i-gang", label: "Kom i gang" },
      { id: "lokal-hva-den-klarer", label: "Hva den klarer" },
      { id: "lokal-feilsoking", label: "Når noe henger" },
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
    children: [{ id: "filstruktur", label: "Filstruktur" }],
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
    agents: 4,
    skills: 24,
    bestFor: "Backend-API-er og hendelseskonsumenter",
    details: {
      agents: "code-review, research, security-champion, nav-pilot",
      skills:
        "api-design, conventional-commit, flyway-migration, java-to-kotlin, kafka, kotlin-app-config, ktor-scaffold, nais, nav-auth, observability-setup, observability-debugging, postgresql-review, readme-review, security-review, security-owasp, spring-boot-scaffold, terse-mode, threat-model, tokenx-auth, workstation-security, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions:
        "code-review, deliberate-ai-use, output-style, kotlin-ktor, kotlin-spring, testing, testing-kotlin, github-actions, docker, database, security-owasp",
      prompts: "ktor-endpoint, spring-boot-endpoint, kafka-topic, nais-manifest",
    },
  },
  {
    name: "frontend",
    description: "Rammeverk-uavhengig frontend (Astro, Remix, Vite …)",
    agents: 5,
    skills: 12,
    bestFor: "Frontends som ikke bruker Next.js",
    details: {
      agents: "accessibility, aksel, code-review, forfatter, nav-pilot",
      skills:
        "aksel-builder, conventional-commit, playwright-testing, readme-review, terse-mode, web-design-reviewer, nav-dekoratoren, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot, security-owasp",
      instructions:
        "code-review, deliberate-ai-use, output-style, norwegian-text, testing, testing-typescript, accessibility, github-actions, docker, security-owasp",
      prompts: "aksel-component, nais-manifest",
    },
  },
  {
    name: "nextjs-frontend",
    description: "Next.js med Aksel Design System",
    agents: 5,
    skills: 12,
    bestFor: "Innbygger- og saksbehandler-frontends",
    details: {
      agents: "accessibility, aksel, code-review, forfatter, nav-pilot",
      skills:
        "aksel-builder, conventional-commit, playwright-testing, readme-review, terse-mode, web-design-reviewer, nav-dekoratoren, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot, security-owasp",
      instructions:
        "code-review, deliberate-ai-use, output-style, nextjs-aksel, norwegian-text, performance, testing, testing-typescript, accessibility, github-actions, docker, security-owasp",
      prompts: "aksel-component, nextjs-api-route, nais-manifest",
    },
  },
  {
    name: "fullstack",
    description: "Komplett stack (backend + frontend)",
    agents: 7,
    skills: 28,
    bestFor: "Team som eier hele stacken",
    details: {
      agents: "accessibility, aksel, code-review, forfatter, research, security-champion, nav-pilot",
      skills:
        "aksel-builder, api-design, conventional-commit, flyway-migration, java-to-kotlin, kafka, kotlin-app-config, ktor-scaffold, nais, nav-auth, observability-setup, observability-debugging, playwright-testing, postgresql-review, readme-review, security-review, security-owasp, spring-boot-scaffold, terse-mode, threat-model, tokenx-auth, web-design-reviewer, nav-dekoratoren, workstation-security, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions:
        "code-review, deliberate-ai-use, output-style, kotlin-ktor, kotlin-spring, golang, nextjs-aksel, norwegian-text, performance, testing, testing-kotlin, testing-typescript, accessibility, github-actions, docker, database, security-owasp",
      prompts:
        "ktor-endpoint, spring-boot-endpoint, kafka-topic, nais-manifest, aksel-component, nextjs-api-route, golang-service",
    },
  },
  {
    name: "platform",
    description: "Nais, observability, sikkerhet og Go",
    agents: 4,
    skills: 15,
    bestFor: "Plattform- og DevOps-team",
    details: {
      agents: "code-review, research, security-champion, nav-pilot",
      skills:
        "conventional-commit, nais, observability-setup, observability-debugging, readme-review, rust-development, security-review, security-owasp, terse-mode, threat-model, workstation-security, nav-plan, nav-deep-interview, nav-architecture-review, nav-troubleshoot",
      instructions:
        "code-review, deliberate-ai-use, output-style, golang, testing, github-actions, docker, security-owasp",
      prompts: "golang-service, nais-manifest",
    },
  },
];

const PLANNING_SKILLS = [
  {
    name: "$nav-deep-interview",
    purpose: "Strukturert intervju som avdekker blindsoner (personvern, auth, avhengigheter)",
    details: [
      "Personvern og data — PII-kategorier, dataklassifisering, sletteregler",
      "Plattform og auth — caller-type, avhengigheter, feilhåndtering",
      "Observerbarhet — forretningsmetrikker, varsling, on-call",
      "Team og prosess — avhengigheter, deadlines, erfaring",
    ],
    refs: "data-classification.md, blind-spots.md (25+ vanlige blindsoner fra ekte Nav-repoer)",
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
  { command: "nav-pilot --client opencode", description: "Start OpenCode-sesjonen med Nav-kontekst levert automatisk" },
  {
    command: "nav-pilot install <collection>",
    description: "Installer en collection — spør om repoet (.github/) eller hjemmekatalogen (~/.copilot/)",
  },
  {
    command: "nav-pilot install --user",
    description: "Installer agenter, skills og instruksjoner til ~/.copilot (alle repoer)",
  },
  { command: "nav-pilot install --dry-run <collection>", description: "Forhåndsvis hva som installeres" },
  { command: "nav-pilot install --force <collection>", description: "Overskriv lokalt endrede filer" },
  { command: "nav-pilot list", description: "Vis tilgjengelige collections og enkeltkomponenter" },
  { command: "nav-pilot list --installed", description: "Vis installerte filer og integritet" },
  { command: "nav-pilot doctor", description: "Kjør helsesjekk av systemet og miljøet" },
  { command: "nav-pilot install <name>", description: "Installer enkeltkomponent (agent, skill, etc.)" },
  {
    command: "nav-pilot install <name> --type <type>",
    description: "Installer med eksplisitt type (agent, skill, instruction, prompt)",
  },
  {
    command: "nav-pilot ignore <type> <name> --user",
    description: "Stopp varsel om en komponent uten å installere den",
  },
  { command: "nav-pilot uninstall", description: "Fjern alle installerte filer" },
  { command: "nav-pilot sync", description: "Sjekk om oppdateringer finnes (exit 1 hvis ja)" },
  { command: "nav-pilot sync --apply", description: "Oppdater filer direkte" },
  { command: "nav-pilot sync --json", description: "Maskinlesbar JSON-output" },
  {
    command: "<command> --json",
    description: "Globalt flagg: JSON-output på alle kommandoer (install, list, sync, export)",
  },
  { command: "nav-pilot env", description: "Skriv shell-eksport for Copilot CLI-integrasjon" },
  { command: "nav-pilot upgrade", description: "Oppdater nav-pilot CLI til nyeste versjon" },
  { command: "nav-pilot feedback", description: "Rapporter feil — åpner GitHub issue med diagnostikk" },
  { command: "nav-pilot feedback --feature", description: "Foreslå ny funksjon" },
  { command: "nav-pilot export opencode", description: "Eksporter til .opencode/-format (OpenCode / oh-my-openagent)" },
  { command: "nav-pilot export opencode --user", description: "Eksporter til ~/.config/opencode/ (globalt)" },
  { command: "nav-pilot config", description: "Interaktiv innstillingsside i terminalen" },
  { command: "nav-pilot config init", description: "Opprett ~/.nav-pilot/config.toml med alle valg kommentert ut" },
  { command: "nav-pilot config setup", description: "Interaktiv konfigurasjonsveileder (klient, modell, modus)" },
  { command: "nav-pilot config show", description: "Vis effektiv konfigurasjon (fil + standardverdier)" },
  { command: "nav-pilot config get <key>", description: "Hent én konfigurasjonsverdi" },
  { command: "nav-pilot config set <key> <value>", description: "Sett én konfigurasjonsverdi" },
  { command: "nav-pilot config validate", description: "Valider konfigurasjonsfilen" },
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
                <KlienterOgKonfigurasjonSection />
                <CollectionsSection />
                <PipelineSection />
                <CompetenceSection />
                <SyncSection />
                <CustomizationSection />
                <LocalModelSection />
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
            nav-pilot er et <strong>CLI-verktøy</strong> og en <strong>AI-agent</strong>. CLI-et klargjør repoet ditt
            med riktige agenter, skills og instruksjoner. Agenten (
            <code
              className="text-sm font-mono rounded px-1.5 py-0.5"
              style={{ background: "#f1f5f9", color: "#3b82f6" }}
            >
              @nav-pilot
            </code>
            ) bruker denne kunnskapen til å planlegge og arkitektere Nav-applikasjoner i Copilot Chat. I bakgrunnen
            sørger CLI-et også for at token-bruken din optimaliseres automatisk.
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            nav-pilot inneholder <strong>én planleggingsagent, fire planning skills og fem collections</strong>.
            Collectionene koder Navs institusjonelle kunnskap som kjørbare arbeidsflyter. CLI-et installerer
            markdown-filer — selve AI-funksjonaliteten kjøres av GitHub Copilot.
          </BodyLong>

          {/* Component overview cards */}
          <div className="mt-6 grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))" }}>
            {[
              { name: "@nav-pilot", desc: "Planleggingsagent — din inngangsport", color: "#3b82f6", Icon: CompassIcon },
              {
                name: "$nav-deep-interview",
                desc: "Avdekker blindsoner (personvern, auth, avhengigheter)",
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

        <VStack id="isolasjon-er-pakrevd" gap="space-12">
          <LinkableHeading size="small" level="3">
            Isolasjon er påkrevd på Nav-utstyr
          </LinkableHeading>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <VStack gap="space-8">
              <BodyLong style={{ color: "#475569" }}>
                Når du bruker en AI-agent på Nav-utstyr, skal agenten kjøre i en sandbox eller tilsvarende isolasjon.
                Kravet gjelder både Nav-relatert og personlig agentarbeid.
              </BodyLong>
              <BodyLong style={{ color: "#475569" }}>
                Bruk{" "}
                <NextLink href="/cplt" className="text-blue-600 hover:underline">
                  cplt
                </NextLink>{" "}
                — det er den anbefalte og enkleste løsningen. Hvis du velger en annen løsning, må du selv sette deg inn
                i hvordan agentklienten isolerer agenten, og aktivere denne funksjonen. Hvis klienten ikke gir
                tilstrekkelig beskyttelse, må du sørge for tilsvarende isolasjon, for eksempel med en VM eller
                container. Ikke kjør agenter med ubegrenset tilgang til Nav-utstyret.
              </BodyLong>
              <BodyLong style={{ color: "#475569" }}>
                <NextLink href="/nyheter/sandboxing-er-pakrevd-pa-nav-utstyr" className="text-blue-600 hover:underline">
                  Les kortversjonen av kravet
                </NextLink>{" "}
                for en lenke du kan dele med andre.
              </BodyLong>
            </VStack>
          </Box>
        </VStack>

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
          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0 mt-4">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
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
              "At HikariCP default pool (10) er for stor for containere — start med 3",
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
            Denne kunnskapen er kodet inn i nav-pilots beslutningstrær, blindsone-sjekklister og diagnostiske trær.
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
            <CodeBlock compact>{`brew install navikt/tap/nav-pilot navikt/tap/cplt`}</CodeBlock>
            <AltInstall />
            <BodyLong className="mt-3" size="small" style={{ color: "#64748b" }}>
              Valgfritt for zsh eller bash: Legg{" "}
              <code className="font-mono text-xs">alias copilot=&apos;cplt --&apos;</code> og{" "}
              <code className="font-mono text-xs">alias np=&apos;nav-pilot&apos;</code> i shell-profilen din. Aliasene
              gjelder bare i terminalen.
            </BodyLong>
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
                    {`cplt -- --agent nav-pilot --prompt "Jeg trenger en ny tjeneste som behandler dagpengesøknader"`}
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
                  Starter interaktiv modus — sjekker oppdateringer og starter Copilot med valgt agent.
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
            Du kan også installere agenter, skills og instruksjoner til hjemmemappen. De blir da tilgjengelige i{" "}
            <em>alle</em> repoer uten å endre hvert enkelt.
          </BodyLong>
          <div className="mt-4">
            <CodeBlock compact>{`nav-pilot install --user`}</CodeBlock>
          </div>
          <BodyLong className="mt-3" size="small" style={{ color: "#64748b" }}>
            Filene installeres til <code className="font-mono text-xs">~/.copilot/</code>. Agenter og skills plukkes opp
            automatisk av GitHub Copilot. Instruksjoner krever{" "}
            <code className="font-mono text-xs">COPILOT_CUSTOM_INSTRUCTIONS_DIRS</code> og fungerer kun med Copilot CLI
            — nav-pilot setter denne automatisk i interaktiv modus. OpenCode mottar Nav-kontekst på en annen måte — se{" "}
            <a href="#opencode" className="text-blue-600 hover:underline">
              OpenCode
            </a>
            .
          </BodyLong>
          <BodyLong className="mt-2" size="small" style={{ color: "#64748b" }}>
            Når nye komponenter dukker opp i kilden, varsler nav-pilot om det ved oppstart. Vil du ikke installere en
            bestemt komponent, stopper du varselet med:
          </BodyLong>
          <div className="mt-2">
            <CodeBlock compact>{`nav-pilot ignore instruction nextjs-aksel --user`}</CodeBlock>
          </div>
          <BodyLong className="mt-2" size="small" style={{ color: "#64748b" }}>
            For direkte bruk av cplt, legg til i shell-profilen:
          </BodyLong>
          <div className="mt-2">
            <CodeBlock compact>{`eval "$(nav-pilot env)"`}</CodeBlock>
          </div>
        </div>

        <Box background="neutral-soft" borderRadius="8" padding="space-12">
          <BodyShort size="small" style={{ color: "#475569" }}>
            Trenger du full kommandoreferanse? Gå til{" "}
            <NextLink href="#kommandooversikt" className="text-blue-600 hover:underline">
              CLI-referanse
            </NextLink>
            . Her i «Kom i gang» holder vi kun minimumsstegene.
          </BodyShort>
        </Box>

        {/* Common tasks — job-oriented view */}
        <div id="vanlige-oppgaver">
          <LinkableHeading size="small" level="3">
            Vanlige oppgaver
          </LinkableHeading>
          <BodyLong className="mt-2 mb-4" style={{ color: "#475569" }}>
            Du trenger ikke huske skill-navn. Bare beskriv oppgaven — nav-pilot bruker riktig kunnskap automatisk. Her
            er eksempler:
          </BodyLong>
          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  <th scope="col" className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                    Oppgave
                  </th>
                  <th scope="col" className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                    Eksempel-prompt
                  </th>
                </tr>
              </thead>
              <tbody>
                {[
                  { task: "Bygge ny tjeneste", prompt: "Jeg trenger en ny tjeneste for dagpenger" },
                  { task: "Legge til autentisering", prompt: "Legg til TokenX-validering i API-et" },
                  { task: "Debugge deploy", prompt: "Poden min krasjer i dev, hjelp meg feilsøke" },
                  { task: "Gjennomgå før PR", prompt: "Gjør en sikkerhetsgjennomgang av disse endringene" },
                  { task: "Sette opp Kafka", prompt: "Vi trenger en Kafka-consumer for vedtakshendelser" },
                  { task: "Legge til observerbarhet", prompt: "Sett opp metrikker og tracing for tjenesten" },
                  { task: "Migrere Java → Kotlin", prompt: "Hjelp meg migrere denne klassen til Kotlin" },
                  { task: "Få kortere svar", prompt: "$terse-mode" },
                  { task: "Planlegge arkitektur", prompt: "Planlegg arkitekturen for nytt saksbehandlersystem" },
                ].map((row) => (
                  <tr key={row.task} style={{ borderBottom: "1px solid #e2e8f0" }}>
                    <td className="py-2 pr-4 font-medium" style={{ color: "#1e293b" }}>
                      {row.task}
                    </td>
                    <td className="py-2 pr-4" style={{ color: "#475569", fontStyle: "italic" }}>
                      «{row.prompt}»
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

          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0 mt-4">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
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

        {/* Collection contents detail */}
        <div>
          <LinkableHeading size="small" level="3">
            Innhold i hver collection
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Hver collection inneholder også planning skills, instruksjoner og prompts:
          </BodyShort>
          <VStack gap="space-12">
            {COLLECTIONS.map((c) => (
              <details key={c.name} className="group">
                <summary
                  className="cursor-pointer list-none flex items-center gap-2 py-2 px-3 rounded-lg transition-colors"
                  style={{ background: "#f8fafc", border: "1px solid #e2e8f0" }}
                >
                  <span style={{ fontSize: "0.75rem", color: "#64748b", transition: "transform 0.2s" }}>▶</span>
                  <code className="text-sm font-mono font-semibold" style={{ color: "#3b82f6" }}>
                    {c.name}
                  </code>
                  <span style={{ color: "#64748b", fontSize: "0.8125rem" }}>— {c.description}</span>
                </summary>
                <div className="mt-2 ml-6 space-y-3">
                  <div>
                    <Label size="small" style={{ color: "#334155" }}>
                      Agenter ({c.agents})
                    </Label>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {c.details.agents.split(", ").map((a) => (
                        <code
                          key={a}
                          className="text-xs font-mono rounded px-1.5 py-0.5"
                          style={{ background: "#eff6ff", color: "#3b82f6" }}
                        >
                          {a}
                        </code>
                      ))}
                    </div>
                  </div>
                  <div>
                    <Label size="small" style={{ color: "#334155" }}>
                      Skills ({c.skills})
                    </Label>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {c.details.skills.split(", ").map((s) => (
                        <code
                          key={s}
                          className="text-xs font-mono rounded px-1.5 py-0.5"
                          style={{ background: "#f5f3ff", color: "#7c3aed" }}
                        >
                          {s}
                        </code>
                      ))}
                    </div>
                  </div>
                  <div>
                    <Label size="small" style={{ color: "#334155" }}>
                      Instruksjoner
                    </Label>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {c.details.instructions.split(", ").map((i) => (
                        <code
                          key={i}
                          className="text-xs font-mono rounded px-1.5 py-0.5"
                          style={{ background: "#f0fdf4", color: "#16a34a" }}
                        >
                          {i}
                        </code>
                      ))}
                    </div>
                  </div>
                  <div>
                    <Label size="small" style={{ color: "#334155" }}>
                      Prompts
                    </Label>
                    <div className="flex flex-wrap gap-1.5 mt-1">
                      {c.details.prompts.split(", ").map((p) => (
                        <code
                          key={p}
                          className="text-xs font-mono rounded px-1.5 py-0.5"
                          style={{ background: "#fff7ed", color: "#ea580c" }}
                        >
                          {p}
                        </code>
                      ))}
                    </div>
                  </div>
                </div>
              </details>
            ))}
          </VStack>
        </div>

        {/* Planning skills table */}
        <div id="planning-skills">
          <LinkableHeading size="small" level="3">
            Planning skills
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Alle collections inkluderer fire planning skills som utgjør <strong>nav-pilot-pipelinen</strong>:
          </BodyShort>
          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
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

          <div className="mt-4 overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
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
   Section 4b: Kompetansebevaring (grønn/rød sone)
   ═══════════════════════════════════════════════════════════════ */

function CompetenceSection() {
  return (
    <section id="kompetansebevaring">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Kompetansebevaring
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Flere studier dokumenterer at passiv bruk av kodegenerering svekker utvikleres forståelse av egen kode. I
            Anthropics RCT (2026) scora utviklere som delegerte blindt 35–39 % på kodeforståelse, mot 86 % for de som
            aktivt stilte spørsmål etter generering. Navs egen longitudinalstudie (Stray et al., HICSS-59 2026)
            bekrefter mønsteret internt.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Samtidig viser MIT/Microsoft-studien (2025, ~5000 utviklere) at AI-assistanse gir størst
            produktivitetsgevinst på repetitive oppgaver. Gevinsten forsvinner — og kan bli negativ — på oppgaver som
            krever dyp forståelse av domenet.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot implementerer dette skillet: oppgaver klassifiseres i <strong>grønn sone</strong> (AI genererer
            full kode) og <strong>rød sone</strong> (utvikleren skriver kjernelogikken selv). Klassifiseringen skjer i
            Fase 2 og håndheves i Fase 4.
          </BodyLong>
        </div>

        <div id="gronn-rod-sone">
          <LinkableHeading size="small" level="3">
            Grønn og rød sone
          </LinkableHeading>

          <div className="mt-4 grid gap-4 md:grid-cols-2">
            <div className="rounded-lg p-4" style={{ background: "#f0fdf4", border: "1px solid #bbf7d0" }}>
              <div className="flex items-center gap-2 mb-2">
                <span style={{ fontSize: "1.25rem" }}>🟢</span>
                <Label size="small" style={{ color: "#166534" }}>
                  Grønn sone — AI genererer full kode
                </Label>
              </div>
              <ul className="text-sm space-y-1" style={{ color: "#15803d" }}>
                <li>Boilerplate og repetitiv kode (Nais-manifest, CRUD)</li>
                <li>Kjent teknologi du allerede behersker</li>
                <li>Konfigurasjon og infrastruktur</li>
                <li>Refaktorering med kjent mål</li>
                <li>Testdata og fixtures</li>
              </ul>
            </div>

            <div className="rounded-lg p-4" style={{ background: "#fef2f2", border: "1px solid #fecaca" }}>
              <div className="flex items-center gap-2 mb-2">
                <span style={{ fontSize: "1.25rem" }}>🔴</span>
                <Label size="small" style={{ color: "#991b1b" }}>
                  Rød sone — du koder, AI leverer stubs
                </Label>
              </div>
              <ul className="text-sm space-y-1" style={{ color: "#dc2626" }}>
                <li>Debugging og feilsøking</li>
                <li>Nye konsepter og ukjent teknologi</li>
                <li>Kjernelogikk og forretningsregler</li>
                <li>Sikkerhetskritisk kode</li>
                <li>Arkitekturbeslutninger</li>
              </ul>
            </div>
          </div>

          <BodyShort size="small" className="mt-4" style={{ color: "#64748b" }}>
            Når nav-pilot identifiserer rød-sone-logikk i Fase 2 (Plan), leverer Fase 4 bare testskjeletter og
            kode-stubs med <code>TODO</code>-kommentarer — ikke full implementasjon. Du skriver kjernelogikken selv for
            å bygge dyp forståelse.
          </BodyShort>
        </div>

        <div id="demo-i-praksis">
          <LinkableHeading size="small" level="3">
            Demo: I praksis
          </LinkableHeading>

          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Her ser du nav-pilot planlegge en ny beregningsregel for sykepenger (§8-20). Legg merke til hvordan den
            skiller mellom grønn sone (plumbing-kode) og rød sone (regelverkslogikk):
          </BodyShort>

          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/demos/nav-pilot-red-zone.gif"
            alt="Demo av nav-pilot som identifiserer kjernelogikk som rød sone og leverer stubs med TODO"
            className="rounded-lg border w-full"
            style={{ border: "1px solid #e2e8f0" }}
          />

          <BodyShort size="small" className="mt-3" style={{ color: "#64748b" }}>
            Basert på forskning fra{" "}
            <NextLink
              href="https://www.anthropic.com/research/AI-assistance-coding-skills"
              target="_blank"
              rel="noopener noreferrer"
              style={{ color: "#2563eb" }}
            >
              Anthropic
            </NextLink>
            ,{" "}
            <NextLink
              href="https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/"
              target="_blank"
              rel="noopener noreferrer"
              style={{ color: "#2563eb" }}
            >
              METR
            </NextLink>{" "}
            og{" "}
            <NextLink
              href="https://arxiv.org/abs/2509.20353"
              target="_blank"
              rel="noopener noreferrer"
              style={{ color: "#2563eb" }}
            >
              Nav ITs egen studie
            </NextLink>
            .
          </BodyShort>
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
              viser hvilke filer som er oppdaterte, med lenker til kilderepoet.
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
  "overrides": [
    ".github/instructions/nextjs-aksel.instructions.md",
    ".github/instructions/performance.instructions.md",
    ".github/prompts/nextjs-api-route.prompt.md"
  ]
}`}
          </CodeBlock>
          <BodyShort size="small" className="mt-3" style={{ color: "#475569" }}>
            Filer i <code className="font-mono text-xs">overrides</code> hoppes helt over under sync — ingen
            hash-sammenligning, ingen PR-diff. Du kan trygt slette filene etterpå, og de blir ikke lagt til igjen.
            Alternativt kan du installere <code className="font-mono text-xs">frontend</code>
            -collectionet som allerede utelater Next.js-spesifikke filer.
          </BodyShort>
          <BodyShort size="small" className="mt-2" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Sletter du en fil manuelt uten override, markeres den som «ignorert» og gjenopprettes ikke av sync. Legg den
            til igjen med <code className="font-mono text-xs">nav-pilot install</code> hvis du ombestemmer deg.
          </BodyShort>
          <BodyShort size="small" className="mt-2" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Har teamet en egen versjon av en fil med samme navn som kilden (f.eks. en egen{" "}
            <code className="font-mono text-xs">kotlin-app-config</code> skill), vil sync prøve å overskrive den. Bruk
            overrides for å beskytte filen. Filer med navn som ikke finnes i kilden blir aldri berørt av sync.
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
                a: "Filen markeres som «ignorert» og legges ikke tilbake ved neste sync. Vil du ha den tilbake, kjør nav-pilot install <name>.",
              },
              {
                q: "Jeg får varsel om en komponent jeg ikke vil installere. Hvordan stopper jeg det?",
                a: "Kjør nav-pilot ignore <type> <name> --user. nav-pilot merker komponenten som ignorert og varsler ikke om den igjen.",
              },
              {
                q: "Kan jeg fjerne filer som ikke passer mitt rammeverk?",
                a: "Ja. Opprett .github/copilot-sync.json med overrides, eller installer frontend-collectionet som allerede utelater Next.js-spesifikke filer.",
              },
              {
                q: "Hva skjer hvis vi har en egen fil med samme navn som kilden?",
                a: "Sync sammenligner hasher og foreslår å overskrive den med kildens versjon. Legg filen i overrides for å beskytte den. Filer med navn som ikke finnes i kilden ignoreres helt.",
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
   Section 6: Tilpasning
   ═══════════════════════════════════════════════════════════════ */

function CustomizationSection() {
  return (
    <section id="tilpasning">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Tilpasning
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot gir teamet et godt utgangspunkt, men repoet ditt trenger ofte egne regler og egen kontekst. Her er
            de fire mekanismene du bruker for å tilpasse installasjonen uten å miste kontrollen.
          </BodyLong>
        </div>

        <div id="team-egne-instruksjoner">
          <LinkableHeading size="small" level="3">
            Team-egne instruksjoner
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Legg egne filer i <code className="font-mono text-xs">.github/instructions/</code> ved siden av det
            nav-pilot installerer. nav-pilot berører aldri filer det ikke selv har installert, så teamet kan trygt legge
            inn egne konvensjoner her.
          </BodyLong>
          <div className="mt-4">
            <CodeBlock compact>
              {`.github/instructions/
  golang.instructions.md           ← installed by nav-pilot
  security-owasp.instructions.md   ← installed by nav-pilot
  team-conventions.instructions.md ← your team's own file`}
            </CodeBlock>
          </div>
        </div>

        <div id="prosjektkontekst-med-nav-pilot-init">
          <LinkableHeading size="small" level="3">
            Prosjektkontekst med nav-pilot init
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Kjør <code className="font-mono text-xs">nav-pilot init</code> når du vil fylle inn prosjektspesifikk
            kontekst. Kommandoen lager tre malfiler med TODO-er som teamet fyller ut selv. nav-pilot oppretter dem én
            gang, men forvalter dem ikke videre.
          </BodyLong>
          <div className="mt-4 space-y-3">
            <CodeBlock compact>{`nav-pilot init`}</CodeBlock>
            <CodeBlock compact>
              {`AGENTS.md
.github/copilot-instructions.md
.github/copilot-review-instructions.md`}
            </CodeBlock>
          </div>
        </div>

        <div id="overstyre-installerte-filer">
          <LinkableHeading size="small" level="3">
            Overstyre installerte filer
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Vil dere eie en fil som nav-pilot vanligvis oppdaterer, legger dere den i{" "}
            <code className="font-mono text-xs">.github/copilot-sync.json</code>. Filer i{" "}
            <code className="font-mono text-xs">overrides</code> hoppes over under{" "}
            <code className="font-mono text-xs">nav-pilot sync</code>, så teamets versjon blir stående.
          </BodyLong>
          <div className="mt-4">
            <CodeBlock compact>
              {`{
  "overrides": [
    ".github/instructions/golang.instructions.md"
  ]
}`}
            </CodeBlock>
          </div>
        </div>

        <div id="ignorere-enkeltkomponenter">
          <LinkableHeading size="small" level="3">
            Ignorere enkeltkomponenter
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Bruker du installasjon på brukernivå, kan du undertrykke enkeltkomponenter du ikke vil ha varsler om.{" "}
            <code className="font-mono text-xs">nav-pilot ignore</code> er bare for{" "}
            <code className="font-mono text-xs">--user</code>-installasjoner.
          </BodyLong>
          <div className="mt-4 space-y-3">
            <CodeBlock compact>{`nav-pilot ignore agent rust-agent --user`}</CodeBlock>
            <CodeBlock compact>{`nav-pilot ignore skill rust-development --user`}</CodeBlock>
          </div>
        </div>

        <Box background="neutral-soft" padding="space-16" borderRadius="8">
          <BodyLong size="small" style={{ color: "#475569" }}>
            Velg mekanisme etter behov: egne instruksjoner for nye regler,{" "}
            <code className="font-mono text-xs">init</code>
            for prosjektkontekst, <code className="font-mono text-xs">overrides</code> når dere vil eie en installert
            fil, og <code className="font-mono text-xs">ignore</code> for brukerinstallerte komponenter dere ikke
            trenger.
          </BodyLong>
        </Box>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 2b: Klienter og konfigurasjon
   ═══════════════════════════════════════════════════════════════ */

const CONFIG_KEYS = [
  {
    key: "client",
    flag: "--client",
    values: "copilot · opencode · pi",
    desc: "Klient å starte. copilot er standard; pi er reservert og støttes ikke ennå.",
  },
  {
    key: "model",
    flag: "--model",
    values: "f.eks. claude-opus-4.8, gpt-5.5 (Copilot); github-copilot/auto (opencode)",
    desc: "Modell å bruke. Format avhenger av klient.",
  },
  {
    key: "mode",
    flag: "--mode",
    values: "default · plan · autopilot",
    desc: "Modus for Copilot-agenten. plan tilsvarer opencode --agent plan; autopilot og øvrige er kun Copilot.",
  },
  {
    key: "reasoning_effort",
    flag: "--effort",
    values: "none · low · medium · high · xhigh · max",
    desc: "Resonneringsinnsats. Fungerer for begge klienter: Copilot bruker --effort, opencode bruker --variant.",
  },
  {
    key: "context_tier",
    flag: "--context",
    values: "default · long_context",
    desc: "Kontekstnivå. Kun Copilot — nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "allow_all_tools",
    flag: "--allow-all-tools / --no-allow-all-tools",
    values: "bool",
    desc: "Gi agenten tilgang til alle tilgjengelige verktøy.",
  },
  {
    key: "ask_user",
    flag: "--ask-user / --no-ask-user",
    values: "bool",
    desc: "Be om bekreftelse på beslutninger. Kun Copilot — nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "log_level",
    flag: "--log-level",
    values: "none · error · warning · info · debug",
    desc: "Loggnivå for nav-pilot CLI.",
  },
  {
    key: "otel_log_level",
    flag: "--otel-log-level",
    values: "none · error · warning · info · debug",
    desc: "Loggnivå for OpenTelemetry-eksport.",
  },
  {
    key: "local_enabled",
    flag: "—",
    values: "true · false",
    desc: "Send avgrensede oppgaver til bakkemodellen (alfa). Settes av 'alpha local init', nullstilles av 'alpha local off'. Så lenge den er false finnes ingen lokale modeller i nav-pilot i det hele tatt.",
  },
  {
    key: "local_autostart",
    flag: "—",
    values: "true · false",
    desc: "La en vanlig 'nav-pilot' starte serveren selv når den trengs og ingen kjører. Av som standard: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.",
  },
  {
    key: "local_loop_guard",
    flag: "—",
    values: "et tall, standard 8",
    desc: "Hvor mange identiske tool calls på rad som avslutter en lokal tur. Lokale modeller setter seg fast og gjentar det samme kallet; vi har målt serier på 203.",
  },
];

function KlienterOgKonfigurasjonSection() {
  return (
    <section id="klienter-og-konfig">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Klienter og konfigurasjon
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot kan starte ulike kodingsagent-klienter. Du velger klient via flagg eller konfigurasjonsfil, og
            tilpasser oppførsel med én felles fil: <code className="font-mono text-xs">~/.nav-pilot/config.toml</code>.
          </BodyLong>
        </div>

        {/* Supported clients */}
        <div id="stotte-klienter">
          <LinkableHeading size="small" level="3">
            Støttede klienter
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Velg klient med <code className="font-mono text-xs">--client</code>-flagget eller{" "}
            <code className="font-mono text-xs">client</code>-nøkkelen i konfigurasjonsfilen.
          </BodyShort>

          <div className="grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(240px, 1fr))" }}>
            {[
              {
                name: "copilot",
                badge: "Standard",
                badgeColor: "#3b82f6",
                badgeBg: "#dbeafe",
                desc: "GitHub Copilot / cplt-sandkassen. Fungerer med VS Code, JetBrains og GitHub.com.",
                color: "#3b82f6",
              },
              {
                name: "opencode",
                badge: "Første klasse",
                badgeColor: "#059669",
                badgeBg: "#d1fae5",
                desc: "OpenCode terminal-klient. Nav-kontekst leveres og holdes oppdatert automatisk.",
                color: "#059669",
              },
              {
                name: "pi",
                badge: "Reservert",
                badgeColor: "#94a3b8",
                badgeBg: "#f1f5f9",
                desc: "Ikke støttet ennå — nav-pilot returnerer feilmelding om du velger denne.",
                color: "#94a3b8",
              },
            ].map((c) => (
              <div
                key={c.name}
                className="rounded-lg overflow-hidden"
                style={{ background: "white", border: "1px solid #e2e8f0" }}
              >
                <div style={{ height: "3px", background: c.color }} />
                <div style={{ padding: "0.75rem 1rem" }}>
                  <div className="flex items-center gap-2 mb-1.5">
                    <code className="text-sm font-mono font-semibold" style={{ color: c.color }}>
                      {c.name}
                    </code>
                    <span
                      className="text-xs font-medium rounded-full"
                      style={{
                        background: c.badgeBg,
                        color: c.badgeColor,
                        padding: "1px 8px",
                      }}
                    >
                      {c.badge}
                    </span>
                  </div>
                  <BodyShort size="small" style={{ color: "#475569" }}>
                    {c.desc}
                  </BodyShort>
                </div>
              </div>
            ))}
          </div>

          <div className="mt-4 space-y-3">
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Start med OpenCode (flagg)
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`nav-pilot --client opencode`}</CodeBlock>
              </div>
            </div>
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Sett OpenCode som standard (config.toml)
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`client = "opencode"`}</CodeBlock>
              </div>
            </div>
          </div>
        </div>

        {/* OpenCode */}
        <div id="opencode">
          <div className="flex items-center gap-2 mb-2">
            <TerminalIcon fontSize="1.125rem" style={{ color: "#059669" }} aria-hidden />
            <LinkableHeading size="small" level="3">
              OpenCode
            </LinkableHeading>
          </div>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            OpenCode er en <strong>første klasse</strong>-klient i nav-pilot. Når du starter med{" "}
            <code className="font-mono text-xs">--client opencode</code>, leverer nav-pilot Nav-kontekst (AGENTS.md,
            skills, kommandoer og agenter) direkte til <code className="font-mono text-xs">~/.config/opencode/</code> og
            holder det oppdatert ved hver kjøring.
          </BodyLong>

          <div className="mt-4 grid gap-3 sm:grid-cols-2">
            {[
              {
                title: "Automatisk kontekstlevering",
                desc: (
                  <>
                    Nav-kontekst materialiseres til <code className="font-mono text-xs">~/.config/opencode/</code> og
                    holdes fersk med konfliktsdeteksjon — dine egne redigeringer overskrives ikke.
                  </>
                ),
                color: "#059669",
                bg: "#ecfdf5",
              },
              {
                title: "Kuratert standardmodell",
                desc: (
                  <>
                    Når ingen modell er konfigurert, settes{" "}
                    <code className="font-mono text-xs">github-copilot/auto</code> som Nav-standard for opencode.
                  </>
                ),
                color: "#3b82f6",
                bg: "#eff6ff",
              },
              {
                title: "OTel-telemetri",
                desc: "OpenTelemetry-konfigurasjon settes opp automatisk — ingen manuell konfigurasjon nødvendig.",
                color: "#7c3aed",
                bg: "#f5f3ff",
              },
              {
                title: "Tilstandsfil",
                desc: (
                  <>
                    <code className="font-mono text-xs">~/.config/opencode/.nav-pilot-state.json</code> sporer
                    installerte filer og versjon, slik at sync vet hva som er endret.
                  </>
                ),
                color: "#ea580c",
                bg: "#fff7ed",
              },
            ].map((item) => (
              <div
                key={item.title}
                className="rounded-lg"
                style={{ padding: "0.875rem 1rem", background: item.bg, border: `1px solid ${item.color}22` }}
              >
                <Label size="small" className="mb-1" style={{ color: item.color }}>
                  {item.title}
                </Label>
                <BodyShort size="small" style={{ color: "#475569" }}>
                  {item.desc}
                </BodyShort>
              </div>
            ))}
          </div>

          <Box background="neutral-soft" padding="space-12" borderRadius="8" className="mt-4">
            <BodyShort size="small" style={{ color: "#475569" }}>
              <code className="font-mono text-xs">nav-pilot export opencode</code> finnes fortsatt for manuell
              engangseksport, men trengs <strong>ikke</strong> i den normale flyten — nav-pilot håndterer dette
              automatisk når du bruker <code className="font-mono text-xs">--client opencode</code>.
            </BodyShort>
          </Box>

          <BodyShort size="small" className="mt-3" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Merk: Noen konfigurasjonsnøkler (mode=autopilot, context_tier, ask_user) gjelder kun GitHub Copilot.
            nav-pilot skriver én advarsel hvis disse er eksplisitt satt og du bruker opencode.
          </BodyShort>
        </div>

        {/* Konfigurasjon */}
        <div id="konfigurasjon">
          <div className="flex items-center gap-2 mb-2">
            <WrenchIcon fontSize="1.125rem" style={{ color: "#64748b" }} aria-hidden />
            <LinkableHeading size="small" level="3">
              Konfigurasjon
            </LinkableHeading>
          </div>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot leser brukerens konfigurasjon fra{" "}
            <code className="font-mono text-xs">~/.nav-pilot/config.toml</code>. Det finnes ingen repo-lokal
            konfigurasjon. Prioritetsrekkefølge: <strong>CLI-flagg › config.toml › innebygd standard</strong>.
          </BodyLong>

          <div className="mt-4 space-y-3">
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Interaktiv innstillingsside i terminalen
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`nav-pilot config`}</CodeBlock>
              </div>
            </div>
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Opprett konfigurasjonsfil (alle nøkler kommentert ut)
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`nav-pilot config init`}</CodeBlock>
              </div>
            </div>
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Interaktiv veiviser — velg klient, modell og modus
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`nav-pilot config setup`}</CodeBlock>
              </div>
            </div>
            <div>
              <Label size="small" style={{ color: "#64748b" }}>
                Vis effektiv konfigurasjon (fil + standardverdier)
              </Label>
              <div className="mt-1">
                <CodeBlock compact>{`nav-pilot config show`}</CodeBlock>
              </div>
            </div>
          </div>

          <div className="mt-4">
            <Label size="small" className="mb-2" style={{ color: "#64748b" }}>
              Eksempel: ~/.nav-pilot/config.toml
            </Label>
            <CodeBlock compact>
              {`# Klient (copilot er standard)
client = "opencode"

# Modell (format avhenger av klient)
model = "github-copilot/auto"

# Modus (default | plan | autopilot) — kun Copilot
# mode = "default"

# Resonneringsinnsats (none|low|medium|high|xhigh|max)
reasoning_effort = "high"

# Loggnivå
# log_level = "info"`}
            </CodeBlock>
          </div>
        </div>

        {/* Config keys table */}
        <div id="konfig-nokler">
          <LinkableHeading size="small" level="3">
            Konfigurasjonsnøkler
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Alle nøkler kan overstyres med tilsvarende CLI-flagg. Flagg har alltid høyest prioritet.
          </BodyShort>
          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
              <thead>
                <tr style={{ borderBottom: "2px solid #e2e8f0" }}>
                  {["Nøkkel", "CLI-flagg", "Tillatte verdier", "Beskrivelse"].map((h) => (
                    <th key={h} className="text-left py-2 pr-4 font-semibold" style={{ color: "#334155" }}>
                      {h}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {CONFIG_KEYS.map((row) => (
                  <tr key={row.key} style={{ borderBottom: "1px solid #e2e8f0", verticalAlign: "top" }}>
                    <td className="py-2.5 pr-4" style={{ whiteSpace: "nowrap" }}>
                      <code
                        className="text-xs font-mono rounded px-1.5 py-0.5"
                        style={{ background: "#f1f5f9", color: "#3b82f6" }}
                      >
                        {row.key}
                      </code>
                    </td>
                    <td className="py-2.5 pr-4" style={{ whiteSpace: "nowrap" }}>
                      <code className="text-xs font-mono" style={{ color: "#475569" }}>
                        {row.flag}
                      </code>
                    </td>
                    <td className="py-2.5 pr-4" style={{ color: "#64748b", fontSize: "0.75rem" }}>
                      {row.values}
                    </td>
                    <td className="py-2.5" style={{ color: "#475569" }}>
                      {row.desc}
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
   Section 7: CLI-referanse
   ═══════════════════════════════════════════════════════════════ */

function LocalModelSection() {
  return (
    <section id="lokal-modell">
      <VStack gap="space-16">
        <VStack gap="space-12">
          <LinkableHeading size="medium" level="2">
            Bakkemodellen{" "}
            <Tag variant="warning" size="small">
              alfa
            </Tag>
          </LinkableHeading>
          <BodyLong textColor="subtle">
            nav-pilot kan kjøre en modell på din egen maskin. Vi kaller den bakkemodellen: hovedagenten blir i skya og
            bestemmer, bakkemodellen står på bakken og utfører. Den trekker ingen AI-credits, uansett hvor mye den
            genererer. Til gjengjeld er den langsommere enn skyen, og den klarer bare en del av arbeidet.
          </BodyLong>
          <BodyLong textColor="subtle">
            Dette er alfa, og av som standard. Ingenting endres før du kjører{" "}
            <code className="font-mono text-xs">init</code> selv. Du trenger en Mac med Apple Silicon og 48 GB minne, og
            rundt 26 GB ledig disk: 25 GB vekter pluss Python-miljøet. Intel-Macer blir avvist, fordi MLX bare finnes
            for M-brikkene.
          </BodyLong>
        </VStack>

        <VStack id="lokal-kom-i-gang" gap="space-12">
          <LinkableHeading size="small" level="3">
            Kom i gang
          </LinkableHeading>
          <BodyShort size="small" textColor="subtle">
            Første <code className="font-mono text-xs">start</code> laster modellen inn i minnet. Ti målte oppstarter på
            seks maskiner lå alle under 50 sekunder, seks av dem under ti.
          </BodyShort>
          <CodeBlock compact>
            {`nav-pilot alpha local init      # laster ned modellen og setter opp miljøet
nav-pilot alpha local start     # starter serveren
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha local stop
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # fjern alt igjen, viser hva og hvor mye først`}
          </CodeBlock>
          <VStack id="lokal-modeller" gap="space-12">
            <LinkableHeading size="small" level="3">
              Modeller i alfa
            </LinkableHeading>
            <BodyLong size="small" textColor="subtle">
              Tre modeller er tilgjengelige. Én er standard, de to andre kan velges. Tallene er fra vårt eget sett på
              åtte oppgaver, og oppgis som spenn fordi det er spennet som skiller dem.
            </BodyLong>
            <div className="overflow-x-auto">
              <Table size="small" className="w-full">
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">Modell</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Vekter</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Løser</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Kort sagt</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  <Table.Row>
                    <Table.DataCell>
                      <VStack gap="space-2">
                        <code className="font-mono text-xs">Qwen3.6-35B-A3B-OptiQ-4bit</code>
                        <div className="text-xs" style={{ color: "#0f6d6a" }}>
                          standard
                        </div>
                      </VStack>
                    </Table.DataCell>
                    <Table.DataCell>25 GB</Table.DataCell>
                    <Table.DataCell>3–6 av 8</Table.DataCell>
                    <Table.DataCell>
                      Rask og forutsigbar. Rundt 10 sekunder per oppgave, og ingen loop i noen kjøring. Den du vil ha
                      med mindre du har en grunn til noe annet.
                    </Table.DataCell>
                  </Table.Row>
                  <Table.Row>
                    <Table.DataCell>
                      <code className="font-mono text-xs">Qwen3.8-27B-4bit</code>
                    </Table.DataCell>
                    <Table.DataCell>16 GB</Table.DataCell>
                    <Table.DataCell>1–5 av 8</Table.DataCell>
                    <Table.DataCell>
                      Dyktig og ujevn. Både det beste og det dårligste enkeltresultatet vi har målt, med to timer mellom
                      seg på samme maskin. Omtrent sju ganger tregere. Verdt å prøve på arbeid du leser gjennom etterpå.
                    </Table.DataCell>
                  </Table.Row>
                  <Table.Row>
                    <Table.DataCell>
                      <code className="font-mono text-xs">Qwen3.8-27B-8bit</code>
                    </Table.DataCell>
                    <Table.DataCell>30 GB</Table.DataCell>
                    <Table.DataCell>ikke målt</Table.DataCell>
                    <Table.DataCell>
                      Den mest omtalte, og den vi kan si minst om: de siste kjøringene ble forstyrret av en endring vi
                      selv gjorde, så vi oppgir ingen tall. Mindre plass til kontekst enn 4-bit, 65k mot 131k.
                    </Table.DataCell>
                  </Table.Row>
                </Table.Body>
              </Table>
            </div>
            <BodyShort size="small" textColor="subtle">
              Én kjøring er ikke en måling — derfor spenn og ikke median. Alle kjøringene ligger i{" "}
              <a
                href="https://github.com/navikt/mlx-workspace/blob/main/MODELS.md"
                style={{ textDecoration: "underline" }}
              >
                MODELS.md
              </a>
              .
            </BodyShort>
          </VStack>

          <LinkableHeading size="small" level="3">
            Bytte modell
          </LinkableHeading>
          <BodyLong size="small" textColor="subtle">
            <code className="font-mono text-xs">nav-pilot models</code> viser hva som er tilgjengelig; de lokale står
            merket <code className="font-mono text-xs">(local)</code>.{" "}
            <code className="font-mono text-xs">local_model</code> velger hvilken av dem serveren laster;{" "}
            <code className="font-mono text-xs">model</code> er modellen økten selv kjører på, og de settes hver for
            seg. Listen oppdateres når du kjører{" "}
            <code className="font-mono text-xs">init</code> eller <code className="font-mono text-xs">start</code>, ikke
            ved hver kommando — et nettverkskall der ville lagt seg foran alt annet nav-pilot gjør.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot models
nav-pilot config set local_model mlx-community/Qwen3.8-27B-4bit
nav-pilot alpha local init      # laster ned vektene for den nye modellen
nav-pilot alpha local start`}
          </CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Qwen 3.6 er standard, og det er ikke tilfeldig. De to Qwen 3.8-modellene ligger der fordi folk spør etter
            dem, ikke fordi de er bedre her. På våre egne oppgaver er 3.8 tregere og mye mer ujevn: to kjøringer av de
            samme åtte oppgavene, samme profil og samme maskin, ga median 88 og 906 sekunder. 3.6 ligger på 12–21
            sekunder uten timeouts over åtte kjøringer. Bytter du, må vektene lastes ned én gang til — 16 GB for 3.8
            4-bit, 30 GB for 8-bit.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Vil du slippe å starte serveren selv, kan en vanlig <code className="font-mono text-xs">nav-pilot</code>{" "}
            gjøre det for deg:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config set local_autostart true`}</CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Den er av som standard, og det er med vilje: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.
            Med den på venter launchen på at serveren er klar. To samtidige launcher starter ikke to servere.
          </BodyLong>
          <Box padding="space-16" borderRadius="8" style={{ background: "#f8fafc" }}>
            <Label size="small" spacing>
              Vi måler dette tettere enn resten av nav-pilot mens det er alfa
            </Label>
            <BodyLong size="small" textColor="subtle">
              Vi samler inn hvor mange oppgaver hver økt sender til bakkemodellen (også når svaret er null, som er
              tallet vi lærer mest av), hvilken modell du kjører, hvor lang tid serveren brukte på å starte, og når den
              henger. Aldri spørsmålene dine, koden din, filnavnene dine eller det modellen svarer.{" "}
              <code className="font-mono text-xs">DO_NOT_TRACK=1</code> skrur av alt sammen, det samme gjør{" "}
              <code className="font-mono text-xs">NAV_PILOT_TELEMETRY_ENABLED=false</code> hvis du heller vil sette det
              per verktøy.
            </BodyLong>
          </Box>
          <Box padding="space-16" borderRadius="8" style={{ background: "#fef2f2" }}>
            <Label size="small" spacing>
              Én kommando, men den ber om passordet ditt
            </Label>
            <BodyLong size="small" textColor="subtle">
              macOS lar ikke GPU-en låse nok minne til en modell på denne størrelsen som standard, så{" "}
              <code className="font-mono text-xs">init</code> hever grensen for deg med{" "}
              <code className="font-mono text-xs">sudo</code> og sier fra når den gjør det. Grensen er et tak og ikke en
              reservasjon: den tar ikke minne fra andre programmer før modellen faktisk bruker det. Den nullstilles ved
              omstart, og <code className="font-mono text-xs">start</code> hever den igjen når den trengs.
            </BodyLong>
          </Box>
          <BodyLong size="small" textColor="subtle">
            Klienten din avgjør hva du får. <code className="font-mono text-xs">nav-pilot config get client</code> sier
            hvilken du kjører.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Under <strong>opencode</strong> blir bakkemodellen en underagent som heter{" "}
            <code className="font-mono text-xs">local-worker</code>, og som hovedagenten i skyen sender avgrensede
            oppgaver til. Hovedagenten bestemmer fortsatt alt, og gjør selv det den vurderer at bakkemodellen ikke
            klarer. Under <strong>Copilot CLI</strong> kjører hele økten lokalt, fordi klienten bare håndterer én
            modelleverandør om gangen.
          </BodyLong>
        </VStack>

        <VStack id="lokal-hva-den-klarer" gap="space-12">
          <LinkableHeading size="small" level="3">
            Hva den klarer
          </LinkableHeading>
          <BodyShort size="small" textColor="subtle">
            Målt i et kontrollert testoppsett, ikke i daglig bruk. Hovedregelen: den er god til å gjennomføre en
            beslutning som allerede er tatt, og dårlig til å ta den selv. Om det lønner seg avhenger av hvor mange steg
            skymodellen trenger når den gjør oppgaven alene: bruker den mange, sparer du mye på å sende det mekaniske
            til bakkemodellen, og går oppgaven unna på to steg koster utsendingen mer enn den sparer.
          </BodyShort>
          <HGrid gap="space-16" columns={{ xs: 1, md: 2 }}>
            <Box padding="space-16" borderRadius="8" style={{ background: "#f0fdf4" }}>
              <Label size="small" spacing>
                Fungerer
              </Label>
              <BodyLong size="small" textColor="subtle">
                Slå opp noe i koden. Legge til en kommentar. Døpe om et symbol i mange filer. Tre et felt gjennom en
                mapper og kallstedene. I våre kjøringer lyktes den omtrent to av tre ganger på de største endringene, og
                der fanget testene feilene.
              </BodyLong>
            </Box>
            <Box padding="space-16" borderRadius="8" style={{ background: "#fef2f2" }}>
              <Label size="small" spacing>
                Fungerer ikke
              </Label>
              <BodyLong size="small" textColor="subtle">
                Skrive en ny fil fra bunnen: i våre forsøk gjorde den da ingenting i det hele tatt. Oppgaver der noe må
                vurderes underveis, eller der en feil endring er dyr.
              </BodyLong>
            </Box>
          </HGrid>
          <Box padding="space-16" borderRadius="8" style={{ background: "#fffbeb" }}>
            <Label size="small" spacing>
              Sjekk resultatet
            </Label>
            <BodyLong size="small" textColor="subtle">
              Bakkemodellen feiler også på måter som kompilerer. Commit eller stash før du setter den i gang, og kjør
              testene etterpå. På store endringer bør du regne med å forkaste et forsøk og prøve på nytt. Det koster deg
              tid, ikke credits.
            </BodyLong>
          </Box>
          <BodyLong size="small" textColor="subtle">
            Tiden varierer mye: fra omtrent likt med skyen på små endringer til rundt fire ganger så lenge på en
            omdøping. På store mekaniske endringer kan den være raskere enn skyen. Kjør{" "}
            <code className="font-mono text-xs">stop</code> når du ikke bruker den; den holder rundt 21 GB minne så
            lenge den er oppe.
          </BodyLong>
        </VStack>

        <VStack id="lokal-feilsoking" gap="space-12">
          <LinkableHeading size="small" level="3">
            Når noe henger
          </LinkableHeading>
          <BodyShort size="small" textColor="subtle">
            <code className="font-mono text-xs">status</code> skiller «treg» fra «død».
          </BodyShort>
          <CodeBlock compact>
            {`nav-pilot alpha local status
# står det hung: nav-pilot alpha local stop && nav-pilot alpha local start`}
          </CodeBlock>
          <BodyLong size="small" textColor="subtle">
            nav-pilot slipper gjennom én forespørsel om gangen, så flere oppgaver står i kø framfor å kjøre parallelt.
            Det er med vilje: serveren selv tar imot samtidige forespørsler og henger seg opp på dem, så ikke kall den
            direkte utenom nav-pilot.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            To ting til, som du vil møte før noe dokument nevner dem. nav-pilot avslutter en tur hvis modellen gjentar
            det samme verktøykallet åtte ganger på rad, og sier fra i økten: det er en vakt mot at den setter seg fast,
            ikke en feil i koden din. Og starter du serveren på nytt midt i en økt, må økten startes på nytt også; den
            gamle er bundet til serveren som forsvant.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Si fra med <code className="font-mono text-xs">nav-pilot feedback</code> om noe henger, om en endring
            kompilerer men er feil, eller om ventetiden ikke er verdt det. Negative erfaringer er like nyttige som
            positive.
          </BodyLong>
        </VStack>
      </VStack>
    </section>
  );
}

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
            <VStack gap="space-12">
              <div>
                <CodeBlock compact>{`brew install navikt/tap/nav-pilot`}</CodeBlock>
                <AltInstall />
              </div>
              <BodyLong size="small" style={{ color: "#64748b" }}>
                Installer også <code className="font-mono text-xs">cplt</code> før du starter en agent. Sandboxing er et
                krav på Nav-utstyr.
              </BodyLong>
            </VStack>
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
              { label: "Selvoppdatering", cmd: "nav-pilot upgrade" },
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

          <div className="overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0 mt-4">
            <table className="w-full min-w-max text-sm" style={{ borderCollapse: "collapse" }}>
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
          <BodyLong size="small" className="mt-3" style={{ color: "#475569" }}>
            <code className="font-mono text-xs">install</code> spør hvor den skal installere — i repoet (
            <code className="font-mono text-xs">.github/</code>) eller i hjemmekatalogen (
            <code className="font-mono text-xs">~/.copilot/</code>). Bruk{" "}
            <code className="font-mono text-xs">--repo</code> eller <code className="font-mono text-xs">--user</code>{" "}
            for å svare på forhånd og hoppe over spørsmålet.
          </BodyLong>
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
                  { label: "JSON-output for alle kommandoer", cmd: "nav-pilot list --installed --json | jq ." },
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

        <Box background="neutral-soft" borderRadius="8" padding="space-12">
          <BodyShort size="small" style={{ color: "#475569" }}>
            Flyten fra installasjon til daglig bruk er dekket i{" "}
            <NextLink href="#installasjon" className="text-blue-600 hover:underline">
              Kom i gang
            </NextLink>{" "}
            og{" "}
            <NextLink href="#kommandooversikt" className="text-blue-600 hover:underline">
              CLI-referanse
            </NextLink>
            . Denne seksjonen viser kun filstrukturen.
          </BodyShort>
        </Box>
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
                desc: "Én inngangsport — ruter til riktig fase og skill. Delegerer til @kafka, @security-champion og laster $nav-auth, $nais.",
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
          <div className="mt-4 grid gap-3 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
            {[
              {
                label: "Kom i gang",
                href: "#kom-i-gang",
                desc: "Minimumssteg for installasjon og første bruk",
              },
              {
                label: "CLI-referanse",
                href: "#kommandooversikt",
                desc: "Kommandoreferanse med eksempler",
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
