export type ClientId = "copilot" | "opencode" | "interactive";
export type SurfaceId = "terminal" | "editor";
export type CollectionId = "kotlin-backend" | "nextjs-frontend" | "fullstack";

export interface BuilderSelection {
  client: ClientId;
  surface: SurfaceId;
  collection: CollectionId;
}

export interface BuiltCommands {
  /** One-time setup / install line, if relevant. */
  install: string;
  /** The primary launch command for the chosen combination. */
  launch: string;
  /** A short, contextual tip rendered under the command. */
  tip: string;
  /** Human label for the resolved client. */
  clientLabel: string;
}

const CLIENT_LABEL: Record<ClientId, string> = {
  copilot: "GitHub Copilot CLI",
  opencode: "OpenCode",
  interactive: "nav-pilot interaktiv",
};

const COLLECTION_MODEL: Record<CollectionId, string> = {
  "kotlin-backend": "github-copilot/claude-sonnet-4.5",
  "nextjs-frontend": "github-copilot/claude-sonnet-4.5",
  fullstack: "github-copilot/claude-opus-4.6",
};

const COLLECTION_LABEL: Record<CollectionId, string> = {
  "kotlin-backend": "Kotlin-backend",
  "nextjs-frontend": "Next.js-frontend",
  fullstack: "Fullstack",
};

/**
 * Pure mapping from a user selection to the concrete commands we want to show.
 * Kept free of React so it can be unit-tested and reused across visual variants.
 */
export function buildCommands(sel: BuilderSelection): BuiltCommands {
  const clientLabel = CLIENT_LABEL[sel.client];
  const model = COLLECTION_MODEL[sel.collection];

  if (sel.surface === "editor") {
    return {
      install: "nav-pilot sync",
      launch: "@nav-pilot Lag en plan for …",
      tip: `Skriv @nav-pilot i ${sel.client === "opencode" ? "OpenCode" : "Copilot Chat"} for å starte planleggingen direkte i editoren.`,
      clientLabel,
    };
  }

  if (sel.client === "interactive") {
    return {
      install: "nav-pilot config setup",
      launch: "nav-pilot",
      tip: "Uten flagg spør nav-pilot deg interaktivt om klient, modell og modus — basert på defaultene dine.",
      clientLabel,
    };
  }

  if (sel.client === "opencode") {
    return {
      install: "nav-pilot config set client opencode",
      launch: `nav-pilot --client opencode --model ${model}`,
      tip: "nav-pilot materialiserer Nav-konteksten i ~/.config/opencode og starter nav-pilot som primær-agent automatisk.",
      clientLabel,
    };
  }

  return {
    install: "nav-pilot config set client copilot",
    launch: `nav-pilot --client copilot --model ${model}`,
    tip: `Starter Copilot CLI med @nav-pilot forhåndsvalgt for ${COLLECTION_LABEL[sel.collection]}.`,
    clientLabel,
  };
}

export const COLLECTIONS: { id: CollectionId; label: string }[] = [
  { id: "kotlin-backend", label: "Kotlin-backend" },
  { id: "nextjs-frontend", label: "Next.js-frontend" },
  { id: "fullstack", label: "Fullstack" },
];
