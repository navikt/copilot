import { Heading, BodyShort, Box, VStack } from "@navikt/ds-react";

interface ErrorStateProps {
  title?: string;
  message: string;
}

export default function ErrorState({ title = "Copilot Statistikk", message }: ErrorStateProps) {
  return (
    <main className="max-w-7xl">
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
