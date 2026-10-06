import { HGrid, Skeleton, VStack } from "@navikt/ds-react";

export default function TeamSpendSkeleton() {
  return (
    <div role="status" aria-label="Henter teamenes bruk og kostnad">
      <VStack gap="space-32" aria-hidden="true">
        <VStack gap="space-8">
          <Skeleton variant="text" width="35%" />
          <Skeleton variant="text" />
          <Skeleton variant="text" width="80%" />
        </VStack>
        <VStack gap="space-16">
          <Skeleton variant="text" width="40%" />
          {Array.from({ length: 7 }, (_, row) => (
            <HGrid key={row} columns="2fr 1fr 1fr 1fr 1fr" gap="space-16">
              {Array.from({ length: 5 }, (_, column) => (
                <Skeleton key={column} variant="text" />
              ))}
            </HGrid>
          ))}
        </VStack>
      </VStack>
    </div>
  );
}
