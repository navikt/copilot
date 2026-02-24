import { Heading, BodyShort, HGrid, Box, Table, HelpText } from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { formatNumber } from "@/lib/format";
import MetricCard from "./metric-card";
import type { PremiumMetrics } from "@/lib/billing-utils";

interface PremiumRequestsContentProps {
  metrics: PremiumMetrics;
}

export default function PremiumRequestsContent({ metrics }: PremiumRequestsContentProps) {
  return (
    <div className="space-y-6">
      <Heading size="medium">Oversikt over premiumforespørsler</Heading>

      <HGrid columns={4} gap="space-16">
        <MetricCard
          value={formatNumber(metrics.totalGrossRequests)}
          label="Totale forespørsler"
          helpTitle="Totale forespørsler"
          helpText="Totalt antall premium AI-forespørsler brukt i perioden, inkludert både inkluderte og fakturerte forespørsler."
          accentColor="blue"
        />

        <MetricCard
          value={formatNumber(metrics.totalIncludedRequests)}
          label="Inkluderte forespørsler"
          helpTitle="Inkluderte forespørsler"
          helpText="Antall forespørsler som er inkludert i abonnementet og ikke medfører ekstra kostnader."
          accentColor="green"
        />

        <MetricCard
          value={formatNumber(metrics.totalBilledRequests)}
          label="Fakturerte forespørsler"
          helpTitle="Fakturerte forespørsler"
          helpText="Antall forespørsler som går utover kvoten og blir fakturert separat."
          accentColor="orange"
        />

        <MetricCard
          value={`$${metrics.totalNetAmount.toFixed(2)}`}
          label="Nettokostnad"
          helpTitle="Nettokostnad"
          helpText="Total kostnad etter rabatter for premiumforespørsler i perioden."
          accentColor="purple"
        />
      </HGrid>

      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="small" level="3" className="mb-4">
          Kostnadsdetaljer
        </Heading>
        <HGrid columns={3} gap="space-16">
          <div>
            <BodyShort className="text-gray-600 mb-1">Bruttokostnad</BodyShort>
            <Heading size="medium" level="4">
              ${metrics.totalGrossAmount.toFixed(2)}
            </Heading>
          </div>
          <div>
            <BodyShort className="text-gray-600 mb-1">Rabatt</BodyShort>
            <Heading size="medium" level="4" className="text-green-600">
              -${metrics.totalDiscountAmount.toFixed(2)}
            </Heading>
          </div>
          <div>
            <BodyShort className="text-gray-600 mb-1">Nettokostnad</BodyShort>
            <Heading size="medium" level="4">
              ${metrics.totalNetAmount.toFixed(2)}
            </Heading>
          </div>
        </HGrid>
      </Box>

      <div>
        <Heading size="medium" level="3" className="mb-4">
          AI-modeller og bruk
        </Heading>
        <BodyShort className="text-gray-600 mb-4">
          Oversikt over hvilke AI-modeller som har blitt brukt og deres tilhørende kostnader. Ulike modeller har
          forskjellige priser per forespørsel basert på deres kapasitet og ytelse.
        </BodyShort>

        <Table>
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">Modell</TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Antall forespørsler
                  <div className="text-gray-900">
                    <HelpText title="Antall forespørsler" placement="top">
                      Totalt antall forespørsler gjort til denne modellen i perioden.
                    </HelpText>
                  </div>
                </div>
              </TableHeaderCell>
              <TableHeaderCell scope="col">
                <div className="flex items-center gap-1">
                  Bruttokostnad
                  <div className="text-gray-900">
                    <HelpText title="Bruttokostnad" placement="top">
                      Totalkostnad for forespørslene til denne modellen før rabatt.
                    </HelpText>
                  </div>
                </div>
              </TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {metrics.modelBreakdown.map((model) => (
              <TableRow key={model.model}>
                <TableDataCell>
                  <BodyShort weight="semibold">{model.model}</BodyShort>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort>{formatNumber(model.requests)}</BodyShort>
                </TableDataCell>
                <TableDataCell>
                  <BodyShort>${model.amount.toFixed(2)}</BodyShort>
                </TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      <div className="space-y-5">
        <div>
          <Heading size="medium" level="3" className="mb-3">
            Hva er premiumforespørsler?
          </Heading>
          <BodyShort className="text-gray-600">
            Premiumforespørsler gir deg tilgang til avanserte AI-modeller og funksjonalitet i GitHub Copilot. Dette
            inkluderer Copilot Chat med premium-modeller, større kontekstvinduer, avanserte resonnementmodeller, og
            funksjoner som Copilot coding agent.
          </BodyShort>
        </div>

        <div>
          <Heading size="small" level="4" className="mb-3">
            Hvordan måles bruken?
          </Heading>
          <div className="space-y-3">
            <BodyShort className="text-gray-600">
              <strong>Månedlig kvote:</strong> Hver lisens inkluderer et fast antall premiumforespørsler per bruker per
              måned. Kvoten nullstilles den 1. i hver måned kl. 00:00:00 UTC.
            </BodyShort>
            <BodyShort className="text-gray-600">
              <strong>Modellmultiplikatorer:</strong> Noen modeller bruker multiplikatorer, noe som betyr at én
              interaksjon kan telle som flere premiumforespørsler. For eksempel kan avanserte resonnementmodeller
              forbruke 5× eller 20× standardraten.
            </BodyShort>
            <BodyShort className="text-gray-600">
              <strong>Copilot coding agent:</strong> Hver coding agent-økt forbruker én premiumforespørsel. En økt
              starter når du ber Copilot om å opprette en pull request eller gjøre endringer i en eksisterende PR.
            </BodyShort>
          </div>
        </div>

        <div>
          <Heading size="small" level="4" className="mb-3">
            Hva skjer hvis vi bruker mer enn kvoten?
          </Heading>
          <BodyShort className="text-gray-600 mb-2">
            Hvis organisasjonen overskrider den inkluderte kvoten, har administratorer mulighet til å:
          </BodyShort>
          <ul className="list-disc list-inside text-gray-600 space-y-1 ml-4">
            <li>Sette et budsjett for ekstra premiumforespørsler</li>
            <li>Konfigurere retningslinjer for om medlemmer kan overskride kvoten</li>
            <li>Motta varsler ved 75%, 90% og 100% av budsjettet</li>
            <li>Blokkere overforbruk når budsjettet er nådd</li>
          </ul>
        </div>
      </div>
    </div>
  );
}
