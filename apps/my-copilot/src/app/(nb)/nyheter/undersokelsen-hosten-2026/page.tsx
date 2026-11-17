import { Box, VStack, Heading, BodyLong, Tag, HStack } from "@navikt/ds-react";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import NextLink from "next/link";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Utviklerundersøkelsen høsten 2026: Fornøyd, men bekymret",
  description:
    "211 svarte på høstens undersøkelse om KI-kodeverktøy. De fleste er fornøyde, og flere enn i vår er bekymret for at KI kan svekke egen dype forståelse.",
};

const statements: { text: string; n: string; spring: string; autumn: string; note?: string }[] = [
  { text: "Jeg er fornøyd med KI-kodeverktøyene", n: "KI-brukere", spring: "3,99", autumn: "4,17" },
  {
    text: "Jeg er bekymret for at KI kan svekke min egen dype forståelse",
    n: "KI-brukere",
    spring: "3,52",
    autumn: "3,87",
  },
  {
    text: "KI hjelper meg videre når jeg står fast og med å fullføre oppgaver raskere",
    n: "KI-brukere",
    spring: "4,05",
    autumn: "4,05",
  },
  { text: "KI reduserer mental innsats på repetitive oppgaver", n: "KI-brukere", spring: "3,94", autumn: "3,84" },
  {
    text: "Kode laget med KI-hjelp holder god nok kvalitet til at den ikke gir ekstra arbeid i review",
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
  {
    text: "Jeg er trygg på å ta fullt ansvar for kode som er generert eller vesentlig hjulpet av KI",
    n: "Alle",
    spring: "3,62",
    autumn: "3,71",
  },
  {
    text: "Usikkerhet rundt personvern eller sikkerhetsregler hindrer meg i å bruke KI fullt ut",
    n: "Alle",
    spring: "2,65",
    autumn: "2,44",
  },
];

const tools: { name: string; spring: string; autumn: string }[] = [
  { name: "Copilot CLI", spring: "53 %", autumn: "65 %" },
  { name: "Copilot i IntelliJ", spring: "55 %", autumn: "36 %" },
  { name: "Copilot i VS Code", spring: "33 %", autumn: "26 %" },
  { name: "OpenCode", spring: "5 %", autumn: "21 %" },
];

const changes: { what: string; share: string }[] = [
  { what: "Mer opplæring og veiledning i effektiv bruk", share: "24 %" },
  { what: "Ingenting, jeg er fornøyd med dagens situasjon", share: "17 %" },
  { what: "Tydeligere retningslinjer for sikkerhet og personvern", share: "12 %" },
  { what: "Bedre forståelse av kodebasen vår og interne rammeverk", share: "11 %" },
  { what: "Færre begrensninger på tilgang til verktøy", share: "10 %" },
  { what: "Annet", share: "10 %" },
  { what: "Støtte for flere KI-verktøy eller miljøer", share: "9 %" },
  { what: "Jeg foretrekker å kode uten KI", share: "8 %" },
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
              </HStack>
              <Heading size="xlarge" level="1">
                Utviklerundersøkelsen høsten 2026: Fornøyd, men bekymret
              </Heading>
            </VStack>

            <article className="prose max-w-none">
              <BodyLong spacing>
                Fra 13. til 27. oktober 2026 svarte 211 personer på utviklerundersøkelsen om KI-kodeverktøy i Nav. I vår
                svarte 165. De fleste er fornøyde med verktøyene. Samtidig er flere enn i vår bekymret for at KI kan
                svekke deres egen dype forståelse av koden. Det er den eneste endringen i påstandene som står seg i
                testen.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Hvem svarte
              </Heading>
              <BodyLong spacing>
                Vi inviterte omtrent 1000 teknologer, utviklere og designere som kan få Copilot-lisens. 211 svar
                tilsvarer omtrent 30 % av de 712 som brukte Copilot i høstperioden. I vår var det 165 av 584, omtrent
                28 %. De aller fleste som svarte, bruker KI-kodeverktøy: 203 av 211 i høst og 153 av 165 i vår.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Fornøyd, men mer bekymret
              </Heading>
              <BodyLong spacing>
                Vi stilte de samme påstandene som i vår, på en skala fra 1 (helt uenig) til 5 (helt enig). Tabellen
                viser gjennomsnittet. To påstander er nye i høst.
              </BodyLong>

              <Table size="small">
                <caption>Gjennomsnitt per påstand, vår og høst 2026 (1 = helt uenig, 5 = helt enig)</caption>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Påstand</TableHeaderCell>
                    <TableHeaderCell scope="col">Hvem svarte</TableHeaderCell>
                    <TableHeaderCell scope="col">Vår</TableHeaderCell>
                    <TableHeaderCell scope="col">Høst</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {statements.map((s) => (
                    <TableRow key={s.text}>
                      <TableHeaderCell scope="row">{s.text}</TableHeaderCell>
                      <TableDataCell>{s.n}</TableDataCell>
                      <TableDataCell>{s.spring}</TableDataCell>
                      <TableDataCell>{s.autumn}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <BodyLong spacing>
                <strong>Tilfredsheten er høy.</strong> 83 % av KI-brukerne er fornøyde. Økningen fra i vår er usikker.
                77 % er enige i at KI hjelper dem videre når de står fast og med å fullføre oppgaver raskere, og 69 %
                opplever mindre mental innsats på repetitive oppgaver. Dette er deres egne vurderinger, ikke målt
                tidsbesparelse.
              </BodyLong>

              <BodyLong spacing>
                <strong>Bekymringen har økt.</strong> 70 % er enige i at de er bekymret for at KI kan svekke deres egen
                dype forståelse, 12 prosentpoeng flere enn i vår. Dette er den eneste endringen i påstandene som står
                seg i testen.
              </BodyLong>

              <Quote>
                Da jeg begynte å bruke KI til å lage funksjoner, merket jeg at jeg fort ble rusten. Jeg kunne prompte
                meg frem til kode som fungerte, uten å kunne forklare hvordan den virket.
              </Quote>

              <BodyLong spacing>
                <strong>Omtrent samme gjennomsnitt for kvalitet.</strong> Påstanden gjelder om KI-hjulpet kode er god
                nok til å unngå ekstra arbeid i review. Dette er deltakernes vurdering, ikke målt kodekvalitet eller
                samlet reviewarbeid.
              </BodyLong>

              <BodyLong spacing>
                <strong>Få stoler på agenter uten tilsyn.</strong> Bare 20 % stoler på at en KI-agent løser oppgaver i
                flere steg uten at de følger med. Få sier at usikkerhet om kostnad gjør at de bruker verktøyene mindre.
              </BodyLong>

              <Quote>
                Det er ikke lett [å se feilen] når den er så skråsikker og som oftest gjør ting helt riktig. Da blir det
                for lett å stole på den.
              </Quote>

              <BodyLong spacing>
                <strong>Personvern og sikkerhet.</strong> Litt færre opplever at usikkerhet rundt personvern eller
                sikkerhetsregler hindrer dem, men nedgangen er usikker.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Større andel bruker terminalen
              </Heading>
              <BodyLong spacing>
                Andelen som bruker Copilot CLI og OpenCode, har økt, mens færre oppgir Copilot i IntelliJ og VS Code.
                Mange bruker begge deler: 80 av dem som svarte i høst, bruker både en editor og et terminalverktøy.
              </BodyLong>

              <Table size="small">
                <caption>Andel av alle som svarte som bruker verktøyet</caption>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Verktøy</TableHeaderCell>
                    <TableHeaderCell scope="col">Vår</TableHeaderCell>
                    <TableHeaderCell scope="col">Høst</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tools.map((t) => (
                    <TableRow key={t.name}>
                      <TableHeaderCell scope="row">{t.name}</TableHeaderCell>
                      <TableDataCell>{t.spring}</TableDataCell>
                      <TableDataCell>{t.autumn}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <Heading size="medium" level="2" spacing>
                Hvor KI-brukerne opplever mest verdi
              </Heading>
              <BodyLong spacing>
                Blant KI-brukerne er de vanligste valgene å forklare eller forstå eksisterende kode (54 %), feilsøking
                (51 %) og kodegenerering (42 %). De kunne velge opptil tre områder.
              </BodyLong>

              <Heading size="small" level="3" spacing>
                Arbeid som ellers kunne blitt liggende
              </Heading>
              <BodyLong spacing>
                I friteksten beskriver en respondent opprydding i eksisterende kode samtidig med videreutvikling.
                Vedkommende vurderer at arbeidet neppe ville blitt prioritert i samme omfang uten KI. En annen forteller
                om å bytte testoppsett i en jobb det ellers var vanskelig å finne motivasjon til.
              </BodyLong>
              <BodyLong spacing>
                Å få gjort utsatt arbeid er en annen mulig verdi enn å fullføre samme oppgave raskere. En tredje
                beskriver hjelp til feilsøking med logger og kode, der KI foreslo en feilårsak og en fiks.
              </BodyLong>
              <BodyLong spacing>
                Dette er redigerte gjengivelser av enkeltsvar, ikke målte gevinster. Svarene viser ikke hvor vanlige
                erfaringene er eller hvor mye tid som ble spart.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Én ting å endre
              </Heading>
              <BodyLong spacing>
                Vi spurte hva deltakerne ville endre hvis de bare kunne endre én ting. Opplæring er igjen det vanligste
                svaret.
              </BodyLong>

              <Table size="small">
                <caption>Én ting å endre, høsten 2026 (andel av alle 211 som svarte)</caption>
                <TableHeader>
                  <TableRow>
                    <TableHeaderCell scope="col">Ønske</TableHeaderCell>
                    <TableHeaderCell scope="col">Andel</TableHeaderCell>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {changes.map((c) => (
                    <TableRow key={c.what}>
                      <TableHeaderCell scope="row">{c.what}</TableHeaderCell>
                      <TableDataCell>{c.share}</TableDataCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>

              <Heading size="medium" level="2" spacing>
                Hva vi har gjort siden i vår
              </Heading>
              <BodyLong spacing>Dette har vi gjort med det dere ba om. Vi har ikke målt om det virker.</BodyLong>
              <ul>
                <li>
                  <BodyLong>
                    <strong>Opplæring og veiledning:</strong> nye guider for{" "}
                    <NextLink href="/nav-pilot/guider/kom-i-gang">å komme i gang</NextLink>, for{" "}
                    <NextLink href="/nav-pilot/guider/cplt-flere-repoer">å jobbe i flere repoer</NextLink> og for{" "}
                    <NextLink href="/nav-pilot/guider/mcp-gjennom-cplt">MCP-servere</NextLink>, og en forklaring av{" "}
                    <NextLink href="/nav-pilot/forklaring/sandkassen">sandkassen</NextLink>.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Det du må vite:</strong> <NextLink href="/ma-vite">én side med det viktigste</NextLink>, og
                    veiledning om <NextLink href="/praksis/eierskap">eierskap til KI-generert kode og review</NextLink>.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Enklere oppsett:</strong> nav-pilot har fått en kortere første kjøring, agentpakken samlet
                    på ett sted og tydeligere feilmeldinger når noe stopper.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Sikkerhet og personvern:</strong> de samme sandkassereglene gjelder nå for alle agenter.
                    Agenten @nav-pilot skal stoppe og spørre før den legger inn nye felt med personopplysninger, og i
                    OpenCode spør nav-pilot før et repos egne tillegg kjøres.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Kvalitet og egen forståelse:</strong> vi skrev om{" "}
                    <NextLink href="/nyheter/kvalitet-foran-fart">kvalitet foran fart</NextLink> og om å bruke KI på
                    vedlikehold.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    <strong>Innsikt i bruk og kostnad:</strong>{" "}
                    <NextLink href="/innsikt/bruk">Bruk og kostnad</NextLink> (krever innlogging) viser nå bruk per
                    klient (Copilot CLI, OpenCode, VS Code og andre) for hele Nav.
                  </BodyLong>
                </li>
              </ul>

              <Heading size="medium" level="2" spacing>
                Hva som kommer
              </Heading>
              <ul>
                <li>
                  <BodyLong>
                    Agentpakken i Copilot-appen og VS Code, uten terminal. Flere ba om bedre støtte for appen.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    En{" "}
                    <NextLink href="/verktoy/fleet-gateway">felles inngang til MCP-servere med Nav-innlogging</NextLink>{" "}
                    er i pilot, og etter hvert kan team registrere egne MCP-servere selv. Det gir agentene bedre tilgang
                    til kodebasen og fagsystemene våre.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    Tydeligere veiledning om hvilken klient du bør velge, og om lokal kontra skybasert kjøring.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong>
                    Resultatene fra undersøkelsene, samlet bak innlogging på ki-utvikling.nav.no, så du kan følge
                    utviklingen selv.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    Neste undersøkelse får svaralternativet «vet ikke» og spørsmål om rolle, hvor ofte du bruker
                    verktøyene og hvilke klienter du bruker.
                  </BodyLong>
                </li>
              </ul>

              <Heading size="medium" level="2" spacing>
                Om undersøkelsen
              </Heading>
              <BodyLong spacing>
                Vår- og høstrunden er to separate, selvrekrutterte utvalg. Svarene er ikke koblet til person, så vi vet
                ikke om de samme personene svarte. Resultatene gjelder dem som svarte, ikke nødvendigvis alle i Nav. Vi
                målte opplevd nytte og bekymring, ikke faktisk produktivitet eller kompetansetap.
              </BodyLong>
              <BodyLong spacing>
                I vår brukte vi Microsoft Forms. I høst svarte folk på nettsiden eller i terminalen, med de samme
                spørsmålene og den samme skalaen. Vi vurderer risikoen for at byttet har påvirket svarene som liten.
                Påstandene om tillit til agenter og om kostnad er nye i høst. Svarandelen regner vi av dem som brukte
                Copilot i perioden. Vårrunden var åpen i 38 dager og høstrunden i 15, og jo lengre perioden er, jo flere
                teller som brukere. Ikke alle som svarte, bruker Copilot, så andelen er ikke en ren svarprosent.
                Vårtallene er regnet på nytt blant KI-brukerne, så de avviker litt fra{" "}
                <NextLink href="/nyheter/utviklerundersokelsen-2026">vårartikkelen</NextLink>, som regnet blant alle som
                svarte.
              </BodyLong>
              <BodyLong spacing>
                For å se om endringene er mer enn tilfeldigheter, brukte vi Mann–Whitney-test og Holm-korreksjon for
                mange sammenligninger. Bare økningen i bekymring for egen forståelse står seg (p ≈ 0,003, korrigert p ≈
                0,02). Sitatene er korte, redigerte og avidentifiserte utdrag. Eksemplene fra arbeidshverdagen er
                parafraser av enkeltsvar.
              </BodyLong>
              <BodyLong spacing>
                Artikkelen er skrevet av teamet som også bygger og forvalter KI-verktøyene i Nav, og er lest av noen
                utenfor teamet før publisering.
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
