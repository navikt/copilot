"use client";

import { useState } from "react";
import { Box, HStack, VStack, Heading, BodyShort, Label, ToggleGroup, Detail } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { CopilotMark, OpenCodeMark } from "./client-marks";
import { buildCommands, COLLECTIONS, type ClientId, type SurfaceId, type CollectionId } from "./command-builder";

type Sel = { client: ClientId; surface: SurfaceId; collection: CollectionId };

function useSel(init?: Partial<Sel>) {
  return useState<Sel>({
    client: init?.client ?? "copilot",
    surface: init?.surface ?? "terminal",
    collection: init?.collection ?? "kotlin-backend",
  });
}

/* ------------------------------------------------------------------ */
/* Variant A — Clean Aksel segmented controls                          */
/* ------------------------------------------------------------------ */
export function VariantA() {
  const [sel, setSel] = useSel();
  const cmd = buildCommands(sel);
  return (
    <Box background="default" borderRadius="12" borderWidth="1" borderColor="neutral-subtle" padding="space-24">
      <VStack gap="space-20">
        <VStack gap="space-8">
          <Label size="small">Klient</Label>
          <ToggleGroup
            size="small"
            value={sel.client}
            onChange={(v) => setSel((s) => ({ ...s, client: v as ClientId }))}
          >
            <ToggleGroup.Item value="copilot">Copilot</ToggleGroup.Item>
            <ToggleGroup.Item value="opencode">OpenCode</ToggleGroup.Item>
            <ToggleGroup.Item value="interactive">Interaktiv</ToggleGroup.Item>
          </ToggleGroup>
        </VStack>
        <VStack gap="space-8">
          <Label size="small">Flate</Label>
          <ToggleGroup
            size="small"
            value={sel.surface}
            onChange={(v) => setSel((s) => ({ ...s, surface: v as SurfaceId }))}
          >
            <ToggleGroup.Item value="terminal">Terminal</ToggleGroup.Item>
            <ToggleGroup.Item value="editor">Editor</ToggleGroup.Item>
          </ToggleGroup>
        </VStack>
        <VStack gap="space-8">
          <Label size="small">Stack</Label>
          <ToggleGroup
            size="small"
            value={sel.collection}
            onChange={(v) => setSel((s) => ({ ...s, collection: v as CollectionId }))}
          >
            {COLLECTIONS.map((c) => (
              <ToggleGroup.Item key={c.id} value={c.id}>
                {c.label}
              </ToggleGroup.Item>
            ))}
          </ToggleGroup>
        </VStack>
        <div aria-live="polite">
          <CodeBlock compact>{cmd.launch}</CodeBlock>
          <BodyShort size="small" textColor="subtle" style={{ marginTop: "var(--ax-space-8)" }}>
            {cmd.tip}
          </BodyShort>
        </div>
      </VStack>
    </Box>
  );
}

/* ------------------------------------------------------------------ */
/* Variant B — Terminal command builder (dark, premium dev feel)       */
/* ------------------------------------------------------------------ */
function Chip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      style={{
        font: "inherit",
        cursor: "pointer",
        padding: "var(--ax-space-4) var(--ax-space-12)",
        borderRadius: "999px",
        border: `1px solid ${active ? "#5eead4" : "rgba(255,255,255,0.16)"}`,
        background: active ? "rgba(94,234,212,0.14)" : "rgba(255,255,255,0.03)",
        color: active ? "#5eead4" : "rgba(255,255,255,0.7)",
        transition: "all 120ms ease",
      }}
    >
      {children}
    </button>
  );
}

