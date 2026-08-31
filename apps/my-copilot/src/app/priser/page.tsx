import type { Metadata } from "next";
import { Box, VStack, Heading, BodyShort } from "@navikt/ds-react";
import { MODEL_PRICING, PRICING_SOURCE_URL, PRICING_LAST_UPDATED } from "@/lib/model-pricing";
import type { ModelPrice } from "@/lib/model-pricing";
import { PageHero } from "@/components/page-hero";
import NextLink from "next/link";

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

const PROVIDER_ORDER = ["OpenAI", "Anthropic", "Google", "GitHub", "Moonshot AI", "Microsoft"] as const;

// `promotionEndsOn` er en ren dato uten klokkeslett, og `new Date("2026-09-03")`
// blir midnatt UTC. Uten `timeZone` her ville formateringen falle tilbake på
// leserens sone, og vest for UTC viser 3. september seg da som 2. september.
const promotionEndFormat = new Intl.DateTimeFormat("nb-NO", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

function formatPrice(price: number): string {
  if (price < 0.1) return `$${price.toFixed(3)}`;
  if (price < 1) return `$${price.toFixed(2)}`;
  return `$${price.toFixed(2)}`;
}

function categoryColor(category: ModelPrice["category"]): string {
  switch (category) {
    case "Lightweight":
      return "#22c55e";
    case "Versatile":
      return "#3b82f6";
    case "Powerful":
      return "#a855f7";
  }
}

function categoryBg(category: ModelPrice["category"]): string {
  switch (category) {
    case "Lightweight":
      return "rgba(34, 197, 94, 0.1)";
    case "Versatile":
      return "rgba(59, 130, 246, 0.1)";
    case "Powerful":
      return "rgba(168, 85, 247, 0.1)";
  }
}

export default function PriserPage() {
  const grouped = PROVIDER_ORDER.map((provider) => {
    const models = MODEL_PRICING.filter((m) => m.provider === provider);
    // Cache write gjelder ikke bare Anthropic: OpenAI-radene har det også, og
    // kolonnen var gjemt bak et leverandørnavn i stedet for bak dataene.
    return { provider, models, showCacheWrite: models.some((m) => m.cacheWrite !== undefined) };
  }).filter((g) => g.models.length > 0);

  return (
    <main>
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
            {grouped.map(({ provider, models, showCacheWrite }) => (
              <Box key={provider}>
                <Heading size="small" level="2" className="mb-4">
                  {provider}
                </Heading>
                {showCacheWrite && (
                  <BodyShort size="small" className="mb-3" style={{ color: "#64748b" }}>
                    «Cache write» er kostnaden for å skrive kontekst til cache, og kommer i tillegg til cached input.
                  </BodyShort>
                )}
                <div className="w-full overflow-x-auto -mx-4 px-4 sm:mx-0 sm:px-0">
                  <table
                    className="w-full min-w-max sm:min-w-0 text-sm border-collapse"
                    style={{ borderRadius: "0.75rem", overflow: "hidden", border: "1px solid #e2e8f0" }}
                  >
                    <thead>
                      <tr style={{ background: "#f8fafc" }}>
                        <th className="text-left px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                          Modell
                        </th>
                        <th className="text-left px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                          Kategori
                        </th>
                        <th className="text-right px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                          Input
                        </th>
                        <th className="text-right px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                          Cached
                        </th>
                        {showCacheWrite && (
                          <th className="text-right px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                            Cache write
                          </th>
                        )}
                        <th className="text-right px-4 py-3 font-semibold" style={{ color: "#475569" }}>
                          Output
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {models.map((m, i) => (
                        <tr
                          key={m.model}
                          style={{ borderTop: "1px solid #e2e8f0", background: i % 2 === 0 ? "white" : "#fafbfc" }}
                        >
                          <td className="px-4 py-3">
                            <span className="font-medium" style={{ color: "#1e293b" }}>
                              {m.model}
                            </span>
                            {m.promotionEndsOn && (
                              <span
                                className="ml-2 inline-block rounded-full px-2 py-0.5 font-medium align-middle"
                                style={{
                                  fontSize: "0.6875rem",
                                  color: "#92400e",
                                  background: "rgba(245, 158, 11, 0.16)",
                                }}
                              >
                                Kampanjepris t.o.m. {promotionEndFormat.format(new Date(m.promotionEndsOn))}
                              </span>
                            )}
                          </td>
                          <td className="px-4 py-3">
                            <span
                              className="inline-block rounded-full px-2.5 py-0.5 font-medium"
                              style={{
                                fontSize: "0.6875rem",
                                color: categoryColor(m.category),
                                background: categoryBg(m.category),
                              }}
                            >
                              {m.category}
                            </span>
                          </td>
                          <td
                            className="px-4 py-3 text-right font-mono"
                            style={{ color: "#1e293b", fontSize: "0.8125rem" }}
                          >
                            {formatPrice(m.input)}
                          </td>
                          <td
                            className="px-4 py-3 text-right font-mono"
                            style={{ color: "#64748b", fontSize: "0.8125rem" }}
                          >
                            {formatPrice(m.cachedInput)}
                          </td>
                          {showCacheWrite && (
                            <td
                              className="px-4 py-3 text-right font-mono"
                              style={{ color: "#64748b", fontSize: "0.8125rem" }}
                            >
                              {m.cacheWrite !== undefined ? formatPrice(m.cacheWrite) : "—"}
                            </td>
                          )}
                          <td
                            className="px-4 py-3 text-right font-mono"
                            style={{ color: "#1e293b", fontSize: "0.8125rem" }}
                          >
                            {formatPrice(m.output)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </Box>
            ))}

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
                    <strong>Nav Business-kvote:</strong> 1 900 credits/bruker/mnd ($19) — poolet på org-nivå
                  </li>
                  <li>
                    <strong>Cached tokens koster 90 % mindre</strong> — fokuserte sesjoner utnytter caching bedre
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
                  <li>Opus er 67 % dyrere enn Sonnet — bruk Opus kun for komplekse arkitekturbeslutninger</li>
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
              {" · "}Sist oppdatert: {PRICING_LAST_UPDATED}
            </BodyShort>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
