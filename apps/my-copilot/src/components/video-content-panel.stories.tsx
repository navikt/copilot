import type { Meta, StoryObj } from "@storybook/nextjs";
import { Box } from "@navikt/ds-react";
import type { OverlayComponent } from "@/lib/public-videos";
import { ContentPanel } from "./video-overlay-components";

const richOverlays: OverlayComponent[] = [
  { kind: "rule-pill", anchor: "bottom-full", labels: ["nytt mål = ny tråd"] },
  {
    kind: "ladder",
    anchor: "center-left",
    labels: ["clone", "init", "test", "publish"],
    highlightIndex: 1,
  },
  { kind: "counter", anchor: "center-right", labels: ["3 → 1"] },
  {
    kind: "chip",
    anchor: "bottom-left",
    monospace: true,
    labels: ["/resume", "/compact", "/clear", "/remote", "/autopilot"],
  },
  { kind: "badge", anchor: "top-right", labels: ["✓"] },
];

const meta = {
  title: "Video/HUD/Content Panel",
  component: ContentPanel,
  tags: ["autodocs"],
  parameters: {
    docs: {
      description: {
        component:
          "Mørk scrim-bakgrunn for rik overlaytekst i video-HUD. Denne storyen dekker de mest komplekse radtypene: regeloverskrift, ladder, counter, monospace-chips og statusmerke.",
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
    overlays: richOverlays,
    accent: "#9af0a8",
  },
} satisfies Meta<typeof ContentPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const RichRows: Story = {};
