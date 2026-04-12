"use client";

import { useState } from "react";
import { Heading, Box } from "@navikt/ds-react";
import { MagnifyingGlassIcon, TasklistIcon, ShieldLockIcon, RocketIcon, ArrowRightIcon } from "@navikt/aksel-icons";

const STEPS = [
  {
    title: "Intervju",
    subtitle: "Dypdykk-intervju",
    description: "Finner blinde flekker — dataklassifisering, auth-type, PII-risiko og avhengigheter.",
    Icon: MagnifyingGlassIcon,
    color: "#a78bfa",
    items: [
      "Personvern — behandler dere PII? Hvilke kategorier?",
      "Auth — hvem kaller tjenesten — bruker, tjeneste, ekstern partner?",
      "Avhengigheter — hva skjer når en avhengighet er nede?",
      "Endringspåvirkning — hvem konsumerer dine API-er/hendelser?",
      "Teststatus — hva er testdekningen i koden som endres?",
      "Observerbarhet — hvilke forretningsmetrikker viser at tjenesten fungerer?",
    ],
  },
  {
    title: "Plan",
    subtitle: "Beslutningstrær",
    description: "Velger arkitektur, teststrategi og leveranser ut fra svarene dine.",
    Icon: TasklistIcon,
    color: "#60a5fa",
    items: [
      "Auth-beslutning — ID-porten, Azure AD, TokenX eller Maskinporten",
      "Nais-manifest — ferdig YAML med riktige ressurser og accessPolicy",
      "Prosjektstruktur — mappestruktur for valgt arketype",
      "CI/CD — GitHub Actions workflow med build, test, deploy",
      "Teststrategi — riktig testnivå per komponent",
      "Database — Flyway-migrasjoner, HikariCP-konfig",
    ],
  },
  {
    title: "Review",
    subtitle: "Arkitektur-review",
    description: "Sjekker Nav-antimønstre, endringspåvirkning, testdekning og teknisk gjeld.",
    Icon: ShieldLockIcon,
    color: "#2dd4bf",
    items: [
      "Sikkerhet — er auth riktig? Er PII beskyttet?",
      "Plattform — passer ressursene? Fungerer observerbarhet?",
      "Arkitektur — er dette den enkleste løsningen?",
      "Endringssikkerhet — er teststrategi definert? Er rollback-plan realistisk?",
    ],
  },
  {
    title: "Lever",
    subtitle: "Kode + dokumentasjon",
    description: "Produksjonsklar kode, tester, endringsdokument, utrullingsplan og verifiseringssjekkliste.",
    Icon: RocketIcon,
    color: "#fb923c",
    items: [
      "Kode, config og tester",
      "Endringsdokument med rollback-plan",
      "Observerbarhetsplan med suksesskriterier",
      "Post-deploy-verifiseringssjekkliste",
    ],
  },
];

export function PipelineFlow() {
  const [activeIndex, setActiveIndex] = useState(0);
  const active = STEPS[activeIndex];

  return (
    <div>
      {/* Phase cards */}
      <div
        className="w-full items-stretch gap-0"
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(4, 1fr)",
          alignItems: "stretch",
        }}
      >
        {STEPS.map((step, i) => {
          const isActive = i === activeIndex;
          return (
            <div key={step.title} className="flex items-stretch">
              <button
                onClick={() => setActiveIndex(i)}
                aria-pressed={isActive}
                className="rounded-lg overflow-hidden flex flex-col flex-1 text-left transition-all"
                style={{
                  background: isActive ? `${step.color}08` : "white",
                  border: isActive ? `2px solid ${step.color}` : "1px solid #e2e8f0",
                  boxShadow: isActive ? `0 2px 8px ${step.color}20` : "0 1px 3px rgba(0,0,0,0.04)",
                  cursor: "pointer",
                }}
              >
                <div
                  style={{
                    height: "3px",
                    background: step.color,
                    opacity: isActive ? 1 : 0.4,
                    transition: "opacity 0.2s",
                  }}
                />
                <Box padding={{ xs: "space-8", md: "space-12" }} className="flex-1 flex flex-col">
                  <div className="flex flex-col items-center text-center flex-1">
                    <div
                      className="flex items-center justify-center rounded-full mb-2"
                      style={{
                        width: "2rem",
                        height: "2rem",
                        background: `${step.color}${isActive ? "20" : "10"}`,
                        border: `1.5px solid ${step.color}${isActive ? "60" : "30"}`,
                        transition: "all 0.2s",
                      }}
                    >
                      <step.Icon fontSize="1rem" style={{ color: step.color }} aria-hidden />
                    </div>
                    <Heading size="xsmall" level="4">
                      {step.title}
                    </Heading>
                    <p
                      style={{
                        color: "#64748b",
                        fontSize: "0.6875rem",
                        margin: "0.125rem 0 0",
                        textAlign: "center",
                      }}
                    >
                      {step.subtitle}
                    </p>
                  </div>
                </Box>
              </button>
              {i < STEPS.length - 1 && (
                <div className="hidden md:flex items-center px-1.5">
                  <ArrowRightIcon fontSize="1rem" style={{ color: "#cbd5e1" }} aria-hidden />
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Active phase detail */}
      <div
        className="mt-4 rounded-lg"
        style={{
          background: `${active.color}08`,
          border: `1px solid ${active.color}25`,
          padding: "1rem 1.25rem",
        }}
      >
        <div className="flex items-center gap-2 mb-2">
          <active.Icon fontSize="1rem" style={{ color: active.color }} aria-hidden />
          <Heading size="xsmall" level="4" style={{ color: "#334155" }}>
            {active.title}
          </Heading>
          <span style={{ color: "#94a3b8", fontSize: "0.75rem" }}>— {active.subtitle}</span>
        </div>
        <ul className="text-sm space-y-1.5" style={{ color: "#475569", paddingLeft: "1.25rem" }}>
          {active.items.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </div>
    </div>
  );
}
