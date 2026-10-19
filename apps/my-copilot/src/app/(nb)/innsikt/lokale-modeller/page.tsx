import { BodyLong, BodyShort, Link, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Bullets, code, linkClass } from "@/components/nav-pilot/doc-page";
import {
  ClassCountsTable,
  LocalModelsTable,
  barText,
  modelCountRows,
  rejectedCountRows,
} from "@/components/nav-pilot/local-model-tables";
import { IntervalChart, ModelSizeChart } from "@/components/nav-pilot/local-model-charts";
import { InsightPage, InsightSection } from "@/components/insight-page";
import { formatDate } from "@/lib/format";
import { GB64_MEASURED, SOURCES } from "@/lib/local-model-results";
import { FALLBACK_TABLE, MANIFEST_URL, getLocalModels, type LocalModel, type RejectedModel } from "@/lib/local-models";

export const metadata: Metadata = {
  title: "Lokale modeller",
  description: "Hvilke KI-modeller nav-pilot kan kjøre på egen Mac, og hva de er godkjent for.",
};

type Table = { models: LocalModel[]; fetchedAt: string | null };

const FALLBACK = { ...FALLBACK_TABLE, fetchedAt: null };

// Everything that reads the manifest. Rendered with the checked-in copy while the live one loads.
function ManifestSections({ table }: { table: Table }) {
  const { models } = table;
  const standard = models.find((m) => m.default) ?? models[0];
  return (
    <>
      <InsightSection id="modeller-per-minne" title="Valgte modeller">
        <BodyLong>
          Hvilken modell du kan kjøre, avhenger av hvor mye minne Macen har. Standardmodellen er den nav-pilot velger
          hvis du ikke velger selv. Kontekst er hvor mye tekst modellen kan lese på én gang, målt i tokens.
          nav-pilot-kolonnen viser hvilken versjon som trengs.
        </BodyLong>
        <LocalModelsTable stack models={models} />
        <ModelSizeChart models={models} />
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
        <BodyShort size="small" textColor="subtle">
          Feilsøking ble målt første gang 12. oktober 2026, og ingen modell er godkjent for den ennå. Skymodellen besto
          10 av 10, så kravet er 95 prosent, og 10 kjøringer er for få til å vise det selv når alle består (
          <Link href={SOURCES.dbg}>rapporten</Link>
          ). Nå måles feilsøking med testkjøringer i en sandkasse med egen Postgres-database. Skymodellen besto 30 av
          30. Qwen3.8-27B-8bit besto 19 av 30, og Qwen3.8-27B-OptiQ-4bit besto 17 av 25. Begge ligger langt under
          kravet, så feilsøking med disse modellene går til skyen (
          <Link href="https://github.com/navikt/mlx-workspace/pull/289">mlx-workspace#289</Link>
          ).
        </BodyShort>
        <IntervalChart model={standard} />
      </InsightSection>
    </>
  );
}

function RejectedSection({ rejected }: { rejected: RejectedModel[] }) {
  if (!rejected.length) return null;
  return (
    <InsightSection id="forkastede-modeller" title="Forkastede modeller">
      <BodyLong>
        Disse modellene er målt, men nav-pilot tilbyr dem ikke. Enten nådde ingen oppgavetype kravet, eller så gjorde en
        annen versjon jobben bedre.
      </BodyLong>
      <ClassCountsTable rows={rejectedCountRows(rejected)} />
    </InsightSection>
  );
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
      Modelloversikten hentes fra <Link href={MANIFEST_URL}>manifestet</Link> i navikt/mlx-workspace. «Sist oppdatert»
      er når den sist ble hentet.
    </>
  ) : (
    <>
      Manifestet kunne ikke hentes eller leses nå, så modelloversikten viser en lagret kopi av{" "}
      <Link href={MANIFEST_URL}>manifestet</Link>.
    </>
  );
}

const source = (
  <VStack gap="space-16">
    <BodyLong>
      <Suspense fallback="Modelloversikten hentes fra manifestet i navikt/mlx-workspace.">
        <ManifestSource />
      </Suspense>{" "}
      Rapportene ligger også der: <Link href={SOURCES.readme}>oversikt over rapportene</Link>,{" "}
      <Link href={SOURCES.template}>malen hver rapport følger</Link> og{" "}
      <Link href={SOURCES.unmeasured}>det som ikke er målt ennå</Link>.
    </BodyLong>
  </VStack>
);

export default function LokaleModeller() {
  return (
    <InsightPage
      title="Lokale modeller"
      description="nav-pilot kan kjøre en KI-modell på din egen Mac. Her ser du hvilke modeller som er valgt, og hva de er godkjent for."
      intro={
        <>
          Hovedagenten er modellen i skyen som leder arbeidet. Den kan delegere avgrensede oppgaver til en lokal modell
          som subagent. Hva den får delegere, avgjør målingene, ikke modellen selv. Vil du ta i bruk en lokal modell,
          les{" "}
          <NextLink href="/nav-pilot/forklaring/lokal-modell" className={linkClass}>
            Lokal modell
          </NextLink>
          . Målingene står på{" "}
          <NextLink href="/innsikt/lokale-modeller/resultater" className={linkClass}>
            Resultater for lokale modeller
          </NextLink>
          .
        </>
      }
      updated={async () => (await getLocalModels()).fetchedAt}
      hourly
      source={source}
    >
      <Suspense fallback={<ManifestSections table={FALLBACK} />}>
        <LiveManifestSections />
      </Suspense>

      <InsightSection id="modell-64-gb" title="Modellen for 64 GB">
        <BodyLong>
          <code className={code}>qwen3.6-35b-a3b-8bit</code> er standardmodellen i 8 bit, for Macer med minst 64 GB
          minne.
        </BodyLong>
        <Bullets>
          <li>
            Den er godkjent for svar om kode og for endringer i én fil når den selv er hovedagent. Etter målingene 12.
            oktober 2026 står den på 96 av 96 for svar og 124 av 128 for endringer i én fil (
            <Link href={SOURCES.capabilities}>manifestet</Link>, <Link href={SOURCES.oq3a}>rapporten</Link>
            ). Den er ikke godkjent for delegering.
          </li>
          <li>
            Standardmodellen fikk samme dag 48 av 48 på endringer i én fil når den fikk et nytt forsøk etter feilet bygg
            eller test (<Link href={SOURCES.r2e}>rapporten</Link>
            ). nav-pilot gir nå også et slikt nytt forsøk når den lokale modellen endrer en fil (
            <Link href="https://github.com/navikt/copilot/pull/1675">copilot#1675</Link>
            ).
          </li>
          <li>I september løste den oppgaven i 12 av 12 Copilot-økter.</li>
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
        <BodyShort size="small" textColor="subtle">
          Målt 27. september og {formatDate(GB64_MEASURED.measured)}. Se{" "}
          <Link href={SOURCES.gb64}>rapporten fra september</Link>.
        </BodyShort>
      </InsightSection>

      <Suspense fallback={<RejectedSection rejected={FALLBACK.rejected} />}>
        <LiveRejectedSection />
      </Suspense>
    </InsightPage>
  );
}
