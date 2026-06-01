import { Suspense } from "react";
import { Box, Heading, BodyShort, VStack, Skeleton, HGrid } from "@navikt/ds-react";
import { getCachedCopilotBilling, getCachedPremiumRequestUsageWithToken } from "@/lib/cached-github";
import { getCachedBigQueryUsage } from "@/lib/cached-bigquery";
import { calculatePremiumMetrics } from "@/lib/billing-utils";
import { getUser, getUserToken } from "@/lib/auth";
import { getCLIMetrics } from "@/lib/data-utils";
import CalculatorContent from "@/components/calculator-content";
import type { ModelPremiumData, CLIData } from "@/lib/billing-calculator";

function KalkulatorHeader() {
  return (
    <section className="hero-gradient-subtle text-white">
      <Box
        paddingBlock={{ xs: "space-16", md: "space-20" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-7xl mx-auto"
      >
        <VStack gap="space-4">
          <Heading size="large" level="1">
            Kalkulator
          </Heading>
          <BodyShort className="max-w-2xl opacity-80">
            Estimer hva dagens bruk vil koste med den nye AI Credits-faktureringsmodellen fra 1. juni 2026.{" "}
            <a href="/priser" className="underline">
              Se modellpriser →
            </a>
          </BodyShort>
        </VStack>
      </Box>
    </section>
  );
}

async function CalculatorData() {
  const token = await getUserToken();
  if (!token) {
    return <BodyShort>Ikke autentisert — kan ikke hente data</BodyShort>;
  }

  // Fetch uncached data first so Next.js marks this as dynamic before we use Date
  const [billingResult, bigqueryResult] = await Promise.all([
    getCachedCopilotBilling(token),
    getCachedBigQueryUsage(token),
  ]);

  const now = new Date();
  const currentYear = now.getFullYear();
  const currentMonth = now.getMonth() + 1;

  const premiumResult = await getCachedPremiumRequestUsageWithToken(token, "navikt", currentYear, currentMonth);

  const seats = billingResult.billing?.seat_breakdown?.total ?? 581;

  let models: ModelPremiumData[] = [];
  let currentGrossCost = 0;
  let currentNetCost = 0;
  let dataPeriodDays = 28;

  if (premiumResult.usage?.usageItems?.length) {
    const metrics = calculatePremiumMetrics(premiumResult.usage);
    models = metrics.modelBreakdown.map((m) => ({
      model: m.model,
      requests: m.requests,
      grossAmount: m.amount,
    }));
    currentGrossCost = metrics.totalGrossAmount;
    currentNetCost = metrics.totalNetAmount;

    // Estimate data period from billing time period
    const tp = premiumResult.usage.timePeriod;
    if (tp.month) {
      const daysInMonth = new Date(tp.year, tp.month, 0).getDate();
      const today = new Date();
      if (tp.year === today.getFullYear() && tp.month === today.getMonth() + 1) {
        dataPeriodDays = today.getDate();
      } else {
        dataPeriodDays = daysInMonth;
      }
    }
  }

  let cli: CLIData = { inputTokens: 0, outputTokens: 0, sessions: 0, requests: 0 };
  if (bigqueryResult.usage?.length) {
    const cliMetrics = getCLIMetrics(bigqueryResult.usage);
    if (cliMetrics) {
      cli = {
        inputTokens: cliMetrics.promptTokensSum,
        outputTokens: cliMetrics.outputTokensSum,
        sessions: cliMetrics.sessionCount,
        requests: cliMetrics.requestCount,
      };
    }
  }

  return (
    <CalculatorContent
      initialSeats={seats}
      initialModels={models}
      initialCLI={cli}
      initialDataPeriodDays={dataPeriodDays}
      currentGrossCost={currentGrossCost}
      currentNetCost={currentNetCost}
    />
  );
}

export default async function KalkulatorPage() {
  await getUser();

  return (
    <main>
      <KalkulatorHeader />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <Suspense
            fallback={
              <VStack gap="space-24">
                <HGrid columns={{ xs: 1, md: 4 }} gap="space-16">
                  <Skeleton variant="rectangle" height={100} />
                  <Skeleton variant="rectangle" height={100} />
                  <Skeleton variant="rectangle" height={100} />
                  <Skeleton variant="rectangle" height={100} />
                </HGrid>
                <Skeleton variant="rectangle" height={300} />
                <Skeleton variant="rectangle" height={400} />
              </VStack>
            }
          >
            <CalculatorData />
          </Suspense>
        </Box>
      </div>
    </main>
  );
}
