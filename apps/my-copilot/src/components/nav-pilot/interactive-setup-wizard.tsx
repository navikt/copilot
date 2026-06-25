/* eslint-disable react-hooks/set-state-in-effect */
"use client";

import { useState, useEffect } from "react";
import { Box, VStack, HStack, Heading, BodyShort, Button, Link, Stepper, Label, Detail } from "@navikt/ds-react";
import {
  MonitorIcon,
  LaptopIcon,
  TerminalIcon,
  ChevronRightIcon,
  ChevronLeftIcon,
  CheckmarkIcon,
} from "@navikt/aksel-icons";
import { CodeBlock } from "@/components/code-block";
import { COLLECTIONS, type CollectionId } from "./command-builder";

// ============================================================================
// Types
// ============================================================================

export type OS = "mac" | "linux" | "windows";
export type Workflow = "editor" | "cli" | "opencode";

interface SetupCommandBlock {
  title: string;
  commands: string[];
}

const WORKFLOW_COMMANDS: Record<Workflow, string[]> = {
  cli: ["nav-pilot"],
  opencode: ["nav-pilot config set client opencode", "nav-pilot --client opencode"],
  editor: [],
};

// ============================================================================
// Business Logic (Pure Functions)
// ============================================================================

export function generateSetupScript(os: OS, workflow: Workflow, stack: CollectionId) {
  if (workflow === "editor") {
    return {
      title: "Klar for koding i editoren!",
      steps: [
        "1. Åpne VS Code eller IntelliJ.",
        "2. Installer utvidelsen 'GitHub Copilot'.",
        "3. Logg inn med GitHub-kontoen din (krever navikt-tilgang).",
        "4. Begynn å skrive kode! Bruk Tab for å godta forslag, eller ⌘+I for å åpne Copilot Chat.",
      ],
      code: null,
    };
  }

  const blocks: SetupCommandBlock[] = [];

  if (os === "windows") {
    blocks.push({
      title:
        "# Nav-pilot (agent og context) fungerer best i WSL (Linux).\n# Åpne WSL2-terminalen din og kjør følgende:",
      commands: [],
    });
  }

  const isMac = os === "mac";

  if (workflow === "cli") {
    blocks.push({ title: "# 1. Installer Copilot CLI (NPM)", commands: ["npm install -g @github/copilot"] });
  } else if (workflow === "opencode") {
    blocks.push({ title: "# 1. Installer OpenCode", commands: ["curl -fsSL https://opencode.ai/install | bash"] });
  }

  if (isMac) {
    blocks.push({
      title: "# 2. Installer Nav-verktøy (inkluderer rtk for token-sparing)",
      commands: ["brew install navikt/tap/nav-pilot navikt/tap/cplt rtk"],
    });
  } else {
    blocks.push({
      title: "# 2. Last ned verktøy (inkluderer rtk for token-sparing og sandbox)",
      commands: ["curl -fsSL https://raw.githubusercontent.com/navikt/copilot/main/scripts/install.sh | bash"],
    });
  }

  blocks.push({
    title: "# 3. Sett opp for ditt prosjekt",
    commands: [`nav-pilot install ${stack}`, ...WORKFLOW_COMMANDS[workflow]],
  });

  const codeString = blocks.map((b) => [b.title, ...b.commands].filter(Boolean).join("\n")).join("\n\n");

  return {
    title: "Kopier denne oppskriften i terminalen",
    steps: ["Oppskriften under installerer alt du trenger og setter opp Nav-kontekst for repoet ditt automatisk."],
    code: codeString,
  };
}

// ============================================================================
// UI Components
// ============================================================================

export function ChoiceCard({
  selected,
  onClick,
  icon,
  title,
  description,
}: {
  selected: boolean;
  onClick: () => void;
  icon: React.ReactNode;
  title: string;
  description: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={`text-left w-full p-5 rounded-xl border-2 transition-all duration-150 ${
        selected
          ? "border-blue-500 bg-blue-50 shadow-md"
          : "border-gray-200 bg-white hover:border-blue-300 hover:bg-gray-50"
      }`}
    >
      <HStack gap="space-12" align="start">
        <div className={`text-2xl ${selected ? "text-blue-600" : "text-gray-500"}`}>{icon}</div>
        <VStack gap="space-2">
          <Label size="small" className={selected ? "text-blue-800" : "text-gray-800"}>
            {title}
          </Label>
          <Detail className={selected ? "text-blue-700" : "text-gray-600"}>{description}</Detail>
        </VStack>
      </HStack>
    </button>
  );
}

