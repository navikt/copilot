import type { Meta, StoryObj } from "@storybook/nextjs";
import { BodyLong, Box, Heading, List } from "@navikt/ds-react";

function StorybookGuidelinesDoc() {
  return (
    <Box className="rounded-xl bg-white" padding="space-24">
      <Heading level="2" size="medium" spacing>
        Storybook-guidelines for videokomponenter
      </Heading>
      <BodyLong spacing>Denne siden beskriver anbefalt struktur og kultur for stories i my-copilot.</BodyLong>

      <Heading level="3" size="small" spacing>
        Struktur
      </Heading>
      <List as="ul">
        <List.Item>
          <code>Video/Primitives/*</code> for små byggesteiner
        </List.Item>
        <List.Item>
          <code>Video/Controls/*</code> for interaktive kontroller
        </List.Item>
        <List.Item>
          <code>Video/Panels/*</code> for informasjonsflater
        </List.Item>
        <List.Item>
          <code>Video/HUD/*</code> for sammensatt overlay-lag
        </List.Item>
        <List.Item>
          <code>Video/Pages/*</code> for full side-/feature-komposisjon
        </List.Item>
      </List>

      <Heading level="3" size="small" spacing>
        Navngivning
      </Heading>
      <List as="ul">
        <List.Item>Story-titler skal være konkrete (Paused, Playing, Completed)</List.Item>
        <List.Item>docs.description.component skal forklare hva komponenten brukes til</List.Item>
        <List.Item>Story-beskrivelse skal forklare hvilken tilstand som demonstreres</List.Item>
      </List>

      <Heading level="3" size="small" spacing>
        Datagrunnlag
      </Heading>
      <List as="ul">
        <List.Item>Bruk realistiske fixtures fra dev-bucket når mulig</List.Item>
        <List.Item>Hold fallback-fixtures stabile for deterministisk UI-testing</List.Item>
        <List.Item>Unngå skjøre stories som krever live backend for å rendre</List.Item>
      </List>

      <Heading level="3" size="small" spacing>
        Tilgjengelighet og designkvalitet
      </Heading>
      <List as="ul">
        <List.Item>Verifiser tastaturfokus i alle interaktive stories</List.Item>
        <List.Item>Bruk mørk bakgrunn for videokomponenter for korrekt kontrastvurdering</List.Item>
        <List.Item>Bruk Aksel spacing-tokens via Box/designsystem der spacing må styres</List.Item>
      </List>

      <Heading level="3" size="small" spacing>
        Kultur for review
      </Heading>
      <List as="ol">
        <List.Item>Legg til minst én story for normaltilstand</List.Item>
        <List.Item>Legg til minst én story for edge state (tom, lang tekst, completed, etc.)</List.Item>
        <List.Item>Legg inn kort doc-beskrivelse av forventet bruk</List.Item>
        <List.Item>Kjør pnpm build-storybook før PR</List.Item>
      </List>
    </Box>
  );
}

const meta = {
  title: "Video/Dokumentasjon/Storybook Guidelines",
  component: StorybookGuidelinesDoc,
  tags: ["autodocs"],
} satisfies Meta<typeof StorybookGuidelinesDoc>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Guidelines: Story = {};
