import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { CostForecastCharts } from "./CostForecastCharts";
import type { AdminMonth, CostForecast } from "@/lib/types";

// Made-up amounts in the shape of October 2026: credits run out on the 6th, data up to the 11th.
const days: CostForecast["days"] = Array.from({ length: 31 }, (_, i) => {
  const d = i + 1;
  const net = d < 6 ? 0 : (d - 5) * 310 + (d > 11 ? (d - 11) * 6 : 0);
  return { day: `2026-10-${String(d).padStart(2, "0")}`, net_usd: net, forecast: d > 11 };
});

const data: CostForecast = {
  month: "2026-10",
  days_elapsed: 11,
  days_in_month: 31,
  allowance_used_up: true,
  net_to_date_usd: days[10].net_usd,
  budget_status: "set",
  budget_usd: 9000,
  days,
  models: [
    {
      model: "trend",
      gross_mid_usd: 14000,
      net_low_usd: days[30].net_usd * 0.85,
      net_mid_usd: days[30].net_usd,
      net_high_usd: days[30].net_usd * 1.15,
      range_pct: 15,
      allowance_runs_out: "2026-10-06",
      backtest_error_pct: 4.9,
      backtest_months: ["2026-08", "2026-09"],
      budget_pct: 90,
      over_budget: false,
    },
  ],
};

const net = [1200, 1500, 1800, 2100, 2400, 3100, 4200, 6900, 7600];
const months = [...net, null].map(
  (v, i) =>
    ({
      month: `2026-${String(i + 1).padStart(2, "0")}`,
      basis: v == null ? "gross" : i < 5 ? "premium" : "net",
      enterprise_net_usd: v,
    }) as AdminMonth
);

const meta = {
  title: "Charts/CostForecastCharts",
  component: CostForecastCharts,
  decorators: [
    (Story) => (
      <Box padding="space-16" className="mx-auto max-w-7xl">
        <Story />
      </Box>
    ),
  ],
  args: { data, months },
} satisfies Meta<typeof CostForecastCharts>;

export default meta;
export const Default: StoryObj<typeof meta> = {};