export function StepAccess({ onNext }: { onNext: () => void }) {
  return (
    <VStack gap="space-16" align="center" className="text-center">
      <Heading size="medium" level="2">
        Har du aktivert Copilot?
      </Heading>
      <BodyShort textColor="subtle" className="max-w-md">
        Alle utviklere i navikt-organisasjonen kan gi seg selv tilgang til GitHub Copilot Business helt gratis.
      </BodyShort>
      <Link href="/abonnement" target="_blank" className="text-blue-600 mb-4">
        Sjekk /abonnement siden
      </Link>
      <Button onClick={onNext} icon={<ChevronRightIcon aria-hidden />} iconPosition="right" size="medium">
        Ja, jeg har tilgang
      </Button>
    </VStack>
  );
}

export function StepOS({
  os,
  setOs,
  onPrev,
  onNext,
}: {
  os: OS;
  setOs: (val: OS) => void;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <VStack gap="space-16">
      <VStack gap="space-4" align="center" className="text-center">
        <Heading size="medium" level="2">
          Hvilket OS bruker du?
        </Heading>
        <BodyShort textColor="subtle">
          Vi gjetter at du er på {os === "mac" ? "macOS" : os === "linux" ? "Linux" : "Windows"}. Stemmer det?
        </BodyShort>
      </VStack>
      <HStack gap="space-12" justify="center" wrap>
        {(["mac", "linux", "windows"] as OS[]).map((val) => (
          <Button
            key={val}
            variant={os === val ? "primary" : "secondary"}
            onClick={() => {
              setOs(val);
              onNext();
            }}
          >
            {val === "mac" ? "macOS" : val === "linux" ? "Linux" : "Windows"}
          </Button>
        ))}
      </HStack>
      <HStack justify="center" marginBlock="space-16">
        <Button variant="tertiary" onClick={onPrev} icon={<ChevronLeftIcon aria-hidden />}>
          Tilbake
        </Button>
      </HStack>
    </VStack>
  );
}

export function StepWorkflow({
  workflow,
  setWorkflow,
  onPrev,
  onNext,
}: {
  workflow: Workflow;
  setWorkflow: (val: Workflow) => void;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <VStack gap="space-16">
      <VStack gap="space-4" align="center" className="text-center mb-4">
        <Heading size="medium" level="2">
          Hvordan vil du jobbe med AI?
        </Heading>
        <BodyShort textColor="subtle">Velg det verktøyet som passer best for oppgaven du skal løse nå.</BodyShort>
      </VStack>
      <VStack gap="space-12" className="max-w-xl mx-auto w-full">
        <ChoiceCard
          selected={workflow === "cli"}
          onClick={() => {
            setWorkflow("cli");
            onNext();
          }}
          icon={<TerminalIcon aria-hidden />}
          title="GitHub Copilot CLI (Anbefalt)"
          description="GitHubs offisielle kodingsagent i terminalen. En kraftig autonom agent for store kodeendringer, refaktorering og terminalarbeid."
        />
        <ChoiceCard
          selected={workflow === "opencode"}
          onClick={() => {
            setWorkflow("opencode");
            onNext();
          }}
          icon={<MonitorIcon aria-hidden />}
          title="OpenCode"
          description="Åpen kildekode-alternativ. Fullverdig autonom agent med et TUI-grensesnitt for de som foretrekker det fremfor GitHubs CLI."
        />
        <ChoiceCard
          selected={workflow === "editor"}
          onClick={() => {
            setWorkflow("editor");
            onNext();
          }}
          icon={<LaptopIcon aria-hidden />}
          title="I Editoren (VS Code / IntelliJ)"
          description="Sanntids kodeforslag og chat i editoren. Perfekt for små endringer og generering av enkel funksjoner."
        />
      </VStack>
      <HStack justify="center" marginBlock="space-16">
        <Button variant="tertiary" onClick={onPrev} icon={<ChevronLeftIcon aria-hidden />}>
          Tilbake
        </Button>
      </HStack>
    </VStack>
  );
}

