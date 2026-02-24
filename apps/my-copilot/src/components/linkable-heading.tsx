"use client";

import { Heading, HeadingProps } from "@navikt/ds-react";
import { LinkIcon } from "@navikt/aksel-icons";

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-zæøå0-9\s-]/g, "")
    .replace(/\s+/g, "-");
}

export function LinkableHeading({ children, ...props }: HeadingProps) {
  const id = typeof children === "string" ? slugify(children) : undefined;

  return (
    <Heading id={id} {...props}>
      {id ? (
        <a href={`#${id}`} className="group no-underline hover:no-underline text-inherit">
          {children}
          <LinkIcon
            className="inline-block ml-2 opacity-0 group-hover:opacity-50 transition-opacity align-baseline"
            aria-hidden
            fontSize="0.75em"
          />
        </a>
      ) : (
        children
      )}
    </Heading>
  );
}
