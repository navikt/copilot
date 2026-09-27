import { BodyLong, BodyShort, Box, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { Suspense } from "react";
import { Table, TableHeader, TableBody, TableRow, TableHeaderCell, TableDataCell } from "@/components/aksel-table";
import { CodeBlock } from "@/components/code-block";
import { FileExplorer } from "@/components/file-explorer";
import { LinkableHeading } from "@/components/linkable-heading";
import { Bullets, DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import { LocalModelsTable } from "@/components/nav-pilot/local-model-tables";
import type { TocItem } from "@/components/table-of-contents";
import { FALLBACK_TABLE, MANIFEST_URL, getLocalModels, type LocalModel } from "@/lib/local-models";
import { CLI_COMMANDS, CONFIG_KEYS } from "./data";

export const metadata: Metadata = {
  title: "Referanse — nav-pilot",
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
  { id: "filstruktur", label: "Filstruktur" },
  { id: "klienter", label: "Klienter" },
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
    when: "sync feilet, også når GitHubs release-API ikke svarer (pinnen i repoet blir stående). install uten terminal og uten --yes, --all eller --frozen: den skriver ingenting og sier hva den ville ha skrevet.",
  },
  {
    code: "3",
    when: "install --frozen installerte ikke det .nav-pilot/agentpakke.lock.json peker på: ingen lås, ingen pinnet versjon, en annen versjon eller en delvis installasjon.",
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
  { setting: "proxy.default_allowlist", standard: "av", strict: "på: bare cplt sin liste og proxy.allowed_domains" },
];

const LOCAL_COMMANDS = `nav-pilot alpha local init      # laster ned modellen, setter opp miljøet og starter serveren
nav-pilot alpha local init --yes # det samme fra et skript, uten å spørre
nav-pilot alpha local start     # starter serveren igjen etter en omstart av maskinen
nav-pilot alpha local status    # kjører den? svarer den? hvilken modell? hva har den gjort?
nav-pilot alpha local models    # modellene som tilbys, og hvilken som er i bruk
nav-pilot alpha local use <key> # velg modellen serveren laster
nav-pilot alpha local ask -p "..."  # still ett spørsmål rett til modellen
nav-pilot alpha decide "..." --options ja,nei --evidence fil  # typet avgjørelse
nav-pilot alpha local stop
nav-pilot alpha local restart   # stop og start i ett
nav-pilot alpha local on        # skru på igjen etter off
nav-pilot alpha local off       # slutt å sende oppgaver dit; vektene blir liggende
nav-pilot alpha local purge     # viser hva som fjernes og hvor mye; --yes sletter, --all tar alle modellene
nav-pilot alpha local setup     # egen server: finner den, foreslår modell og sjekker den
nav-pilot alpha local doctor    # egen server: sjekker verktøykall, logprobs, kontekst og tid til første token`;

const CLIENTS = [
  {
    name: "copilot",
    status: (
      <Tag size="small" variant="info">
        Standard
      </Tag>
    ),
    desc: "GitHub Copilot CLI i cplt-sandkassen. Agentpakka virker også i VS Code, JetBrains og på github.com.",
  },
  {
    name: "opencode",
    status: (
      <Tag size="small" variant="success">
        Full støtte
      </Tag>
    ),
    desc: "nav-pilot legger Nav-konteksten i ~/.config/opencode/ og holder den oppdatert.",
  },
  {
    name: "pi",
    status: (
      <Tag size="small" variant="warning">
        Eksperimentell
      </Tag>
    ),
    desc: "pi i cplt-sandkassen, med skills, agenter og AGENTS.md lagt inn ved oppstart. Krever både pi og cplt.",
  },
];

const nb = (n: number) => n.toLocaleString("nb-NO");

function HeaderRow({ cells }: { cells: string[] }) {
  return (
    <TableHeader>
      <TableRow>
        {cells.map((c) => (
          <TableHeaderCell key={c} scope="col">
            {c}
          </TableHeaderCell>
        ))}
      </TableRow>
    </TableHeader>
  );
}

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
        Står det en versjon under «Krever nav-pilot», skjuler eldre nav-pilot modellen. Peker{" "}
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
      title="Kommandoer og konfig"
      description="Alt nav-pilot kan stilles inn med, på én side. Bruk søk i siden (Ctrl+F)."
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
            <Table size="small">
              <HeaderRow cells={["Kommando", "Hva den gjør"]} />
              <TableBody>
                {CLI_COMMANDS.map((c) => (
                  <TableRow key={c.command}>
                    <TableDataCell>
                      <code className={`${code} whitespace-nowrap`}>{c.command}</code>
                    </TableDataCell>
                    <TableDataCell>{c.description}</TableDataCell>
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
            <Table size="small">
              <HeaderRow cells={["Kode", "Når"]} />
              <TableBody>
                {EXIT_CODES.map((e) => (
                  <TableRow key={e.code}>
                    <TableDataCell>
                      <code className={code}>{e.code}</code>
                    </TableDataCell>
                    <TableDataCell>{e.when}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <BodyShort size="small" textColor="subtle">
            <code className={code}>sync --json</code> gir ett dokument med én oppføring per omfang. Et omfang som
            feilet, har feltet <code className={code}>error</code>.
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
            <Table size="small">
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
            <code className={code}>sandbox.preset</code> i cplt. Sett det med{" "}
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
            <Table size="small">
              <HeaderRow cells={["Variabel", "Virkning"]} />
              <TableBody>
                <TableRow>
                  <TableDataCell>
                    <code className={code}>DO_NOT_TRACK=1</code>
                  </TableDataCell>
                  <TableDataCell>
                    Slår av målingene i nav-pilot, i Copilot og opencode når nav-pilot starter dem, og i andre verktøy
                    som følger konvensjonen.
                  </TableDataCell>
                </TableRow>
                <TableRow>
                  <TableDataCell>
                    <code className={code}>NAV_PILOT_TELEMETRY_ENABLED=false</code>
                  </TableDataCell>
                  <TableDataCell>Slår av målingene i nav-pilot.</TableDataCell>
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
          <LinkableHeading id="klienter" size="medium" level="2">
            Klienter
          </LinkableHeading>
          <BodyLong>
            Velg klient med <code className={code}>--client</code> eller med <code className={code}>client</code> i
            konfigfila: <code className={code}>nav-pilot config set client opencode</code>.
          </BodyLong>
          <LinkableHeading id="stotte-klienter" size="small" level="3">
            Støttede klienter
          </LinkableHeading>
          <div className="overflow-x-auto">
            <Table size="small">
              <HeaderRow cells={["Klient", "Status", "Hva du får"]} />
              <TableBody>
                {CLIENTS.map((c) => (
                  <TableRow key={c.name}>
                    <TableDataCell>
                      <code className={code}>{c.name}</code>
                    </TableDataCell>
                    <TableDataCell>{c.status}</TableDataCell>
                    <TableDataCell>{c.desc}</TableDataCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
          <Box background="info-soft" borderRadius="8" padding="space-16">
            <BodyShort size="small">
              For <code className={code}>copilot</code> henter nav-pilot ikke GitHub-tokenet selv. Med gh-vakta i cplt
              på henter cplt det fra <code className={code}>GH_TOKEN</code>, <code className={code}>GITHUB_TOKEN</code>,{" "}
              <code className={code}>COPILOT_GITHUB_TOKEN</code> eller <code className={code}>gh auth token</code>.{" "}
              <code className={code}>copilot_auth_mode</code> bestemmer hvilke kilder som slipper gjennom:{" "}
              <code className={code}>env_only</code> stopper oppstarten uten token i miljøet, og{" "}
              <code className={code}>gh_only</code> fjerner token-variablene.
            </BodyShort>
          </Box>
          <LinkableHeading id="opencode" size="small" level="3">
            opencode
          </LinkableHeading>
          <BodyLong>
            Med <code className={code}>--client opencode</code> legger nav-pilot AGENTS.md, skills, kommandoer og
            agenter i <code className={code}>~/.config/opencode/</code> og oppdaterer dem ved hver oppstart.
          </BodyLong>
          <Bullets>
            <li>
              Endrer du en av filene selv, overskriver ikke nav-pilot den.{" "}
              <code className={code}>~/.config/opencode/.nav-pilot-state.json</code> holder rede på hvilke filer og
              hvilken versjon som er installert.
            </li>
            <li>
              Velger du ikke modell selv, bruker nav-pilot standarden agentpakka oppgir. For agentpakka nav-pilot er det
              GPT-6 Sol. Oppgir pakka ingen, velger opencode. Hvilke modeller du får bruke, avhenger også av
              Copilot-abonnementet ditt.
            </li>
            <li>nav-pilot setter opp OpenTelemetry for opencode.</li>
            <li>
              Maskeringen av hemmeligheter og fødselsnumre, løkkevakta og hookene du har installert, kjører også i
              opencode. Feiler maskeringen, holder nav-pilot verktøyresultatet tilbake.{" "}
              <code className={code}>tools:</code>
              -lista til en agent blir tillatelser i opencode.
            </li>
            <li>
              Deling til opencode.ai er av, og oppdateringer kommer som varsel. MCP-servere som ikke står i Navs
              MCP-register, slår nav-pilot av for økten, som i Copilot. Godkjente servere står i{" "}
              <NextLink href="/verktoy" className={linkClass}>
                verktøykatalogen
              </NextLink>
              . <code className={code}>nav-pilot doctor</code> viser hvilke servere som slås av, og om versjonen av
              opencode er testet.
            </li>
          </Bullets>
          <BodyShort size="small" textColor="subtle">
            <code className={code}>mode = autopilot</code>, <code className={code}>context_tier</code> og{" "}
            <code className={code}>ask_user</code> gjelder bare Copilot. Har du satt dem og bruker opencode, skriver
            nav-pilot én advarsel. <code className={code}>nav-pilot export opencode</code> skriver agentpakka til{" "}
            <code className={code}>.opencode/</code> i repoet, men trengs ikke for å bruke opencode.
          </BodyShort>
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
