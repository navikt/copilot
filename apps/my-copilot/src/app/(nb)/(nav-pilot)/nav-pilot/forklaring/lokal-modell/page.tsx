import { BodyLong, BodyShort, Box, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import { TrustedClassesTable } from "@/components/nav-pilot/local-model-tables";
import type { TocItem } from "@/components/table-of-contents";
import { SOURCES } from "@/lib/local-model-results";
import { FALLBACK_TABLE, getLocalModels } from "@/lib/local-models";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Lokal modell",
  description:
    "Hvorfor nav-pilot stopper hovedagenten og ber den delegere arbeid til den lokale modellen, hva delegeringsnivåene gjør, og hva modellen er godkjent for.",
};

const TOC: TocItem[] = [
  { id: "delegering", label: "Hvorfor delegeringen er begrenset" },
  { id: "malte-grenser", label: "Hva den lokale modellen klarer" },
  { id: "hva-kommer", label: "Hva kommer" },
];

type Verdict = { task: string; verdict: string };

const VERDICTS: Verdict[] = [
  { task: "Legge til et argument i 1–2 kall, i flere filer", verdict: "Godkjent for delegering" },
  { task: "Det samme i 3–8 kall", verdict: "Ikke avgjort" },
  { task: "Det samme i 9 kall eller flere", verdict: "Blir i skyen" },
  { task: "Endre én fil", verdict: "Ikke godkjent ennå" },
  { task: "Lage en ny fil", verdict: "Godkjent lokalt på de to letteste trinnene, ikke for delegering" },
  { task: "Svare på spørsmål om kodebasen", verdict: "Blir i skyen" },
  { task: "decide: forklarer commit-meldingen hvorfor?", verdict: "Varsler i commit-hooken" },
  { task: "decide: bug, ønske eller spørsmål?", verdict: "Foreslår etikett" },
];

async function LiveTrustedClasses() {
  const { models } = await getLocalModels();
  return <TrustedClassesTable models={models} />;
}

