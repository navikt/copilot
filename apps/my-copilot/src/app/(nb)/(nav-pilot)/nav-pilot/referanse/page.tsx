import { BodyLong, BodyShort, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Table, TableBody, TableRow, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { FileExplorer } from "@/components/file-explorer";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, HeaderRow, code, linkClass } from "@/components/nav-pilot/doc-page";
import { LocalModelsTable } from "@/components/nav-pilot/local-model-tables";
import type { TocItem } from "@/components/table-of-contents";
import { FALLBACK_TABLE, MANIFEST_URL, getLocalModels, type LocalModel } from "@/lib/local-models";
import { CLI_COMMANDS, CONFIG_KEYS } from "./data";

export const metadata: Metadata = {
  title: "Referanse",
  description:
    "Kommandoene, avslutningskodene, konfignøklene, sikkerhetsnivåene, telemetrien, de lokale modellene og filene nav-pilot installerer.",
};

const TOC: TocItem[] = [
  { id: "kommandoer", label: "Kommandoer" },
  { id: "avslutningskoder", label: "Avslutningskoder" },
  { id: "konfignokler", label: "Konfignøkler" },
  { id: "sikkerhetsniva", label: "Sikkerhetsnivå i cplt" },
  { id: "telemetri", label: "Telemetri" },
  { id: "lokale-modeller", label: "Lokale modeller" },
  { id: "ytelse", label: "Ytelse" },
  { id: "filstruktur", label: "Filstruktur" },
  { id: "lenker", label: "Lenker" },
];

const EXIT_CODES = [
  { code: "0", when: "Kommandoen gikk bra. sync og upgrade --dry-run: du har nyeste versjon." },
  {
    code: "1",
    when: "Feil. sync og upgrade --dry-run: en oppdatering finnes. validate: agentpakka bryter kontrakten.",
  },
  {
    code: "2",
    when: "sync feilet, også når GitHubs release-API ikke svarer (låsen i repoet blir stående). install uten terminal og uten --yes, --all eller --frozen: den skriver ingenting og sier hva den ville ha skrevet.",
  },
  {
    code: "3",
    when: "install --frozen installerte ikke det .nav-pilot/agentpakke.lock.json peker på: ingen lås, ingen låst versjon, en annen versjon eller en delvis installasjon.",
  },
  {
    code: "klientens",
    when: "Når nav-pilot starter en klient, gir den videre klientens avslutningskode. Kunne den ikke starte klienten, blir koden 1. En klient som ble stoppet av et signal, gir 128 pluss signalnummeret, slik et skall gjør.",
  },
];

const SECURITY_LEVELS = [
  { setting: "gh_guard", standard: "på", strict: "på" },
  { setting: "git_guard", standard: "advarer", strict: "blokkerer" },
  { setting: "proxy.forced (tvungen proxy)", standard: "av", strict: "på" },
  { setting: "proxy.default_allowlist", standard: "av", strict: "på: bare cplts egen liste og proxy.allowed_domains" },
];

const LOCAL_COMMANDS = `nav-pilot alpha local init      # laster ned modellen, setter opp miljøet og starter serveren
nav-pilot alpha local init --yes # det samme fra et skript, uten å spørre
nav-pilot alpha local start     # starter serveren igjen etter en omstart av maskinen
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local models    # modellene som tilbys, og hvilken som er i bruk
nav-pilot alpha local use <key> # velg modellen serveren laster
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha decide "..." --options ja,nei --evidence fil  # avgjørelse med faste alternativer
nav-pilot alpha local stop
nav-pilot alpha local restart   # stop og start i ett
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # viser hva som fjernes og hvor mye; --yes sletter, --all tar alle modellene
nav-pilot alpha local setup     # egen server: finner den, foreslår modell og sjekker den
nav-pilot alpha local doctor    # egen server: sjekker verktøykall, logprobs, kontekst og tid til første token`;

const nb = (n: number) => n.toLocaleString("nb-NO");

async function LiveLocalModels() {
  const { models } = await getLocalModels();
  return <LocalModels models={models} />;
}

