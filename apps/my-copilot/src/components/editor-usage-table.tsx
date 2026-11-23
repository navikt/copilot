import { PlainTable } from "@/components/charts/chart-figure";
import { formatNumber } from "@/lib/format";
import type { EditorUsage } from "@/lib/types";
import { ScrollRegion } from "@/components/scroll-region";

export default function EditorUsageTable({ editors }: { editors: EditorUsage[] }) {
  const count = (n: number | null) => (n == null ? "Ukjent" : formatNumber(n));
  return (
    <ScrollRegion label="Tabell: bruk per editor">
      <PlainTable
        rows={[
          ["Editor", "Fra", "Til", "Dager med data", "Genereringer", "Aksepteringer", "Interaksjoner"],
          ...editors.map((r) => [
            r.editor,
            r.first_day,
            r.last_day,
            formatNumber(r.days),
            count(r.generations),
            count(r.acceptances),
            count(r.interactions),
          ]),
        ]}
      />
    </ScrollRegion>
  );
}
