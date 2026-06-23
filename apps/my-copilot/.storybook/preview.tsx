import type { Preview } from "@storybook/nextjs";
import "@navikt/ds-css";
import "../src/app/globals.css";
import { Inter } from "next/font/google";

const inter = Inter({ subsets: ["latin"] });

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
      options: {
        dark: { name: "dark", value: "#10141a" },
        light: { name: "light", value: "#ffffff" },
      },
    },
  },

  decorators: [
    (Story) => (
      <div className={`${inter.className} min-h-screen flex flex-col text-gray-900 bg-gray-100 p-8`}>
        <Story />
      </div>
    ),
  ],

  initialGlobals: {
    backgrounds: {
      value: "light",
    },
  },
};

export default preview;
