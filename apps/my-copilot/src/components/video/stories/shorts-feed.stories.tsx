import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { relatedVideos } from "./storybook-video-fixtures";
import { ShortsFeed } from "../shorts-feed";

const meta = {
  title: "Video/Pages/Home Shorts Feed",
  component: ShortsFeed,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Dette er hjemmesidens faktiske videospiller-feed. Samme player-surface brukes også på detaljsiden gjennom delt komponent.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box className="max-w-[720px] rounded-xl bg-black" padding="space-16">
        <Story />
      </Box>
    ),
  ],
  args: {
    videos: relatedVideos,
    initialVideoId: relatedVideos[1]?.id,
  },
} satisfies Meta<typeof ShortsFeed>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
