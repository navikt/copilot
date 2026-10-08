import { BodyLong, BodyShort, Box, VStack, HGrid, Tag } from "@navikt/ds-react";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { PageHero } from "@/components/page-hero";
import { TableOfContents, type TocItem } from "@/components/table-of-contents";
import { BackToTop } from "@/components/back-to-top";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Bullets, code } from "@/components/nav-pilot/doc-page";

export const metadata: Metadata = {
  title: "Agentpakker",
  description:
    "Fire steg i rekkefølge: bruk en pakke som finnes, ta delene du trenger, bygg videre på en, og lag din egen først når ingenting av det holder.",
};

const DOC_SECTIONS: TocItem[] = [
  { id: "hva-det-er", label: "Hva det er" },
  {
    id: "bruk-en-som-finnes",
    label: "1. Bruk en som finnes",
    children: [
      { id: "pakkene-som-finnes", label: "Pakkene som finnes" },
      { id: "las-installasjonen", label: "Lås installasjonen" },
    ],
  },
  { id: "ta-delene-du-trenger", label: "2. Ta delene du trenger" },
  { id: "bygg-videre-pa-en", label: "3. Bygg videre på en" },
  {
    id: "lag-din-egen",
    label: "4. Lag din egen",
    children: [
      { id: "struktur", label: "Struktur" },
      { id: "manifestet", label: "Manifestet" },
      { id: "valider", label: "Valider" },
      { id: "distribuer", label: "Distribuer" },
    ],
  },
  { id: "vedlikehold", label: "Vedlikehold" },
  { id: "ressurser", label: "Ressurser" },
];

const PAKKER: { repo: string; pakke: string; tier: string; what: string; install?: string }[] = [
  {
    repo: "navikt/copilot",
    pakke: "nav-pilot",
    tier: "Tier 1",
    what: "Nav-innholdet: agentene, skillene, instruksjonene, promptene, hooks og extensions, for copilot, opencode og pi. Tier 1 vil si at nav-pilot legger filene i repoet ditt, så du ser dem i diffen.",
    install: "nav-pilot install nav-pilot --source navikt/copilot --repo",
  },
  {
    repo: "navikt/grillmester",
    pakke: "grillmester",
    tier: "Tier 2",
    what: "Team eSyfo sitt agentlag for kodeendringer som skal avklares og avgrenses før de skrives, og gjennomgås uavhengig etterpå. Agentene heter grillmester, barista, designer og doctor-who. En fokusert kontekst gir bare barista og grill-inspektor. For copilot og opencode. Tier 2 vil si at repoet bygger ferdige payload-trær selv, og at nav-pilot verifiserer dem mot en digest og pinner dem per bruker som én revisjon, ikke som filer i repoet ditt.",
  },
  {
    repo: "nais/pilot",
    pakke: "nais-platform",
    tier: "Tier 1",
    what: "For dem som bygger Nais-plattformen: Nais API, tenants og miljøclustere, Fasit, Terraform og Loki/Mimir/Tempo. Agentene nais-platform og nais-review, skills og instruksjoner for alle tre klientene, og en preToolUse-hook som nekter en cluster- eller LGTM-kommando maskinen ikke kan betjene.",
  },
];

const FROZEN = `- run: nav-pilot install nais-platform --frozen --force`;

const ITEMS = `{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa",
  "items": {
    "nav-pilot": "agent",
    "klarsprak": "skill"
  }
}`;

const STRUKTUR = `ditt-repo/
├── .nav-pilot/
│   └── agentpakke.json
├── agents/
│   └── grillmester.agent.md
└── skills/`;

const MANIFEST = `{
  "contractVersion": "1",
  "name": "ditt-team",
  "description": "Hva pakka er til for",
  "layout": {
    "agents": "agents",
    "skills": "skills"
  },
  "clients": {
    "copilot": {
      "primaryAgents": ["grillmester"]
    }
  }
}`;

const VALIDER_CMD = `nav-pilot validate --source "$PWD"`;

