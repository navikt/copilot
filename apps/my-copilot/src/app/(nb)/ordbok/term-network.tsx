"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { BodyShort, Box, Button, HStack, Heading, Search, VStack } from "@navikt/ds-react";
import { hasWebGL2 } from "@/lib/webgl";
import { GLOSSARY_RESET_EVENT } from "../ordliste/glossary";
import { deriveEdges, termId } from "../ordliste/term-graph";
import { categories, type Category, type Term } from "../ordliste/terms";
import type { TermNetworkCanvas } from "./term-network-canvas";

// three.js loads after hydration and only with WebGL 2. Without it the section is not rendered.
// Not next/dynamic with ssr:false: its BAILOUT_TO_CLIENT_SIDE_RENDERING digest trips hack/smoke.sh.
export function TermNetwork({ terms }: { terms: Term[] }) {
  const [Canvas, setCanvas] = useState<typeof TermNetworkCanvas | null>(null);
  const [selected, setSelected] = useState<number | null>(null);
  const [query, setQuery] = useState("");
  const [hidden, setHidden] = useState<Category[]>([]);
  const zoomRef = useRef<((factor: number) => void) | null>(null);
  const edges = useMemo(() => deriveEdges(terms), [terms]);
  const neighbours = useMemo(() => {
    const n = terms.map(() => new Set<number>());
    edges.forEach((e) => {
      n[e.from].add(e.to);
      n[e.to].add(e.from);
    });
    return n;
  }, [terms, edges]);

  useEffect(() => {
    if (!hasWebGL2()) return;
    import("./term-network-canvas").then((m) => setCanvas(() => m.TermNetworkCanvas));
  }, []);

  if (!Canvas) return null;

  const search = (value: string) => {
    setQuery(value);
    const q = value.trim().toLowerCase();
    if (!q) return;
    const names = terms.map((t) => t.term.toLowerCase());
    const hit = names.findIndex((n) => n.startsWith(q));
    const i = hit >= 0 ? hit : names.findIndex((n) => n.includes(q));
    if (i >= 0) setSelected(i);
  };
  const noHit =
    query.trim().length > 0 && !terms.some((t) => t.term.toLowerCase().includes(query.trim().toLowerCase()));

  const goTo = (i: number) => {
    // The glossary filter may hide the term: clear it, then scroll once the list has re-rendered.
    window.dispatchEvent(new Event(GLOSSARY_RESET_EVENT));
    requestAnimationFrame(() => requestAnimationFrame(() => scrollTo(i)));
  };
  const scrollTo = (i: number) => {
    const el = document.getElementById(termId(terms[i].term));
    if (!el) return;
    const smooth = !window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    el.scrollIntoView({ behavior: smooth ? "smooth" : "auto", block: "start" });
    el.focus({ preventScroll: true });
    history.replaceState(null, "", `#${el.id}`);
  };

  const term = selected === null ? null : terms[selected];

  return (
    <section aria-labelledby="begrepsnett-heading">
      <VStack gap="space-12">
        <VStack gap="space-4">
          <Heading id="begrepsnett-heading" size="medium" level="2">
            Begrepene henger sammen
          </Heading>
          <BodyShort className="opacity-80">
            Hvert punkt er et begrep. Store punkter nevnes i mange andre definisjoner. Dra sidelengs for å rotere, og
            trykk på et punkt for å lese om det. Zoom med knappene, med to fingre eller med Ctrl og musehjulet.
          </BodyShort>
        </VStack>
        <div className="md:w-1/2">
          <Search
            label="Finn et begrep i nettverket"
            variant="simple"
            size="small"
            value={query}
            onChange={search}
            onClear={() => {
              setQuery("");
              setSelected(null);
            }}
          />
          {noHit && (
            <BodyShort size="small" className="mt-1 opacity-70">
              Ingen begreper heter «{query.trim()}».
            </BodyShort>
          )}
        </div>
        <Box borderRadius="12" borderWidth="1" borderColor="neutral-subtle" className="overflow-hidden">
          <Canvas
            terms={terms}
            edges={edges}
            selected={selected}
            onSelect={setSelected}
            hidden={hidden}
            zoomRef={zoomRef}
          />
        </Box>
        <HStack gap="space-4" role="group" aria-label="Vis eller skjul kategorier i nettverket">
          {categories.map((c) => {
            const on = !hidden.includes(c.id);
            return (
              <Button
                key={c.id}
                size="xsmall"
                variant="tertiary-neutral"
                aria-pressed={on}
                className={on ? "" : "opacity-50 line-through"}
                icon={
                  <span
                    aria-hidden="true"
                    className="inline-block size-3 rounded-full"
                    style={{ background: `var(${c.token})` }}
                  />
                }
                onClick={() => setHidden((h) => (on ? [...h, c.id] : h.filter((x) => x !== c.id)))}
              >
                {c.label}
              </Button>
            );
          })}
        </HStack>
        <HStack gap="space-8">
          <Button size="small" variant="secondary-neutral" onClick={() => zoomRef.current?.(0.8)}>
            Zoom inn
          </Button>
          <Button size="small" variant="secondary-neutral" onClick={() => zoomRef.current?.(1.25)}>
            Zoom ut
          </Button>
        </HStack>
        <Box padding="space-16" borderRadius="12" background="neutral-soft" aria-live="polite">
          {term === null || selected === null ? (
            <BodyShort className="opacity-80">Trykk på et punkt, eller søk etter et begrep.</BodyShort>
          ) : (
            <VStack gap="space-8">
              <Heading size="small" level="3">
                {term.term}
              </Heading>
              <BodyShort>{term.definition}</BodyShort>
              {neighbours[selected].size > 0 && (
                <HStack gap="space-4" align="center">
                  <BodyShort size="small" className="opacity-70">
                    Henger sammen med:
                  </BodyShort>
                  {[...neighbours[selected]].map((i) => (
                    <Button key={i} size="xsmall" variant="tertiary" onClick={() => setSelected(i)}>
                      {terms[i].term}
                    </Button>
                  ))}
                </HStack>
              )}
              <div>
                <Button size="small" variant="secondary" onClick={() => goTo(selected)}>
                  Gå til begrepet
                </Button>
              </div>
            </VStack>
          )}
        </Box>
      </VStack>
    </section>
  );
}
