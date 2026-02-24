import { Box, Heading, BodyShort, HelpText, VStack } from "@navikt/ds-react";

interface MetricCardProps {
  value: string | number;
  label: string;
  helpText: string;
  helpTitle: string;
  subtitle?: string;
  accentColor?: "blue" | "green" | "purple" | "orange";
}

const accentColors = {
  blue: "border-l-blue-500 text-blue-600",
  green: "border-l-green-500 text-green-600",
  purple: "border-l-purple-500 text-purple-600",
  orange: "border-l-orange-500 text-orange-600",
};

export default function MetricCard({
  value,
  label,
  helpText,
  helpTitle,
  subtitle,
  accentColor = "blue",
}: MetricCardProps) {
  const colorClass = accentColors[accentColor];
  const textColorClass = colorClass.split(" ")[1];

  return (
    <Box
      background="default"
      padding="space-20"
      borderRadius="8"
      className={`border border-gray-200 border-l-4 ${colorClass.split(" ")[0]}`}
    >
      <VStack gap="space-2">
        <div className="flex items-center">
          <BodyShort className="text-gray-600 text-sm">{label}</BodyShort>
          <HelpText title={helpTitle} placement="top">
            {helpText}
          </HelpText>
        </div>
        <Heading size="xlarge" level="2" className={textColorClass}>
          {value}
        </Heading>
        {subtitle && <BodyShort className="text-gray-500 text-sm">{subtitle}</BodyShort>}
      </VStack>
    </Box>
  );
}
