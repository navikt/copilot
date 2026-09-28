import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import UsageDistributionChart from "./UsageDistributionChart";

const meta = {
  title: "Charts/UsageDistributionChart",
  component: UsageDistributionChart,
  decorators: [
    (Story) => (
      <Box padding="space-16" className="mx-auto max-w-7xl">
        <Story />
      </Box>
    ),
  ],
  args: {
    // Shape of the report that prompted the fix: more users with activity
    // than licences now, and two top buckets under 5 users, which the API
    // sends as suppressed.
    distribution: {
      month: "2026-09",
      num_users: 710,
      total_licensed_seats: 682,
      budget_credits: 3000,
      credits_deciles: [],
      interactions_deciles: [],
      acceptances_deciles: [],
      credits_histogram: [
        { bucket: "0%", num_users: 96 },
        { bucket: "1-9%", num_users: 402 },
        { bucket: "10-24%", num_users: 131 },
        { bucket: "25-49%", num_users: 52 },
        { bucket: "50-74%", num_users: 22 },
        { bucket: "75-99%", num_users: 0, suppressed: true },
        { bucket: "100%+", num_users: 0, suppressed: true },
      ],
    },
    currentUserCredits: 4200,
  },
} satisfies Meta<typeof UsageDistributionChart>;

export default meta;

type Story = StoryObj<typeof meta>;

export const SmallTopBuckets: Story = {};
