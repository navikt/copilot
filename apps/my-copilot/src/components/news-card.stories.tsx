import type { Meta, StoryObj } from "@storybook/nextjs";
import { NewsCard } from "./news-card";
import { storyNewsItems } from "./storybook-news-fixtures";

const meta = {
  title: "News/Primitives/NewsCard",
  component: NewsCard,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component: "Kortvisning for en nyhetssak eller ekstern lenke, brukt i både feed og bento-grid.",
      },
    },
  },
  args: {
    item: storyNewsItems[1],
    span: 1,
  },
} satisfies Meta<typeof NewsCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Wide: Story = {
  args: {
    item: storyNewsItems[2],
    span: 2,
  },
};
