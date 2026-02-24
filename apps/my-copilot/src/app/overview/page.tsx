import { getCachedCopilotBilling } from "@/lib/cached-github";
import { Suspense } from "react";
import { Skeleton, Heading, BodyShort, Box, VStack } from "@navikt/ds-react";

function currencyFormat(num: number) {
  return `$${num.toFixed(2).replace(/(\d)(?=(\d{3})+(?!\d))/g, "$1,")} USD`;
}

// Static header component (automatically prerendered)
function OverviewHeader() {
  return (
    <VStack gap="space-8">
      <Heading size="xlarge" level="1">
        Copilot Oversikt
      </Heading>
      <BodyShort className="text-gray-600">
        Oversikt over lisenser, kostnader og organisasjonsinnstillinger for GitHub Copilot
      </BodyShort>
    </VStack>
  );
}

// Cached billing data component
async function BillingOverview() {
  "use cache";
  const { cacheLife, cacheTag } = await import("next/cache");
  cacheLife({ stale: 3600 });
  cacheTag("billing-navikt");

  const { billing, error } = await getCachedCopilotBilling("navikt");

  if (error) {
    return (
      <Box background="danger-soft" padding="space-16" borderRadius="8">
        <BodyShort className="text-red-600">Feil ved henting av faktureringsdata: {error}</BodyShort>
      </Box>
    );
  }

  if (!billing) {
    return (
      <Box background="warning-soft" padding="space-16" borderRadius="8">
        <BodyShort className="text-orange-600">Ingen faktureringsdata tilgjengelig</BodyShort>
      </Box>
    );
  }

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="medium" level="2" className="mb-4">
          Lisensfordeling
        </Heading>
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Beskrivelse
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Verdi</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Totalt antall lisenser</td>
              <td className="px-4 py-3 whitespace-nowrap font-semibold">{billing.seat_breakdown.total}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Lagt til denne perioden</td>
              <td className="px-4 py-3 whitespace-nowrap">{billing.seat_breakdown.added_this_cycle}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Ventende invitasjon</td>
              <td className="px-4 py-3 whitespace-nowrap">{billing.seat_breakdown.pending_invitation}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Ventende kansellering</td>
              <td className="px-4 py-3 whitespace-nowrap">{billing.seat_breakdown.pending_cancellation}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Aktiv denne perioden</td>
              <td className="px-4 py-3 whitespace-nowrap text-green-600 font-semibold">
                {billing.seat_breakdown.active_this_cycle}
              </td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Inaktiv denne perioden</td>
              <td className="px-4 py-3 whitespace-nowrap text-gray-500">
                {billing.seat_breakdown.inactive_this_cycle}
              </td>
            </tr>
            <tr className="bg-blue-50">
              <td className="px-4 py-3 whitespace-nowrap font-semibold">Total kostnad</td>
              <td className="px-4 py-3 whitespace-nowrap font-bold text-blue-600">
                {currencyFormat((billing.seat_breakdown.total ?? 0) * 19)} per måned
              </td>
            </tr>
          </tbody>
        </table>
      </Box>

      <Box background="neutral-soft" padding="space-24" borderRadius="12">
        <Heading size="medium" level="2" className="mb-4">
          Organisasjonsinnstillinger
        </Heading>
        <table className="min-w-full divide-y divide-gray-200">
          <thead className="bg-gray-50">
            <tr>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                Innstilling
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">Verdi</th>
            </tr>
          </thead>
          <tbody className="bg-white divide-y divide-gray-200">
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Administrasjon av lisenser</td>
              <td className="px-4 py-3 whitespace-nowrap font-semibold capitalize">
                {billing.seat_management_setting}
              </td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">IDE Chat</td>
              <td className="px-4 py-3 whitespace-nowrap capitalize">{billing.ide_chat}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Plattform Chat</td>
              <td className="px-4 py-3 whitespace-nowrap capitalize">{billing.platform_chat}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">CLI</td>
              <td className="px-4 py-3 whitespace-nowrap capitalize">{billing.cli}</td>
            </tr>
            <tr>
              <td className="px-4 py-3 whitespace-nowrap">Offentlige kodeforslag</td>
              <td className="px-4 py-3 whitespace-nowrap capitalize">{billing.public_code_suggestions}</td>
            </tr>
          </tbody>
        </table>
      </Box>
    </div>
  );
}

// Main page component using Partial Prerendering
export default function Overview() {
  return (
    <main className="max-w-7xl mx-auto">
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
      >
        {/* Static content - automatically prerendered */}
        <OverviewHeader />

        {/* Cached dynamic content - included in static shell */}
        <Suspense
          fallback={
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <Skeleton variant="rectangle" height={400} />
              <Skeleton variant="rectangle" height={400} />
            </div>
          }
        >
          <BillingOverview />
        </Suspense>
      </Box>
    </main>
  );
}
