import { BodyLong, BodyShort, Box, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Din første decide-hook",
  description:
    "Lag en commit-msg-hook som spør den lokale modellen om commit-meldingen forklarer hvorfor, og advarer uten å stoppe commiten.",
};

const TOC: TocItem[] = [
  { id: "start-serveren", label: "1. Start serveren" },
  { id: "lag-skriptet", label: "2. Lag skriptet" },
  { id: "koble-til", label: "3. Koble det til git" },
  { id: "prov", label: "4. Prøv hooken" },
  { id: "mal-sporsmalet", label: "5. Mål spørsmålet" },
  { id: "uten-server", label: "Når serveren ikke kjører" },
];

// String.raw keeps the shell's \n and \ intact.
const COMMIT_EXPLAINS_WHY_HOOK = String.raw`#!/bin/sh
# Advarer når meldingen bare beskriver det diffen viser. Stopper aldri commiten.
command -v nav-pilot >/dev/null 2>&1 || exit 0

{
  printf 'Commit message:\n-----\n'
  grep -v '^#' "$1"
  printf -- '-----\n\nDiff:\n-----\n'
  git diff --cached | head -c 7500
  printf -- '\n-----\n'
} | nav-pilot alpha decide \
  "Does the commit message explain why the change was made, beyond describing what the diff already shows?" \
  --options yes,no --evidence - --threshold 0.7 --expect no \
  --timeout 3s >/dev/null 2>&1

if [ $? -eq 0 ]; then
  echo "commit-msg: meldingen ser ut til å si hva som endret seg, men ikke hvorfor." >&2
fi
exit 0`;

const DECIDE_EVAL_CASES = String.raw`{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nDiff:\n...","expect":"no"}
{"question":"Does the commit message explain why ...?","options":["yes","no"],"evidence":"Commit message:\nfix: bump timeout to 30s\n\nThe batch job takes 20s on large tenants.\n\nDiff:\n...","expect":"yes"}`;

