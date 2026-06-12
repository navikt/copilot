import type { Meta, StoryObj } from "@storybook/nextjs";
import { Button } from "@navikt/ds-react";
import { PageHeroBase } from "./page-hero";

const meta = {
  title: "Foundations/Primitives/PageHero",
  component: PageHeroBase,
  tags: ["autodocs"],
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Hero-seksjon med tittel, beskrivelse, handlingsknapper og hovednavigasjon.",
      },
    },
  },
  args: {
    title: "Copilot i Nav",
    description: "Nyheter, beste praksis og verktøy for AI-drevet utvikling i Nav.",
    pathname: "/praksis",
    actions: <Button size="small">Kom i gang</Button>,
    badge: <span className="rounded-full bg-white/15 px-3 py-1 text-sm">Beta</span>,
  },
} satisfies Meta<typeof PageHeroBase>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
