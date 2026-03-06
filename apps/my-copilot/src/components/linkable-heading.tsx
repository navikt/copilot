"use client";

import { Heading, HeadingProps } from "@navikt/ds-react";
import { LinkIcon, CheckmarkIcon } from "@navikt/aksel-icons";
import { useState } from "react";

function slugify(text: string): string {
  return text
    .toLowerCase()
    .replace(/[^a-zæøå0-9\s-]/g, "")
    .replace(/\s+/g, "-");
}

export function LinkableHeading({ children, ...props }: HeadingProps) {
  const id = typeof children === "string" ? slugify(children) : undefined;
  const [copied, setCopied] = useState(false);

  const handleClick = (e: React.MouseEvent) => {
    if (!id) return;
    e.preventDefault();
    const url = `${window.location.origin}${window.location.pathname}#${id}`;
    navigator.clipboard.writeText(url);
    window.history.replaceState(null, "", `#${id}`);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  };

  return (
    <Heading id={id} {...props}>
      {id ? (
        <a href={`#${id}`} onClick={handleClick} className="group no-underline hover:no-underline text-inherit">
          {children}
          {copied ? (
            <CheckmarkIcon
              className="inline-block ml-2 text-green-600 align-baseline link-copied-icon"
              aria-label="Lenke kopiert"
              fontSize="0.75em"
            />
          ) : (
            <LinkIcon
              className="inline-block ml-2 opacity-0 group-hover:opacity-50 transition-opacity align-baseline"
              aria-hidden
              fontSize="0.75em"
            />
          )}
        </a>
      ) : (
        children
      )}
    </Heading>
  );
}
