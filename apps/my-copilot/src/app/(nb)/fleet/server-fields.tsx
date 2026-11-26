"use client";

// The fields the registration wizard and the edit page share (#3118): one WizardState, one set of words.
import NextLink from "next/link";
import { useState } from "react";
import {
  Alert,
  BodyShort,
  Button,
  Checkbox,
  ErrorMessage,
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
  TOOL_NAME,
  effectiveRisk,
  mergeDiscovered,
  type Risk,
  type WizardState,
  type WizardTool,
} from "@/lib/fleet-wizard";
import { DATA_HREF, EFFECT, LEVELS_HREF } from "@/lib/mcp-labels";
import { linkClass } from "@/components/nav-pilot/doc-page";
import { ToolDiscovery, pathOf } from "./server-form";

const RISK_HELP: Record<Risk, string> = {
  read: EFFECT.read.text,
  propose: EFFECT.propose.text,
  write: EFFECT.acts.text,
};

export type FieldProps = {
  s: WizardState;
  set: (patch: Partial<WizardState>) => void;
  errors: Record<string, string>;
};

/** The Nais app: a list from Nais, or a text field when the list is unavailable (null) or empty. */
export function AppField({ s, set, errors, apps }: FieldProps & { apps: TeamApp[] | null }) {
  // Same name in several environments is one app: upstream names the app, the cluster is Fleet's.
  const options = apps
    ? Object.entries(
        apps.reduce<Record<string, string[]>>((m, a) => ({ ...m, [a.name]: [...(m[a.name] ?? []), a.environment] }), {})
      )
    : null;
  const pick = (app: string) => set({ app, url: app ? `http://${app}.${s.team}/mcp` : "" });
  if (options?.length && (!s.app || options.some(([n]) => n === s.app)))
    return (
      <Select
        label="Nais-appen som kjører MCP-serveren"
        description={`Appene til ${s.team}, hentet fra Nais.`}
        value={s.app}
        error={errors.app}
        onChange={(e) => pick(e.target.value)}
      >
        <option value="">Velg app</option>
        {options.map(([name, envs]) => (
          <option key={name} value={name}>
            {name} ({envs.join(", ")})
          </option>
        ))}
      </Select>
    );
  return (
    <VStack gap="space-16">
      {!s.app && (
        <Alert variant="info" size="small">
          {apps
            ? `Fant ingen apper for ${s.team} i Nais. Skriv navnet på appen selv.`
            : "Vi fikk ikke hentet appene fra Nais. Skriv navnet på appen selv."}
        </Alert>
      )}
      <TextField
        label="Nais-appen som kjører MCP-serveren"
        description="Navnet på appen i Nais, for eksempel kodeverk-mcp."
        value={s.app}
        error={errors.app}
        onChange={(e) => pick(e.target.value.trim())}
        autoComplete="off"
      />
    </VStack>
  );
}

/** The internal address and «Hent verktøy», which asks the app and address in the fields now. */
export function AddressField({
  s,
  set,
  errors,
  discoveryEnabled,
  onManual,
}: FieldProps & { discoveryEnabled: boolean; onManual: () => void }) {
  return (
    <VStack gap="space-16">
      <TextField
        label="Intern adresse"
        description="Adressen i clusteret der MCP-serveren svarer, for eksempel http://min-app.mitt-team/mcp."
        value={s.url}
        error={errors.url}
        onChange={(e) => set({ url: e.target.value })}
        autoComplete="off"
      />
      {discoveryEnabled && (
        <ToolDiscovery
          namespace={s.team}
          app={s.app}
          path={pathOf(s.url.trim())}
          onTools={(found) => set({ tools: mergeDiscovered(s.tools, found) })}
          onManual={onManual}
        />
      )}
    </VStack>
  );
}

