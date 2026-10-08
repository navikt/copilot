import { BodyLong, BodyShort, HStack, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import type { ReactNode } from "react";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Lokal modell",
  description:
    "Bestem hvor mye hovedagenten sender til den lokale modellen, bytt modell i skyen og lokalt, og bruk alpha decide i hooks og skript.",
};

const TOC: TocItem[] = [
  { id: "utsending", label: "Styr utsendingen" },
  { id: "bytte-modell", label: "Bytte modell i skyen" },
  { id: "bytte-lokal-modell", label: "Bytte lokal modell" },
  { id: "decide-oppskrifter", label: "Oppskrifter for alpha decide" },
];

// Recipes for alpha decide. String.raw keeps the shell's \n and \ intact.

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
      upgrade
      title="Lokal modell"
      description="Oppskrifter for deg som har satt opp en lokal modell. Er du ikke der ennå, start med oppsettet."
      toc={TOC}
    >
      <BodyLong>
        Oppsettet står i{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          Kom i gang med lokal modell på Mac
        </NextLink>{" "}
        og{" "}
        <NextLink href="/nav-pilot/lokal/egen-server" className={linkClass}>
          Kom i gang med egen server
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
            eller repoet en egen, bruker nav-pilot den. Copilot CLI har ingen slik underagent, så der kjører hele økten
            enten lokalt eller i skyen.
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot config get client               # hvilken klient du kjører
nav-pilot config set client opencode
nav-pilot config set local_dispatch <nivå>  # eller --local-dispatch <nivå> for én økt`}
          </CodeBlock>
          <BodyLong>
            Nivåene er <code className={code}>off</code>, <code className={code}>conservative</code>,{" "}
            <code className={code}>balanced</code> og <code className={code}>aggressive</code> (standard). Hva hvert
            nivå gjør, og hvorfor <code className={code}>aggressive</code> er standard, står i{" "}
            <NextLink href="/nav-pilot/forklaring/lokal-modell#utsending" className={linkClass}>
              Hvorfor utsendingen er begrenset
            </NextLink>
            . Har du satt <code className={code}>local_dispatch</code> selv, beholder du verdien din. Vil du slå av alt
            lokalt, bruk <code className={code}>nav-pilot alpha local off</code>.
          </BodyLong>
          <BodyLong>
            Når <code className={code}>local-worker</code> er ferdig, ber nav-pilot hovedagenten bygge prosjektet og
            kjøre testene før den godtar endringen. Har den lokale modellen skrevet en test, skal hovedagenten vise at
            testen kan feile. Feiler bygget eller testene for en ny fil, sendes feilen tilbake til{" "}
            <code className={code}>local-worker</code> én gang. Feiler det igjen, retter hovedagenten den selv.
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
            merket <code className={code}>*</code>. Hva du faktisk får bruke, avhenger også av Copilot-abonnementet
            ditt.
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
            <code className={code}>start</code>. Første oppstart laster modellen inn i minnet, vanligvis på under ett
            minutt.
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
          <LinkableHeading id="modell-64-gb" size="small" level="3">
            Større modell for Macer med 64 GB
          </LinkableHeading>
          <BodyLong>
            Har Macen 64 GB minne eller mer, kan du velge <code className={code}>qwen3.6-35b-a3b-8bit</code>. Det er
            standardmodellen i 8 bit i stedet for 4 bit, og vektene tar 38 GB. Den blir aldri standard, så du må velge
            den selv:
          </BodyLong>
          <CodeBlock compact>
            {`nav-pilot alpha local models    # på mindre maskiner står det «needs 64 GB RAM»
nav-pilot alpha local use qwen3.6-35b-a3b-8bit
nav-pilot alpha local init      # laster ned vektene og starter`}
          </CodeBlock>
          <BodyLong>
            Modellen krever en minnegrense på 48 GB i macOS. <code className={code}>init</code> og{" "}
            <code className={code}>start</code> spør før de hever den. Den har 64k kontekst og 16k svar.
          </BodyLong>
          <BodyLong>
            Hovedagenten sender ingenting til denne modellen ennå, uansett utsendingsnivå. Du kan bruke den til{" "}
            <code className={code}>alpha decide</code>, eller prøve den i en økt selv. I målingene svarte den like godt
            som standardmodellen med decide. Prompter over 49 000 tokens kan sprenge minnegrensen. Se{" "}
            <NextLink href="/innsikt/lokale-modeller#modell-64-gb" className={linkClass}>
              Målinger
            </NextLink>
            .
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
          <LinkableHeading id="decide-oppskrifter" size="medium" level="2">
            Oppskrifter for alpha decide
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot alpha decide</code> stiller den lokale modellen ett flervalgsspørsmål og
            svarer med en sannsynlighet for hvert alternativ. Når serveren er varm, tar et svar under ett sekund, og
            spørsmålet forlater ikke maskinen. Bruk den til vurderinger en regel ikke klarer: om en commit-melding
            følger Conventional Commits, avgjør et regulært uttrykk, men om den forklarer hvorfor, må en modell vurdere.
            Med <code className={code}>--threshold</code> og <code className={code}>--expect</code> blir svaret kode 0
            eller 1. Serveren må kjøre, for <code className={code}>decide</code> starter den ikke selv.
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
            Modellen er mer treffsikker på «ja» enn på «nei», og ordlyden betyr mye. Tallene står i{" "}
            <NextLink href="/innsikt/lokale-modeller#malt-decide" className={linkClass}>
              Målinger
            </NextLink>
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
            En commit-msg-hook som advarer når meldingen bare sier hva diffen viser. Skriptet og hvordan du kobler det
            til git, pre-commit eller Lefthook, står i{" "}
            <NextLink href="/nav-pilot/lokal/decide" className={linkClass}>
              Din første decide-hook
            </NextLink>
            .
          </BodyLong>

          <Label size="small">Mål ditt eget spørsmål</Label>
          <BodyLong>
            Lag en JSONL-fil med eksempler fra repoet ditt der du vet svaret, ett per linje, og omtrent like mange av
            hvert svar. <code className={code}>--eval</code> viser treffsikkerhet, en forvekslingsmatrise, snittet av
            sannsynligheten for riktige og gale svar, og svartid. Er modellen like sikker når den tar feil som når den
            har rett, hjelper ingen terskel, og spørsmålet bør ikke inn i en hook.
          </BodyLong>
          <BodyShort size="small" textColor="subtle">
            Et eksempel på fila står i{" "}
            <NextLink href="/nav-pilot/lokal/decide#mal-sporsmalet" className={linkClass}>
              Din første decide-hook
            </NextLink>
            .
          </BodyShort>
          <CodeBlock compact>{`nav-pilot alpha decide --eval cases.jsonl`}</CodeBlock>

          <Label size="small">Tekst andre har skrevet</Label>
          <BodyLong>
            PR-beskrivelser, issues og logger er skrevet av andre, og kan inneholde instrukser til modellen (prompt
            injection). La sjekker på slik tekst bare advare eller foreslå.
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
            Modellen valgte riktig etikett på 90 % av issuene, og hadde alltid rett når den var minst 90 % sikker.
            Behold filteret: under 0,9 gir kommandoen ingen etikett, og da setter du den selv.
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
            Den finner sjelden beskrivelser der grunnen mangler. Et «no» er verdt å se på. Et «yes» betyr lite.
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
