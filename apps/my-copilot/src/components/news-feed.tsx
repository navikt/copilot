"use client";

import { useState } from "react";
import { Chips, HStack } from "@navikt/ds-react";
import type { NewsItem, NewsCategory } from "@/lib/news";
import { CATEGORY_CONFIG } from "@/lib/news";
import { NewsCard, FeaturedNewsCard } from "./news-card";

const CATEGORIES: NewsCategory[] = ["copilot", "nav", "praksis", "nav-pilot", "oppsummering"];

interface NewsFeedProps {
  items: NewsItem[];
}

export function NewsFeed({ items }: NewsFeedProps) {
  const [selected, setSelected] = useState<NewsCategory | null>(null);

  const filtered = selected ? items.filter((item) => item.category === selected) : items;
  const featured = filtered[0];
  const rest = filtered.slice(1);

  return (
    <>
      <HStack gap="space-8" wrap>
        <Chips>
          <Chips.Toggle selected={selected === null} onClick={() => setSelected(null)}>
            Alle
          </Chips.Toggle>
          {CATEGORIES.map((cat) => (
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

      {featured && <FeaturedNewsCard item={featured} />}
      {rest.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 grid-flow-dense gap-3">
          {rest.map((item) => (
            <NewsCard key={item.slug} item={item} />
          ))}
        </div>
      )}
    </>
  );
}
