import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { BudgetDaysView } from "./budget-days";
import type { BudgetDay, SpendBand } from "@/lib/types";

const LABELS = ["0–10 %", "10–25 %", "25–50 %", "50–75 %", "75–90 %", "90–100 %", "Over 100 %"];

// Made-up counts: users move up the buckets through each month and start over on the 1st.
function month(m: string, lastDay: number, pace: number): BudgetDay[] {
  return Array.from({ length: lastDay }, (_, i) => {
    const t = ((i + 1) / 30) * pace;
    const w = [1 - t, t * 0.5, t * 0.25, t * 0.12, t * 0.06, t * 0.04, t * 0.03].map((x) => Math.max(x, 0));
    const sum = w.reduce((a, b) => a + b, 0);
    const counts = w.map((x) => Math.round((x / sum) * 650));
    const bands: SpendBand[] = counts.map((users, k) => ({ first: k, last: k, label: LABELS[k], users }));
    // Merge the small top buckets the way copilot-api does.
    const top = bands.slice(4);
    const small = top.some((b) => b.users < 5);
    const merged = small
      ? [...bands.slice(0, 4), { first: 4, last: 6, label: "Over 75 %", users: top.reduce((a, b) => a + b.users, 0) }]
      : bands;
    const day = `${m}-${String(i + 1).padStart(2, "0")}`;
    return { day, users: 650, snapshots: m === "2026-10" && i >= 9 ? 640 : 0, bands: merged };
  });
}

const days = [...month("2026-09", 30, 0.9), ...month("2026-10", 12, 0.8)];

const meta: Meta<typeof BudgetDaysView> = {
  title: "Kostnad/BudgetDaysView",
  component: BudgetDaysView,
  decorators: [
    (Story) => (
      <Box padding="space-16" style={{ maxWidth: 960 }}>
        <Story />
      </Box>
    ),
  ],
};
export default meta;

export const Default: StoryObj<typeof BudgetDaysView> = {
  args: {
    days,
    events: [
      { date: "2026-09-22", text: "Grense 800", label: "22. september 2026: forbruksgrensen per person ble 800 USD" },
    ],
  },
};
