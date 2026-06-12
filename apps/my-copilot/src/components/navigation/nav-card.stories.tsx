import type { Meta, StoryObj } from "@storybook/nextjs";
import { ExternalLinkIcon, PlayIcon } from "@navikt/aksel-icons";
import { Box } from "@navikt/ds-react";
import { NavCard } from "./nav-card";

const meta = {
  title: "Foundations/Primitives/NavCard",
  component: NavCard,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component: "Klikkbar ressursflate for startpunkter og eksterne referanser fra forsiden.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box padding="space-16" className="max-w-md">
        <Story />
      </Box>
    ),
  ],
  args: {
    href: "/kom-i-gang",
    icon: <PlayIcon aria-hidden />,
    title: "Kom i gang",
    description: "Alt du trenger for å starte med Copilot",
    external: false,
  },
} satisfies Meta<typeof NavCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const External: Story = {
  args: {
    href: "https://docs.github.com/en/copilot",
    icon: <ExternalLinkIcon aria-hidden />,
    title: "Dokumentasjon",
    description: "Offisiell dokumentasjon fra GitHub",
    external: true,
  },
};
