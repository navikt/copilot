import type { Preview } from "@storybook/nextjs";
import "@navikt/ds-css";
import "../src/app/globals.css";

const preview: Preview = {
  parameters: {
    layout: "centered",
    options: {
      storySort: {
        order: [
          "Storybook",
          "Foundations",
          "News",
          "Customization",
          "Statistikk",
          "Video",
          ["Dokumentasjon", "Primitives", "Patterns", "Pages"],
        ],
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
