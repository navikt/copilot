"use client";

import { useState, useMemo, useEffect, useRef } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import NextLink from "next/link";
import {
  BodyShort,
  Box,
  Button,
  Chips,
  Heading,
  HGrid,
  HStack,
  Search,
  Tag,
  ToggleGroup,
  VStack,
} from "@navikt/ds-react";
import type { CustomizationType, Domain } from "@/lib/customization-types";
import { DOMAIN_CONFIGS, TYPE_LABELS } from "@/lib/customization-types";
import { findByItemKey, itemKey } from "@/lib/item-key";
import type { EnrichedCustomization } from "@/lib/enrich-customizations";
import { linkClass } from "@/components/nav-pilot/doc-page";
import { DetailDrawer } from "./detail-drawer";
import { TYPE_ORDER, TypeBadge, TypeIcon, TypeIconBadge, typeClassName } from "./verktoy-type-meta";

type SortOption = "most-used" | "alpha";

const DOMAIN_ORDER: Domain[] = [
  "platform",
  "frontend",
  "backend",
  "auth",
  "observability",
  "general",
  "testing",
  "design",
];

const STATIC_PLACEHOLDER = "Prøv «deploy til Nais», «Aksel-skjema» eller «testpersoner»";
const FOCUSED_PLACEHOLDER = "Søk etter verktøy";
const PLACEHOLDER_EXAMPLES = [
  "deploy til Nais",
  "Aksel-skjema med validering",
  "testpersoner fra Dolly",
  "trusselmodell for ny app",
  "kodeverk for postnummer",
];

const POPULAR_THRESHOLD = 20;

interface IntentOption {
  label: string;
  query?: string;
  type?: CustomizationType;
}

const INTENTS: IntentOption[] = [
  { label: "Bygge frontend med Aksel", query: "aksel" },
  { label: "Deploye til Nais", query: "nais" },
  { label: "Sikre en app", query: "sikker" },
  { label: "Teste og kvalitetssikre", query: "test" },
  { label: "Hente data fra Nav-tjenester", type: "mcp" },
];

function parseTypes(value: string | null): CustomizationType[] {
  if (!value) return [];
  return value.split(",").filter((v): v is CustomizationType => TYPE_ORDER.includes(v as CustomizationType));
}

function isValidDomain(value: string | null, domains: Domain[]): value is Domain {
  return value !== null && domains.includes(value as Domain);
}

function displayName(item: EnrichedCustomization): string {
  return item.type === "agent" ? `@${item.name}` : item.name;
}

function sortItems(list: EnrichedCustomization[], sortBy: SortOption): EnrichedCustomization[] {
  const copy = [...list];
  if (sortBy === "most-used") {
    return copy.sort((a, b) => b.usageCount - a.usageCount || a.name.localeCompare(b.name, "nb"));
  }
  return copy.sort((a, b) => a.name.localeCompare(b.name, "nb"));
}

interface CustomizationCatalogProps {
  items: EnrichedCustomization[];
}

