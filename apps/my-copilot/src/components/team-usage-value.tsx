import { HStack } from "@navikt/ds-react";
import { ChatIcon, CodeIcon, CpuIcon, RobotIcon, TerminalIcon, WrenchIcon } from "@navikt/aksel-icons";
import {
  SiGo,
  SiJavascript,
  SiKotlin,
  SiMarkdown,
  SiOpenjdk,
  SiPython,
  SiReact,
  SiRust,
  SiTypescript,
  SiYaml,
} from "@icons-pack/react-simple-icons";
import { MODEL_PRICING } from "@/lib/model-pricing";
import type { ModelPrice } from "@/lib/model-pricing";
import { normalizeModelName } from "@/lib/model-policy";
import { ModelProviderIcon } from "./model-icons";

const modelKey = (name: string) => normalizeModelName(name).toLowerCase().replace(/[ .]/g, "-");
const modelProviders = new Map(MODEL_PRICING.map((model) => [modelKey(model.model), model.provider]));
const providerFamilies: [RegExp, ModelPrice["provider"]][] = [
  [/^claude(?:-|$)/, "Anthropic"],
  [/^gemini(?:-|$)/, "Google"],
  [/^(?:gpt-|o[134](?:-|$))/, "OpenAI"],
  [/^mai-/, "Microsoft"],
  [/^kimi(?:-|$)/, "Moonshot AI"],
];

const languageIcons = {
  kotlin: SiKotlin,
  java: SiOpenjdk,
  typescript: SiTypescript,
  javascript: SiJavascript,
  tsx: SiReact,
  javascriptreact: SiReact,
  typescriptreact: SiReact,
  markdown: SiMarkdown,
  python: SiPython,
  go: SiGo,
  rust: SiRust,
  yaml: SiYaml,
};
const featureLabels: Record<string, string> = {
  copilot_cli: "Copilot CLI",
  copilot_app: "Copilot-app",
  chat_panel_agent_mode: "Agentmodus",
  chat_panel_ask_mode: "Spørremodus",
  chat_panel_plan_mode: "Planmodus",
  chat_panel_custom_mode: "Egendefinert modus",
  chat_inline: "Innebygd chat",
  agent_edit: "Agentredigering",
};

export default function TeamUsageValue({ value, kind }: { value: string; kind: "model" | "feature" | "language" }) {
  const key = modelKey(value);
  const provider =
    kind === "model"
      ? (modelProviders.get(key) ?? providerFamilies.find(([pattern]) => pattern.test(key))?.[1])
      : undefined;
  const Icon =
    kind === "language"
      ? (languageIcons[value.toLowerCase() as keyof typeof languageIcons] ?? CodeIcon)
      : kind === "model"
        ? CpuIcon
        : value === "copilot_cli"
          ? TerminalIcon
          : value.includes("agent")
            ? RobotIcon
            : value.startsWith("chat_")
              ? ChatIcon
              : WrenchIcon;
  return (
    <HStack gap="space-8" align="center" wrap={false}>
      {provider ? (
        <ModelProviderIcon provider={provider} />
      ) : (
        <Icon aria-hidden="true" width={16} height={16} className="shrink-0" />
      )}
      <span>{kind === "feature" ? (featureLabels[value] ?? value) : value}</span>
    </HStack>
  );
}
