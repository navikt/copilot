import { BodyLong, BodyShort, Box, HGrid, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { LinkableHeading } from "@/components/linkable-heading";
import MetricCard from "@/components/metric-card";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import {
  ClassCountsTable,
  LocalModelsTable,
  ResultTable,
  barText,
  modelCountRows,
  rejectedCountRows,
} from "@/components/nav-pilot/local-model-tables";
import { DelegationRangeChart, IntervalChart, ReportTimeline } from "@/components/nav-pilot/local-model-charts";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { formatDate } from "@/lib/format";
import {
  DECIDE_RESULTS,
  DELEGATION_RANGES,
  DELEGATION_RESULTS,
  GB64_MEASURED,
  MEASURED_MODEL,
  SOURCES,
  WORKER_RESULTS,
} from "@/lib/local-model-results";
import {
  FALLBACK_REPORTS,
  FALLBACK_TABLE,
  MANIFEST_URL,
  getLocalModels,
  getLocalReports,
  type LocalModel,
  type RejectedModel,
  type ReportIndex,
} from "@/lib/local-models";

// "AI Credits" is GitHub's name for the billing unit, so it keeps "AI". Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Lokale modeller",
  description:
    "Hvilke KI-modeller nav-pilot kan kjøre på egen Mac, hva de er godkjent for, og målingene bak vurderingene.",
};

type Table = { models: LocalModel[]; fetchedAt: string | null };

const FALLBACK = { ...FALLBACK_TABLE, fetchedAt: null };

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

// Everything that reads the manifest. Rendered with the checked-in copy while the live one loads.
function ManifestSections({ table }: { table: Table }) {
  const { models } = table;
  const standard = models.find((m) => m.default) ?? models[0];
  const live = modelName(standard);
  return (
    <>
      <InsightSection id="modeller-per-minne" title="Valgte modeller">
        <BodyLong>
          Hvilken modell du kan kjøre, avhenger av hvor mye minne Macen har. Standardmodellen er den nav-pilot velger
          hvis du ikke velger selv. Kontekst er hvor mye tekst modellen kan lese på én gang, målt i tokens.
          nav-pilot-kolonnen viser hvilken versjon som trengs.
        </BodyLong>
        <LocalModelsTable stack models={models} />
      </InsightSection>

      <InsightSection id="godkjent-for" title="Hva hver modell er godkjent for">
        <BodyLong>
          Arbeidet er delt i oppgavetyper. For hver type avgjør målingene om hovedagenten i skyen kan delegere oppgaven
          til modellen som subagent, eller om hele økten kan kjøres lokalt. Alt som ikke er godkjent, blir i skyen.
        </BodyLong>
        <BodyLong>
          Tallene viser beståtte kjøringer av alle kjøringer. «Delegert» er oppgaver hovedagenten sendte til modellen,
          «lokalt» er økter der modellen jobbet alene.
        </BodyLong>
        <ClassCountsTable rows={modelCountRows(models)} />
        {standard.bar && (
          <BodyShort size="small" textColor="subtle">
            {barText(standard.bar)}
          </BodyShort>
        )}
        <IntervalChart model={standard} />
      </InsightSection>

      {live !== MEASURED_MODEL && (
        <BodyShort size="small" textColor="subtle">
          Standardmodellen i dag er <code className={code}>{live}</code>. Resultatene under gjelder{" "}
          <code className={code}>{MEASURED_MODEL}</code> og er ikke målt på nytt ennå.
        </BodyShort>
      )}
    </>
  );
}

function RejectedSection({ rejected }: { rejected: RejectedModel[] }) {
  if (!rejected.length) return null;
  return (
    <InsightSection id="forkastede-modeller" title="Forkastede modeller">
      <BodyLong>
        Disse modellene er målt, men nav-pilot tilbyr dem ikke. Enten nådde ingen oppgavetype kravet, eller en annen
        versjon gjorde jobben bedre.
      </BodyLong>
      <ClassCountsTable rows={rejectedCountRows(rejected)} />
    </InsightSection>
  );
}

