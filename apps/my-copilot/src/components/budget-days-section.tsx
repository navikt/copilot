import { InsightSection } from "@/components/insight-page";
import ErrorState from "@/components/error-state";
import { BudgetDaysView } from "@/components/budget-days";
import { getAdminBudgetDays } from "@/lib/cached-bigquery";
import type { CostEvent } from "@/lib/cost-events";

export async function BudgetDaysSection({ token, events }: { token: string; events?: CostEvent[] }) {
  const days = await getAdminBudgetDays(token).catch((error) => {
    console.error("[kostnad] Failed to fetch budget days:", error);
    return null;
  });
  return (
    <InsightSection id="budsjett-per-dag" title="Personlig budsjett gjennom måneden">
      {days ? (
        <BudgetDaysView days={days} events={events} />
      ) : (
        <ErrorState message="Kunne ikke hente fordelingen mot personlig budsjett." />
      )}
    </InsightSection>
  );
}