/** One row per tool: allowed, name, description and level. Rows stack into cards below sm. */
export function ToolsFields({ s, set, errors, requireRos }: FieldProps & { requireRos: boolean }) {
  const [newTool, setNewTool] = useState("");
  const [addError, setAddError] = useState<string>();
  const setTool = (name: string, patch: Partial<WizardTool>) =>
    set({ tools: s.tools.map((t) => (t.name === name ? { ...t, ...patch } : t)) });
  const writes = s.tools.some((t) => t.allowed && effectiveRisk(t, s.defaultRisk) === "write");

  function addTool() {
    const name = newTool.trim();
    if (!TOOL_NAME.test(name))
      return setAddError("Verktøynavn kan bare ha bokstaver, tall, _, . og -, høyst 128 tegn.");
    if (s.tools.some((t) => t.name === name)) return setAddError(`${name} står allerede i lista.`);
    set({ tools: [...s.tools, { name, allowed: true, risk: "" }] });
    setNewTool("");
    setAddError(undefined);
  }

  return (
    <VStack gap="space-24">
      <VStack gap="space-8">
        {s.tools.length > 0 ? (
          <Table size="small" className="max-sm:!block">
            <caption className="text-left max-sm:block">
              <BodyShort weight="semibold" spacing>
                Verktøy konsumentene får bruke
              </BodyShort>
              <BodyShort size="small" spacing>
                Kryss av for verktøyene som skal være med. Nye verktøy som kan endre noe, er ikke krysset av.
              </BodyShort>
            </caption>
            <Table.Header className="max-sm:sr-only">
              <Table.Row>
                <Table.HeaderCell scope="col">Tillatt</Table.HeaderCell>
                <Table.HeaderCell scope="col">Verktøy</Table.HeaderCell>
                <Table.HeaderCell scope="col">Beskrivelse</Table.HeaderCell>
                <Table.HeaderCell scope="col">Hva gjør verktøyet?</Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body className="max-sm:!block">
              {s.tools.map((t) => (
                <Table.Row key={t.name} className="max-sm:!grid max-sm:grid-cols-[auto_1fr] max-sm:items-center">
                  <Table.DataCell className="max-sm:!block max-sm:border-0">
                    <Checkbox
                      hideLabel
                      size="small"
                      checked={t.allowed}
                      onChange={(e) => setTool(t.name, { allowed: e.target.checked })}
                    >
                      Tillat {t.name}
                    </Checkbox>
                  </Table.DataCell>
                  <Table.DataCell className="max-sm:!block max-sm:border-0">
                    <HStack gap="space-8" align="center">
                      <code>{t.name}</code>
                      {t.destructive && (
                        <Tag variant="warning" size="xsmall">
                          Kan slette data
                        </Tag>
                      )}
                    </HStack>
                  </Table.DataCell>
                  <Table.DataCell className="max-sm:!block max-sm:col-span-2 max-sm:border-0">
                    <BodyShort size="small">{t.description ?? "–"}</BodyShort>
                  </Table.DataCell>
                  <Table.DataCell className="max-sm:!block max-sm:col-span-2">
                    <Select
                      label={`Hva gjør ${t.name}?`}
                      hideLabel
                      size="small"
                      value={effectiveRisk(t, s.defaultRisk)}
                      onChange={(e) => setTool(t.name, { risk: e.target.value as Risk })}
                    >
                      {RISKS.map((r) => (
                        <option key={r} value={r}>
                          {RISK_LABEL[r]}
                        </option>
                      ))}
                    </Select>
                  </Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        ) : (
          <Alert variant="info" size="small">
            Ingen verktøy ennå. Hent dem fra MCP-serveren, eller legg dem til under.
          </Alert>
        )}
        {errors.tools && <ErrorMessage>{errors.tools}</ErrorMessage>}
      </VStack>

      <HStack gap="space-8" align="end">
        <TextField
          label="Legg til verktøy"
          description="Navnet slik MCP-serveren oppgir det."
          value={newTool}
          error={addError}
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
        legend="Standardnivå"
        description={
          <>
            Gjelder verktøy du legger til selv, til du velger noe annet i tabellen.{" "}
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

      <TextField
        label={requireRos && writes ? "Lenke til ROS" : "Lenke til ROS (valgfritt)"}
        description={
          requireRos
            ? "Verktøy som utfører noe, krever en ROS i prod."
            : "Verktøy som utfører noe, krever en ROS i prod. I dev kan du la feltet stå tomt."
        }
        value={s.rosRef}
        error={errors.rosRef}
        onChange={(e) => set({ rosRef: e.target.value })}
        autoComplete="off"
      />
    </VStack>
  );
}

/** Data category and the description. */
export function DataFields({ s, set, errors }: FieldProps) {
  return (
    <VStack gap="space-24">
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
    </VStack>
  );
}