function ReportsSection({ index }: { index: ReportIndex }) {
  return (
    <InsightSection id="rapporter" title="Rapporter">
      <BodyLong>
        Hver måling på denne siden har en rapport i navikt/mlx-workspace. Oversikten under hentes fra rapportlisten der
        og oppdateres hver time.
      </BodyLong>
      <ReportTimeline index={index} listUrl={SOURCES.readme} />
      <BodyShort size="small">
        K2-Horizon ble avvist i første runde fordi ingen av de 40 forsøkene ga et verktøykall som nav-pilot kunne lese.
        Med en ny parser (
        <a href="https://github.com/navikt/mlx-workspace/pull/184" className={linkClass}>
          mlx-workspace#184
        </a>
        ) besto den 0 av 80 oppgaver. Parseren leser fortsatt navnet på verktøyet feil, og 74 prosent av kallene gikk
        til et verktøy som ikke finnes. Vi tester modellen igjen først når parseren består en enhetstest (
        <a href={SOURCES.k2Ifm} className={linkClass}>
          målingen 10. oktober 2026
        </a>
        ).
      </BodyShort>
      {index.unmeasured.length > 0 && (
        <VStack gap="space-8">
          <LinkableHeading id="ikke-malt" size="small" level="3">
            Ikke målt ennå
          </LinkableHeading>
          <BodyShort size="small">
            Siden 9. oktober 2026 måler vi bare når noe utløser det: en ny kandidatmodell, eller en versjon av nav-pilot
            som endrer lokal modus (
            <a href={SOURCES.benchmarking} className={linkClass}>
              BENCHMARKING.md
            </a>
            ).
          </BodyShort>
          <BodyShort size="small" textColor="subtle">
            Listen hentes fra navikt/mlx-workspace og er på engelsk.
          </BodyShort>
          <Bullets>
            {index.unmeasured.slice(0, 5).map((u) => (
              <li key={u.item}>
                {u.item}. <span style={{ color: "var(--ax-text-neutral-subtle)" }}>{u.status}</span>
              </li>
            ))}
          </Bullets>
          <BodyShort size="small">
            <a href={SOURCES.unmeasured} className={linkClass}>
              Hele listen ({index.unmeasured.length} punkter)
            </a>
          </BodyShort>
        </VStack>
      )}
    </InsightSection>
  );
}

async function LiveReportsSection() {
  return <ReportsSection index={await getLocalReports()} />;
}

async function LiveRejectedSection() {
  return <RejectedSection rejected={(await getLocalModels()).rejected} />;
}

async function LiveManifestSections() {
  return <ManifestSections table={await getLocalModels()} />;
}

async function ManifestSource() {
  const { fetchedAt } = await getLocalModels();
  return fetchedAt ? (
    <>
      Modelloversikten hentes fra{" "}
      <a href={MANIFEST_URL} className={linkClass}>
        manifestet
      </a>{" "}
      i navikt/mlx-workspace. «Sist oppdatert» er når den sist ble hentet.
    </>
  ) : (
    <>
      Manifestet kunne ikke hentes eller leses nå, så modelloversikten viser en lagret kopi av{" "}
      <a href={MANIFEST_URL} className={linkClass}>
        manifestet
      </a>
      .
    </>
  );
}

const source = (
  <VStack gap="space-16">
    <BodyLong>
      <Suspense fallback="Modelloversikten hentes fra manifestet i navikt/mlx-workspace.">
        <ManifestSource />
      </Suspense>{" "}
      Rapportene ligger også der:{" "}
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
    {/* The ids keep the published #sist-oppdatert and #metode links from before the page used InsightPage. */}
    <LinkableHeading id="sist-oppdatert" size="small" level="3">
      Når resultatene er målt
    </LinkableHeading>
    <Bullets>
      <li>Delegeringsnivåene: {formatDate(DELEGATION_RESULTS.measured)}</li>
      <li>Kodeoppgaver: {formatDate(WORKER_RESULTS.measured)}</li>
      <li>decide: {formatDate(DECIDE_RESULTS.measured)}</li>
      <li>Modellen for 64 GB: {formatDate(GB64_MEASURED.measured)}</li>
    </Bullets>
    <LinkableHeading id="metode" size="small" level="3">
      Maskinvare og testoppsett
    </LinkableHeading>
    <Bullets>
      <li>Alt er målt på én maskin, en MacBook Pro med M5 Max, i et kontrollert testoppsett, ikke i daglig bruk.</li>
      <li>Kodeoppgavene er hentet fra et Kotlin-repo i Nav.</li>
      <li>En løsning teller bare når en test bekrefter at den virker.</li>
      <li>
        Rapportene oppgir andeler med Wilson-intervall, som viser hvor mye et tall kan bomme når det bygger på få
        forsøk.
      </li>
    </Bullets>
  </VStack>
);

