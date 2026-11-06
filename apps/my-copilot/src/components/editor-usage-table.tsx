import { PlainTable } from "@/components/charts/chart-figure";
import { formatNumber } from "@/lib/format";
import type { EditorUsage } from "@/lib/types";

export default function EditorUsageTable({ editors }: { editors: EditorUsage[] }) {
  const count = (n: number | null) => (n == null ? "Ukjent" : formatNumber(n));
  return (
    <div className="overflow-x-auto">
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
    </div>
  );
}
