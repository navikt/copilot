"use client";

import { useState } from "react";
import { BodyShort, Box, Heading, Search, VStack } from "@navikt/ds-react";
import type { Term } from "./terms";

export function Glossary({ terms }: { terms: Term[] }) {
  const [query, setQuery] = useState("");

  const filtered = query
    ? terms.filter(
      ({ term, definition }) =>
        term.toLowerCase().includes(query.toLowerCase()) || definition.toLowerCase().includes(query.toLowerCase())
    )
    : terms;

  return (
    <VStack gap="space-8">
      <Search
        label="Søk i ordlisten"
        variant="simple"
        value={query}
        onChange={setQuery}
        onClear={() => setQuery("")}
        size="medium"
      />

      {filtered.length === 0 ? (
        <BodyShort className="text-center opacity-60">Ingen treff for «{query}»</BodyShort>
      ) : (
        <Box as="dl" className="m-0">
          {filtered.map(({ term, definition }, i) => (
            <Box
              key={term}
              paddingBlock="space-16"
              className={i < filtered.length - 1 ? "border-b border-gray-200" : ""}
            >
              <dt>
                <Heading size="xsmall" level="2">
                  {term}
                </Heading>
              </dt>
              <dd className="m-0 mt-1">
                <BodyShort className="opacity-80">{definition}</BodyShort>
              </dd>
            </Box>
          ))}
        </Box>
      )}
    </VStack>
  );
}
