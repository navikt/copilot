import { Box, VStack, Heading, BodyLong, Tag, HStack, Link } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import NextLink from "next/link";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Modellvalget i november 2026: ingen agent byttet modell",
  description:
    "Vi testet nye og billigere modeller på Navs egne oppgaver. Ingen agent byttet modell, men MAI Code 2 er et rimelig valg når du velger modell selv til vanlig koding.",
  robots: { index: false, follow: false },
};

const REPO = "https://github.com/navikt/copilot";
const DOCS = `${REPO}/blob/main/docs`;
const BASELINES = `${DOCS}/golden-baselines`;

const outcomes: { question: string; outcome: string; why: string }[] = [
  {
    question: "Standardmodell for @nav-pilot",
    outcome: "GPT-6.1 Sol beholdes",
    why: "GPT-6.2 Sol var 16 % billigere, men i alle fem kjøringene av én sikkerhetstest la den ikke igjen stubbene for koden brukeren skulle skrive selv. Også GPT-6.1 Sol hadde svakheter: én reell bom på en annen sikkerhetstest, og 3 av 5 på en vanlig test. Utfallet ble derfor formelt «ikke avgjort», og da beholdes dagens modell.",
  },
  {
    question: "Kodegjennomgang med @code-review",
    outcome: "Claude Opus 5.5 beholdes",
    why: "GPT-6.2 Astra besto to av sjekkene i bare 1 og 0 av 5 kjøringer, mot 4 av 5 for Opus. Den kostet også over dobbelt så mye.",
  },
  {
    question: "Universell utforming, Aksel og norsk tekst",
    outcome: "Claude Sonnet 5.5 beholdes. Claude Opus 5.5 er ikke godkjent som reserve",
    why: "Sonnet besto 25 av 25. Opus bommet på samme oppgave i alle fem kjøringene: den laget en egen listeboks i stedet for å spørre, og avvek dermed fra Aksel.",
  },
  {
    question: "Rimelig modell til koding",
    outcome: "MAI Code 2 anbefales når du velger modell selv",
    why: "Den besto alle tre kodeoppgavene i 5 av 5 kjøringer, som GPT-6.1 Sol, til en sjettedel av prisen.",
  },
  {
    question: "Billige roller: @research, @rust og @kafka",
    outcome: "GPT-6 Luna og Claude Haiku 5.5 beholdes",
    why: "MAI Code 2 Flash var billigere og løste oppgavene for @research og @rust, men besto ikke sikkerhetstestene i planleggingsoppgavene. På @kafka besto den én oppgave i bare 4 av 5 kjøringer.",
  },
];