const VALIDER_UT = `Validating: /Users/deg/ditt-repo@c3f7ca3

  ℹ manifest: .nav-pilot/agentpakke.json
  ℹ agentpakke: ditt-team (contract version 1)
  ℹ clients: copilot (tier 1)

✓ /Users/deg/ditt-repo conforms to the agentpakke contract.`;

const INSTALL_CMD = `nav-pilot install ditt-team --source navikt/ditt-repo --repo`;

const GJENBRUK = `{
  "contractVersion": "1",
  "source": "navikt/copilot",
  "sha": "b73a69e0000000000000000000000000000000aa"
}`;

const KOMPONERT_UT = `Source: navikt/ditt-repo@c3f7ca3
Reuses: navikt/copilot@b73a69e`;

const linkClass = "text-blue-600 hover:underline";

export default function Agentpakker() {
  return (
    <main id="hovedinnhold" tabIndex={-1}>
      <PageHero
        title="Agentpakker"
        description="Ta i bruk det som finnes, før du lager noe eget."
        badge={
          <Tag variant="info" size="small" className="uppercase tracking-wide">
            Gjenbruk først
          </Tag>
        }
      />
      <div className="max-w-7xl mx-auto">
        <Box
          paddingBlock={{ xs: "space-16", sm: "space-20", md: "space-24" }}
          paddingInline={{ xs: "space-16", sm: "space-20", md: "space-32", lg: "space-40" }}
        >
          <div className="flex gap-12">
            <div className="min-w-0 flex-1">
              <VStack gap={{ xs: "space-32", md: "space-40" }}>
                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="hva-det-er" size="medium" level="2">
                      Hva det er
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      En agentpakke er et git-repo med KI-artefakter og en fil som beskriver dem. Ingenting skal
                      registreres eller godkjennes. Et repo med et gyldig manifest er en agentpakke, og den som vil ha
                      den, peker på repoet med <code className={code}>--source</code>.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      Agentpakker er måten et team påvirker verktøykassa uten å bygge den selv. Sida går gjennom fire
                      steg i rekkefølge: bruk en pakke som finnes, ta delene du trenger av den, bygg videre på den, og
                      lag din egen først når ingenting av det holder. Skal du bare bruke Nav-innholdet, trenger du ikke
                      denne sida:{" "}
                      <NextLink href="/nav-pilot/guider" className={linkClass}>
                        guidene for nav-pilot
                      </NextLink>{" "}
                      dekker installasjon og bruk.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      Lager du en pakke selv, står alle felt og regler i{" "}
                      <NextLink href="/nav-pilot/agentpakker/referanse#artefakttyper" className={linkClass}>
                        referansen for agentpakker
                      </NextLink>
                      .
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="bruk-en-som-finnes" size="medium" level="2">
                      1. Bruk en som finnes
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Det billigste steget er å installere en pakke noen alt vedlikeholder. nav-pilot finner ikke pakker
                      for deg: <code className={code}>install</code> krever at du kjenner reponavnet, og det finnes
                      ingen kommando som lister pakker. Lista under er ført for hånd.
                    </BodyLong>

                    <LinkableHeading id="pakkene-som-finnes" size="small" level="3">
                      Pakkene som finnes
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Disse pakkene vet vi om. Vil du ta i bruk en annen pakke enn{" "}
                      <code className={code}>nav-pilot</code>, spør du teamet som eier den først.
                    </BodyLong>
                    <VStack gap="space-16">
                      {PAKKER.map((p) => (
                        <Box key={p.repo} background="neutral-soft" padding="space-16" borderRadius="8">
                          <VStack gap="space-8">
                            <BodyShort weight="semibold">
                              <a href={`https://github.com/${p.repo}`} className={linkClass}>
                                {p.repo}
                              </a>
                              <span className="font-normal">
                                , pakka <code className={code}>{p.pakke}</code>, {p.tier}
                              </span>
                            </BodyShort>
                            <BodyLong size="small" textColor="subtle">
                              {p.what}
                            </BodyLong>
                            {p.install && <CodeBlock compact>{p.install}</CodeBlock>}
                          </VStack>
                        </Box>
                      ))}
                    </VStack>
                    <LinkableHeading id="las-installasjonen" size="small" level="3">
                      Lås installasjonen
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Installerer du i et repo, skriver <code className={code}>install</code>{" "}
                      <code className={code}>.nav-pilot/agentpakke.lock.json</code> med kilden og revisjonen.
                      Bruker-scope har ingen erklæring, for <code className={code}>~/.copilot</code> er ikke et repo:
                      der ligger revisjonen i din egen tilstandsfil, og gjelder bare deg. Commit erklæringa: da
                      installerer hele teamet fra samme revisjon, og{" "}
                      <code className={code}>nav-pilot sync --apply</code> flytter pinnen som én linje diff i en pull
                      request.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      I CI bruker du <code className={code}>--frozen</code>: den installerer nøyaktig det erklæringa
                      sier, eller lar være. Den spør aldri og flytter aldri pinnen. Exit-koden skiller «pinnen er ikke
                      det repoet sier» fra «installasjonen røk», se{" "}
                      <NextLink href="/nav-pilot/referanse#avslutningskoder" className={linkClass}>
                        avslutningskodene
                      </NextLink>
                      .
                    </BodyLong>
                    <CodeBlock>{FROZEN}</CodeBlock>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="ta-delene-du-trenger" size="medium" level="2">
                      2. Ta delene du trenger
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Vil du ha fire av tolv agenter fra en plattformpakke, skal du slippe å forke den.{" "}
                      <code className={code}>items</code> i erklæringa parer navn med artefakttype. Uten feltet
                      installeres alt pakka har. Feltet skriver du selv: nav-pilot fyller det aldri ut. Hadde den ført
                      opp alle tolv, ville hver ny agent oppstrøms blitt en merge-konflikt hos hver konsument.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{ITEMS}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Navnene sjekkes mot det pakka har. En skrivefeil stopper hele installasjonen. Du får ikke tre av
                      fire uten beskjed.
                    </BodyLong>
                    <BodyLong textColor="subtle">
                      <code className={code}>items</code> virker bare mot Tier 1. En Tier 1-pakke er filer som kan
                      velges hver for seg. En Tier 2-pakke er payload-trær bundet til en digest, der revisjonen er
                      enheten, ikke fila. <code className={code}>items</code> mot en Tier 2-pakke nektes, ikke
                      ignoreres, og feilmeldinga ber deg fjerne blokka eller be pakka publisere en payload-kontekst med
                      akkurat det teamet trenger.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="bygg-videre-pa-en" size="medium" level="2">
                      3. Bygg videre på en
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Vil du bygge på Nav-pakka i navikt/copilot uten å vedlikeholde en kopi av den, committer du den
                      samme erklæringa i ditt eget pakkerepo. Repoet ditt har da både et manifest og en erklæring, og
                      den som installerer pakka di får begge pakkenes innhold. Erklæringa løses på nytt ved hver{" "}
                      <code className={code}>install</code> og <code className={code}>sync</code>.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.lock.json">{GJENBRUK}</CodeBlock>
                    <BodyLong textColor="subtle">
                      <code className={code}>sha</code> er påkrevd for en kilde på formen{" "}
                      <code className={code}>owner/repo</code>. Uten den ville gjenbruken hentet det main tilfeldigvis
                      holdt, og to installasjoner en uke fra hverandre fått ulikt innhold. Installasjonen sier hva den
                      gjenbrukte:
                    </BodyLong>
                    <CodeBlock>{KOMPONERT_UT}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Har begge pakkene et artefakt med samme navn, installeres ditt. Slik endrer du én ting i en pakke
                      du ellers tar som den er. Hvordan pakkene settes sammen, og hvordan du holder basen oppdatert,
                      står under{" "}
                      <NextLink href="/nav-pilot/agentpakker/referanse#gjenbruk" className={linkClass}>
                        Gjenbruk av en annen pakke
                      </NextLink>
                      .
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="lag-din-egen" size="medium" level="2">
                      4. Lag din egen
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Skygging og <code className={code}>items</code> dekker de fleste grunnene folk har til å forke en
                      pakke. Lag din egen når innholdet ikke finnes noe sted.
                    </BodyLong>

                    <LinkableHeading id="struktur" size="small" level="3">
                      Struktur
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Katalognavnene velger du selv. <code className={code}>layout</code> i manifestet peker på dem:
                      heter katalogen <code className={code}>innhold/agenter</code> hos deg, skriver du det der.
                      Filnavnene inne i katalogene er låst, se{" "}
                      <NextLink href="/nav-pilot/agentpakker/referanse#artefakttyper" className={linkClass}>
                        artefakttypene
                      </NextLink>
                      . Hver agentfil åpner med YAML-frontmatter som minst har <code className={code}>name</code> og{" "}
                      <code className={code}>description</code>.
                    </BodyLong>
                    <CodeBlock>{STRUKTUR}</CodeBlock>

                    <LinkableHeading id="manifestet" size="small" level="3">
                      Manifestet
                    </LinkableHeading>
                    <BodyLong textColor="subtle">
                      Minste form som validerer. <code className={code}>layout</code> navngir katalogene pakka faktisk
                      har, minst én av dem. <code className={code}>primaryAgents</code> er de agentene brukeren kan
                      starte klienten som. Resten er underagenter andre kaller. Første navn startes som standard, og{" "}
                      <code className={code}>nav-pilot --persona &lt;navn&gt;</code> velger et annet av dem. Hvert navn
                      må ha en agentfil i <code className={code}>layout.agents</code>, ellers avviser{" "}
                      <code className={code}>validate</code> og <code className={code}>install</code> manifestet.
                    </BodyLong>
                    <CodeBlock filename=".nav-pilot/agentpakke.json">{MANIFEST}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Resten av manifestet er valgfritt. Referansen beskriver hvert felt:
                    </BodyLong>
                    <Bullets>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#klientoppforinga" className={linkClass}>
                          Klientoppføringa
                        </NextLink>
                        : klienter, versjonsområder, standardmodell og minste nav-pilot-versjon.
                      </li>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#hvilken-tier" className={linkClass}>
                          Hvilken tier
                        </NextLink>{" "}
                        du skal velge. Velg Tier 1 om du ikke har en grunn til noe annet.
                      </li>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#uten-agent" className={linkClass}>
                          Pakke uten agent
                        </NextLink>
                        , for dere som bare deler skills eller instruksjoner.
                      </li>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#mcp-servere" className={linkClass}>
                          MCP-servere
                        </NextLink>{" "}
                        pakka forventer.
                      </li>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#sandkasse" className={linkClass}>
                          Sandkassekonfigurasjon
                        </NextLink>
                        : unntak fra cplt som brukeren må godkjenne.
                      </li>
                      <li>
                        <NextLink href="/nav-pilot/agentpakker/referanse#kjorbar-kode" className={linkClass}>
                          Kjørbar kode
                        </NextLink>{" "}
                        og{" "}
                        <NextLink href="/nav-pilot/agentpakker/referanse#skript-i-en-skill" className={linkClass}>
                          skript i en skill
                        </NextLink>
                        .
                      </li>
                    </Bullets>

                    <LinkableHeading id="valider" size="small" level="3">
                      Valider
                    </LinkableHeading>
                    <CodeBlock>{VALIDER_CMD}</CodeBlock>
                    <CodeBlock>{VALIDER_UT}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Bruk <code className={code}>&quot;$PWD&quot;</code>, eller{" "}
                      <code className={code}>&quot;$GITHUB_WORKSPACE&quot;</code> i CI.{" "}
                      <code className={code}>--source .</code> avvises. Utelater du{" "}
                      <code className={code}>--source</code>, validerer du Nav-pakka i stedet for din, og alt ser grønt
                      ut. Advarsler (<code className={code}>⚠</code>) feiler ikke kommandoen. Mer om kilden og skjemaet
                      står under{" "}
                      <NextLink href="/nav-pilot/agentpakker/referanse#validering" className={linkClass}>
                        Validering
                      </NextLink>
                      .
                    </BodyLong>

                    <LinkableHeading id="distribuer" size="small" level="3">
                      Distribuer
                    </LinkableHeading>
                    <CodeBlock>{INSTALL_CMD}</CodeBlock>
                    <BodyLong textColor="subtle">
                      Det er hele distribusjonen. Den som installerer får{" "}
                      <code className={code}>.nav-pilot/agentpakke.lock.json</code> i sitt eget repo, med kilden og
                      revisjonen, og committer den. Da installerer hele teamet fra samme revisjon.
                    </BodyLong>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="vedlikehold" size="medium" level="2">
                      Vedlikehold
                    </LinkableHeading>

                    <BodyLong textColor="subtle">
                      Konsumentene er pinnet til revisjonen de installerte, så en endring du pusher når dem først når de
                      kjører <code className={code}>nav-pilot sync --apply</code>. Tre ting å vite før du endrer en
                      pakke andre bruker:
                    </BodyLong>
                    <Bullets>
                      <li>
                        Endre ikke <code className={code}>name</code> i manifestet. Da blir eksisterende installasjoner
                        en annen pakke.
                      </li>
                      <li>
                        Vil du jobbe videre på main uten at alt går rett ut, publiserer du{" "}
                        <NextLink href="/nav-pilot/agentpakker/referanse#stabile-releases" className={linkClass}>
                          stabile releases
                        </NextLink>
                        .
                      </li>
                      <li>
                        Sletter du et artefakt, fører du det opp i{" "}
                        <code className={code}>.nav-pilot/retired-artifacts.json</code>, se{" "}
                        <NextLink href="/nav-pilot/agentpakker/referanse#pensjonering" className={linkClass}>
                          Pensjonering
                        </NextLink>
                        .
                      </li>
                    </Bullets>
                  </VStack>
                </section>

                <section>
                  <VStack gap="space-16">
                    <LinkableHeading id="ressurser" size="medium" level="2">
                      Ressurser
                    </LinkableHeading>
                    <HGrid gap="space-16" columns={{ xs: 1, md: 2 }}>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <VStack gap="space-4">
                          <BodyShort weight="semibold">
                            <a
                              href="https://github.com/navikt/copilot/blob/main/docs/README.agentpakke.md"
                              className={linkClass}
                            >
                              Feltreferansen
                            </a>
                          </BodyShort>
                          <BodyLong size="small" textColor="subtle">
                            Beskriver hvert felt i manifestet og erklæringa, tier, stiregler og kompatibilitet.
                          </BodyLong>
                        </VStack>
                      </Box>
                      <Box background="neutral-soft" padding="space-16" borderRadius="8">
                        <VStack gap="space-4">
                          <BodyShort weight="semibold">
                            <a
                              href="https://github.com/navikt/copilot/blob/main/cli/nav-pilot/schemas/agentpakke-v1.json"
                              className={linkClass}
                            >
                              Skjemaet
                            </a>
                          </BodyShort>
                          <BodyLong size="small" textColor="subtle">
                            Kontrakten binæren validerer mot, og den CI kan linte mot.
                          </BodyLong>
                        </VStack>
                      </Box>
                    </HGrid>
                  </VStack>
                </section>
              </VStack>
            </div>
            <aside className="hidden xl:block w-56 shrink-0">
              <div className="sticky top-6">
                <TableOfContents items={DOC_SECTIONS} title="Innhold på siden" />
              </div>
            </aside>
          </div>
        </Box>
      </div>
      <BackToTop />
    </main>
  );
}
