import type { Meta, StoryObj } from "@storybook/nextjs";
import { PraksisSelector } from "./praksis-selector";

const meta: Meta<typeof PraksisSelector> = {
  title: "nav-pilot/PraksisSelector",
  component: PraksisSelector,
  tags: ["autodocs"],
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "PraksisSelector lar brukeren velge hvilket mål de har (f.eks. skrive kode, refaktorere), og gir en anbefaling om hvilket verktøy (Copilot CLI, OpenCode, IDE) som passer best for oppgaven.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof PraksisSelector>;

export const Default: Story = {
  render: () => (
    <div className="max-w-4xl mx-auto w-full">
      <PraksisSelector />
    </div>
  ),
};
