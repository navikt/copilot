import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { demoVideo } from "./storybook-video-fixtures";
import type { ShortsFeedMediaHandlers } from "../hooks/use-shorts-feed-media-adapter";
import { VideoPlayerSurface } from "../video-player-surface";

const noopMediaHandlers: ShortsFeedMediaHandlers = {
  onPlay: () => undefined,
  onPause: () => undefined,
  onTimeUpdate: () => undefined,
  onEnded: () => undefined,
  onError: () => undefined,
  onWaiting: () => undefined,
};

const meta = {
  title: "Video/Pages/Player Surface",
  component: VideoPlayerSurface,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Den delte, samlende spillerflaten (`VideoPlayerSurface`) som både hjemmesidens `ShortsFeed` og detaljsidens `DetailVideoPlayer` bygger på. Den setter sammen poster/video-lag, HUD, idle-tekst og fullført-overlay basert på `playbackState`. Historiene bruker no-op media-handlers slik at HUD-tilstandene kan inspiseres uten faktisk avspilling.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box
        className="rounded-xl bg-black"
        style={{
          width: "320px",
        }}
        padding="space-8"
      >
        <Story />
      </Box>
    ),
  ],
  args: {
    video: demoVideo,
    isActive: true,
    playbackState: "idle",
    mediaHandlers: noopMediaHandlers,
    setVideoNode: () => undefined,
    onPrimaryAction: () => undefined,
    onSeekBackward: () => undefined,
    onSeekForward: () => undefined,
    onReplay: () => undefined,
    onFullscreen: () => undefined,
  },
} satisfies Meta<typeof VideoPlayerSurface>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Idle: Story = {
  parameters: {
    docs: {
      description: {
        story: "Standard hviletilstand: poster, idle-tekst og full HUD synlig før avspilling starter.",
      },
    },
  },
};

export const Playing: Story = {
  parameters: {
    docs: {
      description: {
        story: "Aktiv avspilling. HUD-kontrollene vises, og idle-teksten er borte til fordel for video-laget.",
      },
    },
  },
  args: {
    playbackState: "playing",
  },
};

export const Paused: Story = {
  parameters: {
    docs: {
      description: {
        story: "Pauset avspilling med scrim-gradient over flaten og synlig kontroll-lag.",
      },
    },
  },
  args: {
    playbackState: "paused",
  },
};

export const Completed: Story = {
  parameters: {
    docs: {
      description: {
        story: "Fullført avspilling viser fullført-overlay med replay- og delingsvalg.",
      },
    },
  },
  args: {
    playbackState: "completed",
  },
};

export const Inactive: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "Inaktivt kort i en feed: flaten blir en klikkbar knapp som åpner videoen, med poster og idle-tekst, uten aktive kontroller.",
      },
    },
  },
  args: {
    isActive: false,
    onOpen: () => undefined,
  },
};
