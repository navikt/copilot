"use client";

import { useState, useMemo } from "react";
import {
  Box,
  VStack,
  HGrid,
  Heading,
  BodyShort,
  Table,
  TextField,
  Select,
  Label,
  Radio,
  RadioGroup,
  HelpText,
  Tag,
  Alert,
} from "@navikt/ds-react";
import { TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@navikt/ds-react/Table";
import { formatNumber } from "@/lib/format";
import {
  calculateAll,
  BUSINESS_PLAN_COST,
  PROMO_CREDITS_PER_SEAT,
  DEFAULT_CACHE_RATE,
  DEFAULT_DATA_PERIOD_DAYS,
  DEFAULT_CLI_MODEL,
  CLI_MODEL_OPTIONS,
  PROFILE_LABELS,
  type CalculatorInputs,
  type ModelPremiumData,
  type CLIData,
  type ProfileName,
} from "@/lib/billing-calculator";

interface CalculatorContentProps {
  initialSeats: number;
  initialModels: ModelPremiumData[];
  initialCLI: CLIData;
  initialDataPeriodDays: number;
  currentGrossCost: number;
  currentNetCost: number;
}

function formatUSD(value: number): string {
  return `$${value.toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

function formatTokens(tokens: number): string {
  if (tokens >= 1_000_000_000) return `${(tokens / 1_000_000_000).toFixed(1)}B`;
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(1)}M`;
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(0)}K`;
  return tokens.toString();
}

export default function CalculatorContent({
  initialSeats,
  initialModels,
  initialCLI,
  initialDataPeriodDays,
  currentGrossCost,
  currentNetCost,
}: CalculatorContentProps) {
  const [seats, setSeats] = useState(initialSeats);
  const [creditsPerSeat, setCreditsPerSeat] = useState(BUSINESS_PLAN_COST);
  const [cacheRate, setCacheRate] = useState(DEFAULT_CACHE_RATE * 100);
  const [profile, setProfile] = useState<ProfileName>("moderate");
  const [dataPeriodDays, setDataPeriodDays] = useState(initialDataPeriodDays || DEFAULT_DATA_PERIOD_DAYS);
  const [cliInputTokens, setCliInputTokens] = useState(initialCLI.inputTokens);
  const [cliOutputTokens, setCliOutputTokens] = useState(initialCLI.outputTokens);
  const [cliModel, setCliModel] = useState(DEFAULT_CLI_MODEL);

  const inputs: CalculatorInputs = useMemo(
    () => ({
      seats,
      creditsPerSeat,
      cacheRate: cacheRate / 100,
      profile,
      dataPeriodDays,
      models: initialModels,
      cliModel,
      cli: {
        inputTokens: cliInputTokens,
        outputTokens: cliOutputTokens,
        sessions: initialCLI.sessions,
        requests: initialCLI.requests,
      },
    }),
    [
      seats,
      creditsPerSeat,
      cacheRate,
      profile,
      dataPeriodDays,
      initialModels,
      cliModel,
      cliInputTokens,
      cliOutputTokens,
      initialCLI.sessions,
      initialCLI.requests,
    ]
  );

  const result = useMemo(() => calculateAll(inputs), [inputs]);
  const hasModelData = initialModels.length > 0 && initialModels.some((m) => m.requests > 0);

  return (
    <VStack gap="space-24">
      {!hasModelData && (
        <Alert variant="warning">
          Ingen bruksdata tilgjengelig for denne perioden. Tallene under er kun basert på innstillingene du velger.
          Bruksdata lastes fra GitHub og er kun tilgjengelig i produksjon.
        </Alert>
      )}
      {/* Settings panel */}
      <Box background="neutral-soft" padding={{ xs: "space-16", md: "space-24" }} borderRadius="12">
        <VStack gap="space-16">
          <Heading size="small" level="2">
            Innstillinger
          </Heading>
          <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
            <TextField
              label="Antall lisenser"
              type="number"
              value={seats.toString()}
              onChange={(e) => setSeats(Math.max(1, parseInt(e.target.value) || 1))}
              size="small"
              htmlSize={8}
            />
            <Select
              label="Kreditt per lisens"
              value={creditsPerSeat.toString()}
              onChange={(e) => setCreditsPerSeat(parseInt(e.target.value))}
              size="small"
            >
              <option value={BUSINESS_PLAN_COST.toString()}>${BUSINESS_PLAN_COST} (standard)</option>
              <option value={PROMO_CREDITS_PER_SEAT.toString()}>${PROMO_CREDITS_PER_SEAT} (promo jun–aug)</option>
            </Select>
            <div>
              <Label size="small" htmlFor="cache-rate">
                <span className="flex items-center gap-1">
                  Cache-rate: {cacheRate} %
                  <HelpText title="Cache-rate" placement="top">
                    Andel av input-tokens som treffer kontekst-cache. Høyere cache-rate gir lavere kostnad. 80 % er et
                    vanlig estimat for Copilot.
                  </HelpText>
                </span>
              </Label>
              <input
                id="cache-rate"
                type="range"
                min="0"
                max="95"
                step="5"
                value={cacheRate}
                onChange={(e) => setCacheRate(parseInt(e.target.value))}
                className="w-full mt-2"
              />
            </div>
            <TextField
              label="Dataperiode (dager)"
              type="number"
              value={dataPeriodDays.toString()}
              onChange={(e) => setDataPeriodDays(Math.max(1, parseInt(e.target.value) || 1))}
              size="small"
              htmlSize={6}
            />
          </HGrid>
          <RadioGroup
            legend="Estimatprofil"
            description="Hvor mange tokens som estimeres per forespørsel. Forsiktig = korte samtaler, Tung = lange agentøkter."
            value={profile}
            onChange={(val) => setProfile(val as ProfileName)}
            size="small"
            className="flex gap-4"
          >
            <div className="flex gap-4">
              {(Object.entries(PROFILE_LABELS) as [ProfileName, string][]).map(([key, label]) => (
                <Radio key={key} value={key}>
                  {label}
                </Radio>
              ))}
            </div>
          </RadioGroup>
          <HGrid columns={{ xs: 1, sm: 3 }} gap="space-16">
            <TextField
              label="CLI input-tokens (siste periode)"
              type="number"
              value={cliInputTokens.toString()}
              onChange={(e) => setCliInputTokens(Math.max(0, parseInt(e.target.value) || 0))}
              size="small"
              description={`${formatTokens(cliInputTokens)} tokens — fra Copilot CLI i hele organisasjonen`}
            />
            <TextField
              label="CLI output-tokens (siste periode)"
              type="number"
              value={cliOutputTokens.toString()}
              onChange={(e) => setCliOutputTokens(Math.max(0, parseInt(e.target.value) || 0))}
              size="small"
              description={`${formatTokens(cliOutputTokens)} tokens — genererte svar fra CLI`}
            />
            <Select
              label="CLI-modell"
              value={cliModel}
              onChange={(e) => setCliModel(e.target.value)}
              size="small"
              description="Modell CLI bruker — påvirker kostnaden vesentlig"
            >
              {CLI_MODEL_OPTIONS.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </Select>
          </HGrid>
        </VStack>
      </Box>

      {/* Summary cards */}
      <div>
        <Heading size="medium" level="2" className="mb-4">
          Nøkkeltall
        </Heading>
        <HGrid columns={{ xs: 1, sm: 2, lg: 3 }} gap="space-16">
          <SummaryCard
            label="Dagens bruttokostnad"
            value={formatUSD(currentGrossCost)}
            subtitle={`${dataPeriodDays} dager (PRU-modell)`}
          />
          <SummaryCard label="Dagens nettokostnad" value={formatUSD(currentNetCost)} subtitle="Etter rabatt" />
          <SummaryCard
            label="Estimert ny kostnad"
            value={formatUSD(result.totalNewCost)}
            subtitle={`${dataPeriodDays} dager (AI Credits)`}
            highlight
          />
          <SummaryCard
            label="Estimert månedskostnad"
            value={formatUSD(result.monthlyCost)}
            subtitle="30-dagers projeksjon"
          />
          <SummaryCard
            label="Inkluderte kreditter/mnd"
            value={formatUSD(result.creditPool.monthlyCredits)}
            subtitle={`${formatNumber(seats)} lisenser × $${creditsPerSeat}`}
          />
          <SummaryCard
            label={result.creditPool.surplusStandard >= 0 ? "Overskudd / mnd" : "Overskridelse / mnd"}
            value={`${result.creditPool.surplusStandard >= 0 ? "+" : ""}${formatUSD(result.creditPool.surplusStandard)}`}
            highlight={result.creditPool.surplusStandard < 0}
          />
        </HGrid>
      </div>

      {/* Credit pool analysis */}
      <Box background="neutral-soft" padding={{ xs: "space-16", md: "space-24" }} borderRadius="12">
        <VStack gap="space-16">
          <Heading size="small" level="2">
            Kredittanalyse
          </Heading>
          <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
            <div>
              <BodyShort className="text-gray-600 mb-1">Estimert månedskostnad</BodyShort>
              <Heading size="medium" level="3">
                {formatUSD(result.monthlyCost)}
              </Heading>
            </div>
            <div>
              <BodyShort className="text-gray-600 mb-1">
                Standardkreditter ({formatNumber(seats)} × ${creditsPerSeat})
              </BodyShort>
              <Heading size="medium" level="3">
                {formatUSD(result.creditPool.monthlyCredits)}
              </Heading>
            </div>
            <div>
              <BodyShort className="text-gray-600 mb-1">
                {result.creditPool.surplusStandard >= 0 ? "Overskudd" : "Overskridelse"}
              </BodyShort>
              <Heading
                size="medium"
                level="3"
                className={result.creditPool.surplusStandard >= 0 ? "text-green-600" : "text-red-600"}
              >
                {result.creditPool.surplusStandard >= 0 ? "+" : ""}
                {formatUSD(result.creditPool.surplusStandard)}
              </Heading>
            </div>
          </HGrid>
          {creditsPerSeat !== PROMO_CREDITS_PER_SEAT && (
            <BodyShort size="small" className="text-gray-500">
              Med promo-kreditter (${PROMO_CREDITS_PER_SEAT}/lisens, jun–aug):{" "}
              <strong className={result.creditPool.surplusPromo >= 0 ? "text-green-600" : "text-red-600"}>
                {result.creditPool.surplusPromo >= 0 ? "+" : ""}
                {formatUSD(result.creditPool.surplusPromo)}
              </strong>
            </BodyShort>
          )}
        </VStack>
      </Box>

      {/* Scenario comparison */}
      <div>
        <Heading size="medium" level="2" className="mb-4">
          Scenariosammenligning
        </Heading>
        <HGrid columns={{ xs: 1, md: 3 }} gap="space-16">
          {result.scenarios.map((s) => (
            <Box
              key={s.profile}
              background={s.profile === profile ? "accent-soft" : "default"}
              padding="space-20"
              borderRadius="8"
              className="border border-gray-200"
            >
              <VStack gap="space-8">
                <div className="flex items-center gap-2">
                  <Heading size="small" level="3">
                    {s.label}
                  </Heading>
                  {s.profile === profile && (
                    <Tag variant="info" size="small">
                      Valgt
                    </Tag>
                  )}
                </div>
                <div>
                  <BodyShort size="small" className="text-gray-600">
                    Estimert månedskostnad
                  </BodyShort>
                  <Heading size="medium" level="4">
                    {formatUSD(s.monthlyCost)}
                  </Heading>
                </div>
                <div>
                  <BodyShort size="small" className="text-gray-600">
                    Modellkostnad ({dataPeriodDays}d)
                  </BodyShort>
                  <BodyShort>{formatUSD(s.totalModelCost)}</BodyShort>
                </div>
                <div>
                  <BodyShort size="small" className="text-gray-600">
                    CLI-kostnad ({dataPeriodDays}d)
                  </BodyShort>
                  <BodyShort>{formatUSD(s.cliCost)}</BodyShort>
                </div>
                <div>
                  <BodyShort size="small" className="text-gray-600">
                    {s.surplus >= 0 ? "Overskudd" : "Overskridelse"}
                  </BodyShort>
                  <BodyShort weight="semibold" className={s.surplus >= 0 ? "text-green-600" : "text-red-600"}>
                    {s.surplus >= 0 ? "+" : ""}
                    {formatUSD(s.surplus)} / mnd
                  </BodyShort>
                </div>
              </VStack>
            </Box>
          ))}
        </HGrid>
      </div>

      {/* Per-model breakdown */}
      <div>
        <Heading size="medium" level="2" className="mb-2">
          Modellkostnader
        </Heading>
        <BodyShort className="text-gray-600 mb-4">
          Estimert tokenforbruk og kostnad per modell med {PROFILE_LABELS[profile].toLowerCase()} profil og {cacheRate}{" "}
          % cache-rate.
        </BodyShort>
        <div className="overflow-x-auto">
          <Table size="small">
            <TableHeader>
              <TableRow>
                <TableHeaderCell scope="col">Modell</TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Forespørsler
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Dagens kostnad
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Est. input-tokens
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Est. output-tokens
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Ny kostnad
                </TableHeaderCell>
                <TableHeaderCell scope="col" align="right">
                  Endring
                </TableHeaderCell>
              </TableRow>
            </TableHeader>
            <TableBody>
              {result.modelEstimates.map((m) => {
                const delta = m.newCost - m.currentGrossCost;
                const deltaPct = m.currentGrossCost > 0 ? (delta / m.currentGrossCost) * 100 : 0;
                return (
                  <TableRow key={m.model}>
                    <TableDataCell>
                      <BodyShort weight="semibold" size="small">
                        {m.model}
                      </BodyShort>
                      <BodyShort size="small" className="text-gray-500">
                        {m.category}
                      </BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small">{formatNumber(Math.round(m.requests))}</BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small">{formatUSD(m.currentGrossCost)}</BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small">{formatTokens(m.estInputTokens)}</BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small">{formatTokens(m.estOutputTokens)}</BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small" weight="semibold">
                        {formatUSD(m.newCost)}
                      </BodyShort>
                    </TableDataCell>
                    <TableDataCell align="right">
                      <BodyShort size="small" className={delta >= 0 ? "text-red-600" : "text-green-600"}>
                        {delta >= 0 ? "+" : ""}
                        {deltaPct.toFixed(0)} %
                      </BodyShort>
                    </TableDataCell>
                  </TableRow>
                );
              })}
              {/* Totals row */}
              <TableRow>
                <TableDataCell>
                  <BodyShort weight="semibold" size="small">
                    Totalt (modeller)
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  <BodyShort weight="semibold" size="small">
                    {formatNumber(Math.round(initialModels.reduce((s, m) => s + m.requests, 0)))}
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  <BodyShort weight="semibold" size="small">
                    {formatUSD(currentGrossCost)}
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  <BodyShort weight="semibold" size="small">
                    {formatTokens(result.modelEstimates.reduce((s, m) => s + m.estInputTokens, 0))}
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  <BodyShort weight="semibold" size="small">
                    {formatTokens(result.modelEstimates.reduce((s, m) => s + m.estOutputTokens, 0))}
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  <BodyShort weight="semibold" size="small">
                    {formatUSD(result.modelEstimates.reduce((s, m) => s + m.newCost, 0))}
                  </BodyShort>
                </TableDataCell>
                <TableDataCell align="right">
                  {(() => {
                    const totalNew = result.modelEstimates.reduce((s, m) => s + m.newCost, 0);
                    const delta = totalNew - currentGrossCost;
                    const pct = currentGrossCost > 0 ? (delta / currentGrossCost) * 100 : 0;
                    return (
                      <BodyShort
                        weight="semibold"
                        size="small"
                        className={delta >= 0 ? "text-red-600" : "text-green-600"}
                      >
                        {delta >= 0 ? "+" : ""}
                        {pct.toFixed(0)} %
                      </BodyShort>
                    );
                  })()}
                </TableDataCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </div>

      {/* CLI analysis */}
      <Box background="neutral-soft" padding={{ xs: "space-16", md: "space-24" }} borderRadius="12">
        <VStack gap="space-16">
          <Heading size="small" level="2">
            CLI-analyse ({cliModel})
          </Heading>
          <HGrid columns={{ xs: 1, sm: 2, md: 4 }} gap="space-16">
            <div>
              <BodyShort className="text-gray-600 mb-1">Input-tokens</BodyShort>
              <Heading size="small" level="3">
                {formatTokens(cliInputTokens)}
              </Heading>
            </div>
            <div>
              <BodyShort className="text-gray-600 mb-1">Output-tokens</BodyShort>
              <Heading size="small" level="3">
                {formatTokens(cliOutputTokens)}
              </Heading>
            </div>
            <div>
              <BodyShort className="text-gray-600 mb-1">Cache-rate</BodyShort>
              <Heading size="small" level="3">
                {cacheRate} %
              </Heading>
            </div>
            <div>
              <BodyShort className="text-gray-600 mb-1">Estimert kostnad ({dataPeriodDays}d)</BodyShort>
              <Heading size="small" level="3">
                {formatUSD(result.cliEstimate.cost)}
              </Heading>
            </div>
          </HGrid>
          {initialCLI.sessions > 0 && (
            <BodyShort size="small" className="text-gray-500">
              {formatNumber(initialCLI.sessions)} sesjoner, {formatNumber(initialCLI.requests)} forespørsler siste{" "}
              {dataPeriodDays} dager
            </BodyShort>
          )}
        </VStack>
      </Box>

      {/* Methodology note */}
      <Box padding={{ xs: "space-16", md: "space-20" }} borderRadius="8" className="border border-gray-200">
        <VStack gap="space-8">
          <Heading size="xsmall" level="3">
            Om estimatene
          </Heading>
          <BodyShort size="small" className="text-gray-600">
            Estimatene er basert på faktiske premiumforespørseldata fra GitHub og faktisk tokenforbruk fra CLI. Siden
            premium-forespørseldata ikke inneholder tokentall per forespørsel, estimeres tokens per forespørsel basert
            på modellkategori og valgt profil. Cache-raten påvirker kostnaden betydelig — 80 % er et rimelig
            standardanslag for Copilot.
          </BodyShort>
          <BodyShort size="small" className="text-gray-600">
            Kilde:{" "}
            <a
              href="https://docs.github.com/copilot/reference/copilot-billing/models-and-pricing"
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-600 hover:underline"
            >
              GitHub Copilot — Models and pricing
            </a>
          </BodyShort>
        </VStack>
      </Box>
    </VStack>
  );
}

function SummaryCard({
  label,
  value,
  subtitle,
  highlight,
}: {
  label: string;
  value: string;
  subtitle?: string;
  highlight?: boolean;
}) {
  return (
    <Box
      background={highlight ? "accent-soft" : "default"}
      padding="space-20"
      borderRadius="8"
      className="border border-gray-200"
    >
      <VStack gap="space-2">
        <BodyShort className="text-gray-600 text-sm">{label}</BodyShort>
        <Heading size="medium" level="3" className="break-all">
          {value}
        </Heading>
        {subtitle && <BodyShort className="text-gray-500 text-sm">{subtitle}</BodyShort>}
      </VStack>
    </Box>
  );
}