function LocalModels({ models }: { models: LocalModel[] }) {
  const standard = models.find((m) => m.default) ?? models[0];
  return (
    <>
      <BodyLong>
        {models.length} modeller er tilgjengelige. Én er standard, resten velger du selv. Tabellen hentes fra{" "}
        <a href={MANIFEST_URL} className={linkClass}>
          modellmanifestet
        </a>
        , det samme nav-pilot leser når du kjører <code className={code}>init</code> og{" "}
        <code className={code}>start</code>. Kontekst og svar er det største vinduet og det lengste svaret nav-pilot gir
        modellen.
      </BodyLong>
      <LocalModelsTable models={models} />
      <BodyShort size="small" textColor="subtle">
        Står det en versjon under «Krever nav-pilot», skjuler eldre versjoner av nav-pilot modellen. Peker{" "}
        <code className={code}>local_model</code> på den, faller nav-pilot tilbake til standardmodellen, og{" "}
        <code className={code}>init</code>, <code className={code}>start</code> og <code className={code}>status</code>{" "}
        sier hvilken versjon du trenger. Oppdater med <code className={code}>nav-pilot upgrade</code>.
      </BodyShort>
      {standard?.temperature != null && (
        <BodyShort size="small" textColor="subtle">
          Standardmodellen kjører med temperatur {nb(standard.temperature)}
          {standard.top_p != null && <> og top_p {nb(standard.top_p)}</>}, verdiene den ble målt med.
        </BodyShort>
      )}
    </>
  );
}

