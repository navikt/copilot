"use client";

import { useState } from "react";
import { BodyShort, Label } from "@navikt/ds-react";
import { FilesIcon, PersonGroupIcon, WrenchIcon, DocPencilIcon, ChatIcon, FileTextIcon } from "@navikt/aksel-icons";

type FileEntry = {
  name: string;
  type: "agent" | "skill" | "instruction" | "prompt" | "config" | "dir";
  description: string;
  detail: string;
  when: string;
};

const FILE_TREE: FileEntry[] = [
  {
    name: "agents/nav-pilot.agent.md",
    type: "agent",
    description: "Hovedagenten — én inngangsport til alt",
    detail:
      "Definerer @nav-pilot sin persona, hvilke skills den bruker, og hvordan den delegerer til andre agenter. Copilot leser denne filen når du skriver @nav-pilot i chatten.",
    when: "Når du skriver @nav-pilot i Copilot Chat",
  },
  {
    name: "skills/nais/SKILL.md",
    type: "skill",
    description: "Nais-kunnskap for deploy og plattform",
    detail:
      "Kjenner Nais-manifestformat, GCP-ressurser og kubectl-kommandoer. Lastes av nav-pilot når du spør om deploy, eller med $nais.",
    when: "Når du spør nav-pilot om deploy, eller skriver $nais",
  },
  {
    name: "skills/nav-plan/SKILL.md",
    type: "skill",
    description: "Beslutningstrær for arkitekturvalg",
    detail:
      "Inneholder strukturerte spørsmål og beslutningstrær for å velge auth-strategi, database, kommunikasjonsmønster og Nais-konfig. Agenten følger treet steg for steg.",
    when: "Når du ber nav-pilot planlegge et nytt prosjekt",
  },
  {
    name: "skills/security-review/SKILL.md",
    type: "skill",
    description: "Sikkerhetssjekkliste for kodegjennomgang",
    detail:
      "Sjekklister for OWASP Top 10, input-validering, autentisering og hemmeligheter. Hver sjekk har alvorlighetsgrad og anbefalt handling.",
    when: "Når du ber om sikkerhetsgjennomgang av koden din",
  },
  {
    name: "instructions/kotlin-ktor.instructions.md",
    type: "instruction",
    description: "Kodestandarder for Kotlin/Ktor",
    detail:
      "Alltid aktiv i Kotlin-filer. Forteller Copilot om sealed classes, Kotliquery-mønstre, ApplicationBuilder og feilhåndtering — uten at du trenger å spørre.",
    when: "Automatisk i alle .kt-filer (applyTo-mønster)",
  },
  {
    name: "instructions/github-actions.instructions.md",
    type: "instruction",
    description: "CI/CD-standarder for GitHub Actions",
    detail:
      "SHA-pinning av actions, Nais deploy-workflow, caching og sikkerhet. Aktiveres automatisk i workflow-filer.",
    when: "Automatisk i .github/workflows/*.yml",
  },
  {
    name: "prompts/nais-manifest.prompt.md",
    type: "prompt",
    description: "Generer Nais-manifest fra mal",
    detail:
      "En forhåndsdefinert prompt du kan kjøre fra Copilot. Stiller spørsmål om appen din og genererer et komplett .nais/nais.yaml-manifest.",
    when: "Når du velger prompten i Copilots prompt-meny",
  },
  {
    name: ".nav-pilot-state.json",
    type: "config",
    description: "Installert tilstand (for sync og uninstall)",
    detail:
      "Holder styr på hvilken collection som er installert, versjon, og hash av hver fil. Brukes av nav-pilot sync for å oppdage endringer og av nav-pilot uninstall for å rydde opp.",
    when: "Lest av nav-pilot CLI ved sync, status og uninstall",
  },
];

const TYPE_META: Record<FileEntry["type"], { label: string; color: string; bg: string; Icon: typeof FilesIcon }> = {
  agent: { label: "Agent", color: "#7c3aed", bg: "#f5f3ff", Icon: PersonGroupIcon },
  skill: { label: "Skill", color: "#059669", bg: "#ecfdf5", Icon: WrenchIcon },
  instruction: { label: "Instruksjon", color: "#3b82f6", bg: "#eff6ff", Icon: DocPencilIcon },
  prompt: { label: "Prompt", color: "#d97706", bg: "#fffbeb", Icon: ChatIcon },
  config: { label: "Konfig", color: "#64748b", bg: "#f8fafc", Icon: FileTextIcon },
  dir: { label: "Mappe", color: "#64748b", bg: "#f8fafc", Icon: FilesIcon },
};

export function FileExplorer() {
  const [selected, setSelected] = useState(0);
  const entry = FILE_TREE[selected];
  const meta = TYPE_META[entry.type];

  return (
    <div className="rounded-xl border overflow-hidden" style={{ borderColor: "#e2e8f0" }}>
      {/* Header */}
      <div
        className="flex items-center gap-2 px-4 py-2"
        style={{ background: "#f8fafc", borderBottom: "1px solid #e2e8f0" }}
      >
        <FilesIcon aria-hidden fontSize="1rem" style={{ color: "#94a3b8" }} />
        <BodyShort
          size="small"
          weight="semibold"
          style={{ color: "#64748b", fontFamily: "monospace", fontSize: "0.8rem" }}
        >
          .github/
        </BodyShort>
      </div>

      <div className="flex" style={{ minHeight: "280px" }}>
        {/* File list (left) */}
        <div className="w-1/2 overflow-y-auto" style={{ borderRight: "1px solid #e2e8f0", background: "#fafbfc" }}>
          {FILE_TREE.map((f, i) => {
            const m = TYPE_META[f.type];
            const isSelected = i === selected;
            return (
              <button
                key={f.name}
                onClick={() => setSelected(i)}
                className="w-full text-left flex items-center gap-2 px-3 py-2 transition-colors"
                style={{
                  background: isSelected ? m.bg : "transparent",
                  borderLeft: isSelected ? `3px solid ${m.color}` : "3px solid transparent",
                  cursor: "pointer",
                  border: "none",
                  borderBottom: "1px solid #f1f5f9",
                }}
                aria-current={isSelected ? "true" : undefined}
              >
                <m.Icon aria-hidden fontSize="0.875rem" style={{ color: m.color, flexShrink: 0 }} />
                <span
                  style={{
                    fontFamily: "monospace",
                    fontSize: "0.75rem",
                    color: isSelected ? m.color : "#475569",
                    fontWeight: isSelected ? 600 : 400,
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                  }}
                >
                  {f.name}
                </span>
              </button>
            );
          })}
        </div>

        {/* Detail panel (right) */}
        <div className="w-1/2 p-4 flex flex-col gap-3">
          <div className="flex items-center gap-2">
            <span
              className="text-xs font-semibold rounded-full px-2 py-0.5"
              style={{ background: meta.bg, color: meta.color }}
            >
              {meta.label}
            </span>
          </div>
          <div>
            <Label size="small" style={{ color: "#1e293b" }}>
              {entry.description}
            </Label>
            <BodyShort size="small" className="mt-2" style={{ color: "#475569" }}>
              {entry.detail}
            </BodyShort>
          </div>
          <div className="mt-auto rounded-lg px-3 py-2" style={{ background: "#f8fafc", border: "1px solid #e2e8f0" }}>
            <BodyShort size="small" style={{ color: "#64748b", fontSize: "0.75rem" }}>
              <strong>Aktiveres:</strong> {entry.when}
            </BodyShort>
          </div>
        </div>
      </div>
    </div>
  );
}
