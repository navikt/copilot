import { Box, VStack, Heading, BodyShort, BodyLong, Tag, HStack, Table } from "@navikt/ds-react";
import NextLink from "next/link";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import type { Metadata } from "next";

// Utkast: ikke i nyhetslista eller sitemap før docs/news/articles-fila mister draft: true.
export const metadata: Metadata = {
  title: "Utviklerundersøkelsen høsten 2026: Fornøyd, men bekymret",
  description:
    "211 svarte på høstens undersøkelse om KI-kodeverktøy. Flere er fornøyde enn i vår, og flere er bekymret for at KI kan svekke egen dyp forståelse.",
  robots: { index: false, follow: false },
};

const statements: { text: string; n: string; spring: string; autumn: string; note?: string }[] = [
  { text: "Jeg er fornøyd med KI-kodeverktøyene", n: "KI-brukere", spring: "3,99", autumn: "4,17" },
  {
    text: "Jeg er bekymret for at KI kan svekke min egen dype forståelse",
    n: "KI-brukere",
    spring: "3,52",
    autumn: "3,87",
  },
  { text: "KI hjelper meg videre når jeg står fast", n: "KI-brukere", spring: "4,05", autumn: "4,05" },
  { text: "KI reduserer mental innsats på repetitive oppgaver", n: "KI-brukere", spring: "3,94", autumn: "3,84" },
  {
    text: "KI-generert kode holder god nok kvalitet til at den ikke gir ekstra arbeid i review",
    n: "KI-brukere",
    spring: "3,27",
    autumn: "3,26",
  },
  {
    text: "Jeg stoler på at en KI-agent løser oppgaver i flere steg uten at jeg følger med",
    n: "KI-brukere",
    spring: "Ny",
    autumn: "2,44",
  },
  {
    text: "Jeg bruker verktøyene mindre fordi jeg er usikker på hva de koster",
    n: "KI-brukere",
    spring: "Ny",
    autumn: "2,11",
  },
  { text: "Jeg er trygg på å ta fullt ansvar for koden jeg leverer", n: "Alle", spring: "3,6", autumn: "3,7" },
  { text: "Personvern eller sikkerhet hindrer meg i å bruke KI fullt ut", n: "Alle", spring: "2,65", autumn: "2,44" },
];

const tools: { name: string; spring: string; autumn: string }[] = [
  { name: "Copilot CLI", spring: "53 %", autumn: "65 %" },
  { name: "Copilot i IntelliJ", spring: "55 %", autumn: "36 %" },
  { name: "Copilot i VS Code", spring: "33 %", autumn: "26 %" },
  { name: "OpenCode", spring: "5 %", autumn: "21 %" },
];

const changes: { what: string; share: string }[] = [
  { what: "Opplæring", share: "24 %" },
  { what: "Ingenting, jeg er fornøyd", share: "17 %" },
  { what: "Retningslinjer", share: "12 %" },
  { what: "At verktøyene forstår kodebasen bedre", share: "11 %" },
  { what: "Færre begrensninger", share: "10 %" },
  { what: "Jeg foretrekker å jobbe uten KI", share: "8 %" },
];

function Quote({ children }: { children: string }) {
  return (
    <blockquote>
      <BodyLong>
        <em>«{children}»</em>
      </BodyLong>
    </blockquote>
  );
}

