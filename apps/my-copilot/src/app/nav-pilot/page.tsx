import type { Metadata } from "next";
import React from "react";
import { Box, VStack, HGrid, Heading, CopyButton } from "@navikt/ds-react";
import NextLink from "next/link";
import {
  TerminalIcon,
  PaletteIcon,
  BranchingIcon,
  CloudIcon,
  CheckmarkCircleIcon,
  XMarkOctagonIcon,
  MagnifyingGlassIcon,
  TasklistIcon,
  ShieldLockIcon,
  RocketIcon,
  ArrowsCirclepathIcon,
  WrenchIcon,
  SparklesIcon,
  FileTextIcon,
  FileSearchIcon,
  ChatIcon,
} from "@navikt/aksel-icons";
import {
  KotlinLogo,
  TypeScriptLogo,
  ReactLogo,
  PostgreSQLLogo,
  KafkaLogo,
  NextjsLogo,
  GoLogo,
  KubernetesLogo,
  TechLogoRow,
} from "@/components/tech-logos";

export const metadata: Metadata = {
  title: "nav-pilot — Copilot i Nav",
  description:
    "nav-pilot gir GitHub Copilot Navs institusjonelle kunnskap — fra Nais-manifester til TokenX, rett i editoren din.",
};

/* ---------- Data ---------- */

const INSTALL_COMMAND = "curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash";

const COLLECTIONS = [
  {
    id: "kotlin-backend",
    title: "kotlin-backend",
    description: "Kotlin/Ktor, Spring Boot, Kafka og Flyway",
    agents: 6,
    skills: 10,
    highlights: ["Kafka & TokenX", "Flyway", "Modernisering"],
    Icon: TerminalIcon,
    logos: [KotlinLogo, KafkaLogo, PostgreSQLLogo],
    codePreview: `River(rapidsConnection).apply {
  validate { it.demandValue(
    "@event_name", "vedtak"
  )}
}`,
  },
  {
    id: "nextjs-frontend",
    title: "nextjs-frontend",
    description: "Next.js, React, Aksel og Playwright",
    agents: 4,
    skills: 7,
    highlights: ["Aksel spacing", "Playwright", "Refaktorering"],
    Icon: PaletteIcon,
    logos: [NextjsLogo, ReactLogo, TypeScriptLogo],
    codePreview: `<Box padding="space-24">
  <HGrid columns={{ xs: 1, md: 2 }}>
    <Heading level="1" size="large">`,
  },
  {
    id: "fullstack",
    title: "fullstack",
    description: "Backend + frontend — komplett for din tjeneste",
    agents: 10,
    skills: 13,
    highlights: ["Komplett pakke", "BFF-mønster", "Migrering"],
    Icon: BranchingIcon,
    logos: [KotlinLogo, ReactLogo, TypeScriptLogo, PostgreSQLLogo],
    codePreview: `accessPolicy:
  inbound:
    rules:
      - application: frontend`,
  },
  {
    id: "platform",
    title: "platform",
    description: "Plattform, observerbarhet, DevOps og sikkerhet",
    agents: 4,
    skills: 7,
    highlights: ["Observerbarhet", "Sikkerhet", "Infrastruktur"],
    Icon: CloudIcon,
    logos: [KubernetesLogo, GoLogo],
    codePreview: `observability:
  autoInstrumentation:
    enabled: true
    runtime: java`,
  },
];

const PIPELINE_STEPS = [
  {
    title: "Intervju",
    subtitle: "Dypdykk-intervju",
    description: "Finner blinde flekker — dataklassifisering, auth-type, PII-risiko og avhengigheter.",
    Icon: MagnifyingGlassIcon,
    color: "#a78bfa",
  },
  {
    title: "Plan",
    subtitle: "Beslutningstrær",
    description: "Velger arkitektur, teststrategi og leveransedokumenter — nybygg, refaktorering eller migrering.",
    Icon: TasklistIcon,
    color: "#60a5fa",
  },
  {
    title: "Review",
    subtitle: "Arkitektur-review",
    description: "Sjekker Nav-antimønstre, endringspåvirkning, testdekning og teknisk gjeld.",
    Icon: ShieldLockIcon,
    color: "#2dd4bf",
  },
  {
    title: "Lever",
    subtitle: "Kode + dokumentasjon",
    description: "Produksjonsklar kode, tester, endringsdokument, utrullingsplan og verifiseringssjekkliste.",
    Icon: RocketIcon,
    color: "#fb923c",
  },
];

