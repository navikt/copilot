"use client";

import { useState } from "react";
import { Box, VStack, HStack, Heading, BodyShort, ToggleGroup, Label, Link, Detail } from "@navikt/ds-react";
import { MonitorIcon, TerminalIcon, LaptopIcon } from "@navikt/aksel-icons";

type GoalId = "write" | "refactor" | "learn" | "review";

export function PraksisSelector() {
  const [goal, setGoal] = useState<GoalId>("write");

  const GOALS: Record<
    GoalId,
    {
      title: string;
      description: string;
      client: string;
      clientIcon: React.ReactNode;
      advice: string;
      links: { label: string; href: string }[];
    }
  > = {
    write: {
      title: "Skrive ny kode",
      description: "Generere nye funksjoner, komponenter eller filer fra bunnen av.",
      client: "IDE (VS Code / JetBrains)",
      clientIcon: <LaptopIcon aria-hidden />,
      advice:
        "Bruk inline autocomplete (Tab) for små snutter. For nye filer eller moduler, bruk Copilot Chat (⌘I) med en prompt-mal som #nextjs-api-route for å få riktig struktur umiddelbart.",
      links: [
        { label: "Verktøy og moduser", href: "#verktøy-og-moduser" },
        { label: "Prompt engineering", href: "#prompt-engineering" },
      ],
    },
    refactor: {
      title: "Refaktorere",
      description: "Gjøre større endringer på tvers av flere filer.",
      client: "OpenCode (Terminal)",
      clientIcon: <TerminalIcon aria-hidden />,
      advice:
        "Bruk OpenCode for dype endringer. Start gjerne med `nav-pilot --client opencode --agent plan` for å planlegge refaktoreringen. Agenten har tilgang til Nav-kontekst for å oppdatere riktig.",
      links: [
        { label: "WRAP-metoden for agent mode", href: "#wrap-metoden-for-coding-agent" },
        { label: "Vanlige mønstre", href: "#vanlige-mønstre-for-agent-mode" },
      ],
    },
    learn: {
      title: "Lære & Utforske",
      description: "Forstå ukjent kode, avluse (debugge), eller lære et nytt konsept.",
      client: "IDE Chat / Copilot CLI",
      clientIcon: <LaptopIcon aria-hidden />,
      advice:
        "Bruk @workspace i IDE for å spørre om arkitektur, eller bruk `copilot explain` i terminalen for å få forklart en fil eller et script.",
      links: [
        { label: "Gjennomgå Copilots arbeid", href: "#gjennomgå-copilots-arbeid" },
        { label: "Verifisering", href: "#verifisering-nøkkelen-til-kvalitet" },
      ],
    },
    review: {
      title: "Kode-review",
      description: "Gjennomgå PR-er eller sjekke sikkerhet.",
      client: "GitHub.com / OpenCode",
      clientIcon: <MonitorIcon aria-hidden />,
      advice:
        "Be @copilot reviewe en Pull Request på GitHub, eller bruk sikkerhets-skillen lokalt i OpenCode for å skanne etter sårbarheter før du committer.",
      links: [
        { label: "Forbered for suksess", href: "#forbered-for-suksess" },
        { label: "Effektive tilpasninger", href: "#skriv-effektive-tilpasninger" },
      ],
    },
  };

  const selected = GOALS[goal];

  return (
    <Box
      background="default"
      borderRadius="12"
      borderWidth="1"
      borderColor="neutral-subtle"
      padding="space-24"
      className="mb-8"
    >
      <VStack gap="space-16">
        <Heading size="small" level="2">
          Finn din beste arbeidsflyt
        </Heading>
        <BodyShort size="small" textColor="subtle">
          Hva prøver du å oppnå akkurat nå? Velg målet ditt, så anbefaler vi riktig klient og praksis.
        </BodyShort>

        <VStack gap="space-8" marginBlock="space-8">
          <Label size="small">Hva er målet?</Label>
          <ToggleGroup size="small" value={goal} onChange={(v) => setGoal(v as GoalId)}>
            <ToggleGroup.Item value="write">Skrive ny kode</ToggleGroup.Item>
            <ToggleGroup.Item value="refactor">Refaktorere</ToggleGroup.Item>
            <ToggleGroup.Item value="learn">Lære & utforske</ToggleGroup.Item>
            <ToggleGroup.Item value="review">Kode-review</ToggleGroup.Item>
          </ToggleGroup>
        </VStack>

        <Box background="accent-soft" borderRadius="12" padding="space-16" borderWidth="1" borderColor="accent-subtle">
          <HStack gap="space-16" align="start">
            <div style={{ flex: 1 }}>
              <VStack gap="space-12">
                <HStack gap="space-8" align="center">
                  <div className="text-blue-700">{selected.clientIcon}</div>
                  <Heading size="xsmall" level="3" className="text-blue-700">
                    Anbefalt: {selected.client}
                  </Heading>
                </HStack>
                <BodyShort size="small">{selected.advice}</BodyShort>
                <HStack gap="space-12" wrap>
                  <Detail textColor="subtle">Les mer:</Detail>
                  {selected.links.map((link) => (
                    <Link key={link.href} href={link.href}>
                      {link.label} →
                    </Link>
                  ))}
                </HStack>
              </VStack>
            </div>
          </HStack>
        </Box>
      </VStack>
    </Box>
  );
}