export default function LokalModellForklaring() {
  return (
    <DocPage
      label="Forklaring"
      title="Lokal modell"
      description="Hovedagenten blir i skyen og bestemmer. Den lokale modellen utfører det den er målt og godkjent for."
      toc={TOC}
    >
      <BodyLong>
        Oppsettet står i{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          Kom i gang med lokal modell på Mac
        </NextLink>
        , og innstillingene i guiden{" "}
        <NextLink href="/nav-pilot/guider/lokal" className={linkClass}>
          Lokal modell
        </NextLink>
        .
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="delegering" size="medium" level="2">
            Hvorfor delegeringen er begrenset
          </LinkableHeading>
          <BodyLong>
            Delegering krever opencode. Copilot CLI velger én leverandør for hele økten, så der kjører økten enten helt
            lokalt eller helt i skyen (
            <a href="https://github.com/github/copilot-cli/issues/4703" className={linkClass}>
              github/copilot-cli#4703
            </a>
            ).
          </BodyLong>
          <BodyLong>
            En instruks om hva hovedagenten burde delegere, holdt ikke: nyere skymodeller gjør jobben selv. Derfor
            stopper nav-pilot hovedagenten når den gjør en stor mekanisk endring selv, og ber den delegere resten til{" "}
            <code className={code}>local-worker</code>. Hvor tidlig den stopper, avhenger av delegeringsnivået:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>off</code>: ingen lokal subagent.
            </li>
            <li>
              <code className={code}>conservative</code>: hovedagenten får bare en instruks om å delegere store
              mekaniske endringer (minst 10 filer eller 20 kall). Ingenting stoppes.
            </li>
            <li>
              <code className={code}>balanced</code>: redigerer hovedagenten selv en femte fil eller et tiende kall i
              samme runde, stopper nav-pilot den én gang. Da ber nav-pilot den delegere resten. Et søk-og-erstatt teller
              hvert sted det endrer.
            </li>
            <li>
              <code className={code}>aggressive</code> (standard): en stoppet fil slipper gjennom først når den er
              delegert til <code className={code}>local-worker</code>. Nye filer går også dit, hvis modellen er godkjent
              for det.
            </li>
          </Bullets>
          <BodyLong>
            Sperren får hovedagenten til å delegere, men sparer ikke AI-kreditter. Både{" "}
            <code className={code}>aggressive</code> og <code className={code}>balanced</code> kostet i de fleste
            målingene mer og tok lengre tid enn å la skymodellen gjøre alt selv. <code className={code}>balanced</code>{" "}
            delegerte nesten ingenting og kostet likevel mer, så <code className={code}>aggressive</code> er standard
            fra 30. september 2026. Tallene står i{" "}
            <NextLink href="/innsikt/lokale-modeller#delegeringsnivaer" className={linkClass}>
              Målinger
            </NextLink>
            .
          </BodyLong>
          <BodyLong>
            Derfor er ingen flere oppgavetyper godkjent for delegering. Siden 8. oktober 2026 må delegering koste høyst
            like mye som skyen alene, regnet samlet over alle målingene for oppgavetypen (
            <a href={SOURCES.costRule} className={linkClass}>
              kostnadsregelen
            </a>
            ). Ingen av de nye målingene klarte det:
          </BodyLong>
          <Bullets>
            <li>
              Delegering til 8-bitsmodellen kostet 1,28 ganger så mye som skyen alene (
              <a href={SOURCES.phaseC} className={linkClass}>
                1. oktober
              </a>
              ). Mekaniske endringer i flere filer besto 13 av 13, men kostet omtrent dobbelt så mye, og den nedre
              grensen på 0,888 nådde ikke kravet på 0,90 (
              <a href={SOURCES.emm8} className={linkClass}>
                30. september
              </a>
              ).
            </li>
            <li>
              Delegering av små oppgaver som bare skriver tester, kostet 1,3–1,5 ganger så mye som skyen. Endringer på
              én linje ble aldri delegert (0 av 32) (
              <a href={SOURCES.smallDelegate} className={linkClass}>
                10. oktober
              </a>
              ).
            </li>
          </Bullets>
          <BodyLong>nav-pilot stopper bare det manifestet har godkjent modellen for. I dag betyr det:</BodyLong>
          <Bullets>
            <li>
              Standardmodellen på Mac er godkjent bare for mekaniske endringer i flere filer, så regelen om nye filer
              slår ikke inn.
            </li>
            <li>
              Lager modellen en ny fil som ikke bygger eller består testene, får den feilen tilbake og ett nytt forsøk
              (retry2, levert i{" "}
              <a href="https://github.com/navikt/copilot/pull/1156" className={linkClass}>
                navikt/copilot#1156
              </a>
              ). Med retry2 ble 37 av 40 nye filer godkjent, mot 26 av 40 uten (
              <a
                href="https://github.com/navikt/mlx-workspace/blob/main/reports/2026-10-10-gap-fill/report.md"
                className={linkClass}
              >
                målingen 10. oktober 2026
              </a>
              ).
            </li>
            <li>Qwen 3.8-modellene og modellen for 64 GB er ikke godkjent for noe. Med dem stoppes ingenting.</li>
            <li>En modell på egen LLM-server er ikke målt. Den får den generelle instruksen, og ingenting stoppes.</li>
          </Bullets>
          <BodyLong>
            nav-pilot stopper heller ingenting når den lokale serveren ikke svarer, i subagentenes egne økter, eller når
            du starter opencode med <code className={code}>--pure</code>.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="malte-grenser" size="medium" level="2">
            Hva den lokale modellen klarer
          </LinkableHeading>
          <BodyLong>
            Den lokale modellen er god til å gjennomføre en beslutning som er tatt, og dårlig til å ta den selv. Om
            delegering lønner seg, avhenger av hvor mange steg skymodellen trenger alene. Trenger den mange, sparer du
            mye. Er oppgaven gjort på to steg, koster delegeringen mer enn den sparer.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack w-full" role="table">
              <HeaderRow stack cells={["Oppgave", "Vurdering"]} />
              <TableBody role="rowgroup">
                {VERDICTS.map((v) => (
                  <TableRow role="row" key={v.task}>
                    <TableDataCell role="cell">{v.task}</TableDataCell>
                    <TableDataCell role="cell">{v.verdict}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyShort size="small" textColor="subtle">
            Resultatene bak hver vurdering står i{" "}
            <NextLink href="/innsikt/lokale-modeller" className={linkClass}>
              Målinger
            </NextLink>
            .
          </BodyShort>
          <BodyLong>
            En oppgavetype blir godkjent først når modellen har holdt kvalitetsgrensen mot skyen over nok kjøringer og
            ulike oppgaver. Manifestet sier hva hver modell er godkjent for:
          </BodyLong>
          {/* Skallet viser reservekopien til manifestet er hentet. */}
          <Suspense fallback={<TrustedClassesTable models={FALLBACK_TABLE.models} />}>
            <LiveTrustedClasses />
          </Suspense>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Sjekk resultatet</Label>
              <BodyLong size="small">
                Den lokale modellen feiler også på måter som kompilerer. Commit eller stash før du setter den i gang, og
                kjør testene etterpå. På store endringer må du regne med å forkaste et forsøk og prøve igjen. Det koster
                tid, ikke AI-kreditter. Ikke la decide stoppe noe ut fra tekst andre har skrevet: grunnlaget kan styre
                svaret.
              </BodyLong>
            </VStack>
          </Box>
          <BodyLong>
            Serveren holder rundt 21 GB minne så lenge den er oppe. Kjør <code className={code}>stop</code> når du ikke
            bruker den.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hva-kommer" size="medium" level="2">
            Hva kommer
          </LinkableHeading>
          <BodyLong>Dette jobber vi med nå. Det er planer, ikke løfter, og noe av det kan bli lagt bort.</BodyLong>
          <Bullets>
            <li>
              Vi måler hva hovedagenten kan delegere til{" "}
              <NextLink href="/nav-pilot/guider/lokal#modell-64-gb" className={linkClass}>
                modellen for Macer med 64 GB
              </NextLink>
              , og hvor mye minne den bruker med lange prompter.
            </li>
            <li>Vi måler delegeringsnivåene videre, på flere modeller og oppgaver.</li>
            <li>
              Vi har ennå ikke målt noen modell på egen LLM-server, verken på Linux eller med Ollama og llama-server.
            </li>
            <li>
              Vi vurderer å kjøre decide på en server for dem som ikke har en passende Mac. Da forlater grunnlaget
              maskinen din.
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