export default function ModelChoiceNovemberArticlePage() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <div className="max-w-3xl" style={{ marginInline: "auto" }}>
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32" }}
        >
          <VStack gap="space-16">
            <NextLink
              href="/"
              className="inline-flex items-center gap-1.5 text-sm text-text-subtle no-underline hover:underline print-hidden"
            >
              <ArrowLeftIcon aria-hidden fontSize="1rem" />
              Nyheter
            </NextLink>

            <VStack gap="space-8">
              <HStack gap="space-4" align="center">
                <Tag size="small" variant="info">
                  nav-pilot
                </Tag>
                <Tag size="small" variant="warning">
                  Utkast
                </Tag>
              </HStack>
              <Heading size="xlarge" level="1">
                Modellvalget i november 2026: ingen agent byttet modell
              </Heading>
            </VStack>

            <article className="prose max-w-none">
              <BodyLong spacing>
                De fleste agentene i nav-pilot har en fast modell. I november testet vi om nyere eller billigere
                modeller burde ta over noen av rollene. Ingen agent byttet modell. MAI Code 2 er likevel et godt og
                rimelig valg når du selv velger modell til vanlig koding.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Hvorfor vi måler selv
              </Heading>
              <BodyLong spacing>
                Eksterne rangeringer er et nyttig utgangspunkt, men de svarer ikke på spørsmålet vårt. Artificial
                Analysis anslo 18. november 2026 at GPT-6.2 Sol var 14 % dyrere enn GPT-6.1 Sol per oppgave på
                innsatsnivået low, men billigere på høyeste innsatsnivå. På våre oppgaver var den 16 % billigere. På
                Terminal-Bench 4.1 fikk GPT-6.2 Astra 62,1 % og GPT-6.2 Sol 61,5 %, men usikkerhetsintervallene
                overlapper, så tavla skiller dem ikke. For MAI-modellene fant vi bare leverandørens egne tall.
              </BodyLong>
              <BodyLong spacing>
                Hva en oppgave koster, avhenger av innsatsnivået og av hva slags oppgave det er. Derfor er det Navs egne
                oppgaver som avgjør.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Slik målte vi
              </Heading>
              <ul>
                <li>
                  <BodyLong>
                    <strong>Planen kom først.</strong> Før vi kjørte noe, låste vi spørsmålene, testene og kravene for å
                    bytte modell. Det kalles forhåndsregistrering, og det hindrer oss i å justere reglene etter at vi
                    har sett resultatet.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Fem kjøringer per oppgave.</strong> Hver modell løste hver oppgave fem ganger, 18. og 19.
                    november, med samme versjon av Copilot CLI.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Sikkerhetstestene er en port.</strong> Én reell bom stopper et bytte, uansett hvor godt
                    modellen gjør det ellers. Et grensetilfelle teller som bom. Flere av testene gjelder rød sone: kode
                    som brukeren skal skrive og forstå selv, som kjernelogikk og sikkerhet. Der skal agenten ikke skrive
                    koden, men lage stubber, altså tomme funksjoner merket TODO, som brukeren fyller ut.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Hver bom leses enkeltvis.</strong> Lesingen ble gjort av KI-modeller, ikke av mennesker. For
                    standardmodellen ble hver bom lest av to ulike modeller: KI-agenten som kjørte målingen, og en
                    uavhengig andre modell. Var de uenige, gjaldt den strengeste vurderingen.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    <strong>Språkmodell som dommer der enkle tekstsjekker sviktet.</strong> Noen sjekker lette etter
                    faste ord og mønstre i svaret og avviste riktige svar, fordi de ikke skiller et funn fra en
                    negasjon. Disse sjekkene flyttes til en språkmodell som dommer, prøvd mot et kontrollsett med kjent
                    fasit.
                  </BodyLong>
                </li>
              </ul>

              <Heading size="medium" level="2" spacing>
                Dette ble resultatet
              </Heading>

              <Table size="small">
                <caption>Utfall av modellvalget i november 2026</caption>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Spørsmål</TableHeaderCell>
                    <TableHeaderCell scope="col">Utfall</TableHeaderCell>
                    <TableHeaderCell scope="col">Hvorfor</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {outcomes.map((o) => (
                    <TableRow key={o.question}>
                      <TableHeaderCell scope="row">{o.question}</TableHeaderCell>
                      <TableDataCell>{o.outcome}</TableDataCell>
                      <TableDataCell>{o.why}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <Heading size="small" level="3" spacing>
                Billig og god på oppgavene, men ikke på sikkerhet
              </Heading>
              <BodyLong spacing>
                MAI Code 2 Flash løste oppgavene for @research og @rust like godt som modellene de har i dag, og var
                billigere i alle kjøringene. Den fikk derfor en egen beslutningsmåling 20. november. Der kostet den 2,87
                AI Credits per kjøring i median på de samme planleggingsoppgavene som GPT-6.1 Sol, mot 116,6 for Sol.
                Men den bommet på flere av sikkerhetstestene, blant annet ved å skrive kode i rød sone som brukeren
                skulle skrive selv. Etter planen stopper én reell bom på en sikkerhetstest begge rollene.
              </BodyLong>
              <BodyLong spacing>
                Etter den første målingen anbefalte vi MAI Code 2 Flash som et utforskende valg for @worker. Den
                anbefalingen er trukket, fordi Flash bommet på sikkerhetstestene i beslutningsmålingen.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Hva det betyr for deg
              </Heading>
              <BodyLong spacing>Du trenger ikke endre noe. Ingen agent har fått ny modell.</BodyLong>
              <ul>
                <li>
                  <BodyLong>@nav-pilot bruker fortsatt GPT-6.1 Sol.</BodyLong>
                </li>
                <li>
                  <BodyLong>Til kodegjennomgang bruker du @code-review med Claude Opus 5.5.</BodyLong>
                </li>
                <li>
                  <BodyLong>
                    Til universell utforming, Aksel og norsk tekst bruker du @accessibility, @aksel og @forfatter med
                    Claude Sonnet 5.5. De har ingen målt reserve.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    Velger du modell selv til vanlig koding, er MAI Code 2 et rimelig valg. Den er ikke målt på
                    planlegging eller på sikkerhetstestene. Vårt råd, som ikke bygger på målingen: bruk standardmodellen
                    til sikkerhetskritisk kode.
                  </BodyLong>
                </li>
              </ul>
              <BodyLong spacing>
                Priser og anbefalinger for alle modellene står på <NextLink href="/modeller">modellsiden</NextLink>.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Begrensninger
              </Heading>
              <ul>
                <li>
                  <BodyLong>
                    <strong>Fem kjøringer avdekker bare store forskjeller.</strong> En modell som består 5 av 5, kan i
                    praksis lykkes bare litt over halvparten av gangene. At to modeller begge består, viser ikke at de
                    er like gode.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Testene alene bytter aldri modell.</strong> Et bytte krever en egen beslutningsmåling og
                    trinnvis utrulling etter <Link href={`${DOCS}/pin-protokoll.md`}>pin-protokollen</Link>.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Lesingen var ikke blind.</strong> Begge leserne så modellnavnet.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    <strong>Dommeren har kjente svakheter.</strong> Vi har dokumentert dem i{" "}
                    <Link href={`${REPO}/issues/2482`}>#2482</Link>.
                  </BodyLong>
                </li>
              </ul>

              <Heading size="medium" level="2" spacing>
                Les mer
              </Heading>
              <ul>
                <li>
                  <BodyLong>
                    <Link href={`${BASELINES}/2026-11-modellvalg-resultat.md`}>Resultatrapporten</Link> med alle tall,
                    avvik fra planen og kostnad
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <Link href={`${BASELINES}/2026-11-modellvalg-plan.md`}>Den forhåndsregistrerte planen</Link>
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <Link href={`${BASELINES}/2026-11-modellvalg-regnskap.md`}>Regnskapet</Link> og{" "}
                    <Link href={`${REPO}/pull/2397`}>rådataene</Link>
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    Beslutningsmålingen for MAI Code 2 Flash:{" "}
                    <Link href={`${BASELINES}/2026-12-mai-flash-plan.md`}>planen</Link> og{" "}
                    <Link href={`${REPO}/pull/2518`}>resultatet</Link>
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    <Link href={`${DOCS}/modellvalg.md`}>Modellvalg for alle agentene</Link> med begrunnelser
                  </BodyLong>
                </li>
              </ul>
              <BodyLong spacing>Artikkelen er skrevet av teamet som bygger og forvalter KI-verktøyene i Nav.</BodyLong>
            </article>

            <Box paddingBlock="space-8" className="print-hidden">
              <NextLink href="/" className="inline-flex items-center gap-1.5 text-sm no-underline hover:underline">
                <ArrowLeftIcon aria-hidden fontSize="1rem" />
                Alle nyheter
              </NextLink>
            </Box>
          </VStack>
        </Box>
      </div>
    </main>
  );
}
