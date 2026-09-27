import { BodyLong, BodyShort, Box, HStack, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Lokal modell — nav-pilot",
  description:
    "Bestem hvor mye hovedagenten sender til den lokale modellen, bytt modell i skyen og lokalt, bruk en server du kjører selv, og bruk alpha decide i hooks og skript.",
};

const TOC: TocItem[] = [
  { id: "utsending", label: "Styr utsendingen" },
  { id: "bytte-modell", label: "Bytte modell i skyen" },
  { id: "bytte-lokal-modell", label: "Bytte lokal modell" },
  { id: "egen-server", label: "Bruke egen server" },
  { id: "decide-oppskrifter", label: "Oppskrifter for decide" },
];

// Recipes for alpha decide. String.raw keeps the shell's \n and \ intact.

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

const DECIDE_PR_DESCRIPTION = String.raw`gh pr view N --json title,body \
    -q '"Pull request title: " + .title + "\n-----\n"
        + (if (.body // "") == "" then "(empty)" else .body end) + "\n-----"' \
  | nav-pilot alpha decide \
    "Does this pull request description explain why the change is needed?" \
    --options yes,no --evidence -`;

const DECIDE_ISSUE_LABEL = String.raw`gh issue view N --json title,body -q '"Title: " + .title + "\n\n" + (.body // "")' \
  | nav-pilot alpha decide \
    "Is this GitHub issue a bug report (something does not work as intended), a feature request (new or changed functionality), or a question (something to clarify, investigate or decide)?" \
    --options bug,feature,question --evidence - --json \
  | jq -r 'select(.p[.choice] >= 0.9) | .choice'`;

const DECIDE_LOG_TRIAGE = String.raw`kubectl logs deploy/min-app --since=1h | tail -c 30000 \
  | nav-pilot alpha decide \
    "Do these logs show the app failing to reach a dependency?" \
    --options yes,no --evidence -`;

function Recipe({ title, tag }: { title: string; tag: ReactNode }) {
  return (
    <HStack gap="space-8" align="center">
      <Label size="small">{title}</Label>
      {tag}
    </HStack>
  );
}