const COMPARISONS = [
  { feature: "Auth", generic: "«Prøver JWT …»", navPilot: "TokenX / ID-porten" },
  { feature: "Refaktorering", generic: "Generiske tips", navPilot: "Strangler fig + feature toggles" },
  { feature: "Testing", generic: "«Skriv unit-tester»", navPilot: "Teststrategi per lag + karakteriseringstester" },
  { feature: "Dokumentasjon", generic: "Generisk README", navPilot: "Endringsdokument + utrullingsplan + runbook" },
  { feature: "Sikkerhet", generic: "Generiske råd", navPilot: "PII-blokkering + teknisk gjeld-vurdering" },
  { feature: "Plattform", generic: "«Hva er Nais?»", navPilot: "nais.yaml + accessPolicy + observerbarhet" },
  { feature: "Migrering", generic: "Ingen kontekst", navPilot: "Konsekvensanalyse + tre-fase-migrering" },
];

/* ---------- Page ---------- */

export default function NavPilotPage() {
  return (
    <main>
      <HeroSection />
      <UseCasesSection />
      <CollectionsSection />
      <PipelineSection />
      <ComparisonSection />
      <TestimonialsSection />
      <TechStackStrip />
      <GetStartedSection />
      <FooterTagline />
    </main>
  );
}

/* ---------- Hero ---------- */

