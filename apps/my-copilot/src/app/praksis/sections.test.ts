import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { join } from "node:path";

/**
 * Aksel ships `"use client"` at the package root, so `@navikt/ds-react` is a
 * client module. A server component may render such a component, but it cannot
 * read properties off it: `Accordion.Item` resolves to `undefined` and React
 * fails with "Element type is invalid ... but got: undefined".
 *
 * Only `Accordion` is exported from the package root, so the subcomponents
 * cannot be imported by name as a workaround — the file has to be a client
 * component. Guide sections are rendered by a server component, so this is easy
 * to get wrong (see issue #378).
 */

const SECTIONS_DIR = join(__dirname, "sections");

// Aksel components that are only reachable through dot notation.
const COMPOUND_COMPONENTS = [
  "Accordion",
  "ActionMenu",
  "Dropdown",
  "ExpansionCard",
  "List",
  "Modal",
  "Stepper",
  "Table",
  "Tabs",
  "ToggleGroup",
];

const dotNotation = new RegExp(`<(${COMPOUND_COMPONENTS.join("|")})\\.`);

describe("praksis-seksjoner", () => {
  const files = readdirSync(SECTIONS_DIR).filter((f) => f.endsWith(".tsx"));

  it("finner seksjonsfiler", () => {
    expect(files.length).toBeGreaterThan(0);
  });

  it.each(files)('%s bruker ikke dot-notasjon på Aksel-komponenter uten "use client"', (file) => {
    const source = readFileSync(join(SECTIONS_DIR, file), "utf8");
    const match = source.match(dotNotation);

    if (!match) return;

    const isClientComponent = /^\s*(\/\/.*|\/\*[\s\S]*?\*\/)?\s*"use client";/m.test(source);

    expect(
      isClientComponent,
      `${file} bruker <${match[1]}.…> fra @navikt/ds-react. Legg til "use client" øverst i filen, ` +
        `ellers blir subkomponenten undefined når seksjonen rendres fra en server-komponent.`
    ).toBe(true);
  });
});
