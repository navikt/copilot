import type { Meta, StoryObj } from "@storybook/nextjs";
import { TerminalIcon, MonitorIcon } from "@navikt/aksel-icons";
import {
  InteractiveSetupWizard,
  ChoiceCard,
  StepAccess,
  StepOS,
  StepWorkflow,
  StepStack,
  StepResult,
} from "./interactive-setup-wizard";

const meta: Meta<typeof InteractiveSetupWizard> = {
  title: "nav-pilot/InteractiveSetupWizard",
  component: InteractiveSetupWizard,
  tags: ["autodocs"],
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Hovedkomponenten for 'Kom i gang'-veiviseren. Samler inn informasjon om brukerens operativsystem, foretrukne arbeidsflyt og tech-stack, for å generere et ferdig terminalscript.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof InteractiveSetupWizard>;

/**
 * Hele veiviseren slik den vises for brukeren med state-håndtering og animasjoner mellom stegene.
 */
export const Default: Story = {
  render: () => (
    <div className="max-w-4xl mx-auto w-full">
      <InteractiveSetupWizard />
    </div>
  ),
};

// === Underkomponenter ===

/**
 * Kort som representerer et valgt alternativ i veiviseren (blå stil).
 */
export const CardSelected: StoryObj<typeof ChoiceCard> = {
  render: () => (
    <div className="max-w-md">
      <ChoiceCard
        selected={true}
        onClick={() => {}}
        icon={<TerminalIcon aria-hidden />}
        title="GitHub Copilot CLI (Anbefalt)"
        description="GitHubs offisielle kodingsagent i terminalen. En kraftig autonom agent for store kodeendringer, refaktorering og terminalarbeid."
      />
    </div>
  ),
};

/**
 * Kort som representerer et ikke-valgt alternativ (hvit stil med hover-effekt).
 */
export const CardUnselected: StoryObj<typeof ChoiceCard> = {
  render: () => (
    <div className="max-w-md">
      <ChoiceCard
        selected={false}
        onClick={() => {}}
        icon={<MonitorIcon aria-hidden />}
        title="OpenCode"
        description="Åpen kildekode-alternativ. Fullverdig autonom agent med et TUI-grensesnitt for de som foretrekker det fremfor GitHubs CLI."
      />
    </div>
  ),
};

/**
 * Steg 1: Informasjon om tilgang til Copilot Business.
 */
export const AccessStep: StoryObj<typeof StepAccess> = {
  render: () => (
    <div className="max-w-4xl p-8 border rounded-xl bg-white">
      <StepAccess onNext={() => {}} />
    </div>
  ),
};

/**
 * Steg 2: Valg av operativsystem (macOS, Linux, Windows).
 */
export const OsStep: StoryObj<typeof StepOS> = {
  render: () => (
    <div className="max-w-4xl p-8 border rounded-xl bg-white">
      <StepOS os="mac" setOs={() => {}} onPrev={() => {}} onNext={() => {}} />
    </div>
  ),
};

/**
 * Steg 3: Valg av arbeidsflyt (Copilot CLI, OpenCode, eller IDE).
 */
export const WorkflowStep: StoryObj<typeof StepWorkflow> = {
  render: () => (
    <div className="max-w-4xl p-8 border rounded-xl bg-white">
      <StepWorkflow workflow="cli" setWorkflow={() => {}} onPrev={() => {}} onNext={() => {}} />
    </div>
  ),
};

/**
 * Steg 4: Valg av tech-stack for å konfigurere Nav-kontekst.
 */
export const StackStep: StoryObj<typeof StepStack> = {
  render: () => (
    <div className="max-w-4xl p-8 border rounded-xl bg-white">
      <StepStack stack="kotlin-backend" setStack={() => {}} onPrev={() => {}} onNext={() => {}} />
    </div>
  ),
};

/**
 * Steg 5 (Siste): Viser det resulterende oppsettsscriptet basert på valgene.
 */
export const ResultStep: StoryObj<typeof StepResult> = {
  render: () => (
    <div className="max-w-4xl p-8 border rounded-xl bg-white">
      <StepResult os="linux" workflow="opencode" stack="nextjs-frontend" onPrev={() => {}} />
    </div>
  ),
};
