"use client";

import { useMemo, useState } from "react";
import { BodyShort, Checkbox, CheckboxGroup, HStack, Search, Tag, VStack } from "@navikt/ds-react";
import { MODEL_PRICING } from "@/lib/model-pricing";
import type { ModelPrice } from "@/lib/model-pricing";
import { isNavAllowedModel, navPilotPurposesFor } from "@/lib/model-policy";

const PROVIDER_ORDER = ["OpenAI", "Anthropic", "Google", "GitHub", "Moonshot AI", "Microsoft"] as const;

const COLUMNS = [
  { key: "provider", label: "Leverandør" },
  { key: "model", label: "Modell" },
  { key: "category", label: "Kategori" },
  { key: "navStatus", label: "Nav-status" },
  { key: "navPilot", label: "nav-pilot" },
  { key: "input", label: "Input" },
  { key: "cachedInput", label: "Cached" },
  { key: "cacheWrite", label: "Cache write" },
  { key: "output", label: "Output" },
] as const;

type ColumnKey = (typeof COLUMNS)[number]["key"];
type DiscreteFilterKey = "provider" | "model" | "category" | "navStatus" | "navPilot";
type RangeFilterKey = "input" | "cachedInput" | "cacheWrite" | "output";
type SortKey = ColumnKey;
type SortDirection = "ascending" | "descending";
type RangeFilter = { min?: string; max?: string };
type ColumnFilters = Partial<Record<DiscreteFilterKey, string[]>> & Partial<Record<RangeFilterKey, RangeFilter>>;

const DISCRETE_FILTER_COLUMNS = COLUMNS.filter(({ key }) =>
  ["provider", "model", "category", "navStatus", "navPilot"].includes(key)
) as {
  key: DiscreteFilterKey;
  label: string;
}[];
const RANGE_FILTER_COLUMNS = [
  { key: "input", label: "Input" },
  { key: "cachedInput", label: "Cached" },
  { key: "cacheWrite", label: "Cache write" },
  { key: "output", label: "Output" },
] as const;

const SORT_LABELS = Object.fromEntries(COLUMNS.map(({ key, label }) => [key, label])) as Record<ColumnKey, string>;

const promotionEndFormat = new Intl.DateTimeFormat("nb-NO", {
  day: "numeric",
  month: "long",
  year: "numeric",
  timeZone: "UTC",
});

function formatPrice(price: number): string {
  return `$${price.toFixed(price < 0.1 ? 3 : 2)}`;
}

function columnValue(model: ModelPrice, key: DiscreteFilterKey): string {
  switch (key) {
    case "provider":
      return model.provider;
    case "model":
      return model.model;
    case "category":
      return model.category;
    case "navStatus":
      return isNavAllowedModel(model.model) ? "Aktivert i Nav" : "Ikke aktivert i Nav";
    case "navPilot":
      return navPilotPurposesFor(model.model).join(", ") || "Ikke brukt";
  }
}

function sortableValue(model: ModelPrice, key: SortKey): string | number | undefined {
  switch (key) {
    case "provider":
    case "model":
    case "category":
    case "navStatus":
    case "navPilot":
      return columnValue(model, key);
    case "input":
    case "cachedInput":
    case "cacheWrite":
    case "output":
      return model[key];
  }
}

const CATEGORY_STYLES: Record<ModelPrice["category"], { color: string; background: string }> = {
  Lightweight: { color: "#22c55e", background: "rgba(34, 197, 94, 0.1)" },
  Versatile: { color: "#3b82f6", background: "rgba(59, 130, 246, 0.1)" },
  Powerful: { color: "#a855f7", background: "rgba(168, 85, 247, 0.1)" },
};

function SortableHeader({
  sortKey,
  activeSortKey,
  direction,
  align = "left",
  onSort,
}: {
  sortKey: SortKey;
  activeSortKey: SortKey | null;
  direction: SortDirection;
  align?: "left" | "right";
  onSort: (key: SortKey) => void;
}) {
  const active = activeSortKey === sortKey;

  return (
    <th
      scope="col"
      aria-sort={active ? direction : undefined}
      className={`${align === "right" ? "text-right" : "text-left"} font-semibold`}
      style={{ color: "#475569", paddingBlock: "var(--ax-space-12)", paddingInline: "var(--ax-space-16)" }}
    >
      <button
        type="button"
        onClick={() => onSort(sortKey)}
        className="inline-flex items-center font-semibold"
        style={{ gap: "var(--ax-space-4)" }}
        aria-label={`Sorter etter ${SORT_LABELS[sortKey]}${active ? (direction === "ascending" ? ", stigende" : ", synkende") : ""}`}
      >
        {SORT_LABELS[sortKey]}
        <span aria-hidden="true">{active ? (direction === "ascending" ? "↑" : "↓") : "↕"}</span>
      </button>
    </th>
  );
}

