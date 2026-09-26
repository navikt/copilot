import { Heading, BodyShort, BodyLong, Box, HGrid, HStack, Label, VStack, Tag } from "@navikt/ds-react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
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
import { Suspense } from "react";
import { FALLBACK_TABLE, MANIFEST_URL, getLocalModels, type LocalModel } from "@/lib/local-models";

export const metadata: Metadata = {
  title: "nav-pilot dokumentasjon",
  description: "Dokumentasjon for nav-pilot, Navs AI-utviklerverktøy for GitHub Copilot.",
};

/* Oppskrifter for alpha decide. String.raw keeps the shell's \n and \ intact. */

const COMMIT_EXPLAINS_WHY_HOOK = String.raw`#!/bin/sh
# Advarer når meldingen bare beskriver det diffen viser. Stopper aldri commiten.
command -v nav-pilot >/dev/null 2>&1 || exit 0

{
  printf 'Commit message:\n-----\n'
  grep -v '^#' "$1"
  printf -- '-----\n\nDiff:\n-----\n'
  git diff --cached | head -c 7500
  printf -- '\n-----\n'
} | nav-pilot alpha decide \
  "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \
  --options yes,no --evidence - --threshold 0.7 --expect no \
  --timeout 3s >/dev/null 2>&1

if [ $? -eq 0 ]; then
  echo "commit-msg: meldingen ser ut til å si hva som endret seg, men ikke hvorfor." >&2
fi
exit 0`;

const DECIDE_EVAL_CASES = String.raw`{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nDiff:\n...","expect":"no"}
{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nThe batch job takes 20s on large tenants.\n\nDiff:\n...","expect":"yes"}`;

const DECIDE_PR_DESCRIPTION = String.raw`gh pr view N --json title,body \
    -q '"Pull request title: " + .title + "\n-----\n"
        + (if (.body // "") == "" then "(empty)" else .body end) + "\n-----"' \
  | nav-pilot alpha decide \
    "Does this pull request description explain why the change is needed?" \
    --options yes,no --evidence -`;

const DECIDE_ISSUE_LABEL = String.raw`gh issue view N --json title,body -q '"Title: " + .title + "\n\n" + (.body // "")' \
  | nav-pilot alpha decide \
    "Is this GitHub issue a bug report (something does not work as intended), a feature request (new or changed functionality), or a question (something to clarify, investigate or decide)?" \
    --options bug,feature,question --evidence - --json \
  | jq -r 'select(.p[.choice] >= 0.9) | .choice'`;

