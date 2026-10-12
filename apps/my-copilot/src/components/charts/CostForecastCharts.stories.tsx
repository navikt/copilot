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

// The year forecast in the API's shape: low flat at October's midpoint, mid growing 25 % a month, high 35 % above (the backtest error).
const oct = data.models[0];
const closed = net.reduce<number[]>((acc, v) => [...acc, (acc.at(-1) ?? 0) + v], []);
let [mid, low, high] = [closed[8] + oct.net_mid_usd, closed[8] + oct.net_low_usd, closed[8] + oct.net_high_usd];
data.year_forecast = [
  ...closed.map((c, i) => ({ month: months[i].month, cumulative_net_usd: c, low: c, high: c, forecast: false })),
  { month: "2026-10", cumulative_net_usd: mid, low, high, forecast: true },
  ...[1, 2].map((k) => {
    const trend = oct.net_mid_usd * 1.25 ** k * (k === 2 ? 0.83 : 1);
    [mid, low, high] = [mid + trend, low + oct.net_mid_usd, high + trend * 1.35];
    return { month: `2026-1${k}`, cumulative_net_usd: mid, low, high, forecast: true };
  }),
];
data.year_end = { mid, low, high, range_pct: 35, backtest_error_pct: 35, label: "grovt anslag, få måneder å bygge på" };

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
