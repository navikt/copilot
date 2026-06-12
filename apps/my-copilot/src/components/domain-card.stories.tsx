import type { Meta, StoryObj } from "@storybook/nextjs";
import { DomainCard } from "./domain-card";

const meta = {
  title: "Customization/DomainCard",
  component: DomainCard,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component: "Valgbar domeneflate for customizations, brukt til å filtrere innhold og fremheve temaer.",
      },
    },
  },
  args: {
    domain: "frontend",
    count: 12,
    selected: false,
    onClick: () => undefined,
  },
} satisfies Meta<typeof DomainCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Selected: Story = {
  args: {
    selected: true,
    count: 7,
  },
};