const DECIDE_LOG_TRIAGE = String.raw`kubectl logs deploy/min-app --since=1h | tail -c 30000 \
  | nav-pilot alpha decide \
    "Do these logs show the app failing to reach a dependency?" \
    --options yes,no --evidence -`;

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
      { id: "cplt-sikkerhetsniva", label: "Sikkerhetsnivå i cplt" },
      { id: "nar-strict-ikke-anbefales", label: "Når strict ikke anbefales" },
      { id: "logging-av-blokkeringer", label: "Logging av blokkeringer" },
      { id: "hvorfor-nav-pilot", label: "Hvorfor nav-pilot?" },
      { id: "hva-nav-pilot-vet", label: "Hva nav-pilot vet" },
    ],
  },
  {
    id: "kom-i-gang",
    label: "Kom i gang",
    children: [
      { id: "installasjon", label: "Installasjon (5 min)" },
      { id: "hvor-installere", label: "Hvor skal artefaktene installeres?" },
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
      { id: "personvern", label: "Personvern og telemetri" },
    ],
  },
  {
    id: "collections",
    label: "Agentpakke",
    children: [{ id: "planning-skills", label: "Planning skills" }],
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
      { id: "lokal-decide", label: "Typede avgjørelser" },
      { id: "lokal-decide-oppskrifter", label: "Oppskrifter for decide" },
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
    command: "nav-pilot install nav-pilot",
    description: "Installer agentpakka. Spør om repoet (.github/) eller hjemmekatalogen (~/.copilot/)",
  },
  {
    command: "nav-pilot install --user",
    description: "Installer agenter, skills og instruksjoner til ~/.copilot (alle repoer)",
  },
  { command: "nav-pilot install --dry-run nav-pilot", description: "Forhåndsvis hva som installeres" },
  { command: "nav-pilot install --force nav-pilot", description: "Overskriv lokalt endrede filer" },
  { command: "nav-pilot list", description: "Vis agentpakka og enkeltkomponenter" },
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
  { command: "nav-pilot feedback", description: "Rapporter feil. Åpner GitHub issue med diagnostikk" },
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
  {
    command: "nav-pilot alpha local <command>",
    description: "Lokal modell (alfa): init, start, status, models, use, restart, stop, ask, on, off, purge",
  },
  {
    command: 'nav-pilot alpha decide "<spørsmål>" --options a,b',
    description: "Typet avgjørelse fra den lokale modellen (alfa). Se --help",
  },
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
                <PakkeSection />
                <PipelineSection />
                <CompetenceSection />
                <SyncSection />
                <CustomizationSection />
                {/* Skallet viser reservekopien til manifestet er hentet. */}
                <Suspense fallback={<LocalModelSection models={FALLBACK_TABLE.models} />}>
                  <LiveLocalModelSection />
                </Suspense>
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
            nav-pilot inneholder <strong>én planleggingsagent, fire planning skills og én agentpakke</strong> med alle
            Navs agenter, skills, instruksjoner og prompts. CLI-et installerer markdown-filer. Selve AI-funksjonaliteten
            kjøres av GitHub Copilot.
          </BodyLong>

          {/* Component overview cards */}
          <div className="mt-6 grid gap-3" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(260px, 1fr))" }}>
            {[
              { name: "@nav-pilot", desc: "Planleggingsagent, din inngangsport", color: "#3b82f6", Icon: CompassIcon },
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
                </NextLink>
                . Det er den anbefalte og enkleste løsningen. Hvis du velger en annen løsning, må du selv sette deg inn
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
          <BodyLong style={{ color: "#475569" }}>
            nav-pilot gir cplt katalogen du står i som prosjektkatalog, med{" "}
            <code className="font-mono text-xs">--project-dir</code>. Agenten kan lese og skrive der og under, ikke i
            mapper ved siden av. Står du i en undermappe av et repo, gjelder sandboxen bare undermappa, ikke hele
            repoet. Repoets instruksjoner i roten, som <code className="font-mono text-xs">.github/</code> og{" "}
            <code className="font-mono text-xs">AGENTS.md</code>, kan agenten likevel lese. Trenger agenten hele repoet,
            starter du fra roten av repoet eller kjører{" "}
            <code className="font-mono text-xs">nav-pilot --project-dir &lt;katalog&gt;</code>. Hjemmekatalogen og{" "}
            <code className="font-mono text-xs">/</code> avviser cplt selv, fordi de er for vide.
          </BodyLong>
        </VStack>

        <VStack id="cplt-sikkerhetsniva" gap="space-12">
          <LinkableHeading size="small" level="3">
            Sikkerhetsnivå i cplt
          </LinkableHeading>
          <BodyLong style={{ color: "#475569" }}>
            <code className="font-mono text-xs">nav-pilot doctor</code> sjekker sikkerhetsnivået til cplt og anbefaler{" "}
            <code className="font-mono text-xs">sandbox.preset = strict</code>. Det presetet er en nettverkslås.{" "}
            <code className="font-mono text-xs">gh_guard</code> og <code className="font-mono text-xs">git_guard</code>{" "}
            er allerede på i <code className="font-mono text-xs">standard</code>, så det strict legger til er
            nettverket: tvungen proxy, <code className="font-mono text-xs">git_guard</code> som blokkerer i stedet for å
            advare, og <code className="font-mono text-xs">proxy.default_allowlist</code>. Den siste er den viktige.
            Bare cplt sin innebygde host-liste, pluss det{" "}
            <code className="font-mono text-xs">proxy.allowed_domains</code> peker på, er nåbart. Alt annet blokkeres.
          </BodyLong>
          <Box background="warning-soft" borderRadius="8" padding="space-16">
            <BodyLong style={{ color: "#475569" }}>
              cplt sin innebygde liste dekker GitHub Copilot og de offentlige pakkeregistrene. Den dekker ingenting av
              Navs. Setter du presetet for hånd, slutter nav-pilot sin telemetri å komme fram, og skills som{" "}
              <code className="font-mono text-xs">aksel-builder</code>,{" "}
              <code className="font-mono text-xs">observability-debugging</code> og{" "}
              <code className="font-mono text-xs">nav-auth</code> mister hostene de er bygget rundt, uten at noe på
              skjermen forteller deg hvorfor.
            </BodyLong>
          </Box>
          <BodyLong style={{ color: "#475569" }}>Sett det derfor via nav-pilot:</BodyLong>
          <CodeBlock>{"nav-pilot config     # velg raden «cplt security posture»"}</CodeBlock>
          <BodyLong style={{ color: "#475569" }}>
            Den skriver host-lista til <code className="font-mono text-xs">~/.nav-pilot/cplt-allowed-domains.txt</code>,
            peker <code className="font-mono text-xs">proxy.allowed_domains</code> dit, og setter så presetet, i den
            rekkefølgen, slik at låsen aldri rekker å tre i kraft uten hostene. Har du allerede en egen{" "}
            <code className="font-mono text-xs">proxy.allowed_domains</code>, lar nav-pilot den være i fred og sier fra
            at du må ta med hostene selv. cplt-config er personlig, så nav-pilot setter den aldri stilltiende, og nøkler
            du har satt selv gjelder fortsatt foran presetet.
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            Fila er en fullstendig liste, ikke bare Nav-hostene, fordi{" "}
            <code className="font-mono text-xs">proxy.allowed_domains</code> blokkerer alt utenfor seg selv uansett hva{" "}
            <code className="font-mono text-xs">proxy.default_allowlist</code> står på, og cplt sin innebygde liste er
            per agent: bare copilot-lista har GitHub og Copilot i seg, mens opencode har{" "}
            <code className="font-mono text-xs">opencode.ai</code> og{" "}
            <code className="font-mono text-xs">models.dev</code>.
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            <code className="font-mono text-xs">nav-pilot local</code> sender prompten via en loop-guard på{" "}
            <code className="font-mono text-xs">127.0.0.1</code>. cplt blokkerer localhost som standard, så nav-pilot
            sender porten med som <code className="font-mono text-xs">--allow-localhost &lt;port&gt;</code> ved hver
            lokale oppstart. Det er én navngitt port, ikke den maskinvide bryteren, som{" "}
            <code className="font-mono text-xs">proxy.forced</code> overstyrer. Én port overlever tvungen proxy på både
            macOS og Linux, så strict og lokal modell utelukker ikke hverandre.
          </BodyLong>
        </VStack>

        <VStack id="nar-strict-ikke-anbefales" gap="space-12">
          <LinkableHeading size="small" level="3">
            Når strict ikke anbefales
          </LinkableHeading>
          <BodyLong style={{ color: "#475569" }}>
            På Linux krever <code className="font-mono text-xs">proxy.forced</code> at kjernen kan håndheve
            nettverksrestriksjon i Landlock: ABI v4, altså kjerne 6.7 eller nyere med Landlock påslått. Under det
            degraderer ikke cplt, den nekter å starte i det hele tatt. En anbefaling som stopper hver eneste økt på
            maskinen er verre enn problemet den løser, så <code className="font-mono text-xs">nav-pilot doctor</code> og
            innstillingssiden anbefaler ikke strict der, og sier hvorfor i stedet.
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            nav-pilot spør kjernen direkte, med samme systemkall som cplt bruker, i stedet for å lese{" "}
            <code className="font-mono text-xs">uname</code>. En kjerneversjon er bare en indikasjon: Landlock kan være
            kompilert bort eller slått av ved oppstart, og da ville en versjonssjekk sagt «går fint» rett før cplt
            nekter å starte. macOS har ingen slik grense; der håndheves det samme med Seatbelt.
          </BodyLong>
        </VStack>

        <VStack id="logging-av-blokkeringer" gap="space-12">
          <LinkableHeading size="small" level="3">
            Logging av blokkeringer
          </LinkableHeading>
          <BodyLong style={{ color: "#475569" }}>
            <code className="font-mono text-xs">proxy.log_level</code> styrer hva cplt sin proxy skriver til stderr.
            Standardverdien er <code className="font-mono text-xs">none</code>, men cplt hever den selv til{" "}
            <code className="font-mono text-xs">blocked</code> så snart en host-liste er aktiv. Sikkerhetsnivået
            nav-pilot anbefaler skriver alltid host-lista til fil, så på strict er{" "}
            <code className="font-mono text-xs">blocked</code> allerede i kraft uten at du setter noe.
          </BodyLong>
          <BodyLong style={{ color: "#475569" }}>
            Kjører du <code className="font-mono text-xs">standard</code> uten egen{" "}
            <code className="font-mono text-xs">proxy.allowed_domains</code>, er det ingen host-liste å heve for, og
            proxyen tier da om alle andre blokkeringer og feil også: treff i cplt sin egen blokkliste, stengte porter,
            hosts som slår opp til private eller link-local IP-er, og oppslag som feiler. Vil du se dem, setter du
            nivået selv:
          </BodyLong>
          <CodeBlock>{"cplt config set proxy.log_level blocked"}</CodeBlock>
          <BodyLong style={{ color: "#475569" }}>
            <code className="font-mono text-xs">audit.enabled</code> ser ut som svaret på det samme, men er det ikke i
            dagens cplt. <code className="font-mono text-xs">[audit]</code>-seksjonen leses og vises av{" "}
            <code className="font-mono text-xs">cplt config show</code>, men ingenting bruker den. Å slå den på gir
            ingen logg. Vil du ha en fil å lese etterpå, er det{" "}
            <code className="font-mono text-xs">proxy.log_file</code> som faktisk skriver en, og den dekker
            proxy-verdiktene, ikke avgjørelsene til gh- og git-vakta. Ikke forveksle den med{" "}
            <code className="font-mono text-xs">sandbox.audit</code>, som er noe annet og allerede på: den viser
            rapporten over filendringer på skjermen etter at agenten har avsluttet.
          </BodyLong>
        </VStack>

        {/* Why nav-pilot */}
        <div id="hvorfor-nav-pilot">
          <LinkableHeading size="small" level="3">
            Hvorfor nav-pilot?
          </LinkableHeading>
          <BodyLong className="mt-3" style={{ color: "#475569" }}>
            oh-my-openagent og lignende verktøy bygger bedre <em>orkestrering</em>, som multi-agent-delegering,
            parallellkjøring og selvkorrigering. nav-pilot bygger bedre <em>kunnskap</em>. Orkestrering blir
            standardvare, mens institusjonell kunnskap er vanskelig å kopiere.
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
              "At HikariCP default pool (10) er for stor for containere, så start med 3",
              "At du aldri skal sette CPU-limits i Nais (bare requests)",
              "At PII aldri skal logges, så du logger sakId, ikke fnr",
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
                Installer agentpakka i repoet ditt
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
              Du kan bruke nav-pilot på tre måter. Velg den som passer deg best:
            </BodyLong>
            <div className="space-y-4">
              <div>
                <Label size="small" style={{ color: "#64748b" }}>
                  Terminal (GitHub Copilot CLI)
                </Label>
                <div className="mt-1">
                  <CodeBlock compact>
                    {`cplt --project-dir . -- --agent nav-pilot --prompt "Jeg trenger en ny tjeneste som behandler dagpengesøknader"`}
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
                  Starter interaktiv modus, som sjekker oppdateringer og starter Copilot med valgt agent.
                </BodyLong>
              </div>
            </div>
          </div>
        </div>

        <div id="hvor-installere">
          <LinkableHeading size="small" level="3">
            Hvor skal artefaktene installeres?
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Tre former er i bruk i Nav, og de løser ulike problemer. <code className="font-mono text-xs">install</code>{" "}
            spør hvis du ikke svarer på forhånd med <code className="font-mono text-xs">--repo</code> eller{" "}
            <code className="font-mono text-xs">--user</code>.
          </BodyLong>
          <ul className="mt-3 space-y-2 list-disc pl-5" style={{ color: "#475569" }}>
            <li>
              <strong>Repo</strong> (<code className="font-mono text-xs">--repo</code>, skriver til{" "}
              <code className="font-mono text-xs">.github/</code>): hele teamet får det samme, prompts virker, og
              Copilot på github.com ser filene fordi de er sjekket inn. Til gjengjeld ligger de i repoet og i hver diff.
            </li>
            <li>
              <strong>Personlig</strong> (<code className="font-mono text-xs">--user</code>, skriver til{" "}
              <code className="font-mono text-xs">~/.copilot/</code>): følger deg på tvers av alle repoer, og ingenting
              sjekkes inn. Den tar ikke med prompts, og når verken github.com eller resten av teamet.
            </li>
            <li>
              <strong>Hub-repo</strong>: en repo-installasjon i et repo som ikke er en applikasjon, pluss teamets egne
              skills lagt inn for hånd i det samme <code className="font-mono text-xs">.github/</code>. Konteksten
              følger arbeidskatalogen, så den gjelder mens du står i hub-repoet.
            </li>
          </ul>
          <BodyLong className="mt-3" size="small" style={{ color: "#64748b" }}>
            Formene utelukker ikke hverandre, og <code className="font-mono text-xs">nav-pilot sync</code> uten
            scope-flagg synker alle scope som har en tilstandsfil. Avveiningene i sin helhet står i{" "}
            <a
              href="https://github.com/navikt/copilot/blob/main/docs/README.nav-pilot.md#hvor-skal-artefaktene-installeres"
              className="text-blue-600 hover:underline"
            >
              README.nav-pilot.md
            </a>
            .
          </BodyLong>
          <BodyLong className="mt-4" style={{ color: "#475569" }}>
            Personlig installasjon:
          </BodyLong>
          <div className="mt-4">
            <CodeBlock compact>{`nav-pilot install --user`}</CodeBlock>
          </div>
          <BodyLong className="mt-3" size="small" style={{ color: "#64748b" }}>
            Filene installeres til <code className="font-mono text-xs">~/.copilot/</code>. Agenter og skills plukkes opp
            automatisk av GitHub Copilot. Instruksjoner krever{" "}
            <code className="font-mono text-xs">COPILOT_CUSTOM_INSTRUCTIONS_DIRS</code> og fungerer kun med Copilot CLI.
            nav-pilot setter denne automatisk i interaktiv modus. OpenCode mottar Nav-kontekst på en annen måte, se{" "}
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
            Du trenger ikke huske skill-navn. Bare beskriv oppgaven, så bruker nav-pilot riktig kunnskap automatisk. Her
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
   Section 3: Agentpakke
   ═══════════════════════════════════════════════════════════════ */

function PakkeSection() {
  return (
    <section id="collections">
      <VStack gap="space-16">
        <div>
          <LinkableHeading size="medium" level="2">
            Agentpakke
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Alt Nav-innhold installeres som én agentpakke: <code className="font-mono text-xs">nav-pilot</code>.{" "}
            <code className="font-mono text-xs">nav-pilot install nav-pilot</code> gir deg alle agenter, skills,
            instruksjoner, prompts, hooks og extensions. Det er bevisst alt: skills lastes når de trengs, de fleste
            instruksjonene er scopet til filmønstre og slår aldri til i et repo som ikke har dem, noen få gjelder hver
            tur, og bare nav-pilot-personaene er primæragenter. Vil du ha mindre, velger du bort i den interaktive
            velgeren. Fravalgene huskes og overlever sync.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Merk at hooks er kjørbar kode: et Python-skript Copilot CLI kjører ved hvert verktøykall som treffer
            matcheren, ikke tekst modellen leser. <code className="font-mono text-xs">nav-pilot install</code> legger
            dem inn sammen med resten, så les dem før du stoler på dem. Skriptene ligger i{" "}
            <code className="font-mono text-xs">.github/hooks/</code> (repo) eller{" "}
            <code className="font-mono text-xs">~/.copilot/hooks/</code> (bruker), og{" "}
            <code className="font-mono text-xs">nav-pilot uninstall</code> fjerner bare oppføringene nav-pilot selv har
            skrevet. Dine egne hooks blir stående.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Portene slipper gjennom når Python svikter: mangler <code className="font-mono text-xs">python3</code>,
            feiler skriptet eller svarer det ikke innen ett sekund før fristen, blir kallet tillatt. Hver port har et
            unntak, og begrunnelsen modellen får sier hvilket: <code className="font-mono text-xs">POLL_OK=1</code>{" "}
            foran kommandoen for polling-porten, og en kommentar med <code className="font-mono text-xs">ARIA_OK</code>{" "}
            og begrunnelsen ved rollen for ARIA-porten, skrevet etter at utvikleren har sagt ja.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilots egne hooks (løkkevakt og maskering) kjører med maskering og løkkevakt på selv om{" "}
            <code className="font-mono text-xs">config.toml</code> ikke lar seg lese, og sier fra på stderr. Beskjeden
            fra løkkevakten nevner ikke terskelen, så modellen ikke hever den selv; den står på stderr for deg.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Teamet ditt kan påvirke verktøykassa uten å bygge den selv.{" "}
            <NextLink href="/nav-pilot/agentpakker" className="underline">
              Agentpakker
            </NextLink>{" "}
            tar det i fire steg: bruk en pakke som finnes, ta delene du trenger, bygg videre på en, og lag din egen
            først når ingenting av det holder.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            For noen agentpakker, for eksempel <code className="font-mono text-xs">source = nais/pilot</code>, henter
            nav-pilot manifestet fra GitHub ved hver oppstart. Svarer ikke GitHub innen 15 sekunder, bruker nav-pilot
            manifestet fra forrige vellykkede oppstart og skriver en advarsel: hvilken kilde, hvor gammel kopien er og
            hvilken commit den er fra. Uten en lagret kopi venter nav-pilot på GitHub som før, og starter ingenting hvis
            hentingen feiler.
          </BodyLong>
        </div>

        {/* Planning skills table */}
        <div id="planning-skills">
          <LinkableHeading size="small" level="3">
            Planning skills
          </LinkableHeading>
          <BodyShort size="small" className="mt-2 mb-4" style={{ color: "#475569" }}>
            Agentpakka inkluderer fire planning skills som utgjør <strong>nav-pilot-pipelinen</strong>:
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
            nav-pilot jobber i fire faser med eksplisitte stopp mellom hver. Du bestemmer når du går videre. nav-pilot
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
            produktivitetsgevinst på repetitive oppgaver. Gevinsten forsvinner, og kan bli negativ, på oppgaver som
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
                  Grønn sone, der AI genererer full kode
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
                  Rød sone, der du koder og AI leverer stubs
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
            kode-stubs med <code>TODO</code>-kommentarer, ikke full implementasjon. Du skriver kjernelogikken selv for å
            bygge dyp forståelse.
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
              GitHub Actions-workflow som åpner PR-er automatisk, som Dependabot, men for Copilot-tilpasninger. PR-en
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
                { label: "Oppdater filer direkte (spør før den sletter filer)", cmd: "nav-pilot sync --apply" },
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
            AGENTS.md og .github/copilot-instructions.md oppdateres aldri automatisk, siden de alltid er
            repo-spesifikke.
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
            Filer i <code className="font-mono text-xs">overrides</code> hoppes helt over under sync. Ingen
            hash-sammenligning, ingen PR-diff. Du kan trygt slette filene etterpå, og de blir ikke lagt til igjen.
            Alternativt kan du velge bort Next.js-filene i den interaktive velgeren ved installasjon.
          </BodyShort>
          <BodyShort size="small" className="mt-2" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Sletter du en fil manuelt uten override, markeres den som «ignorert» og gjenopprettes ikke av sync. Legg den
            til igjen med <code className="font-mono text-xs">nav-pilot install</code> hvis du ombestemmer deg.
          </BodyShort>
          <BodyShort size="small" className="mt-2" style={{ color: "#94a3b8", fontStyle: "italic" }}>
            Har teamet en egen versjon av en fil med samme navn som kilden (f.eks. en egen{" "}
            <code className="font-mono text-xs">kotlin-app-config</code> skill) fra før{" "}
            <code className="font-mono text-xs">nav-pilot install</code>, hopper install over den og sync lar den være.
            I et repo med kopierte filer og uten install vil sync prøve å overskrive den. Bruk overrides for å beskytte
            filen der. Filer med navn som ikke finnes i kilden blir aldri berørt av sync.
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
                a: "Oppdateringen tar kildens versjon, men lagrer din kopi som <fil>.orig ved siden av og sier fra. I CI står filen under «Changed» i PR-en med merknad om lokale endringer. Du kan gjennomgå, merge selektivt eller lukke PR-en. Workflowen tvinger aldri oppdateringer.",
              },
              {
                q: "Kan jeg sjekke oppdateringer lokalt uten CI?",
                a: "Ja. Kjør nav-pilot sync for å sjekke, eller nav-pilot sync --apply for å oppdatere direkte.",
              },
              {
                q: "Hvordan er dette forskjellig fra Dependabot?",
                a: "Samme konsept med automatiske oppdaterings-PR-er, men for Copilot-tilpasningsfiler. Sammenligner SHA-256-hasher i stedet for semantisk versjonering.",
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
                a: "Ja. Opprett .github/copilot-sync.json med overrides, eller velg bort Next.js-filene i den interaktive velgeren ved installasjon.",
              },
              {
                q: "Hva skjer hvis vi har en egen fil med samme navn som kilden?",
                a: "Fantes filen før nav-pilot install, tar nav-pilot den aldri over: install hopper over den og sier fra, og sync rører den ikke. I et repo uten nav-pilot install (kopierte filer) foreslår sync å overskrive den med kildens versjon. Legg den i overrides for å beskytte den. Filer med navn som ikke finnes i kilden ignoreres helt.",
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
    key: "version",
    flag: "—",
    values: "1",
    desc: "Skjemaversjon. Mangler den, leses filen som versjon 1, og nav-pilot sier fra med én linje.",
  },
  {
    key: "client",
    flag: "--client",
    values: "copilot · opencode · pi (standard: copilot)",
    desc: "Klient å starte: copilot, opencode eller pi (eksperimentell). Alle kjører i cplt-sandkassen.",
  },
  {
    key: "source",
    flag: "--source",
    values: "owner/name eller en absolutt sti (standard: navikt/copilot)",
    desc: "Hvor agentpakken hentes fra: et GitHub-repo eller en lokal checkout. Settes av install --source --save-source; nav-pilot config unset source går tilbake til standarden.",
  },
  {
    key: "model",
    flag: "--model",
    values: "modell-id, f.eks. claude-opus-4.8",
    desc: "Modell å bruke. En Copilot-id som claude-opus-4.8 virker for copilot og opencode (opencode kjører den som github-copilot/<id>); opencode tar også provider/model. nav-pilot config explain model lister id-ene.",
  },
  {
    key: "mode",
    flag: "--mode",
    values: "default · plan · autopilot (standard: default)",
    desc: "Modus for Copilot-agenten. plan tilsvarer opencode --agent plan; autopilot er kun Copilot.",
  },
  {
    key: "reasoning_effort",
    flag: "--effort",
    values: "none · low · medium · high · xhigh · max",
    desc: "Resonneringsinnsats. Copilot bruker --effort, opencode bruker --variant.",
  },
  {
    key: "context_tier",
    flag: "--context",
    values: "default · long_context",
    desc: "Kontekstnivå. Kun Copilot, og nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "allow_all_tools",
    flag: "--allow-all-tools / --no-allow-all-tools",
    values: "true · false (standard: false)",
    desc: "La agenten kjøre alle verktøy uten å spørre først.",
  },
  {
    key: "ask_user",
    flag: "--ask-user / --no-ask-user",
    values: "true · false (standard: true)",
    desc: "La agenten stoppe og spørre deg. Kun Copilot, og nav-pilot advarer om feltet er satt for opencode.",
  },
  {
    key: "auto_launch",
    flag: "--auto-launch / --no-auto-launch",
    values: "true · false (standard: true)",
    desc: "Start kodeagenten etter synk eller installasjon. Med false skriver nav-pilot bare ut kommandoen.",
  },
  {
    key: "auto_update",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "Oppgrader nav-pilot automatisk når en ny versjon er ute, uten å spørre.",
  },
  {
    key: "log_level",
    flag: "--log-level",
    values: "none · error · warning · info · debug · all · default",
    desc: "Loggnivå for Copilot CLI.",
  },
  {
    key: "otel_log_level",
    flag: "--otel-log-level",
    values: "none · error · warning · warn · info · debug · verbose · all (standard: none)",
    desc: "Loggnivå for OpenTelemetry i Copilot CLI (OTEL_LOG_LEVEL). En OTEL_LOG_LEVEL i skallet vinner, og config show merker den env.",
  },
  {
    key: "local_enabled",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "Send avgrensede oppgaver til en lokal modell (alfa). Settes av alpha local init, nullstilles av alpha local off. Så lenge den er false finnes ingen lokale modeller i nav-pilot.",
  },
  {
    key: "local_autostart",
    flag: "—",
    values: "true · false (standard: false)",
    desc: "La en vanlig nav-pilot starte den lokale serveren når den trengs og ingen kjører. Av som standard: å starte en 21 GB prosess uten å bli bedt om det er ikke greit.",
  },
  {
    key: "local_loop_guard",
    flag: "—",
    values: "et heltall (standard: 8)",
    desc: "Hvor mange identiske verktøykall på rad som avslutter en lokal tur, uansett hva de returnerer. Gir kallene samme resultat hver gang, holder det med halvparten (minst 2).",
  },
  {
    key: "local_model",
    flag: "—",
    values: "modell-id fra manifestet",
    desc: "Hvilken lokal modell serveren laster (alfa). Tom betyr standardmodellen i manifestet. Enklest satt med nav-pilot alpha local use <key>.",
  },
  {
    key: "hook_loop_guard",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Samme løkkeregel i alle Copilot CLI-økter, også i skyen: en postToolUse-hook i ~/.copilot/hooks/ sier fra til modellen når den står fast. false fjerner hooken ved neste oppstart.",
  },
  {
    key: "hook_redact_secrets",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Masker hemmeligheter (GitHub-tokener, AWS-nøkkel-id-er, private nøkler, JWT-er, verdien i password=/api_key=) i verktøyresultater før modellen leser dem, i alle Copilot CLI-økter.",
  },
  {
    key: "hook_redact_fnr",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Masker fødselsnummer, D-nummer og H-nummer i verktøyresultater. Bare elleve sifre der datoen og begge kontrollsifrene stemmer blir maskert.",
  },
  {
    key: "hook_injection_note",
    flag: "—",
    values: "true · false (standard: true)",
    desc: "Sett en merknad foran verktøyresultater som ser ut som instrukser til modellen («ignore previous instructions», rollemarkører), så modellen behandler dem som data. Stopper ingenting.",
  },
  {
    key: "copilot_auth_mode",
    flag: "—",
    values: "auto · env_only · gh_only (standard: auto)",
    desc: "Hvilken innlogging som når cplt for Copilot. auto begrenser ingenting; env_only krever et token i GH_TOKEN, GITHUB_TOKEN eller COPILOT_GITHUB_TOKEN; gh_only fjerner dem.",
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
                badge: "Eksperimentell",
                badgeColor: "#b45309",
                badgeBg: "#fef3c7",
                desc: "pi-klienten i cplt-sandkassen, med Nav-kontekst (skills, agenter og AGENTS.md) levert ved oppstart. Krever både pi og cplt.",
                color: "#b45309",
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
            <Box background="info-soft" borderRadius="8" padding="space-12">
              <BodyShort size="small" style={{ color: "#475569" }}>
                For <code className="font-mono text-xs">copilot</code> via{" "}
                <code className="font-mono text-xs">cplt</code>: nav-pilot henter ikke ut GitHub-tokenet selv. Med
                gh-guarden i <code className="font-mono text-xs">cplt</code> på skaffer{" "}
                <code className="font-mono text-xs">cplt</code> det, fra{" "}
                <code className="font-mono text-xs">GH_TOKEN</code>,{" "}
                <code className="font-mono text-xs">GITHUB_TOKEN</code>,{" "}
                <code className="font-mono text-xs">COPILOT_GITHUB_TOKEN</code> eller{" "}
                <code className="font-mono text-xs">gh auth token</code>. Med{" "}
                <code className="font-mono text-xs">copilot_auth_mode</code> styrer du hvilke kilder som slipper fram:{" "}
                <code className="font-mono text-xs">env_only</code> avbryter oppstart uten token i miljøet,{" "}
                <code className="font-mono text-xs">gh_only</code> fjerner token-variablene fra barnemiljøet.
              </BodyShort>
            </Box>
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
                    holdes fersk med konfliktsdeteksjon, så dine egne redigeringer overskrives ikke.
                  </>
                ),
                color: "#059669",
                bg: "#ecfdf5",
              },
              {
                title: "GPT-6 Sol som standard",
                desc: (
                  <>
                    Når ingen modell er konfigurert, starter nav-pilot opencode med GPT-6 Sol. Ditt eget modellvalg
                    vinner over standarden.
                  </>
                ),
                color: "#3b82f6",
                bg: "#eff6ff",
              },
              {
                title: "OTel-telemetri",
                desc: "OpenTelemetry-konfigurasjon settes opp automatisk. Ingen manuell konfigurasjon er nødvendig.",
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
              engangseksport, men trengs <strong>ikke</strong> i den normale flyten, siden nav-pilot håndterer dette
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
                Interaktiv veiviser der du velger klient, modell og modus
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
              {`# Skjemaversjon
version = 1

# Klient (copilot er standard)
client = "opencode"

# Modell. En Copilot-id som claude-opus-4.8 virker for copilot og opencode;
# opencode kjører den som github-copilot/claude-opus-4.8.
# Ubestemt lar klienten velge selv.
# model = "claude-opus-4.8"

# Modus (default | plan | autopilot), kun Copilot
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

        <div id="personvern">
          <LinkableHeading size="small" level="3">
            Personvern og telemetri
          </LinkableHeading>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            nav-pilot sender bruksmålinger til Nav: hvor ofte kommandoene kjøres, hvilken klient og hvilke innstillinger
            som brukes, og hvilke typer feil som oppstår. Målingene er faste kategorier og tall. Prompter, kode,
            filinnhold og filnavn er aldri med. Maskinen kjennes igjen på en pseudonym ID, ikke på navn eller
            brukernavn. Starter nav-pilot Copilot, slår den også på Copilots egne målinger og sporinger mot samme
            mottaker. Kjører økten i et navikt-repo, merkes de med repoets navn.
          </BodyLong>
          <BodyLong className="mt-2" style={{ color: "#475569" }}>
            Første gang du kjører nav-pilot i en terminal, står dette på én linje. Slå av målingene i shell-profilen din
            med én av disse:
          </BodyLong>
          <CodeBlock compact>{`export DO_NOT_TRACK=1
export NAV_PILOT_TELEMETRY_ENABLED=false`}</CodeBlock>
          <BodyLong className="mt-2" size="small" style={{ color: "#64748b" }}>
            <code className="font-mono text-xs">DO_NOT_TRACK=1</code> slår av målinger i alle verktøy som følger
            konvensjonen, ikke bare nav-pilot. Alt som måles, står i{" "}
            <a
              href="https://github.com/navikt/copilot/blob/main/cli/nav-pilot/TELEMETRY.md"
              className="text-blue-600 hover:underline"
            >
              TELEMETRY.md
            </a>
            .
          </BodyLong>
        </div>
      </VStack>
    </section>
  );
}

/* ═══════════════════════════════════════════════════════════════
   Section 7: CLI-referanse
   ═══════════════════════════════════════════════════════════════ */

// Tabellen over lokale modeller hentes fra manifestet i navikt/mlx-workspace
// når siden kjører (src/lib/local-models.ts), med src/lib/local-models.json som
// reservekopi. Alle tall kommer derfra. Her står bare de norske beskrivelsene,
// uten tall. Mangler en modell beskrivelse, viser siden rollen fra manifestet
// (engelsk) med en synlig merknad.
const LOCAL_MODEL_TEXT: Record<string, string> = {
  "qwen3.6-35b-a3b-optiq":
    "Rask og forutsigbar, og svarer på sekunder. Det eneste hovedagenten kan sende hit uten forbehold, er en mekanisk endring over flere filer.",
  "qwen3.8-27b-optiq-4bit":
    "Mye tregere enn standard. Bruker 8 bit på de mest følsomme lagene og 4 bit på resten. I siste måling nådde ingen oppgaver tidsgrensen, noe den vanlige 4-bitversjonen den erstatter gjorde.",
  "qwen3.8-27b-8bit-mlx":
    "Den tregeste. Løste litt flere oppgaver enn standard i siste måling, men bruker mange ganger så lang tid. Leser lange prompter i små steg for å bruke mindre minne, og det steget kjenner bare nyere nav-pilot til.",
};

const TASK_CLASS_LABEL: Record<string, string> = {
  "read-qa": "svar og forklaringer om kode",
  "edit-single": "endring i én fil",
  "edit-multi-mechanical": "mekanisk endring over flere filer",
  "create-file": "ny fil",
  debug: "feilsøking",
};

const kTokens = (n: number) => `${Math.round(n / 1024)}k`;
const classLabel = (id: string) => TASK_CLASS_LABEL[id] ?? id;
const nbNumber = (n: number) => n.toLocaleString("nb-NO");

function trustedClasses(m: LocalModel) {
  return Object.entries(m.classes).flatMap(([id, c]) => [
    ...(c.delegate === "trusted" ? [`${classLabel(id)} (sendt fra en skyagent)`] : []),
    ...(c.local === "trusted" ? [`${classLabel(id)} (hele økten lokalt)`] : []),
  ]);
}

function cloudClasses(m: LocalModel) {
  return Object.entries(m.classes)
    .filter(([, c]) => c.delegate !== "trusted" && c.local !== "trusted")
    .map(([id]) => classLabel(id));
}

function LocalModelText({ m }: { m: LocalModel }) {
  const text = LOCAL_MODEL_TEXT[m.id];
  if (text) return text;
  return (
    <>
      {m.role} <span className="text-xs italic">(norsk beskrivelse mangler)</span>
    </>
  );
}

async function LiveLocalModelSection() {
  const { models } = await getLocalModels();
  return <LocalModelSection models={models} />;
}

function LocalModelSection({ models }: { models: LocalModel[] }) {
  const defaultModel = models.find((m) => m.default) ?? models[0];
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
            <code className="font-mono text-xs">init</code> selv. Du trenger en Mac med Apple Silicon og{" "}
            {defaultModel.min_ram_gb} GB minne, og ledig disk til {defaultModel.weights_gb} GB vekter pluss
            Python-miljøet. Intel-Macer blir avvist, fordi MLX bare finnes for M-brikkene.
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
          <BodyShort size="small" textColor="subtle">
            <code className="font-mono text-xs">init</code> viser hva den skal laste ned, og om den trenger{" "}
            <code className="font-mono text-xs">sudo</code> for å heve minnegrensen, og spør før den begynner. Uten
            terminal nekter den, med mindre du sender med <code className="font-mono text-xs">--yes</code>.
          </BodyShort>
          <CodeBlock compact>
            {`nav-pilot alpha local init      # laster ned modellen og setter opp miljøet
nav-pilot alpha local init --yes # det samme fra et skript, uten å spørre
nav-pilot alpha local start     # starter serveren
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local models    # modellene som tilbys, og hvilken som er i bruk
nav-pilot alpha local use <key> # velg modellen serveren laster
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha decide "..." --options ja,nei --evidence fil  # typet avgjørelse
nav-pilot alpha local stop
nav-pilot alpha local restart   # stop og start i ett
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # viser hva som fjernes og hvor mye; --yes sletter, --all tar alle modellene`}
          </CodeBlock>
          <VStack id="lokal-modeller" gap="space-12">
            <LinkableHeading size="small" level="3">
              Modeller i alfa
            </LinkableHeading>
            <BodyLong size="small" textColor="subtle">
              {models.length} modeller er tilgjengelige. Én er standard, resten må du velge selv. Tabellen hentes fra{" "}
              <a href={MANIFEST_URL} style={{ textDecoration: "underline" }}>
                modellmanifestet
              </a>
              , det samme nav-pilot leser når du kjører <code className="font-mono text-xs">init</code> og{" "}
              <code className="font-mono text-xs">start</code>. Kontekst og svar er det største vinduet og det lengste
              svaret nav-pilot gir modellen.
            </BodyLong>
            <div className="overflow-x-auto">
              <Table size="small" className="w-full">
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Modell</TableHeaderCell>
                    <TableHeaderCell scope="col">Kontekst / svar</TableHeaderCell>
                    <TableHeaderCell scope="col">Minne</TableHeaderCell>
                    <TableHeaderCell scope="col">Krever nav-pilot</TableHeaderCell>
                    <TableHeaderCell scope="col">Kort sagt</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {models.map((m) => (
                    <TableRow key={m.id}>
                      <TableDataCell>
                        <VStack gap="space-2">
                          <code className="font-mono text-xs">{m.model.split("/").pop()}</code>
                          <div className="text-xs" style={{ color: m.default ? "#0f6d6a" : "#64748b" }}>
                            {m.default ? "standard" : "valgfri"}
                          </div>
                        </VStack>
                      </TableDataCell>
                      <TableDataCell className="whitespace-nowrap">
                        {kTokens(m.context)} / {kTokens(m.output)}
                      </TableDataCell>
                      <TableDataCell>
                        {m.min_ram_gb} GB, vektene tar {m.weights_gb} GB
                      </TableDataCell>
                      <TableDataCell>
                        {m.min_nav_pilot ? (
                          <code className="font-mono text-xs">≥ {m.min_nav_pilot}</code>
                        ) : (
                          "alle versjoner"
                        )}
                      </TableDataCell>
                      <TableDataCell>
                        <LocalModelText m={m} />
                      </TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
            <BodyShort size="small" textColor="subtle">
              Står det en versjon under «Krever nav-pilot», skjuler eldre nav-pilot modellen. Peker{" "}
              <code className="font-mono text-xs">local_model</code> på den, faller nav-pilot tilbake til
              standardmodellen, og <code className="font-mono text-xs">init</code>,{" "}
              <code className="font-mono text-xs">start</code> og <code className="font-mono text-xs">status</code> sier
              hvilken versjon du trenger. Oppdater med <code className="font-mono text-xs">nav-pilot upgrade</code>.
            </BodyShort>
            {defaultModel?.temperature != null && (
              <BodyShort size="small" textColor="subtle">
                Standardmodellen kjører med temperatur {nbNumber(defaultModel.temperature)}
                {defaultModel.top_p != null && <> og top_p {nbNumber(defaultModel.top_p)}</>}, verdiene den ble målt
                med.
              </BodyShort>
            )}
            <BodyShort size="small" textColor="subtle">
              Én kjøring er ikke en måling. Alle kjøringene står i{" "}
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
            <code className="font-mono text-xs">nav-pilot alpha local models</code> viser de lokale modellene:
            størrelse, kontekst, hva de er anbefalt til, om de er lastet ned eller kjører, og hvilken serveren laster
            (merket <code className="font-mono text-xs">*</code>).{" "}
            <code className="font-mono text-xs">nav-pilot alpha local use &lt;key&gt;</code> velger modell og skriver
            den til <code className="font-mono text-xs">local_model</code>. Den laster ikke ned og starter ikke noe
            selv. <code className="font-mono text-xs">model</code> er modellen økten selv kjører på, og settes for seg.
            Listen oppdateres når du kjører <code className="font-mono text-xs">init</code> eller{" "}
            <code className="font-mono text-xs">start</code>, ikke ved hver kommando. Et nettverkskall der ville lagt
            seg foran alt annet nav-pilot gjør.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot alpha local models
nav-pilot alpha local use qwen3.8-27b-optiq-4bit
nav-pilot alpha local init      # laster ned vektene hvis de mangler, og starter
nav-pilot alpha local restart   # hvis serveren allerede kjører en annen modell`}
          </CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Qwen 3.6 er standard fordi den er rask og forutsigbar, ikke fordi den løser mest. Bytter du, må vektene til
            den nye modellen lastes ned én gang. Størrelsen står i tabellen over.{" "}
            <code className="font-mono text-xs">purge</code> fjerner Python-miljøet, den valgte modellen og modeller
            manifestet har erstattet. Andre modeller du har lastet ned, blir liggende, og listen sier hvilke.{" "}
            <code className="font-mono text-xs">purge --all</code> fjerner alle. Ingenting slettes før du legger til{" "}
            <code className="font-mono text-xs">--yes</code>.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            <code className="font-mono text-xs">nav-pilot alpha local status</code> viser hvilken modell som er valgt,
            og om den er valgt med <code className="font-mono text-xs">local_model</code> eller er standard. Kjører
            serveren en annen modell, sier status det og gir deg kommandoen for omstart. Krever modellen du har valgt en
            nyere nav-pilot, sier status at den har falt tilbake til standard, og hvorfor.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Har du valgt en annen modell enn standard, sier <code className="font-mono text-xs">start</code>,{" "}
            <code className="font-mono text-xs">status</code> og <code className="font-mono text-xs">models</code> én
            gang hva standardmodellen er anbefalt til, og hvordan du bytter. Du ser beskjeden igjen bare hvis manifestet
            endrer den, og aldri fra <code className="font-mono text-xs">alpha decide</code>, en vanlig launch eller når
            utskriften går til et skript. Er modellen i <code className="font-mono text-xs">local_model</code> fjernet
            og erstattet av en annen, fortsetter nav-pilot med den gamle så lenge bare vektene til den gamle ligger på
            maskinen, og bytter når erstatningen er lastet ned. Konfigurasjonen din endres ikke;{" "}
            <code className="font-mono text-xs">nav-pilot alpha local use &lt;key&gt;</code> gjør valget eksplisitt.
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
              omstart. Er standardgrensen i macOS høy nok, som på en maskin med mye minne, skjer ingenting. Er den for
              lav, spør <code className="font-mono text-xs">start</code> før den hever den igjen. Uten terminal skriver
              den kommandoen du må kjøre i stedet, og en automatisk start fra en vanlig{" "}
              <code className="font-mono text-xs">nav-pilot</code> ber aldri om passord.
            </BodyLong>
          </Box>
          <BodyLong size="small" textColor="subtle">
            Klienten din avgjør hva du får. <code className="font-mono text-xs">nav-pilot config get client</code> sier
            hvilken du kjører.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Utsending til en lokal underagent krever <strong>opencode</strong>. Der blir bakkemodellen en underagent som
            heter <code className="font-mono text-xs">local-worker</code>, og som hovedagenten i skyen sender avgrensede
            oppgaver til. Hovedagenten bestemmer fortsatt alt, og gjør selv det den vurderer at bakkemodellen ikke
            klarer.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Under <strong>Copilot CLI</strong> finnes ingen slik underagent i dag. Copilot CLI er standardklienten, så
            dette gjelder deg med mindre du har byttet. Valget der er hele økten på den lokale modellen eller ingenting
            lokalt, fordi Copilot CLI leser modelleverandøren fra en miljøvariabel for hele prosessen, så én leverandør
            betjener hele økten. Vi har verifisert det mot Copilot CLI 1.0.83-3. Runtimen under Copilot CLI kan ha flere
            leverandører i én økt, men Copilot CLI lar ikke en agent velge sin egen ennå, og det er ikke dokumentert. Vi
            tester om det kan tas i bruk (
            <a href="https://github.com/github/copilot-cli/issues/4703" style={{ textDecoration: "underline" }}>
              github/copilot-cli#4703
            </a>
            ). Vil du ha utsending nå, bytt med{" "}
            <code className="font-mono text-xs">nav-pilot config set client opencode</code>.
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
          <BodyShort size="small" textColor="subtle">
            Manifestet sier for hver modell hvilke oppgavetyper hovedagenten kan sende til den. En oppgavetype blir
            godkjent først når modellen har holdt kvalitetsgrensen mot skyen over nok kjøringer og ulike oppgaver. Det
            som ikke er godkjent, gjør hovedagenten selv.
          </BodyShort>
          <div className="overflow-x-auto">
            <Table size="small" className="w-full">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col">Modell</TableHeaderCell>
                  <TableHeaderCell scope="col">Godkjent</TableHeaderCell>
                  <TableHeaderCell scope="col">Blir i skyen</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {models.map((m) => {
                  const trusted = trustedClasses(m);
                  return (
                    <TableRow key={m.id}>
                      <TableDataCell>
                        <code className="font-mono text-xs">{m.model.split("/").pop()}</code>
                      </TableDataCell>
                      <TableDataCell>{trusted.length ? trusted.join(", ") : "ingen oppgavetyper ennå"}</TableDataCell>
                      <TableDataCell>{cloudClasses(m).join(", ")}</TableDataCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
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

        <VStack id="lokal-decide" gap="space-12">
          <LinkableHeading size="small" level="3">
            Typede avgjørelser med <code className="font-mono">alpha decide</code>
          </LinkableHeading>
          <BodyShort size="small" textColor="subtle">
            <code className="font-mono text-xs">nav-pilot alpha decide</code> stiller bakkemodellen ett
            flervalgsspørsmål og svarer med en sannsynlighet for hvert alternativ, ikke med fritekst. Modellen genererer
            ett token, så et varmt svar tar under ett sekund. Spørsmålet og grunnlaget forlater ikke maskinen.
          </BodyShort>
          <CodeBlock compact>
            {`nav-pilot alpha decide \\
  "Does the commit message explain why?" \\
  --options yes,no --evidence msg.txt
# {"choice":"yes","p":{"yes":0.93,"no":0.07},…}`}
          </CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Bruk den til vurderinger en regel ikke kan gjøre. Om en commit-melding følger Conventional Commits, avgjør
            et regulært uttrykk. Om meldingen forklarer hvorfor, kan bare en modell vurdere. Med{" "}
            <code className="font-mono text-xs">--threshold</code> og{" "}
            <code className="font-mono text-xs">--expect</code> blir exit-koden 0 eller 1, så den passer i hooks og
            skript.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Mål spørsmålet før du bygger på det. <code className="font-mono text-xs">--eval cases.jsonl</code> kjører
            eksempler du kjenner fasiten på, og viser treffsikkerhet, en forvekslingsmatrise, gjennomsnittlig
            sannsynlighet for riktige og gale svar, og svartid. Serveren må kjøre, for{" "}
            <code className="font-mono text-xs">decide</code> starter den ikke selv. Alle valg står i{" "}
            <code className="font-mono text-xs">nav-pilot alpha decide --help</code>.
          </BodyLong>
        </VStack>

        <VStack id="lokal-decide-oppskrifter" gap="space-12">
          <LinkableHeading size="small" level="3">
            Oppskrifter for <code className="font-mono">alpha decide</code>
          </LinkableHeading>
          <BodyShort size="small" textColor="subtle">
            Commit-hooken og etikettforslaget er målt. PR-sjekken er målt og svak, og loggsorteringen er ikke målt ennå.
            Alle advarer eller foreslår, ingen stopper noe.
          </BodyShort>
          <BodyLong size="small" textColor="subtle">
            Tre råd når du skriver egne spørsmål:
          </BodyLong>
          <VStack
            as="ul"
            gap="space-4"
            className="text-sm list-disc"
            style={{ color: "#64748b", paddingInlineStart: "var(--ax-space-20)" }}
          >
            <li>Still spørsmålet positivt: «Forklarer meldingen hvorfor?», ikke «Mangler meldingen en forklaring?».</li>
            <li>
              Sett <code className="font-mono text-xs">yes</code> først i alternativene.
            </li>
            <li>
              Kjør <code className="font-mono text-xs">--eval</code> på nøyaktig den ordlyden du skal bruke.
            </li>
          </VStack>
          <BodyLong size="small" textColor="subtle">
            Grunnen: «ja»-svarene er stabile, men «nei»-svarene vipper mot «teksten er grei» når alternativene bytter
            plass eller spørsmålet snus. Standardmodellen svarte riktig på 85 % av spørsmålene i opprinnelig form og 58
            % når de var snudd.{" "}
            <a
              href="https://github.com/navikt/mlx-workspace/blob/main/bench/decide-layout-results.md"
              className="text-blue-600 hover:underline"
            >
              Se målingen
            </a>
            .
          </BodyLong>

          <HStack gap="space-8" align="center">
            <Label size="small">Commit-meldingen forklarer hvorfor</Label>
            <Tag size="small" variant="success">
              Målt
            </Tag>
          </HStack>
          <BodyLong size="small" textColor="subtle">
            Lagre skriptet som <code className="font-mono text-xs">scripts/commit-explains-why.sh</code> i repoet. Det
            advarer når meldingen bare sier hva diffen viser, og slipper alltid commiten gjennom. Uten nav-pilot på
            maskinen gjør det ingenting.
          </BodyLong>
          <CodeBlock compact>{COMMIT_EXPLAINS_WHY_HOOK}</CodeBlock>
          <CodeBlock compact>{`chmod +x scripts/commit-explains-why.sh`}</CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Med <code className="font-mono text-xs">pre-commit</code> legger du det inn som en lokal hook i{" "}
            <code className="font-mono text-xs">.pre-commit-config.yaml</code>:
          </BodyLong>
          <CodeBlock compact>
            {`repos:
  - repo: local
    hooks:
      - id: commit-explains-why
        name: commit-meldingen forklarer hvorfor
        entry: scripts/commit-explains-why.sh
        language: script
        stages: [commit-msg]`}
          </CodeBlock>
          <CodeBlock compact>{`pre-commit install --hook-type commit-msg`}</CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Med Lefthook går det i <code className="font-mono text-xs">lefthook.yml</code>.{" "}
            <code className="font-mono text-xs">{"{1}"}</code> er fila git lagrer meldingen i:
          </BodyLong>
          <CodeBlock compact>
            {`commit-msg:
  commands:
    explains-why:
      run: scripts/commit-explains-why.sh {1}`}
          </CodeBlock>

          <Label size="small">Mål ditt eget spørsmål</Label>
          <BodyLong size="small" textColor="subtle">
            Vil du stille modellen et annet spørsmål, mål det først. Lag en JSONL-fil med eksempler fra ditt eget repo
            der du vet svaret, ett per linje. Ta med omtrent like mange av hvert svar:
          </BodyLong>
          <CodeBlock compact>{DECIDE_EVAL_CASES}</CodeBlock>
          <CodeBlock compact>{`nav-pilot alpha decide --eval cases.jsonl`}</CodeBlock>
          <BodyLong size="small" textColor="subtle">
            Er modellen like sikker når den tar feil som når den har rett, hjelper ingen terskel. Da bør spørsmålet ikke
            inn i en hook.
          </BodyLong>

          <Label size="small">Tekst andre har skrevet</Label>
          <BodyLong size="small" textColor="subtle">
            PR-beskrivelser, issues og logger er skrevet av andre, og teksten kan inneholde instruksjoner til modellen
            (prompt injection). La sjekker på slik tekst bare advare eller foreslå, aldri stoppe noe. Tallene under er
            fra{" "}
            <a
              href="https://github.com/navikt/mlx-workspace/blob/main/bench/decide-sets-20260925-225356.md"
              className="text-blue-600 hover:underline"
            >
              målingene 25. september
            </a>{" "}
            med standardmodellen.
          </BodyLong>

          <HStack gap="space-8" align="center">
            <Label size="small">Foreslå en etikett på et issue</Label>
            <Tag size="small" variant="success">
              Målt
            </Tag>
          </HStack>
          <BodyLong size="small" textColor="subtle">
            Modellen valgte riktig mellom bug, feature og question på 95 av 105 issues (90 %). Når den bare fikk svare
            ved p ≥ 0,9, svarte den på omtrent to tredjedeler av issuene og hadde rett alle 71 gangene. Behold filteret:
            under 0,9 gir kommandoen ingen etikett, og da setter du den selv.
          </BodyLong>
          <CodeBlock compact>{DECIDE_ISSUE_LABEL}</CodeBlock>

          <HStack gap="space-8" align="center">
            <Label size="small">Forklarer PR-beskrivelsen hvorfor?</Label>
            <Tag size="small" variant="warning">
              Målt: svak – bruk som et hint, ikke som sperre
            </Tag>
          </HStack>
          <BodyLong size="small" textColor="subtle">
            Den flagget ingen av de 24 beskrivelsene som forklarer hvorfor, men fant bare 3 av 12 der grunnen var tatt
            ut. Et «no» er verdt å se på. Et «yes» betyr lite.
          </BodyLong>
          <CodeBlock compact>{DECIDE_PR_DESCRIPTION}</CodeBlock>

          <HStack gap="space-8" align="center">
            <Label size="small">Sorter en logg før du leser den selv</Label>
            <Tag size="small" variant="neutral">
              Ikke målt
            </Tag>
          </HStack>
          <BodyShort size="small" textColor="subtle">
            <code className="font-mono text-xs">tail -c 30000</code> holder grunnlaget under grensen på 32 KiB.
          </BodyShort>
          <CodeBlock compact>{DECIDE_LOG_TRIAGE}</CodeBlock>
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
# står det hung: nav-pilot alpha local restart`}
          </CodeBlock>
          <BodyLong size="small" textColor="subtle">
            nav-pilot slipper gjennom én forespørsel om gangen, så flere oppgaver står i kø framfor å kjøre parallelt.
            Det er med vilje: serveren selv tar imot samtidige forespørsler og henger seg opp på dem, så ikke kall den
            direkte utenom nav-pilot.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            nav-pilot avslutter en tur hvis modellen gjør det samme verktøykallet fire ganger på rad og får samme
            resultat hver gang, eller åtte ganger på rad uansett resultat, og sier fra i økten. Det er en vakt mot at
            modellen setter seg fast, ikke en feil i koden din. Grensene er standardverdier, og du endrer dem med{" "}
            <code className="font-mono text-xs">local_loop_guard</code>.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Går modellen tom for minne, for eksempel på en lang prompt, dør tråden som genererer svar. Serveren
            avslutter seg da selv i stedet for å henge, og neste økt sier{" "}
            <code className="font-mono text-xs">generation thread died, most likely out of memory</code>, med stien til
            tracebacken. Start den igjen med <code className="font-mono text-xs">nav-pilot alpha local restart</code>.
            Skjer det igjen, velg en modell med kortere kontekst.
          </BodyLong>
          <BodyLong size="small" textColor="subtle">
            Starter du serveren på nytt midt i en økt, må økten startes på nytt også. Den gamle er bundet til serveren
            som forsvant.
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
            nav-pilot sjekker automatisk om det finnes en nyere versjon ved oppstart. Du kan oppgradere på tre måter:
          </BodyLong>
          <div className="mt-4 space-y-3">
            {[
              { label: "Selvoppdatering", cmd: "nav-pilot upgrade" },
              { label: "Via Homebrew (macOS)", cmd: "brew update && brew upgrade nav-pilot" },
              {
                label: "Via apt (Debian, Ubuntu)",
                cmd: "sudo apt update && sudo apt install --only-upgrade nav-pilot",
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
          <BodyLong size="small" className="mt-3" style={{ color: "#64748b" }}>
            Har du installert med apt, bruk <code className="font-mono text-xs">apt</code> og ikke{" "}
            <code className="font-mono text-xs">nav-pilot upgrade</code>. Selvoppdateringen kjenner igjen en
            Homebrew-installasjon og lar den være, men ikke en dpkg-installasjon, så den ville byttet ut binæren uten at
            dpkg vet om det.
          </BodyLong>
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
            <code className="font-mono text-xs">install</code> spør hvor den skal installere, enten i repoet (
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
                Installer agentpakka med forhåndsvisning
              </Label>
              <div className="space-y-3">
                {[
                  { label: "Se hva som installeres", cmd: "nav-pilot install --dry-run nav-pilot" },
                  { label: "Installer", cmd: "nav-pilot install nav-pilot" },
                  { label: "Installer i annet repo", cmd: "nav-pilot install --target /path/to/repo nav-pilot" },
                  {
                    label: "Overskriv lokalt endrede filer",
                    cmd: "nav-pilot install --force nav-pilot",
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
                  { label: "Installer i CI med JSON-resultat", cmd: "nav-pilot install nav-pilot --repo --yes --json" },
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
                  sync-sjekk feilet, eller install uten terminal og uten{" "}
                  <code className="font-mono text-xs">--yes</code> (den skriver ingenting og sier hva den ville ha
                  skrevet). Når nav-pilot starter en klient, gir den videre klientens exit-kode. Kunne den ikke starte
                  klienten i det hele tatt, blir koden 1, og en klient som ble drept av et signal gir 128 pluss
                  signalnummeret, slik et shell gjør. <code className="font-mono text-xs">--json</code> fungerer på
                  install, add, status, sync, list og export. <code className="font-mono text-xs">sync --json</code> gir
                  ett dokument med én oppføring per scope, og et scope som feilet har et{" "}
                  <code className="font-mono text-xs">error</code>-felt. Når ikke sync fram til GitHubs release-API,
                  blir den committede pinnen stående, og sync avslutter med 2.
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
                desc: "Alltid aktive, med Nav-mønstre, kodestandarder og anti-patterns. Hver Copilot-sesjon er Nav-bevisst automatisk.",
                Icon: DocPencilIcon,
                bg: "#eff6ff",
                accent: "#3b82f6",
              },
              {
                label: "@nav-pilot agent",
                desc: "Én inngangsport som ruter til riktig fase og skill. Delegerer til @kafka, @security-champion og laster $nav-auth, $nais.",
                Icon: PersonGroupIcon,
                bg: "#f5f3ff",
                accent: "#7c3aed",
              },
              {
                label: "Skills",
                desc: "Byggeklosser for intervju, plan, review og feilsøking. Brukes via @nav-pilot eller alene.",
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
                desc: "Institusjonell kunnskap er varig, mens orkestrering blir standardvare.",
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
