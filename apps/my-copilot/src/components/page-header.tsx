import { Heading, BodyShort, VStack } from "@navikt/ds-react";
import type { ReactNode } from "react";

interface PageHeaderProps {
  title: string;
  description: string;
  actions?: ReactNode;
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
      <VStack gap="space-8">
        <Heading size="xlarge" level="1">
          {title}
        </Heading>
        <BodyShort className="max-w-2xl">{description}</BodyShort>
      </VStack>
      {actions && <div className="shrink-0">{actions}</div>}
    </div>
  );
}
