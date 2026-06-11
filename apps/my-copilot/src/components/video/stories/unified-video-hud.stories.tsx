import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { demoVideo } from "./storybook-video-fixtures";
import { UnifiedVideoHUD } from "../unified-video-hud";

const meta = {
  title: "Video/HUD/Unified HUD",
  component: UnifiedVideoHUD,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Samlet HUD for video-flaten: toppfelt med metadata og deling, overlayinnhold og avspillingslag. Egnet for visuell regresjon av HUD-tilstander.",
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
    episodeLabel: "S1E2",
    accent: "#9af0a8",
    durationLabel: "2:09",
    shareHref: `/videos/${encodeURIComponent(demoVideo.id)}`,
    shareTitle: demoVideo.title,
    playing: false,
    isActive: true,
    completed: false,
    showHud: true,
    playbackState: "idle",
    onTogglePlayback: () => undefined,
    onSeekBackward: () => undefined,
    onSeekForward: () => undefined,
    title: demoVideo.title,
  },
} satisfies Meta<typeof UnifiedVideoHUD>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Idle: Story = {};

export const Playing: Story = {
  parameters: {
    docs: {
      description: {
        story: "Avspillingsmodus uten innholdspanel, med fokus på kontrollaget og synlighet.",
      },
    },
  },
  args: {
    playing: true,
    playbackState: "playing",
  },
};
