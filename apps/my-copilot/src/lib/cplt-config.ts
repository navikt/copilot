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

const CONFIG_RS_URL = "https://raw.githubusercontent.com/navikt/cplt/main/src/config.rs";

const TYPE_MAP: Record<string, string> = {
  Bool: "bool",
  U16: "integer",
  Str: "string",
  U16Array: "integer[]",
  StrArray: "string[]",
};

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

function parseConfigKeys(source: string): CpltConfigKey[] {
  const keys: CpltConfigKey[] = [];

  // Match each ConfigKeyInfo struct block
  const pattern = /ConfigKeyInfo\s*\{([^}]+)\}/g;
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(source)) !== null) {
    const block = match[1];

    const section = extractField(block, "section");
    const key = extractField(block, "key");
    const description = extractField(block, "description");
    const defaultDisplay = extractField(block, "default_display");
    const valueType = extractEnumField(block, "value_type");
    const dangerous = block.includes("dangerous: true");

    if (section && key) {
      keys.push({
        key: `${section}.${key}`,
        section,
        type: TYPE_MAP[valueType] || valueType || "string",
        default: defaultDisplay || "",
        description: description || "",
        dangerous,
      });
    }
  }

  return keys;
}

function extractField(block: string, field: string): string {
  const re = new RegExp(`${field}:\\s*"([^"]*)"`, "s");
  const m = block.match(re);
  return m ? m[1] : "";
}

function extractEnumField(block: string, field: string): string {
  const re = new RegExp(`${field}:\\s*ConfigValueType::(\\w+)`);
  const m = block.match(re);
  return m ? m[1] : "";
}
