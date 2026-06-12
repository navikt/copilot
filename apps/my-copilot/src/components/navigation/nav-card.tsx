"use client";

import { Box, VStack, Heading } from "@navikt/ds-react";
import NextLink from "next/link";
import type { ReactNode } from "react";

interface NavCardProps {
  href: string;
  icon: ReactNode;
  title: string;
  description: string;
  external?: boolean;
}

export function NavCard({ href, icon, title, description, external = false }: NavCardProps) {
  const linkProps = external ? { target: "_blank", rel: "noopener noreferrer" } : {};

  return (
    <Box borderColor="neutral" borderWidth="1" borderRadius="8" padding="space-16" asChild>
      <NextLink href={href} {...linkProps} className="no-underline hover:shadow-md transition-shadow">
        <VStack gap="space-8">
          <Heading size="xsmall" level="3">
            <span className="flex items-center gap-2">
              {icon}
              {title}
            </span>
          </Heading>
          <span className="text-text-subtle text-sm">{description}</span>
        </VStack>
      </NextLink>
    </Box>
  );
}
