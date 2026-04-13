"use client";

import { useEffect, useState, useRef, useMemo, type MouseEvent } from "react";

export interface TocItem {
  id: string;
  label: string;
  children?: TocItem[];
}

interface TableOfContentsProps {
  items: TocItem[];
}

function flattenItems(items: TocItem[]): TocItem[] {
  const flat: TocItem[] = [];
  for (const item of items) {
    flat.push(item);
    if (item.children) {
      flat.push(...item.children);
    }
  }
  return flat;
}

export function TableOfContents({ items }: TableOfContentsProps) {
  const [activeId, setActiveId] = useState<string>("");
  const observerRef = useRef<IntersectionObserver | null>(null);

  const allItems = useMemo(() => flattenItems(items), [items]);
  const hasGroups = items.some((item) => item.children);

  useEffect(() => {
    observerRef.current = new IntersectionObserver(
      (entries) => {
        const visible = entries
          .filter((e) => e.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
        if (visible.length > 0) {
          setActiveId(visible[0].target.id);
        }
      },
      { rootMargin: "-80px 0px -60% 0px", threshold: 0 }
    );

    const elements = allItems
      .map((item) => document.getElementById(item.id))
      .filter((el): el is HTMLElement => el !== null);

    elements.forEach((el) => observerRef.current?.observe(el));

    return () => observerRef.current?.disconnect();
  }, [allItems]);

  const linkClass = (id: string) =>
    `block text-sm no-underline rounded-md transition-colors ${
      activeId === id ? "bg-blue-50 text-blue-700 font-medium" : "text-gray-500 hover:text-gray-800 hover:bg-gray-50"
    }`;

  const handleClick = (e: MouseEvent, id: string) => {
    e.preventDefault();
    document.getElementById(id)?.scrollIntoView({ behavior: "smooth", block: "start" });
    window.history.replaceState(null, "", `#${id}`);
  };

  return (
    <nav aria-label="Innholdsfortegnelse" className="toc">
      <p
        className="text-xs font-semibold text-gray-400 uppercase tracking-wider"
        style={{ marginBottom: "var(--a-spacing-3)" }}
      >
        Innhold
      </p>
      <ul style={{ display: "flex", flexDirection: "column", gap: "var(--a-spacing-1)" }}>
        {items.map((item) =>
          hasGroups && item.children ? (
            <li key={item.id} style={{ marginTop: "var(--a-spacing-3)" }} className="first:mt-0">
              <a
                href={`#${item.id}`}
                onClick={(e) => handleClick(e, item.id)}
                className={`block text-[11px] font-semibold uppercase tracking-wider no-underline transition-colors ${
                  activeId === item.id ? "text-blue-700" : "text-gray-400 hover:text-gray-600"
                }`}
                style={{ padding: "var(--a-spacing-1) var(--a-spacing-3)" }}
              >
                {item.label}
              </a>
              <ul style={{ display: "flex", flexDirection: "column", gap: "2px", marginTop: "2px" }}>
                {item.children.map((child) => (
                  <li key={child.id}>
                    <a
                      href={`#${child.id}`}
                      onClick={(e) => handleClick(e, child.id)}
                      className={linkClass(child.id)}
                      style={{ padding: "var(--a-spacing-2) var(--a-spacing-3)" }}
                    >
                      {child.label}
                    </a>
                  </li>
                ))}
              </ul>
            </li>
          ) : (
            <li key={item.id}>
              <a
                href={`#${item.id}`}
                onClick={(e) => handleClick(e, item.id)}
                className={linkClass(item.id)}
                style={{ padding: "var(--a-spacing-2) var(--a-spacing-3)" }}
              >
                {item.label}
              </a>
            </li>
          )
        )}
      </ul>
    </nav>
  );
}
