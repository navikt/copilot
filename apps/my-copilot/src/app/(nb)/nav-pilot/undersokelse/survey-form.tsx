"use client";

import { useEffect, useRef, useState, useTransition } from "react";
import {
  Alert,
  BodyLong,
  Button,
  Checkbox,
  CheckboxGroup,
  ErrorSummary,
  Fieldset,
  GuidePanel,
  Heading,
  Radio,
  RadioGroup,
  Textarea,
  TextField,
  VStack,
} from "@navikt/ds-react";
import {
  choseOther,
  isSkipped,
  optionsOf,
  otherKey,
  scaleSteps,
  type Answers,
  type Survey,
  type SubmitResult,
  type SurveyQuestion,
} from "@/lib/survey";
import { sendSurvey } from "./actions";

function missing(q: SurveyQuestion, answers: Answers): string | undefined {
  if (q.type === "matrix") {
    const all = (q.items ?? []).every((it) => typeof answers[it.id] === "number");
    return q.required && !all ? "Svar på alle påstandene." : undefined;
  }
  const a = answers[q.id];
  if (q.type === "multi" && Array.isArray(a) && q.max_choices && a.length > q.max_choices) {
    return `Du kan velge opptil ${q.max_choices}.`;
  }
  if (q.type === "text" && typeof a === "string" && q.max_length && [...a.trim()].length > q.max_length) {
    return `Skriv høyst ${q.max_length} tegn.`;
  }
  const empty = a === undefined || (typeof a === "string" && a.trim() === "") || (Array.isArray(a) && a.length === 0);
  if (q.required && empty) return "Svar på dette spørsmålet.";
  return undefined;
}

export function SurveyForm({ survey }: { survey: Survey }) {
  const [answers, setAnswers] = useState<Answers>({});
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [result, setResult] = useState<SubmitResult | null>(null);
  const [pending, startTransition] = useTransition();
  const summaryRef = useRef<HTMLDivElement>(null);
  const resultRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (result) resultRef.current?.focus();
  }, [result]);

  const visible = survey.questions.filter((q) => !isSkipped(q, answers));
  const set = (id: string, value: Answers[string] | undefined) =>
    setAnswers((prev) => {
      const next = { ...prev };
      if (value === undefined) delete next[id];
      else next[id] = value;
      return next;
    });

  function submit(e: React.FormEvent) {
    e.preventDefault();
    const found: Record<string, string> = {};
    for (const q of visible) {
      const m = missing(q, answers);
      if (m) found[q.id] = m;
      const other = answers[otherKey(q)];
      if (
        choseOther(q, answers) &&
        typeof other === "string" &&
        q.max_length &&
        [...other.trim()].length > q.max_length
      ) {
        found[otherKey(q)] = `Skriv høyst ${q.max_length} tegn under «${q.other}».`;
      }
    }
    setErrors(found);
    if (Object.keys(found).length > 0) {
      requestAnimationFrame(() => summaryRef.current?.focus());
      return;
    }
    const toSend: Answers = {};
    for (const q of visible) {
      for (const it of q.items ?? []) {
        const n = answers[it.id];
        if (typeof n === "number") toSend[it.id] = n;
      }
      const a = answers[q.id];
      const other = answers[otherKey(q)];
      if (choseOther(q, answers) && typeof other === "string" && other.trim() !== "") {
        toSend[otherKey(q)] = other.trim();
      }
      if (typeof a === "string" && a.trim() === "") continue;
      if (a !== undefined) toSend[q.id] = typeof a === "string" ? a.trim() : a;
    }
    if (Object.keys(toSend).length === 0) {
      setErrors({ [visible[0].id]: "Svar på minst ett spørsmål." });
      requestAnimationFrame(() => summaryRef.current?.focus());
      return;
    }
    startTransition(async () => {
      setResult(await sendSurvey(survey.id, toSend));
    });
  }

  if (result && (result.status === "recorded" || result.status === "duplicate")) {
    return (
      <div ref={resultRef} tabIndex={-1}>
        {result.status === "recorded" ? (
          <Alert variant="success">Takk! Svaret ditt er sendt.</Alert>
        ) : (
          <Alert variant="info">Du har allerede svart på denne undersøkelsen. Dette svaret ble ikke lagret.</Alert>
        )}
      </div>
    );
  }

  return (
    <form onSubmit={submit} noValidate>
      <VStack gap="space-24">
        <Heading size="large" level="2">
          {survey.title}
        </Heading>
        {survey.intro && <BodyLong>{survey.intro}</BodyLong>}
        <GuidePanel poster={false}>
          <BodyLong spacing>
            Svarene er anonyme. Vi lagrer svarene og at de kom fra nettsiden, ikke navn, e-post eller noe annet som
            knytter dem til deg.
          </BodyLong>
          <BodyLong>
            Vi bruker innloggingen bare til å hindre at noen svarer to ganger. Siden ingenting knytter svarene til deg,
            kan de ikke endres eller trekkes tilbake etterpå.
          </BodyLong>
        </GuidePanel>

        {Object.keys(errors).length > 0 && (
          <ErrorSummary ref={summaryRef} heading="Rett dette før du sender svaret">
            {Object.entries(errors).map(([id, message]) => (
              <ErrorSummary.Item key={id} href={`#q-${id}`}>
                {`${survey.questions.find((q) => q.id === id || otherKey(q) === id)?.text ?? id}: ${message}`}
              </ErrorSummary.Item>
            ))}
          </ErrorSummary>
        )}

        {visible.map((q) => (
          <VStack key={q.id} gap="space-8">
            {q.type === "matrix" ? (
              <Matrix q={q} answers={answers} error={errors[q.id]} onChange={(id, n) => set(id, n)} />
            ) : (
              <Question q={q} value={answers[q.id]} error={errors[q.id]} onChange={(v) => set(q.id, v)} />
            )}
            {choseOther(q, answers) && (
              <TextField
                id={`q-${otherKey(q)}`}
                label={`${q.other}: skriv gjerne hva du tenker på (valgfritt)`}
                error={errors[otherKey(q)]}
                description="Ikke skriv noe som kan identifisere deg eller andre."
                maxLength={q.max_length}
                value={String(answers[otherKey(q)] ?? "")}
                onChange={(e) => set(otherKey(q), e.target.value)}
              />
            )}
          </VStack>
        ))}

        {result?.status === "invalid" && (
          <Alert variant="error" ref={resultRef} tabIndex={-1}>
            Svaret ble ikke godtatt. Last inn siden på nytt og prøv igjen.
          </Alert>
        )}
        {result?.status === "no-identity" && (
          <Alert variant="error" ref={resultRef} tabIndex={-1}>
            Vi fant ingen Nav-identitet i innloggingen din, så svaret ble ikke sendt.
          </Alert>
        )}
        {result?.status === "closed" && (
          <Alert variant="warning" ref={resultRef} tabIndex={-1}>
            Undersøkelsen er avsluttet, så svaret ble ikke sendt.
          </Alert>
        )}
        {result?.status === "error" && (
          <Alert variant="error" ref={resultRef} tabIndex={-1}>
            Svaret ble ikke sendt. Prøv igjen om litt.
          </Alert>
        )}

        <div>
          <Button type="submit" loading={pending}>
            Send svar
          </Button>
        </div>
      </VStack>
    </form>
  );
}

