import type { PremiumMetrics } from "@/lib/billing-utils";
import { formatNumber } from "@/lib/format";
import { Box, Heading, HGrid, HStack, Label, BodyShort, VStack } from "@navikt/ds-react";

interface PremiumRequestsContentProps {
  metrics: PremiumMetrics;
}

export default function PremiumRequestsContent({ metrics }: PremiumRequestsContentProps) {
  return (
    <VStack gap="space-24">
      <HGrid columns={{ xs: 2, md: 4 }} gap="space-16">
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <VStack gap="space-4">
            <Label size="small" as="p" className="text-gray-500">
              Totale forespørsler
            </Label>
            <Heading size="medium">{formatNumber(metrics.totalGrossRequests)}</Heading>
          </VStack>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <VStack gap="space-4">
            <Label size="small" as="p" className="text-gray-500">
              Inkluderte forespørsler
            </Label>
            <Heading size="medium">{formatNumber(metrics.totalIncludedRequests)}</Heading>
          </VStack>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <VStack gap="space-4">
            <Label size="small" as="p" className="text-gray-500">
              Fakturerte forespørsler
            </Label>
            <Heading size="medium">{formatNumber(metrics.totalBilledRequests)}</Heading>
          </VStack>
        </Box>
        <Box padding="space-16" background="neutral-soft" borderRadius="8">
          <VStack gap="space-4">
            <Label size="small" as="p" className="text-gray-500">
              Nettokostnad
            </Label>
            <Heading size="medium">${metrics.totalNetAmount.toFixed(2)}</Heading>
          </VStack>
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
                <Box key={model.model} paddingBlock="space-8" className="border-b border-gray-100 last:border-0">
                  <HStack justify="space-between" align="center">
                    <BodyShort>{model.model}</BodyShort>
                    <HStack gap="space-24" align="center">
                      <BodyShort size="small" className="text-gray-600">
                        {formatNumber(model.requests)} forespørsler
                      </BodyShort>
                      <BodyShort size="small" className="text-gray-600" style={{ width: "5rem", textAlign: "right" }}>
                        ${model.amount.toFixed(2)}
                      </BodyShort>
                    </HStack>
                  </HStack>
                </Box>
              ))}
          </VStack>
        </Box>
      )}
    </VStack>
  );
}