function HeroSection() {
  return (
    <section
      className="dark-section"
      style={{
        background: "linear-gradient(165deg, #0c0e1a 0%, #0f172a 40%, #162044 70%, #1a1040 100%)",
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
            <div className="flex items-center justify-center gap-3 hero-animate">
              <Heading size="xlarge" level="1">
                Slutt å lære opp Copilot.
              </Heading>
              <span
                className="uppercase tracking-wide font-semibold rounded-full"
                style={{ background: "#dbeafe", color: "#1e3a5f", fontSize: "0.75rem", padding: "2px 10px" }}
              >
                Beta
              </span>
            </div>
            <p
              className="max-w-2xl mx-auto hero-animate-d1"
              style={{ color: "#94a3b8", fontSize: "1.125rem", lineHeight: 1.7, marginBlock: 0, textAlign: "center" }}
            >
              Navs institusjonelle kunnskap — arkitektur, modernisering og beste praksis — direkte i editoren din.
            </p>
          </VStack>

          {/* Side-by-side code diff */}
          <div
            className="grid grid-cols-1 md:grid-cols-2 gap-0 max-w-4xl mx-auto w-full hero-animate-d2 rounded-xl overflow-hidden"
            style={{ border: "1px solid rgba(255,255,255,0.08)" }}
          >
            {/* Generic side */}
            <div style={{ background: "#1a1a2e" }}>
              <div
                className="flex items-center gap-2 px-4 py-2.5"
                style={{ borderBottom: "1px solid rgba(255,255,255,0.06)" }}
              >
                <div className="flex gap-1.5">
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#ff5f57" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#febc2e" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#28c840" }} />
                </div>
                <span className="font-mono ml-2" style={{ color: "#6b7280", fontSize: "0.75rem" }}>
                  Generic Copilot
                </span>
              </div>
              <pre
                className="p-5 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: "0.8rem", color: "#9ca3af" }}
              >
                {`// Refaktorer auth-laget
fun authenticate(token: String)
  = JWT.decode(token)     `}
                <span style={{ color: "#f87171" }}>❌ Feil auth</span>
                {`

val pool = HikariConfig().apply {
  maximumPoolSize = 10    `}
                <span style={{ color: "#f87171" }}>❌ For stor pool</span>
                {`
}
// ❌ Ingen migreringsplan
// ❌ Kjenner ikke TokenX`}
              </pre>
            </div>

            {/* nav-pilot side */}
            <div style={{ background: "#0f172a", borderLeft: "1px solid rgba(96,165,250,0.2)" }}>
              <div
                className="flex items-center gap-2 px-4 py-2.5"
                style={{ borderBottom: "1px solid rgba(96,165,250,0.15)" }}
              >
                <div className="flex gap-1.5">
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#ff5f57" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#febc2e" }} />
                  <div className="w-2.5 h-2.5 rounded-full" style={{ background: "#28c840" }} />
                </div>
                <span className="font-mono ml-2" style={{ color: "#60a5fa", fontSize: "0.75rem" }}>
                  @nav-pilot
                </span>
              </div>
              <pre
                className="p-5 font-mono leading-relaxed overflow-x-auto"
                style={{ margin: 0, fontSize: "0.8rem", color: "#93c5fd" }}
              >
                {`// TokenX token exchange   `}
                <span style={{ color: "#4ade80" }}>✅ Nav-auth</span>
                {`
val token = tokenXClient
  .exchange(subjectToken)

val pool = HikariConfig().apply {
  maximumPoolSize = 3     `}
                <span style={{ color: "#4ade80" }}>✅ Nav-standard</span>
                {`
}
// ✅ Flyway-migrering V3__
// ✅ Strangler fig-plan`}
              </pre>
            </div>
          </div>

          {/* CTAs */}
          <div className="flex flex-col items-center gap-4 hero-animate-d2">
            <div className="flex flex-wrap gap-3 justify-center">
              <NextLink
                href="/nav-pilot/docs"
                className="inline-flex items-center gap-2 px-6 py-3 rounded-lg font-medium no-underline transition-all"
                style={{
                  background: "linear-gradient(135deg, #3b82f6, #6366f1)",
                  color: "white",
                  fontSize: "0.9rem",
                }}
              >
                Dokumentasjon →
              </NextLink>
            </div>
            <div
              className="rounded-lg px-4 py-2.5 flex items-center gap-3 max-w-full overflow-x-auto"
              style={{
                background: "rgba(255,255,255,0.04)",
                border: "1px solid rgba(255,255,255,0.08)",
              }}
            >
              <code
                className="font-mono whitespace-nowrap"
                style={{ fontSize: "0.8rem", color: "rgba(255,255,255,0.7)" }}
              >
                {INSTALL_COMMAND}
              </code>
              <CopyButton copyText={INSTALL_COMMAND} size="xsmall" style={{ color: "white" }} />
            </div>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Use Cases ---------- */

const USE_CASES = [
  {
    Icon: ArrowsCirclepathIcon,
    color: "#a78bfa",
    title: "Moderniser",
    description: "Strangler fig, feature toggles, tre-fase-datamigrering — nav-pilot kjenner mønstrene.",
  },
  {
    Icon: WrenchIcon,
    color: "#60a5fa",
    title: "Refaktorer",
    description: "Bytt auth-lag, optimaliser databasespørringer, oppdater avhengigheter trygt.",
  },
  {
    Icon: FileSearchIcon,
    color: "#2dd4bf",
    title: "Test trygt",
    description: "Teststrategi per komponent, karakteriseringstester for brownfield, konsekvensanalyse før endring.",
  },
  {
    Icon: SparklesIcon,
    color: "#fb923c",
    title: "Bygg nytt",
    description: "Fra idé til produksjonsklar tjeneste med Nais-manifest, auth og CI/CD.",
  },
  {
    Icon: FileTextIcon,
    color: "#f472b6",
    title: "Dokumenter",
    description: "Endringsdokument, utrullingsplan, runbook og post-deploy-verifisering — alt i ett.",
  },
  {
    Icon: ShieldLockIcon,
    color: "#34d399",
    title: "Sikre",
    description: "PII-sjekk, tilgangsstyring, sikkerhetsreview og teknisk gjeld-vurdering.",
  },
];

function UseCasesSection() {
  return (
    <section className="dark-section" style={{ background: "#0f172a", color: "white" }}>
      <Box
        paddingBlock={{ xs: "space-16", md: "space-32" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3" style={{ color: "white" }}>
              Ikke bare for nye prosjekter
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#94a3b8", marginBlock: 0, textAlign: "center" }}>
              De fleste utviklere bygger ikke fra bunnen av — de vedlikeholder, moderniserer og forbedrer. nav-pilot
              hjelper med hele spekteret.
            </p>
          </div>

          <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
            {USE_CASES.map((uc) => (
              <div
                key={uc.title}
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
                    background: `${uc.color}15`,
                    border: `1px solid ${uc.color}30`,
                  }}
                >
                  <uc.Icon fontSize="1.25rem" style={{ color: uc.color }} aria-hidden />
                </div>
                <div>
                  <p className="font-semibold mb-1" style={{ color: "white", fontSize: "0.9rem", margin: 0 }}>
                    {uc.title}
                  </p>
                  <p style={{ color: "#94a3b8", fontSize: "0.8125rem", lineHeight: 1.6, margin: 0 }}>
                    {uc.description}
                  </p>
                </div>
              </div>
            ))}
          </HGrid>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Collections ---------- */

function CollectionsSection() {
  return (
    <section className="dark-section" style={{ background: "#0f172a", color: "white" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap={{ xs: "space-16", md: "space-24" }}>
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3" style={{ color: "white" }}>
              Ferdigpakkede samlinger for din stack
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#94a3b8", marginBlock: 0, textAlign: "center" }}>
              Velg arketype — få agenter, ferdigheter og instruksjoner tilpasset din stack.
            </p>
          </div>

          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            {COLLECTIONS.map((c) => (
              <CollectionCard key={c.id} {...c} />
            ))}
          </HGrid>

          <div className="text-center">
            <NextLink
              href="/verktoy"
              className="no-underline transition-colors"
              style={{ color: "#60a5fa", fontSize: "0.875rem" }}
            >
              Se alle agenter og ferdigheter →
            </NextLink>
          </div>
        </VStack>
      </Box>

      {/* Gradient transition to light */}
      <div
        className="h-40"
        style={{
          background: `linear-gradient(to bottom,
            #0f172a 0%,
            #111a30 8%,
            #151f38 16%,
            #1c2844 26%,
            #263552 36%,
            #3b4f6e 48%,
            #5e7491 58%,
            #8da0b8 68%,
            #b8c5d5 78%,
            #dce2ea 87%,
            #eef1f5 93%,
            #f8fafc 100%
          )`,
        }}
      />
    </section>
  );
}

function CollectionCard({
  title,
  description,
  agents,
  skills,
  highlights,
  Icon,
  logos,
  codePreview,
}: (typeof COLLECTIONS)[number]) {
  return (
    <div
      className="rounded-xl overflow-hidden h-full flex flex-col transition-all"
      style={{
        background: "linear-gradient(180deg, #1e293b 0%, #162044 100%)",
        border: "1px solid rgba(255,255,255,0.08)",
      }}
    >
      {/* Text area — 2/3 */}
      <Box padding={{ xs: "space-12", md: "space-16" }} className="flex flex-col" style={{ flex: "2 1 0%" }}>
        <div className="flex flex-col flex-1">
          <div className="flex items-center gap-3 mb-3">
            <Icon fontSize="1.5rem" style={{ color: "#60a5fa" }} aria-hidden />
            <TechLogoRow
              logos={logos.map((Logo, i) => (
                <Logo key={i} size={18} />
              ))}
            />
          </div>
          <Heading size="xsmall" level="3" style={{ color: "white" }}>
            {title}
          </Heading>
          <p
            className="flex-1"
            style={{ color: "#cbd5e1", fontSize: "0.875rem", margin: "0.5rem 0 0", lineHeight: 1.6 }}
          >
            {description}
          </p>

          <div className="flex gap-4 mt-3 mb-3">
            <div className="flex items-center gap-1.5">
              <div className="w-1.5 h-1.5 rounded-full" style={{ background: "#60a5fa" }} />
              <span style={{ color: "#94a3b8", fontSize: "0.75rem" }}>{agents} agenter</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-1.5 h-1.5 rounded-full" style={{ background: "#a78bfa" }} />
              <span style={{ color: "#94a3b8", fontSize: "0.75rem" }}>{skills} ferdigheter</span>
            </div>
          </div>

          <div className="flex flex-wrap gap-1.5">
            {highlights.map((h) => (
              <span
                key={h}
                className="rounded-full px-2.5 py-0.5"
                style={{ fontSize: "0.7rem", background: "rgba(96,165,250,0.15)", color: "#93c5fd" }}
              >
                {h}
              </span>
            ))}
          </div>
        </div>
      </Box>

      {/* Code preview — 1/3 */}
      <div
        className="px-4 py-3 flex flex-col"
        style={{
          flex: "1 1 0%",
          borderTop: "1px solid rgba(255,255,255,0.06)",
          background: "rgba(0,0,0,0.3)",
        }}
      >
        <div className="flex-1">
          <pre
            className="font-mono leading-relaxed overflow-hidden"
            style={{ margin: 0, fontSize: "0.7rem", color: "#93c5fd" }}
          >
            {codePreview}
          </pre>
        </div>
      </div>
    </div>
  );
}

/* ---------- Pipeline ---------- */

function PipelineSection() {
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
              Fra idé til produksjon — eller fra teknisk gjeld til moderne løsning
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              nav-pilot guider deg gjennom fire faser — enten du bygger nytt eller forbedrer eksisterende kode.
            </p>
          </div>

          {/* Connected stepper */}
          <div
            className="w-full items-stretch gap-0"
            style={{
              display: "grid",
              gridTemplateColumns: "1fr auto 1fr auto 1fr auto 1fr",
              alignItems: "stretch",
            }}
          >
            {PIPELINE_STEPS.map((step, i) => (
              <React.Fragment key={step.title}>
                {/* Step box */}
                <div
                  className="rounded-xl overflow-hidden flex flex-col"
                  style={{
                    background: "white",
                    border: "1px solid #e2e8f0",
                    boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
                  }}
                >
                  <div style={{ height: "3px", background: step.color }} />
                  <Box padding={{ xs: "space-12", md: "space-16" }} className="flex-1 flex flex-col">
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
                      <Heading size="xsmall" level="3">
                        {step.title}
                      </Heading>
                      <p
                        style={{
                          color: "#64748b",
                          fontSize: "0.75rem",
                          margin: "0.25rem 0 0",
                          textAlign: "center",
                        }}
                      >
                        {step.subtitle}
                      </p>
                      <p
                        className="flex-1"
                        style={{
                          color: "#475569",
                          fontSize: "0.8125rem",
                          lineHeight: 1.5,
                          margin: "0.5rem 0 0",
                          textAlign: "center",
                        }}
                      >
                        {step.description}
                      </p>
                    </div>
                  </Box>
                </div>

                {/* Arrow connector */}
                {i < PIPELINE_STEPS.length - 1 && (
                  <div className="hidden md:flex items-center px-3">
                    <svg width="24" height="24" viewBox="0 0 24 24" fill="none" aria-hidden>
                      <path
                        d="M5 12h14m0 0l-5-5m5 5l-5 5"
                        stroke="#cbd5e1"
                        strokeWidth="1.5"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </div>
                )}
              </React.Fragment>
            ))}
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Comparison ---------- */

function ComparisonSection() {
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
              En smartere Copilot for Nav
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Vanlig Copilot vet ingenting om Nav. nav-pilot gir Copilot konteksten som mangler.
            </p>
          </div>

          {/* High-contrast comparison table */}
          <div
            className="w-full rounded-xl overflow-hidden"
            style={{ border: "1px solid #1e293b", boxShadow: "0 4px 12px rgba(0,0,0,0.08)" }}
          >
            {/* Dark header */}
            <div className="grid gap-0" style={{ gridTemplateColumns: "1fr 1fr 2fr", background: "#0f172a" }}>
              <div
                className="px-6 py-4 flex items-center justify-center"
                style={{ borderRight: "1px solid rgba(255,255,255,0.1)" }}
              >
                <p
                  className="font-semibold uppercase tracking-wider"
                  style={{ color: "rgba(255,255,255,0.5)", fontSize: "0.7rem", margin: 0, letterSpacing: "0.08em" }}
                >
                  Område
                </p>
              </div>
              <div
                className="px-6 py-4 flex items-center justify-center"
                style={{ borderRight: "1px solid rgba(255,255,255,0.1)" }}
              >
                <p
                  className="font-semibold"
                  style={{ color: "rgba(255,255,255,0.6)", fontSize: "0.8125rem", margin: 0 }}
                >
                  Vanlig Copilot
                </p>
              </div>
              <div className="px-6 py-4 flex items-center justify-center gap-2">
                <p className="font-bold" style={{ color: "#60a5fa", fontSize: "0.875rem", margin: 0 }}>
                  nav-pilot
                </p>
              </div>
            </div>

            {/* Rows */}
            {COMPARISONS.map((row, i) => (
              <div
                key={row.feature}
                className="grid gap-0"
                style={{
                  gridTemplateColumns: "1fr 1fr 2fr",
                  borderTop: "1px solid #e2e8f0",
                }}
              >
                <div
                  className="px-6 py-4 flex items-center justify-center"
                  style={{
                    borderRight: "1px solid #e2e8f0",
                    background: i % 2 === 0 ? "#f8fafc" : "white",
                  }}
                >
                  <p className="font-semibold" style={{ color: "#1e293b", fontSize: "0.875rem", margin: 0 }}>
                    {row.feature}
                  </p>
                </div>
                <div
                  className="px-6 py-4 flex items-center justify-center gap-2"
                  style={{
                    borderRight: "1px solid #e2e8f0",
                    background: i % 2 === 0 ? "#fef2f2" : "#fff5f5",
                  }}
                >
                  <XMarkOctagonIcon fontSize="0.875rem" style={{ color: "#ef4444", flexShrink: 0 }} aria-hidden />
                  <p style={{ color: "#64748b", fontSize: "0.8125rem", margin: 0, fontStyle: "italic" }}>
                    {row.generic}
                  </p>
                </div>
                <div
                  className="px-6 py-4 flex items-center gap-2 justify-center"
                  style={{
                    background: i % 2 === 0 ? "#f0fdf4" : "#f7fef9",
                  }}
                >
                  <CheckmarkCircleIcon fontSize="0.875rem" style={{ color: "#22c55e", flexShrink: 0 }} aria-hidden />
                  <p style={{ color: "#1e293b", fontSize: "0.8125rem", margin: 0, fontWeight: 600 }}>{row.navPilot}</p>
                </div>
              </div>
            ))}
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Testimonials ---------- */

const TESTIMONIALS = [
  {
    quote: "Din tilbakemelding her",
    team: "Ditt team",
    context: "Din stack",
    color: "#a78bfa",
  },
  {
    quote: "Din tilbakemelding her",
    team: "Ditt team",
    context: "Din stack",
    color: "#60a5fa",
  },
  {
    quote: "Din tilbakemelding her",
    team: "Ditt team",
    context: "Din stack",
    color: "#2dd4bf",
  },
];

function TestimonialsSection() {
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
              Hva utviklere sier
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Tilbakemeldinger fra team som bruker nav-pilot i hverdagen.
            </p>
          </div>

          <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
            {TESTIMONIALS.map((t) => (
              <div
                key={t.team}
                className="rounded-xl flex flex-col h-full"
                style={{
                  background: "white",
                  border: "1px solid #e2e8f0",
                  boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
                  overflow: "hidden",
                }}
              >
                <div style={{ height: "3px", background: t.color }} />
                <Box padding={{ xs: "space-16", md: "space-20" }} className="flex-1 flex flex-col">
                  <ChatIcon fontSize="1.5rem" style={{ color: t.color, marginBottom: "0.75rem" }} aria-hidden />
                  <p
                    className="flex-1"
                    style={{
                      color: "#334155",
                      fontSize: "0.9375rem",
                      lineHeight: 1.7,
                      margin: 0,
                      fontStyle: "italic",
                    }}
                  >
                    &ldquo;{t.quote}&rdquo;
                  </p>
                  <div style={{ marginTop: "1rem", borderTop: "1px solid #f1f5f9", paddingTop: "0.75rem" }}>
                    <p className="font-semibold" style={{ color: "#1e293b", fontSize: "0.8125rem", margin: 0 }}>
                      {t.team}
                    </p>
                    <p style={{ color: "#94a3b8", fontSize: "0.75rem", margin: "0.125rem 0 0" }}>{t.context}</p>
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

/* ---------- Tech Stack Strip ---------- */

function TechStackStrip() {
  const techs = [
    { Logo: KotlinLogo, name: "Kotlin" },
    { Logo: TypeScriptLogo, name: "TypeScript" },
    { Logo: ReactLogo, name: "React" },
    { Logo: NextjsLogo, name: "Next.js" },
    { Logo: PostgreSQLLogo, name: "PostgreSQL" },
    { Logo: KafkaLogo, name: "Kafka" },
    { Logo: KubernetesLogo, name: "Kubernetes" },
    { Logo: GoLogo, name: "Go" },
  ];

  const renderLogos = (prefix: string) =>
    techs.map(({ Logo, name }) => (
      <div key={`${prefix}-${name}`} className="flex items-center gap-2 opacity-60 shrink-0 px-6">
        <Logo size={24} />
        <span style={{ color: "#475569", fontSize: "0.8125rem", fontWeight: 500 }}>{name}</span>
      </div>
    ));

  return (
    <section style={{ background: "white" }}>
      <Box paddingBlock={{ xs: "space-20", md: "space-32" }} className="max-w-5xl mx-auto">
        <VStack gap="space-12">
          <p
            className="font-medium"
            style={{
              color: "#64748b",
              fontSize: "0.8125rem",
              margin: 0,
              letterSpacing: "0.05em",
              textAlign: "center",
            }}
          >
            BYGGET FOR NAVS TEKNOLOGI-STACK
          </p>
          <div
            className="overflow-hidden"
            style={{ maskImage: "linear-gradient(to right, transparent, black 10%, black 90%, transparent)" }}
          >
            <div className="flex items-center animate-marquee">
              {renderLogos("a")}
              {renderLogos("b")}
            </div>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Get Started ---------- */

function GetStartedSection() {
  return (
    <section style={{ background: "#f8fafc" }}>
      <Box
        paddingBlock={{ xs: "space-24", md: "space-40" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap="space-16">
          <div className="text-center">
            <Heading size="medium" level="2" className="mb-3">
              Kom i gang
            </Heading>
            <p className="max-w-2xl mx-auto" style={{ color: "#64748b", marginBlock: 0, textAlign: "center" }}>
              Installer en samling og bruk @nav-pilot med én gang.
            </p>
          </div>

          <div
            className="w-full rounded-xl p-6 md:p-8"
            style={{
              background: "white",
              border: "1px solid #e2e8f0",
              boxShadow: "0 1px 3px rgba(0,0,0,0.04)",
            }}
          >
            <VStack gap="space-16">
              <div>
                <Heading size="xsmall" level="3" className="mb-2">
                  1. Installer nav-pilot
                </Heading>
                <div
                  className="rounded-lg p-4 overflow-x-auto flex items-center gap-3"
                  style={{ background: "#1e1e1e" }}
                >
                  <code className="font-mono whitespace-nowrap flex-1" style={{ fontSize: "0.8rem", color: "#d4d4d4" }}>
                    {INSTALL_COMMAND}
                  </code>
                  <CopyButton copyText={INSTALL_COMMAND} size="xsmall" style={{ color: "white" }} />
                </div>
              </div>

              <div>
                <Heading size="xsmall" level="3" className="mb-2">
                  2. Installer en samling i repoet ditt
                </Heading>
                <div
                  className="rounded-lg p-4 overflow-x-auto flex items-center gap-3"
                  style={{ background: "#1e1e1e" }}
                >
                  <code className="font-mono whitespace-nowrap flex-1" style={{ fontSize: "0.8rem", color: "#d4d4d4" }}>
                    nav-pilot install kotlin-backend
                  </code>
                  <CopyButton copyText="nav-pilot install kotlin-backend" size="xsmall" style={{ color: "white" }} />
                </div>
                <p className="mt-2" style={{ color: "#64748b", fontSize: "0.8125rem", margin: "0.5rem 0 0" }}>
                  Tilgjengelige samlinger: fullstack · kotlin-backend · nextjs-frontend · platform
                </p>
              </div>

              <div>
                <Heading size="xsmall" level="3" className="mb-2">
                  3. Start @nav-pilot i editoren
                </Heading>
                <p style={{ color: "#475569", margin: 0 }}>Åpne Copilot Chat og skriv:</p>
                <div
                  className="rounded-lg p-4 overflow-x-auto flex items-center gap-3 mt-2"
                  style={{ background: "#1e1e1e" }}
                >
                  <code className="font-mono whitespace-nowrap flex-1" style={{ fontSize: "0.8rem", color: "#d4d4d4" }}>
                    @nav-pilot Jeg trenger en ny tjeneste for dagpenger
                  </code>
                  <CopyButton
                    copyText="@nav-pilot Jeg trenger en ny tjeneste for dagpenger"
                    size="xsmall"
                    style={{ color: "white" }}
                  />
                </div>
              </div>

              <div>
                <Heading size="xsmall" level="3" className="mb-2">
                  4. Følg de fire fasene
                </Heading>
                <p style={{ color: "#475569", margin: 0 }}>
                  nav-pilot guider deg gjennom intervju, planlegging, review og levering — hvert steg venter på din
                  bekreftelse.
                </p>
              </div>
            </VStack>
          </div>

          <div className="flex flex-wrap gap-3 justify-center">
            <NextLink
              href="/nav-pilot/docs"
              className="inline-flex items-center gap-1.5 px-5 py-2.5 rounded-lg font-medium no-underline transition-all"
              style={{ background: "#3b82f6", color: "white", fontSize: "0.875rem" }}
            >
              Les dokumentasjonen →
            </NextLink>
            <NextLink
              href="/verktoy"
              className="inline-flex items-center gap-1.5 px-5 py-2.5 rounded-lg font-medium no-underline transition-colors"
              style={{ border: "1px solid #d1d5db", color: "#374151", fontSize: "0.875rem" }}
            >
              Se alle verktøy →
            </NextLink>
          </div>
        </VStack>
      </Box>
    </section>
  );
}

/* ---------- Footer Tagline ---------- */

function FooterTagline() {
  return (
    <section
      className="dark-section"
      style={{
        background: "linear-gradient(165deg, #0c0e1a 0%, #0f172a 50%, #1a1040 100%)",
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
            Bygget av Nav-utviklere, for Nav-utviklere.
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
            Ingen hallusinasjoner, bare Nais.
          </p>
          <div className="flex flex-wrap gap-6 justify-center" style={{ fontSize: "0.875rem" }}>
            <NextLink
              href="https://github.com/navikt/copilot"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              GitHub
            </NextLink>
            <NextLink
              href="/verktoy"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              Verktøy
            </NextLink>
            <NextLink
              href="https://aksel.nav.no"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              Aksel
            </NextLink>
            <NextLink
              href="https://doc.nais.io"
              target="_blank"
              rel="noopener noreferrer"
              className="no-underline transition-colors"
              style={{ color: "rgba(255,255,255,0.5)" }}
            >
              Nais Docs
            </NextLink>
          </div>
        </VStack>
      </Box>
    </section>
  );
}
