import { Box, Heading, BodyShort, HelpText, VStack } from "@navikt/ds-react";

interface MetricCardProps {
  value: string | number;
  label: string;
  helpText: string;
  helpTitle: string;
  subtitle?: string;
}

export default function MetricCard({ value, label, helpText, helpTitle, subtitle }: MetricCardProps) {
  const isLongText = typeof value === "string" && value.length > 6;

  return (
    <Box background="default" padding="space-20" borderRadius="8" className="border border-gray-200">
      <VStack gap="space-2">
        <div className="flex items-center">
          <BodyShort className="text-gray-600 text-sm">{label}</BodyShort>
          <HelpText title={helpTitle} placement="top">
            {helpText}
          </HelpText>
        </div>
        <Heading size={isLongText ? "medium" : "xlarge"} level="2" className="break-all">
          {value}
        </Heading>
        {subtitle && <BodyShort className="text-gray-500 text-sm">{subtitle}</BodyShort>}
      </VStack>
    </Box>
  );
}
