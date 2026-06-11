import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { relatedVideos } from "./storybook-video-fixtures";
import { RelatedVideos } from "../related-videos";

const meta = {
  title: "Video/Panels/Related Videos",
  component: RelatedVideos,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Panel med relaterte videoer i 9:16-format. Historiene bruker faktiske media-URL-er fra dev-bucket for realistisk layout- og innholdsvalidering.",
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
    videos: relatedVideos,
  },
} satisfies Meta<typeof RelatedVideos>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
