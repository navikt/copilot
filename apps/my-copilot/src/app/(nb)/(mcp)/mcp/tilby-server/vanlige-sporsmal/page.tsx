import { BodyLong, ReadMore, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Fleet: vanlige spørsmål",
  description:
    "Korte svar for team som registrerer en MCP-server i fleet: cluster, adresse, «Hent verktøy» og hva brukerne gjør etter godkjenning.",
};

const DOLLY_URL = "https://github.com/navikt/copilot/tree/main/apps/mcp-dolly";
const SYNC_ISSUE_URL = "https://github.com/navikt/copilot/issues/2765";
const DEV_FLEET = "https://ki-utvikling.ekstern.dev.nav.no/fleet";

const TOC: TocItem[] = [
  { id: "kort-fortalt", label: "Kort fortalt" },
  { id: "cluster", label: "Hvilket cluster?" },
  { id: "dev-fra-prod", label: "Appen finnes bare i dev" },
  { id: "hent-verktoy", label: "Hva «Hent verktøy» krever" },
  { id: "feiler", label: "Når «Hent verktøy» feiler" },
  { id: "etter-godkjenning", label: "Etter godkjenning" },
];

export default function FleetFaqPage() {
  return (
    <DocPage label="MCP" title="Fleet: vanlige spørsmål" description={metadata.description as string} toc={TOC}>
      <VStack gap="space-12">
        <LinkableHeading id="kort-fortalt" size="medium" level="2">
          Kort fortalt
        </LinkableHeading>
        <Bullets>
          <li>
            En MCP-server hører til ett cluster.{" "}
            <NextLink href="/fleet" className={linkClass}>
              /fleet
            </NextLink>{" "}
            her er prod-gcp.
          </li>
          <li>
            Adressen er intern: <code className={code}>http://&lt;app&gt;.&lt;namespace&gt;</code>, ikke en ingress.
          </li>
          <li>Appen må slippe inn fleet-gateway i accessPolicy.inbound.</li>
          <li>
            Etter godkjenning skrur brukerne på serveren med{" "}
            <code className={code}>nav-pilot mcp enable &lt;navn&gt;</code>.
          </li>
        </Bullets>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="cluster" size="medium" level="2">
          Hvilket cluster hører serveren til?
        </LinkableHeading>
        <BodyLong>
          Ett. /fleet på ki-utvikling.nav.no er prod-gcp og når bare apper i prod-gcp. Dev har sin egen fleet på{" "}
          <a href={DEV_FLEET} className={linkClass}>
            ki-utvikling.ekstern.dev.nav.no/fleet
          </a>
          .
        </BodyLong>
        <BodyLong>
          Adressen må peke på appen i teamets eget navnerom:{" "}
          <code className={code}>http://&lt;app&gt;.&lt;namespace&gt;</code> eller{" "}
          <code className={code}>http://&lt;app&gt;.&lt;namespace&gt;.svc.cluster.local</code>. En ingress godtas ikke.
        </BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="dev-fra-prod" size="medium" level="2">
          Appen finnes bare i dev. Kan vi bruke den fra prod?
        </LinkableHeading>
        <BodyLong>
          Ikke direkte. Lag en liten relé-app i prod, i teamets eget navnerom. Den veksler brukerens token med OBO til
          dev-appen og kaller dev-ingressen. Det virker fordi begge clusterne er i tenanten nav.no.{" "}
          <a href={DOLLY_URL} className={linkClass}>
            mcp-dolly
          </a>{" "}
          gjør det slik.
        </BodyLong>
        <ReadMore header="Vilkår for en relé-app">
          <Bullets>
            <li>
              Godta bare tokens med azp <code className={code}>prod-gcp:copilot:fleet-gateway</code>.
            </li>
            <li>Godta bare persontokens med NAVident. Avvis app-tokens.</li>
            <li>Utgående trafikk bare til dev-ingressen.</li>
            <li>Riktig datakategori på serveren.</li>
            <li>ROS-en sier at prod-identitet brukes mot dev.</li>
          </Bullets>
        </ReadMore>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="hent-verktoy" size="medium" level="2">
          Hva krever «Hent verktøy» av appen?
        </LinkableHeading>
        <BodyLong>
          At appen svarer på MCP over Streamable HTTP og slipper inn fleet-gateway. Gatewayen kaller appen med brukerens
          token, vekslet med OBO til appen.
        </BodyLong>
        <ReadMore header="Detaljene">
          <Bullets>
            <li>
              HTTP POST med <code className={code}>initialize</code> og protocolVersion{" "}
              <code className={code}>2025-06-18</code>, så <code className={code}>notifications/initialized</code> (svar
              202, 200 eller 204), så <code className={code}>tools/list</code>.
            </li>
            <li>
              <code className={code}>MCP-Protocol-Version</code> og <code className={code}>Mcp-Session-Id</code> sendes
              videre på kallene etter initialize.
            </li>
            <li>Svar som JSON eller SSE. Ingen omdirigeringer.</li>
            <li>10 sekunder og 512 KiB totalt.</li>
            <li>
              Alle verktøy på første side. <code className={code}>nextCursor</code> følges ikke.
            </li>
            <li>
              Verktøynavn bare med <code className={code}>A–Z a–z 0–9 _ . -</code>, høyst 128 tegn.
            </li>
            <li>
              fleet-gateway står i <code className={code}>accessPolicy.inbound</code>, og appen godtar azp{" "}
              <code className={code}>&lt;cluster&gt;:copilot:fleet-gateway</code>.
            </li>
          </Bullets>
        </ReadMore>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="feiler" size="medium" level="2">
          Hvorfor feiler «Hent verktøy»?
        </LinkableHeading>
        <BodyLong>/fleet viser veiledning når hentingen feiler. De vanligste årsakene:</BodyLong>
        <Bullets>
          <li>Appen kjører i et annet cluster enn fleet-en.</li>
          <li>fleet-gateway mangler i accessPolicy.inbound.</li>
          <li>Appen er ikke registrert i Entra ID, så OBO-vekslingen feiler.</li>
          <li>Appen svarer ikke innen 10 sekunder.</li>
        </Bullets>
        <BodyLong>Får du ikke hentet verktøyene, kan du legge dem inn for hånd i skjemaet.</BodyLong>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="etter-godkjenning" size="medium" level="2">
          Serveren er godkjent. Hva gjør brukerne?
        </LinkableHeading>
        <Bullets>
          <li>
            Be om en oppføring i MCP-registeret (PR til <code className={code}>apps/mcp-registry/allowlist.json</code>,
            med samme verktøy og risiko som i /fleet). Senere hentes oppføringen automatisk fra Fleet (
            <a href={SYNC_ISSUE_URL} className={linkClass}>
              #2765
            </a>
            ).
          </li>
          <li>
            Brukeren kjører <code className={code}>nav-pilot mcp enable &lt;navn&gt;</code>.
          </li>
          <li>Brukeren starter en ny økt i klienten.</li>
          <li>Første gang logger brukeren inn i fleet-gateway.</li>
        </Bullets>
        <BodyLong>
          Mer om koblingen står i{" "}
          <NextLink href="/mcp/tilby-server" className={linkClass}>
            veiledningen
          </NextLink>{" "}
          og i{" "}
          <NextLink href="/mcp/tilby-server/retningslinjer" className={linkClass}>
            retningslinjene
          </NextLink>
          .
        </BodyLong>
      </VStack>
    </DocPage>
  );
}
