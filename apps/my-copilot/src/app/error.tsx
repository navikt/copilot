"use client";

import { captureException } from "@nais/apm";
import { useEffect } from "react";
import { Box, Button, Heading, BodyLong } from "@navikt/ds-react";

export default function Error({ error, reset }: { error: Error & { digest?: string }; reset: () => void }) {
  useEffect(() => {
    captureException(error, { context: { digest: error.digest } });
  }, [error]);

  return (
    <Box paddingBlock="space-40" paddingInline="space-16" className="max-w-2xl mx-auto text-center">
      <Heading size="large" spacing>
        Noe gikk galt
      </Heading>
      <BodyLong spacing>En uventet feil oppstod. Prøv igjen, eller kontakt oss hvis problemet vedvarer.</BodyLong>
      <Button onClick={reset}>Prøv igjen</Button>
    </Box>
  );
}
