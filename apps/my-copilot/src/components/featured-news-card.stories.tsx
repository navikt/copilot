import type { Meta, StoryObj } from "@storybook/nextjs";
import { FeaturedNewsCard } from "./news-card";
import { storyNewsItems } from "./storybook-news-fixtures";

const meta = {
  title: "News/Primitives/FeaturedNewsCard",
  component: FeaturedNewsCard,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component: "Fremhevet førsteoppføring i nyhetsfeeden, med større format og mer plass til ingress.",
      },
    },
  },
  args: {
    item: storyNewsItems[0],
  },
} satisfies Meta<typeof FeaturedNewsCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
