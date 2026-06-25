/**
 * Fetches cplt config keys from the source of truth (src/config.rs in navikt/cplt).
 * Parses the Rust ConfigKeyInfo structs into a typed array.
 * Revalidates every hour to stay fresh without hammering GitHub.
 */

export type CpltConfigKey = {
  key: string;
  section: string;
  type: string;
  default: string;
  description: string;
  dangerous: boolean;
};

const CONFIG_GO_URL = "https://raw.githubusercontent.com/navikt/copilot/main/cli/nav-pilot/internal/cli/config_cmd.go";

export async function fetchCpltConfigKeys(): Promise<CpltConfigKey[]> {
  try {
    const res = await fetch(CONFIG_GO_URL, { next: { revalidate: 3600 } });
    if (!res.ok) return [];
    const source = await res.text();
    return parseConfigKeys(source);
  } catch {
    return [];
  }
}

function parseConfigKeys(source: string): CpltConfigKey[] {
  const keys: CpltConfigKey[] = [];
  const pattern =
    /name:\s*"([^"]+)",\s*kind:\s*(keyKind[a-zA-Z]+),\s*description:\s*"([^"]+)",[\s\S]*?defaultVal:\s*"([^"]*)",/g;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(source)) !== null) {
    const key = match[1];
    const kind = match[2];
    const description = match[3];
    const defaultDisplay = match[4];

    let typeStr = "string";
    if (kind === "keyKindInt") typeStr = "integer";
    if (kind === "keyKindBool") typeStr = "bool";

    keys.push({
      key,
      section: "general",
      type: typeStr,
      default: defaultDisplay,
      description,
      dangerous: false,
    });
  }

  return keys;
}
