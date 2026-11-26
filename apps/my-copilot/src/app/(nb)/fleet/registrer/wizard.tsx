"use client";

import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import {
  Alert,
  BodyShort,
  Box,
  Button,
  FormProgress,
  Heading,
  HStack,
  Select,
  Table,
  TextField,
  VStack,
} from "@navikt/ds-react";
import type { TeamApp } from "@/lib/fleet-control";
import {
  DATA_LABEL,
  RISK_LABEL,
  STEPS,
  effectiveRisk,
  emptyState,
  toSpec,
  validateStep,
  type WizardState,
} from "@/lib/fleet-wizard";
import { FleetTable } from "../fleet-ui";
import { AddressField, AppField, DataFields, ToolsFields } from "../server-fields";

/** Which step a field from /api/fleet belongs to, so a refused submit opens that step. */
const FIELD_STEP: Record<string, number> = {
  name: 1,
  app: 2,
  url: 3,
  tools: 4,
  defaultRisk: 4,
  rosRef: 4,
  dataCategory: 5,
  readme: 5,
};

type Props = {
  teams: string[];
  team: string;
  /** null: the list from Nais is unavailable, so the app is typed in. */
  apps: TeamApp[] | null;
  requireRos: boolean;
  discoveryEnabled: boolean;
};

