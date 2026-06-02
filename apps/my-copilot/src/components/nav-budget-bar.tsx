"use client";

import { useEffect, useState } from "react";

interface BudgetData {
  budgetAmount: number;
  consumedAmount: number | null;
  isOverride: boolean;
}

export default function NavBudgetBar() {
  const [budget, setBudget] = useState<BudgetData | null>(null);

  useEffect(() => {
    fetch("/api/budget")
      .then((r) => (r.ok ? r.json() : null))
      .then((data) => {
        if (data?.budgetAmount) setBudget(data);
      })
      .catch(() => {});
  }, []);

  if (!budget || budget.consumedAmount === null) return null;

  const pct = Math.min(100, Math.round((budget.consumedAmount / budget.budgetAmount) * 100));
  const color = pct > 90 ? "#f9251d" : pct > 70 ? "#FF9100" : "#06893A";

  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: "6px" }}
      title={`${pct}% av AI-kreditbudsjettet brukt denne måneden`}
    >
      <div
        style={{
          width: "64px",
          height: "4px",
          borderRadius: "999px",
          backgroundColor: "rgba(255,255,255,0.2)",
          overflow: "hidden",
        }}
        role="progressbar"
        aria-label={`${pct}% av AI-kreditbudsjettet brukt`}
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          style={{
            width: `${pct}%`,
            height: "100%",
            borderRadius: "999px",
            backgroundColor: color,
          }}
        />
      </div>
      <span style={{ color: "rgba(255,255,255,0.5)", fontSize: "11px", lineHeight: 1 }}>{pct}%</span>
    </div>
  );
}
