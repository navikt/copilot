"use client";

import { Box, BodyShort, Heading, VStack } from "@navikt/ds-react";
import type { Domain, DomainConfig } from "@/lib/customization-types";
import { DOMAIN_CONFIGS } from "@/lib/customization-types";
import {
  CloudIcon,
  PaletteIcon,
  CogIcon,
  ShieldLockIcon,
  LineGraphIcon,
  BulletListIcon,
  TasklistIcon,
  PencilIcon,
} from "@navikt/aksel-icons";
import type { ReactElement } from "react";

const DOMAIN_ICONS: Record<Domain, ReactElement> = {
  platform: <CloudIcon fontSize="2rem" aria-hidden />,
  frontend: <PaletteIcon fontSize="2rem" aria-hidden />,
  backend: <CogIcon fontSize="2rem" aria-hidden />,
  auth: <ShieldLockIcon fontSize="2rem" aria-hidden />,
  observability: <LineGraphIcon fontSize="2rem" aria-hidden />,
  general: <BulletListIcon fontSize="2rem" aria-hidden />,
  testing: <TasklistIcon fontSize="2rem" aria-hidden />,
  design: <PencilIcon fontSize="2rem" aria-hidden />,
};

interface DomainCardProps {
  domain: Domain;
  count: number;
  selected: boolean;
  onClick: (domain: Domain) => void;
}

export function DomainCard({ domain, count, selected, onClick }: DomainCardProps) {
  const config: DomainConfig = DOMAIN_CONFIGS[domain];

  return (
    <button
      type="button"
      onClick={() => onClick(domain)}
      className={`text-left w-full h-full rounded-xl transition-all cursor-pointer border-2 ${
        selected ? "border-blue-500 ring-2 ring-blue-200" : "border-transparent hover:border-gray-300"
      }`}
    >
      <Box
        background={config.background}
        padding={{ xs: "space-12", md: "space-16" }}
        borderRadius="12"
        className="h-full"
      >
        <VStack gap="space-8">
          <div className="flex items-center justify-between">
            {DOMAIN_ICONS[domain]}
            <span className="text-sm font-semibold text-gray-500">{count}</span>
          </div>
          <Heading size="small" level="3">
            {config.label}
          </Heading>
          <BodyShort size="small" className="text-gray-600">
            {config.description}
          </BodyShort>
        </VStack>
      </Box>
    </button>
  );
}
