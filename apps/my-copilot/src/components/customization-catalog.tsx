"use client";

import { useState, useMemo, useEffect, useRef, useId } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Box, Button, Search, HGrid, HStack, VStack, BodyShort, Chips, UNSAFE_Combobox } from "@navikt/ds-react";
import type { CustomizationType, Domain, CollectionId } from "@/lib/customization-types";
import { TYPE_LABELS, COLLECTION_CONFIGS } from "@/lib/customization-types";
import { VALID_COLLECTIONS } from "@/lib/manifest-types";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { CustomizationCard } from "./customization-card";
import { DetailDrawer } from "./detail-drawer";

const TYPES: CustomizationType[] = ["agent", "instruction", "prompt", "skill", "mcp"];

type SortOption = "alpha" | "most-used";

function parseTypes(value: string | null): CustomizationType[] {
  if (!value) return [];
  return value.split(",").filter((v): v is CustomizationType => TYPES.includes(v as CustomizationType));
}

function parseCollections(value: string | null): CollectionId[] {
  if (!value) return [];
  return value.split(",").filter((v): v is CollectionId => (VALID_COLLECTIONS as readonly string[]).includes(v));
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

  const initialDomain = searchParams.get("domain");
  const initialSearch = searchParams.get("q") ?? "";
  const initialItem = searchParams.get("item");

  const [search, setSearch] = useState(initialSearch);
  const [selectedTypes, setSelectedTypes] = useState<CustomizationType[]>(parseTypes(searchParams.get("type")));
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(
    isValidDomain(initialDomain, allDomains) ? initialDomain : null
  );
  const [selectedCollections, setSelectedCollections] = useState<CollectionId[]>(
    parseCollections(searchParams.get("collection"))
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
    if (selectedTypes.length > 0) params.set("type", selectedTypes.join(","));
    if (selectedDomain) params.set("domain", selectedDomain);
    if (selectedCollections.length > 0) params.set("collection", selectedCollections.join(","));
    if (search) params.set("q", search);
    if (selectedItem) params.set("item", selectedItem.id);
    const qs = params.toString();
    router.replace(qs ? `?${qs}` : "/verktoy", { scroll: false });
  }, [selectedTypes, selectedDomain, selectedCollections, search, selectedItem, router]);

  useEffect(() => {
    const handler = (e: Event) => {
      const domain = (e as CustomEvent<Domain>).detail;
      setSelectedDomain((prev) => (prev === domain ? null : domain));
    };
    window.addEventListener("domain-filter", handler);
    return () => window.removeEventListener("domain-filter", handler);
  }, []);

  const nonDeprecatedCount = useMemo(() => items.filter((i) => !i.deprecated).length, [items]);

  const filtered = useMemo(() => {
    const result = items.filter((item) => {
      if (item.deprecated) return false;
      if (selectedTypes.length > 0 && !selectedTypes.includes(item.type)) return false;
      if (selectedDomain && item.domain !== selectedDomain) return false;
      if (selectedCollections.length > 0 && !item.collections?.some((c) => selectedCollections.includes(c)))
        return false;
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
  }, [items, search, selectedTypes, selectedDomain, selectedCollections, sortBy]);

  const typeId = useId();
  const collectionId = useId();

  const typeOptions = useMemo(() => TYPES.map((type) => ({ label: TYPE_LABELS[type], value: type })), []);
  const collectionOptions = useMemo(
    () => VALID_COLLECTIONS.map((c) => ({ label: COLLECTION_CONFIGS[c].label, value: c })),
    []
  );

  const hasActiveFilters = selectedTypes.length > 0 || !!selectedDomain || selectedCollections.length > 0 || !!search;

  function resetFilters() {
    setSelectedTypes([]);
    setSelectedDomain(null);
    setSelectedCollections([]);
    setSearch("");
  }

  return (
    <VStack gap="space-16">
      <div className="flex gap-4 items-end">
        <div className="w-1/2">
          <Search
            label="Søk i tilpasninger"
            hideLabel
            variant="simple"
            placeholder="Søk etter navn, beskrivelse..."
            value={search}
            onChange={setSearch}
            onClear={() => setSearch("")}
          />
        </div>
        <div className="w-1/4">
          <UNSAFE_Combobox
            id={typeId}
            label="Type"
            isMultiSelect
            options={typeOptions}
            selectedOptions={typeOptions.filter((o) => selectedTypes.includes(o.value))}
            onToggleSelected={(value, isSelected) => {
              const type = value as CustomizationType;
              setSelectedTypes((prev) => (isSelected ? [...prev, type] : prev.filter((t) => t !== type)));
            }}
            placeholder="Alle typer"
          />
        </div>
        <div className="w-1/4">
          <UNSAFE_Combobox
            id={collectionId}
            label="Samling"
            isMultiSelect
            options={collectionOptions}
            selectedOptions={collectionOptions.filter((o) => selectedCollections.includes(o.value as CollectionId))}
            onToggleSelected={(value, isSelected) => {
              const col = value as CollectionId;
              setSelectedCollections((prev) => (isSelected ? [...prev, col] : prev.filter((c) => c !== col)));
            }}
            placeholder="Alle samlinger"
          />
        </div>
        {hasActiveFilters && (
          <Button variant="tertiary-neutral" size="small" onClick={resetFilters}>
            Nullstill filtre
          </Button>
        )}
      </div>

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
          {filtered.length} av {nonDeprecatedCount} tilpasninger
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

      <DetailDrawer
        item={selectedItem}
        allItems={items}
        open={selectedItem !== null}
        onClose={() => setSelectedItem(null)}
        onNavigate={setSelectedItem}
      />
    </VStack>
  );
}