function Question({
  q,
  value,
  error,
  onChange,
}: {
  q: SurveyQuestion;
  value: Answers[string] | undefined;
  error?: string;
  onChange: (v: Answers[string] | undefined) => void;
}) {
  const legend = q.required ? q.text : `${q.text} (valgfritt)`;
  switch (q.type) {
    case "scale": {
      const steps = scaleSteps(q);
      return (
        <RadioGroup
          id={`q-${q.id}`}
          tabIndex={-1}
          legend={legend}
          error={error}
          value={typeof value === "number" ? String(value) : ""}
          onChange={(v: string) => onChange(Number(v))}
        >
          {steps.map((n, i) => (
            <Radio key={n} value={String(n)}>
              {q.labels?.[i] ?? String(n)}
            </Radio>
          ))}
        </RadioGroup>
      );
    }
    case "choice":
      return (
        <RadioGroup
          id={`q-${q.id}`}
          tabIndex={-1}
          legend={legend}
          error={error}
          value={typeof value === "string" ? value : ""}
          onChange={(v: string) => onChange(v)}
        >
          {optionsOf(q).map((o) => (
            <Radio key={o} value={o}>
              {o}
            </Radio>
          ))}
        </RadioGroup>
      );
    case "multi":
      return (
        <CheckboxGroup
          id={`q-${q.id}`}
          tabIndex={-1}
          legend={legend}
          description={
            /\(velg /i.test(q.text)
              ? undefined
              : q.max_choices
                ? `Velg opptil ${q.max_choices}.`
                : "Velg alle som passer."
          }
          error={error}
          value={Array.isArray(value) ? value : []}
          onChange={(v: string[]) => onChange(v.length > 0 ? v : undefined)}
        >
          {optionsOf(q).map((o) => (
            <Checkbox key={o} value={o}>
              {o}
            </Checkbox>
          ))}
        </CheckboxGroup>
      );
    case "text":
      return (
        <Textarea
          id={`q-${q.id}`}
          label={legend}
          description="Ikke skriv noe som kan identifisere deg eller andre."
          error={error}
          maxLength={q.max_length}
          value={typeof value === "string" ? value : ""}
          onChange={(e) => onChange(e.target.value)}
        />
      );
  }
}

/**
 * A matrix as a table: one row per statement, one column per scale step, a
 * radio in each cell named by its row and column headers. It scrolls sideways
 * when the screen is too narrow.
 */
function Matrix({
  q,
  answers,
  error,
  onChange,
}: {
  q: SurveyQuestion;
  answers: Answers;
  error?: string;
  onChange: (itemId: string, n: number) => void;
}) {
  const steps = scaleSteps(q);
  return (
    <Fieldset id={`q-${q.id}`} tabIndex={-1} legend={q.required ? q.text : `${q.text} (valgfritt)`} error={error}>
      <div className="overflow-x-auto">
        <table className="w-full border-collapse">
          <thead>
            <tr>
              <td />
              {steps.map((n, i) => (
                <th
                  key={n}
                  id={`${q.id}-step-${n}`}
                  scope="col"
                  className="text-center align-bottom font-normal"
                  style={{ padding: "0 var(--ax-space-8) var(--ax-space-8)" }}
                >
                  {q.labels?.[i] ?? String(n)}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(q.items ?? []).map((it) => (
              <tr key={it.id} className="border-t border-[var(--ax-border-neutral-subtle)]">
                <th
                  id={`${q.id}-item-${it.id}`}
                  scope="row"
                  className="text-left font-normal"
                  style={{ padding: "var(--ax-space-12) var(--ax-space-16) var(--ax-space-12) 0" }}
                >
                  {it.text}
                </th>
                {steps.map((n) => (
                  <td key={n} className="text-center" style={{ padding: "0 var(--ax-space-8)" }}>
                    <input
                      type="radio"
                      className="size-5 cursor-pointer"
                      name={`q-${it.id}`}
                      value={n}
                      checked={answers[it.id] === n}
                      onChange={() => onChange(it.id, n)}
                      aria-labelledby={`${q.id}-item-${it.id} ${q.id}-step-${n}`}
                    />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Fieldset>
  );
}
