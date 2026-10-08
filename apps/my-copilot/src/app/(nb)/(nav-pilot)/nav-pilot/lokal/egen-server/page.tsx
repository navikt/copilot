import { BodyLong, BodyShort, Box, Label, Tag, VStack } from "@navikt/ds-react";
import type { Metadata } from "next";
import NextLink from "next/link";
import { CodeBlock } from "@/components/code-block";
import { LinkableHeading } from "@/components/linkable-heading";
import { DocPage, code, linkClass } from "@/components/nav-pilot/doc-page";
import type { TocItem } from "@/components/table-of-contents";
import { NAV_PILOT_APT_INSTALL, NAV_PILOT_BREW_INSTALL, NAV_PILOT_INSTALL_SCRIPT } from "@/lib/install-commands";

export const metadata: Metadata = {
  title: "Kom i gang med egen LLM-server",
  description:
    "Bruk en OpenAI-kompatibel server du kjører selv, som Ollama eller llama-server, som lokal modell for nav-pilot på Linux eller Mac.",
};

const TOC: TocItem[] = [
  { id: "installer", label: "1. Installer nav-pilot" },
  { id: "start-serveren", label: "2. Start serveren" },
  { id: "setup", label: "3. Koble til med setup" },
  { id: "init", label: "4. Slå den på med init" },
  { id: "doctor", label: "5. Sjekk den med doctor" },
  { id: "klient", label: "6. Bytt til opencode" },
  { id: "forste-okt", label: "7. Første økt" },
];

const OLLAMA = `# terminal 1: blir stående så lenge serveren kjører
OLLAMA_CONTEXT_LENGTH=65536 ollama serve

# terminal 2
ollama pull qwen3.6:35b`;

const LLAMA_SERVER = `# egen terminal: blir stående så lenge serveren kjører
# legg til --n-cpu-moe 999 på en GPU med 8 GB eller mindre
llama-server --jinja -c 65536 --port 8080 -hf unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL`;

const SMALL_MEMORY = `# Ollama
OLLAMA_CONTEXT_LENGTH=16384 ollama serve

# llama-server: ekspertene i vanlig minne (MoE-modeller)
llama-server --jinja -c 16384 --n-cpu-moe 999 --port 8080 -hf unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL

# llama-server: bare 20 lag på grafikkortet (tette modeller)
llama-server --jinja -c 16384 -ngl 20 --port 8080 -m modell.gguf`;

const BY_HAND = `# Ollama
nav-pilot config set local_endpoint http://127.0.0.1:11434/v1
nav-pilot config set local_endpoint_model qwen3.6:35b

# llama-server
nav-pilot config set local_endpoint http://127.0.0.1:8080/v1
nav-pilot config set local_endpoint_model unsloth/Qwen3.6-35B-A3B-GGUF:UD-Q4_K_XL`;