export function CustomizationCatalog({ items }: CustomizationCatalogProps) {
  const searchParams = useSearchParams();
  const router = useRouter();

  const activeItems = useMemo(() => items.filter((i) => !i.deprecated), [items]);

  const allDomains = useMemo(() => {
    const domains = new Set(activeItems.map((i) => i.domain));
    return DOMAIN_ORDER.filter((d) => domains.has(d));
  }, [activeItems]);

  const initialDomain = searchParams.get("domain");
  const initialSearch = searchParams.get("q") ?? "";
  const initialItem = searchParams.get("item");

  const [search, setSearch] = useState(initialSearch);
  const [selectedTypes, setSelectedTypes] = useState<CustomizationType[]>(parseTypes(searchParams.get("type")));
  const [selectedDomain, setSelectedDomain] = useState<Domain | null>(
    isValidDomain(initialDomain, allDomains) ? initialDomain : null
  );
  const [selectedItem, setSelectedItem] = useState<EnrichedCustomization | null>(() => {
    if (initialItem) {
      return findByItemKey(items, initialItem) ?? null;
    }
    return null;
  });
  const [sortBy, setSortBy] = useState<SortOption>("most-used");
  const [searchFocused, setSearchFocused] = useState(false);
  const [animatedPlaceholder, setAnimatedPlaceholder] = useState(STATIC_PLACEHOLDER);

  // Sync filter state to URL params (?domain=&type=&q=&item=).
  const isInitialRender = useRef(true);
  useEffect(() => {
    if (isInitialRender.current) {
      isInitialRender.current = false;
      return;
    }
    const params = new URLSearchParams();
    if (selectedTypes.length > 0) params.set("type", selectedTypes.join(","));
    if (selectedDomain) params.set("domain", selectedDomain);
    if (search) params.set("q", search);
    if (selectedItem) params.set("item", itemKey(selectedItem));
    const qs = params.toString();
    router.replace(qs ? `?${qs}` : "/verktoy", { scroll: false });
  }, [selectedTypes, selectedDomain, search, selectedItem, router]);

  // Typing placeholder animation; disabled when prefers-reduced-motion: reduce, focused, or non-empty.
  useEffect(() => {
    if (search || searchFocused) return;
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") return;
    if (window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;

    let timer: ReturnType<typeof setTimeout> | undefined;
    let exampleIndex = 0;
    let charIndex = 0;
    let deleting = false;

    const step = () => {
      const current = PLACEHOLDER_EXAMPLES[exampleIndex];
      charIndex += deleting ? -1 : 1;
      setAnimatedPlaceholder(`Prøv «${current.slice(0, charIndex)}»`);

      if (!deleting && charIndex === current.length) {
        deleting = true;
        timer = setTimeout(step, 1600);
        return;
      }
      if (deleting && charIndex === 0) {
        deleting = false;
        exampleIndex = (exampleIndex + 1) % PLACEHOLDER_EXAMPLES.length;
      }
      timer = setTimeout(step, deleting ? 30 : 70);
    };

    timer = setTimeout(step, 400);
    return () => {
      if (timer) clearTimeout(timer);
    };
  }, [search, searchFocused]);

  const placeholder = searchFocused ? FOCUSED_PLACEHOLDER : animatedPlaceholder;

  const typeCounts = useMemo(() => {
    const counts: Record<CustomizationType, number> = {
      agent: 0,
      instruction: 0,
      skill: 0,
      mcp: 0,
      prompt: 0,
    };
    for (const item of activeItems) {
      counts[item.type]++;
    }
    return counts;
  }, [activeItems]);

  const groupedByDomain = useMemo(() => {
    return allDomains.map((domain) => {
      const domainItems = sortItems(
        activeItems.filter((item) => item.domain === domain),
        "most-used"
      );
      return {
        domain,
        total: domainItems.length,
        preview: domainItems.slice(0, 4),
      };
    });
  }, [activeItems, allDomains]);

  const filtered = useMemo(() => {
    const result = activeItems.filter((item) => {
      if (selectedTypes.length > 0 && !selectedTypes.includes(item.type)) return false;
      if (selectedDomain && item.domain !== selectedDomain) return false;
      if (search) {
        const q = search.toLowerCase();
        const domainLabel = DOMAIN_CONFIGS[item.domain].label.toLowerCase();
        return (
          item.name.toLowerCase().includes(q) ||
          item.description.toLowerCase().includes(q) ||
          item.domain.toLowerCase().includes(q) ||
          domainLabel.includes(q) ||
          (item.tags?.some((t) => t.toLowerCase().includes(q)) ?? false)
        );
      }
      return true;
    });
    return sortItems(result, sortBy);
  }, [activeItems, search, selectedTypes, selectedDomain, sortBy]);

  const hasActiveFilters = selectedTypes.length > 0 || selectedDomain !== null || search.length > 0;
  const showMcpGatewayInfo = selectedTypes.includes("mcp");

  function toggleType(type: CustomizationType) {
    setSelectedTypes((prev) => (prev.includes(type) ? prev.filter((t) => t !== type) : [...prev, type]));
  }

  function handleIntentClick(intent: IntentOption) {
    if (intent.type) {
      toggleType(intent.type);
      return;
    }
    if (intent.query) {
      setSearch((prev) => (prev.toLowerCase() === intent.query ? "" : intent.query!));
    }
  }

  function isIntentActive(intent: IntentOption): boolean {
    if (intent.type) return selectedTypes.includes(intent.type);
    if (intent.query) return search.toLowerCase() === intent.query;
    return false;
  }

  function resetFilters() {
    setSelectedTypes([]);
    setSelectedDomain(null);
    setSearch("");
  }

  return (
    <VStack gap="space-24">
      <VStack gap="space-16" align="center" className="verktoy-hero">
        <VStack gap="space-4" align="center">
          <BodyShort size="small" weight="semibold" className="page-kicker">
            Tilpasning
          </BodyShort>
          <Heading level="1" size="xlarge">
            Hva skal Copilot hjelpe deg med?
          </Heading>
        </VStack>

        <div className="verktoy-hero-search">
          <Search
            label="Søk i verktøykatalogen"
            hideLabel
            variant="simple"
            placeholder={placeholder}
            value={search}
            onChange={setSearch}
            onClear={() => setSearch("")}
            onFocus={() => setSearchFocused(true)}
            onBlur={() => setSearchFocused(false)}
          />
        </div>

        <Chips size="small" className="justify-center">
          {INTENTS.map((intent) => (
            <Chips.Toggle
              key={intent.label}
              checkmark={false}
              selected={isIntentActive(intent)}
              onClick={() => handleIntentClick(intent)}
              className="verktoy-intent-item"
            >
              {intent.label}
            </Chips.Toggle>
          ))}
        </Chips>

        <div role="group" aria-label="Filtrer på type">
          <Chips size="small" className="justify-center">
            {TYPE_ORDER.map((type) => {
              const selected = selectedTypes.includes(type);
              return (
                <button
                  key={type}
                  type="button"
                  aria-pressed={selected}
                  onClick={() => toggleType(type)}
                  className={`aksel-chips__chip aksel-chips__toggle verktoy-type-chip ${typeClassName(type)}`}
                >
                  <HStack as="span" gap="space-6" align="center">
                    <TypeIcon type={type} size="1rem" />
                    <span>{TYPE_LABELS[type]}</span>
                    <span className="verktoy-chip-count">{typeCounts[type]}</span>
                  </HStack>
                </button>
              );
            })}
          </Chips>
        </div>
      </VStack>

      {showMcpGatewayInfo && (
        <Box background="info-soft" borderRadius="8" paddingBlock="space-12" paddingInline="space-16">
          <BodyShort size="small">
            Bruke en MCP-server fra et Nav-team?{" "}
            <NextLink href="/nav-pilot/guider/nav-mcp-servere" className={linkClass}>
              Koble til Navs MCP-servere
            </NextLink>
            . Eier teamet en MCP-server?{" "}
            <NextLink href="/verktoy/fleet-gateway" className={linkClass}>
              Tilby den gjennom fleet-gateway
            </NextLink>
            .
          </BodyShort>
        </Box>
      )}

      {hasActiveFilters ? (
        <VStack gap="space-16">
          <HStack gap="space-12" align="center" justify="space-between" wrap>
            <HStack gap="space-12" align="center" wrap>
              <BodyShort size="small" className="verktoy-subtle">
                {filtered.length} av {activeItems.length} tilpasninger
              </BodyShort>
              {selectedDomain && (
                <Chips size="small">
                  <Chips.Removable onClick={() => setSelectedDomain(null)}>
                    {DOMAIN_CONFIGS[selectedDomain].label}
                  </Chips.Removable>
                </Chips>
              )}
              <ToggleGroup
                size="small"
                value={sortBy}
                onChange={(value) => setSortBy(value as SortOption)}
                aria-label="Sorter tilpasninger"
              >
                <ToggleGroup.Item value="most-used" label="Mest brukt" />
                <ToggleGroup.Item value="alpha" label="A–Å" />
              </ToggleGroup>
            </HStack>

            <Button variant="tertiary-neutral" size="small" onClick={resetFilters}>
              Nullstill filtre
            </Button>
          </HStack>

          {filtered.length > 0 ? (
            <VStack gap="space-8">
              {filtered.map((item) => (
                <button
                  key={itemKey(item)}
                  type="button"
                  className="verktoy-card"
                  onClick={() => setSelectedItem(item)}
                >
                  <HStack gap="space-12" align="center" justify="space-between" wrap={false}>
                    <HStack gap="space-12" align="center" className="min-w-0 flex-1" wrap={false}>
                      <TypeIconBadge type={item.type} size="md" />
                      <VStack gap="space-2" className="min-w-0 flex-1">
                        <HStack gap="space-8" align="center" wrap>
                          <Heading level="3" size="xsmall" className="truncate">
                            {displayName(item)}
                          </Heading>
                          {item.usageCount >= POPULAR_THRESHOLD && (
                            <Tag size="xsmall" variant="warning">
                              Populær
                            </Tag>
                          )}
                        </HStack>
                        <BodyShort size="small" className="verktoy-subtle line-clamp-2">
                          {item.description}
                        </BodyShort>
                      </VStack>
                    </HStack>

                    <HStack gap="space-6" align="center" className="hidden sm:flex shrink-0">
                      <TypeBadge type={item.type} size="xsmall" />
                      <Tag size="xsmall" variant="neutral">
                        {DOMAIN_CONFIGS[item.domain].label}
                      </Tag>
                    </HStack>
                  </HStack>
                </button>
              ))}
            </VStack>
          ) : (
            <Box background="default" borderRadius="8" padding="space-24" className="text-center">
              <VStack gap="space-12" align="center">
                <BodyShort className="verktoy-subtle">Ingen tilpasninger matcher søket ditt.</BodyShort>
                <Button variant="secondary" size="small" onClick={resetFilters}>
                  Nullstill filtre
                </Button>
              </VStack>
            </Box>
          )}
        </VStack>
      ) : (
        <VStack gap="space-24">
          {groupedByDomain.map(({ domain, total, preview }) => (
            <section key={domain} aria-labelledby={`domain-${domain}`}>
              <VStack gap="space-12">
                <HStack gap="space-8" align="center" justify="space-between">
                  <Heading id={`domain-${domain}`} level="2" size="small">
                    {DOMAIN_CONFIGS[domain].label}
                  </Heading>
                  <Button variant="tertiary" size="small" onClick={() => setSelectedDomain(domain)}>
                    Se alle {total}
                  </Button>
                </HStack>

                <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-12">
                  {preview.map((item) => (
                    <button
                      key={itemKey(item)}
                      type="button"
                      className="verktoy-card flex flex-col"
                      onClick={() => setSelectedItem(item)}
                    >
                      <VStack gap="space-8" className="h-full">
                        <HStack gap="space-8" align="center" justify="space-between">
                          <TypeIconBadge type={item.type} size="sm" />
                          {item.usageCount >= POPULAR_THRESHOLD && (
                            <Tag size="xsmall" variant="warning">
                              Populær
                            </Tag>
                          )}
                        </HStack>
                        <VStack gap="space-4">
                          <Heading level="3" size="xsmall" className="break-words">
                            {displayName(item)}
                          </Heading>
                          <HStack gap="space-6" align="center" wrap>
                            <TypeBadge type={item.type} size="xsmall" />
                          </HStack>
                          <BodyShort size="small" className="verktoy-subtle line-clamp-2">
                            {item.description}
                          </BodyShort>
                        </VStack>
                      </VStack>
                    </button>
                  ))}
                </HGrid>
              </VStack>
            </section>
          ))}
        </VStack>
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
