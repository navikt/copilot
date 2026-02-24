import { Heading, BodyShort, Box, Skeleton } from "@navikt/ds-react";

export default function Loading() {
  return (
    <main className="p-6 mx-4 max-w-7xl">
      <section>
        <Heading size="xlarge" level="1" className="mb-2">
          Beste Praksis og Læring
        </Heading>
        <BodyShort className="text-gray-600 mb-12">
          <Skeleton variant="text" width="60%" />
        </BodyShort>

        <div className="space-y-8">
          {Array(9)
            .fill(null)
            .map((_, i) => (
              <Box key={i} background="neutral-soft" padding="space-24" borderRadius="12">
                <Skeleton variant="text" width="40%" className="mb-4" />
                <Skeleton variant="text" width="100%" className="mb-2" />
                <Skeleton variant="text" width="95%" className="mb-6" />
                <Skeleton variant="rectangle" height={120} />
              </Box>
            ))}
        </div>
      </section>
    </main>
  );
}
