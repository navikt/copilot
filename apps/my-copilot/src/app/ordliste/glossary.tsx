"use client";

import { useState } from "react";
import { BodyShort, Box, Button, Heading, Link, Search, UNSAFE_Combobox, VStack } from "@navikt/ds-react";
import type { Term } from "./terms";

type CategoryId = "agentisk" | "copilot" | "sikkerhet" | "plattform" | "grunnbegreper";

const categoryLabels: Record<CategoryId, string> = {
  agentisk: "Agentisk KI",
  copilot: "Copilot-funksjoner",
  sikkerhet: "Sikkerhet og styring",
  plattform: "Plattform og integrasjon",
  grunnbegreper: "Grunnbegreper",
};
const categoryOrder: CategoryId[] = ["agentisk", "copilot", "sikkerhet", "plattform", "grunnbegreper"];

function getCategory(term: Term): CategoryId {
  const name = term.term.toLowerCase();

  if (
    name.includes("agent") ||
    name === "agency" ||
    name === "autonomi" ||
    name === "human-in-the-loop" ||
    name === "tool calling" ||
    name === "subagent"
  ) {
    return "agentisk";
  }

  if (
    name.startsWith("copilot") ||
    name === "ask mode" ||
    name === "edit mode" ||
    name === "plan mode" ||
    name === "chat" ||
    name === "hooks" ||
    name === "inline suggestion" ||
    name === "next edit suggestions (nes)" ||
    name === "skills" ||
    name === "instructions"
  ) {
    return "copilot";
  }

  if (
    name === "allowlist (mcp)" ||
    name === "context exclusion" ||
    name === "inference context" ||
    name === "org policy" ||
    name === "sandbox (cplt)" ||
    name === "prompt injection" ||
    name === "excessive agency"
  ) {
    return "sikkerhet";
  }

  if (name.includes("mcp") || name === "opencode" || name === "agents.md" || name === "model provider") {
    return "plattform";
  }

  return "grunnbegreper";
}

export function Glossary({ terms }: { terms: Term[] }) {
  const [query, setQuery] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<CategoryId[]>([]);

  const termsWithCategory = terms.map((term) => ({ ...term, category: getCategory(term) }));
  const categoryCounts = termsWithCategory.reduce<Record<CategoryId, number>>(
    (acc, term) => {
      acc[term.category] += 1;
      return acc;
    },
    { agentisk: 0, copilot: 0, sikkerhet: 0, plattform: 0, grunnbegreper: 0 }
  );
  const categoryOptions = categoryOrder.map((id) => ({
    value: id,
    label: `${categoryLabels[id]} (${categoryCounts[id]})`,
  }));

  const normalizedQuery = query.toLowerCase();
  const filtered = termsWithCategory.filter(({ term, definition, category }) => {
    const matchesQuery =
      normalizedQuery.length === 0 ||
      term.toLowerCase().includes(normalizedQuery) ||
      definition.toLowerCase().includes(normalizedQuery);
    const matchesCategory = selectedCategories.length === 0 || selectedCategories.includes(category);
    return matchesQuery && matchesCategory;
  });

  return (
    <VStack gap="space-16">
      <div className="flex flex-col gap-4 md:flex-row md:items-end">
        <div className="w-full md:w-2/3">
          <Search
            label="Søk i ordlisten"
            hideLabel
            variant="simple"
            placeholder="Søk etter begrep eller definisjon..."
            value={query}
            onChange={setQuery}
            onClear={() => setQuery("")}
            size="medium"
          />
        </div>
        <div className="w-full md:w-1/3 [&_.navds-combobox__selected-options]:hidden [&_.aksel-combobox__selected-options]:hidden">
          <UNSAFE_Combobox
            id="ordbok-kategori-filter"
            label="Kategori"
            isMultiSelect
            options={categoryOptions}
            selectedOptions={categoryOptions.filter((option) => selectedCategories.includes(option.value))}
            onToggleSelected={(value, isSelected) => {
              const category = value as CategoryId;
              setSelectedCategories((prev) =>
                isSelected
                  ? prev.includes(category)
                    ? prev
                    : [...prev, category]
                  : prev.filter((item) => item !== category)
              );
            }}
            placeholder={selectedCategories.length > 0 ? `${selectedCategories.length} valgt` : "Alle kategorier"}
          />
        </div>
      </div>
      <Box className="flex items-center justify-between">
        <BodyShort size="small" className="opacity-70">
          {filtered.length} av {terms.length} begreper
        </BodyShort>
        {selectedCategories.length > 0 && (
          <Button size="small" variant="tertiary-neutral" onClick={() => setSelectedCategories([])}>
            Nullstill filtre
          </Button>
        )}
      </Box>

      {filtered.length === 0 ? (
        <BodyShort className="text-center opacity-60">Ingen treff for «{query}»</BodyShort>
      ) : (
        <Box as="dl" className="m-0">
          {filtered.map(({ term, definition, link, category }, i) => (
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
                <BodyShort size="small" className="mt-1 opacity-70">
                  Kategori: {categoryLabels[category]}
                </BodyShort>
                {link && (
                  <Link href={link.href} className="mt-1 inline-block text-sm">
                    {link.label} →
                  </Link>
                )}
              </dd>
            </Box>
          ))}
        </Box>
      )}
    </VStack>
  );
}
