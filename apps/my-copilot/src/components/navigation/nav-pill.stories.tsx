import type { Meta, StoryObj } from "@storybook/nextjs";
import { PlayIcon } from "@navikt/aksel-icons";
import { Box } from "@navikt/ds-react";
import { NavPill } from "./nav-pill";

const meta = {
  title: "Foundations/Primitives/NavPill",
  component: NavPill,
  tags: ["autodocs"],
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Navigasjonsflis som brukes i hero-områdene for å vise snarveier til hovedsidene.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box className="bg-[#10141a] p-6">
        <Story />
      </Box>
    ),
  ],
  args: {
    href: "/kom-i-gang",
    icon: <PlayIcon aria-hidden fontSize="1rem" />,
    label: "Kom i gang",
    active: false,
    locked: false,
    muted: false,
  },
} satisfies Meta<typeof NavPill>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Locked: Story = {
  args: {
    href: "/statistikk",
    label: "Statistikk",
    locked: true,
  },
};

export const ActiveMuted: Story = {
  args: {
    href: "/praksis",
    label: "God praksis",
    active: true,
    muted: true,
  },
};