export default function EgenServer() {
  return (
    <DocPage
      label="Introduksjon"
      title="Kom i gang med egen LLM-server"
      description="Har du Linux, eller vil du bruke Ollama eller llama-server på Macen, kan nav-pilot bruke en server du kjører selv. Da laster nav-pilot ikke ned noe, starter ingenting og trenger ikke sudo."
      badge={
        <Tag variant="warning" size="small" className="uppercase tracking-wide">
          Alfa, ikke målt
        </Tag>
      }
      toc={TOC}
    >
      <BodyLong>
        Delegerte oppgaver, loop guard og <code className={code}>alpha decide</code> bruker serveren din. Koden din
        sendes dit, så nav-pilot godtar bare localhost og private IP-adresser, som 127.0.0.1 og 192.168.x.x. Modellen på
        serveren din er ikke målt, så tallene i{" "}
        <NextLink href="/innsikt/lokale-modeller" className={linkClass}>
          Målinger
        </NextLink>{" "}
        gjelder ikke.
      </BodyLong>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="installer" size="medium" level="2">
            1. Installer nav-pilot
          </LinkableHeading>
          <Label size="small">Linux og WSL</Label>
          <CodeBlock compact>{NAV_PILOT_INSTALL_SCRIPT}</CodeBlock>
          <Label size="small">Debian og Ubuntu, fra apt-arkivet</Label>
          <CodeBlock compact>{NAV_PILOT_APT_INSTALL}</CodeBlock>
          <Label size="small">Mac</Label>
          <CodeBlock compact>{NAV_PILOT_BREW_INSTALL}</CodeBlock>
          <BodyLong>
            Du får både nav-pilot og cplt. Har du nav-pilot fra før, eller vil du oppgradere, se{" "}
            <NextLink href="/nav-pilot/guider/installere-og-oppgradere" className={linkClass}>
              Installere og oppgradere
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="start-serveren" size="medium" level="2">
            2. Start serveren
          </LinkableHeading>
          <Label size="small">Ollama</Label>
          <CodeBlock compact>{OLLAMA}</CodeBlock>
          <Label size="small">llama-server (llama.cpp)</Label>
          <CodeBlock compact>{LLAMA_SERVER}</CodeBlock>
          <BodyLong>
            Både <code className={code}>ollama serve</code> og <code className={code}>llama-server</code> kjører i
            forgrunnen og opptar terminalen. Start serveren i en egen terminal, og kjør resten av stegene i en annen.
          </BodyLong>
          <BodyLong>
            Hvis <code className={code}>llama-server</code> fra llama.cpp-releasen for Ubuntu ikke starter og feilen er{" "}
            <code className={code}>libgomp.so.1: cannot open shared object file</code>, mangler OpenMP-biblioteket. Det
            mangler ofte i containere, minimale serverimager og WSL. Installer det med{" "}
            <code className={code}>sudo apt install libgomp1</code>, eller{" "}
            <code className={code}>sudo apt-get install -y libgomp1</code> i et skript.
          </BodyLong>
          <BodyLong>
            Vi anbefaler Qwen3.6-35B-A3B i dynamisk 4-bit (unsloth UD-Q4_K_XL). Det er den GGUF-varianten som ligger
            nærmest modellen vi har målt på Mac. <code className={code}>setup</code> finner også LM Studio og vLLM, men
            dem har vi ikke prøvd.
          </BodyLong>
          <Box background="warning-soft" padding="space-16" borderRadius="8">
            <VStack gap="space-8">
              <Label size="small">Ollama kan gi modellen for lite kontekst</Label>
              <BodyShort size="small">
                Ollama gir modellen bare 4 096 tokens kontekst på maskiner med under 24 GB grafikkminne, og det kan ikke
                endres over <code className={code}>/v1</code>. Første melding i en Copilot-økt er på rundt 22 000
                tokens, så resten blir kuttet eller avvist. Start derfor Ollama med{" "}
                <code className={code}>OLLAMA_CONTEXT_LENGTH=65536</code>, som over, eller lag en modell med en
                Modelfile som har <code className={code}>PARAMETER num_ctx 65536</code>.
              </BodyShort>
            </VStack>
          </Box>
          <LinkableHeading id="lite-minne" size="small" level="3">
            Lite minne
          </LinkableHeading>
          <BodyLong>
            Modellen og en kontekst på 65 536 tokens får ikke plass sammen på et grafikkort med 8 GB eller mindre. Bruk
            en kontekst som får plass, for eksempel 16 384, og la en del av modellen ligge i vanlig minne:
          </BodyLong>
          <CodeBlock compact>{SMALL_MEMORY}</CodeBlock>
          <BodyLong>
            Ollama fordeler lagene selv. <code className={code}>--n-cpu-moe 999</code> holder ekspertene i en MoE-modell
            som Qwen3.6-35B-A3B i vanlig minne. Med en tett modell velger du antall lag på grafikkortet med{" "}
            <code className={code}>-ngl</code>.
          </BodyLong>
          <BodyLong>
            En Copilot- eller opencode-økt trenger minst 30 000 tokens kontekst. Med mindre feiler kontekstsjekken i{" "}
            <code className={code}>setup</code>, men den tilbyr å lagre likevel. Korte prompter og{" "}
            <code className={code}>alpha decide</code> virker da, men ikke en hel økt. Går serveren tom for minne under
            sjekken, lagrer <code className={code}>setup</code> ingenting. Trenger du økter, må du ha et grafikkort med
            mer minne, en Mac med 32 GB eller mer, eller bruke modellene i skyen.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="setup" size="medium" level="2">
            3. Koble til med setup
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot alpha local setup`}</CodeBlock>
          <BodyLong>
            <code className={code}>setup</code> finner servere som kjører på maskinen, foreslår modellen som ligger
            nærmest vår egen, og sjekker den. Mangler modellen i Ollama, eller er konteksten for liten, tilbyr den å
            hente modellen eller lage en kopi med 64k kontekst. Den starter aldri en server selv.
          </BodyLong>
          <BodyLong>
            Til slutt viser den <code className={code}>local_endpoint</code>,{" "}
            <code className={code}>local_endpoint_model</code> og <code className={code}>local_enabled = true</code>, og
            lagrer dem bare hvis du svarer ja. Uten terminal lagrer den bare med <code className={code}>--yes</code>.
            Skal den også hente modellen eller lage kopien, legg til <code className={code}>--pull</code> eller{" "}
            <code className={code}>--fix-context</code>.
          </BodyLong>
          <BodyLong>
            Feiler bare kontekstsjekken, spør <code className={code}>setup</code> om du vil lagre likevel (se{" "}
            <a href="#lite-minne" className={linkClass}>
              Lite minne
            </a>
            ). Uten terminal lagrer den ikke, heller ikke med <code className={code}>--yes</code>, men viser kommandoene
            som gjør det.
          </BodyLong>
          <BodyLong>
            <code className={code}>mlx_lm.server</code> lister alle MLX-modellene i Hugging Face-cachen, ikke bare den
            du startet den med, og laster den modellen en forespørsel ber om. <code className={code}>setup</code>{" "}
            foreslår da samme bygg som nav-pilot selv kjører, og sier fra om at det kan være en annen modell enn den du
            startet serveren med. Vil du ha en bestemt modell, bruk <code className={code}>--model</code>.
          </BodyLong>
          <BodyLong>
            Slutter serveren å svare midt i kontekstsjekken, er den trolig tom for minne. Da foreslår{" "}
            <code className={code}>setup</code> en mindre kontekst, ikke en større.
          </BodyLong>
          <BodyLong>Vil du heller sette det for hånd:</BodyLong>
          <CodeBlock compact>{BY_HAND}</CodeBlock>
          <BodyShort size="small" textColor="subtle">
            Modell-id-en er det serveren lister på <code className={code}>/v1/models</code>.{" "}
            <code className={code}>setup</code> viser dem når den leter etter servere, og{" "}
            <code className={code}>doctor</code> viser dem hvis modellen du har satt, ikke er blant dem.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="init" size="medium" level="2">
            4. Slå den på med init
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot alpha local init`}</CodeBlock>
          <BodyLong>
            Med <code className={code}>local_endpoint</code> satt laster <code className={code}>init</code> ikke ned
            noe. Den sjekker serveren og slår på delegering. Feiler en av sjekkene, slår den ingenting på og sier hva du
            må rette. Svarte du ja i <code className={code}>setup</code>, er delegering allerede på, og{" "}
            <code className={code}>init</code> kjører de samme sjekkene en gang til.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="doctor" size="medium" level="2">
            5. Sjekk den med doctor
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot alpha local doctor`}</CodeBlock>
          <BodyLong>
            <code className={code}>doctor</code> sjekker verktøykall, logprobs, kontekst og tid til første token. Den
            sender rundt 30 000 tokens og feiler hvis serveren kutter prompten. Kommandoen sjekker bare egen LLM-server
            (<code className={code}>local_endpoint</code>), ikke modellen nav-pilot setter opp på Mac.
          </BodyLong>
          <BodyShort size="small" textColor="subtle">
            <code className={code}>alpha decide</code> trenger logprobs. Ollama fra v0.12.11, llama-server og vLLM gir
            dem, LM Studio gjør det ikke. Uten logprobs sier <code className={code}>decide</code> fra med en gang, mens
            delegeringen virker som før.
          </BodyShort>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="klient" size="medium" level="2">
            6. Bytt til opencode
          </LinkableHeading>
          <CodeBlock compact>{`nav-pilot config set client opencode`}</CodeBlock>
          <BodyLong>
            Delegering krever opencode som klient. Der blir modellen på serveren din subagenten{" "}
            <code className={code}>local-worker</code>. Copilot CLI har ingen slik subagent.
          </BodyLong>
        </VStack>
      </section>

      <section>
        <VStack gap="space-16">
          <LinkableHeading id="forste-okt" size="medium" level="2">
            7. Første økt
          </LinkableHeading>
          <CodeBlock compact>{`cd ~/kode/mitt-repo
nav-pilot`}</CodeBlock>
          <BodyLong>
            Be om en mekanisk endring over flere filer, som «legg til parameteren <code className={code}>ctx</code> i
            alle kall til <code className={code}>hentBruker</code>». Modellen på serveren din er ikke målt, så nav-pilot
            bruker delegeringsnivået <code className={code}>conservative</code>, uansett hva{" "}
            <code className={code}>local_dispatch</code> står på: hovedagenten får bare den generelle instruksen om
            delegering, og nav-pilot stopper ingen redigeringer. Hovedagenten vurderer selv hva den delegerer.
          </BodyLong>
          <BodyLong>
            <code className={code}>nav-pilot alpha local status</code> viser serveren, modellen og om delegering er på.
            Vil du stille modellen spørsmål fra hooks og skript, fortsett med{" "}
            <NextLink href="/nav-pilot/lokal/decide" className={linkClass}>
              Din første decide-hook
            </NextLink>
            .
          </BodyLong>
        </VStack>
      </section>
    </DocPage>
  );
}
