import { HStack } from "@navikt/ds-react";
import { ChatIcon, CodeIcon, CpuIcon, RobotIcon, TerminalIcon, WrenchIcon } from "@navikt/aksel-icons";
import {
  SiClaude,
  SiGooglegemini,
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
  const Icon =
    kind === "language"
      ? (languageIcons[value.toLowerCase() as keyof typeof languageIcons] ?? CodeIcon)
      : kind === "model"
        ? value.toLowerCase().startsWith("claude")
          ? SiClaude
          : value.toLowerCase().startsWith("gemini")
            ? SiGooglegemini
            : CpuIcon
        : value === "copilot_cli"
          ? TerminalIcon
          : value.includes("agent")
            ? RobotIcon
            : value.startsWith("chat_")
              ? ChatIcon
              : WrenchIcon;
  return (
    <HStack gap="space-8" align="center" wrap={false}>
      <Icon aria-hidden="true" width={16} height={16} className="shrink-0" />
      <span>{kind === "feature" ? (featureLabels[value] ?? value) : value}</span>
    </HStack>
  );
}
