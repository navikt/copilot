import { BodyLong, BodyShort, Box, HGrid, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { PipelineFlow } from "@/components/pipeline-flow";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Planlegging",
  description:
    "Hvordan nav-pilot planlegger i fire faser med stopp mellom hver, hvilke skills som driver fasene, og hvorfor du skriver kjernelogikken selv.",
};

const TOC: TocItem[] = [
  {
    id: "planleggingspipelinen",
    label: "Planleggingen",
    children: [
      { id: "fire-faser", label: "De fire fasene" },
      { id: "skills-i-detalj", label: "Skillene" },
    ],
  },
  {
    id: "kompetansebevaring",
    label: "Kompetansebevaring",
    children: [
      { id: "gronn-rod-sone", label: "Grønn og rød sone" },
      { id: "demo-i-praksis", label: "Slik ser det ut" },
    ],
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
    purpose: "Går fra beslutningstrær til Nais-manifest, CI/CD og prosjektstruktur",
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
    purpose: "Gjennomgår fra tre perspektiver og skriver en ADR",
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

export default function Planlegging() {
  return (
    <DocPage
      label="Forklaring"
      title="Planlegging"
      description="nav-pilot foreslår, du godkjenner. Slik er planleggingen bygget opp, og hvorfor."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="planleggingspipelinen" size="medium" level="2">
            Planleggingen
          </LinkableHeading>
          <BodyLong>
            Agenten <code className={code}>@nav-pilot</code> planlegger i fire faser, med et stopp mellom hver. Du
            bestemmer når den går videre. Agenten er tynn: den velger fase og henter en skill. Kunnskapen ligger i
            skillene, i beslutningstrær, sjekklister og diagnosetrær.
          </BodyLong>
          <LinkableHeading id="fire-faser" size="small" level="3">
            De fire fasene
          </LinkableHeading>
          <PipelineFlow />
          <div id="planning-skills">
            <VStack gap="space-16">
              <LinkableHeading id="skills-i-detalj" size="small" level="3">
                Skillene
              </LinkableHeading>
              <BodyLong>Agentpakka har fire skills for planlegging. Du kan også bruke dem uten agenten.</BodyLong>
              <div className="overflow-x-auto">
                <Table size="small">
                  <TableHeader>
                    <TableRow>
                      <TableHeaderCell scope="col">Skill</TableHeaderCell>
                      <TableHeaderCell scope="col">Hva den gjør</TableHeaderCell>
                      <TableHeaderCell scope="col">Dekker</TableHeaderCell>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {PLANNING_SKILLS.map((s) => (
                      <TableRow key={s.name}>
                        <TableDataCell>
                          <code className={`${code} whitespace-nowrap`}>{s.name}</code>
                        </TableDataCell>
                        <TableDataCell>{s.purpose}</TableDataCell>
                        <TableDataCell>
                          <VStack gap="space-4">
                            <span>{s.details.map((d) => d.split("—")[0].trim()).join(", ")}</span>
                            <BodyShort size="small" textColor="subtle">
                              {s.refs}
                            </BodyShort>
                          </VStack>
                        </TableDataCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </div>
            </VStack>
          </div>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="kompetansebevaring" size="medium" level="2">
            Kompetansebevaring
          </LinkableHeading>
          <BodyLong>
            Utviklere som lar modellen skrive alt, forstår sin egen kode dårligere. I Anthropics randomiserte studie
            (2026) fikk de som delegerte blindt 35–39 % på kodeforståelse, mot 86 % for dem som stilte spørsmål etter at
            koden var generert. Navs egen langtidsstudie (Stray mfl., HICSS-59 2026) viser det samme mønsteret.
          </BodyLong>
          <BodyLong>
            Samtidig viser studien fra MIT og Microsoft (2025, rundt 5 000 utviklere) at KI-hjelp gir mest på repetitive
            oppgaver. På oppgaver som krever at du forstår domenet, forsvinner gevinsten, og den kan bli negativ.
          </BodyLong>
          <LinkableHeading id="gronn-rod-sone" size="small" level="3">
            Grønn og rød sone
          </LinkableHeading>
          <BodyLong>
            nav-pilot deler derfor oppgavene i to. I fase 2 (Plan) merker den hver del som grønn eller rød sone. I fase
            4 leverer den full kode for det grønne. For det røde får du testskjeletter og stubber med{" "}
            <code className={code}>TODO</code>, og skriver kjernelogikken selv.
          </BodyLong>
          <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
            <Box background="success-soft" borderRadius="8" padding="space-16">
              <VStack gap="space-8">
                <Label size="small">Grønn sone: modellen skriver koden</Label>
                <Bullets>
                  <li>Boilerplate og repetitiv kode (Nais-manifest, CRUD)</li>
                  <li>Teknologi du kan fra før</li>
                  <li>Konfigurasjon og infrastruktur</li>
                  <li>Refaktorering med kjent mål</li>
                  <li>Testdata og fixtures</li>
                </Bullets>
              </VStack>
            </Box>
            <Box background="danger-soft" borderRadius="8" padding="space-16">
              <VStack gap="space-8">
                <Label size="small">Rød sone: du skriver koden, modellen gir stubber</Label>
                <Bullets>
                  <li>Feilsøking</li>
                  <li>Nye konsepter og ukjent teknologi</li>
                  <li>Kjernelogikk og forretningsregler</li>
                  <li>Sikkerhetskritisk kode</li>
                  <li>Arkitekturbeslutninger</li>
                </Bullets>
              </VStack>
            </Box>
          </HGrid>
          <LinkableHeading id="demo-i-praksis" size="small" level="3">
            Slik ser det ut
          </LinkableHeading>
          <BodyLong>
            Her planlegger nav-pilot en ny beregningsregel for sykepenger (§ 8-20). Koden rundt regelen er grønn sone,
            selve regelverket er rødt.
          </BodyLong>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src="/demos/nav-pilot-red-zone.gif"
            alt="nav-pilot merker kjernelogikken som rød sone og leverer stubber med TODO"
            className="rounded-lg w-full"
            style={{ border: "1px solid var(--ax-border-neutral-subtle)" }}
          />
          <BodyShort size="small" textColor="subtle">
            Kilder:{" "}
            <a href="https://www.anthropic.com/research/AI-assistance-coding-skills" className={linkClass}>
              Anthropic
            </a>
            ,{" "}
            <a href="https://metr.org/blog/2025-07-10-early-2025-ai-experienced-os-dev-study/" className={linkClass}>
              METR
            </a>{" "}
            og{" "}
            <a href="https://arxiv.org/abs/2509.20353" className={linkClass}>
              studien fra Nav IT
            </a>
            .
          </BodyShort>
        </VStack>
      </section>
    </DocPage>
  );
}
