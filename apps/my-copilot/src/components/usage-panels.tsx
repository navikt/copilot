import { BodyShort, HGrid, VStack } from "@navikt/ds-react";
import RankedBars from "@/components/charts/RankedBars";
import { NumbersFold } from "@/components/charts/chart-figure";
import EditorUsageTable from "@/components/editor-usage-table";
import { formatDate, formatNumber } from "@/lib/format";
import { topTakeaway } from "@/lib/ranked";
import type { ClientUsage, EditorUsage } from "@/lib/types";

const CLIENT_LABELS: Record<string, string> = {
  vscode: "VS Code",
  copilot_cli: "Copilot CLI",
  opencode: "OpenCode",
  jetbrains: "JetBrains",
  other: "andre",
};

/** The label people read; the raw name stays in the table. */
export const clientLabel = (raw: string) => CLIENT_LABELS[raw] ?? raw;

/** GitHub has reported a version number («1.138.0») as an editor name. Keep the row, flag the label. */
export const looksLikeVersion = (name: string) => /^\d+(\.\d+){1,3}$/.test(name);
export const editorLabel = (name: string) =>
  looksLikeVersion(name) ? `${name} (uvanlig navn)` : name === "vscode" ? CLIENT_LABELS.vscode : name;

/** Requests and sessions are different units: two panels in the same row order, each on its own scale, no ratio. */
export function ClientPanels({ clients }: { clients: ClientUsage[] }) {
  const byRequests = [...clients].sort((a, b) => b.requests - a.requests);
  const requests = byRequests.map((c) => ({ label: clientLabel(c.client), value: c.requests }));
  const sessions = byRequests.map((c) => ({ label: clientLabel(c.client), value: c.sessions }));
  const rows = (head: string, key: "requests" | "sessions") => [
    ["Verktøy", "GitHubs navn", head],
    ...byRequests.map((c) => [clientLabel(c.client), c.client, formatNumber(c[key])]),
  ];
  return (
    <VStack gap="space-8">
      <BodyShort size="small" textColor="subtle">
        Forespørsler og økter er ulike mål. Panelene har samme rekkefølge og hver sin skala.
      </BodyShort>
      <HGrid columns={{ xs: 1, md: 2 }} gap="space-24">
        <RankedBars
          title="Forespørsler"
          takeaway={topTakeaway(requests, "forespørsler")}
          summary="Forespørsler per verktøy de siste 28 dagene, flest først."
          items={requests}
          rows={rows("Forespørsler", "requests")}
        />
        <RankedBars
          title="Økter"
          takeaway={topTakeaway(sessions, "økter")}
          summary="Økter per verktøy de siste 28 dagene, i samme rekkefølge som forespørslene."
          items={sessions}
          color={1}
          rows={rows("Økter", "sessions")}
        />
      </HGrid>
    </VStack>
  );
}

const EDITOR_METRICS = [
  { key: "generations", title: "Genereringer", color: 0 },
  { key: "acceptances", title: "Aksepteringer", color: 1 },
  { key: "interactions", title: "Interaksjoner", color: 3 },
] as const;

/**
 * Generations, acceptances and interactions are different events, so each gets its own panel with its own scale,
 * all in the same editor order. Unknown (null) is not zero. Coverage per editor stays visible under the panels.
 */
export function EditorPanels({ editors }: { editors: EditorUsage[] }) {
  const order = [...editors].sort(
    (a, b) => (b.interactions ?? -1) - (a.interactions ?? -1) || a.editor.localeCompare(b.editor)
  );
  return (
    <VStack gap="space-16">
      <HGrid columns={{ xs: 1, md: 3 }} gap="space-24">
        {EDITOR_METRICS.map(({ key, title, color }) => {
          const items = order.map((e) => ({ label: editorLabel(e.editor), value: e[key] }));
          const missing = items.filter((i) => i.value == null).length;
          return (
            <RankedBars
              key={key}
              title={title}
              takeaway={
                topTakeaway(items, title.toLowerCase()) +
                (missing ? ` ${formatNumber(missing)} av ${formatNumber(items.length)} mangler tall.` : "")
              }
              summary={`${title} per editor siste 28 dager, i samme editorrekkefølge som de andre panelene.`}
              items={items}
              color={color}
              rows={[
                ["Editor", "GitHubs navn", title],
                ...order.map((e) => [
                  editorLabel(e.editor),
                  e.editor,
                  e[key] == null ? "Ukjent" : formatNumber(e[key]),
                ]),
              ]}
            />
          );
        })}
      </HGrid>
      <div>
        <BodyShort size="small" weight="semibold">
          Dekning per editor
        </BodyShort>
        <ul className="m-0 list-none p-0 text-sm text-(--ax-text-neutral-subtle)">
          {order.map((e) => (
            <li key={e.editor}>
              <span className="font-medium text-(--ax-text-neutral)">{editorLabel(e.editor)}</span>:{" "}
              {formatDate(e.first_day)} til {formatDate(e.last_day)}, {formatNumber(e.days)} dager med data
            </li>
          ))}
        </ul>
      </div>
      <NumbersFold summary="Vis alle rader, med navn slik GitHub oppgir dem">
        <EditorUsageTable editors={editors} />
      </NumbersFold>
    </VStack>
  );
}
