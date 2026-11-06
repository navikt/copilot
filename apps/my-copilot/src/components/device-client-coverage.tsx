import { BodyShort, HGrid } from "@navikt/ds-react";
import RankedBars from "@/components/charts/RankedBars";
import { formatNumber } from "@/lib/format";
import type { NavPilotTelemetry } from "@/lib/types";

const CLIENTS = [
  ["copilot", "Copilot"],
  ["opencode", "OpenCode"],
  ["pi", "Pi"],
] as const;

const count = (n: number | null | undefined) => (n == null ? "Ukjent" : formatNumber(n));

/**
 * Three overlapping device counts, each in its own panel: clients on PATH (availability), clients with a
 * configuration (setup) and installations per scope and collection. They are not steps in a funnel and
 * cannot be added up to unique devices.
 */
export default function DeviceClientCoverage({ telemetry: t }: { telemetry: NavPilotTelemetry }) {
  const path = CLIENTS.map(([key, label]) => ({ label, value: t.client_available_7d?.[key] ?? null }));
  const hasConfigured = t.copilot_devices_7d > 0 || t.opencode_devices_7d > 0;
  const configured = [
    { label: "Copilot", value: t.copilot_devices_7d },
    { label: "OpenCode", value: t.opencode_devices_7d },
    { label: "Begge", value: t.both_clients_7d },
  ];
  const install = (t.install_present_7d ?? []).map((r) => ({
    label: `${r.scope} / ${r.collection}`,
    value: r.devices,
  }));
  const scope = [
    { label: "Personlig oppsett", value: t.user_scope_devices_7d },
    { label: "Repo-oppsett", value: t.repo_scope_devices_7d },
  ];
  const hasScope = t.user_scope_devices_7d > 0 || t.repo_scope_devices_7d > 0;
  return (
    <>
      <BodyShort>
        Enheter siste syv dager, flere ulike mål. Gruppene overlapper, er ikke trinn i en trakt og kan ikke summeres til
        unike enheter, personer eller lisenser.
      </BodyShort>
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        <RankedBars
          title="Klient på PATH"
          takeaway="Tilgjengelighet, ikke bruk. Ukjent betyr at raden mangler, blant annet når enhetstellinger under fem skjules."
          summary="Enheter med klienten på PATH siste syv dager."
          items={path}
          rows={[["Klient på PATH", "Enheter, 7 dager"], ...path.map((p) => [p.label, count(p.value)])]}
        />
        {hasConfigured && (
          <RankedBars
            title="Klient med oppsett"
            takeaway="Enheter som har rapportert konfigurasjon. «Begge» er også med i Copilot og OpenCode."
            summary="Enheter med konfigurasjon for Copilot, OpenCode og begge siste syv dager."
            items={configured}
            color={1}
            rows={[["Klient med oppsett", "Enheter, 7 dager"], ...configured.map((c) => [c.label, count(c.value)])]}
          />
        )}
        {hasScope && (
          <RankedBars
            title="Personlig mot repo-oppsett"
            takeaway={`Aktive tilpasninger i ~/.copilot mot enkeltrepoer. En enhet kan ha begge.${t.rtk_devices_7d > 0 ? ` ${formatNumber(t.rtk_devices_7d)} enheter har rtk.` : ""}`}
            summary="Enheter med personlig oppsett og repo-oppsett siste syv dager."
            items={scope}
            color={2}
            rows={[["Oppsett", "Enheter, 7 dager"], ...scope.map((r) => [r.label, count(r.value)])]}
          />
        )}
        {install.length > 0 ? (
          <RankedBars
            title="Installasjon per scope og samling"
            takeaway="Registrert installasjon. Gruppene overlapper også her."
            summary="Enheter med registrert installasjon per scope og samling siste syv dager."
            items={install}
            color={3}
            rows={[["Scope / samling", "Enheter, 7 dager"], ...install.map((r) => [r.label, count(r.value)])]}
          />
        ) : (
          <BodyShort>Installasjonstilstedeværelse er ukjent. Manglende rader betyr ikke null installasjoner.</BodyShort>
        )}
      </HGrid>
    </>
  );
}
