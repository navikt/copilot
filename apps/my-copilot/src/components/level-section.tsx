import { Box, Heading, BodyShort, VStack } from "@navikt/ds-react";

export type SkillLevel = "grunnleggende" | "mellom" | "avansert";

interface LevelSectionProps {
  level: SkillLevel;
  title: string;
  description: string;
  children: React.ReactNode;
}

const LEVEL_CONFIG: Record<SkillLevel, { label: string; border: string; bg: string }> = {
  grunnleggende: {
    label: "Nivå 1",
    border: "border-l-blue-300",
    bg: "bg-blue-50/50",
  },
  mellom: {
    label: "Nivå 2",
    border: "border-l-blue-500",
    bg: "bg-blue-50/70",
  },
  avansert: {
    label: "Nivå 3",
    border: "border-l-blue-700",
    bg: "bg-blue-50",
  },
};

export function LevelSection({ level, title, description, children }: LevelSectionProps) {
  const config = LEVEL_CONFIG[level];
  const id = level === "mellom" ? "mellomnivå" : level;

  return (
    <section id={id} className={`${config.bg} border-l-4 ${config.border} rounded-lg`}>
      <Box padding={{ xs: "space-16", sm: "space-20", md: "space-24" }}>
        <VStack gap="space-8">
          <div className="flex items-center gap-3">
            <span className="text-xs font-semibold uppercase tracking-wider text-blue-600 bg-white px-2 py-0.5 rounded border border-blue-200">
              {config.label}
            </span>
            <Heading size="medium" level="2">
              {title}
            </Heading>
          </div>
          <BodyShort className="text-gray-600">{description}</BodyShort>
        </VStack>
        <VStack gap={{ xs: "space-32", md: "space-40" }} className="mt-8">
          {children}
        </VStack>
      </Box>
    </section>
  );
}

interface LevelTransitionProps {
  text: string;
}

export function LevelTransition({ text }: LevelTransitionProps) {
  return (
    <div className="flex items-center gap-4 py-4">
      <div className="flex-1 h-px bg-gray-300" />
      <BodyShort size="small" className="text-gray-500 italic">
        {text}
      </BodyShort>
      <div className="flex-1 h-px bg-gray-300" />
    </div>
  );
}