export default function SurveyAutumnArticlePage() {
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
                <Tag size="small" variant="success">
                  Nav
                </Tag>
                <Tag size="small" variant="warning">
                  Utkast
                </Tag>
              </HStack>
              <Heading size="xlarge" level="1">
                Utviklerundersøkelsen høsten 2026: Fornøyd, men bekymret
              </Heading>
            </VStack>

            <article className="prose max-w-none">
              <BodyLong spacing>
                Fra 13. til 27. oktober 2026 svarte 211 personer på utviklerundersøkelsen om KI-kodeverktøy i Nav. I vår
                svarte 165. Flere er fornøyde med verktøyene enn i vår. Samtidig er flere bekymret for at KI kan svekke
                deres egen dype forståelse av koden. Det er den eneste endringen vi kan si sikkert noe om.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Hvem svarte
              </Heading>
              <BodyLong spacing>
                Vi inviterte omtrent 1000 teknologer, utviklere og designere som kan få Copilot-lisens. Svarprosenten
                var omtrent 21 % i høst og omtrent 16–17 % i vår. De aller fleste som svarte, bruker KI-kodeverktøy: 203
                av 211 i høst og 153 av 165 i vår.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Mer fornøyd, og mer bekymret
              </Heading>
              <BodyLong spacing>
                Vi stilte de samme påstandene som i vår, på en skala fra 1 (helt uenig) til 5 (helt enig). Tabellen
                viser gjennomsnittet. To påstander er nye i høst.
              </BodyLong>

              <Table size="small">
                <caption>Gjennomsnitt per påstand, vår og høst 2026 (1 = helt uenig, 5 = helt enig)</caption>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">Påstand</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Hvem svarte</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Vår</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Høst</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {statements.map((s) => (
                    <Table.Row key={s.text}>
                      <Table.HeaderCell scope="row">{s.text}</Table.HeaderCell>
                      <Table.DataCell>{s.n}</Table.DataCell>
                      <Table.DataCell>{s.spring}</Table.DataCell>
                      <Table.DataCell>{s.autumn}</Table.DataCell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>

              <BodyLong spacing>
                <strong>Tilfredsheten er høy.</strong> 83 % av KI-brukerne er enige i at de er fornøyde, og snittet gikk
                opp fra 3,99 til 4,17. Mange sier også at KI hjelper dem videre når de står fast.
              </BodyLong>

              <BodyLong spacing>
                <strong>Bekymringen har økt.</strong> 70 % er enige i at de er bekymret for at KI kan svekke deres egen
                dype forståelse, 12 prosentpoeng flere enn i vår. Dette er den eneste endringen som står seg når vi
                korrigerer for at vi sammenligner mange påstander samtidig.
              </BodyLong>

              <Quote>
                Da jeg begynte å bruke KI til å lage features, merket jeg at jeg fort ble rusten. Jeg kunne prompte meg
                frem til kode som fungerte, uten å kunne forklare hvordan den virket.
              </Quote>

              <Quote>
                Når kollegaene spurte meg hvorfor jeg hadde tatt valgene i en PR, kunne jeg ikke svare dem. Mesteparten
                av koden var KI-generert, og jeg hadde glemt den.
              </Quote>

              <BodyLong spacing>
                <strong>Kvaliteten står på stedet hvil.</strong> Spørsmålet om KI-kode holder god nok kvalitet til at
                den ikke gir ekstra arbeid i review, ligger rett over midten, akkurat som i vår.
              </BodyLong>

              <BodyLong spacing>
                <strong>Få lar agenten jobbe alene.</strong> Bare 19 % stoler på at en KI-agent løser oppgaver i flere
                steg uten at de følger med. Få sier at usikkerhet om kostnad gjør at de bruker verktøyene mindre.
              </BodyLong>

              <Quote>
                Den la seg flat da jeg oppdaget feilen, men det er ikke lett når den er så skråsikker og som oftest gjør
                ting helt riktig. Da blir det for lett å stole på den.
              </Quote>

              <BodyLong spacing>
                <strong>Personvern og sikkerhet.</strong> Litt færre opplever at personvern eller sikkerhet hindrer dem,
                men forskjellen er så liten at den kan skyldes tilfeldigheter.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Flere bruker terminalen
              </Heading>
              <BodyLong spacing>
                Andelen som bruker Copilot CLI og OpenCode har økt, mens færre oppgir Copilot i IntelliJ og VS Code.
                Mange bruker begge deler: 80 av de som svarte i høst bruker både en editor og et terminalverktøy.
              </BodyLong>

              <Table size="small">
                <caption>Andel av alle som svarte som bruker verktøyet</caption>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">Verktøy</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Vår</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Høst</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {tools.map((t) => (
                    <Table.Row key={t.name}>
                      <Table.HeaderCell scope="row">{t.name}</Table.HeaderCell>
                      <Table.DataCell>{t.spring}</Table.DataCell>
                      <Table.DataCell>{t.autumn}</Table.DataCell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>

              <Heading size="medium" level="2" spacing>
                Hvor KI gir mest verdi
              </Heading>
              <BodyLong spacing>
                Å forklare kode er fortsatt på topp (54 %), fulgt av feilsøking (51 %, opp 8 prosentpoeng) og
                kodegenerering (42 %). Færre nevner å lære nye språk, 8 % mot 14 % i vår.
              </BodyLong>

              <Quote>
                Positiv opplevelse med agentisk feilsøking av en prodsak: den leste logger, fant rotårsaken i koden og
                foreslo en fiks.
              </Quote>

              <Quote>
                Siden sommeren har vi skrevet en arvtaker til en tjeneste, refaktorert store mengder kode og redusert
                teknisk gjeld betraktelig.
              </Quote>

              <Heading size="medium" level="2" spacing>
                Én ting å endre
              </Heading>
              <BodyLong spacing>
                Vi spurte hva de ville endret hvis de bare kunne endre én ting. Opplæring er igjen det vanligste svaret.
              </BodyLong>

              <Table size="small">
                <caption>Én ting å endre, høsten 2026 (andel av alle 211 som svarte)</caption>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell scope="col">Ønske</Table.HeaderCell>
                    <Table.HeaderCell scope="col">Andel</Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {changes.map((c) => (
                    <Table.Row key={c.what}>
                      <Table.HeaderCell scope="row">{c.what}</Table.HeaderCell>
                      <Table.DataCell>{c.share}</Table.DataCell>
                    </Table.Row>
                  ))}
                </Table.Body>
              </Table>

              <Heading size="medium" level="2" spacing>
                Hva vi prøver videre
              </Heading>
              <BodyLong spacing>Dette er forsøk. Vi vet ikke ennå om de virker.</BodyLong>
              <ul>
                <li>
                  <BodyLong>Samle det du må vite på ett sted, med mindre tekst.</BodyLong>
                </li>
                <li>
                  <BodyLong>Lage veiledning om eierskap til KI-generert kode og om review.</BodyLong>
                </li>
                <li>
                  <BodyLong>Gjøre oppsettet av verktøyene enklere.</BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    Utvide neste undersøkelse med svaralternativet «vet ikke» og spørsmål om rolle, hvor ofte du bruker
                    verktøyene og hvilke klienter du bruker.
                  </BodyLong>
                </li>
              </ul>

              <Heading size="medium" level="2" spacing>
                Om undersøkelsen
              </Heading>
              <BodyLong spacing>
                Vår- og høstrunden er to separate utvalg. Svarene er ikke koblet til person, så vi kan ikke følge den
                enkelte over tid. I vår brukte vi Microsoft Forms. I høst svarte folk på nettsiden eller i terminalen,
                med de samme spørsmålene og den samme skalaen. Vi vurderer risikoen for at byttet har påvirket svarene
                som liten. Påstandene om tillit til agenter og om kostnad er nye i høst. Alle tall gjelder dem som
                svarte.
              </BodyLong>
              <BodyLong spacing>
                For å se om endringene er mer enn tilfeldigheter, brukte vi Mann–Whitney-test og Holm-korreksjon for
                mange sammenligninger. Bare økningen i bekymring for egen forståelse står seg (p ≈ 0,003, korrigert p ≈
                0,02). Sitatene er korte utdrag fra fritekstsvar, redigert og avidentifisert.
              </BodyLong>
              <BodyLong spacing>
                Artikkelen er skrevet av teamet som også bygger og forvalter KI-verktøyene i Nav, og er lest av [ekstern
                leser – fylles inn] før publisering.
              </BodyLong>
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