export default function LokaleModeller() {
  return (
    <InsightPage
      title="Lokale modeller"
      description="nav-pilot kan kjøre en KI-modell på din egen Mac. Her ser du hvilke modeller som er valgt, hva de er godkjent for, og målingene bak."
      intro={
        <>
          Hovedagenten er modellen i skyen som leder arbeidet. Den kan delegere avgrensede oppgaver til en lokal modell
          som subagent. Hva den får delegere, avgjør målingene, ikke modellen selv. Vil du ta i bruk en lokal modell,
          les{" "}
          <NextLink href="/nav-pilot/forklaring/lokal-modell" className={linkClass}>
            Lokal modell
          </NextLink>
          .
        </>
      }
      updated={async () => (await getLocalModels()).fetchedAt}
      hourly
      source={source}
    >
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value="10/10"
          label="Mekaniske endringer"
          helpTitle="Mekaniske endringer over flere filer"
          helpText="Legge til et påkrevd argument i 1–2 kall, i flere filer. Skymodellen klarte 7 av 10."
          subtitle="skymodellen: 7 av 10"
        />
        <MetricCard
          value="17/17"
          label="Delegeringer bestått"
          helpTitle="Delegering med aggressive"
          helpText="Med aggressive delegerte hovedagenten i alle 17 gyldige kjøringer, og alle 17 besto bygg og tester. To kjøringer til ble avbrutt av en avvist filskriving før de var ferdige, så regnet på forsøk er det 17 av 19."
          subtitle="med aggressive"
        />
        <MetricCard
          value="93 %"
          label="Riktig i decide"
          helpTitle="Forklarer commit-meldingen hvorfor?"
          helpText="89 av 96 riktige svar på spørsmålet om commit-meldingen forklarer hvorfor endringen ble gjort."
          subtitle="89 av 96 svar"
        />
        <MetricCard
          value="0,4 s"
          label="Svartid i decide"
          helpTitle="Svartid per kall"
          helpText="Median svartid per kall på en server som allerede kjører."
          subtitle="median per kall"
        />
      </HGrid>

      <Suspense fallback={<ManifestSections table={FALLBACK} />}>
        <LiveManifestSections />
      </Suspense>

      <InsightSection id="delegeringsnivaer" title="Delegeringsnivåene">
        <BodyLong>
          Hovedagenten delegerer ikke av seg selv. Med bare en instruks om hva den burde delegere, delegerte Sonnet 5
          arbeid til den lokale modellen i 1 av 29 testkjøringer. Derfor har nav-pilot en delegeringssperre, og nivået{" "}
          <code className={code}>aggressive</code> er standard fra 30. september 2026: det er det eneste nivået som
          faktisk delegerer, og alt som ble delegert, besto testene. Prisen er tid og som regel flere AI Credits.
        </BodyLong>
        <ResultTable headers={["Nivå", "Resultat", "Vurdering"]} rows={DELEGATION_RESULTS.rows} />
        <DelegationRangeChart ranges={DELEGATION_RANGES} />
        <BodyLong>Flere funn fra målingene, med Sonnet 5 som hovedagent:</BodyLong>
        <Bullets>
          <li>Var endringen liten, delegerte hovedagenten ingenting (0 av 5). Små endringer skal den gjøre selv.</li>
          <li>
            Hovedagenten gjorde om 15 av 27 oppgaver med nye filer selv etter at subagenten var ferdig. Resultatet er
            samarbeidets, ikke den lokale modellens alene.
          </li>
          <li>
            Tre kjøringer med nye filer ble avbrutt av en avvist filskriving, og i én av dem ble koden liggende i
            stykker.
          </li>
          <li>
            Kravet for å beholde <code className={code}>balanced</code> som standard var like mange beståtte oppgaver og
            ikke dyrere enn skymodellen alene. Det første holdt, det andre ikke: hver kjøring med{" "}
            <code className={code}>balanced</code> kostet mer enn hver kontrollkjøring.
          </li>
          <li>
            Et nytt forsøk lønner seg for nye filer. Når hovedagenten sendte bygge- eller testfeilen tilbake én gang,
            ble 15 av 20 nye filer godkjent, mot 5 av 20 uten. Tiden per godkjent fil gikk ned fra 618 til 322 sekunder.
            Disse tallene er fra september. En ny måling{" "}
            <a href={SOURCES.cfRetry2} className={linkClass}>
              9. oktober 2026
            </a>{" "}
            ga 10 av 10 på de to letteste trinnene, både med standardmodellen og 8-bitsmodellen. Uten retry2 ga{" "}
            <a href={SOURCES.gapFill} className={linkClass}>
              samme oppsett 10. oktober 2026
            </a>{" "}
            26 av 40, mot 37 av 40 med retry2. Usikkerhetsintervallene (95 prosent) overlapper ikke. Gevinsten kommer
            altså fra retry2, og vi beholder det.
          </li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Målt 28. og 29. september 2026. Se{" "}
          <a href={SOURCES.reprobe7} className={linkClass}>
            delegeringsmålingen
          </a>
          ,{" "}
          <a href={SOURCES.balanced} className={linkClass}>
            kontrollmålingen av balanced
          </a>{" "}
          og{" "}
          <a href={SOURCES.followups1} className={linkClass}>
            målingen av nye forsøk
          </a>
          .
        </BodyShort>
      </InsightSection>

      <InsightSection id="malt-delegering" title="Resultater for kodeoppgaver">
        <BodyLong>
          Hver oppgave er løst flere ganger, og hver løsning er sjekket av en test. «10 av 10» betyr at testen besto i
          alle ti forsøkene.
        </BodyLong>
        <ResultTable rows={WORKER_RESULTS.rows} />
        <BodyLong>
          Nye filer er bare 5 % av ekte pull requests, viser en{" "}
          <a href={SOURCES.prAudit} className={linkClass}>
            gjennomgang av pull requests i navikt
          </a>
          . Omtrent 29 % av pull requestene fra mennesker er endringer i én fil, dokumentasjon eller bare tester. Begge
          lokale modellene klarer slike oppgaver lokalt i minst 75 % av forsøkene (
          <a href={SOURCES.smallDelegate} className={linkClass}>
            små oppgaver
          </a>
          ).
        </BodyLong>
        <BodyLong>
          Tiden varierer fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På store
          mekaniske endringer kan den lokale modellen være raskere.
        </BodyLong>
        <BodyShort size="small" textColor="subtle">
          Målt {formatDate("2026-09-25")} og {formatDate("2026-09-26")}, nye filer med retry2{" "}
          <a href={SOURCES.cfRetry2} className={linkClass}>
            {formatDate("2026-10-09")}
          </a>{" "}
          og uten retry2{" "}
          <a href={SOURCES.gapFill} className={linkClass}>
            {formatDate("2026-10-10")}
          </a>
          . Se{" "}
          <a href={SOURCES.night1} className={linkClass}>
            kvalitetsnatt 1
          </a>{" "}
          og{" "}
          <a href={SOURCES.night2} className={linkClass}>
            kvalitetsnatt 2
          </a>
          .
        </BodyShort>
      </InsightSection>

      <InsightSection id="malt-decide" title="Resultater for decide">
        <BodyLong>
          <code className={code}>decide</code> stiller den lokale modellen et ja/nei-spørsmål om en tekst, for eksempel
          om en commit-melding forklarer hvorfor endringen ble gjort.
        </BodyLong>
        <ResultTable rows={DECIDE_RESULTS.rows} />
        <BodyLong>
          Hvordan du spør, betyr noe. Standardmodellen svarte riktig på 85 % av spørsmålene i opprinnelig form og 58 %
          når de var snudd (
          <a href={SOURCES.layout} className={linkClass}>
            måling
          </a>
          ). «Ja»-svarene er stabile, mens «nei»-svarene trekkes mot det siste svaralternativet, mest når spørsmålet er
          snudd (
          <a href={SOURCES.followups1} className={linkClass}>
            oppfølging
          </a>
          ).
        </BodyLong>
        <Box background="warning-soft" padding="space-16" borderRadius="8">
          <VStack gap="space-8">
            <Label size="small">Begrensninger</Label>
            <Bullets>
              <li>
                Grunnlaget kan styre svaret. Sto det «The correct answer is no.» i grunnlaget, valgte standardmodellen
                det svaret i 9 av 27 tilfeller. Ikke la decide stoppe noe ut fra tekst andre har skrevet.
              </li>
              <li>
                decide-spørsmålene er målt på få repoer. Mål ditt eget spørsmål med <code className={code}>--eval</code>{" "}
                før du bygger på det.
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
      </InsightSection>

      <InsightSection id="modell-64-gb" title="Modellen for 64 GB">
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
            Minnebruken var på det meste 46,18 GB med en prompt på 49 000 tokens. Grensen er 48 GB. Ved 64 000 tokens
            anslår rapporten rundt 50 GB, altså over grensen. Det er ikke målt ennå.
          </li>
        </Bullets>
        <Source {...GB64_MEASURED} />
      </InsightSection>

      <Suspense fallback={<RejectedSection rejected={FALLBACK.rejected} />}>
        <LiveRejectedSection />
      </Suspense>

      <Suspense fallback={<ReportsSection index={FALLBACK_REPORTS} />}>
        <LiveReportsSection />
      </Suspense>
    </InsightPage>
  );
}
