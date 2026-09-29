import type { Metadata } from "next";
import { Box, VStack, Heading, BodyShort, Tag } from "@navikt/ds-react";
import { PRICING_SOURCE_URL, PRICING_LAST_UPDATED } from "@/lib/model-pricing";
import { isNavAllowedModel, NAV_MODEL_POLICY_LAST_UPDATED, NAV_PILOT_MODEL_CHOICES } from "@/lib/model-policy";
import { PageHero } from "@/components/page-hero";
import NextLink from "next/link";
import { ModelPricingTables } from "./model-pricing-tables";

export const metadata: Metadata = {
  title: "Modellpriser — Token-priser for GitHub Copilot",
  description:
    "Oppdatert pristabell for alle modeller tilgjengelig i GitHub Copilot. Pris per million tokens for input, cached input og output.",
  openGraph: {
    title: "Modellpriser — Token-priser for GitHub Copilot",
    description:
      "Oppdatert pristabell for alle modeller i GitHub Copilot. Se hva ulike modeller koster per million tokens.",
    type: "website",
  },
  twitter: {
    card: "summary_large_image",
    title: "Modellpriser — Token-priser for GitHub Copilot",
    description: "Se hva ulike modeller koster per million tokens i GitHub Copilot.",
  },
};

function ModelWithAvailability({ model }: { model: string }) {
  const allowed = isNavAllowedModel(model);
  return (
    <>
      {model}{" "}
      <Tag size="xsmall" variant="moderate" data-color={allowed ? "success" : "warning"}>
        {allowed ? "Aktivert i Nav" : "Ikke aktivert i Nav"}
      </Tag>
    </>
  );
}

export default function PriserPage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Modellpriser"
        description="Pris per million tokens for alle modeller i GitHub Copilot. 1 AI Credit = $0.01."
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <VStack gap={{ xs: "space-24", md: "space-32" }}>
            <Box as="section">
              <VStack gap="space-16">
                <div>
                  <Heading size="small" level="2" spacing>
                    nav-pilots modellvalg
                  </Heading>
                  <BodyShort>
                    nav-pilot velger modell etter oppgaven. Modellpolicyen avgjør om valget faktisk kan brukes i Nav.
                    Statusen under er kontrollert {NAV_MODEL_POLICY_LAST_UPDATED}.
                  </BodyShort>
                </div>
                <ul
                  style={{
                    display: "grid",
                    gap: "var(--ax-space-12)",
                    listStyle: "none",
                    margin: 0,
                    padding: 0,
                  }}
                >
                  {NAV_PILOT_MODEL_CHOICES.map((choice) => (
                    <Box
                      as="li"
                      key={choice.purpose}
                      padding="space-16"
                      background="neutral-soft"
                      borderColor="neutral-subtle"
                      borderWidth="1"
                      borderRadius="8"
                    >
                      <VStack gap="space-8">
                        <Heading size="xsmall" level="3">
                          {choice.purpose}
                        </Heading>
                        <BodyShort size="small">
                          <strong>Primærvalg:</strong> <ModelWithAvailability model={choice.primary} />
                        </BodyShort>
                        {choice.fallbacks.length > 0 && (
                          <BodyShort size="small">
                            <strong>Fallback:</strong>{" "}
                            {choice.fallbacks.map((model, index) => (
                              <span key={model}>
                                {index > 0 && ", "}
                                <ModelWithAvailability model={model} />
                              </span>
                            ))}
                          </BodyShort>
                        )}
                        <BodyShort size="small" textColor="subtle">
                          {choice.usedBy}
                        </BodyShort>
                      </VStack>
                    </Box>
                  ))}
                </ul>
              </VStack>
            </Box>

            <Box
              padding={{ xs: "space-16", md: "space-20" }}
              background="info-soft"
              borderColor="info"
              borderWidth="1"
              borderRadius="12"
            >
              <VStack gap="space-8">
                <Heading size="xsmall" level="2">
                  Hvorfor tabellen viser flere modeller
                </Heading>
                <BodyShort size="small">
                  GitHubs prisliste inneholder hele Copilot-utvalget. Den sier ikke hvilke modeller Nav har aktivert.
                  Bruk filtrene «Nav-status» og «nav-pilot» for å skille tilgjengelige modeller fra resten av prislista.
                </BodyShort>
              </VStack>
            </Box>

            <ModelPricingTables />

            {/* Context section */}
            <Box
              padding={{ xs: "space-16", md: "space-20" }}
              className="rounded-xl"
              style={{ background: "#f8fafc", border: "1px solid #e2e8f0" }}
            >
              <VStack gap="space-12">
                <Heading size="xsmall" level="3">
                  Hva betyr dette i praksis?
                </Heading>
                <ul className="space-y-2" style={{ color: "#475569", fontSize: "0.875rem", paddingLeft: "1.25rem" }}>
                  <li>
                    <strong>Nav Business-kvote:</strong> 1 900 credits/bruker/mnd ($19), poolet på org-nivå
                  </li>
                  <li>
                    <strong>Cached tokens koster 90 % mindre</strong>, så fokuserte sesjoner utnytter caching bedre
                  </li>
                  <li>
                    <strong>Auto-modus</strong> velger modell etter oppgave og gir innebygd rabatt
                  </li>
                  <li>
                    <strong>Code completions</strong> (ghost text) er gratis og teller ikke mot kvoten
                  </li>
                  <li>
                    Rader merket <strong>Long context</strong> er ikke egne modeller, men en høyere sats som slår inn
                    når konteksten passerer terskelen i navnet
                  </li>
                  <li>
                    <strong>Kampanjepris</strong> betyr at prisen i tabellen gjelder til datoen som står i merket, og så
                    går modellen tilbake til standardpris. GitHub oppgir ikke standardprisen. For GPT-5.6 Sol er
                    kampanjen 50 % av standardpris, så fra 4. september 2026 blir prisen etter alt å dømme $4.00 input
                    og $20.00 output.
                  </li>
                  <li>Opus er 67 % dyrere enn Sonnet. Bruk Opus kun for komplekse arkitekturbeslutninger</li>
                </ul>
              </VStack>
            </Box>

            {/* Source link */}
            <BodyShort size="small" style={{ color: "#94a3b8" }}>
              Kilde:{" "}
              <NextLink
                href={PRICING_SOURCE_URL}
                target="_blank"
                rel="noopener noreferrer"
                className="underline"
                style={{ color: "#64748b" }}
              >
                GitHub Copilot Models and Pricing
              </NextLink>
              {" · "}Priser sist oppdatert: {PRICING_LAST_UPDATED}
              {" · "}Nav-status sist kontrollert: {NAV_MODEL_POLICY_LAST_UPDATED}
            </BodyShort>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
