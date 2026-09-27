import { BodyLong, Box, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { TrustedClassesTable } from "@/components/nav-pilot/local-model-tables";
import { EXPLANATION_PAGES } from "@/components/nav-pilot/doc-pages";
import type { TocItem } from "@/components/table-of-contents";
import { FALLBACK_TABLE, getLocalModels } from "@/lib/local-models";

export const metadata: Metadata = {
  title: "Lokal modell — nav-pilot",
  description:
    "Hvorfor hovedagenten sender lite til den lokale modellen, hvorfor nav-pilot stopper den, og hva modellene klarer i målingene våre.",
};

const TOC: TocItem[] = [
  { id: "utsending", label: "Hvorfor utsendingen er begrenset" },
  { id: "malte-grenser", label: "Målte grenser" },
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
      siblings={{ pages: EXPLANATION_PAGES, current: "/nav-pilot/forklaring/lokal-modell" }}
    >
      <BodyLong>
        Oppsettet står i{" "}
        <NextLink href="/nav-pilot/lokal" className={linkClass}>
          Lokal modell og decide
        </NextLink>
        , og innstillingene i{" "}
        <NextLink href="/nav-pilot/guider/lokal" className={linkClass}>
          guiden
        </NextLink>
        .
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="utsending" size="medium" level="2">
            Hvorfor utsendingen er begrenset
          </LinkableHeading>
          <BodyLong>
            Utsending krever opencode. Copilot CLI leser hvilken leverandør modellen kommer fra, fra en miljøvariabel
            som gjelder hele prosessen, så én leverandør betjener hele økten. Derfor er valget der hele økten lokalt
            eller ingenting lokalt. Vi har sjekket det mot Copilot CLI 1.0.83-3. Runtimen under Copilot CLI kan ha flere
            leverandører i én økt, men Copilot CLI lar ikke en agent velge sin egen ennå, og det er ikke dokumentert. Vi
            tester om det kan brukes (
            <a href="https://github.com/github/copilot-cli/issues/4703" className={linkClass}>
              github/copilot-cli#4703
            </a>
            ).
          </BodyLong>
          <BodyLong>
            I opencode fikk hovedagenten først bare en instruks om hva den burde sende. Det holdt ikke. Nyere modeller i
            skyen følger instruksen dårlig: Sonnet 5 sendte arbeid til den lokale modellen i 1 av 29 testkjøringer, mens
            Sonnet 4.6 gjorde det i 23 av 24. Derfor stopper nav-pilot hovedagenten på nivået{" "}
            <code className={code}>balanced</code> når den gjør en stor mekanisk endring selv, og ber den sende resten.
            Instruksen ber hovedagenten dele en stor endring i én oppgave per fil med en sjekk for hver, og bygge og
            kjøre testene selv til slutt. En endring ett søk-og-erstatt klarer, skal den gjøre selv.
          </BodyLong>
          <BodyLong>nav-pilot stopper bare det manifestet har godkjent modellen for. I dag betyr det:</BodyLong>
          <Bullets>
            <li>
              Standardmodellen på Mac er godkjent bare for mekaniske endringer i flere filer. Regelen om nye filer på{" "}
              <code className={code}>aggressive</code> slår derfor ikke inn.
            </li>
            <li>Qwen 3.8-modellene er ikke godkjent for noe. Med dem stopper nav-pilot ingenting på noe nivå.</li>
            <li>
              En modell på egen server er ikke målt. Den får den generelle instruksen om utsending, ikke den som er
              tilpasset modellen, og nav-pilot stopper ingen redigeringer.
            </li>
          </Bullets>
          <BodyLong>
            nav-pilot stopper heller ingenting når den lokale serveren ikke tar imot tilkoblinger, eller i
            underagentenes egne økter. Nivåene er nye. Vi har ikke målt om stoppet får hovedagenten til å sende mer,
            eller om det sparer AI-credits.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="malte-grenser" size="medium" level="2">
            Målte grenser
          </LinkableHeading>
          <BodyLong>
            Målt i et kontrollert testoppsett, ikke i daglig bruk. Den lokale modellen er god til å gjennomføre en
            beslutning som er tatt, og dårlig til å ta den selv. Om det lønner seg, avhenger av hvor mange steg
            skymodellen trenger alene. Trenger den mange, sparer du mye på å sende det mekaniske lokalt. Er oppgaven
            gjort på to steg, koster utsendingen mer enn den sparer.
          </BodyLong>
          <BodyLong>
            Manifestet sier for hver modell hvilke oppgavetyper hovedagenten kan sende til den. En oppgavetype blir
            godkjent først når modellen har holdt kvalitetsgrensen mot skyen over nok kjøringer og ulike oppgaver.
            Resten gjør hovedagenten selv.
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
                tid, ikke credits.
              </BodyLong>
            </VStack>
          </Box>
          <BodyLong>
            Tiden varierer mye: fra omtrent som skyen på små endringer til rundt fire ganger så lenge på en omdøping. På
            store mekaniske endringer kan den være raskere enn skyen. Kjør <code className={code}>stop</code> når du
            ikke bruker den, for den holder rundt 21 GB minne så lenge den er oppe. Tallene per oppgave står på{" "}
            <NextLink href="/nav-pilot/lokal#malt" className={linkClass}>
              Lokal modell og decide
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
