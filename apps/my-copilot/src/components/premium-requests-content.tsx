import type { PremiumMetrics } from "@/lib/billing-utils";
import { formatNumber } from "@/lib/format";
import { Box, Heading, HGrid, Label, BodyShort, VStack } from "@navikt/ds-react";

interface PremiumRequestsContentProps {
  metrics: PremiumMetrics;
}

export default function PremiumRequestsContent({ metrics }: PremiumRequestsContentProps) {
  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 2, md: 4 }} gap="space-16">
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <Label size="small" as="p" className="text-gray-500 mb-1">
            Totale forespørsler
          </Label>
          <Heading size="medium">{formatNumber(metrics.totalGrossRequests)}</Heading>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <Label size="small" as="p" className="text-gray-500 mb-1">
            Inkluderte forespørsler
          </Label>
          <Heading size="medium">{formatNumber(metrics.totalIncludedRequests)}</Heading>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <Label size="small" as="p" className="text-gray-500 mb-1">
            Fakturerte forespørsler
          </Label>
          <Heading size="medium">{formatNumber(metrics.totalBilledRequests)}</Heading>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <Label size="small" as="p" className="text-gray-500 mb-1">
            Nettokostnad
          </Label>
          <Heading size="medium">${metrics.totalNetAmount.toFixed(2)}</Heading>
        </Box>
      </HGrid>

      {metrics.modelBreakdown.length > 0 && (
        <Box>
          <Heading size="small" spacing>
            Modellfordeling
          </Heading>
          <VStack gap="space-8">
            {metrics.modelBreakdown
              .sort((a, b) => b.requests - a.requests)
              .map((model) => (
                <div
                  key={model.model}
                  className="flex items-center justify-between py-2 border-b border-gray-100 last:border-0"
                >
                  <BodyShort>{model.model}</BodyShort>
                  <div className="flex gap-6 text-right">
                    <BodyShort size="small" className="text-gray-600">
                      {formatNumber(model.requests)} forespørsler
                    </BodyShort>
                    <BodyShort size="small" className="text-gray-600 w-20">
                      ${model.amount.toFixed(2)}
                    </BodyShort>
                  </div>
                </div>
              ))}
          </VStack>
        </Box>
      )}
    </VStack>
  );
}