export function VariantB() {
  const [sel, setSel] = useSel({ client: "opencode" });
  const cmd = buildCommands(sel);
  return (
    <div
      style={{
        borderRadius: "var(--ax-radius-12)",
        overflow: "hidden",
        border: "1px solid rgba(255,255,255,0.08)",
        background: "linear-gradient(170deg, #0c1222 0%, #131d31 100%)",
        color: "#e6edf3",
        fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace",
      }}
    >
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: "var(--ax-space-8)",
          padding: "var(--ax-space-12) var(--ax-space-16)",
          borderBottom: "1px solid rgba(255,255,255,0.08)",
        }}
      >
        <span style={{ width: 10, height: 10, borderRadius: 99, background: "#ff5f56" }} />
        <span style={{ width: 10, height: 10, borderRadius: 99, background: "#ffbd2e" }} />
        <span style={{ width: 10, height: 10, borderRadius: 99, background: "#27c93f" }} />
        <span style={{ marginLeft: "auto", fontSize: 12, opacity: 0.5 }}>nav-pilot</span>
      </div>
      <div style={{ padding: "var(--ax-space-20)", display: "grid", gap: "var(--ax-space-16)" }}>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--ax-space-8)" }}>
          {(["copilot", "opencode", "interactive"] as ClientId[]).map((c) => (
            <Chip key={c} active={sel.client === c} onClick={() => setSel((s) => ({ ...s, client: c }))}>
              {c === "copilot" ? "Copilot" : c === "opencode" ? "OpenCode" : "Interaktiv"}
            </Chip>
          ))}
        </div>
        <div style={{ display: "flex", flexWrap: "wrap", gap: "var(--ax-space-8)" }}>
          {COLLECTIONS.map((c) => (
            <Chip
              key={c.id}
              active={sel.collection === c.id}
              onClick={() => setSel((s) => ({ ...s, collection: c.id }))}
            >
              {c.label}
            </Chip>
          ))}
        </div>
        <div
          aria-live="polite"
          style={{
            background: "rgba(0,0,0,0.35)",
            borderRadius: "var(--ax-radius-8)",
            padding: "var(--ax-space-16)",
            fontSize: 14,
            lineHeight: 1.6,
          }}
        >
          <div>
            <span style={{ color: "#5eead4" }}>❯</span> {cmd.launch}
            <span
              style={{
                display: "inline-block",
                width: 8,
                height: 16,
                marginLeft: 4,
                verticalAlign: "text-bottom",
                background: "#5eead4",
                animation: "none",
                opacity: 0.8,
              }}
            />
          </div>
          <div style={{ marginTop: "var(--ax-space-8)", opacity: 0.55, fontSize: 12.5 }}># {cmd.tip}</div>
        </div>
      </div>
    </div>
  );
}

/* ------------------------------------------------------------------ */
/* Variant C — Dual-client split cards (both first-class)              */
/* ------------------------------------------------------------------ */
function ClientCard({
  active,
  onClick,
  mark,
  title,
  subtitle,
}: {
  active: boolean;
  onClick: () => void;
  mark: React.ReactNode;
  title: string;
  subtitle: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      style={{
        flex: 1,
        textAlign: "left",
        cursor: "pointer",
        padding: "var(--ax-space-20)",
        borderRadius: "var(--ax-radius-12)",
        border: `2px solid ${active ? "var(--ax-border-accent)" : "var(--ax-border-neutral-subtle)"}`,
        background: active ? "var(--ax-bg-accent-soft)" : "var(--ax-bg-default)",
        transition: "all 140ms ease",
        boxShadow: active ? "0 6px 20px -8px rgba(0,82,173,0.45)" : "none",
      }}
    >
      <HStack gap="space-12" align="center">
        <span style={{ color: active ? "var(--ax-text-accent)" : "var(--ax-text-subtle)" }}>{mark}</span>
        <VStack gap="space-0">
          <Label size="small">{title}</Label>
          <Detail textColor="subtle">{subtitle}</Detail>
        </VStack>
      </HStack>
    </button>
  );
}

export function VariantC() {
  const [sel, setSel] = useSel();
  const cmd = buildCommands(sel);
  return (
    <Box background="default" borderRadius="12" borderWidth="1" borderColor="neutral-subtle" padding="space-24">
      <VStack gap="space-20">
        <HStack gap="space-16">
          <ClientCard
            active={sel.client === "copilot"}
            onClick={() => setSel((s) => ({ ...s, client: "copilot" }))}
            mark={<CopilotMark size={32} />}
            title="GitHub Copilot"
            subtitle="Terminal & VS Code Chat"
          />
          <ClientCard
            active={sel.client === "opencode"}
            onClick={() => setSel((s) => ({ ...s, client: "opencode" }))}
            mark={<OpenCodeMark size={32} />}
            title="OpenCode"
            subtitle="Åpen TUI med Nav-agenter"
          />
        </HStack>
        <VStack gap="space-8">
          <Label size="small">Stack</Label>
          <ToggleGroup
            size="small"
            value={sel.collection}
            onChange={(v) => setSel((s) => ({ ...s, collection: v as CollectionId }))}
          >
            {COLLECTIONS.map((c) => (
              <ToggleGroup.Item key={c.id} value={c.id}>
                {c.label}
              </ToggleGroup.Item>
            ))}
          </ToggleGroup>
        </VStack>
        <div aria-live="polite">
          <CodeBlock compact>{cmd.launch}</CodeBlock>
          <BodyShort size="small" textColor="subtle" style={{ marginTop: "var(--ax-space-8)" }}>
            {cmd.tip}
          </BodyShort>
        </div>
      </VStack>
    </Box>
  );
}

