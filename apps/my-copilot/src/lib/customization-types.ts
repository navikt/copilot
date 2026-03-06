export type CustomizationType = "agent" | "instruction" | "prompt" | "skill" | "mcp";

export type Domain = "platform" | "frontend" | "backend" | "auth" | "observability" | "general";

interface BaseCustomization {
  id: string;
  name: string;
  description: string;
  type: CustomizationType;
  domain: Domain;
  filePath: string;
  rawGitHubUrl: string;
  installUrl: string | null;
  insidersInstallUrl: string | null;
}

export interface Agent extends BaseCustomization {
  type: "agent";
  tools: string[];
}

export interface Instruction extends BaseCustomization {
  type: "instruction";
  applyTo: string;
}

export interface Prompt extends BaseCustomization {
  type: "prompt";
  invocation: string;
}

export interface Skill extends BaseCustomization {
  type: "skill";
}

export interface McpServerCustomization extends BaseCustomization {
  type: "mcp";
  version: string;
  remotes: { type: string; url: string }[];
}

export type AnyCustomization = Agent | Instruction | Prompt | Skill | McpServerCustomization;

export interface DomainConfig {
  label: string;
  description: string;
  color: "blue" | "green" | "orange" | "purple" | "red";
  background: "info-soft" | "success-soft" | "warning-soft" | "accent-soft" | "danger-soft";
}

export const DOMAIN_CONFIGS: Record<Domain, DomainConfig> = {
  platform: {
    label: "Plattform",
    description: "Nais, Kubernetes, deploy og infrastruktur",
    color: "blue",
    background: "info-soft",
  },
  frontend: {
    label: "Frontend",
    description: "React, Next.js, Aksel Design System",
    color: "green",
    background: "success-soft",
  },
  backend: {
    label: "Backend",
    description: "Kotlin, Ktor, database, Kafka",
    color: "orange",
    background: "warning-soft",
  },
  auth: {
    label: "Sikkerhet",
    description: "Azure AD, TokenX, ID-porten, trusselmodellering",
    color: "purple",
    background: "accent-soft",
  },
  observability: {
    label: "Observability",
    description: "Prometheus, Grafana, OpenTelemetry, logging",
    color: "red",
    background: "danger-soft",
  },
  general: {
    label: "Generelt",
    description: "Forskning, analyse og generelle verktøy",
    color: "blue",
    background: "info-soft",
  },
};

export const TYPE_LABELS: Record<CustomizationType, string> = {
  agent: "Agent",
  instruction: "Instruksjon",
  prompt: "Prompt",
  skill: "Ferdighet",
  mcp: "MCP Server",
};
