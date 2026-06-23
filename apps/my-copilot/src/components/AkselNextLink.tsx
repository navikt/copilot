"use client";
import { Link, type LinkProps } from "@navikt/ds-react";
import NextLink from "next/link";

function AkselNextLink({
  href,
  children,
  external,
  ...restProps
}: { href: string; children: React.ReactNode; external?: boolean } & LinkProps) {
  return (
    <Link
      {...restProps}
      href={href}
      as={NextLink}
      target={external ? "_blank" : undefined}
      rel={external ? "noopener noreferrer" : undefined}
    >
      {children}
    </Link>
  );
}

export { AkselNextLink };