export function StepStack({
  stack,
  setStack,
  onPrev,
  onNext,
}: {
  stack: CollectionId;
  setStack: (val: CollectionId) => void;
  onPrev: () => void;
  onNext: () => void;
}) {
  return (
    <VStack gap="space-16">
      <VStack gap="space-4" align="center" className="text-center mb-4">
        <Heading size="medium" level="2">
          Hva bygger du primært?
        </Heading>
        <BodyShort textColor="subtle">Dette lar oss legge inn riktig Nav-kontekst (Skills og Instruksjoner).</BodyShort>
      </VStack>
      <HStack justify="center" gap="space-12" wrap className="max-w-2xl mx-auto w-full">
        {COLLECTIONS.map((c) => (
          <ChoiceCard
            key={c.id}
            selected={stack === c.id}
            onClick={() => {
              setStack(c.id);
              onNext();
            }}
            icon={<CheckmarkIcon aria-hidden />}
            title={c.label}
            description=""
          />
        ))}
      </HStack>
      <HStack justify="center" marginBlock="space-16">
        <Button variant="tertiary" onClick={onPrev} icon={<ChevronLeftIcon aria-hidden />}>
          Tilbake
        </Button>
      </HStack>
    </VStack>
  );
}

export function StepResult({
  os,
  workflow,
  stack,
  onPrev,
}: {
  os: OS;
  workflow: Workflow;
  stack: CollectionId;
  onPrev: () => void;
}) {
  const currentResult = generateSetupScript(os, workflow, stack);

  return (
    <VStack gap="space-16" className="max-w-3xl mx-auto w-full">
      <VStack gap="space-4" align="center" className="text-center mb-4">
        <Heading size="medium" level="2">
          {currentResult.title}
        </Heading>
        {currentResult.steps.map((text, idx) => (
          <BodyShort key={idx} textColor="subtle">
            {text}
          </BodyShort>
        ))}
      </VStack>

      {currentResult.code && (
        <Box background="default" borderRadius="8" padding="space-16">
          <CodeBlock compact>{currentResult.code}</CodeBlock>
        </Box>
      )}

      <HStack justify="center" gap="space-16" marginBlock="space-16">
        <Button variant="tertiary" onClick={onPrev} icon={<ChevronLeftIcon aria-hidden />}>
          Tilbake
        </Button>
        <Button as="a" href="/praksis" variant="secondary">
          Gå til God Praksis →
        </Button>
      </HStack>
    </VStack>
  );
}

// ============================================================================
// Main Orchestrator
// ============================================================================

export function InteractiveSetupWizard() {
  const [activeStep, setActiveStep] = useState(1);
  const [hasDetected, setHasDetected] = useState(false);

  const [os, setOs] = useState<OS>("mac");
  const [workflow, setWorkflow] = useState<Workflow>("cli");
  const [stack, setStack] = useState<CollectionId>("kotlin-backend");

  useEffect(() => {
    const platform = (navigator.userAgent || navigator.platform)?.toLowerCase() || "";
    if (platform.includes("win")) {
      setOs("windows");
    } else if (platform.includes("linux")) {
      setOs("linux");
    } else {
      setOs("mac");
    }
    setHasDetected(true);
  }, []);

  const nextStep = () => setActiveStep((prev) => Math.min(prev + 1, 5));
  const prevStep = () => setActiveStep((prev) => Math.max(prev - 1, 1));

  if (!hasDetected) return null;

  return (
    <Box
      background="default"
      borderRadius="12"
      borderWidth="1"
      borderColor="neutral-subtle"
      padding="space-24"
      className="shadow-sm"
    >
      <VStack gap="space-24">
        <Stepper activeStep={activeStep} onStepChange={setActiveStep} orientation="horizontal" interactive={false}>
          <Stepper.Step href="#" completed={activeStep > 1}>
            Tilgang
          </Stepper.Step>
          <Stepper.Step href="#" completed={activeStep > 2}>
            OS
          </Stepper.Step>
          <Stepper.Step href="#" completed={activeStep > 3}>
            Arbeidsflyt
          </Stepper.Step>
          <Stepper.Step href="#" completed={activeStep > 4}>
            Stack
          </Stepper.Step>
          <Stepper.Step href="#">Ferdig</Stepper.Step>
        </Stepper>

        <Box paddingBlock="space-16">
          {activeStep === 1 && <StepAccess onNext={nextStep} />}
          {activeStep === 2 && <StepOS os={os} setOs={setOs} onPrev={prevStep} onNext={nextStep} />}
          {activeStep === 3 && (
            <StepWorkflow workflow={workflow} setWorkflow={setWorkflow} onPrev={prevStep} onNext={nextStep} />
          )}
          {activeStep === 4 && <StepStack stack={stack} setStack={setStack} onPrev={prevStep} onNext={nextStep} />}
          {activeStep === 5 && <StepResult os={os} workflow={workflow} stack={stack} onPrev={prevStep} />}
        </Box>
      </VStack>
    </Box>
  );
}
