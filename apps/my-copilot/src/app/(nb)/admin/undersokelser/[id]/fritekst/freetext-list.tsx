"use client";

import { useState, useTransition } from "react";
import { Alert, BodyLong, BodyShort, Box, Button, Heading, Search, VStack } from "@navikt/ds-react";
import type { FreetextField } from "@/lib/survey";
import { hideFreetext } from "./actions";

function Field({ surveyId, field }: { surveyId: string; field: FreetextField }) {
  const [query, setQuery] = useState("");
  const [hidden, setHidden] = useState<string[]>([]);
  const [failed, setFailed] = useState(false);
  const [pending, startTransition] = useTransition();
  const texts = field.texts.filter((t) => !hidden.includes(t.token));
  const shown = texts.filter((t) => t.text.toLowerCase().includes(query.trim().toLowerCase()));

  const hide = (token: string) => {
    if (!window.confirm("Skjule dette svaret? Det forsvinner fra admin og fra eksporten.")) return;
    startTransition(async () => {
      const ok = await hideFreetext(surveyId, token);
      setFailed(!ok);
      if (ok) setHidden((h) => [...h, token]);
    });
  };

  return (
    <section>
      <VStack gap="space-8">
        <Heading level="2" size="small">
          {field.question}
        </Heading>
        <BodyShort size="small">
          {field.other ? `Svar under «${field.other}»` : "Fritekst"}: {texts.length} svar
        </BodyShort>
        {texts.length > 0 ? (
          <Search
            label={`Søk i svarene på «${field.question}»`}
            hideLabel
            variant="simple"
            size="small"
            value={query}
            onChange={setQuery}
          />
        ) : null}
        {failed ? <Alert variant="error">Kunne ikke skjule svaret. Prøv igjen.</Alert> : null}
        {query && shown.length === 0 ? <BodyShort>Ingen svar passer søket.</BodyShort> : null}
        <ul className="flex flex-col gap-2">
          {shown.map((t) => (
            <li key={t.token}>
              <Box padding="space-12" borderRadius="8" background="neutral-soft">
                <div className="flex items-start justify-between gap-4">
                  <BodyLong className="whitespace-pre-wrap break-words">{t.text}</BodyLong>
                  <Button size="xsmall" variant="tertiary" disabled={pending} onClick={() => hide(t.token)}>
                    Skjul
                  </Button>
                </div>
              </Box>
            </li>
          ))}
        </ul>
      </VStack>
    </section>
  );
}

export function FreetextList({ surveyId, fields }: { surveyId: string; fields: FreetextField[] }) {
  if (fields.length === 0) return <BodyShort>Undersøkelsen har ingen fritekstspørsmål.</BodyShort>;
  return fields.map((f) => <Field key={f.id} surveyId={surveyId} field={f} />);
}
