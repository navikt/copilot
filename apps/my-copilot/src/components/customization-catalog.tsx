"use client";

import { useState, useMemo, useEffect, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Box, Search, HGrid, HStack, VStack, BodyShort, Chips } from "@navikt/ds-react";
import type { CustomizationType, Domain } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { CustomizationCard } from "./customization-card";
import { DetailDrawer } from "./detail-drawer";

const TYPES: CustomizationType[] = ["agent", "instruction", "prompt", "skill", "mcp"];

type SortOption = "alpha" | "most-used";

function isValidType(value: string | null): value is CustomizationType {
  return value !== null && TYPES.includes(value as CustomizationType);
}

function isValidDomain(value: string | null, domains: Domain[]): value is Domain {
  return value !== null && domains.includes(value as Domain);
}

interface CustomizationCatalogProps {
  items: EnrichedCustomization[];
}

export function CustomizationCatalog({ items }: CustomizationCatalogProps) {
  const searchParams = useSearchParams();
  const router = useRouter();

  const allDomains = useMemo(() => {
    const domains = new Set(items.map((i) => i.domain));
    return Array.from(domains) as Domain[];
  }, [items]);

  const initialType = searchParams.get("type");
  const initialDomain = searchParams.get("domain");
  const initialSearch = searchParams.get("q") ?? "";
  const initialItem = searchParams.get("item");

  const [search, setSearch] = useState(initialSearch);
  const [selectedType, setSelectedType] = useState<CustomizationType | null>(
    isValidType(initialType) ? initialType : null
  );
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(
    isValidDomain(initialDomain, allDomains) ? initialDomain : null
  );
  const [selectedItem, setSelectedItem] = useState<EnrichedCustomization | null>(() => {
    if (initialItem) {
      return items.find((i) => i.id === initialItem) ?? null;
    }
    return null;
  });
  const [sortBy, setSortBy] = useState<SortOption>("alpha");

  const isInitialRender = useRef(true);
  useEffect(() => {
    if (isInitialRender.current) {
      isInitialRender.current = false;
      return;
    }
    const params = new URLSearchParams();
    if (selectedType) params.set("type", selectedType);
    if (selectedDomain) params.set("domain", selectedDomain);
    if (search) params.set("q", search);
    if (selectedItem) params.set("item", selectedItem.id);
    const qs = params.toString();
    router.replace(qs ? `?${qs}` : "/verktoy", { scroll: false });
  }, [selectedType, selectedDomain, search, selectedItem, router]);

  useEffect(() => {
    const handler = (e: Event) => {
      const domain = (e as CustomEvent<Domain>).detail;
      setSelectedDomain((prev) => (prev === domain ? null : domain));
    };
    window.addEventListener("domain-filter", handler);
    return () => window.removeEventListener("domain-filter", handler);
  }, []);

  const filtered = useMemo(() => {
    const result = items.filter((item) => {
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

    if (sortBy === "most-used") {
      return result.sort((a, b) => b.usageCount - a.usageCount || a.name.localeCompare(b.name, "nb"));
    }
    return result.sort((a, b) => a.name.localeCompare(b.name, "nb"));
  }, [items, search, selectedType, selectedDomain, sortBy]);

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
          {allDomains.map((domain) => (
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

      <HStack gap="space-8" align="center" justify="space-between" wrap>
        <HStack gap="space-8" align="center">
          <BodyShort size="small" className="text-gray-500">
            Sorter:
          </BodyShort>
          <Chips size="small">
            <Chips.Toggle selected={sortBy === "alpha"} onClick={() => setSortBy("alpha")}>
              A–Å
            </Chips.Toggle>
            <Chips.Toggle selected={sortBy === "most-used"} onClick={() => setSortBy("most-used")}>
              Mest brukt
            </Chips.Toggle>
          </Chips>
        </HStack>
        <BodyShort size="small" className="text-gray-500">
          {filtered.length} av {items.length} tilpasninger
        </BodyShort>
      </HStack>

      <HGrid columns={{ xs: 1, md: 2 }} gap="space-16">
        {filtered.map((item) => (
          <CustomizationCard key={`${item.type}-${item.id}`} item={item} onClick={() => setSelectedItem(item)} />
        ))}
      </HGrid>

      {filtered.length === 0 && (
        <Box padding="space-24" className="text-center">
          <BodyShort className="text-gray-500">Ingen tilpasninger matcher søket ditt.</BodyShort>
        </Box>
      )}

      <DetailDrawer item={selectedItem} open={selectedItem !== null} onClose={() => setSelectedItem(null)} />
    </VStack>
  );
}
