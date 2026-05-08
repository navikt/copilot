"use client";

import { useState, useMemo } from "react";
import { Chips, HStack, VStack, BodyShort, Heading } from "@navikt/ds-react";
import type { NewsItem, NewsCategory } from "@/lib/news-types";
import { CATEGORY_CONFIG } from "@/lib/news-types";
import { NewsCard, FeaturedNewsCard } from "./news-card";

interface NewsFeedProps {
  items: NewsItem[];
}

const COLS = 3;

/**
 * Greedy row-packing: articles prefer span-2, links span-1.
 * When an article doesn't fit in the remaining columns, it
 * downgrades to span-1 so the row fills without gaps.
 */
function computeGridSpans(items: NewsItem[]): number[] {
  const spans: number[] = [];
  let col = 0;

  for (const item of items) {
    const preferred = item.type === "article" ? 2 : 1;
    const remaining = COLS - col;

    if (preferred <= remaining) {
      spans.push(preferred);
      col += preferred;
    } else {
      spans.push(1);
      col += 1;
    }

    if (col >= COLS) col = 0;
  }

  return spans;
}

export function NewsFeed({ items }: NewsFeedProps) {
  const [selected, setSelected] = useState<NewsCategory | null>(null);

  const availableCategories = useMemo(() => {
    const cats = new Set(items.map((item) => item.category));
    return (Object.keys(CATEGORY_CONFIG) as NewsCategory[]).filter((cat) => cats.has(cat));
  }, [items]);

  const filtered = selected ? items.filter((item) => item.category === selected) : items;
  const featured = filtered[0];
  const rest = filtered.slice(1);
  const spans = useMemo(() => computeGridSpans(rest), [rest]);

  return (
    <VStack gap="space-12">
      <HStack gap="space-12" align="center" wrap>
        <Heading size="small" level="2">
          Siste nytt
        </Heading>
        <Chips>
          <Chips.Toggle selected={selected === null} onClick={() => setSelected(null)}>
            Alle
          </Chips.Toggle>
          {availableCategories.map((cat) => (
            <Chips.Toggle
              key={cat}
              selected={selected === cat}
              onClick={() => setSelected(selected === cat ? null : cat)}
            >
              {CATEGORY_CONFIG[cat].label}
            </Chips.Toggle>
          ))}
        </Chips>
      </HStack>

      {filtered.length === 0 && <BodyShort className="text-text-subtle">Ingen nyheter i denne kategorien.</BodyShort>}
      {featured && <FeaturedNewsCard item={featured} />}
      {rest.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
          {rest.map((item, i) => (
            <NewsCard key={item.slug} item={item} span={spans[i]} />
          ))}
        </div>
      )}
    </VStack>
  );
}
