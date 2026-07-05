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

const CONFIG_RS_URL = "https://raw.githubusercontent.com/navikt/cplt/main/src/config/registry.rs";

export async function fetchCpltConfigKeys(): Promise<CpltConfigKey[]> {
  try {
    const res = await fetch(CONFIG_RS_URL, { next: { revalidate: 3600 } });
    if (!res.ok) return [];
    const source = await res.text();
    return parseConfigKeys(source);
  } catch {
    return [];
  }
}

/** Unescape the string escapes we expect in registry.rs literals (\" and \\). */
function unescapeRustString(value: string): string {
  return value.replace(/\\(["\\])/g, "$1");
}

function parseConfigKeys(source: string): CpltConfigKey[] {
  const keys: CpltConfigKey[] = [];
  // default_display and description may contain escaped quotes (\") in the Rust source.
  const pattern =
    /section:\s*"([^"]+)",\s*key:\s*"([^"]+)",\s*value_type:\s*ConfigValueType::([a-zA-Z0-9]+),\s*dangerous:\s*(true|false),\s*default_display:\s*"((?:[^"\\]|\\.)*)",\s*description:\s*"((?:[^"\\]|\\.)+)",/g;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(source)) !== null) {
    const section = match[1];
    const key = match[2];
    const kind = match[3];
    const dangerous = match[4] === "true";
    const defaultDisplay = unescapeRustString(match[5]);
    const description = unescapeRustString(match[6]);

    let typeStr = "string";
    if (kind === "U16") typeStr = "integer";
    if (kind === "U64") typeStr = "integer";
    if (kind === "U16Array") typeStr = "integer[]";
    if (kind === "Bool") typeStr = "bool";
    if (kind === "StrArray") typeStr = "string[]";
    if (kind === "ArrayOfTables") typeStr = "string[]"; // Hack for now

    keys.push({
      key: `${section}.${key}`,
      section,
      type: typeStr,
      default: defaultDisplay,
      description,
      dangerous,
    });
  }

  return keys;
}
