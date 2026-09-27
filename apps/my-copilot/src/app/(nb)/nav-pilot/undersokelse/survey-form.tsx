"use client";

import { useRef, useState, useTransition } from "react";
import {
  Alert,
  BodyLong,
  Button,
  Checkbox,
  CheckboxGroup,
  ErrorSummary,
  GuidePanel,
  Heading,
  Radio,
  RadioGroup,
  Textarea,
  VStack,
} from "@navikt/ds-react";
import { isSkipped, type Answers, type Survey, type SubmitResult, type SurveyQuestion } from "@/lib/survey";
import { sendSurvey } from "./actions";

function missing(q: SurveyQuestion, answers: Answers): string | undefined {
  const a = answers[q.id];
  if (q.type === "multi" && Array.isArray(a) && q.max_choices && a.length > q.max_choices) {
    return `Velg høyst ${q.max_choices}.`;
  }
  const empty = a === undefined || a === "" || (Array.isArray(a) && a.length === 0);
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
    }
    setErrors(found);
    if (Object.keys(found).length > 0) {
      requestAnimationFrame(() => summaryRef.current?.focus());
      return;
    }
    const toSend: Answers = {};
    for (const q of visible) {
      const a = answers[q.id];
      if (typeof a === "string" && a.trim() === "") continue;
      if (a !== undefined) toSend[q.id] = typeof a === "string" ? a.trim() : a;
    }
    startTransition(async () => {
      setResult(await sendSurvey(survey.id, toSend));
      requestAnimationFrame(() => resultRef.current?.focus());
    });
  }

  if (result && (result.status === "recorded" || result.status === "duplicate")) {
    return (
      <div ref={resultRef} tabIndex={-1}>
        {result.status === "recorded" ? (
          <Alert variant="success">Takk! Svaret ditt er sendt.</Alert>
        ) : (
          <Alert variant="info">Du har allerede svart på denne undersøkelsen, så dette svaret ble ikke sendt.</Alert>
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
            Svaret er anonymt. Vi lagrer svarene dine, og at svaret kom fra nettsiden. Vi lagrer ikke navn, e-post eller
            noe annet som knytter svaret til deg.
          </BodyLong>
          <BodyLong spacing>
            Innloggingen brukes bare til å hindre at noen svarer to ganger. Fordi ingenting knytter svaret til deg, kan
            svaret ikke endres eller trekkes tilbake etterpå.
          </BodyLong>
          <BodyLong>Skriv ikke noe i fritekstfeltet som kan identifisere deg eller andre.</BodyLong>
        </GuidePanel>

        {Object.keys(errors).length > 0 && (
          <ErrorSummary ref={summaryRef} heading="Svar på disse spørsmålene før du sender">
            {Object.entries(errors).map(([id, message]) => (
              <ErrorSummary.Item key={id} href={`#q-${id}`}>
                {message}
              </ErrorSummary.Item>
            ))}
          </ErrorSummary>
        )}

        {visible.map((q) => (
          <Question key={q.id} q={q} value={answers[q.id]} error={errors[q.id]} onChange={(v) => set(q.id, v)} />
        ))}

        {result?.status === "invalid" && (
          <Alert variant="error" ref={resultRef} tabIndex={-1}>
            Svaret ble ikke godtatt{result.message ? `: ${result.message}` : ""}.
          </Alert>
        )}
        {result?.status === "no-identity" && (
          <Alert variant="error" ref={resultRef} tabIndex={-1}>
            Vi fant ikke Nav-identiteten din, så svaret ble ikke sendt.
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
      const steps: number[] = [];
      for (let n = q.min ?? 1; n <= (q.max ?? 5); n++) steps.push(n);
      return (
        <RadioGroup
          id={`q-${q.id}`}
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
          legend={legend}
          error={error}
          value={typeof value === "string" ? value : ""}
          onChange={(v: string) => onChange(v)}
        >
          {(q.options ?? []).map((o) => (
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
          {(q.options ?? []).map((o) => (
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
          description="Skriv ikke noe som kan identifisere deg eller andre."
          error={error}
          maxLength={q.max_length}
          value={typeof value === "string" ? value : ""}
          onChange={(e) => onChange(e.target.value)}
        />
      );
  }
}
