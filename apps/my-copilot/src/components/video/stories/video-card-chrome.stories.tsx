import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { PlaybackControls } from "../video-card-chrome";

const meta = {
  title: "Video/Controls/Playback Controls",
  component: PlaybackControls,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Primære avspillingskontroller (play/pause + 5s hopp). Bruk denne storyen for tastatur/fokus- og kontrastverifisering av interaktive kontroller.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box padding="space-16" className="rounded-xl bg-black">
        <Story />
      </Box>
    ),
  ],
  args: {
    ariaLabel: "Spill av video: Kontekst og session-flyt",
    playing: false,
    showSkip: true,
    title: "Kontekst og session-flyt",
    onToggle: () => undefined,
    onSeekBackward: () => undefined,
    onSeekForward: () => undefined,
  },
} satisfies Meta<typeof PlaybackControls>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Paused: Story = {};

export const Playing: Story = {
  parameters: {
    docs: {
      description: {
        story: "Viser pause-tilstand med aktiv spill-av knapp og tilgjengelige hopp-kontroller.",
      },
    },
  },
  args: {
    playing: true,
    ariaLabel: "Sett på pause: Kontekst og session-flyt",
  },
};
