"use client";

import { useEffect, useState } from "react";
import { BodyShort, Box, Button, Heading, Link, Search, UNSAFE_Combobox, VStack } from "@navikt/ds-react";
import { termId } from "./term-graph";
import { categories, type Category, type Term } from "./terms";

const categoryLabels = Object.fromEntries(categories.map((c) => [c.id, c.label])) as Record<Category, string>;

/** Clears search and category filters, so a term can be scrolled to. */
export const GLOSSARY_RESET_EVENT = "ordbok:nullstill-filter";

export function Glossary({ terms }: { terms: Term[] }) {
  const [query, setQuery] = useState("");
  const [selectedCategories, setSelectedCategories] = useState<Category[]>([]);
  useEffect(() => {
    const reset = () => {
      setQuery("");
      setSelectedCategories([]);
    };
    window.addEventListener(GLOSSARY_RESET_EVENT, reset);
    return () => window.removeEventListener(GLOSSARY_RESET_EVENT, reset);
  }, []);

  const categoryOptions = categories.map(({ id, label }) => ({
    value: id,
    label: `${label} (${terms.filter((t) => t.category === id).length})`,
  }));

  const normalizedQuery = query.toLowerCase();
  const filtered = terms.filter(({ term, definition, category }) => {
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
              const category = value as Category;
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
              id={termId(term)}
              tabIndex={-1}
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
