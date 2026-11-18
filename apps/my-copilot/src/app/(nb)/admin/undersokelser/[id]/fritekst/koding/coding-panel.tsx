"use client";

import { useState, useTransition } from "react";
import { useRouter } from "next/navigation";
import {
  Alert,
  BodyLong,
  BodyShort,
  Box,
  Button,
  Checkbox,
  Heading,
  HStack,
  ReadMore,
  Table,
  VStack,
} from "@navikt/ds-react";
import { formatNumber, formatShare } from "@/lib/format";
import type { CodedField, CodingField, CodingText, FreetextCode, SurveyCoding } from "@/lib/survey";

async function post(surveyId: string, path: "coding" | "coding/model", body: string) {
  try {
    const res = await fetch(`/api/survey/${encodeURIComponent(surveyId)}/freetext/${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      redirect: "manual",
    });
    const json = (await res.json().catch(() => ({}))) as Record<string, unknown>;
    return { ok: res.ok, json };
  } catch {
    return { ok: false, json: {} };
  }
}

const codeLabel = (field: CodingField, c: FreetextCode) => {
  const name = field.categories.find((x) => x.id === c.category)?.name ?? c.category;
  return field.onske === "kodes" && c.onske ? `${name} (ønske)` : name;
};

const codesLabel = (field: CodingField, codes?: FreetextCode[]) =>
  codes?.length ? codes.map((c) => codeLabel(field, c)).join(", ") : "ingen";

/**
 * Picks codes for one text. A code is a category and onske, so where the field codes onske a category can be both an
 * experience and a wish, as two codes.
 */
function CodePicker({
  field,
  initial,
  submitLabel,
  pending,
  onSubmit,
}: {
  field: CodingField;
  initial: FreetextCode[];
  submitLabel: string;
  pending: boolean;
  onSubmit: (codes: FreetextCode[]) => void;
}) {
  const [codes, setCodes] = useState<FreetextCode[]>(initial);
  const has = (category: string, onske: boolean) => codes.some((c) => c.category === category && c.onske === onske);
  const toggle = (category: string, onske: boolean) =>
    setCodes((cs) =>
      has(category, onske)
        ? cs.filter((c) => !(c.category === category && c.onske === onske))
        : [...cs, { category, onske }]
    );
  const tooMany = codes.length > field.max_codes;
  const annetMixed = codes.length > 1 && codes.some((c) => c.category === "annet");
  const choices: { onske: boolean; label: string }[] =
    field.onske === "kodes"
      ? [
          { onske: false, label: "Erfaring" },
          { onske: true, label: "Ønske" },
        ]
      : [{ onske: field.onske === "alltid", label: "" }];

  return (
    <VStack gap="space-8">
      <BodyShort size="small">
        Velg 1–{field.max_codes} koder.{" "}
        {field.onske === "kodes" ? "Samme kategori kan være både erfaring og ønske. " : ""}
        «Annet» brukes alene.
      </BodyShort>
      {field.categories.map((cat) => (
        <Box
          key={cat.id}
          padding="space-8"
          borderRadius="8"
          background={codes.some((c) => c.category === cat.id) ? "accent-soft" : "neutral-soft"}
        >
          <HStack gap="space-16" align="center" justify="space-between">
            {choices.map((ch) => (
              <Checkbox
                key={String(ch.onske)}
                checked={has(cat.id, ch.onske)}
                onChange={() => toggle(cat.id, ch.onske)}
              >
                {ch.label ? `${cat.name}: ${ch.label.toLowerCase()}` : cat.name}
              </Checkbox>
            ))}
          </HStack>
          <ReadMore header={`Definisjon av «${cat.name}»`} size="small">
            <VStack gap="space-4">
              <BodyShort size="small">{cat.definition}</BodyShort>
              {cat.include.length ? <BodyShort size="small">Ta med: {cat.include.join(" ")}</BodyShort> : null}
              {cat.exclude.length ? <BodyShort size="small">Hold utenfor: {cat.exclude.join(" ")}</BodyShort> : null}
            </VStack>
          </ReadMore>
        </Box>
      ))}
      {tooMany ? (
        <Alert variant="warning" size="small">
          Høyst {field.max_codes} koder.
        </Alert>
      ) : null}
      {annetMixed ? (
        <Alert variant="warning" size="small">
          «Annet» brukes alene.
        </Alert>
      ) : null}
      <div>
        <Button
          size="small"
          disabled={pending || codes.length === 0 || tooMany || annetMixed}
          onClick={() => onSubmit(codes)}
        >
          {submitLabel}
        </Button>
      </div>
    </VStack>
  );
}

function fieldOf(coding: SurveyCoding, id: string) {
  return coding.codebook.fields.find((f) => f.id === id)!;
}

/** One text the person has not coded. The model's codes come back only after saving. */
function NextText({ coding }: { coding: SurveyCoding }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [failed, setFailed] = useState(false);
  const [model, setModel] = useState<FreetextCode[] | null | undefined>(undefined);
  const next = coding.next;
  if (!next) return <BodyShort>Du har kodet alle tekstene.</BodyShort>;
  const field = fieldOf(coding, next.field);

  const save = (codes: FreetextCode[]) =>
    startTransition(async () => {
      const res = await post(coding.id, "coding", JSON.stringify({ token: next.token, coder: "human", codes }));
      setFailed(!res.ok);
      if (res.ok) setModel((res.json.model as FreetextCode[] | null) ?? null);
    });

  return (
    <VStack gap="space-12">
      <BodyShort size="small" weight="semibold">
        {field.question}
        {field.other ? ` (svar under «${field.other}»)` : ""}
      </BodyShort>
      <Box padding="space-12" borderRadius="8" background="neutral-soft">
        <BodyLong className="whitespace-pre-wrap break-words">{next.text}</BodyLong>
      </Box>
      {failed ? <Alert variant="error">Kunne ikke lagre kodingen. Prøv igjen.</Alert> : null}
      {model === undefined ? (
        <CodePicker
          key={next.token}
          field={field}
          initial={[]}
          submitLabel="Lagre koding"
          pending={pending}
          onSubmit={save}
        />
      ) : (
        <VStack gap="space-8">
          <BodyShort>Lagret. Modellen kodet: {model ? codesLabel(field, model) : "ikke kodet ennå"}.</BodyShort>
          <div>
            <Button
              size="small"
              onClick={() => {
                setModel(undefined);
                router.refresh();
              }}
            >
              Neste tekst
            </Button>
          </div>
        </VStack>
      )}
    </VStack>
  );
}

function Disagreement({ coding, text }: { coding: SurveyCoding; text: CodingText }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [failed, setFailed] = useState(false);
  const field = fieldOf(coding, text.field);
  const decide = (codes: FreetextCode[]) =>
    startTransition(async () => {
      const res = await post(coding.id, "coding", JSON.stringify({ token: text.token, coder: "final", codes }));
      setFailed(!res.ok);
      if (res.ok) router.refresh();
    });
  return (
    <Box padding="space-12" borderRadius="8" borderWidth="1" borderColor="neutral-subtle">
      <VStack gap="space-8">
        <BodyShort size="small" weight="semibold">
          {field.question}
        </BodyShort>
        <BodyLong className="whitespace-pre-wrap break-words">{text.text}</BodyLong>
        <BodyShort size="small">Du: {codesLabel(field, text.human)}</BodyShort>
        <BodyShort size="small">Modellen: {codesLabel(field, text.model)}</BodyShort>
        {failed ? <Alert variant="error">Kunne ikke lagre avgjørelsen. Prøv igjen.</Alert> : null}
        <ReadMore header="Avgjør" size="small">
          <CodePicker
            field={field}
            initial={text.human ?? []}
            submitLabel="Lagre avgjørelse"
            pending={pending}
            onSubmit={decide}
          />
        </ReadMore>
      </VStack>
    </Box>
  );
}

function ModelImport({ surveyId }: { surveyId: string }) {
  const router = useRouter();
  const [pending, startTransition] = useTransition();
  const [result, setResult] = useState<string | null>(null);
  const upload = (file: File) =>
    startTransition(async () => {
      const res = await post(surveyId, "coding/model", await file.text());
      setResult(
        res.ok
          ? `Importerte modellens koder for ${formatNumber(Number(res.json.saved))} tekster. ${formatNumber(Number(res.json.unmatched))} fant ingen tekst.`
          : `Importen feilet: ${String(res.json.error ?? "ukjent feil")}.`
      );
      if (res.ok) router.refresh();
    });
  return (
    <VStack gap="space-8">
      <BodyShort size="small">
        Filen fra <code>copilot-survey code</code> inneholder bare hasher av maskerte tekster og kodene, ingen tekst.
      </BodyShort>
      <input
        type="file"
        accept="application/json,.json"
        aria-label="Last opp modellens koder"
        disabled={pending}
        onChange={(e) => e.target.files?.[0] && upload(e.target.files[0])}
      />
      {result ? <BodyShort size="small">{result}</BodyShort> : null}
    </VStack>
  );
}

const kappaText = (k: number | null) => (k === null ? "–" : k.toFixed(2).replace(".", ","));
const statusText: Record<string, string> = { published: "Publiseres", merged: "Slås sammen", hidden: "Skjules" };

function Results({ field }: { field: CodedField }) {
  return (
    <VStack gap="space-8">
      <Heading level="3" size="xsmall">
        {field.question}
        {field.other ? ` (svar under «${field.other}»)` : ""}
      </Heading>
      <BodyShort size="small">
        {formatNumber(field.coded)} av {formatNumber(field.written)} som skrev, er ferdig kodet.
      </BodyShort>
      <Table size="small">
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell scope="col">Kategori</Table.HeaderCell>
            <Table.HeaderCell scope="col" align="right">
              Antall
            </Table.HeaderCell>
            <Table.HeaderCell scope="col" align="right">
              Ønske
            </Table.HeaderCell>
            <Table.HeaderCell scope="col" align="right">
              Kappa
            </Table.HeaderCell>
            <Table.HeaderCell scope="col">Publisering</Table.HeaderCell>
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {(field.categories ?? []).map((c) => (
            <Table.Row key={c.id}>
              <Table.DataCell>{c.name}</Table.DataCell>
              <Table.DataCell align="right">{formatNumber(c.count ?? 0)}</Table.DataCell>
              <Table.DataCell align="right">{formatNumber(c.onske ?? 0)}</Table.DataCell>
              <Table.DataCell align="right">{kappaText(c.kappa)}</Table.DataCell>
              <Table.DataCell>
                {statusText[c.status]}
                {c.merged_into ? ` (${c.merged_into})` : ""}
              </Table.DataCell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
      {field.published ? (
        <BodyShort size="small">
          Publiseres:{" "}
          {field.published.length
            ? field.published
                .map((c) => `${c.name} ${c.small ? "<5" : `${formatNumber(c.count!)} (${formatShare(c.share!)})`}`)
                .join(", ")
            : "ingen kategorier"}
        </BodyShort>
      ) : (
        <BodyShort size="small">Under 5 skrev i feltet: ingenting publiseres.</BodyShort>
      )}
    </VStack>
  );
}

export function CodingPanel({ coding }: { coding: SurveyCoding }) {
  const open = coding.progress.reduce((n, p) => n + p.disagreements, 0);
  return (
    <VStack gap="space-32">
      <section>
        <VStack gap="space-12">
          <Heading level="2" size="small">
            Fremdrift (kodebok versjon {coding.codebook.version})
          </Heading>
          <Table size="small">
            <Table.Header>
              <Table.Row>
                <Table.HeaderCell scope="col">Felt</Table.HeaderCell>
                <Table.HeaderCell scope="col" align="right">
                  Tekster
                </Table.HeaderCell>
                <Table.HeaderCell scope="col" align="right">
                  Kodet av deg
                </Table.HeaderCell>
                <Table.HeaderCell scope="col" align="right">
                  Kodet av modellen
                </Table.HeaderCell>
                <Table.HeaderCell scope="col" align="right">
                  Ferdige
                </Table.HeaderCell>
                <Table.HeaderCell scope="col" align="right">
                  Uenige
                </Table.HeaderCell>
              </Table.Row>
            </Table.Header>
            <Table.Body>
              {coding.progress.map((p) => (
                <Table.Row key={p.field}>
                  <Table.DataCell>{fieldOf(coding, p.field).question}</Table.DataCell>
                  <Table.DataCell align="right">{formatNumber(p.texts)}</Table.DataCell>
                  <Table.DataCell align="right">{formatNumber(p.human)}</Table.DataCell>
                  <Table.DataCell align="right">{formatNumber(p.model)}</Table.DataCell>
                  <Table.DataCell align="right">{formatNumber(p.final)}</Table.DataCell>
                  <Table.DataCell align="right">{formatNumber(p.disagreements)}</Table.DataCell>
                </Table.Row>
              ))}
            </Table.Body>
          </Table>
        </VStack>
      </section>
      <section>
        <VStack gap="space-12">
          <Heading level="2" size="small">
            Kod neste tekst
          </Heading>
          <NextText coding={coding} />
        </VStack>
      </section>
      <section>
        <VStack gap="space-12">
          <Heading level="2" size="small">
            Uenighet ({formatNumber(open)})
          </Heading>
          {coding.disagreements.length === 0 ? (
            <BodyShort>Ingen uavklart uenighet.</BodyShort>
          ) : (
            coding.disagreements.map((t) => <Disagreement key={t.token} coding={coding} text={t} />)
          )}
        </VStack>
      </section>
      <section>
        <VStack gap="space-12">
          <Heading level="2" size="small">
            Modellens koder
          </Heading>
          <ModelImport surveyId={coding.id} />
        </VStack>
      </section>
      <section>
        <VStack gap="space-16">
          <Heading level="2" size="small">
            Resultat og samsvar
          </Heading>
          <BodyShort size="small">
            Kappa er samsvaret mellom deg og modellen per kategori. Under 0,6 slås kategorien sammen med partneren sin
            hvis paret når 0,6, ellers skjules den. Antall under 5 publiseres som «&lt;5».
          </BodyShort>
          {coding.results.map((f) => (
            <Results key={f.id} field={f} />
          ))}
        </VStack>
      </section>
    </VStack>
  );
}
