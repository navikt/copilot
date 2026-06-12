"use client";

import { useState, useMemo, type ReactNode } from "react";
import { Chips, HStack, VStack, BodyShort, Heading } from "@navikt/ds-react";
import type { NewsItem, NewsCategory } from "@/lib/news-types";
import { CATEGORY_CONFIG } from "@/lib/news-types";
import { NewsCard, FeaturedNewsCard } from "./news-card";

interface NewsFeedProps {
  items: NewsItem[];
  compact?: boolean;
  afterFeatured?: ReactNode;
}

const COLS = 3;
const COLS_COMPACT = 2;

/**
 * Deterministic bento row-packing.
 *
 * The span pattern is derived from the *grid position*, not the item
 * type, so the layout stays visually varied no matter how the feed
 * filter changes the article/link mix over time.
 *
 * For a 3-column grid each row is a wide (span-2) + narrow (span-1)
 * pair, and the wide cell alternates side per row: [2,1] then [1,2]…
 * This guarantees no run of identical spans and a stable bento rhythm.
 *
 * For narrower grids (e.g. the compact 2-column variant) the wide span
 * is clamped so it always leaves room for a neighbour, which gracefully
 * degrades to a simple uniform grid on small layouts.
 */
export function computeGridSpans(count: number, cols: number = COLS): number[] {
  // Wide cell never fills the whole row, so every row keeps >=2 cards.
  const wide = Math.max(1, Math.min(2, cols - 1));

  const spans: number[] = [];
  let col = 0;
  let row = 0;

  for (let i = 0; i < count; i++) {
    const remaining = cols - col;
    let span: number;

    if (col === 0) {
      // Start of a row: a lone trailing card becomes a full-width closer
      // (clean bento finish, no dangling gap); otherwise alternate the
      // wide cell between left and right per row.
      if (i === count - 1) {
        span = cols;
      } else {
        span = row % 2 === 0 ? wide : 1;
      }
    } else {
      // Fill the rest of the current row in one go.
      span = remaining;
    }

    if (span > remaining) span = remaining;

    spans.push(span);
    col += span;

    if (col >= cols) {
      col = 0;
      row++;
    }
  }

  return spans;
}

export function NewsFeed({ items, compact = false, afterFeatured }: NewsFeedProps) {
  const [selected, setSelected] = useState<NewsCategory | null>(null);

  const availableCategories = useMemo(() => {
    const cats = new Set(items.map((item) => item.category));
    return (Object.keys(CATEGORY_CONFIG) as NewsCategory[]).filter((cat) => cats.has(cat));
  }, [items]);

  const filtered = selected ? items.filter((item) => item.category === selected) : items;
  const featured = filtered[0];
  const rest = filtered.slice(1);
  const cols = compact ? COLS_COMPACT : COLS;
  const spans = useMemo(() => computeGridSpans(rest.length, cols), [rest.length, cols]);

  const gridCols = compact ? "grid-cols-1 sm:grid-cols-2" : "grid-cols-1 sm:grid-cols-2 md:grid-cols-3";

  return (
    <VStack gap="space-12">
      <HStack gap="space-12" align="center" wrap>
        <Heading size="small" level="2">
          Siste nytt
        </Heading>
        <Chips>
          <Chips.Toggle key="all" selected={selected === null} onClick={() => setSelected(null)}>
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
      {featured && afterFeatured ? <div key="after-featured-slot">{afterFeatured}</div> : null}
      {rest.length > 0 && (
        <div className={`grid ${gridCols} gap-3`}>
          {rest.map((item, i) => (
            <NewsCard key={item.slug} item={item} span={spans[i]} />
          ))}
        </div>
      )}
    </VStack>
  );
}
