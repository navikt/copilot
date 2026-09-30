import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Nettverk i sandkassen",
  description:
    "Hva agenten når på nettet i cplt, hvordan du ser hva som blir stoppet, og hvordan du slipper gjennom interne tjenester, andre porter og hoster på en tillatelsesliste.",
};

const TOC: TocItem[] = [
  { id: "standard", label: "Slik er nettverket satt opp" },
  { id: "se-blokkert", label: "Se hva som blir stoppet" },
  { id: "interne", label: "Interne tjenester" },
  { id: "porter", label: "Andre porter enn 443" },
  { id: "tillatelsesliste", label: "Tillatelsesliste" },
  { id: "finn-hoster", label: "Finn hostene bygget bruker" },
];

const FAQ = "/nav-pilot/guider/cplt-feilmeldinger";

export default function CpltNettverk() {
  return (
    <DocPage
      label="Guider"
      upgrade
      title="Nettverk i sandkassen"
      description="Hva agenten når på nettet i cplt, og hvordan du åpner for det den trenger."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="standard" size="medium" level="2">
            Slik er nettverket satt opp
          </LinkableHeading>
          <BodyLong>
            cplt sender trafikken gjennom en proxy. Med standardoppsettet gjelder dette for agenten og alt den starter:
          </BodyLong>
          <Bullets>
            <li>Alle hoster er tillatt, bortsett fra en innebygd liste med kjente steder å lekke data til.</li>
            <li>Bare port 443 er åpen.</li>
            <li>Hoster som peker til en privat IP-adresse, er stengt. Det gjelder mange interne tjenester hos Nav.</li>
            <li>
              localhost er stengt, se{" "}
              <NextLink href={`${FAQ}#localhost`} className={linkClass}>
                connect EPERM 127.0.0.1
              </NextLink>
              .
            </li>
          </Bullets>
          <BodyLong>
            Endringer med <code className={code}>cplt config set</code> gjelder fra neste økt.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="se-blokkert" size="medium" level="2">
            Se hva som blir stoppet
          </LinkableHeading>
          <CodeBlock compact>
            {`cplt check net registry.npmjs.org        # ALLOWED eller BLOCKED, med grunn
cplt config set proxy.log_level blocked   # vis blokkerte kall mens agenten jobber`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>cplt check net</code> sier hvorfor en host er stoppet og hva som løser det.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="interne" size="medium" level="2">
            Interne tjenester
          </LinkableHeading>
          <BodyLong>
            Tillat domenet ved navn. Underdomener blir med, og du skriver det uten <code className={code}>*</code>:
          </BodyLong>
          <CodeBlock compact>{`cplt config set proxy.allow_private_domains intern.nav.no`}</CodeBlock>
          <BodyLong>
            Har du en tillatelsesliste, må domenet stå der også. Adresser skrevet som IP kan ikke åpnes. Bruk
            DNS-navnet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="porter" size="medium" level="2">
            Andre porter enn 443
          </LinkableHeading>
          <CodeBlock compact>{`cplt config set allow.ports 8443`}</CodeBlock>
          <BodyLong>
            Med tvungen proxy, som i <code className={code}>strict</code>, går trafikken på porten gjennom proxyen.
            Verktøy som ikke bruker HTTP-proxy, virker ikke da.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="tillatelsesliste" size="medium" level="2">
            Tillatelsesliste
          </LinkableHeading>
          <BodyLong>
            Med en tillatelsesliste når agenten bare hostene på lista. Slå den på med nav-pilot, så kommer Nav-hostene
            med. Hvordan, står i{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen#sikkerhetsniva" className={linkClass}>
              Sandkassen
            </NextLink>
            . Agentens egne hoster er alltid med. Se dem slik:
          </BodyLong>
          <CodeBlock compact>{`cplt config hosts --agent copilot`}</CodeBlock>
          <BodyLong>Legg til en host. Underdomener blir med:</BodyLong>
          <CodeBlock compact>
            {`cplt config set allow.domains cloud.nais.io
cplt config set allow.domains cloud.nais.io --unset   # fjern igjen`}
          </CodeBlock>
          <BodyLong>
            <code className={code}>allow.domains</code> utvider en liste som allerede er på. Den slår ikke på en liste.
            Peker du <code className={code}>proxy.allowed_domains</code> til en egen fil, må fila ligge utenfor
            prosjektet, for eksempel i <code className={code}>~/.config/cplt/</code>. Ellers starter ikke cplt, fordi
            agenten kunne endret lista selv.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="finn-hoster" size="medium" level="2">
            Finn hostene bygget bruker
          </LinkableHeading>
          <BodyLong>
            Kjør bygget eller testene én gang uten tillatelsesliste, og la cplt skrive ut hostene det kontaktet:
          </BodyLong>
          <CodeBlock compact>{`cplt --observe-domains exec -- ./gradlew build`}</CodeBlock>
          <BodyLong>
            Denne kjøringen håndhever ingen tillatelsesliste. Blokkeringslista, porter og private adresser gjelder
            fortsatt.
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
