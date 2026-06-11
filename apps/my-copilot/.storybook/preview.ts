import type { Preview } from "@storybook/nextjs";
import "@navikt/ds-css";
import "../src/app/globals.css";

const preview: Preview = {
  parameters: {
    layout: "centered",
    options: {
      storySort: {
        order: ["Storybook", "Video", ["Dokumentasjon", "Primitives", "Controls", "Panels", "HUD", "Pages"]],
      },
    },
    controls: {
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    backgrounds: {
      default: "dark",
      values: [
        { name: "dark", value: "#10141a" },
        { name: "light", value: "#ffffff" },
      ],
    },
  },
};

export default preview;
