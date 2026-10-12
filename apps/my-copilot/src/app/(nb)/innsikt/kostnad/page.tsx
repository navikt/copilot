import type { Metadata } from "next";
import { InsightPage } from "@/components/insight-page";
import { requireAdmin } from "@/lib/auth";

// Spend and limits are internal: only the copilot team sees this page, everyone else gets 404.
export const metadata: Metadata = {
  title: "Kostnad",
  description: "Forbruk og grenser for Copilot i Nav.",
  robots: { index: false, follow: false },
};

export default async function CostPage() {
  await requireAdmin();
  return (
    <InsightPage
      title="Kostnad"
      description="Forbruk og grenser for Copilot i Nav."
      intro="Siden er bare for team copilot."
      updated={async () => null}
      source="Kommer."
    >
      {null}
    </InsightPage>
  );
}
