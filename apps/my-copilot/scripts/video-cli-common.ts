const VALID_TAG_RE = /^[a-z0-9][a-z0-9-]{0,31}$/;

export function arg(name: string): string | undefined {
  const index = process.argv.indexOf(`--${name}`);
  if (index < 0) return undefined;
  return process.argv[index + 1];
}

export function required(name: string): string {
  const value = arg(name);
  if (!value) {
    throw new Error(`Missing required argument --${name}`);
  }
  return value;
}

export function parsePositiveInteger(name: string, value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    throw new Error(`Invalid value for --${name}; expected a positive integer`);
  }
  return parsed;
}

export function parseNonNegativeInteger(name: string, value: string): number {
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new Error(`Invalid value for --${name}; expected a non-negative integer`);
  }
  return parsed;
}

export function parseOptionalPositiveInteger(name: string, value: string | undefined): number | undefined {
  if (value === undefined) return undefined;
  return parsePositiveInteger(name, value);
}

export function parseTags(raw: string | undefined): string[] {
  if (!raw) return [];
  const tags = raw
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean);
  if (tags.length > 20) {
    throw new Error("Invalid --tags; expected at most 20 tags");
  }

  const seen = new Set<string>();
  for (const tag of tags) {
    if (!VALID_TAG_RE.test(tag)) {
      throw new Error(`Invalid tag "${tag}"; use lowercase letters, numbers, and hyphens`);
    }
    if (seen.has(tag)) {
      throw new Error(`Duplicate tag "${tag}"`);
    }
    seen.add(tag);
  }
  return tags;
}
