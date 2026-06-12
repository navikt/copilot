import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { NewsFeed } from "./news-feed";
import { storyNewsItems } from "./storybook-news-fixtures";

const meta = {
  title: "News/Patterns/NewsFeed",
  component: NewsFeed,
  tags: ["autodocs"],
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Bento-feeden som viser fremhevet sak først og resten i et stabilt responsivt rutenett.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box padding="space-16" className="mx-auto max-w-7xl">
        <Story />
      </Box>
    ),
  ],
  args: {
    items: storyNewsItems,
    compact: false,
  },
} satisfies Meta<typeof NewsFeed>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Compact: Story = {
  args: {
    compact: true,
  },
};
