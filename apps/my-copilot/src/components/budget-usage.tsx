"use client";

import { BodyShort, ProgressBar, VStack } from "@navikt/ds-react";
import { useEffect, useState } from "react";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

interface BudgetData {
  budgetAmount: number;
  consumedAmount: number | null;
}

// One request per page load, shared by the user menu and the «Meny» panel.
let budgetRequest: Promise<number | null> | undefined;
const loadPct = () =>
  (budgetRequest ??= fetch("/api/budget")
    .then((r) => (r.ok ? (r.json() as Promise<BudgetData>) : null))
    .then((b) =>
      b && b.budgetAmount > 0 && typeof b.consumedAmount === "number"
        ? Math.min(100, Math.round((b.consumedAmount / b.budgetAmount) * 100))
        : null
    )
    .catch(() => null));

/** The share of this month's AI credit limit used, 0–100, or null when unknown. */
export function useBudgetPct() {
  const [pct, setPct] = useState<number | null>(null);
  useEffect(() => {
    loadPct().then(setPct);
  }, []);
  return pct;
}

/** High enough to flag next to the name in the header. */
export const budgetHigh = (pct: number | null): pct is number => pct !== null && pct >= 80;
/** `text` is the shell label with a {pct} placeholder, such as «Du har brukt {pct} % av AI-kredittene denne måneden». */
export const budgetText = (text: string, pct: number) => text.replace("{pct}", String(pct));

export function BudgetUsage({ pct, text }: { pct: number | null; text: string }) {
  if (pct === null) return null;
  return (
    <VStack gap="space-8">
      <ProgressBar
        size="small"
        value={pct}
        valueMax={100}
        data-color={pct >= 90 ? "danger" : pct >= 80 ? "warning" : "accent"}
        aria-hidden
      />
      <BodyShort size="small">{budgetText(text, pct)}</BodyShort>
    </VStack>
  );
}
