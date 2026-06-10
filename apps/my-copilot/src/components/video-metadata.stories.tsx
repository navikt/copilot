import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { demoVideo } from "./storybook-video-fixtures";
import { VideoMetadata } from "./video-metadata";

const meta = {
  title: "Video/Panels/Metadata",
  component: VideoMetadata,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Metadata-panel for video-detaljsiden (tittel, beskrivelse, tags og varighet). Brukes for å verifisere typografi, spacing og teksthåndtering.",
      },
    },
  },
  decorators: [
    (Story) => (
      <Box padding="space-16" className="max-w-md rounded-xl bg-black">
        <Story />
      </Box>
    ),
  ],
  args: {
    video: demoVideo,
  },
} satisfies Meta<typeof VideoMetadata>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
