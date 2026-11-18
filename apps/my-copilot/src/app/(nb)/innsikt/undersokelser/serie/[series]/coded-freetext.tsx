import { BodyShort, Heading, VStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { formatNumber, formatShare } from "@/lib/format";
import type { WaveFreetext, WaveMeta } from "@/lib/survey";

const kappa = (k: number | null) => (k === null ? "–" : k.toFixed(2).replace(".", ","));

/** Coded free text per wave: count and share of those who wrote in the field, «<5» for small groups. No quotes. */
export function CodedFreetext({ waves, freetext }: { waves: WaveMeta[]; freetext?: WaveFreetext[] }) {
  if (!freetext?.length) return null;
  const versions = new Set(freetext.map((w) => w.codebook_version));
  return (
    <section>
      <VStack gap="space-16">
        <Heading level="2" size="medium">
          Fritekst, kodet
        </Heading>
        <BodyShort size="small">
          Fritekstsvarene er kodet i kategorier av {freetext[0].coders}, og vises som antall og andel av dem som skrev i
          feltet. Kappa er samsvaret mellom de to koderne. En kategori vises bare når kappa er minst 0,6, ellers slått
          sammen med en nær kategori eller ikke i det hele tatt. Grupper under fem vises som «&lt;5». Ingen sitater.
          {versions.size > 1
            ? " Rundene er kodet med ulike versjoner av kodeboka og kan ikke sammenlignes direkte."
            : ""}
        </BodyShort>
        {freetext.map((w) => (
          <VStack key={w.survey_id} gap="space-12">
            <Heading level="3" size="small">
              {waves.find((s) => s.id === w.survey_id)?.title ?? w.survey_id}
            </Heading>
            {w.fields.map((f) => (
              <Table key={f.id} size="small">
                <caption>
                  <BodyShort size="small" weight="semibold" className="text-left">
                    {f.question}
                    {f.other ? ` (svar under «${f.other}»)` : ""}, {formatNumber(f.written)} skrev
                  </BodyShort>
                </caption>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Kategori</TableHeaderCell>
                    <TableHeaderCell scope="col" align="right">
                      Antall
                    </TableHeaderCell>
                    <TableHeaderCell scope="col" align="right">
                      Andel
                    </TableHeaderCell>
                    <TableHeaderCell scope="col" align="right">
                      Herav ønsker
                    </TableHeaderCell>
                    <TableHeaderCell scope="col" align="right">
                      Kappa
                    </TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {f.published.map((c) => (
                    <TableRow key={c.id}>
                      <TableDataCell>{c.name}</TableDataCell>
                      <TableDataCell align="right">{c.count === null ? "<5" : formatNumber(c.count)}</TableDataCell>
                      <TableDataCell align="right">{c.share === null ? "<5" : formatShare(c.share)}</TableDataCell>
                      <TableDataCell align="right">{c.onske === null ? "<5" : formatNumber(c.onske)}</TableDataCell>
                      <TableDataCell align="right">{kappa(c.kappa)}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            ))}
          </VStack>
        ))}
      </VStack>
    </section>
  );
}
