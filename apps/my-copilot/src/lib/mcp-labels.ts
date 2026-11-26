// One set of words for what an MCP tool does (#3081). API values stay as they are; only the labels are shared.

export type Effect = "read" | "propose" | "files" | "acts" | "unsandboxed";

export const LEVELS_HREF = "/mcp/referanse#verktoynivaer";
export const DATA_HREF = "/mcp/referanse#datakategorier";

export const EFFECT: Record<Effect, { label: string; tag: "neutral" | "info" | "warning" | "error"; text: string }> = {
  read: { label: "Leser", tag: "neutral", text: "Henter eller søker i data. Endrer ingenting." },
  propose: {
    label: "Foreslår",
    tag: "info",
    text: "Lager et forslag eller utkast som et menneske må godta før noe endres.",
  },
  files: {
    label: "Endrer filer",
    tag: "info",
    text: "Endrer filer i repoet på maskinen din. Gjelder bare servere som kjører lokalt.",
  },
  acts: {
    label: "Utfører",
    tag: "warning",
    text: "Endrer noe i et annet system, for eksempel lager en issue, bestiller testdata eller klikker i nettleseren.",
  },
  unsandboxed: {
    label: "Fri tilgang",
    tag: "error",
    text: "Kjører kommandoer på maskinen din utenfor cplt-sandkassen. Agenten kan da gjøre alt du kan.",
  },
};

/** Registry (read/write/external/host-exec) or fleet (read/propose/write) value to label. A remote server never changes your files and never runs outside the sandbox, so it never shows «Endrer filer» or «Fri tilgang». */
export function effectOf(risk: string, local: boolean): Effect {
  if (risk === "read" || risk === "propose") return risk;
  if (risk === "write") return local ? "files" : "acts";
  if (risk === "host-exec") return local ? "unsandboxed" : "acts";
  // Unknown value: fail closed, the most serious label this server can have.
  return local ? "unsandboxed" : "acts";
}

export const runsWhere = (local: boolean) => (local ? "Kjører lokalt" : "Kjører eksternt");
