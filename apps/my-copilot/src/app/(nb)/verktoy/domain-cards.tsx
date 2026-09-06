"use client";

import { DomainCard } from "@/components/domain-card";
import type { Domain } from "@/lib/customization-types";

interface DomainCardsProps {
  domain: Domain;
  count: number;
}

export function DomainCards({ domain, count }: DomainCardsProps) {
  const handleClick = (d: Domain) => {
    const el = document.getElementById("catalog");
    if (el) el.scrollIntoView({ behavior: "smooth" });

    const event = new CustomEvent("domain-filter", { detail: d });
    window.dispatchEvent(event);
  };

  return <DomainCard domain={domain} count={count} selected={false} onClick={handleClick} />;
}