export default function Referanse() {
  return (
    <DocPage
      label="Referanse"
      title="Kommandoer, konfig og tabeller"
      description="Kommandoer, konfignøkler og tabeller på én side. Søk på siden med Ctrl+F (Cmd+F på Mac)."
      toc={TOC}
      wide
    >
      <section>
        <VStack gap="space-16">
          <LinkableHeading id="kommandoer" size="medium" level="2">
            Kommandoer
          </LinkableHeading>
          <BodyLong>
            <code className={code}>nav-pilot help &lt;kommando&gt;</code> viser alle flaggene til en kommando.{" "}
            <code className={code}>install</code> spør om du vil installere i repoet (
            <code className={code}>.github/</code>) eller i hjemmekatalogen (<code className={code}>~/.copilot/</code>
            ). Med <code className={code}>--repo</code> eller <code className={code}>--user</code> svarer du på forhånd.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Kommando", "Hva den gjør"]} />
              <TableBody role="rowgroup">
                {CLI_COMMANDS.map((c) => (
                  <TableRow role="row" key={c.command}>
                    <TableDataCell role="cell">
                      <code className={code}>{c.command}</code>
                    </TableDataCell>
                    <TableDataCell role="cell">{c.description}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="avslutningskoder" size="medium" level="2">
            Avslutningskoder
          </LinkableHeading>
          <BodyLong>
            Kodene under gjelder alle kommandoer. <code className={code}>doctor</code> avslutter alltid med 0, også når
            den finner feil. Les linjene med «Solution». Kommandoene under <code className={code}>alpha local</code> har
            egne koder, som står i <code className={code}>--help</code>.
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Kode", "Når"]} />
              <TableBody role="rowgroup">
                {EXIT_CODES.map((e) => (
                  <TableRow role="row" key={e.code}>
                    <TableDataCell role="cell">
                      <code className={code}>{e.code}</code>
                    </TableDataCell>
                    <TableDataCell role="cell">{e.when}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyShort size="small" textColor="subtle">
            <code className={code}>sync --json</code> gir ett dokument med én oppføring per installasjonssted. Et sted
            som feilet, har feltet <code className={code}>error</code>.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="konfignokler" size="medium" level="2">
            Konfignøkler
          </LinkableHeading>
          <BodyLong>
            nav-pilot leser <code className={code}>~/.nav-pilot/config.toml</code>, eller fila{" "}
            <code className={code}>NAV_PILOT_CONFIG</code> peker på. Det finnes ingen konfig per repo. Et flagg vinner
            over fila, og fila vinner over standardverdien. Slik endrer du dem:{" "}
            <NextLink href="/nav-pilot/guider/tilpasse#endre-innstillinger" className={linkClass}>
              Endre innstillinger
            </NextLink>
            .
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" style={{ minWidth: "40rem" }}>
              <HeaderRow cells={["Nøkkel", "Flagg", "Verdier", "Hva den gjør"]} />
              <TableBody>
                {CONFIG_KEYS.map((k) => (
                  <TableRow key={k.key}>
                    <TableDataCell>
                      <code className={`${code} whitespace-nowrap`}>{k.key}</code>
                    </TableDataCell>
                    <TableDataCell>
                      <code className={code}>{k.flag}</code>
                    </TableDataCell>
                    <TableDataCell>{k.values}</TableDataCell>
                    <TableDataCell>{k.desc}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="sikkerhetsniva" size="medium" level="2">
            Sikkerhetsnivå i cplt
          </LinkableHeading>
          <BodyLong>
            Nivået er <code className={code}>sandbox.preset</code> i cplt. Sett det med{" "}
            <code className={code}>nav-pilot config</code>, raden «cplt security posture», så kommer Nav-hostene med.
            Hvorfor står i{" "}
            <NextLink href="/nav-pilot/forklaring/sandkassen#sikkerhetsniva" className={linkClass}>
              Sandkassen
            </NextLink>
            .
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small">
              <HeaderRow cells={["Innstilling", "standard", "strict"]} />
              <TableBody>
                {SECURITY_LEVELS.map((l) => (
                  <TableRow key={l.setting}>
                    <TableDataCell>
                      <code className={code}>{l.setting}</code>
                    </TableDataCell>
                    <TableDataCell>{l.standard}</TableDataCell>
                    <TableDataCell>{l.strict}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyShort size="small" textColor="subtle">
            Host-lista nav-pilot skriver: <code className={code}>~/.nav-pilot/cplt-allowed-domains.txt</code>. På Linux
            krever strict Landlock ABI v4 (kjerne 6.7 eller nyere).
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="telemetri" size="medium" level="2">
            Telemetri
          </LinkableHeading>
          <BodyLong>
            Slå av målingene i skallprofilen din med én av disse. Hva som måles og hvorfor, står i{" "}
            <NextLink href="/nav-pilot/forklaring/personvern" className={linkClass}>
              Personvern og telemetri
            </NextLink>
            . Hele lista står i{" "}
            <a href="https://github.com/navikt/copilot/blob/main/cli/nav-pilot/TELEMETRY.md" className={linkClass}>
              TELEMETRY.md
            </a>
            .
          </BodyLong>
          <div className="overflow-x-auto">
            <Table size="small" className="table-stack" role="table">
              <HeaderRow stack cells={["Variabel", "Virkning"]} />
              <TableBody role="rowgroup">
                <TableRow role="row">
                  <TableDataCell role="cell">
                    <code className={code}>DO_NOT_TRACK=1</code>
                  </TableDataCell>
                  <TableDataCell role="cell">
                    Slår av målingene i nav-pilot, i Copilot og opencode når nav-pilot starter dem, og i andre verktøy
                    som følger konvensjonen.
                  </TableDataCell>
                </TableRow>
                <TableRow role="row">
                  <TableDataCell role="cell">
                    <code className={code}>NAV_PILOT_TELEMETRY_ENABLED=false</code>
                  </TableDataCell>
                  <TableDataCell role="cell">Slår av målingene i nav-pilot.</TableDataCell>
                </TableRow>
              </TableBody>
            </Table>
          </div>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lokale-modeller" size="medium" level="2">
            Lokale modeller{" "}
            <Tag variant="warning" size="small">
              alfa
            </Tag>
          </LinkableHeading>
          <CodeBlock compact>{LOCAL_COMMANDS}</CodeBlock>
          {/* Skallet viser reservekopien til manifestet er hentet. */}
          <Suspense fallback={<LocalModels models={FALLBACK_TABLE.models} />}>
            <LiveLocalModels />
          </Suspense>
          <BodyShort size="small" textColor="subtle">
            Én kjøring er ikke en måling. Alle kjøringene står i{" "}
            <a href="https://github.com/navikt/mlx-workspace/blob/main/MODELS.md" className={linkClass}>
              MODELS.md
            </a>
            . Hva hver modell er godkjent for, står i{" "}
            <NextLink href="/nav-pilot/forklaring/lokal-modell#malte-grenser" className={linkClass}>
              Målte grenser
            </NextLink>
            .
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="ytelse" size="medium" level="2">
            Ytelse
          </LinkableHeading>
          <BodyLong>
            nav-pilot skal starte klienten uten å vente på nettet. Målene gjelder også når nettet ikke svarer:
          </BodyLong>
          <Bullets>
            <li>Under 150 ms fra du kjører nav-pilot til klienten starter.</li>
            <li>Under 200 ms fra økten slutter til du har terminalen tilbake.</li>
            <li>
              Under 50 ms for <code className={code}>--version</code> og <code className={code}>--help</code>. De sender
              ikke telemetri.
            </li>
            <li>
              Kommandoer som ikke trenger nettet, som <code className={code}>config get</code>, venter høyst 300 ms på å
              sende telemetrien.
            </li>
          </Bullets>
          <BodyLong>
            Det som trenger nett, skjer i bakgrunnen eller leses fra en kopi på maskinen. Alle filene ligger i{" "}
            <code className={code}>~/.nav-pilot/</code>:
          </BodyLong>
          <Bullets>
            <li>
              <code className={code}>cache.json</code>: versjonssjekken spør GitHub høyst én gang i døgnet, i
              bakgrunnen. Neste kommando sier fra hvis det finnes en ny versjon.
            </li>
            <li>
              <code className={code}>sources/</code>: en kopi av agentpakka per kilde: navikt/copilot (for opencode og
              pi) og en agentpakke fra et annet team (<code className={code}>source</code> i konfigurasjonen). En ny
              kopi hentes mens økten kjører, høyst én gang i timen.
            </li>
            <li>
              <code className={code}>client-versions.json</code>: svaret fra{" "}
              <code className={code}>copilot --version</code> og <code className={code}>opencode --version</code>.
              nav-pilot spør på nytt når klienten er oppdatert eller installert på nytt.
            </li>
            <li>
              <code className={code}>surveys.json</code> og <code className={code}>news.json</code>: undersøkelser og
              nyheter hentes mens økten kjører. Nyhetslinja etter en økt kommer høyst én gang i døgnet.
            </li>
          </Bullets>
          <BodyLong>
            Unntaket er den første nedlastingen av en agentpakke. Finnes ingen kopi i{" "}
            <code className={code}>sources/</code>, venter oppstarten på nedlastingen, høyst 30 sekunder. Mislykkes den,
            venter ikke oppstartene den neste timen på nettet, men prøver igjen i bakgrunnen. En kopi av en agentpakke
            fra et annet team brukes i høyst ett døgn, fordi manifestet bestemmer hvordan økten starter. Er kopien
            eldre, venter oppstarten på en ny: høyst 15 sekunder når nav-pilot har manifestet fra før, ellers 30.
          </BodyLong>
          <BodyLong>
            En oppstart som ikke trenger noe fra deg, skriver ingenting. Det som er nytt, sier nav-pilot én gang, og en
            advarsel kommer på nytt først når noe endrer seg. <code className={code}>nav-pilot --verbose</code> viser
            hva oppstarten gjør: sandkassemappe, klient, agent og modell.
          </BodyLong>
          <BodyLong>
            Testen <code className={code}>TestLaunchBudget</code> passer på målene i CI. Den starter nav-pilot med
            falske klienter, med telemetrien på og et nett som tar imot forbindelser uten å svare. Den feiler når
            medianen av fem kjøringer er mer enn tre ganger målet. Så mye tregere blir det bare når noe venter på
            nettet.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="filstruktur" size="medium" level="2">
            Filstruktur
          </LinkableHeading>
          <BodyLong>
            Filene <code className={code}>nav-pilot install --repo</code> legger i{" "}
            <code className={code}>.github/</code>. Copilot leser dem og tilpasser svarene. Klikk på en fil for å se hva
            den gjør.
          </BodyLong>
          <FileExplorer />
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="lenker" size="medium" level="2">
            Lenker
          </LinkableHeading>
          <Bullets>
            <li>
              <NextLink href="/kom-i-gang" className={linkClass}>
                Kom i gang
              </NextLink>
              : installer nav-pilot og cplt
            </li>
            <li>
              <NextLink href="/nav-pilot/guider" className={linkClass}>
                Guider
              </NextLink>
              : installere, tilpasse, synkronisere, lokal modell og feilsøking
            </li>
            <li>
              <NextLink href="/nav-pilot/forklaring" className={linkClass}>
                Forklaring
              </NextLink>
              : hvorfor nav-pilot virker som det gjør
            </li>
            <li>
              <NextLink href="/nav-pilot/klienter" className={linkClass}>
                Klienter
              </NextLink>
              : Copilot CLI, opencode og pi, og hva hver av dem kan
            </li>
            <li>
              <NextLink href="/nav-pilot/agentpakker" className={linkClass}>
                Agentpakker
              </NextLink>
              : for deg som vil lage eller bygge videre på en pakke
            </li>
            <li>
              <NextLink href="/verktoy" className={linkClass}>
                Verktøykatalog
              </NextLink>
              : installer enkeltkomponenter
            </li>
            <li>
              <a href="https://github.com/navikt/copilot/blob/main/docs/README.nav-pilot.md" className={linkClass}>
                README.nav-pilot.md
              </a>{" "}
              på GitHub
            </li>
          </Bullets>
        </VStack>
      </section>
    </DocPage>
  );
}