export default function LokalGuide() {
  return (
    <DocPage
      label="Guider"
      title="Lokal modell"
      description="Oppskrifter for deg som har satt opp en lokal modell. Er du ikke der ennå, start med introduksjonen."
      toc={TOC}
    >
      <BodyLong>
        Oppsettet står i{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          Lokal modell på Mac
        </NextLink>
        . Den lokale modellen er alfa og av som standard.
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="utsending" size="medium" level="2">
            Styr utsendingen
          </LinkableHeading>
          <BodyLong>
            Utsending krever opencode som klient. Der blir den lokale modellen en underagent som heter{" "}
            <code className={code}>local-worker</code>, og hovedagenten i skyen sender avgrensede oppgaver dit.
            nav-pilot legger inn <code className={code}>local-worker</code> selv hvis agentpakka ikke har den. Har pakka
            eller repoet en egen, brukes den. Copilot CLI har ingen slik underagent, så der kjører hele økten enten
            lokalt eller i skyen.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot config get client               # hvilken klient du kjører
nav-pilot config set client opencode
nav-pilot config set local_dispatch <nivå>  # eller --local-dispatch <nivå> for én økt`}
          </CodeBlock>
          <Bullets>
            <li>
              <code className={code}>off</code>: hovedagenten får ingen lokal underagent. Vil du slå av alt lokalt, bruk{" "}
              <code className={code}>nav-pilot alpha local off</code>.
            </li>
            <li>
              <code className={code}>conservative</code>: bare store mekaniske endringer (10 filer eller 20 kallsteder
              og mer), og hovedagenten vurderer selv om det er verdt det.
            </li>
            <li>
              <code className={code}>balanced</code> (standard): mekaniske endringer på 5 filer eller 10 kallsteder og
              mer. Redigerer hovedagenten selv en femte fil i samme tur, stopper nav-pilot redigeringen én gang og ber
              om at resten sendes til <code className={code}>local-worker</code>. Trenger endringen en vurdering per
              fil, går samme redigering gjennom andre gang.
            </li>
            <li>
              <code className={code}>aggressive</code>: en stoppet fil slipper gjennom først når den er sendt til{" "}
              <code className={code}>local-worker</code>. Nye filer, også tester, går dit først når modellen er godkjent
              for nye filer.
            </li>
          </Bullets>
          <BodyLong>
            Uansett nivå sendes bare oppgavetyper modellen er godkjent for. Stoppet ligger i en plugin for opencode, så
            det virker ikke hvis du starter opencode med <code className={code}>--pure</code>. Hvorfor nivåene finnes,
            og hva de gjør med hver modell, står i{" "}
            <NextLink href="/nav-pilot/forklaring/lokal-modell#utsending" className={linkClass}>
              Hvorfor utsendingen er begrenset
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="bytte-modell" size="medium" level="2">
            Bytte modell i skyen
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot models</code> viser modellene klienten kan bruke, med den du har valgt
            merket <code className={code}>*</code>. Lista er nav-pilot sin. Hva du faktisk får bruke, avhenger også av
            Copilot-abonnementet ditt.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot models                      # alle, for klienten din
nav-pilot models claude opus          # bare de med disse ordene i navnet
nav-pilot models --client opencode    # for en annen klient
nav-pilot config set model <id>       # velg modell`}
          </CodeBlock>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="bytte-lokal-modell" size="medium" level="2">
            Bytte lokal modell
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot alpha local models</code> viser de lokale modellene: størrelse, kontekst,
            hva de er anbefalt til, om de er lastet ned eller kjører, og hvilken serveren laster (merket{" "}
            <code className={code}>*</code>). <code className={code}>use</code> skriver valget til{" "}
            <code className={code}>local_model</code>, men laster ikke ned og starter ikke noe.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot alpha local models
nav-pilot alpha local use qwen3.8-27b-optiq-4bit
nav-pilot alpha local init      # laster ned vektene hvis de mangler, og starter
nav-pilot alpha local restart   # hvis serveren allerede kjører en annen modell`}
          </CodeBlock>
          <BodyLong>
            Vektene til den nye modellen lastes ned én gang. Størrelsene står i{" "}
            <NextLink href="/nav-pilot/referanse#lokale-modeller" className={linkClass}>
              tabellen over lokale modeller
            </NextLink>
            . Lista oppdateres når du kjører <code className={code}>init</code> eller{" "}
            <code className={code}>start</code>, ikke ved hver kommando.
          </BodyLong>
          <BodyLong>
            <code className={code}>status</code> viser hvilken modell som er valgt, og om den er valgt med{" "}
            <code className={code}>local_model</code> eller er standard. Kjører serveren en annen modell, gir status deg
            kommandoen for omstart. Krever modellen en nyere nav-pilot, sier status at den har falt tilbake til
            standard. Er modellen du har valgt, erstattet i manifestet, bruker nav-pilot den gamle til erstatningen er
            lastet ned.
          </BodyLong>
          <BodyLong>
            <code className={code}>purge</code> fjerner Python-miljøet, den valgte modellen og modeller manifestet har
            erstattet. <code className={code}>purge --all</code> fjerner alle. Ingenting slettes før du legger til{" "}
            <code className={code}>--yes</code>.
          </BodyLong>
          <LinkableHeading id="autostart" size="small" level="3">
            Start serveren automatisk
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot config set local_autostart true`}</CodeBlock>
          <BodyLong>
            Da starter en vanlig <code className={code}>nav-pilot</code> serveren når den trengs, og venter til den er
            klar. To oppstarter samtidig gir ikke to servere. Den er av som standard fordi serveren bruker rundt 21 GB
            minne. En automatisk start ber aldri om passord. Er minnegrensen i macOS for lav, skriver den kommandoen du
            må kjøre i stedet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="egen-server" size="medium" level="2">
            Bruke egen server{" "}
            <Tag variant="warning" size="small">
              alfa, ikke målt
            </Tag>
          </LinkableHeading>
          <BodyLong>
            Har du Linux, eller vil du bruke Ollama, llama-server eller LM Studio på Macen, kan nav-pilot bruke en
            server du kjører selv. Da laster nav-pilot ikke ned noe og starter ingenting. Løkkevakta, utsendingen og{" "}
            <code className={code}>alpha decide</code> går til serveren din. Koden din sendes dit, så nav-pilot godtar
            bare localhost og private IP-adresser, som 127.0.0.1 og 192.168.x.x.
          </BodyLong>
          <BodyLong>
            Enklest er <code className={code}>nav-pilot alpha local setup</code>. Den finner servere som kjører på
            maskinen (Ollama, llama-server, LM Studio, vLLM), foreslår modellen som ligger nærmest vår egen og sjekker
            den. Mangler modellen i Ollama, eller er konteksten for liten, tilbyr den å hente modellen eller lage en
            kopi med 64k kontekst. Den spør først, og starter aldri en server selv. Uten terminal trenger den{" "}
            <code className={code}>--pull</code>, <code className={code}>--fix-context</code> og{" "}
            <code className={code}>--yes</code>. For hånd:
          </BodyLong>
          <CodeBlock compact>
            {`# Ollama
OLLAMA_CONTEXT_LENGTH=65536 ollama serve
ollama pull qwen3.6:35b
nav-pilot config set local_endpoint http://127.0.0.1:11434/v1
nav-pilot config set local_endpoint_model qwen3.6:35b

# llama-server (llama.cpp); legg til --n-cpu-moe 999 på en GPU med 8 GB
llama-server --jinja -c 65536 --port 8080 -hf unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL
nav-pilot config set local_endpoint http://127.0.0.1:8080/v1
nav-pilot config set local_endpoint_model unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL

nav-pilot alpha local init     # sjekker serveren og skrur på utsending
nav-pilot alpha local doctor   # verktøykall, logprobs, kontekst og tid til første token`}
          </CodeBlock>
          <BodyLong>
            Vi anbefaler Qwen3.6-35B-A3B i dynamisk 4-bit (unsloth UD-Q4_K_XL). Det er den GGUF-varianten som ligger
            nærmest modellen vi har målt på Mac, men tallene våre gjelder ikke for den. Modell-id-en er det serveren
            lister på <code className={code}>/v1/models</code>, og <code className={code}>doctor</code> viser dem.
            Hovedagenten får den eldre, generelle instruksen, og nav-pilot stopper ingenting.
          </BodyLong>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Ollama kutter lange prompter uten å si fra</Label>
              <BodyShort size="small">
                På maskiner med under 24 GB grafikkminne gir Ollama modellen 4 096 tokens kontekst, og det kan ikke
                endres via <code className={code}>/v1</code>. En økt i Copilot starter med rundt 22 000 tokens, så
                resten blir borte. Start Ollama med <code className={code}>OLLAMA_CONTEXT_LENGTH=65536</code>, eller lag
                en egen modell med en Modelfile som har <code className={code}>PARAMETER num_ctx 65536</code>.{" "}
                <code className={code}>doctor</code> sender rundt 30 000 tokens og feiler hvis serveren kutter.
              </BodyShort>
            </VStack>
          </Box>
          <BodyShort size="small" textColor="subtle">
            <code className={code}>alpha decide</code> trenger logprobs. Ollama fra v0.12.11, llama-server og vLLM gir
            dem, LM Studio gjør det ikke. Uten logprobs sier <code className={code}>decide</code> fra med en gang, mens
            utsendingen virker som før.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="decide-oppskrifter" size="medium" level="2">
            Oppskrifter for alpha decide
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot alpha decide</code> stiller den lokale modellen ett flervalgsspørsmål og
            svarer med en sannsynlighet for hvert alternativ. Et varmt svar tar under ett sekund, og spørsmålet forlater
            ikke maskinen. Bruk den til vurderinger en regel ikke klarer: om en commit-melding følger Conventional
            Commits, avgjør et regulært uttrykk, men om den forklarer hvorfor, må en modell vurdere. Med{" "}
            <code className={code}>--threshold</code> og <code className={code}>--expect</code> blir svaret kode 0 eller
            1. Serveren må kjøre, for <code className={code}>decide</code> starter den ikke selv.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot alpha decide \\
  "Does the commit message explain why?" \\
  --options yes,no --evidence msg.txt
# {"choice":"yes","p":{"yes":0.93,"no":0.07},…}`}
          </CodeBlock>
          <BodyLong>
            Commit-hooken og etikettforslaget er målt. PR-sjekken er målt og svak, og loggsorteringen er ikke målt ennå.
            Alle advarer eller foreslår, ingen stopper noe. Tre råd når du skriver egne spørsmål:
          </BodyLong>
          <Bullets>
            <li>Still spørsmålet positivt: «Forklarer meldingen hvorfor?», ikke «Mangler meldingen en forklaring?».</li>
            <li>
              Sett <code className={code}>yes</code> først i alternativene.
            </li>
            <li>
              Kjør <code className={code}>--eval</code> på nøyaktig den ordlyden du skal bruke.
            </li>
          </Bullets>
          <BodyLong>
            «Ja»-svarene er stabile, men «nei»-svarene vipper mot «teksten er grei» når alternativene bytter plass eller
            spørsmålet snus. Standardmodellen svarte riktig på 85 % av spørsmålene i opprinnelig form og 58 % når de var
            snudd.{" "}
            <a
              href="https://github.com/navikt/mlx-workspace/blob/main/bench/decide-layout-results.md"
              className={linkClass}
            >
              Se målingen
            </a>
            .
          </BodyLong>

          <Recipe
            title="Commit-meldingen forklarer hvorfor"
            tag={
              <Tag size="small" variant="success">
                Målt
              </Tag>
            }
          />
          <BodyLong>
            Lagre skriptet som <code className={code}>scripts/commit-explains-why.sh</code>. Det advarer når meldingen
            bare sier hva diffen viser, og slipper alltid commiten gjennom. Uten nav-pilot på maskinen gjør det
            ingenting.
          </BodyLong>
          <CodeBlock compact>{COMMIT_EXPLAINS_WHY_HOOK}</CodeBlock>
          <CodeBlock compact>{`chmod +x scripts/commit-explains-why.sh`}</CodeBlock>
          <BodyLong>
            Med <code className={code}>pre-commit</code> legger du det inn som en lokal hook:
          </BodyLong>
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
          <BodyLong>
            Med Lefthook er <code className={code}>{"{1}"}</code> fila git lagrer meldingen i:
          </BodyLong>
          <CodeBlock compact filename="lefthook.yml">
            {`commit-msg:
  commands:
    explains-why:
      run: scripts/commit-explains-why.sh {1}`}
          </CodeBlock>

          <Label size="small">Mål ditt eget spørsmål</Label>
          <BodyLong>
            Lag en JSONL-fil med eksempler fra ditt eget repo der du vet svaret, ett per linje, og omtrent like mange av
            hvert svar. <code className={code}>--eval</code> viser treffsikkerhet, en forvekslingsmatrise, snittet av
            sannsynligheten for riktige og gale svar, og svartid. Er modellen like sikker når den tar feil som når den
            har rett, hjelper ingen terskel, og spørsmålet bør ikke inn i en hook.
          </BodyLong>
          <CodeBlock compact filename="cases.jsonl">
            {DECIDE_EVAL_CASES}
          </CodeBlock>
          <CodeBlock compact>{`nav-pilot alpha decide --eval cases.jsonl`}</CodeBlock>

          <Label size="small">Tekst andre har skrevet</Label>
          <BodyLong>
            PR-beskrivelser, issues og logger er skrevet av andre, og kan inneholde instrukser til modellen (prompt
            injection). La sjekker på slik tekst bare advare eller foreslå. Tallene under er fra{" "}
            <a
              href="https://github.com/navikt/mlx-workspace/blob/main/bench/decide-sets-20260925-225356.md"
              className={linkClass}
            >
              målingene 25. september
            </a>{" "}
            med standardmodellen.
          </BodyLong>

          <Recipe
            title="Foreslå en etikett på et issue"
            tag={
              <Tag size="small" variant="success">
                Målt
              </Tag>
            }
          />
          <BodyLong>
            Modellen valgte riktig mellom bug, feature og question på 95 av 105 issues (90 %). Når den bare fikk svare
            ved p ≥ 0,9, svarte den på omtrent to tredjedeler og hadde rett alle 71 gangene. Behold filteret: under 0,9
            gir kommandoen ingen etikett, og da setter du den selv.
          </BodyLong>
          <CodeBlock compact>{DECIDE_ISSUE_LABEL}</CodeBlock>

          <Recipe
            title="Forklarer PR-beskrivelsen hvorfor?"
            tag={
              <Tag size="small" variant="warning">
                Målt: svak, bruk som hint
              </Tag>
            }
          />
          <BodyLong>
            Den flagget ingen av de 24 beskrivelsene som forklarer hvorfor, men fant bare 3 av 12 der grunnen var tatt
            ut. Et «no» er verdt å se på. Et «yes» betyr lite.
          </BodyLong>
          <CodeBlock compact>{DECIDE_PR_DESCRIPTION}</CodeBlock>

          <Recipe
            title="Sorter en logg før du leser den selv"
            tag={
              <Tag size="small" variant="neutral">
                Ikke målt
              </Tag>
            }
          />
          <BodyShort size="small" textColor="subtle">
            <code className={code}>tail -c 30000</code> holder grunnlaget under grensen på 32 KiB.
          </BodyShort>
          <CodeBlock compact>{DECIDE_LOG_TRIAGE}</CodeBlock>
        </VStack>
      </section>
    </DocPage>
  );
}