export function ModelPricingTables() {
  const [search, setSearch] = useState("");
  const [sortKey, setSortKey] = useState<SortKey>("provider");
  const [sortDirection, setSortDirection] = useState<SortDirection>("ascending");
  const [columnFilters, setColumnFilters] = useState<ColumnFilters>({});

  const filterOptions = useMemo(
    () =>
      Object.fromEntries(
        DISCRETE_FILTER_COLUMNS.map(({ key }) => {
          const values = [
            ...new Set(
              MODEL_PRICING.flatMap((model) => {
                if (key !== "navPilot") return [columnValue(model, key)];
                const purposes = navPilotPurposesFor(model.model);
                return purposes.length > 0 ? purposes : ["Ikke brukt"];
              })
            ),
          ];
          if (key === "provider") {
            values.sort(
              (a, b) =>
                PROVIDER_ORDER.indexOf(a as (typeof PROVIDER_ORDER)[number]) -
                PROVIDER_ORDER.indexOf(b as (typeof PROVIDER_ORDER)[number])
            );
          } else {
            values.sort((a, b) => a.localeCompare(b, "nb-NO"));
          }
          return [key, values];
        })
      ) as Record<DiscreteFilterKey, string[]>,
    []
  );

  const models = useMemo(() => {
    const query = search.trim().toLocaleLowerCase("nb-NO");
    const matchingModels = MODEL_PRICING.filter((model) => {
      if (
        query &&
        !`${model.model} ${model.provider} ${model.category} ${columnValue(model, "navStatus")} ${columnValue(model, "navPilot")}`
          .toLocaleLowerCase("nb-NO")
          .includes(query)
      ) {
        return false;
      }
      const matchesDiscreteFilters = DISCRETE_FILTER_COLUMNS.every(({ key }) => {
        const selectedValues = columnFilters[key];
        if (!selectedValues) return true;
        if (key !== "navPilot") return selectedValues.includes(columnValue(model, key));
        const purposes = navPilotPurposesFor(model.model);
        return (purposes.length > 0 ? purposes : ["Ikke brukt"]).some((purpose) => selectedValues.includes(purpose));
      });
      const matchesRanges = RANGE_FILTER_COLUMNS.every(({ key }) => {
        const range = columnFilters[key];
        const price = model[key];
        if ((!range?.min && !range?.max) || price === undefined) return !range?.min && !range?.max;
        return (!range.min || price >= Number(range.min)) && (!range.max || price <= Number(range.max));
      });
      return matchesDiscreteFilters && matchesRanges;
    });

    return [...matchingModels].sort((a, b) => {
      const aValue = sortableValue(a, sortKey);
      const bValue = sortableValue(b, sortKey);
      if (aValue == null || bValue == null) {
        if (aValue == null && bValue == null) return 0;
        return aValue == null ? 1 : -1;
      }
      const comparison =
        sortKey === "provider"
          ? PROVIDER_ORDER.indexOf(a.provider) - PROVIDER_ORDER.indexOf(b.provider)
          : typeof aValue === "number" && typeof bValue === "number"
            ? aValue - bValue
            : String(aValue).localeCompare(String(bValue), "nb-NO");
      return sortDirection === "ascending" ? comparison : -comparison;
    });
  }, [search, sortKey, sortDirection, columnFilters]);

  function handleSort(key: SortKey) {
    if (sortKey === key) {
      setSortDirection((direction) => (direction === "ascending" ? "descending" : "ascending"));
    } else {
      setSortKey(key);
      setSortDirection("ascending");
    }
  }

  const hasCacheWrite = models.some((model) => model.cacheWrite !== undefined);

  return (
    <VStack gap="space-16">
      <HStack gap="space-16" align="end" wrap>
        <Search
          label="Filtrer modeller"
          description="Søk på modell, leverandør eller kategori."
          size="small"
          variant="simple"
          value={search}
          onChange={setSearch}
          className="max-w-md"
        />
      </HStack>
      <HStack gap="space-8" wrap aria-label="Filtrer på kolonneverdier">
        {DISCRETE_FILTER_COLUMNS.map(({ key, label }) => {
          const values = filterOptions[key];
          const selectedValues = columnFilters[key] ?? values;
          const allSelected = selectedValues.length === values.length;
          return (
            <details key={key} className="relative">
              <summary
                className="cursor-pointer rounded-md border border-gray-300 text-sm"
                style={{ paddingBlock: "var(--ax-space-8)", paddingInline: "var(--ax-space-12)" }}
              >
                {label}: {allSelected ? "Alle" : `${selectedValues.length}/${values.length}`}
              </summary>
              <div
                className="absolute left-0 z-10 max-h-80 min-w-56 overflow-y-auto rounded-md border border-gray-200 bg-white shadow-lg"
                style={{ marginTop: "var(--ax-space-4)", padding: "var(--ax-space-12)" }}
              >
                <CheckboxGroup
                  legend={`Vis ${label.toLocaleLowerCase("nb-NO")}`}
                  size="small"
                  value={selectedValues}
                  onChange={(value) =>
                    setColumnFilters((current) => ({
                      ...current,
                      [key]: value.length === values.length ? undefined : value,
                    }))
                  }
                >
                  <VStack gap="space-4">
                    {values.map((value) => (
                      <Checkbox key={value} value={value}>
                        {value}
                      </Checkbox>
                    ))}
                  </VStack>
                </CheckboxGroup>
              </div>
            </details>
          );
        })}
        {RANGE_FILTER_COLUMNS.map(({ key, label }) => {
          const range = columnFilters[key] ?? {};
          const prices = MODEL_PRICING.map((model) => model[key]).filter(
            (price): price is number => price !== undefined
          );
          const minimum = Math.min(...prices);
          const maximum = Math.max(...prices);
          const summary = range.min || range.max ? `${range.min || "Min"}–${range.max || "Maks"}` : "Alle";

          return (
            <details key={key} className="relative">
              <summary
                className="cursor-pointer rounded-md border border-gray-300 text-sm"
                style={{ paddingBlock: "var(--ax-space-8)", paddingInline: "var(--ax-space-12)" }}
              >
                {label}: {summary}
              </summary>
              <div
                className="absolute left-0 z-10 flex min-w-64 rounded-md border border-gray-200 bg-white shadow-lg"
                style={{ marginTop: "var(--ax-space-4)", gap: "var(--ax-space-12)", padding: "var(--ax-space-12)" }}
              >
                <label className="flex flex-col text-sm" style={{ gap: "var(--ax-space-4)" }}>
                  Min
                  <input
                    aria-label={`${label} min`}
                    type="number"
                    min={minimum}
                    max={maximum}
                    step="any"
                    value={range.min ?? ""}
                    onChange={(event) =>
                      setColumnFilters((current) => ({
                        ...current,
                        [key]: { ...current[key], min: event.target.value || undefined },
                      }))
                    }
                    className="w-28 rounded-md border border-gray-300"
                    style={{ paddingBlock: "var(--ax-space-4)", paddingInline: "var(--ax-space-8)" }}
                  />
                </label>
                <label className="flex flex-col text-sm" style={{ gap: "var(--ax-space-4)" }}>
                  Maks
                  <input
                    aria-label={`${label} maks`}
                    type="number"
                    min={minimum}
                    max={maximum}
                    step="any"
                    value={range.max ?? ""}
                    onChange={(event) =>
                      setColumnFilters((current) => ({
                        ...current,
                        [key]: { ...current[key], max: event.target.value || undefined },
                      }))
                    }
                    className="w-28 rounded-md border border-gray-300"
                    style={{ paddingBlock: "var(--ax-space-4)", paddingInline: "var(--ax-space-8)" }}
                  />
                </label>
              </div>
            </details>
          );
        })}
      </HStack>
      {models.length > 0 ? (
        <>
          <BodyShort size="small" style={{ color: "#64748b" }}>
            {models.length} {models.length === 1 ? "modell" : "modeller"}
          </BodyShort>
          {hasCacheWrite && (
            <BodyShort size="small" style={{ color: "#64748b" }}>
              «Cache write» er kostnaden for å skrive kontekst til cache, og kommer i tillegg til cached input.
            </BodyShort>
          )}
          <div className="w-full overflow-x-auto">
            <table
              className="w-full min-w-max sm:min-w-0 text-sm border-collapse"
              style={{ borderRadius: "0.75rem", overflow: "hidden", border: "1px solid #e2e8f0" }}
            >
              <thead>
                <tr style={{ background: "#f8fafc" }}>
                  {COLUMNS.map(({ key }) => (
                    <SortableHeader
                      key={key}
                      sortKey={key}
                      activeSortKey={sortKey}
                      direction={sortDirection}
                      align={
                        key === "input" || key === "cachedInput" || key === "cacheWrite" || key === "output"
                          ? "right"
                          : "left"
                      }
                      onSort={handleSort}
                    />
                  ))}
                </tr>
              </thead>
              <tbody>
                {models.map((model, index) => (
                  <tr
                    key={`${model.provider}-${model.model}`}
                    style={{ borderTop: "1px solid #e2e8f0", background: index % 2 === 0 ? "white" : "#fafbfc" }}
                  >
                    {COLUMNS.map(({ key }) => {
                      if (key === "provider") {
                        return (
                          <td
                            key={key}
                            style={{ paddingBlock: "var(--ax-space-12)", paddingInline: "var(--ax-space-16)" }}
                          >
                            {model.provider}
                          </td>
                        );
                      }
                      if (key === "model") {
                        return (
                          <td
                            key={key}
                            style={{ paddingBlock: "var(--ax-space-12)", paddingInline: "var(--ax-space-16)" }}
                          >
                            <span className="font-medium" style={{ color: "#1e293b" }}>
                              {model.model}
                            </span>
                            {model.promotionEndsOn && (
                              <span
                                className="inline-block rounded-full font-medium align-middle"
                                style={{
                                  fontSize: "0.6875rem",
                                  color: "#92400e",
                                  background: "rgba(245, 158, 11, 0.16)",
                                  marginInlineStart: "var(--ax-space-8)",
                                  paddingBlock: "var(--ax-space-2)",
                                  paddingInline: "var(--ax-space-8)",
                                }}
                              >
                                Kampanjepris t.o.m. {promotionEndFormat.format(new Date(model.promotionEndsOn))}
                              </span>
                            )}
                          </td>
                        );
                      }
                      if (key === "category") {
                        const style = CATEGORY_STYLES[model.category];
                        return (
                          <td
                            key={key}
                            style={{ paddingBlock: "var(--ax-space-12)", paddingInline: "var(--ax-space-16)" }}
                          >
                            <span
                              className="inline-block rounded-full font-medium"
                              style={{
                                fontSize: "0.6875rem",
                                paddingBlock: "var(--ax-space-2)",
                                paddingInline: "var(--ax-space-10)",
                                ...style,
                              }}
                            >
                              {model.category}
                            </span>
                          </td>
                        );
                      }
                      if (key === "navStatus") {
                        const allowed = isNavAllowedModel(model.model);
                        return (
                          <td
                            key={key}
                            style={{ paddingBlock: "var(--ax-space-12)", paddingInline: "var(--ax-space-16)" }}
                          >
                            <Tag size="xsmall" variant="moderate" data-color={allowed ? "success" : "warning"}>
                              {columnValue(model, key)}
                            </Tag>
                          </td>
                        );
                      }
                      if (key === "navPilot") {
                        return (
                          <td
                            key={key}
                            style={{
                              color: "#475569",
                              fontSize: "0.8125rem",
                              paddingBlock: "var(--ax-space-12)",
                              paddingInline: "var(--ax-space-16)",
                            }}
                          >
                            {columnValue(model, key)}
                          </td>
                        );
                      }

                      const price =
                        key === "input"
                          ? model.input
                          : key === "cachedInput"
                            ? model.cachedInput
                            : key === "output"
                              ? model.output
                              : model.cacheWrite;
                      return (
                        <td
                          key={key}
                          className="text-right font-mono"
                          style={{
                            color: key === "cachedInput" || key === "cacheWrite" ? "#64748b" : "#1e293b",
                            fontSize: "0.8125rem",
                            paddingBlock: "var(--ax-space-12)",
                            paddingInline: "var(--ax-space-16)",
                          }}
                        >
                          {price !== undefined ? formatPrice(price) : "—"}
                        </td>
                      );
                    })}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      ) : (
        <BodyShort>Ingen modeller funnet for søket ditt.</BodyShort>
      )}
    </VStack>
  );
}
