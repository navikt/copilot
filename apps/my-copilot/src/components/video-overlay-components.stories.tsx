import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box, HStack } from "@navikt/ds-react";
import { EpisodePill, GlyphBadge } from "./video-overlay-components";

const meta = {
  title: "Video/Primitives/Overlay Components",
  component: EpisodePill,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Små visuelle byggesteiner i video-HUD: episode-pill og status-badges. Brukes for konsekvent metadata-visning på tvers av videokomponenter.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box padding="space-16" className="rounded-lg bg-black">
        <Story />
      </Box>
    ),
  ],
  args: {
    label: "02",
    accent: "#9af0a8",
  },
} satisfies Meta<typeof EpisodePill>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Episode: Story = {};

export const StatusBadges: Story = {
  parameters: {
    docs: {
      description: {
        story: "Statusindikatorer for korte signaler (f.eks. fullført/check eller advarsel).",
      },
    },
  },
  render: () => (
    <HStack gap="space-8">
      <GlyphBadge label="✓" accent="#9af0a8" />
      <GlyphBadge label="!" accent="#9af0a8" />
    </HStack>
  ),
};