export function RegisterWizard({ teams, team, apps, requireRos, discoveryEnabled }: Props) {
  const router = useRouter();
  const [s, setS] = useState<WizardState>(() => emptyState(team));
  const [step, setStep] = useState(1);
  const [reached, setReached] = useState(1);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [submit, setSubmit] = useState<{ busy?: boolean; error?: string; detail?: string }>({});
  const opts = { requireRos };
  // A changed field drops its own error; the others stay until the next «Neste».
  const set = (patch: Partial<WizardState>) => {
    setS((p) => ({ ...p, ...patch }));
    setErrors((e) => Object.fromEntries(Object.entries(e).filter(([k]) => !(k in patch))));
  };

  const go = (n: number) => {
    setErrors({});
    setStep(n);
    setReached((r) => Math.max(r, n));
  };
  const next = () => {
    const e = validateStep(step, s, opts);
    setErrors(e);
    if (Object.keys(e).length === 0) go(step + 1);
  };

  async function send() {
    setSubmit({ busy: true });
    try {
      const res = await fetch("/api/fleet", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ action: "register", namespace: s.team, name: s.name, spec: toSpec(s) }),
      });
      const out = (await res.json().catch(() => ({}))) as {
        href?: string;
        error?: string;
        detail?: string;
        fields?: Record<string, string>;
      };
      if (res.ok && out.href) {
        router.push(out.href);
        router.refresh();
        return;
      }
      const fields = out.fields ?? {};
      const back = Math.min(...Object.keys(fields).map((f) => FIELD_STEP[f] ?? 6), 6);
      setSubmit({ error: out.error ?? "Noe gikk galt. Prøv igjen om litt.", detail: out.detail });
      if (back < 6) {
        setStep(back);
        setErrors(fields);
      }
    } catch {
      setSubmit({ error: "Vi får ikke kontakt med Fleet akkurat nå. Prøv igjen om litt." });
    }
  }

  const allowed = s.tools.filter((t) => t.allowed);
  const fields = { s, set, errors };
  const route = `/mcp/${s.team}/${s.name || "navn"}`;

  return (
    <VStack gap="space-24" className="max-w-3xl">
      <FormProgress
        totalSteps={STEPS.length}
        activeStep={step}
        interactiveSteps
        onStepChange={(n) => n <= reached && n !== step && go(n)}
      >
        {STEPS.map((label, i) => (
          <FormProgress.Step key={label} interactive={i + 1 <= reached}>
            {label}
          </FormProgress.Step>
        ))}
      </FormProgress>

      <Box background="default" borderColor="neutral-subtle" borderWidth="1" borderRadius="12" padding="space-24">
        <VStack gap="space-24">
          {step === 1 && (
            <>
              <Heading level="2" size="medium">
                Navn
              </Heading>
              <Select
                label="Team"
                description="Teamet som eier MCP-serveren."
                value={s.team}
                error={errors.team}
                onChange={(e) => {
                  set({ team: e.target.value, app: "", url: "" });
                  // The app list belongs to the team, so the page loads it again.
                  router.replace(`/fleet/registrer?team=${encodeURIComponent(e.target.value)}`, { scroll: false });
                }}
              >
                {teams.map((t) => (
                  <option key={t} value={t}>
                    {t}
                  </option>
                ))}
              </Select>
              <TextField
                label="Navn på MCP-serveren"
                description="Små bokstaver, tall og bindestrek. Blir en del av ruten."
                value={s.name}
                error={errors.name}
                onChange={(e) => set({ name: e.target.value.trim() })}
                autoComplete="off"
              />
              <BodyShort>
                Ruten blir <code>{route}</code>
              </BodyShort>
            </>
          )}

          {step === 2 && (
            <>
              <Heading level="2" size="medium">
                Mål-app
              </Heading>
              <AppField {...fields} apps={apps} />
              <Alert variant="info" size="small">
                Appen må slippe inn fleet-gateway i <code>accessPolicy.inbound</code>. Det sjekker vi i neste steg, når
                vi henter verktøyene.
              </Alert>
            </>
          )}

          {step === 3 && (
            <>
              <Heading level="2" size="medium">
                MCP-endpointet
              </Heading>
              <AddressField {...fields} discoveryEnabled={discoveryEnabled} onManual={next} />
            </>
          )}

          {step === 4 && (
            <>
              <Heading level="2" size="medium">
                Verktøy og risiko
              </Heading>
              <ToolsFields {...fields} requireRos={requireRos} />
            </>
          )}

          {step === 5 && (
            <>
              <Heading level="2" size="medium">
                Data og beskrivelse
              </Heading>
              <DataFields {...fields} />
            </>
          )}

          {step === 6 && (
            <>
              <Heading level="2" size="medium">
                Se over
              </Heading>
              <FleetTable aria-label="Det du sender inn">
                <Table.Body>
                  {(
                    [
                      ["Rute", <code key="r">{route}</code>],
                      ["Mål-app", s.app],
                      ["Intern adresse", <code key="u">{s.url}</code>],
                      [
                        "Verktøy",
                        allowed.map((t) => `${t.name} (${RISK_LABEL[effectiveRisk(t, s.defaultRisk)]})`).join(", "),
                      ],
                      ["ROS", s.rosRef || "Ingen"],
                      ["Datakategori", DATA_LABEL[s.dataCategory]?.[0] ?? ""],
                      ["Beskrivelse", s.readme.trim() ? "Ja" : "Ingen"],
                    ] as [string, ReactNode][]
                  ).map(([k, v]) => (
                    <Table.Row key={k}>
                      <Table.HeaderCell scope="row">{k}</Table.HeaderCell>
                      <Table.DataCell>{v}</Table.DataCell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </FleetTable>
              <Alert variant="info" size="small">
                Når du sender inn, må team copilot godkjenne før MCP-serveren blir publisert.
              </Alert>
              {submit.error && (
                <Alert variant="error" size="small">
                  <VStack gap="space-8">
                    <BodyShort>{submit.error}</BodyShort>
                    {submit.detail && (
                      <BodyShort size="small" lang="en">
                        {submit.detail}
                      </BodyShort>
                    )}
                  </VStack>
                </Alert>
              )}
            </>
          )}

          <HStack
            gap="space-16"
            justify="space-between"
            className="border-t border-gray-200"
            paddingBlock="space-24 space-0"
          >
            <HStack gap="space-16">
              {step > 1 && (
                <Button variant="secondary" type="button" onClick={() => go(step - 1)}>
                  Forrige
                </Button>
              )}
              {step < 6 ? (
                <Button type="button" onClick={next}>
                  Neste
                </Button>
              ) : (
                <Button type="button" onClick={send} loading={submit.busy}>
                  Send til godkjenning
                </Button>
              )}
            </HStack>
            <Button as={NextLink} href="/fleet" variant="tertiary">
              Avbryt
            </Button>
          </HStack>
        </VStack>
      </Box>
    </VStack>
  );
}
