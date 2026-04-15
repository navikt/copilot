import { Box, VStack, Heading, BodyShort, BodyLong, Tag, HStack } from "@navikt/ds-react";
import NextLink from "next/link";
import { ArrowLeftIcon } from "@navikt/aksel-icons";
import { SurveyCharts } from "./survey-charts";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Utviklerundersøkelsen 2026: Slik bruker Nav AI-kodeverktøy | Min Copilot",
  description:
    "163 utviklere svarte på undersøkelsen om AI-kodeverktøy. 73 % er fornøyde, men 59 % er bekymret for at AI kan svekke dyp forståelse.",
};

export default function SurveyArticlePage() {
  return (
    <main>
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
                <BodyShort size="small" className="text-text-subtle">
                  15. april 2026
                </BodyShort>
              </HStack>
              <Heading size="xlarge" level="1">
                Utviklerundersøkelsen 2026: Slik bruker Nav AI-kodeverktøy
              </Heading>
            </VStack>

            <article className="prose max-w-none">
              <BodyLong spacing>
                I mars 2026 gjennomførte vi en spørreundersøkelse blant utviklere i Nav om erfaringer med
                AI-kodeverktøy. 163 personer svarte over 21 dager. Hovedbildet: stor entusiasme for produktivitetsverdi
                — men også en uro for hva vi mister på veien.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Hvem svarte?
              </Heading>
              <BodyLong spacing>
                Respondentene er erfarne: 57 % har over 11 års erfaring som teknolog, og 27 % har 6–10 år. Kun 5 % har
                0–2 år. Svarene gjenspeiler perspektivet til seniorutviklere.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                93 % bruker AI-kodeverktøy aktivt
              </Heading>
              <BodyLong spacing>
                Bare 12 av 163 respondenter (7 %) oppgir at de ikke bruker AI-kodeverktøy. De som bruker verktøyene,
                bruker i snitt 2,6 stykker — 74 % bruker to eller flere.
              </BodyLong>

              <SurveyCharts section="tools" />

              <BodyLong spacing>
                <strong>Terminalagenter er overraskende populære.</strong> 58 % bruker minst én terminalbasert agent.
                Copilot CLI alene brukes av over halvparten. Merk at noen kan ha forvekslet Claude Code med bruk av
                Claude-modellene via chat.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>
                    «Copilot CLI og plan-modus ble en øyeåpner, og jeg innså at AI faktisk kan bli ekstremt nyttig. Fram
                    til da brukte vi AI mest som erstatning for Google og StackOverflow.»
                  </em>
                </BodyLong>
              </blockquote>

              <Heading size="medium" level="2" spacing>
                Hvor gir AI mest verdi?
              </Heading>
              <BodyLong spacing>
                Respondentene valgte opptil tre områder. At kodeforståelse topper lista, samsvarer godt med at Nav har
                store og komplekse kodebaser. AI som «forklarer hva koden gjør» er verdifullt selv for erfarne
                utviklere.
              </BodyLong>

              <SurveyCharts section="value" />

              <blockquote>
                <BodyLong>
                  <em>
                    «Samlet inn info og genererte dokumentasjon av ca 250 brev som utveksles mellom Nav og institusjoner
                    i EU. Tidligere ansett som en &lsquo;for stor&rsquo; oppgave.»
                  </em>
                </BodyLong>
              </blockquote>

              <Heading size="medium" level="2" spacing>
                73 % er fornøyde — men med nyanser
              </Heading>
              <BodyLong spacing>
                Undersøkelsen stilte syv påstander på en skala fra «helt uenig» til «helt enig». Grønt viser enighet,
                rødt uenighet og grått nøytrale svar:
              </BodyLong>

              <SurveyCharts section="likert" />

              <BodyLong spacing>
                <strong>Produktivitetsverdi oppleves bredt.</strong> Tre av fire mener AI hjelper dem å komme videre og
                fullføre oppgaver raskere.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>
                    «Etter å ha planlagt sammen i et par timer, brukte den 20 minutter på å implementere og treffer
                    blink etter første forsøk. Løsningen ville tatt meg mer enn en uke å kode for hånd.»
                  </em>
                </BodyLong>
              </blockquote>

              <BodyLong spacing>
                <strong>Kodekvalitet er et åpent spørsmål.</strong> Bare 34 % mener AI-generert kode holder god nok
                kvalitet til at den ikke skaper ekstra arbeid i code review — den største enkeltgruppen (43 %) er
                nøytral.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>
                    «AI er veldig god med å skrive kode som ser riktig ut, men det betyr ikke at koden er riktig. Til
                    slutt føler jeg ikke at koden er min kode og da mister jeg eierskap.»
                  </em>
                </BodyLong>
              </blockquote>

              <BodyLong spacing>
                <strong>Bekymringen for kompetansetap er reell.</strong> 59 % er bekymret for at AI kan svekke den dype
                forståelsen av kode og teknologi. Faktisk er 41 % <em>både</em> fornøyde med verktøyene <em>og</em>{" "}
                bekymret for kompetanseeffektene — det er ikke et enten/eller.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>
                    «Føler ofte at code completion påvirker meg til å ta enkleste vei og hindrer meg fra å tenke
                    ordentlig. Veldig positiv opplevelse når man beskriver et problem godt og får akkurat den koden man
                    kunne tenkt seg å skrive.»
                  </em>
                </BodyLong>
              </blockquote>

              <Heading size="medium" level="2" spacing>
                Sikkerhet og personvern er stort sett avklart
              </Heading>
              <BodyLong spacing>
                Halvparten (50 %) er <em>uenige</em> i at personvern eller sikkerhet hindrer dem i å bruke AI-verktøy
                fullt ut. Kun 25 % opplever dette som en barriere — et tegn på at sikkerhetsarbeidet i Nav har hatt
                effekt.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                Én ting å endre
              </Heading>
              <BodyLong spacing>Respondentene valgte det viktigste forbedringsområdet:</BodyLong>

              <SurveyCharts section="change" />

              <BodyLong spacing>
                <strong>Opplæring er det klart viktigste.</strong> Nesten en tredjedel ønsker bedre veiledning i
                effektiv bruk. Nr. 2 — at AI-verktøyene forstår kodebasen og interne rammeverk bedre — handler om det
                samme: å gjøre verktøyene mer nyttige i praksis.
              </BodyLong>

              <Heading size="medium" level="2" spacing>
                De som ikke bruker AI
              </Heading>
              <BodyLong spacing>
                12 respondenter (7 %) oppgir at de ikke bruker AI-kodeverktøy. Halvparten foretrekker å kode uten AI. De
                har i snitt lavere tilfredshet (3,2 vs. 4,0 av 5), og flere uttrykker bekymring for kompetanseeffekter.
                Denne gruppen bør ikke avfeies — de stiller viktige spørsmål om langsiktig kompetanseutvikling og
                teknologimodenhet.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>«Jeg har kastet bort mer tid med å krangle med chatbotten enn det tok å løse problemet selv.»</em>
                </BodyLong>
              </blockquote>

              <Heading size="medium" level="2" spacing>
                Det sammensatte bildet
              </Heading>
              <BodyLong spacing>53 respondenter delte en minneverdig opplevelse.</BodyLong>

              <BodyLong spacing>
                Holdningene i de 53 svarene fordeler seg jevnt: 26 % er overveiende positive, 25 % overveiende negative,
                13 % tydelig blandet, og 36 % nøytralt beskrivende. Det er altså ingen overvekt av entusiasme —
                bekymringer og frustrasjoner er like godt representert.
              </BodyLong>

              <blockquote>
                <BodyLong>
                  <em>
                    «Jeg spinnet opp en AI-agent som stort sett på egenhånd klarte å redusere byggtiden på en repo fra
                    20+ min til 8 min.»
                  </em>
                </BodyLong>
              </blockquote>

              <blockquote>
                <BodyLong>
                  <em>
                    «Junior commitet en Scala-funksjon som var veldig obfuskert. Junior kunne heller ikke forklare hva
                    koden gjorde i etterkant.»
                  </em>
                </BodyLong>
              </blockquote>

              <blockquote>
                <BodyLong>
                  <em>«En ulempe med AI er at det fører til større og flere PR-er fra kolleger.»</em>
                </BodyLong>
              </blockquote>

              <blockquote>
                <BodyLong>
                  <em>«Den gangen Copilot ledet oss til PostMortem…»</em>
                </BodyLong>
              </blockquote>

              <Heading size="medium" level="2" spacing>
                Tre ting å ta med videre
              </Heading>
              <BodyLong spacing>
                AI-kodeverktøy er bredt tatt i bruk i Nav og verdsatt for produktiviteten de gir — men bekymringer om
                kodekvalitet, kompetansetap og eierskap til koden følger med.
              </BodyLong>
              <ol>
                <li>
                  <BodyLong spacing>
                    <strong>Invester i opplæring.</strong> Det er det utviklerne selv ber om mest. Konkrete eksempler på
                    effektiv bruk, ikke generelle presentasjoner.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    <strong>Styrk code review-praksisen.</strong> Når 43 % er nøytrale til om AI-kode holder kvaliteten,
                    trenger vi bedre verktøy og rutiner for å fange opp det AI-en bommer på.
                  </BodyLong>
                </li>
                <li>
                  <BodyLong spacing>
                    <strong>Ta bekymringene på alvor.</strong> 59 % bekymrer seg for kompetansetap. Det er ikke
                    irrasjonelt — det er et signal om at vi trenger bevisste strategier for å sikre at utviklere
                    fortsetter å bygge dyp forståelse.
                  </BodyLong>
                </li>
              </ol>

              <Heading size="medium" level="2" spacing>
                Om undersøkelsen
              </Heading>
              <BodyLong spacing>
                Undersøkelsen bygger på SPACE-rammeverket for utviklerproduktivitet og seksfaktormodellen fra «Beyond
                the Commit» (Chen et al., ICSE-SEIP 2026). Vi lot kollegaer fagfellevurdere designet før utsending, og
                kortet ned fra 16 til 12 spørsmål for å senke terskelen. Syv Likert-påstander dekker fem av seks
                faktorer fra seksfaktormodellen, i tillegg til SPACE-dimensjonen tilfredshet.
              </BodyLong>
              <BodyLong spacing>
                Undersøkelsen ble gjennomført av Audun Fachald Strand (Plattform &amp; Infra), Kjetil Åmdal-Sævik
                (Seksjonsleder, Innsikt og KI), Ole-Alexander Moy (Teknologileder Velferd) og Hans Kristian Flaatten,
                med hjelp fra Viggo Tellefsen Wivestad (SINTEF).
              </BodyLong>
              <BodyLong spacing>
                Undersøkelsen ble distribuert via Slack og annonsert på et felles allmøte. Den var åpen i 21 dager
                (mars–april 2026) som en anonym Microsoft Forms-undersøkelse. Det tok i snitt 7 minutter og 48 sekunder
                å svare.
              </BodyLong>
              <BodyLong spacing>
                163 av over 500 Copilot-brukere svarte — en svarprosent på ca. 32 %. Det er en god respons for en
                frivillig undersøkelse, men to tredjedeler svarte altså ikke. Vi distribuerte via Slack og allmøte, noe
                som kan gi skjevhet mot utviklere som er mest aktive i disse kanalene.
              </BodyLong>
              <BodyLong spacing>
                Én ting om dataene: Noen respondenter kan ha forvekslet Claude Code (Anthropics terminalagent) med bruk
                av Claude-modellene (Sonnet/Opus) via chat. De 22 som oppgir Claude Code kan derfor være noe høyere enn
                reelt.
              </BodyLong>
              <BodyLong spacing>
                Som med alle undersøkelser basert på selvseleksjon, bør vi lese resultatene som et nyttig bilde av
                holdningene blant engasjerte utviklere — ikke nødvendigvis representative for alle 500+ brukere.
              </BodyLong>
            </article>

            <Box paddingBlock="space-8" className="print-hidden">
              <NextLink
                href="/"
                className="inline-flex items-center gap-1.5 text-sm no-underline hover:underline"
              >
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
