"use client";

import { useState, useMemo, useEffect } from "react";
import { Box, Search, HStack, VStack, BodyShort, Chips } from "@navikt/ds-react";
import type { AnyCustomization, CustomizationType, Domain } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import { CustomizationCard } from "./customization-card";
import { DetailDrawer } from "./detail-drawer";

const TYPES: CustomizationType[] = ["agent", "instruction", "prompt", "skill", "mcp"];

interface CustomizationCatalogProps {
  items: AnyCustomization[];
}

export function CustomizationCatalog({ items }: CustomizationCatalogProps) {
  const [search, setSearch] = useState("");
  const [selectedType, setSelectedType] = useState<CustomizationType | null>(null);
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(null);
  const [selectedItem, setSelectedItem] = useState<AnyCustomization | null>(null);

  useEffect(() => {
    const handler = (e: Event) => {
      const domain = (e as CustomEvent<Domain>).detail;
      setSelectedDomain((prev) => (prev === domain ? null : domain));
    };
    window.addEventListener("domain-filter", handler);
    return () => window.removeEventListener("domain-filter", handler);
  }, []);

  const filtered = useMemo(() => {
    return items.filter((item) => {
      if (selectedType && item.type !== selectedType) return false;
      if (selectedDomain && item.domain !== selectedDomain) return false;
      if (search) {
        const q = search.toLowerCase();
        return (
          item.name.toLowerCase().includes(q) ||
          item.description.toLowerCase().includes(q) ||
          item.domain.toLowerCase().includes(q)
        );
      }
      return true;
    });
  }, [items, search, selectedType, selectedDomain]);

  const activeDomains = useMemo(() => {
    const domains = new Set(items.map((i) => i.domain));
    return Array.from(domains) as Domain[];
  }, [items]);

  return (
    <VStack gap="space-16">
      <Box>
        <Search
          label="Søk i tilpasninger"
          hideLabel
          variant="simple"
          placeholder="Søk etter navn, beskrivelse..."
          value={search}
          onChange={setSearch}
          onClear={() => setSearch("")}
        />
      </Box>

      <HStack gap="space-8" wrap>
        <Chips>
          <Chips.Toggle selected={selectedType === null} onClick={() => setSelectedType(null)}>
            Alle typer
          </Chips.Toggle>
          {TYPES.map((type) => (
            <Chips.Toggle
              key={type}
              selected={selectedType === type}
              onClick={() => setSelectedType(selectedType === type ? null : type)}
            >
              {TYPE_LABELS[type]}
            </Chips.Toggle>
          ))}
        </Chips>
      </HStack>

      <HStack gap="space-8" wrap>
        <Chips>
          <Chips.Toggle selected={selectedDomain === null} onClick={() => setSelectedDomain(null)}>
            Alle domener
          </Chips.Toggle>
          {activeDomains.map((domain) => (
            <Chips.Toggle
              key={domain}
              selected={selectedDomain === domain}
              onClick={() => setSelectedDomain(selectedDomain === domain ? null : domain)}
            >
              {DOMAIN_CONFIGS[domain].label}
            </Chips.Toggle>
          ))}
        </Chips>
      </HStack>

      <BodyShort size="small" className="text-gray-500">
        {filtered.length} av {items.length} tilpasninger
      </BodyShort>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {filtered.map((item) => (
          <CustomizationCard key={`${item.type}-${item.id}`} item={item} onClick={() => setSelectedItem(item)} />
        ))}
      </div>

      {filtered.length === 0 && (
        <Box padding="space-24" className="text-center">
          <BodyShort className="text-gray-500">Ingen tilpasninger matcher søket ditt.</BodyShort>
        </Box>
      )}

      <DetailDrawer item={selectedItem} open={selectedItem !== null} onClose={() => setSelectedItem(null)} />
    </VStack>
  );
}
