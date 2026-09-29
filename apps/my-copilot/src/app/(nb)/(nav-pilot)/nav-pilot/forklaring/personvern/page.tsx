import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Personvern og telemetri",
  description: "Hva nav-pilot måler, hva som aldri er med, og hvordan du slår målingene av.",
};

const TOC: TocItem[] = [
  { id: "hva-som-males", label: "Hva som måles" },
  { id: "lokal-modell", label: "Lokal modell" },
  { id: "sla-av", label: "Slå av målingene" },
  { id: "brukerundersokelser", label: "Brukerundersøkelser" },
  { id: "nyheter", label: "Nyheter i terminalen" },
];

export default function Personvern() {
  return (
    <DocPage
      label="Forklaring"
      title="Personvern og telemetri"
      description="nav-pilot teller hendelser, ikke innhold. Prompter, kode, filinnhold og filnavn er aldri med."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hva-som-males" size="medium" level="2">
            Hva som måles
          </LinkableHeading>
          <BodyLong>
            nav-pilot sender bruksmålinger til Nav: hvor ofte kommandoene kjøres, hvilken klient og hvilke innstillinger
            du bruker, og hvilke typer feil som oppstår. Målingene er faste kategorier og tall. Maskinen kjennes igjen
            på en pseudonym ID, ikke på navn eller brukernavn.
          </BodyLong>
          <BodyLong>
            Starter nav-pilot Copilot, slår den også på målingene og sporingen til Copilot, mot samme mottaker. Kjører
            økten i et repo i navikt, merkes de med navnet på repoet. Første gang du kjører nav-pilot i en terminal,
            står dette på én linje.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lokal-modell" size="medium" level="2">
            Lokal modell
          </LinkableHeading>
          <BodyLong>
            Mens den lokale modellen er alfa, måler vi den tettere enn resten av nav-pilot. Vi samler inn hvor mange
            oppgaver hver økt sender til den, også når svaret er null, som er tallet vi lærer mest av. Vi samler også
            inn hvilken modell du kjører, hvor lang tid serveren brukte på å starte, og når den henger. Aldri
            spørsmålene dine, koden din, filnavnene dine eller det modellen svarer.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sla-av" size="medium" level="2">
            Slå av målingene
          </LinkableHeading>
          <BodyLong>Legg én av disse i skallprofilen din:</BodyLong>
          <CodeBlock compact>{`export DO_NOT_TRACK=1
export NAV_PILOT_TELEMETRY_ENABLED=false`}</CodeBlock>
          <BodyLong>
            <code className={code}>DO_NOT_TRACK=1</code> slår av målingene i alle verktøy som følger konvensjonen, også
            i Copilot og opencode når nav-pilot starter dem. Hva hver variabel gjør, står i{" "}
            <NextLink href="/nav-pilot/referanse#telemetri" className={linkClass}>
              referansen
            </NextLink>
            , og alt som måles, står i{" "}
            <a href="https://github.com/navikt/copilot/blob/main/cli/nav-pilot/TELEMETRY.md" className={linkClass}>
              TELEMETRY.md
            </a>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="brukerundersokelser" size="medium" level="2">
            Brukerundersøkelser
          </LinkableHeading>
          <BodyLong>
            Når vi kjører en brukerundersøkelse, kan nav-pilot spørre om du vil svare. Det skjer rett etter at en økt er
            ferdig, aldri midt i arbeidet. Du kan svare nå, senere eller aldri. Velger du senere, spør nav-pilot igjen
            om noen dager, høyst tre ganger per undersøkelse. Før første spørsmål står det hva som sendes: svarene dine,
            nav-pilot-versjon, operativsystem, klient og om lokale modeller er på. Navn, GitHub-bruker, kode, device-ID
            og innhold fra øktene dine sendes ikke.
          </BodyLong>
          <BodyLong>
            nav-pilot bruker GitHub-innloggingen din (<code className={code}>nav-pilot auth login</code>) bare til å
            hindre at noen svarer to ganger. Svarene lagres uten noe som knytter dem til deg, så de kan ikke endres
            eller trekkes tilbake etterpå. nav-pilot spør ikke uten terminal, i CI, når klientens argumenter står etter{" "}
            <code className={code}>--</code>, eller når målinger er slått av. Vil du slå av bare undersøkelsene, ikke
            målingene:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config set surveys false`}</CodeBlock>
          <BodyLong>
            Tokenet fra innloggingen kan ha en utløpstid, for GitHub-apper åtte timer, og kommer da med et refresh-token
            som varer i seks måneder. Begge ligger i nøkkelringen på maskinen, og nav-pilot fornyer tokenet selv. Du må
            logge inn på nytt først når refresh-tokenet har gått ut, eller hvis GitHub trekker det tilbake.
          </BodyLong>
          <BodyLong>
            Noen undersøkelser nevner nav-pilot i stedet med én linje når en økt starter. Du kan alltid svare selv med{" "}
            <code className={code}>nav-pilot survey</code>, også etter at du har svart aldri, etter tre spørsmål, eller
            når surveys er false. Uten terminal lister kommandoen bare de åpne undersøkelsene. Har du svart før, sier
            den fra.
          </BodyLong>
          <BodyLong>
            Du kan også svare på{" "}
            <NextLink href="/nav-pilot/undersokelse" className={linkClass}>
              undersøkelsessiden
            </NextLink>{" "}
            i nettleseren. Der bruker du Nav-innloggingen din, ikke GitHub-innloggingen. Hver person kan svare én gang,
            enten i terminalen eller i nettleseren.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="nyheter" size="medium" level="2">
            Nyheter i terminalen
          </LinkableHeading>
          <BodyLong>
            Når en økt er ferdig, kan nav-pilot vise én linje om en ny sak på{" "}
            <NextLink href="/nyheter" className={linkClass}>
              nyhetssiden
            </NextLink>
            , med tittel og lenke. Det gjelder bare saker skrevet for nav-pilot-brukere og som er under 30 dager gamle,
            og hver sak vises én gang. nav-pilot henter nyhetene som en offentlig fil fra ki-utvikling.nav.no, høyst
            hver sjette time, og sender ingenting tilbake. Svarer ikke nettstedet innen et halvt sekund, henter
            nav-pilot ingenting den gangen.
          </BodyLong>
          <BodyLong>
            Linjen følger de samme reglene som undersøkelsene: ikke uten terminal, ikke i CI, ikke når klientens
            argumenter står etter <code className={code}>--</code>, ikke etter Ctrl-C og ikke når målinger er slått av.
            Har nav-pilot spurt om en undersøkelse i samme økt, venter nyheten til neste gang. Vil du slå av bare
            nyhetene, ikke målingene:
          </BodyLong>
          <CodeBlock compact>{`nav-pilot config set news false`}</CodeBlock>
          <BodyLong>
            <code className={code}>nav-pilot news</code> lister de nyeste sakene, også når linjen er slått av.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
