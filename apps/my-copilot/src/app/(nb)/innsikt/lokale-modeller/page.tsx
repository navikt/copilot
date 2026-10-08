import { BodyLong, BodyShort, Box, HGrid, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { LinkableHeading } from "@/components/linkable-heading";
import MetricCard from "@/components/metric-card";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import { LocalModelsTable, ResultTable, TrustedClassesTable } from "@/components/nav-pilot/local-model-tables";
import { PageHero } from "@/components/page-hero";
import {
  DECIDE_RESULTS,
  DELEGATION_MEASURED,
  GB64_MEASURED,
  MEASURED_MODEL,
  SOURCES,
  WORKER_RESULTS,
  formatDate,
} from "@/lib/local-model-results";
import { FALLBACK_TABLE, MANIFEST_URL, getLocalModels, type LocalModel } from "@/lib/local-models";

// "AI" stays in the AI credit wording: AI credits is GitHub's name for the billing unit. Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Lokale modeller",
  description:
    "Hvilke KI-modeller nav-pilot kan kjøre på egen Mac, hva de er godkjent for, og målingene bak vurderingene.",
};

type Table = { models: LocalModel[]; fetchedAt: string | null };

const modelName = (m: LocalModel) => m.model.split("/").pop();

function Source({ measured, source }: { measured: string; source: string }) {
  return (
    <BodyShort size="small" textColor="subtle">
      Målt {formatDate(measured)}.{" "}
      <a href={source} className={linkClass}>
        Se rapporten
      </a>
      .
    </BodyShort>
  );
}

function Section({ id, title, children }: { id: string; title: string; children: React.ReactNode }) {
  return (
    <section>
      <VStack gap="space-16">
        <LinkableHeading id={id} size="medium" level="2">
          {title}
        </LinkableHeading>
        {children}
      </VStack>
    </section>
  );
}

// Everything that reads the manifest. Rendered with the checked-in copy while the live one loads.
function ManifestSections({ table }: { table: Table }) {
  const { models } = table;
  const byRam = [...new Set(models.map((m) => m.min_ram_gb))].sort((a, b) => a - b);
  const live = modelName(models.find((m) => m.default) ?? models[0]);
  return (
    <>
      <Section id="modeller-per-minne" title="Valgte modeller per minne">
        <BodyLong>
          Hvilken modell du kan kjøre, avhenger av hvor mye minne Macen har. Standardmodellen er den nav-pilot velger
          hvis du ikke velger selv. Kontekst er hvor mye tekst modellen kan lese på én gang, målt i tokens.
        </BodyLong>
        {byRam.map((gb) => (
          <VStack gap="space-8" key={gb}>
            <Label as="h3">{gb === byRam[0] ? `Fra ${gb} GB minne` : `Fra ${gb} GB minne, i tillegg`}</Label>
            <LocalModelsTable stack models={models.filter((m) => m.min_ram_gb === gb)} />
          </VStack>
        ))}
      </Section>

      <Section id="godkjent-for" title="Hva hver modell er godkjent for">
        <BodyLong>
          Arbeidet er delt i oppgavetyper. For hver type avgjør målingene om modellen kan få oppgaven som subagent,
          sendt fra hovedagenten i skyen (delegering), eller om hele økten kan kjøres lokalt. Alt som ikke er godkjent,
          blir i skyen.
        </BodyLong>
        <TrustedClassesTable stack models={models} />
      </Section>

      {live !== MEASURED_MODEL && (
        <BodyShort size="small" textColor="subtle">
          Standardmodellen i dag er <code className={code}>{live}</code>. Resultatene under gjelder{" "}
          <code className={code}>{MEASURED_MODEL}</code> og er ikke målt på nytt ennå.
        </BodyShort>
      )}
    </>
  );
}

async function LiveManifestSections() {
  return <ManifestSections table={await getLocalModels()} />;
}

async function ManifestTime() {
  const { fetchedAt } = await getLocalModels();
  return fetchedAt ? (
    <>
      Modelloversikten er hentet fra{" "}
      <a href={MANIFEST_URL} className={linkClass}>
        manifestet
      </a>{" "}
      {formatDate(fetchedAt)}, og hentes på nytt hver time.
    </>
  ) : (
    <>
      Manifestet kunne ikke hentes nå, så modelloversikten viser en lagret kopi av{" "}
      <a href={MANIFEST_URL} className={linkClass}>
        manifestet
      </a>
      .
    </>
  );
}

