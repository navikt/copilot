import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { demoVideo } from "./storybook-video-fixtures";
import { VideoOverlayRenderer } from "./video-overlay-renderer";

const meta = {
  title: "Video/HUD/Overlay Renderer",
  component: VideoOverlayRenderer,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Plasserer overlay-data fra video-metadata som et absolutt posisjonert lag over spillerflaten: episode-pill og status-glyfer i topp-raden, og innholdspanel over tittelen. Komponenten returnerer `null` når det ikke finnes overlays.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box
        className="relative overflow-hidden rounded-xl bg-black"
        style={{
          width: "320px",
          aspectRatio: "9 / 16",
        }}
      >
        <Story />
      </Box>
    ),
  ],
  args: {
    overlays: demoVideo.metadata?.overlay,
  },
} satisfies Meta<typeof VideoOverlayRenderer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const EpisodeOnly: Story = {
  parameters: {
    docs: {
      description: {
        story: "Kun episodenummer i topp-raden, uten status-glyfer eller innholdspanel.",
      },
    },
  },
  args: {
    overlays: [{ kind: "episode-number", anchor: "top-left", labels: ["S1E2"] }],
  },
};

export const Empty: Story = {
  parameters: {
    docs: {
      description: {
        story: "Uten overlays rendrer komponenten ingenting (`null`). Flaten viser bare den svarte bakgrunnen.",
      },
    },
  },
  args: {
    overlays: [],
  },
};
