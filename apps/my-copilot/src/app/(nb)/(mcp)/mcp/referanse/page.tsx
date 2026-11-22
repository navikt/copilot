import { BodyLong, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableBody, TableDataCell, TableHeader, TableHeaderCell, TableRow } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { PILOT_SERVERS } from "@/components/mcp/servers";

export const metadata: Metadata = {
  title: "MCP-referanse",
  description: "Adresser til serverne bak fleet-gateway, og hvor du finner protokollversjoner, feilkoder og verktøy.",
};

const GATEWAY = "https://fleet-gateway.intern.nav.no";

const TOC: TocItem[] = [
  { id: "adresser", label: "Adresser" },
  { id: "oppslag", label: "Hvor står hva" },
];

export default function McpReferansePage() {
  return (
    <DocPage label="MCP" title="MCP-referanse" description={metadata.description as string} toc={TOC} wide>
      <VStack gap="space-12">
        <LinkableHeading id="adresser" size="medium" level="2">
          Adresser
        </LinkableHeading>
        <BodyLong>
          Serverne i piloten, med prod-adressen klienten kobler til. Registeret har den fulle listen på{" "}
          <code className={code}>https://mcp-registry.nav.no</code>.
        </BodyLong>
        <Table size="small">
          <TableHeader>
            <TableRow>
              <TableHeaderCell scope="col">Server</TableHeaderCell>
              <TableHeaderCell scope="col">Navn i registeret</TableHeaderCell>
              <TableHeaderCell scope="col">Adresse</TableHeaderCell>
              <TableHeaderCell scope="col">Tilgang</TableHeaderCell>
            </TableRow>
          </TableHeader>
          <TableBody>
            {PILOT_SERVERS.map((s) => (
              <TableRow key={s.id}>
                <TableDataCell>{s.title}</TableDataCell>
                <TableDataCell>
                  <code className={code}>{s.id}</code>
                </TableDataCell>
                <TableDataCell>
                  <code className={code}>{GATEWAY + s.route}</code>
                </TableDataCell>
                <TableDataCell>{s.readOnly ? "Bare lesing" : "Lesing og skriving"}</TableDataCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </VStack>

      <VStack gap="space-12">
        <LinkableHeading id="oppslag" size="medium" level="2">
          Hvor står hva
        </LinkableHeading>
        <Bullets>
          <li>
            Protokollversjon per klient:{" "}
            <NextLink href="/mcp/bruke#protokollversjon" className={linkClass}>
              Bruke, Protokollversjon
            </NextLink>
          </li>
          <li>
            Innlogging, varighet og godkjenning:{" "}
            <NextLink href="/mcp/bruke#innlogging" className={linkClass}>
              Bruke, Innlogging
            </NextLink>
          </li>
          <li>
            Hvem som slipper inn:{" "}
            <NextLink href="/mcp/fleet-gateway#hvem-slipper-inn" className={linkClass}>
              fleet-gateway, Hvem slipper inn
            </NextLink>
          </li>
          <li>
            Krav til serveren og autorisasjon:{" "}
            <NextLink href="/mcp/tilby-server#krav-til-serveren" className={linkClass}>
              Tilby en server, Krav til serveren
            </NextLink>
          </li>
          <li>
            Feilkoder og årsaker:{" "}
            <NextLink href="/mcp/tilby-server#feil" className={linkClass}>
              Tilby en server, Vanlige feil
            </NextLink>
          </li>
          <li>
            Maler, samsvarstesten og GitHub Action-en:{" "}
            <NextLink href="/mcp/tilby-server/retningslinjer#eksempler" className={linkClass}>
              Retningslinjer, Eksempler
            </NextLink>
          </li>
        </Bullets>
      </VStack>
    </DocPage>
  );
}