export default function LokaleModeller() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Lokale modeller"
        description="nav-pilot kan kjøre en KI-modell på din egen Mac. Her ser du hvilke modeller som er valgt, hva de er godkjent for, og målingene bak."
      />
      <Box
        paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
        paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        className="max-w-5xl mx-auto"
      >
        <VStack gap="space-40">
          <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
            <MetricCard
              value="10 av 10"
              label="Mekaniske endringer"
              helpTitle="Mekaniske endringer over flere filer"
              helpText="Legge til et påkrevd argument i 1–2 kall, i flere filer. Skymodellen klarte 7 av 10."
              subtitle="skymodellen: 7 av 10"
            />
            <MetricCard
              value="17 av 17"
              label="Delegering besto testene"
              helpTitle="Delegering med aggressive"
              helpText="Med aggressive delegerte hovedagenten i alle 17 gyldige kjøringer, og alle besto bygg og tester."
              subtitle="med aggressive"
            />
            <MetricCard
              value="93 %"
              label="decide: commit-meldinger"
              helpTitle="Forklarer commit-meldingen hvorfor?"
              helpText="89 av 96 riktige svar."
              subtitle="89 av 96 riktige"
            />
            <MetricCard
              value="0,4 s"
              label="Svartid i decide"
              helpTitle="Svartid per kall"
              helpText="Median svartid per kall på en server som allerede kjører."
              subtitle="median per kall"
            />
          </HGrid>

          <BodyLong>
            Hovedagenten er modellen i skyen som leder arbeidet. Den kan gi avgrensede oppgaver til en lokal modell som
            subagent. Hva den får gi bort, avgjøres av målinger, ikke av hva modellen selv mener. Vil du ta i bruk en
            lokal modell, les{" "}
            <NextLink href="/nav-pilot/forklaring/lokal-modell" className={linkClass}>
              Lokal modell
            </NextLink>
            .
          </BodyLong>

          <Suspense fallback={<ManifestSections table={{ ...FALLBACK_TABLE, fetchedAt: null }} />}>
            <LiveManifestSections />
          </Suspense>

          <Section id="delegeringsnivaer" title="Delegeringsnivåene">
            <BodyLong>
              Med bare en instruks om hva den burde delegere, delegerte Sonnet 5 arbeid til den lokale modellen i 1 av
              29 testkjøringer, mot 23 av 24 for Sonnet 4.6. Derfor bestemmer delegeringsnivået i nav-pilot hva
              hovedagenten skal delegere.
            </BodyLong>
            <BodyLong>Målingen fra september 2026, med Sonnet 5 som hovedagent:</BodyLong>
            <Bullets>
              <li>
                Med <code className={code}>aggressive</code> delegerte hovedagenten i alle 17 gyldige kjøringer med
                mange kallsteder eller nye filer, og alle 17 besto bygg og tester. Med{" "}
                <code className={code}>balanced</code> delegerte den i 2 av 20.
              </li>
              <li>Var endringen liten, delegerte den ingenting (0 av 5).</li>
              <li>
                Det kostet 0,83–2,1 ganger så mye i AI-kreditter og tok 2,7–3,6 ganger så lang tid som når skymodellen
                gjorde alt selv.
              </li>
              <li>
                Hovedagenten gjorde likevel om 15 av 27 oppgaver med nye filer selv. To kjøringer ble avbrutt, og i én
                av dem ble koden liggende i stykker.
              </li>
            </Bullets>
            <BodyLong>
              Kravet til <code className={code}>balanced</code> var like mange beståtte oppgaver og ikke dyrere enn
              skymodellen alene. I en ny måling 29. september besto alle oppgavene, men{" "}
              <code className={code}>balanced</code> kostet 1,57 og 1,49 ganger så mye på to store oppgaver, og hver
              kjøring var dyrere enn hver kontrollkjøring. Derfor ble <code className={code}>aggressive</code> standard
              30. september 2026.
            </BodyLong>
            <BodyLong>
              Et nytt forsøk lønner seg for nye filer. Når hovedagenten sendte bygge- eller testfeilen tilbake én gang,
              ble 15 av 20 nye filer godkjent, mot 5 av 20 uten. Tiden per godkjent fil gikk ned fra 618 til 322
              sekunder.
            </BodyLong>
            <Source {...DELEGATION_MEASURED} />
          </Section>

          <Section id="malt-delegering" title="Resultater for kodeoppgaver">
            <BodyLong>
              Hver oppgave er løst flere ganger, og hver løsning er sjekket av en test. «10 av 10» betyr at testen besto
              i alle ti forsøkene.
            </BodyLong>
            <ResultTable rows={WORKER_RESULTS.rows} />
            <BodyLong>
              Tiden varierer fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På
              store mekaniske endringer kan den lokale modellen være raskere.
            </BodyLong>
            <BodyShort size="small" textColor="subtle">
              Målt {formatDate("2026-09-25")} og {formatDate(WORKER_RESULTS.measured)}. Se{" "}
              <a href={SOURCES.night1} className={linkClass}>
                kvalitetsnatt 1
              </a>{" "}
              og{" "}
              <a href={SOURCES.night2} className={linkClass}>
                kvalitetsnatt 2
              </a>
              .
            </BodyShort>
          </Section>

          <Section id="malt-decide" title="Resultater for decide">
            <BodyLong>
              <code className={code}>decide</code> stiller den lokale modellen et ja/nei-spørsmål om en tekst, for
              eksempel om en commit-melding forklarer hvorfor endringen ble gjort.
            </BodyLong>
            <ResultTable rows={DECIDE_RESULTS.rows} />
            <BodyLong>
              Hvordan du spør, betyr noe. Standardmodellen svarte riktig på 85 % av spørsmålene i opprinnelig form og 58
              % når de var snudd (
              <a href={SOURCES.layout} className={linkClass}>
                måling
              </a>
              ). «Ja»-svarene er stabile, men «nei»-svarene vipper mot «teksten er grei».
            </BodyLong>
            <Box background="warning-soft" padding="space-16" borderRadius="8">
              <VStack gap="space-8">
                <Label size="small">Begrensninger</Label>
                <Bullets>
                  <li>
                    Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte
                    standardmodellen det svaret i 9 av 27 tilfeller. Ikke la decide stoppe noe ut fra tekst andre har
                    skrevet.
                  </li>
                  <li>
                    decide-spørsmålene er målt på få repoer. Mål ditt eget spørsmål med{" "}
                    <code className={code}>--eval</code> før du bygger på det.
                  </li>
                  <li>Med Egen LLM-server, også på Linux, er ingenting målt.</li>
                </Bullets>
              </VStack>
            </Box>
            <BodyShort size="small" textColor="subtle">
              Målt {formatDate(DECIDE_RESULTS.measured)}. Svartid per kall på varm server: median 0,4 sekunder. Se{" "}
              <a href={SOURCES.why} className={linkClass}>
                commit-spørsmålet
              </a>
              ,{" "}
              <a href={SOURCES.sets} className={linkClass}>
                issue og PR
              </a>{" "}
              og{" "}
              <a href={SOURCES.systemOne} className={linkClass}>
                decide-rapporten
              </a>
              .
            </BodyShort>
          </Section>

          <Section id="modell-64-gb" title="Modellen for 64 GB">
            <BodyLong>
              <code className={code}>qwen3.6-35b-a3b-8bit</code> er standardmodellen i 8 bit, for Macer med minst 64 GB
              minne.
            </BodyLong>
            <Bullets>
              <li>Den løste oppgaven i 12 av 12 Copilot-økter.</li>
              <li>
                Med decide svarte den like godt som standardmodellen: 184 av 218 riktige mot 182, og 91 av 96 mot 89 på
                commit-spørsmålet. På testene av svakheter fikk den 806 av 974 mot 827, mest fordi den var svakere når
                grunnlaget prøvde å styre svaret eller var langt.
              </li>
              <li>
                Minnebruken var på det meste 46,18 GB med en prompt på 49 000 tokens. Grensen er 48 GB. Ved 64 000
                tokens anslår rapporten rundt 50 GB, altså over grensen. Det er ikke målt ennå.
              </li>
            </Bullets>
            <Source {...GB64_MEASURED} />
          </Section>

          {/* TODO(PR B): «Forkastede modeller», from the manifest's `replaced` field once local-models-manifest.ts maps it. */}

          <Section id="metode" title="Maskinvare og metode">
            <Bullets>
              <li>
                Alt er målt på én maskin, en MacBook Pro med M5 Max, i et kontrollert testoppsett, ikke i daglig bruk.
              </li>
              <li>Kodeoppgavene er hentet fra et Kotlin-repo i Nav.</li>
              <li>En løsning teller bare når en test bekrefter at den virker.</li>
              <li>
                Andeler oppgis med Wilson-intervall, som viser hvor mye tallet kan bomme når det bygger på få forsøk.
              </li>
            </Bullets>
            <BodyLong>
              Rapportene ligger i navikt/mlx-workspace:{" "}
              <a href={SOURCES.readme} className={linkClass}>
                oversikt over rapportene
              </a>
              ,{" "}
              <a href={SOURCES.template} className={linkClass}>
                malen hver rapport følger
              </a>{" "}
              og{" "}
              <a href={SOURCES.unmeasured} className={linkClass}>
                det som ikke er målt ennå
              </a>
              .
            </BodyLong>
          </Section>

          <Section id="sist-oppdatert" title="Sist oppdatert">
            <BodyLong>
              <Suspense fallback="Modelloversikten hentes fra manifestet i navikt/mlx-workspace.">
                <ManifestTime />
              </Suspense>
            </BodyLong>
            <Bullets>
              <li>Delegeringsnivåene: {formatDate(DELEGATION_MEASURED.measured)}</li>
              <li>Kodeoppgaver: {formatDate(WORKER_RESULTS.measured)}</li>
              <li>decide: {formatDate(DECIDE_RESULTS.measured)}</li>
              <li>Modellen for 64 GB: {formatDate(GB64_MEASURED.measured)}</li>
            </Bullets>
          </Section>
        </VStack>
      </Box>
    </main>
  );
}
