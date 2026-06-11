import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import { VerticalVideoContainer } from "../vertical-video-container";

const meta = {
  title: "Video/Pages/Vertical Container",
  component: VerticalVideoContainer,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Kinomatisk to-kolonnes layout for video-detaljsiden. På mobil (< 768px) stables video over metadata; på desktop (≥ 768px) vises en smal videokolonne til venstre og et metadata-panel til høyre. Selve containeren er svart slik at hele siden føles som en video-opplevelse.",
      },
    },
  },
  args: {
    children: (
      <>
        <Box
          className="flex items-center justify-center bg-black text-white md:w-[360px]"
          style={{ aspectRatio: "9 / 16" }}
        >
          Videokolonne
        </Box>
        <Box className="flex-1 bg-white p-6">Metadata-panel</Box>
      </>
    ),
  },
} satisfies Meta<typeof VerticalVideoContainer>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
