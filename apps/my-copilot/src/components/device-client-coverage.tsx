import { PlainTable } from "@/components/charts/chart-figure";
import { formatNumber } from "@/lib/format";
import type { NavPilotTelemetry } from "@/lib/types";
import { BodyShort } from "@navikt/ds-react";

export default function DeviceClientCoverage({ telemetry: t }: { telemetry: NavPilotTelemetry }) {
  return (
    <>
      <BodyShort>
        Klienter funnet på PATH siste syv dager. Dette viser tilgjengelighet, ikke bruk. Samme enhet kan ha flere
        klienter. Gruppene kan ikke summeres til unike enheter, personer eller lisenser. Ukjent betyr at raden mangler,
        blant annet når enhetstellinger under fem skjules.
      </BodyShort>
      <PlainTable
        rows={[
          ["Klient på PATH", "Enheter, 7 dager"],
          ...(["copilot", "opencode", "pi"] as const).map((client) => [
            client === "copilot" ? "Copilot" : client === "opencode" ? "OpenCode" : "Pi",
            t.client_available_7d?.[client] == null ? "Ukjent" : formatNumber(t.client_available_7d[client]),
          ]),
        ]}
      />
      <BodyShort>Registrert installasjon per scope og samling siste syv dager. Gruppene overlapper også her.</BodyShort>
      {t.install_present_7d?.length ? (
        <PlainTable
          rows={[
            ["Scope / samling", "Enheter, 7 dager"],
            ...t.install_present_7d.map((r) => [`${r.scope} / ${r.collection}`, formatNumber(r.devices)]),
          ]}
        />
      ) : (
        <BodyShort>Installasjonstilstedeværelse er ukjent. Manglende rader betyr ikke null installasjoner.</BodyShort>
      )}
    </>
  );
}
