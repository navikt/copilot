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
  exclusivePick,
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

/** preview: the form as it will be, for a survey not open yet. Submit checks the answers and sends nothing. */
export function SurveyForm({ survey, preview = false }: { survey: Survey; preview?: boolean }) {
  const [previewed, setPreviewed] = useState(false);
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
    if (preview) {
      setPreviewed(true);
      return;
    }
    startTransition(async () => {
      setResult(await sendSurvey(survey.id, toSend));
    });
  }

  if (previewed) {
    return <Alert variant="info">Forhåndsvisning: svarene er godkjent av skjemaet, men ikke sendt eller lagret.</Alert>;
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
        {preview && (
          <Alert variant="warning">Forhåndsvisning. Undersøkelsen er ikke åpen, og svarene sendes ikke.</Alert>
        )}
        {survey.intro && <BodyLong>{survey.intro}</BodyLong>}
        <GuidePanel poster={false}>
          <BodyLong spacing>
            Svarene lagres uten navn, e-post eller bruker. Innloggingen brukes bare til å hindre dobbeltsvar: mens
            undersøkelsen er åpen, lagrer vi en nøkkelbasert sjekksum av e-posten din. Den slettes etter at
            undersøkelsen har stengt.
          </BodyLong>
          <BodyLong>Svarene kan ikke endres eller trekkes tilbake etterpå.</BodyLong>
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
              <Question
                q={q}
                value={answers[q.id]}
                error={errors[q.id]}
                onChange={(v) =>
                  set(
                    q.id,
                    q.type === "multi" && Array.isArray(v)
                      ? exclusivePick(q, (answers[q.id] as string[] | undefined) ?? [], v)
                      : v
                  )
                }
              />
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
    case "multi": {
      const picked = Array.isArray(value) ? value : [];
      // At the limit the unpicked boxes are disabled, and the description says
      // why. The exclusive option replaces the picks (exclusivePick), so it
      // stays enabled, and with it picked every other option does too.
      const full = !!q.max_choices && picked.length >= q.max_choices && !(q.exclusive && picked.includes(q.exclusive));
      return (
        <CheckboxGroup
          id={`q-${q.id}`}
          tabIndex={-1}
          legend={legend}
          description={
            // A live region from the start, so the change is announced while focus stays on a checkbox.
            <span aria-live="polite">
              {full
                ? `Du har valgt ${q.max_choices}. Fjern et valg for å velge noe annet.`
                : /\(velg /i.test(q.text)
                  ? undefined
                  : q.max_choices
                    ? `Velg opptil ${q.max_choices}.`
                    : "Velg alle som passer."}
            </span>
          }
          error={error}
          value={picked}
          onChange={(v: string[]) => onChange(v.length > 0 ? v : undefined)}
        >
          {optionsOf(q).map((o) => (
            <Checkbox key={o} value={o} disabled={full && !picked.includes(o) && o !== q.exclusive}>
              {o}
            </Checkbox>
          ))}
        </CheckboxGroup>
      );
    }
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
 * A matrix as one segmented row per statement: native radios styled as large
 * buttons, same on every screen width. Arrow keys move within a row, nothing
 * is preselected, and the chosen step gets a check mark besides the colour.
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
      <VStack gap="space-24">
        {(q.items ?? []).map((it) => (
          <fieldset key={it.id} className="m-0 border-0 p-0">
            <legend className="mb-2 font-semibold">{it.text}</legend>
            <div className="grid gap-1" style={{ gridTemplateColumns: `repeat(${steps.length}, minmax(0, 1fr))` }}>
              {steps.map((n, i) => (
                <label key={n} className="seg">
                  <input
                    type="radio"
                    name={`q-${it.id}`}
                    value={n}
                    className="sr-only"
                    checked={answers[it.id] === n}
                    onChange={() => onChange(it.id, n)}
                  />
                  <span>{q.labels?.[i] ?? String(n)}</span>
                </label>
              ))}
            </div>
          </fieldset>
        ))}
      </VStack>
      <style>{`
        .seg { position: relative; } /* the sr-only radio sits on its button, so focus scrolls there */
        .seg span { display:flex; align-items:center; justify-content:center; gap:4px; text-align:center; min-height:48px;
          padding:4px; border:1px solid var(--ax-border-neutral); border-radius:8px; cursor:pointer; font-size:0.875rem; }
        .seg span:hover { background: var(--ax-bg-neutral-moderate-hover); }
        .seg input:checked + span { background: var(--ax-bg-accent-strong); color: var(--ax-text-accent-contrast);
          border-color: transparent; font-weight: 600; }
        .seg input:checked + span::before { content: "\\2713" / ""; } /* empty alt: not part of the radio's name */
        .seg input:focus-visible + span { outline: 3px solid var(--ax-border-focus); outline-offset: 2px; }
      `}</style>
    </Fieldset>
  );
}
