"use client";

import { LinkCard } from "@navikt/ds-react";
import { LinkCardAnchor, LinkCardDescription, LinkCardIcon, LinkCardTitle } from "@navikt/ds-react/LinkCard";
import NextLink from "next/link";
import type { ReactNode } from "react";

interface NavCardProps {
  href: string;
  icon: ReactNode;
  title: string;
  description: string;
  external?: boolean;
  prefetch?: boolean;
}

export function NavCard({ href, icon, title, description, external = false, prefetch }: NavCardProps) {
  const linkProps = external ? { target: "_blank", rel: "noopener noreferrer" } : {};

  return (
    <LinkCard arrow={false} size="small">
      <LinkCardIcon>{icon}</LinkCardIcon>
      <LinkCardTitle>
        <LinkCardAnchor asChild>
          <NextLink href={href} prefetch={prefetch} {...linkProps}>
            {title}
          </NextLink>
        </LinkCardAnchor>
      </LinkCardTitle>
      <LinkCardDescription>{description}</LinkCardDescription>
    </LinkCard>
  );
}
