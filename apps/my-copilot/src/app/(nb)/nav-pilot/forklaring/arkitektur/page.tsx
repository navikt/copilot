import { BodyLong, BodyShort, Box, HGrid, Label, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";

export const metadata: Metadata = {
  title: "Arkitektur — nav-pilot",
  description:
    "Hvorfor nav-pilot finnes, hva den vet som Copilot ikke vet, hvordan agentpakka er satt sammen, og prinsippene den er bygget på.",
};

const TOC: TocItem[] = [
  { id: "hvorfor", label: "Hvorfor nav-pilot" },
  { id: "agentpakka", label: "Agentpakka" },
  { id: "arkitektur", label: "Tre lag" },
  { id: "designprinsipper", label: "Designprinsipper" },
];

const COMPARISON = [
  ["Fokus", "Orkestrering og flere agenter", "Kunnskap om Nav"],
  ["Hvor du bruker den", "ultrawork i terminalen", "Terminalen, VS Code, JetBrains, github.com"],
  ["Kunnskap", "Generell koding", "Kunnskapsbasen til Nav"],
  ["Autentisering", "Vet ikke hva TokenX er", "Velger autentisering ut fra hvem som kaller"],
  ["Plattform", "Vet ikke hva Nais er", "Lager Nais-manifest med riktig accessPolicy"],
  ["Oppdateringer", "git pull eller for hånd", "Ukentlig pull request fra workflowen for sync"],
];

const KNOWS = [
  "Innbyggere logger inn med ID-porten, saksbehandlere med Azure AD.",
  "Uten accessPolicy.inbound i Nais-manifestet kan ingen kalle tjenesten din.",
  "Standardpoolen i HikariCP (10) er for stor for containere. Start med 3.",
  "Nais skal ha CPU-requests, aldri CPU-limits.",
  "PII skal aldri logges. Logg sakId, ikke fødselsnummer.",
  "Chainguard-images er standard i Nav, ikke distroless.",
  "Meldinger i Rapids & Rivers trenger @event_name og demandValue.",
];

const LAYERS = [
  {
    label: "Instruksjoner",
    desc: "Alltid aktive, med mønstre, kodestandarder og antimønstre fra Nav. Hver økt i Copilot kjenner Nav uten at du gjør noe.",
  },
  {
    label: "Agenten @nav-pilot",
    desc: "Én inngang som velger fase og skill. Sender videre til @kafka og @security-champion, og laster $nav-auth og $nais.",
  },
  {
    label: "Skills",
    desc: "Byggeklosser for intervju, plan, gjennomgang og feilsøking. Du bruker dem via @nav-pilot eller alene.",
  },
];

const PRINCIPLES = [
  { title: "Kunnskap, ikke orkestrering", desc: "Kunnskapen om Nav varer. Orkestrering blir en vare alle har." },
  { title: "Tynn agent, tykke skills", desc: "Agenten sender videre. Skillene har beslutningstrær og sjekklister." },
  { title: "Stopp mellom fasene", desc: "nav-pilot foreslår, du godkjenner, nav-pilot fortsetter." },
  { title: "Arketypen først", desc: "«Hva bygger du?» bestemmer stack, autentisering og Nais-konfig." },
  { title: "Et lite CLI", desc: "Én Go-binær uten avhengigheter. Copilot kjører all KI." },
];

export default function Arkitektur() {
  return (
    <DocPage
      label="Forklaring"
      title="Arkitektur"
      description="nav-pilot installerer markdown-filer og starter klienten. Modellen er Copilot sin. Kunnskapen er Nav sin."
      toc={TOC}
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="hvorfor" size="medium" level="2">
            Hvorfor nav-pilot
          </LinkableHeading>
          <BodyLong>
            oh-my-openagent og lignende verktøy bygger bedre orkestrering: flere agenter, parallelle kjøringer og
            selvkorrigering. nav-pilot bygger bedre kunnskap. Orkestrering får alle etter hvert. Kunnskapen om hvordan
            Nav bygger, er vanskelig å kopiere.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small">
              <TableHeader>
                <TableRow>
                  <TableHeaderCell scope="col" />
                  <TableHeaderCell scope="col">oh-my-openagent</TableHeaderCell>
                  <TableHeaderCell scope="col">nav-pilot</TableHeaderCell>
                </TableRow>
              </TableHeader>
              <TableBody>
                {COMPARISON.map(([what, other, ours]) => (
                  <TableRow key={what}>
                    <TableHeaderCell scope="row">{what}</TableHeaderCell>
                    <TableDataCell>{other}</TableDataCell>
                    <TableDataCell>{ours}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <LinkableHeading id="hva-nav-pilot-vet" size="small" level="3">
            Hva nav-pilot vet som Copilot ikke vet
          </LinkableHeading>
          <Bullets>
            {KNOWS.map((k) => (
              <li key={k}>{k}</li>
            ))}
          </Bullets>
          <BodyShort size="small" textColor="subtle">
            Denne kunnskapen ligger i beslutningstrærne, sjekklistene for blindsoner og diagnosetrærne i skillene.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="agentpakka" size="medium" level="2">
            Agentpakka
          </LinkableHeading>
          <BodyLong>
            Alt Nav-innholdet er én agentpakke: <code className={code}>nav-pilot</code>.{" "}
            <code className={code}>nav-pilot install nav-pilot</code> gir deg alle agenter, skills, instruksjoner,
            prompts, hooks og extensions. Det er med vilje alt. Skills lastes når de trengs. De fleste instruksjonene
            gjelder bare for bestemte filtyper og slår aldri inn i et repo uten dem. Noen få gjelder hver tur:
            skrivestil, bevisst bruk av KI og sikkerhetskjernen. Bare nav-pilot-agentene er hovedagenter. Vil du ha
            mindre, velger du bort i velgeren når du installerer, og valgene overlever sync.
          </BodyLong>
          <BodyLong>
            Noen agentpakker, for eksempel <code className={code}>source = nais/pilot</code>, henter nav-pilot
            manifestet til fra GitHub ved hver oppstart. Svarer ikke GitHub innen 15 sekunder, bruker nav-pilot
            manifestet fra forrige vellykkede oppstart og sier hvilken kilde, hvor gammel kopien er og hvilken commit
            den er fra. Uten en lagret kopi venter nav-pilot på GitHub, og starter ingenting hvis det feiler.
          </BodyLong>
          <BodyLong>
            Vil teamet ditt lage eller bygge videre på en pakke, se{" "}
            <NextLink href="/nav-pilot/agentpakker" className={linkClass}>
              Agentpakker
            </NextLink>
            . Pakkene som finnes, står i{" "}
            <NextLink href="/nav-pilot/agentpakker#pakkene-som-finnes" className={linkClass}>
              Pakkene som finnes
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="arkitektur" size="medium" level="2">
            Tre lag
          </LinkableHeading>
          <BodyLong>nav-pilot er bygget i tre lag:</BodyLong>
          <VStack gap="space-8">
            {LAYERS.map((l, i) => (
              <Box key={l.label} background="neutral-soft" borderRadius="8" padding="space-16">
                <VStack gap="space-4">
                  <Label size="small">
                    Lag {i + 1}: {l.label}
                  </Label>
                  <BodyShort size="small">{l.desc}</BodyShort>
                </VStack>
              </Box>
            ))}
          </VStack>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="designprinsipper" size="medium" level="2">
            Designprinsipper
          </LinkableHeading>
          <HGrid columns={{ xs: 1, sm: 2 }} gap="space-12">
            {PRINCIPLES.map((p) => (
              <Box key={p.title} borderWidth="1" borderColor="neutral-subtle" borderRadius="8" padding="space-16">
                <VStack gap="space-4">
                  <Label size="small">{p.title}</Label>
                  <BodyShort size="small">{p.desc}</BodyShort>
                </VStack>
              </Box>
            ))}
          </HGrid>
        </VStack>
      </section>
    </DocPage>
  );
}
