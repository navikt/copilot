"use client";

import NextLink from "next/link";
import { useRouter } from "next/navigation";
import { useState, type ReactNode } from "react";
import {
  Alert,
  BodyShort,
  Box,
  Button,
  Checkbox,
  CheckboxGroup,
  ExpansionCard,
  FormProgress,
  Heading,
  HStack,
  Radio,
  RadioGroup,
  Select,
  Table,
  Tag,
  Textarea,
  TextField,
  VStack,
} from "@navikt/ds-react";
import type { TeamApp } from "@/lib/fleet-control";
import {
  DATA_CATEGORIES,
  DATA_LABEL,
  RISKS,
  RISK_LABEL,
  STEPS,
  TOOL_NAME,
  effectiveRisk,
  emptyState,
  toSpec,
  validateStep,
  type Risk,
  type WizardState,
  type WizardTool,
} from "@/lib/fleet-wizard";
import { FleetTable, ToolDiscovery } from "../fleet-ui";
import { DATA_HREF, EFFECT, LEVELS_HREF } from "@/lib/mcp-labels";
import { linkClass } from "@/components/nav-pilot/doc-page";

const RISK_HELP: Record<Risk, string> = {
  read: EFFECT.read.text,
  propose: EFFECT.propose.text,
  write: EFFECT.acts.text,
};

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
  const [newTool, setNewTool] = useState("");
  const [submit, setSubmit] = useState<{ busy?: boolean; error?: string; detail?: string }>({});
  const opts = { requireRos };
  // A changed field drops its own error; the others stay until the next «Neste».
  const set = (patch: Partial<WizardState>) => {
    setS((p) => ({ ...p, ...patch }));
    setErrors((e) => Object.fromEntries(Object.entries(e).filter(([k]) => !(k in patch))));
  };
  const setTool = (name: string, patch: Partial<WizardTool>) =>
    setS((p) => ({ ...p, tools: p.tools.map((t) => (t.name === name ? { ...t, ...patch } : t)) }));

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

  // Same name in several environments is one app: upstream names the app, the cluster is Fleet's.
  const appOptions = apps
    ? Object.entries(
        apps.reduce<Record<string, string[]>>((m, a) => ({ ...m, [a.name]: [...(m[a.name] ?? []), a.environment] }), {})
      )
    : null;

  function addTool() {
    const name = newTool.trim();
    if (!TOOL_NAME.test(name)) {
      setErrors({ newTool: "Verktøynavn kan bare ha bokstaver, tall, _, . og -, høyst 128 tegn." });
      return;
    }
    if (s.tools.some((t) => t.name === name)) {
      setErrors({ newTool: `${name} står allerede i lista.` });
      return;
    }
    set({ tools: [...s.tools, { name, allowed: true, risk: "" }] });
    setNewTool("");
    setErrors({});
  }

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
  const writes = allowed.some((t) => effectiveRisk(t, s.defaultRisk) === "write");
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
              {appOptions && appOptions.length > 0 ? (
                <Select
                  label="Nais-appen som kjører MCP-serveren"
                  description={`Appene til ${s.team}, hentet fra Nais.`}
                  value={s.app}
                  error={errors.app}
                  onChange={(e) =>
                    set({ app: e.target.value, url: e.target.value ? `http://${e.target.value}.${s.team}/mcp` : "" })
                  }
                >
                  <option value="">Velg app</option>
                  {appOptions.map(([name, envs]) => (
                    <option key={name} value={name}>
                      {name} ({envs.join(", ")})
                    </option>
                  ))}
                </Select>
              ) : (
                <>
                  <Alert variant="info" size="small">
                    {apps
                      ? `Fant ingen apper for ${s.team} i Nais. Skriv navnet på appen selv.`
                      : "Vi fikk ikke hentet appene fra Nais. Skriv navnet på appen selv."}
                  </Alert>
                  <TextField
                    label="Nais-appen som kjører MCP-serveren"
                    description="Navnet på appen i Nais, for eksempel kodeverk-mcp."
                    value={s.app}
                    error={errors.app}
                    onChange={(e) => {
                      const app = e.target.value.trim();
                      set({ app, url: app ? `http://${app}.${s.team}/mcp` : "" });
                    }}
                    autoComplete="off"
                  />
                </>
              )}
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
              <TextField
                label="Intern adresse"
                description="Adressen i clusteret der MCP-serveren svarer. Vi har fylt inn det vanlige mønsteret."
                value={s.url}
                error={errors.url}
                onChange={(e) => set({ url: e.target.value })}
                autoComplete="off"
              />
              {discoveryEnabled && (
                <ToolDiscovery
                  namespace={s.team}
                  app={s.app}
                  url={() => s.url.trim()}
                  // The team ticks; destructive tools start unticked. Annotations never decide access.
                  onTools={(tools) =>
                    set({
                      tools: tools.map((t) => ({
                        name: t.name,
                        description: t.description,
                        allowed: !t.destructive,
                        risk: "",
                        suggested: t.suggestedRisk,
                        destructive: t.destructive,
                      })),
                    })
                  }
                  onManual={next}
                />
              )}
            </>
          )}

          {step === 4 && (
            <>
              <Heading level="2" size="medium">
                Verktøy og risiko
              </Heading>
              {s.tools.length > 0 ? (
                <CheckboxGroup
                  legend="Verktøy konsumentene får bruke"
                  description="Kryss av for verktøyene som skal være med. Verktøy som kan endre eller slette data, er ikke krysset av."
                  value={allowed.map((t) => t.name)}
                  onChange={(names: string[]) =>
                    set({ tools: s.tools.map((t) => ({ ...t, allowed: names.includes(t.name) })) })
                  }
                  error={errors.tools}
                >
                  {s.tools.map((t) => (
                    <Checkbox key={t.name} value={t.name} description={t.description}>
                      <HStack gap="space-8" align="center">
                        <code>{t.name}</code>
                        {t.destructive && (
                          <Tag variant="warning" size="xsmall">
                            Kan endre data
                          </Tag>
                        )}
                      </HStack>
                    </Checkbox>
                  ))}
                </CheckboxGroup>
              ) : (
                <Alert variant={errors.tools ? "error" : "info"} size="small">
                  {errors.tools ?? "Ingen verktøy ennå. Hent dem i forrige steg, eller legg dem til her."}
                </Alert>
              )}
              <HStack gap="space-8" align="end">
                <TextField
                  label="Legg til verktøy"
                  description="Navnet slik MCP-serveren oppgir det."
                  value={newTool}
                  error={errors.newTool}
                  onChange={(e) => setNewTool(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      addTool();
                    }
                  }}
                  autoComplete="off"
                />
                <Button variant="secondary" type="button" onClick={addTool}>
                  Legg til
                </Button>
              </HStack>

              <RadioGroup
                legend="Hva gjør verktøyene?"
                description={
                  <>
                    Gjelder alle verktøyene, med mindre du velger noe annet for ett verktøy under «Avansert».{" "}
                    <NextLink href={LEVELS_HREF} className={linkClass}>
                      Om verktøynivåene
                    </NextLink>
                  </>
                }
                value={s.defaultRisk}
                onChange={(v: Risk) => set({ defaultRisk: v })}
                error={errors.defaultRisk}
              >
                {RISKS.map((r) => (
                  <Radio key={r} value={r} description={RISK_HELP[r]}>
                    {RISK_LABEL[r]}
                  </Radio>
                ))}
              </RadioGroup>

              {allowed.length > 0 && (
                <ExpansionCard aria-label="Avansert">
                  <ExpansionCard.Header>
                    <ExpansionCard.Title as="h3" size="small">
                      Avansert
                    </ExpansionCard.Title>
                    <ExpansionCard.Description>Velg et annet nivå for enkeltverktøy.</ExpansionCard.Description>
                  </ExpansionCard.Header>
                  <ExpansionCard.Content>
                    <VStack gap="space-16">
                      {allowed.map((t) => (
                        <Select
                          key={t.name}
                          label={<code>{t.name}</code>}
                          description={
                            t.suggested ? `MCP-serveren oppgir: ${RISK_LABEL[t.suggested].toLowerCase()}.` : undefined
                          }
                          value={t.risk}
                          onChange={(e) => setTool(t.name, { risk: e.target.value as Risk | "" })}
                          size="small"
                        >
                          <option value="">Som de andre ({RISK_LABEL[s.defaultRisk || "write"].toLowerCase()})</option>
                          {RISKS.map((r) => (
                            <option key={r} value={r}>
                              {RISK_LABEL[r]}
                            </option>
                          ))}
                        </Select>
                      ))}
                    </VStack>
                  </ExpansionCard.Content>
                </ExpansionCard>
              )}

              <TextField
                label={requireRos && writes ? "Lenke til ROS" : "Lenke til ROS (valgfritt)"}
                description={
                  requireRos
                    ? "Verktøy som skriver krever en ROS i prod."
                    : "Verktøy som skriver krever en ROS i prod. I dev kan du la feltet stå tomt."
                }
                value={s.rosRef}
                error={errors.rosRef}
                onChange={(e) => set({ rosRef: e.target.value })}
                autoComplete="off"
              />
            </>
          )}

          {step === 5 && (
            <>
              <Heading level="2" size="medium">
                Data og beskrivelse
              </Heading>
              <RadioGroup
                legend="Hva slags data gir verktøyene ut?"
                description={
                  <NextLink href={DATA_HREF} className={linkClass}>
                    Om datakategoriene
                  </NextLink>
                }
                value={s.dataCategory}
                onChange={(v: string) => set({ dataCategory: v })}
                error={errors.dataCategory}
              >
                {DATA_CATEGORIES.map((c) => (
                  <Radio key={c} value={c} description={DATA_LABEL[c][1]}>
                    {DATA_LABEL[c][0]}
                  </Radio>
                ))}
              </RadioGroup>
              <Textarea
                label="Beskrivelse (valgfritt)"
                description="Fortell hva MCP-serveren gjør og hvordan den brukes. Du kan bruke Markdown. Høyst 64 KiB."
                value={s.readme}
                error={errors.readme}
                onChange={(e) => set({ readme: e.target.value })}
              />
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
                        allowed
                          .map((t) => `${t.name} (${RISK_LABEL[effectiveRisk(t, s.defaultRisk)].toLowerCase()})`)
                          .join(", "),
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
