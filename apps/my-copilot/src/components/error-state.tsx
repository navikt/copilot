import { Alert, Heading, BodyShort, Box, VStack } from "@navikt/ds-react";

interface ErrorStateProps {
  title?: string;
  message: string;
}

/** Whole-page fallback, rendered instead of the page shell. Inside a page, use SectionError. */
export default function ErrorState({ title = "Kunne ikke hente dataene", message }: ErrorStateProps) {
  return (
    <main id="hovedinnhold" tabIndex={-1} className="max-w-7xl">
      <Box paddingBlock="space-12" paddingInline="space-8">
        <VStack gap="space-12">
          <Heading size="xlarge" level="1">
            {title}
          </Heading>
          <BodyShort className={message.startsWith("Feil") ? "text-red-500" : ""}>{message}</BodyShort>
        </VStack>
      </Box>
    </main>
  );
}

/** Inline error for a section of a page that already has its own headings. */
export function SectionError({ message }: { message: string }) {
  return (
    <Alert variant="error" size="small">
      {message}
    </Alert>
  );
}