/* ------------------------------------------------------------------ */
/* Variant D — Choose-your-adventure stepper                           */
/* ------------------------------------------------------------------ */
function StepRow({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <HStack gap="space-16" align="start" wrap={false}>
      <span
        style={{
          flexShrink: 0,
          width: 28,
          height: 28,
          borderRadius: 99,
          display: "grid",
          placeItems: "center",
          background: "var(--ax-bg-accent-strong)",
          color: "var(--ax-text-on-accent, #fff)",
          fontSize: 13,
          fontWeight: 600,
        }}
      >
        {n}
      </span>
      <VStack gap="space-8" style={{ flex: 1 }}>
        <Label size="small">{title}</Label>
        {children}
      </VStack>
    </HStack>
  );
}

export function VariantD() {
  const [sel, setSel] = useSel();
  const cmd = buildCommands(sel);
  return (
    <Box background="default" borderRadius="12" borderWidth="1" borderColor="neutral-subtle" padding="space-24">
      <VStack gap="space-20">
        <StepRow n={1} title="Velg klient">
          <ToggleGroup
            size="small"
            value={sel.client}
            onChange={(v) => setSel((s) => ({ ...s, client: v as ClientId }))}
          >
            <ToggleGroup.Item value="copilot">Copilot</ToggleGroup.Item>
            <ToggleGroup.Item value="opencode">OpenCode</ToggleGroup.Item>
            <ToggleGroup.Item value="interactive">Interaktiv</ToggleGroup.Item>
          </ToggleGroup>
        </StepRow>
        <StepRow n={2} title="Hvor jobber du?">
          <ToggleGroup
            size="small"
            value={sel.surface}
            onChange={(v) => setSel((s) => ({ ...s, surface: v as SurfaceId }))}
          >
            <ToggleGroup.Item value="terminal">Terminal</ToggleGroup.Item>
            <ToggleGroup.Item value="editor">Editor</ToggleGroup.Item>
          </ToggleGroup>
        </StepRow>
        <StepRow n={3} title="Hva bygger du?">
          <ToggleGroup
            size="small"
            value={sel.collection}
            onChange={(v) => setSel((s) => ({ ...s, collection: v as CollectionId }))}
          >
            {COLLECTIONS.map((c) => (
              <ToggleGroup.Item key={c.id} value={c.id}>
                {c.label}
              </ToggleGroup.Item>
            ))}
          </ToggleGroup>
        </StepRow>
        <Box background="accent-soft" borderRadius="12" padding="space-16" borderWidth="1" borderColor="accent-subtle">
          <div aria-live="polite">
            <Detail textColor="subtle" style={{ marginBottom: "var(--ax-space-8)" }}>
              Kjør dette
            </Detail>
            <CodeBlock compact>{cmd.launch}</CodeBlock>
            <BodyShort size="small" textColor="subtle" style={{ marginTop: "var(--ax-space-8)" }}>
              {cmd.tip}
            </BodyShort>
          </div>
        </Box>
      </VStack>
    </Box>
  );
}

export function VariantShowcase() {
  const variants: { id: string; title: string; blurb: string; node: React.ReactNode }[] = [
    {
      id: "a",
      title: "Variant A — Rolig Aksel",
      blurb: "Segmenterte kontroller, helt on-brand. Trygt og rolig.",
      node: <VariantA />,
    },
    {
      id: "b",
      title: "Variant B — Terminal-bygger",
      blurb: "Mørk terminal-estetikk med live-kommando. Skiller seg ut, dev-følelse.",
      node: <VariantB />,
    },
    {
      id: "c",
      title: "Variant C — Dual-klient",
      blurb: "To likestilte klientkort. Understreker at begge er førsteklasses.",
      node: <VariantC />,
    },
    {
      id: "d",
      title: "Variant D — Velg-din-vei",
      blurb: "Nummererte steg som guider deg fram til kommandoen.",
      node: <VariantD />,
    },
  ];
  return (
    <VStack gap="space-40">
      {variants.map((v) => (
        <VStack key={v.id} gap="space-12">
          <VStack gap="space-2">
            <Heading size="small" level="2">
              {v.title}
            </Heading>
            <BodyShort textColor="subtle">{v.blurb}</BodyShort>
          </VStack>
          {v.node}
        </VStack>
      ))}
    </VStack>
  );
}
