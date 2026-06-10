import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { DetailVideoPlayer } from "./detail-video-player";
import { demoVideo } from "./storybook-video-fixtures";

const meta = {
  title: "Video/Pages/Detail Video Player",
  component: DetailVideoPlayer,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Detaljsidens spillerflate. Denne bruker samme delte `VideoPlayerSurface`-implementasjon som hjemmesidens `ShortsFeed`.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box className="w-[min(100vw-2rem,420px)] rounded-xl bg-black" padding="space-8">
        <Story />
      </Box>
    ),
  ],
  args: {
    video: demoVideo,
  },
} satisfies Meta<typeof DetailVideoPlayer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