export default function DecideHook() {
  return (
    <DocPage
      label="Introduksjon"
      title="Din første decide-hook"
      description="Du lager en commit-msg-hook som spør den lokale modellen om commit-meldingen forklarer hvorfor endringen ble gjort. Hooken advarer, men stopper aldri commiten."
      badge={
        <Tag variant="warning" size="small" className="uppercase tracking-wide">
          Alfa
        </Tag>
      }
      toc={TOC}
    >
      <BodyLong>
        Du trenger en lokal modell som kjører, enten{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          på Macen
        </NextLink>{" "}
        eller på{" "}
        <NextLink href="/nav-pilot/lokal/egen-server" className={linkClass}>
          egen server
        </NextLink>
        . Spørsmålet, meldingen og diffen forlater ikke maskinen.
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="start-serveren" size="medium" level="2">
            1. Start serveren
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot alpha local start
nav-pilot alpha local status`}</CodeBlock>
          <BodyLong>
            <code className={code}>decide</code> starter ikke serveren selv. Med egen server starter du den som du
            pleier.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lag-skriptet" size="medium" level="2">
            2. Lag skriptet
          </LinkableHeading>
          <BodyLong>
            Lagre dette som <code className={code}>scripts/commit-explains-why.sh</code> i repoet:
          </BodyLong>
          <CodeBlock compact filename="scripts/commit-explains-why.sh">
            {COMMIT_EXPLAINS_WHY_HOOK}
          </CodeBlock>
          <CodeBlock compact>{`chmod +x scripts/commit-explains-why.sh`}</CodeBlock>
          <BodyLong>
            Skriptet sender meldingen og de første 7 500 tegnene av diffen til modellen. Er sannsynligheten for «no»
            minst 0,7, skriver det en advarsel. Det avslutter alltid med 0, så commiten går gjennom uansett. Uten
            nav-pilot på maskinen gjør det ingenting.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="koble-til" size="medium" level="2">
            3. Koble det til git
          </LinkableHeading>
          <CodeBlock
            compact
          >{`ln -s "$PWD/scripts/commit-explains-why.sh" "$(git rev-parse --git-path hooks)/commit-msg"`}</CodeBlock>
          <BodyLong>
            Kjør kommandoen fra rota av repoet. Git kjører <code className={code}>commit-msg</code> med stien til fila
            meldingen ligger i, og skriptet leser den som <code className={code}>$1</code>.
          </BodyLong>
          <BodyLong>
            <code className={code}>git rev-parse --git-path hooks</code> gir mappa git henter hooks fra. Uten{" "}
            <code className={code}>core.hooksPath</code> er det <code className={code}>.git/hooks</code>, og hooken
            gjelder bare denne klonen. Har du satt <code className={code}>core.hooksPath</code> globalt, havner lenken i
            den felles mappa, og hooken kjører i alle repoer som bruker den. Sjekk med{" "}
            <code className={code}>git config --show-origin core.hooksPath</code> før du lenker.
          </BodyLong>
          <LinkableHeading id="pre-commit" size="small" level="3">
            Med pre-commit
          </LinkableHeading>
          <CodeBlock compact filename=".pre-commit-config.yaml">
            {`repos:
  - repo: local
    hooks:
      - id: commit-explains-why
        name: commit-meldingen forklarer hvorfor
        entry: scripts/commit-explains-why.sh
        language: script
        stages: [commit-msg]`}
          </CodeBlock>
          <CodeBlock compact>{`pre-commit install --hook-type commit-msg`}</CodeBlock>
          <LinkableHeading id="lefthook" size="small" level="3">
            Med Lefthook
          </LinkableHeading>
          <BodyLong>
            <code className={code}>{"{1}"}</code> er fila git lagrer meldingen i:
          </BodyLong>
          <CodeBlock compact filename="lefthook.yml">
            {`commit-msg:
  commands:
    explains-why:
      run: scripts/commit-explains-why.sh {1}`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="prov" size="medium" level="2">
            4. Prøv hooken
          </LinkableHeading>
          <CodeBlock compact>{`git commit --allow-empty -m "Legg til retry i klienten"`}</CodeBlock>
          <BodyLong>
            Meldingen sier hva, men ikke hvorfor. Svarer modellen «no» med minst 0,7, ser du linjen{" "}
            <code className={code}>commit-msg: meldingen ser ut til å si hva som endret seg, men ikke hvorfor.</code>{" "}
            før git bekrefter commiten. Prøv igjen med en melding som forklarer hvorfor, og advarselen skal utebli.
            Angre en prøvecommit med <code className={code}>git reset --soft HEAD~1</code>.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="mal-sporsmalet" size="medium" level="2">
            5. Mål spørsmålet
          </LinkableHeading>
          <BodyLong>
            Hvor godt modellen svarer på et spørsmål, vet du ikke før du har målt det. Lag en JSONL-fil med meldinger
            fra repoet ditt der du vet svaret, én per linje, og omtrent like mange av hvert svar:
          </BodyLong>
          <CodeBlock compact filename="cases.jsonl">
            {DECIDE_EVAL_CASES}
          </CodeBlock>
          <CodeBlock compact>{`nav-pilot alpha decide --eval cases.jsonl`}</CodeBlock>
          <BodyLong>
            Du får treffsikkerhet, en forvekslingsmatrise, snittet av sannsynligheten for riktige og gale svar, og
            svartid. Er modellen like sikker når den tar feil som når den har rett, hjelper ingen terskel. Da bør
            spørsmålet ikke inn i en hook. Våre tall for dette spørsmålet står i{" "}
            <NextLink href="/nav-pilot/forklaring/lokal-modell#malt-decide" className={linkClass}>
              målte grenser
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="uten-server" size="medium" level="2">
            Når serveren ikke kjører
          </LinkableHeading>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Hooken slipper alt gjennom uten å si fra</Label>
              <BodyShort size="small">
                Uten server svarer <code className={code}>decide</code> med kode 2. Skriptet skriver bare advarselen ved
                kode 0, og avslutter alltid med 0. Commiten går derfor gjennom uten advarsel, også når meldingen ikke
                forklarer hvorfor. Det samme skjer når svaret tar mer enn tre sekunder.
              </BodyShort>
            </VStack>
          </Box>
          <BodyLong>
            Flere oppskrifter, som etikettforslag på issues og sjekk av PR-beskrivelser, står i{" "}
            <NextLink href="/nav-pilot/guider/lokal#decide-oppskrifter" className={linkClass}>
              Oppskrifter for alpha decide
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
