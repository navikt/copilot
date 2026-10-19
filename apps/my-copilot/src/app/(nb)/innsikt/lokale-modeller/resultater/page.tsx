import { BodyLong, BodyShort, Box, HGrid, Label, Link, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { LinkableHeading } from "@/components/linkable-heading";
import MetricCard from "@/components/metric-card";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import { ResultTable } from "@/components/nav-pilot/local-model-tables";
import { CloudComparisonChart, DelegationRangeChart, ReportTimeline } from "@/components/nav-pilot/local-model-charts";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { getUser, getUserToken } from "@/lib/auth";
import { getNavPilotTelemetry } from "@/lib/cached-bigquery";
import { formatDate, formatNumber } from "@/lib/format";
import {
  CLOUD_COMPARISONS,
  DECIDE_RESULTS,
  DELEGATION_RANGES,
  DELEGATION_RESULTS,
  GB64_MEASURED,
  MEASURED_MODEL,
  SOURCES,
  WORKER_RESULTS,
  orderUnmeasured,
} from "@/lib/local-model-results";
import {
  FALLBACK_REPORTS,
  FALLBACK_TABLE,
  getLocalModels,
  getLocalReports,
  type LocalModel,
  type ReportIndex,
} from "@/lib/local-models";

// "AI Credits" is GitHub's name for the billing unit, so it keeps "AI". Other Norwegian text says KI.

export const metadata: Metadata = {
  title: "Resultater for lokale modeller",
  description: "Målingene bak de lokale modellene i nav-pilot, med rapportene de bygger på.",
};

const modelName = (m: LocalModel) => m.model.split("/").pop();

function MeasuredModelNote({ models }: { models: LocalModel[] }) {
  const standard = models.find((m) => m.default) ?? models[0];
  const live = modelName(standard);
  if (live === MEASURED_MODEL) return null;
  return (
    <BodyShort size="small" textColor="subtle">
      Standardmodellen i dag er <code className={code}>{live}</code>. Resultatene under gjelder{" "}
      <code className={code}>{MEASURED_MODEL}</code> og er ikke målt på nytt ennå.
    </BodyShort>
  );
}

async function LiveMeasuredModelNote() {
  return <MeasuredModelNote models={(await getLocalModels()).models} />;
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
        Med en ny parser (<Link href="https://github.com/navikt/mlx-workspace/pull/184">mlx-workspace#184</Link>) besto
        den 0 av 80 oppgaver. Parseren leser fortsatt navnet på verktøyet feil, og 74 prosent av kallene gikk til et
        verktøy som ikke finnes. Vi tester modellen igjen først når parseren består en enhetstest (
        <Link href={SOURCES.k2Ifm}>målingen 10. oktober 2026</Link>
        ).
      </BodyShort>
      {index.unmeasured.length > 0 && (
        <VStack gap="space-8">
          <LinkableHeading id="ikke-malt" size="small" level="3">
            Ikke målt ennå
          </LinkableHeading>
          <BodyShort size="small">
            Siden 9. oktober 2026 måler vi bare når noe utløser det: en ny kandidatmodell, eller en versjon av nav-pilot
            som endrer lokal modus (<Link href={SOURCES.benchmarking}>BENCHMARKING.md</Link>
            ).
          </BodyShort>
          <BodyShort size="small" textColor="subtle">
            Listen hentes fra navikt/mlx-workspace og er på engelsk.
          </BodyShort>
          <BodyShort size="small">Disse punktene avgjør neste beslutning:</BodyShort>
          <UnmeasuredList rows={orderUnmeasured(index.unmeasured).first} />
          <details>
            <summary className={linkClass} style={{ cursor: "pointer" }}>
              Vis de andre {orderUnmeasured(index.unmeasured).rest.length} punktene
            </summary>
            <UnmeasuredList rows={orderUnmeasured(index.unmeasured).rest} />
          </details>
          <BodyShort size="small">
            <Link href={SOURCES.unmeasured}>Hele listen i navikt/mlx-workspace</Link>
          </BodyShort>
        </VStack>
      )}
    </InsightSection>
  );
}

function UnmeasuredList({ rows }: { rows: { item: string; status: string }[] }) {
  return (
    <Bullets>
      {rows.map((u) => (
        <li key={u.item} style={{ overflowWrap: "anywhere" }}>
          {u.item}. <span style={{ color: "var(--ax-text-neutral-subtle)" }}>{u.status}</span>
        </li>
      ))}
    </Bullets>
  );
}

async function LiveReportsSection() {
  return <ReportsSection index={await getLocalReports()} />;
}

async function LiveObservedLocalSection() {
  const user = await getUser(false);
  const token = user ? await getUserToken() : null;
  if (!token) return null;
  const { telemetry } = await getNavPilotTelemetry(token);
  const local = telemetry?.local_7d;
  if (!local || (local.observed_sessions === 0 && local.decide_local_count === 0 && local.devices_7d === 0))
    return null;

  return (
    <InsightSection id="observert-bruk" title="Observert bruk siste syv dager">
      <BodyShort size="small" textColor="subtle">
        Fra aggregerte øktmålinger i nav-pilot for innloggede brukere. En økt med aktivert lokal modell teller også når
        den avsluttes før det sendes trafikk til den lokale modellen.
      </BodyShort>
      <HGrid columns={{ xs: 1, sm: 2, lg: 4 }} gap="space-16">
        <MetricCard
          value={formatNumber(local.observed_sessions)}
          label="Økter med lokal modell"
          helpTitle="Observerte økter"
          helpText="Økter der lokal subagent var slått på i nav-pilot siste syv dager."
          subtitle={local.devices_7d > 0 ? `${formatNumber(local.devices_7d)} enheter siste syv dager` : undefined}
        />
        <MetricCard
          value={formatNumber(local.sessions_with_traffic)}
          label="Økter med lokal trafikk"
          helpTitle="Økter med og uten målt trafikk"
          helpText="Økter der den lokale modellen faktisk mottok forespørsler, sammenlignet med økter som ble avsluttet uten lokal trafikk."
          subtitle={`${formatNumber(local.sessions_without_traffic)} uten målt trafikk`}
        />
        <MetricCard
          value={formatNumber(local.dispatch_attempts)}
          label="Delegeringsforsøk"
          helpTitle="Registrerte delegeringsforsøk"
          helpText="Antall forsøk på å sende oppgaver til den lokale subagenten i perioden. Teller kall, ikke bekreftede løsninger."
        />
        <MetricCard
          value={local.decide_local_count > 0 ? formatNumber(local.decide_local_count) : "–"}
          label="Kall til decide"
          helpTitle="Lokale decide-kall"
          helpText="Antall ja/nei-vurderinger kjørt mot lokal modell med nav-pilot decide siste syv dager. Vises først når minst fem enheter har brukt funksjonen i perioden."
          subtitle={local.decide_local_count === 0 ? "Under 5 enheter siste syv dager" : undefined}
        />
      </HGrid>
    </InsightSection>
  );
}

const source = (
  <VStack gap="space-16">
    <BodyLong>
      Rapportene ligger i navikt/mlx-workspace: <Link href={SOURCES.readme}>oversikt over rapportene</Link>,{" "}
      <Link href={SOURCES.template}>malen hver rapport følger</Link> og{" "}
      <Link href={SOURCES.unmeasured}>det som ikke er målt ennå</Link>.
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

export default function LokaleModellerResultater() {
  return (
    <InsightPage
      title="Resultater for lokale modeller"
      description="Hva vi har målt, og hvordan det avgjør hva hovedagenten får delegere til en lokal modell."
      intro={
        <>
          Hver oppgavetype er målt på én maskin, og tallene her avgjør hva hovedagenten i skyen får delegere til en
          lokal modell. Hvilke modeller du kan kjøre, og hva de er godkjent for, står på{" "}
          <Link as={NextLink} href="/innsikt/lokale-modeller">
            Lokale modeller
          </Link>
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
          helpText="Med aggressive delegerte Sonnet 5 i alle 17 gyldige kjøringer, og alle 17 besto bygg og tester (28. september 2026). To kjøringer til ble avbrutt av en avvist filskriving før de var ferdige, så regnet på forsøk er det 17 av 19. Sonnet 5.5 er målt på andre oppgaver, se Delegeringsnivåene."
          subtitle="aggressive, Sonnet 5, 28. september"
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

      <Suspense fallback={<MeasuredModelNote models={FALLBACK_TABLE.models} />}>
        <LiveMeasuredModelNote />
      </Suspense>

      <Suspense fallback={null}>
        <LiveObservedLocalSection />
      </Suspense>

      <InsightSection id="delegeringsnivaer" title="Delegeringsnivåene">
        <BodyLong>
          Hovedagenten delegerer ikke av seg selv. Med bare en instruks om hva den burde delegere, delegerte Sonnet 5
          arbeid til den lokale modellen i 1 av 29 testkjøringer. Derfor har nav-pilot en delegeringssperre, og nivået{" "}
          <code className={code}>aggressive</code> er standard fra 30. september 2026: det er det eneste nivået som
          faktisk delegerer, og alt som ble delegert, besto testene. Prisen er tid og som regel flere AI Credits.
        </BodyLong>
        <BodyLong>
          Tallet 17 av 17 er målt med Sonnet 5 den 28. september 2026. Den 12. oktober 2026 målte vi Sonnet 5.5 med{" "}
          <code className={code}>aggressive</code> på to små endringer i én fil. Den delegerte i 4 av 30 kjøringer og
          valgte selv å gjøre resten. Alle 4 delegeringene besto (<Link href={SOURCES.rp7c}>rapporten</Link>
          ). Oppgavene var andre og mindre enn i september, så de to resultatene kan ikke sammenlignes direkte.{" "}
          <code className={code}>aggressive</code> er fortsatt standard.
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
            Disse tallene er fra september. En ny måling <Link href={SOURCES.cfRetry2}>9. oktober 2026</Link> ga 10 av
            10 på de to letteste trinnene, både med standardmodellen og 8-bitsmodellen. Uten retry2 ga{" "}
            <Link href={SOURCES.gapFill}>samme oppsett 10. oktober 2026</Link> 26 av 40, mot 37 av 40 med retry2.
            Usikkerhetsintervallene (95 prosent) overlapper ikke. Gevinsten kommer altså fra retry2, og vi beholder det.
          </li>
          <li>
            På de to vanskeligere trinnene er retry2 ikke godt nok ennå. På trinn 3 ga{" "}
            <Link href={SOURCES.cfR34}>målingen 10. oktober 2026</Link> 19 av 20 med standardmodellen og 9 av 10 med
            8-bitsmodellen. Kravet er at nedre grense skal være minst 0,90, og dit er det ikke ennå. En{" "}
            <Link href={SOURCES.cfR3c}>oppfølging 11. oktober 2026</Link> ga standardmodellen 36 av 40 samlet. Den
            ensidige nedre grensen (90 prosent) er 0,82, under kravet på 0,90. Alle de fire feilene nådde tidsgrensen på
            420 sekunder per forsøk. Trinn 4 går til skymodellen med begge profilene: standardmodellen ga 4 av 10 og
            8-bitsmodellen 3 av 10, og de fleste feilene var tidsavbrudd. Rutingen stopper derfor fortsatt etter trinn
            2. Oppfølgingen målte ikke kostnad, så vi sier ingenting om den.
          </li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Målt 28. og 29. september og 12. oktober 2026. Se <Link href={SOURCES.reprobe7}>delegeringsmålingen</Link>,{" "}
          <Link href={SOURCES.balanced}>kontrollmålingen av balanced</Link>,{" "}
          <Link href={SOURCES.followups1}>målingen av nye forsøk</Link> og{" "}
          <Link href={SOURCES.rp7c}>målingen med Sonnet 5.5</Link>.
        </BodyShort>
      </InsightSection>

      <InsightSection id="malt-delegering" title="Resultater for kodeoppgaver">
        <BodyLong>
          Hver oppgave er løst flere ganger, og hver løsning er sjekket av en test. «10 av 10» betyr at testen besto i
          alle ti forsøkene.
        </BodyLong>
        <ResultTable rows={WORKER_RESULTS.rows} />
        <CloudComparisonChart comparisons={CLOUD_COMPARISONS} />
        <BodyLong>
          Nye filer er bare 5 % av ekte pull requests, viser en{" "}
          <Link href={SOURCES.prAudit}>gjennomgang av pull requests i navikt</Link>. Omtrent 29 % av pull requestene fra
          mennesker er endringer i én fil, dokumentasjon eller bare tester. Begge lokale modellene klarer slike oppgaver
          lokalt i minst 75 % av forsøkene (<Link href={SOURCES.smallDelegate}>små oppgaver</Link>
          ).
        </BodyLong>
        <BodyLong>
          Endringer i konfigurasjon og deploy feilet først i alle 45 forsøkene. Feilen lå i testoppsettet. Etter
          rettingen (<Link href="https://github.com/navikt/mlx-workspace/pull/189">mlx-workspace#189</Link>) besto
          standardmodellen 12 av 14 og 8-bitsmodellen 14 av 14 (
          <Link href={SOURCES.fixedSmall}>målingen 10. oktober 2026</Link>
          ). Samme måling ga disse resultatene per oppgavetype, standardmodellen mot 8-bitsmodellen:
        </BodyLong>
        <Bullets>
          <li>endring i én fil: 37 av 42 mot 42 av 42</li>
          <li>ny fil: 29 av 42 mot 31 av 42</li>
          <li>mekanisk endring i flere filer: 25 av 28 mot 19 av 28</li>
          <li>spørsmål om kodebasen: 39 av 42 mot 42 av 42</li>
        </Bullets>
        <BodyShort size="small" textColor="subtle">
          Én oppgave er utelatt fordi testen trolig er feil, og én er pensjonert. Alle forsøkene kjørte lokalt. Målingen
          er ikke brukt til å vurdere hva modellene er godkjent for.
        </BodyShort>
        <BodyLong>
          Tiden varierer fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På store
          mekaniske endringer kan den lokale modellen være raskere.
        </BodyLong>
        <BodyShort size="small" textColor="subtle">
          Målt {formatDate("2026-09-25")} og {formatDate("2026-09-26")}, nye filer med retry2{" "}
          <Link href={SOURCES.cfRetry2}>{formatDate("2026-10-09")}</Link> og uten retry2{" "}
          <Link href={SOURCES.gapFill}>{formatDate("2026-10-10")}</Link>. Se{" "}
          <Link href={SOURCES.night1}>kvalitetsnatt 1</Link> og <Link href={SOURCES.night2}>kvalitetsnatt 2</Link>.
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
          når de var snudd (<Link href={SOURCES.layout}>måling</Link>
          ). «Ja»-svarene er stabile, mens «nei»-svarene trekkes mot det siste svaralternativet, mest når spørsmålet er
          snudd (<Link href={SOURCES.followups1}>oppfølging</Link>
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
          <Link href={SOURCES.why}>commit-spørsmålet</Link>, <Link href={SOURCES.sets}>issue og PR</Link> og{" "}
          <Link href={SOURCES.systemOne}>decide-rapporten</Link>.
        </BodyShort>
      </InsightSection>

      <Suspense fallback={<ReportsSection index={FALLBACK_REPORTS} />}>
        <LiveReportsSection />
      </Suspense>
    </InsightPage>
  );
}
